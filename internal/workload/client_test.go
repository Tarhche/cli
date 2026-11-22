package workload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const (
	mine     = "/api/dashboard/my/workload"
	everyone = "/api/dashboard/workload"
)

// clientOf is a client of audience asking fake.
func clientOf(t *testing.T, fake *workloadtest.API, audience Audience) *Client {
	t.Helper()

	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	client, err := api.New(server.URL)
	require.NoError(t, err)

	return NewClient(client, audience)
}

// route is one route of the client: what calling it sends, and what it
// makes of what it is answered.
type route struct {
	name     string
	audience Audience
	call     func(ctx context.Context, c *Client) (any, error)

	method string
	path   string
	query  url.Values
	body   string

	status int
	answer any

	check func(t *testing.T, got any)
}

// test calls each route against an API that answers it, and checks the
// request it sent and what it made of the answer.
func test(t *testing.T, routes []route) {
	t.Helper()

	for _, r := range routes {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()

			fake := workloadtest.New().On(r.method, r.path, r.status, r.answer)

			got, err := r.call(context.Background(), clientOf(t, fake, r.audience))
			require.NoError(t, err)

			sent, found := fake.Sent(r.method, r.path)
			require.True(t, found, "it was sent %v", fake.Routes())

			assert.Equal(t, valuesOf(r.query), valuesOf(sent.Query))

			if len(r.body) == 0 {
				assert.Empty(t, sent.Body)
			} else {
				assert.JSONEq(t, r.body, sent.Body)
			}

			if r.check != nil {
				r.check(t, got)
			}
		})
	}
}

func valuesOf(query url.Values) url.Values {
	if len(query) == 0 {
		return url.Values{}
	}

	return query
}

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("a client is of the audience it is given", func(t *testing.T) {
		t.Parallel()

		client, err := api.New("https://backend.example")
		require.NoError(t, err)

		assert.Equal(t, Mine, NewClient(client, Mine).Audience())
		assert.Equal(t, Everyone, NewClient(client, Everyone).Audience())
	})

	t.Run("what is not an answer is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/v1", http.StatusInternalServerError, `{"code":"internal"}`).
			On(http.MethodGet, mine+"/containers", http.StatusBadRequest, workloadtest.Refused(map[string]string{"vm": "not a docker vm"})).
			On(http.MethodPost, everyone+"/vms", http.StatusOK, `{`).
			On(http.MethodGet, mine+"/vms", http.StatusForbidden, `{"code":"forbidden"}`)

		c := clientOf(t, fake, Mine)
		ctx := context.Background()

		_, err := c.VM(ctx, "v1")
		assert.EqualError(t, err, "internal")

		_, err = c.Containers(ctx, "")
		assert.True(t, api.IsValidation(err))

		_, err = c.CreateVM(ctx, CreateVMRequest{})
		assert.ErrorContains(t, err, "reading the answer")

		_, err = c.VMs(ctx)
		assert.True(t, api.IsForbidden(err))
	})

	t.Run("every route the api refuses is an error", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, workloadtest.New(), Everyone)
		ctx := context.Background()

		calls := map[string]func() error{
			"DockerVMs":           func() error { _, err := c.DockerVMs(ctx); return err },
			"UpdateVM":            func() error { _, err := c.UpdateVM(ctx, "v1", UpdateVMRequest{}); return err },
			"DeleteVM":            func() error { _, err := c.DeleteVM(ctx, "v1", false); return err },
			"VMDependents":        func() error { _, err := c.VMDependents(ctx, "v1"); return err },
			"VMLogs":              func() error { _, err := c.VMLogs(ctx, "v1", LogsQuery{}); return err },
			"VMUsage":             func() error { _, err := c.VMUsage(ctx, "v1", Span{}); return err },
			"Container":           func() error { _, err := c.Container(ctx, "v1", "c1"); return err },
			"CreateContainer":     func() error { _, err := c.CreateContainer(ctx, CreateContainerRequest{}); return err },
			"ContainerLogs":       func() error { _, err := c.ContainerLogs(ctx, "v1", "c1", LogsQuery{}); return err },
			"ContainerStats":      func() error { _, err := c.ContainerStats(ctx, "v1", "c1"); return err },
			"ContainerUsage":      func() error { _, err := c.ContainerUsage(ctx, "v1", "c1", Span{}); return err },
			"PullImage":           func() error { _, err := c.PullImage(ctx, "v1", "nginx"); return err },
			"CreateNetwork":       func() error { _, err := c.CreateNetwork(ctx, "v1", CreateNetworkRequest{}); return err },
			"DeleteNetwork":       func() error { _, err := c.DeleteNetwork(ctx, "v1", "n1", false); return err },
			"NetworkDependents":   func() error { _, err := c.NetworkDependents(ctx, "v1", "n1"); return err },
			"CreateVolume":        func() error { _, err := c.CreateVolume(ctx, "v1", CreateVolumeRequest{}); return err },
			"DeleteVolume":        func() error { _, err := c.DeleteVolume(ctx, "v1", "data", false, false); return err },
			"VolumeDependents":    func() error { _, err := c.VolumeDependents(ctx, "v1", "data"); return err },
			"Stack":               func() error { _, err := c.Stack(ctx, "s1"); return err },
			"CreateStack":         func() error { _, err := c.CreateStack(ctx, CreateStackRequest{}); return err },
			"Snapshot":            func() error { _, err := c.Snapshot(ctx, "p1"); return err },
			"CreateSnapshot":      func() error { _, err := c.CreateSnapshot(ctx, "v1", "x"); return err },
			"RenameSnapshot":      func() error { _, err := c.RenameSnapshot(ctx, "p1", "x"); return err },
			"DeleteSnapshot":      func() error { return c.DeleteSnapshot(ctx, "p1") },
			"DisconnectContainer": func() error { return c.DisconnectContainer(ctx, "v1", "c1", "n") },
		}

		for name, call := range calls {
			assert.True(t, api.IsNotFound(call()), name)
		}
	})
}

func TestClientVMRoutes(t *testing.T) {
	t.Parallel()

	vm := workloadtest.VM("v1", "web", false, "running")
	removal := workloadtest.Object{"dependents": []workloadtest.Object{{"kind": "container", "uuid": "c1", "name": "nginx", "recreates": true}}, "recreated": true}
	from := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)

	test(t, []route{
		{
			name: "one's own vms are read page by page", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VMs(ctx) },
			method: http.MethodGet, path: mine + "/vms", query: url.Values{"page": {"1"}},
			status: http.StatusOK, answer: workloadtest.Page(vm),
			check: func(t *testing.T, got any) {
				vms := got.([]VMManifest)
				require.Len(t, vms, 1)
				assert.Equal(t, "web", vms[0].Metadata.Name)
			},
		},
		{
			name: "anybody's vms", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VMs(ctx) },
			method: http.MethodGet, path: everyone + "/vms", query: url.Values{"page": {"1"}},
			status: http.StatusOK, answer: workloadtest.Page(),
		},
		{
			name: "docker vms are the vms whose image makes them one", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.DockerVMs(ctx) },
			method: http.MethodGet, path: mine + "/vms", query: url.Values{"page": {"1"}},
			status: http.StatusOK, answer: workloadtest.Page(vm, workloadtest.VM("v2", "box", true, "running")),
			check: func(t *testing.T, got any) {
				vms := got.([]VMManifest)
				require.Len(t, vms, 1)
				assert.Equal(t, "v2", vms[0].Metadata.UUID)
			},
		},
		{
			name: "a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VM(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1",
			status: http.StatusOK, answer: vm,
			check: func(t *testing.T, got any) { assert.Equal(t, "v1", got.(VMManifest).Metadata.UUID) },
		},
		{
			name: "a vm is created on anybody's route, whoever's vms the client reaches", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateVM(ctx, CreateVMRequest{
					Name: "web", Kind: KindMachine, Image: "debian:12",
					Resources: Resources{CPUs: 2, Memory: 128 << 20, Disk: 1 << 30},
					Ports:     []uint{80}, Network: Network{Ingress: Allow, Egress: Deny},
					PersistentDisk: true, LifetimeSeconds: 3600, SnapshotUUID: "s1",
				})
			},
			method: http.MethodPost, path: everyone + "/vms",
			body:   `{"name":"web","kind":"machine","image":"debian:12","resources":{"cpus":2,"memory":134217728,"disk":1073741824},"ports":[80],"network":{"ingress":"allow","egress":"deny"},"persistent_disk":true,"lifetime_seconds":3600,"snapshot_uuid":"s1"}`,
			status: http.StatusCreated, answer: vm,
			check: func(t *testing.T, got any) { assert.Equal(t, "v1", got.(VMManifest).Metadata.UUID) },
		},
		{
			name: "a vm is changed only in what is asked", audience: Everyone,
			call: func(ctx context.Context, c *Client) (any, error) {
				name, lifetime, ports := "api", int64(0), []uint{}
				return c.UpdateVM(ctx, "v1", UpdateVMRequest{Name: &name, LifetimeSeconds: &lifetime, Ports: &ports})
			},
			method: http.MethodPatch, path: everyone + "/vms/v1",
			body:   `{"name":"api","lifetime_seconds":0,"ports":[]}`,
			status: http.StatusOK, answer: vm,
			check: func(t *testing.T, got any) { assert.Equal(t, "web", got.(VMManifest).Metadata.Name) },
		},
		{
			name: "a vm is deleted, saying what needs it", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.DeleteVM(ctx, "v1", false) },
			method: http.MethodDelete, path: mine + "/vms/v1",
			status: http.StatusAccepted, answer: removal,
			check: func(t *testing.T, got any) {
				assert.Equal(t, Removal{Dependents: []Dependent{{Kind: "container", UUID: "c1", Name: "nginx", Recreates: true}}, Recreated: true}, got)
			},
		},
		{
			name: "a vm is deleted with what needs it", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.DeleteVM(ctx, "v1", true) },
			method: http.MethodDelete, path: mine + "/vms/v1", query: url.Values{"cascade": {"true"}},
			status: http.StatusAccepted, answer: nil,
			check: func(t *testing.T, got any) { assert.Equal(t, Removal{}, got) },
		},
		{
			name: "what needs a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VMDependents(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1/dependents",
			status: http.StatusOK, answer: removal,
			check: func(t *testing.T, got any) { assert.True(t, got.(Removal).Recreated) },
		},
		{
			name: "a vm is started", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StartVM(ctx, "v1") },
			method: http.MethodPost, path: mine + "/vms/v1/start", status: http.StatusAccepted,
		},
		{
			name: "a vm is stopped", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StopVM(ctx, "v1") },
			method: http.MethodPost, path: everyone + "/vms/v1/stop", status: http.StatusAccepted,
		},
		{
			name: "a vm is restarted", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.RestartVM(ctx, "v1") },
			method: http.MethodPost, path: mine + "/vms/v1/restart", status: http.StatusAccepted,
		},
		{
			name: "a vm is restored from a snapshot", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.RestoreVM(ctx, "v1", "s1") },
			method: http.MethodPost, path: mine + "/vms/v1/restore", body: `{"snapshot_uuid":"s1"}`, status: http.StatusAccepted,
		},
		{
			name: "what a vm wrote", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.VMLogs(ctx, "v1", LogsQuery{From: from, To: to, Tail: 10})
			},
			method: http.MethodGet, path: mine + "/vms/v1/logs",
			query:  url.Values{"from": {"2026-11-22T09:00:00Z"}, "to": {"2026-11-22T10:00:00Z"}, "tail": {"10"}},
			status: http.StatusOK, answer: `{"items":[{"at":"2026-11-22T09:30:00Z","source":"main","line":"hello"}],"truncated":true}`,
			check: func(t *testing.T, got any) {
				assert.Equal(t, Logs{Items: []LogLine{{At: from.Add(30 * time.Minute), Source: "main", Line: "hello"}}, Truncated: true}, got)
			},
		},
		{
			name: "what a vm used", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VMUsage(ctx, "v1", Span{}) },
			method: http.MethodGet, path: mine + "/vms/v1/usage",
			status: http.StatusOK, answer: `{"from":"2026-11-22T09:00:00Z","to":"2026-11-22T10:00:00Z","step":10,"points":[{"at":"2026-11-22T09:00:10Z","cpu_percent":12.5,"memory_used":1024,"memory_limit":2048,"disk_used":5,"disk_total":10,"network_rx":1,"network_tx":2}]}`,
			check: func(t *testing.T, got any) {
				usage := got.(Usage)
				assert.Equal(t, int64(10), usage.Step)
				assert.Equal(t, []UsagePoint{{At: from.Add(10 * time.Second), CPUPercent: 12.5, MemoryUsed: 1024, MemoryLimit: 2048, DiskUsed: 5, DiskTotal: 10, NetworkRx: 1, NetworkTx: 2}}, usage.Points)
			},
		},
	})
}

func TestClientContainerRoutes(t *testing.T) {
	t.Parallel()

	container := workloadtest.Container("c1", "abcdef1234567890", "nginx", "running", "v1", "web")

	test(t, []route{
		{
			name: "the containers across one's docker vms", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Containers(ctx, "") },
			method: http.MethodGet, path: mine + "/containers",
			status: http.StatusOK, answer: workloadtest.Items(container),
			check: func(t *testing.T, got any) {
				containers := got.([]ContainerManifest)
				require.Len(t, containers, 1)
				assert.Equal(t, "web", containers[0].VMName)
			},
		},
		{
			name: "the containers of one docker vm, across anybody's", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Containers(ctx, "v1") },
			method: http.MethodGet, path: everyone + "/containers", query: url.Values{"vm": {"v1"}},
			status: http.StatusOK, answer: workloadtest.Items(),
		},
		{
			name: "the containers of a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VMContainers(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1/containers",
			status: http.StatusOK, answer: workloadtest.Items(container),
			check: func(t *testing.T, got any) { assert.Len(t, got, 1) },
		},
		{
			name: "a container", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Container(ctx, "v1", "nginx") },
			method: http.MethodGet, path: mine + "/vms/v1/containers/nginx",
			status: http.StatusOK, answer: container,
			check: func(t *testing.T, got any) { assert.Equal(t, "abcdef1234567890", got.(ContainerManifest).Status.ID) },
		},
		{
			name: "a container is created on anybody's route", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateContainer(ctx, CreateContainerRequest{
					VM:   &NewDockerVM{Name: "box", Resources: &Resources{Memory: 1 << 30}, Ports: []uint{80}, Network: &Network{Egress: Deny}},
					Name: "nginx", Image: "nginx", Command: []string{"nginx", "-g", "daemon off;"}, Entrypoint: []string{"/bin/sh"},
					Env: []string{"A=b"}, WorkingDir: "/srv",
					Ports:    []PortBinding{{ContainerPort: 80, HostPort: 8080, HostIP: "127.0.0.1", Protocol: "udp"}},
					Mounts:   []Mount{{Type: MountVolume, Source: "data", Target: "/data", ReadOnly: true}},
					Networks: []string{"backend"}, RestartPolicy: "always", CPUs: 0.5, Memory: 64 << 20, PidsLimit: 100,
					Privileged: true, User: "1000:1000", Runtime: "runc",
				})
			},
			method: http.MethodPost, path: everyone + "/containers",
			body: `{"vm":{"name":"box","resources":{"cpus":0,"memory":1073741824,"disk":0},"ports":[80],"network":{"egress":"deny"}},
				"name":"nginx","image":"nginx","command":["nginx","-g","daemon off;"],"entrypoint":["/bin/sh"],"env":["A=b"],"working_dir":"/srv",
				"ports":[{"container_port":80,"host_port":8080,"host_ip":"127.0.0.1","protocol":"udp"}],
				"mounts":[{"type":"volume","source":"data","target":"/data","read_only":true}],
				"networks":["backend"],"restart_policy":"always","cpus":0.5,"memory":67108864,"pids_limit":100,
				"privileged":true,"user":"1000:1000","runtime":"runc"}`,
			status: http.StatusCreated, answer: workloadtest.Object{"vm": workloadtest.Object{"uuid": "v9", "name": "box", "created": true}, "container": container},
			check: func(t *testing.T, got any) {
				created := got.(CreatedContainer)
				assert.Equal(t, ChosenVM{UUID: "v9", Name: "box", Created: true}, created.VM)
				assert.Equal(t, "c1", created.Container.Metadata.UUID)
			},
		},
		{
			name: "a container is created in a vm named by its uuid", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateContainer(ctx, CreateContainerRequest{VMUUID: "v1", Image: "nginx"})
			},
			method: http.MethodPost, path: everyone + "/containers", body: `{"vm_uuid":"v1","image":"nginx"}`,
			status: http.StatusCreated, answer: workloadtest.Object{"vm": workloadtest.Object{"uuid": "v1", "name": "web"}, "container": container},
		},
		{
			name: "a container is removed", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return nil, c.DeleteContainer(ctx, "v1", "c1", false)
			},
			method: http.MethodDelete, path: mine + "/vms/v1/containers/c1", status: http.StatusNoContent,
		},
		{
			name: "a running container is removed by force", audience: Everyone,
			call: func(ctx context.Context, c *Client) (any, error) {
				return nil, c.DeleteContainer(ctx, "v1", "c1", true)
			},
			method: http.MethodDelete, path: everyone + "/vms/v1/containers/c1", query: url.Values{"force": {"true"}}, status: http.StatusNoContent,
		},
		{
			name: "a container is started", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StartContainer(ctx, "v1", "c1") },
			method: http.MethodPost, path: mine + "/vms/v1/containers/c1/start", status: http.StatusNoContent,
		},
		{
			name: "a container is stopped", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StopContainer(ctx, "v1", "c1") },
			method: http.MethodPost, path: mine + "/vms/v1/containers/c1/stop", status: http.StatusNoContent,
		},
		{
			name: "a container is restarted", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.RestartContainer(ctx, "v1", "c1") },
			method: http.MethodPost, path: mine + "/vms/v1/containers/c1/restart", status: http.StatusNoContent,
		},
		{
			name: "what a container wrote", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ContainerLogs(ctx, "v1", "c1", LogsQuery{Tail: 5})
			},
			method: http.MethodGet, path: mine + "/vms/v1/containers/c1/logs", query: url.Values{"tail": {"5"}},
			status: http.StatusOK, answer: `{"items":[{"at":"2026-11-22T09:30:00Z","stream":"stderr","line":"oops"}],"truncated":false}`,
			check: func(t *testing.T, got any) {
				assert.Equal(t, []LogLine{{At: time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC), Stream: Stderr, Line: "oops"}}, got.(Logs).Items)
			},
		},
		{
			name: "what a container uses now", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.ContainerStats(ctx, "v1", "c1") },
			method: http.MethodGet, path: mine + "/vms/v1/containers/c1/stats",
			status: http.StatusOK, answer: `{"cpu_percent":1.5,"memory_used":10,"memory_limit":20,"network_rx":1,"network_tx":2,"block_read":3,"block_write":4,"pids":5,"sampled_at":"2026-11-22T09:30:00Z"}`,
			check: func(t *testing.T, got any) {
				assert.Equal(t, ContainerStats{CPUPercent: 1.5, MemoryUsed: 10, MemoryLimit: 20, NetworkRx: 1, NetworkTx: 2, BlockRead: 3, BlockWrite: 4, PIDs: 5, SampledAt: time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC)}, got)
			},
		},
		{
			name: "what a container used", audience: Everyone,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ContainerUsage(ctx, "v1", "c1", Span{From: time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)})
			},
			method: http.MethodGet, path: everyone + "/vms/v1/containers/c1/usage", query: url.Values{"from": {"2026-11-22T09:00:00Z"}},
			status: http.StatusOK, answer: `{"step":60,"points":[]}`,
			check: func(t *testing.T, got any) { assert.Equal(t, int64(60), got.(Usage).Step) },
		},
		{
			name: "a container is put on a network", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return nil, c.ConnectContainer(ctx, "v1", "c1", "backend", []string{"api"})
			},
			method: http.MethodPost, path: mine + "/vms/v1/containers/c1/networks", body: `{"network":"backend","aliases":["api"]}`, status: http.StatusNoContent,
		},
		{
			name: "a container is taken off a network", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return nil, c.DisconnectContainer(ctx, "v1", "c1", "backend")
			},
			method: http.MethodDelete, path: mine + "/vms/v1/containers/c1/networks/backend", status: http.StatusNoContent,
		},
	})
}

func TestClientBuildingBlockRoutes(t *testing.T) {
	t.Parallel()

	removal := workloadtest.Object{"dependents": []workloadtest.Object{{"kind": "stack", "uuid": "s1", "recreates": false}}, "recreated": false}

	test(t, []route{
		{
			name: "the images of a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Images(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1/images",
			status: http.StatusOK, answer: workloadtest.Items(workloadtest.Image("i1", "sha256:0123456789abcdef", "nginx:1.27")),
			check: func(t *testing.T, got any) { assert.Equal(t, "nginx:1.27", got.([]ImageManifest)[0].Reference()) },
		},
		{
			name: "an image is pulled", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.PullImage(ctx, "v1", "nginx") },
			method: http.MethodPost, path: mine + "/vms/v1/images", body: `{"reference":"nginx"}`,
			status: http.StatusCreated, answer: workloadtest.Image("i1", "sha256:0123456789abcdef", "nginx:latest"),
			check: func(t *testing.T, got any) { assert.Equal(t, "i1", got.(ImageManifest).Metadata.UUID) },
		},
		{
			name: "an image is removed by force", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.DeleteImage(ctx, "v1", "i1", true) },
			method: http.MethodDelete, path: everyone + "/vms/v1/images/i1", query: url.Values{"force": {"true"}}, status: http.StatusNoContent,
		},
		{
			name: "the networks of a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Networks(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1/networks",
			status: http.StatusOK, answer: workloadtest.Items(workloadtest.Network("n1", "f00d", "backend")),
			check: func(t *testing.T, got any) { assert.Equal(t, "backend", got.([]NetworkManifest)[0].DockerName()) },
		},
		{
			name: "a network is created", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateNetwork(ctx, "v1", CreateNetworkRequest{Name: "backend", Driver: "bridge", Internal: true, Labels: map[string]string{"a": "b"}})
			},
			method: http.MethodPost, path: mine + "/vms/v1/networks", body: `{"name":"backend","driver":"bridge","internal":true,"labels":{"a":"b"}}`,
			status: http.StatusCreated, answer: workloadtest.Network("n1", "f00d", "backend"),
			check: func(t *testing.T, got any) { assert.Equal(t, "f00d", got.(NetworkManifest).Status.ID) },
		},
		{
			name: "a network is removed, saying what needs it", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.DeleteNetwork(ctx, "v1", "n1", true) },
			method: http.MethodDelete, path: mine + "/vms/v1/networks/n1", query: url.Values{"cascade": {"true"}},
			status: http.StatusAccepted, answer: removal,
			check: func(t *testing.T, got any) { assert.Len(t, got.(Removal).Dependents, 1) },
		},
		{
			name: "what needs a network", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.NetworkDependents(ctx, "v1", "n1") },
			method: http.MethodGet, path: mine + "/vms/v1/networks/n1/dependents",
			status: http.StatusOK, answer: removal,
		},
		{
			name: "the volumes of a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Volumes(ctx, "v1") },
			method: http.MethodGet, path: mine + "/vms/v1/volumes",
			status: http.StatusOK, answer: workloadtest.Items(workloadtest.Volume("o1", "data")),
			check: func(t *testing.T, got any) { assert.Equal(t, "data", got.([]VolumeManifest)[0].DockerName()) },
		},
		{
			name: "a volume is created", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateVolume(ctx, "v1", CreateVolumeRequest{Name: "data"})
			},
			method: http.MethodPost, path: mine + "/vms/v1/volumes", body: `{"name":"data"}`,
			status: http.StatusCreated, answer: workloadtest.Volume("o1", "data"),
		},
		{
			name: "a volume is removed by force and with what needs it", audience: Everyone,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.DeleteVolume(ctx, "v1", "data", true, true)
			},
			method: http.MethodDelete, path: everyone + "/vms/v1/volumes/data", query: url.Values{"force": {"true"}, "cascade": {"true"}},
			status: http.StatusNoContent,
		},
		{
			name: "what needs a volume", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.VolumeDependents(ctx, "v1", "data") },
			method: http.MethodGet, path: mine + "/vms/v1/volumes/data/dependents",
			status: http.StatusOK, answer: removal,
		},
	})
}

func TestClientStackAndSnapshotRoutes(t *testing.T) {
	t.Parallel()

	stack := workloadtest.Stack("s1", "blog", "v1", "web")
	snapshot := workloadtest.Snapshot("p1", "before", "v1", "web")

	test(t, []route{
		{
			name: "the stacks of a vm", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Stacks(ctx, "v1") },
			method: http.MethodGet, path: mine + "/stacks", query: url.Values{"vm": {"v1"}, "page": {"1"}},
			status: http.StatusOK, answer: workloadtest.Page(stack),
			check: func(t *testing.T, got any) { assert.Equal(t, "web", got.([]StackManifest)[0].VMName) },
		},
		{
			name: "a stack", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Stack(ctx, "s1") },
			method: http.MethodGet, path: everyone + "/stacks/s1",
			status: http.StatusOK, answer: stack,
			check: func(t *testing.T, got any) {
				assert.Equal(t, []Service{{Name: "web", State: "running", Containers: []ServiceContainer{{ID: "c-web", Name: "blog-a1b2_web", State: "running"}}}}, got.(StackManifest).Status.Services)
			},
		},
		{
			name: "a stack is deployed on anybody's route", audience: Mine,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateStack(ctx, CreateStackRequest{Name: "blog", Compose: "services: {}", VMUUID: "v1"})
			},
			method: http.MethodPost, path: everyone + "/stacks", body: `{"name":"blog","compose":"services: {}","vm_uuid":"v1"}`,
			status: http.StatusCreated, answer: workloadtest.Object{"vm": workloadtest.Object{"uuid": "v1", "name": "web"}, "stack": stack},
			check: func(t *testing.T, got any) { assert.Equal(t, "s1", got.(CreatedStack).Stack.Metadata.UUID) },
		},
		{
			name: "a stack is removed", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.DeleteStack(ctx, "s1") },
			method: http.MethodDelete, path: mine + "/stacks/s1", status: http.StatusAccepted,
		},
		{
			name: "a stack is started", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StartStack(ctx, "s1") },
			method: http.MethodPost, path: mine + "/stacks/s1/start", status: http.StatusAccepted,
		},
		{
			name: "a stack is stopped", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.StopStack(ctx, "s1") },
			method: http.MethodPost, path: mine + "/stacks/s1/stop", status: http.StatusAccepted,
		},
		{
			name: "a stack is restarted", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.RestartStack(ctx, "s1") },
			method: http.MethodPost, path: mine + "/stacks/s1/restart", status: http.StatusAccepted,
		},
		{
			name: "every snapshot", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Snapshots(ctx, "") },
			method: http.MethodGet, path: mine + "/snapshots", query: url.Values{"page": {"1"}},
			status: http.StatusOK, answer: workloadtest.Page(snapshot),
			check: func(t *testing.T, got any) { assert.Equal(t, uint64(4<<30), got.([]SnapshotManifest)[0].Status.Disk) },
		},
		{
			name: "a snapshot", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.Snapshot(ctx, "p1") },
			method: http.MethodGet, path: mine + "/snapshots/p1",
			status: http.StatusOK, answer: snapshot,
			check: func(t *testing.T, got any) { assert.Equal(t, "web", got.(SnapshotManifest).Spec.VM.Name) },
		},
		{
			name: "a snapshot is taken on anybody's route", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.CreateSnapshot(ctx, "v1", "before") },
			method: http.MethodPost, path: everyone + "/vms/v1/snapshots", body: `{"name":"before"}`,
			status: http.StatusCreated, answer: snapshot,
		},
		{
			name: "a snapshot is renamed", audience: Everyone,
			call:   func(ctx context.Context, c *Client) (any, error) { return c.RenameSnapshot(ctx, "p1", "after") },
			method: http.MethodPatch, path: everyone + "/snapshots/p1", body: `{"name":"after"}`,
			status: http.StatusOK, answer: snapshot,
		},
		{
			name: "a snapshot is removed", audience: Mine,
			call:   func(ctx context.Context, c *Client) (any, error) { return nil, c.DeleteSnapshot(ctx, "p1") },
			method: http.MethodDelete, path: mine + "/snapshots/p1", status: http.StatusNoContent,
		},
	})
}

func TestManifests(t *testing.T) {
	t.Parallel()

	t.Run("a vm is read whole, and kept as it was answered", func(t *testing.T) {
		t.Parallel()

		answered := `{"kind":"vm","metadata":{"uuid":"v1","name":"web","slug":"web-x","owner_uuid":"u1","labels":{"a":"b"},"owners":[{"kind":"stack","uuid":"s1"}],"refs":[{"kind":"vm","uuid":"v0"}],"node":"node-1","lifetime":3600000000000,"created_at":"2026-11-22T09:00:00Z"},
			"spec":{"image":"ubuntu:24.04","resources":{"cpus":2,"memory":536870912,"disk":4294967296},"ports":[80,443],"network":{"ingress":"allow","egress":"deny"},"persistent_disk":true,"source":{"snapshot":"p1"}},
			"status":{"state":"running","expected":"running","reason":"started","since":"2026-11-22T09:01:00Z","stats":{"cpu_percent":3.5,"memory_used":1,"memory_limit":2,"disk_used":3,"disk_total":4,"network_rx":5,"network_tx":6,"sampled_at":"2026-11-22T09:02:00Z"},"started_at":"2026-11-22T09:01:00Z"},
			"docker":false,"owner":{"uuid":"u1","name":"Mahdi","username":"mahdi"},"urls":[{"port":80,"url":"https://web-x-80.example"}],"future":"kept"}`

		var vm VMManifest
		require.NoError(t, json.Unmarshal([]byte(answered), &vm))

		assert.Equal(t, time.Hour, vm.Metadata.Lifetime)
		assert.Equal(t, []Reference{{Kind: "stack", UUID: "s1"}}, vm.Metadata.Owners)
		assert.Equal(t, Resources{CPUs: 2, Memory: 512 << 20, Disk: 4 << 30}, vm.Spec.Resources)
		assert.Equal(t, &VMSource{Snapshot: "p1"}, vm.Spec.Source)
		assert.Equal(t, "started", vm.Status.Reason)
		assert.Equal(t, 3.5, vm.Status.Stats.CPUPercent)
		assert.Equal(t, "mahdi", vm.Owner.Username)
		assert.Equal(t, []URL{{Port: 80, URL: "https://web-x-80.example"}}, vm.URLs)
		assert.JSONEq(t, answered, string(vm.Raw()), "what the client does not know of is kept too")
		assert.Equal(t, VM{UUID: "v1", Name: "web", Slug: "web-x", State: "running", Ports: []uint{80, 443}}, vm.Reach())
	})

	t.Run("a container is read with what docker said of it within its status", func(t *testing.T) {
		t.Parallel()

		var container ContainerManifest
		require.NoError(t, json.Unmarshal(mustJSON(t, workloadtest.Container("c1", "abc", "nginx", "running", "v1", "web").
			With("status.failure", workloadtest.Object{"code": "conflict", "message": "in use"}).
			With("status.stats", workloadtest.Object{"cpu_percent": 2, "pids": 3})), &container))

		assert.Equal(t, "abc", container.Status.ID)
		assert.Equal(t, "Up 2 hours", container.Status.DockerStatus)
		assert.Equal(t, "running", container.Status.State)
		assert.Equal(t, []PortBinding{{ContainerPort: 80, HostPort: 8080, HostIP: "0.0.0.0", Protocol: "tcp"}}, container.Status.Ports)
		assert.Equal(t, &Failure{Code: "conflict", Message: "in use"}, container.Status.Failure)
		assert.Equal(t, uint64(3), container.Status.Stats.PIDs)
		assert.True(t, container.Running())
		assert.NotEmpty(t, container.Raw())

		vm := VM{UUID: "v1", Name: "web", Docker: true}
		assert.Equal(t, Container{UUID: "c1", DockerID: "abc", Name: "nginx", State: "running", VM: vm}, container.Reach(vm))
	})

	t.Run("a container is called what docker calls it, else what it was asked to be called", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "web", ContainerManifest{Status: ContainerStatus{ContainerDocker: ContainerDocker{Name: "/web"}}, Spec: ContainerSpec{Name: "asked"}}.DockerName())
		assert.Equal(t, "asked", ContainerManifest{Spec: ContainerSpec{Name: "asked"}, Metadata: Metadata{Name: "meta"}}.DockerName())
		assert.Equal(t, "meta", ContainerManifest{Metadata: Metadata{Name: "meta"}}.DockerName())
	})

	t.Run("a container runs as docker lists it without -a", func(t *testing.T) {
		t.Parallel()

		for state, running := range map[string]bool{"running": true, "starting": true, "restarting": true, "stopping": true, "stopped": false, "completed": false, "pending": false, "failed": false} {
			assert.Equal(t, running, ContainerManifest{Status: ContainerStatus{Status: Status{State: state}}}.Running(), state)
		}
	})

	t.Run("an image is known by every reference it is held under", func(t *testing.T) {
		t.Parallel()

		image := ImageManifest{Spec: ImageSpec{Reference: "nginx"}, Status: ImageStatus{ImageDocker: ImageDocker{Reference: "docker.io/library/nginx:latest", Tags: []string{"nginx:1.27", "<none>:<none>"}}}}

		assert.Equal(t, "docker.io/library/nginx:latest", image.Reference())
		assert.Equal(t, []string{"nginx:latest", "nginx:latest", "nginx:1.27"}, image.References())
		assert.Equal(t, "nginx", ImageManifest{Spec: ImageSpec{Reference: "nginx"}}.Reference())
	})

	t.Run("networks and volumes are called what docker calls them", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "seen", NetworkManifest{Status: NetworkStatus{NetworkDocker: NetworkDocker{Name: "seen"}}, Spec: NetworkSpec{Name: "asked"}}.DockerName())
		assert.Equal(t, "asked", NetworkManifest{Spec: NetworkSpec{Name: "asked"}}.DockerName())
		assert.Equal(t, "seen", VolumeManifest{Status: VolumeStatus{VolumeDocker: VolumeDocker{Name: "seen"}}}.DockerName())
		assert.Equal(t, "meta", VolumeManifest{Metadata: Metadata{Name: "meta"}}.DockerName())
	})

	t.Run("what is not json is not read", func(t *testing.T) {
		t.Parallel()

		for _, into := range []json.Unmarshaler{&VMManifest{}, &ContainerManifest{}, &ImageManifest{}, &NetworkManifest{}, &VolumeManifest{}, &StackManifest{}, &SnapshotManifest{}} {
			assert.Error(t, into.UnmarshalJSON([]byte(`{"kind":`)))
		}

		assert.Nil(t, VMManifest{}.Raw(), "a manifest that was not read has nothing kept")
	})
}

func TestNormalizedReference(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"nginx":                           "nginx:latest",
		"nginx:1.27":                      "nginx:1.27",
		"docker.io/library/nginx:latest":  "nginx:latest",
		"index.docker.io/library/nginx":   "nginx:latest",
		"registry-1.docker.io/acme/app:2": "acme/app:2",
		"library/redis":                   "redis:latest",
		"localhost:5000/app":              "localhost:5000/app:latest",
		"localhost:5000/app:1":            "localhost:5000/app:1",
		"ghcr.io/acme/app@sha256:abc":     "ghcr.io/acme/app@sha256:abc",
		" docker.io/nginx@sha256:def ":    "nginx@sha256:def",
	}

	for reference, want := range cases {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, NormalizedReference(reference))
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	content, err := json.Marshal(value)
	require.NoError(t, err)

	return content
}
