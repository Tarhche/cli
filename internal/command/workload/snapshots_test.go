package workload

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

func TestListSnapshotsCommand(t *testing.T) {
	t.Parallel()

	header := []string{"UUID", "NAME", "VM", "STATE", "SIZE", "DISK", "CREATED"}

	t.Run("one's snapshots", func(t *testing.T) {
		t.Parallel()

		taking := workloadtest.Snapshot("p-new", "now", vmBox, "").With("status", workloadtest.Object{"state": "creating"})
		fake := workloadtest.New().On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before, taking))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListSnapshotsCommand(h.Cli)))
		assert.Equal(t, table(header,
			[]string{snapshotBefore, "before", "box", "ready", "300MiB", "4GiB", "3 hours ago"},
			[]string{"p-new", "now", vmBox, "creating", "", "", "3 hours ago"},
		), h.Out.String())
	})

	t.Run("anybody's of one vm, with whose they are", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(box)).
			On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(before))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListSnapshotsCommand(h.Cli), "-A", "--vm", "box", "--format", "{{.Name}} {{.Owner}} {{.Image}} {{.Docker}}"))
		assert.Equal(t, "before owner-1 ubuntu:24.04 false\n", h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, everyone+"/snapshots")
		assert.Equal(t, url.Values{"vm": {vmBox}, "page": {"1"}}, sent.Query)
	})

	t.Run("uuids alone", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListSnapshotsCommand(h.Cli), "-q"))
		assert.Equal(t, snapshotBefore+"\n", h.Out.String())
	})

	t.Run("a vm that is not there, or snapshots that cannot be listed, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, mine+"/snapshots", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListSnapshotsCommand(h.Cli), "--vm", "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newListSnapshotsCommand(h.Cli)))
		assert.Equal(t, "Error: no such vm: box\nError: internal\n", h.Err.String())
	})

	testRequiresArguments(t, newListSnapshotsCommand, `"tarhche ls" accepts no arguments.`, "x")
}

func TestCreateSnapshotCommand(t *testing.T) {
	t.Parallel()

	t.Run("a snapshot of one's vm, named as it is told", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(box)).
			On(http.MethodPost, everyone+"/vms/"+vmBox+"/snapshots", http.StatusCreated, before)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateSnapshotCommand(h.Cli), "--name", "before", "box"))
		assert.Equal(t, snapshotBefore+"\n", h.Out.String())
		assert.Equal(t, map[string]any{"name": "before"}, sentJSON(t, fake, http.MethodPost, everyone+"/vms/"+vmBox+"/snapshots"))
	})

	t.Run("a snapshot given no name is named after its vm and when it was taken", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+vmBox, http.StatusOK, box).
			On(http.MethodPost, everyone+"/vms/"+vmBox+"/snapshots", http.StatusCreated, before)
		h := harnessOf(t, fake)

		cmd := newCreateSnapshotCommand(h.Cli)
		cmd.now = func() time.Time { return time.Date(2026, 11, 22, 10, 30, 5, 0, time.FixedZone("Tehran", 12600)) }

		assert.Equal(t, console.ExitSuccess, h.Run(cmd, vmBox))
		assert.Equal(t, map[string]any{"name": "box-20261122-070005"}, sentJSON(t, fake, http.MethodPost, everyone+"/vms/"+vmBox+"/snapshots"))
	})

	t.Run("a vm that is not there, or a snapshot refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(box)).
			On(http.MethodPost, everyone+"/vms/"+vmBox+"/snapshots", http.StatusBadRequest, workloadtest.Refused(map[string]string{"snapshots": "over your quota"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateSnapshotCommand(h.Cli), "web"))
		assert.Equal(t, console.ExitFailure, h.Run(newCreateSnapshotCommand(h.Cli), "box"))
		assert.Equal(t, "Error: no such vm: web\nError: snapshots: over your quota\n", h.Err.String())
	})

	testRequiresArguments(t, newCreateSnapshotCommand, `"tarhche create" requires exactly 1 argument.`)
}

func TestRemoveSnapshotsCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveSnapshotsCommand, mine+"/snapshots", workloadtest.Page(before), http.MethodDelete,
		func(uuid string) string { return mine + "/snapshots/" + uuid },
		map[string]string{"before": snapshotBefore})

	t.Run("anybody's", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodDelete, everyone+"/snapshots/"+snapshotBefore, http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveSnapshotsCommand(h.Cli), "-A", snapshotBefore))
		assert.Equal(t, snapshotBefore+"\n", h.Out.String())
	})

	testRequiresArguments(t, newRemoveSnapshotsCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestInspectSnapshotsCommand(t *testing.T) {
	t.Parallel()

	t.Run("snapshots are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectSnapshotsCommand(h.Cli), "before"))

		var written []map[string]any
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &written))
		assert.Equal(t, "snapshot", written[0]["kind"])
	})

	t.Run("each through a template, and what is not there on its own", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(before))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectSnapshotsCommand(h.Cli), "-A", "-f", "{{.status.engine}}", "before", "after"))
		assert.Equal(t, "microsandbox\n", h.Out.String())
		assert.Equal(t, "Error: no such snapshot: after\n", h.Err.String())
	})

	t.Run("snapshots that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/snapshots", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectSnapshotsCommand(h.Cli), "before"))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectSnapshotsCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestRenameSnapshotCommand(t *testing.T) {
	t.Parallel()

	t.Run("a snapshot is renamed", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodPatch, mine+"/snapshots/"+snapshotBefore, http.StatusOK, before.With("metadata.name", "after"))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRenameSnapshotCommand(h.Cli), "before", "after"))
		assert.Empty(t, h.Out.String())
		assert.Equal(t, map[string]any{"name": "after"}, sentJSON(t, fake, http.MethodPatch, mine+"/snapshots/"+snapshotBefore))
	})

	t.Run("a snapshot that is not there, or a name refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/snapshots/"+snapshotBefore, http.StatusOK, before).
			On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodPatch, everyone+"/snapshots/"+snapshotBefore, http.StatusBadRequest, workloadtest.Refused(map[string]string{"name": "required"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRenameSnapshotCommand(h.Cli), "-A", "after", "x"))
		assert.Equal(t, console.ExitFailure, h.Run(newRenameSnapshotCommand(h.Cli), "-A", snapshotBefore, " "))
		assert.Equal(t, "Error: no such snapshot: after\nError: name: required\n", h.Err.String())
	})

	testRequiresArguments(t, newRenameSnapshotCommand, `"tarhche rename" requires exactly 2 arguments.`, "before")
}

func TestInspectCommand(t *testing.T) {
	t.Parallel()

	// everything is an API answering every kind of workload.
	everything := func(base string) *workloadtest.API {
		return workloadtest.New().
			On(http.MethodGet, base+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, base+"/vms", http.StatusOK, workloadtest.Page(web, box)).
			On(http.MethodGet, base+"/vms/"+vmWeb+"/images", http.StatusOK, workloadtest.Items(nginxImage)).
			On(http.MethodGet, base+"/vms/"+vmWeb+"/networks", http.StatusOK, workloadtest.Items(backend)).
			On(http.MethodGet, base+"/vms/"+vmWeb+"/volumes", http.StatusOK, workloadtest.Items(data)).
			On(http.MethodGet, base+"/stacks", http.StatusOK, workloadtest.Page(blog)).
			On(http.MethodGet, base+"/snapshots", http.StatusOK, workloadtest.Page(before))
	}

	t.Run("whatever a reference names is found, each kind read once", func(t *testing.T) {
		t.Parallel()

		fake := everything(mine)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectCommand(h.Cli), "-f", "{{.kind}}", "nginx", "box", "nginx:1.27", "backend", "data", "blog", "before", "web"))
		assert.Equal(t, "container\nvm\nimage\nnetwork\nvolume\nstack\nsnapshot\nvm\n", h.Out.String())

		counted := make(map[string]int)
		for _, route := range fake.Routes() {
			counted[route]++
		}

		for route, count := range counted {
			assert.Equal(t, 1, count, route)
		}
	})

	t.Run("a reference is looked for as the type it is said to be, anybody's", func(t *testing.T) {
		t.Parallel()

		fake := everything(everyone)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectCommand(h.Cli), "-A", "--type", "vm", "--format", "{{.metadata.name}}", "web", "nginx"))
		assert.Equal(t, "web\n", h.Out.String())
		assert.Equal(t, "Error: no such vm: nginx\n", h.Err.String())

		for _, route := range fake.Routes() {
			assert.NotContains(t, route, "/containers", "only vms were looked for")
		}
	})

	t.Run("what names nothing names no object", func(t *testing.T) {
		t.Parallel()

		h := harnessOf(t, everything(mine))

		assert.Equal(t, console.ExitFailure, h.Run(newInspectCommand(h.Cli), "nothing"))
		assert.Equal(t, "[]\n", h.Out.String())
		assert.Equal(t, "Error: no such object: nothing\n", h.Err.String())
	})

	t.Run("what names more than one of a kind is said", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx, nginx.With("metadata.uuid", "c-other").With("vm_name", "box")))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectCommand(h.Cli), "nginx"))
		assert.Contains(t, h.Err.String(), `Error: "nginx" names more than one container`)
	})

	t.Run("what cannot be read stops it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items()).
			On(http.MethodGet, mine+"/vms", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectCommand(h.Cli), "a", "b"))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())

		for _, kind := range []string{"image", "network", "volume"} {
			h := harnessOf(t, fake)

			assert.Equal(t, console.ExitFailure, h.Run(newInspectCommand(h.Cli), "--type", kind, "a"), kind)
			assert.Equal(t, "Error: forbidden\n", h.Err.String(), kind)
		}
	})

	t.Run("a type that is none is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harnessOf(t, workloadtest.New())

		assert.Equal(t, console.ExitUsageError, h.Run(newInspectCommand(h.Cli), "--type", "pod", "web"))
		assert.Equal(t, usage("inspect", `"pod" is not a type: give one of container, vm, image, network, volume, stack, snapshot`), h.Err.String())
	})

	testRequiresArguments(t, newInspectCommand, `"tarhche inspect" requires at least 1 argument.`)
}
