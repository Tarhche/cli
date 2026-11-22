// Package workloadtest serves the workload's routes in tests: an API that
// answers what a test says it does, keeping every request it was sent, and
// the workload's resources as the API answers them.
package workloadtest

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Request is a request the API was sent.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

// Route is the route of a request, as "GET /api/...".
func (r Request) Route() string {
	return r.Method + " " + r.Path
}

// answer is what a route is answered with.
type answer struct {
	status int
	body   []byte
}

// API answers each route as a test says, and every other one 404 as the API
// answers what is not there.
type API struct {
	lock     sync.Mutex
	answers  map[string][]answer
	requests []Request
	onEach   func(Request)
}

// New is an API that answers nothing yet.
func New() *API {
	return &API{answers: make(map[string][]answer)}
}

// On answers method and path, whatever the query, with status and body: a
// string is JSON as it is, nil is nothing, and anything else is written as
// JSON. Answering a route again queues the answer: each request takes the
// next, and the last is the answer to every request after it.
func (a *API) On(method, path string, status int, body any) *API {
	a.lock.Lock()
	defer a.lock.Unlock()

	route := method + " " + path
	a.answers[route] = append(a.answers[route], answer{status: status, body: encoded(body)})

	return a
}

// OnEach calls observe with every request as it is answered, which is how a
// test stops what follows an API at the request it waits for.
func (a *API) OnEach(observe func(Request)) *API {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.onEach = observe

	return a
}

func encoded(body any) []byte {
	switch b := body.(type) {
	case nil:
		return nil
	case string:
		return []byte(b)
	}

	content, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	return content
}

// ServeHTTP answers r as the test said.
func (a *API) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	request := Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)}

	a.lock.Lock()
	a.requests = append(a.requests, request)

	queued, found := a.answers[request.Route()]
	if len(queued) > 1 {
		a.answers[request.Route()] = queued[1:]
	}

	observe := a.onEach
	a.lock.Unlock()

	if observe != nil {
		observe(request)
	}

	if !found {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(rw, `{"code":"not_found"}`)

		return
	}

	if queued[0].body != nil {
		rw.Header().Set("Content-Type", "application/json")
	}

	rw.WriteHeader(queued[0].status)
	_, _ = rw.Write(queued[0].body)
}

// Requests are every request the API was sent, in order.
func (a *API) Requests() []Request {
	a.lock.Lock()
	defer a.lock.Unlock()

	return append([]Request(nil), a.requests...)
}

// Routes are the routes of every request the API was sent, in order.
func (a *API) Routes() []string {
	requests := a.Requests()

	routes := make([]string, len(requests))
	for i, r := range requests {
		routes[i] = r.Route()
	}

	return routes
}

// Sent is the last request the API was sent on method and path, and whether
// it was sent one.
func (a *API) Sent(method, path string) (Request, bool) {
	requests := a.Requests()

	for i := len(requests) - 1; i >= 0; i-- {
		if requests[i].Method == method && requests[i].Path == path {
			return requests[i], true
		}
	}

	return Request{}, false
}

// Object is something the API answers, as JSON reads it.
type Object map[string]any

// With is a copy of o with value at path, its keys separated by dots:
// "status.state".
func (o Object) With(path string, value any) Object {
	copied := deepCopy(o)

	keys := strings.Split(path, ".")
	at := copied

	for _, key := range keys[:len(keys)-1] {
		next, isObject := at[key].(Object)
		if !isObject {
			next = Object{}
			at[key] = next
		}

		at = next
	}

	at[keys[len(keys)-1]] = value

	return copied
}

func deepCopy(o Object) Object {
	copied := maps.Clone(o)

	for key, value := range copied {
		if nested, isObject := value.(Object); isObject {
			copied[key] = deepCopy(nested)
		}
	}

	return copied
}

// Page is a page of a listing that is paged, the only one.
func Page(items ...Object) Object {
	return Object{"items": orNone(items), "pagination": Object{"total_pages": 1, "current_page": 1}}
}

// Items is a listing that is not paged.
func Items(items ...Object) Object {
	return Object{"items": orNone(items)}
}

func orNone(items []Object) []Object {
	if items == nil {
		return []Object{}
	}

	return items
}

// Created is when every fixture was created: three hours before the test,
// which docker says as "3 hours ago".
func Created() time.Time {
	return time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Second)
}

func metadata(uuid, name string) Object {
	return Object{
		"uuid":       uuid,
		"name":       name,
		"owner_uuid": "owner-1",
		"created_at": Created().Format(time.RFC3339),
	}
}

// VM is a VM as the API answers one: a machine booting ubuntu, or a Docker
// VM, with 1 vCPU, 512 MiB of memory and 4 GiB of disk.
func VM(uuid, name string, docker bool, state string) Object {
	image := "ubuntu:24.04"
	if docker {
		image = "ghcr.io/tarhche/docker:29"
	}

	return Object{
		"kind":     "vm",
		"metadata": metadata(uuid, name).With("slug", name+"-x1y2"),
		"spec": Object{
			"image":           image,
			"resources":       Object{"cpus": 1, "memory": 512 << 20, "disk": 4 << 30},
			"ports":           []uint{8080},
			"network":         Object{"ingress": "allow", "egress": "allow"},
			"persistent_disk": true,
		},
		"status": Object{"state": state, "stats": nil},
		"docker": docker,
		"urls":   []Object{{"port": 8080, "url": "https://" + name + "-x1y2-8080.workload.example"}},
	}
}

// Container is a container as a listing across VMs answers one: made, its
// docker id id, running nginx in the VM vmUUID, called vmName.
func Container(uuid, id, name, state, vmUUID, vmName string) Object {
	return Object{
		"kind":     "container",
		"metadata": metadata(uuid, name),
		"spec": Object{
			"vm":    Object{"uuid": vmUUID},
			"name":  name,
			"image": "nginx:1.27",
			"ports": []Object{{"container_port": 80, "host_port": 8080, "protocol": "tcp"}},
		},
		"status": Object{
			"state":         state,
			"id":            id,
			"name":          name,
			"image":         "nginx:1.27",
			"docker_state":  "running",
			"docker_status": "Up 2 hours",
			"command":       "/docker-entrypoint.sh nginx -g 'daemon off;'",
			"ports":         []Object{{"container_port": 80, "host_port": 8080, "host_ip": "0.0.0.0", "protocol": "tcp"}},
			"networks":      []string{"bridge"},
			"created_at":    Created().Format(time.RFC3339),
			"stats":         nil,
		},
		"vm_uuid": vmUUID,
		"vm_name": vmName,
	}
}

// Image is an image as a VM's listing answers one, held under reference,
// its docker id id.
func Image(uuid, id, reference string) Object {
	return Object{
		"kind":     "image",
		"metadata": metadata(uuid, reference),
		"spec":     Object{"reference": reference},
		"status": Object{
			"state":      "present",
			"reference":  reference,
			"id":         id,
			"tags":       []string{reference},
			"size":       187 << 20,
			"created_at": Created().Format(time.RFC3339),
			"in_use":     true,
		},
	}
}

// Network is a network as a VM's listing answers one, its docker id id.
func Network(uuid, id, name string) Object {
	return Object{
		"kind":     "network",
		"metadata": metadata(uuid, name),
		"spec":     Object{"name": name, "driver": "bridge"},
		"status": Object{
			"state":      "present",
			"id":         id,
			"name":       name,
			"driver":     "bridge",
			"scope":      "local",
			"created_at": Created().Format(time.RFC3339),
		},
	}
}

// Volume is a volume as a VM's listing answers one.
func Volume(uuid, name string) Object {
	return Object{
		"kind":     "volume",
		"metadata": metadata(uuid, name),
		"spec":     Object{"name": name, "driver": "local"},
		"status": Object{
			"state":      "present",
			"name":       name,
			"driver":     "local",
			"mountpoint": "/var/lib/docker/volumes/" + name + "/_data",
			"created_at": Created().Format(time.RFC3339),
		},
	}
}

// Stack is a stack as the API answers one, deployed into the VM vmUUID,
// called vmName, its one service web running a container.
func Stack(uuid, name, vmUUID, vmName string) Object {
	return Object{
		"kind":     "stack",
		"metadata": metadata(uuid, name).With("slug", name+"-a1b2"),
		"spec":     Object{"vm": Object{"uuid": vmUUID}, "compose": "services:\n  web:\n    image: nginx\n"},
		"status": Object{
			"state": "running",
			"services": []Object{{
				"name":       "web",
				"state":      "running",
				"containers": []Object{{"id": "c-web", "name": name + "-a1b2_web", "state": "running"}},
			}},
		},
		"vm_name": vmName,
	}
}

// Snapshot is a snapshot as the API answers one, taken of the VM vmUUID,
// called vmName.
func Snapshot(uuid, name, vmUUID, vmName string) Object {
	return Object{
		"kind":     "snapshot",
		"metadata": metadata(uuid, name),
		"spec":     Object{"vm": Object{"uuid": vmUUID, "name": vmName}},
		"status": Object{
			"state":  "ready",
			"image":  "ubuntu:24.04",
			"disk":   4 << 30,
			"engine": "microsandbox",
			"size":   300 << 20,
		},
		"docker": false,
	}
}

// Refused is what the API answers a request it refuses with, field by
// field.
func Refused(fields map[string]string) Object {
	errors := Object{}
	for field, message := range fields {
		errors[field] = message
	}

	return Object{"errors": errors}
}
