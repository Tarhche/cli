package desktop_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/desktop"
)

func TestWorkloads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// answering calls a method of the service and is what it answered.
	type answering func(w *desktop.Workloads) (json.RawMessage, error)

	// acting calls a method of the service that answers nothing.
	acting := func(act func(w *desktop.Workloads) error) answering {
		return func(w *desktop.Workloads) (json.RawMessage, error) {
			return nil, act(w)
		}
	}

	cases := []struct {
		name   string
		call   answering
		method string
		path   string
		query  string
		body   string
	}{
		{
			name:   "a page of one's VMs",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.VMs(ctx, 2) },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms", query: "page=2",
		},
		{
			name:   "a VM",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.VM(ctx, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1",
		},
		{
			name: "a VM created",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.CreateVM(ctx, desktop.VMRequest{
					Name:      "box",
					Kind:      "machine",
					Image:     "ubuntu:24.04",
					Resources: desktop.VMResources{CPUs: 1, Memory: 128 << 20, Disk: 1 << 30},
					Ports:     []uint{8080},
					Network:   desktop.VMNetwork{Ingress: "allow", Egress: "deny"},
				})
			},
			method: http.MethodPost, path: "/api/dashboard/workload/vms",
			body: `{"name":"box","kind":"machine","image":"ubuntu:24.04","resources":{"cpus":1,"memory":134217728,"disk":1073741824},"ports":[8080],"network":{"ingress":"allow","egress":"deny"},"persistent_disk":false,"lifetime_seconds":0}`,
		},
		{
			name:   "a VM started",
			call:   acting(func(w *desktop.Workloads) error { return w.StartVM(ctx, "v-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/start",
		},
		{
			name:   "a VM stopped",
			call:   acting(func(w *desktop.Workloads) error { return w.StopVM(ctx, "v-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/stop",
		},
		{
			name:   "a VM restarted",
			call:   acting(func(w *desktop.Workloads) error { return w.RestartVM(ctx, "v-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/restart",
		},
		{
			name:   "a VM restored",
			call:   acting(func(w *desktop.Workloads) error { return w.RestoreVM(ctx, "v-1", "s-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/restore", body: `{"snapshot_uuid":"s-1"}`,
		},
		{
			name:   "a VM deleted with what needs it",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.DeleteVM(ctx, "v-1", true) },
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1", query: "cascade=true",
		},
		{
			name:   "a VM deleted alone",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.DeleteVM(ctx, "v-1", false) },
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1",
		},
		{
			name:   "what needs a VM",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.VMDependents(ctx, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/dependents",
		},
		{
			name: "a VM's log",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.VMLogs(ctx, "v-1", desktop.LogsQuery{From: "2026-11-22T09:00:00Z", To: "2026-11-22T10:00:00Z", Tail: 100})
			},
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/logs", query: "from=2026-11-22T09%3A00%3A00Z&tail=100&to=2026-11-22T10%3A00%3A00Z",
		},
		{
			name: "a VM's usage",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.VMUsage(ctx, "v-1", desktop.UsageQuery{From: "2026-11-22T09:00:00Z"})
			},
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/usage", query: "from=2026-11-22T09%3A00%3A00Z",
		},
		{
			name:   "a page of one's snapshots of a VM",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Snapshots(ctx, 1, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/snapshots", query: "page=1&vm=v-1",
		},
		{
			name:   "a snapshot",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Snapshot(ctx, "s-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/snapshots/s-1",
		},
		{
			name:   "a snapshot taken",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.CreateSnapshot(ctx, "v-1", "before") },
			method: http.MethodPost, path: "/api/dashboard/workload/vms/v-1/snapshots", body: `{"name":"before"}`,
		},
		{
			name:   "a snapshot renamed",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.RenameSnapshot(ctx, "s-1", "after") },
			method: http.MethodPatch, path: "/api/dashboard/my/workload/snapshots/s-1", body: `{"name":"after"}`,
		},
		{
			name:   "a snapshot deleted",
			call:   acting(func(w *desktop.Workloads) error { return w.DeleteSnapshot(ctx, "s-1") }),
			method: http.MethodDelete, path: "/api/dashboard/my/workload/snapshots/s-1",
		},
		{
			name:   "one's containers",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Containers(ctx, "") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/containers",
		},
		{
			name:   "a container",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Container(ctx, "v-1", "web") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/containers/web",
		},
		{
			name: "a container run",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.RunContainer(ctx, desktop.ContainerRequest{
					VM:            &desktop.NewDockerVM{Name: "docker"},
					Image:         "nginx",
					Env:           []string{"A=b"},
					Ports:         []desktop.PortBinding{{ContainerPort: 80, HostPort: 8080}},
					RestartPolicy: "unless-stopped",
				})
			},
			method: http.MethodPost, path: "/api/dashboard/workload/containers",
			body: `{"vm":{"name":"docker"},"image":"nginx","env":["A=b"],"ports":[{"container_port":80,"host_port":8080}],"restart_policy":"unless-stopped"}`,
		},
		{
			name:   "a container started",
			call:   acting(func(w *desktop.Workloads) error { return w.StartContainer(ctx, "v-1", "web") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/containers/web/start",
		},
		{
			name:   "a container stopped",
			call:   acting(func(w *desktop.Workloads) error { return w.StopContainer(ctx, "v-1", "web") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/containers/web/stop",
		},
		{
			name:   "a container restarted",
			call:   acting(func(w *desktop.Workloads) error { return w.RestartContainer(ctx, "v-1", "web") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/containers/web/restart",
		},
		{
			name:   "a container deleted by force",
			call:   acting(func(w *desktop.Workloads) error { return w.DeleteContainer(ctx, "v-1", "web", true) }),
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1/containers/web", query: "force=true",
		},
		{
			name: "a container's log",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.ContainerLogs(ctx, "v-1", "web", desktop.LogsQuery{})
			},
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/containers/web/logs",
		},
		{
			name:   "a container's stats",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.ContainerStats(ctx, "v-1", "web") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/containers/web/stats",
		},
		{
			name: "a container's usage",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.ContainerUsage(ctx, "v-1", "web", desktop.UsageQuery{To: "now"})
			},
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/containers/web/usage", query: "to=now",
		},
		{
			name:   "a Docker VM's images",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Images(ctx, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/images",
		},
		{
			name:   "an image pulled",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.PullImage(ctx, "v-1", "nginx:1.27") },
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/images", body: `{"reference":"nginx:1.27"}`,
		},
		{
			name:   "an image deleted, escaped",
			call:   acting(func(w *desktop.Workloads) error { return w.DeleteImage(ctx, "v-1", "my image", false) }),
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1/images/my%20image",
		},
		{
			name:   "a Docker VM's networks",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Networks(ctx, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/networks",
		},
		{
			name: "a network created",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.CreateNetwork(ctx, "v-1", desktop.NetworkRequest{Name: "back", Internal: true})
			},
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/networks", body: `{"name":"back","internal":true}`,
		},
		{
			name:   "a network deleted with what needs it",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.DeleteNetwork(ctx, "v-1", "back", true) },
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1/networks/back", query: "cascade=true",
		},
		{
			name:   "what needs a network",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.NetworkDependents(ctx, "v-1", "back") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/networks/back/dependents",
		},
		{
			name:   "a Docker VM's volumes",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Volumes(ctx, "v-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/volumes",
		},
		{
			name: "a volume created",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.CreateVolume(ctx, "v-1", desktop.VolumeRequest{Name: "data"})
			},
			method: http.MethodPost, path: "/api/dashboard/my/workload/vms/v-1/volumes", body: `{"name":"data"}`,
		},
		{
			name: "a volume deleted with what needs it, by force",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.DeleteVolume(ctx, "v-1", "data", true, true)
			},
			method: http.MethodDelete, path: "/api/dashboard/my/workload/vms/v-1/volumes/data", query: "cascade=true&force=true",
		},
		{
			name:   "what needs a volume",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.VolumeDependents(ctx, "v-1", "data") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/vms/v-1/volumes/data/dependents",
		},
		{
			name:   "a page of one's stacks",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Stacks(ctx, 1) },
			method: http.MethodGet, path: "/api/dashboard/my/workload/stacks", query: "page=1",
		},
		{
			name:   "a stack",
			call:   func(w *desktop.Workloads) (json.RawMessage, error) { return w.Stack(ctx, "k-1") },
			method: http.MethodGet, path: "/api/dashboard/my/workload/stacks/k-1",
		},
		{
			name: "a stack deployed",
			call: func(w *desktop.Workloads) (json.RawMessage, error) {
				return w.DeployStack(ctx, desktop.StackRequest{Name: "site", Compose: "services: {}", VMUUID: "v-1"})
			},
			method: http.MethodPost, path: "/api/dashboard/workload/stacks", body: `{"name":"site","compose":"services: {}","vm_uuid":"v-1"}`,
		},
		{
			name:   "a stack started",
			call:   acting(func(w *desktop.Workloads) error { return w.StartStack(ctx, "k-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/stacks/k-1/start",
		},
		{
			name:   "a stack stopped",
			call:   acting(func(w *desktop.Workloads) error { return w.StopStack(ctx, "k-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/stacks/k-1/stop",
		},
		{
			name:   "a stack restarted",
			call:   acting(func(w *desktop.Workloads) error { return w.RestartStack(ctx, "k-1") }),
			method: http.MethodPost, path: "/api/dashboard/my/workload/stacks/k-1/restart",
		},
		{
			name:   "a stack deleted",
			call:   acting(func(w *desktop.Workloads) error { return w.DeleteStack(ctx, "k-1") }),
			method: http.MethodDelete, path: "/api/dashboard/my/workload/stacks/k-1",
		},
	}

	for _, c := range cases {
		t.Run(c.name+" is asked of its route", func(t *testing.T) {
			t.Parallel()

			recorded := newRecorder(http.StatusOK, `{"kind":"vm"}`)
			w, _ := newWorkspace(t, recorded)

			_, err := c.call(desktop.NewWorkloads(w))

			require.NoError(t, err)

			asked := recorded.last(t)
			assert.Equal(t, c.method, asked.Method)
			assert.Equal(t, c.path, asked.Path)
			assert.Equal(t, c.query, asked.Query)
			assert.Equal(t, "Bearer test-token", asked.Token)
			assert.True(t, access.Required(asked.Method, asked.Path).Known, "what %s %s is asked under is known, which what the session may do is read off", asked.Method, asked.Path)

			if len(c.body) > 0 {
				assert.JSONEq(t, c.body, asked.Body)
			} else {
				assert.Empty(t, asked.Body)
			}
		})

		t.Run(c.name+" fails as the API refuses it", func(t *testing.T) {
			t.Parallel()

			w, _ := newWorkspace(t, newRecorder(http.StatusNotFound, `{"code":"not_found"}`))

			_, err := c.call(desktop.NewWorkloads(w))

			assert.True(t, api.IsNotFound(err))
		})

		t.Run(c.name+" is not asked of a context that cannot be read", func(t *testing.T) {
			t.Parallel()

			_, err := c.call(desktop.NewWorkloads(brokenWorkspace(t)))

			assert.Error(t, err)
		})
	}

	t.Run("what the API answers is answered as it is", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, newRecorder(http.StatusOK, `{"kind":"vm","metadata":{"uuid":"v-1"}}`))

		got, err := desktop.NewWorkloads(w).VM(context.Background(), "v-1")

		require.NoError(t, err)
		assert.JSONEq(t, `{"kind":"vm","metadata":{"uuid":"v-1"}}`, string(got))
	})

	t.Run("an answer of nothing is none", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, newRecorder(http.StatusAccepted, ``))

		got, err := desktop.NewWorkloads(w).DeleteVM(context.Background(), "v-1", false)

		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
