package workload

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const snapshotBefore = "0193a5b2-7c41-7000-8000-00000000000b"

var before = workloadtest.Snapshot(snapshotBefore, "before", vmBox, "box")

func TestListVMsCommand(t *testing.T) {
	t.Parallel()

	header := []string{"UUID", "NAME", "IMAGE", "STATE", "CPUS", "MEMORY", "DISK", "PORTS", "CREATED"}

	t.Run("one's vms, machines and docker vms alike", func(t *testing.T) {
		t.Parallel()

		failed := box.With("status", workloadtest.Object{"state": "failed", "reason": "no_capacity"}).With("spec.ports", []uint{22, 80})
		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, failed))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListVMsCommand(h.Cli)))
		assert.Equal(t, table(header,
			[]string{vmWeb, "web", "docker", "running", "1", "512MiB", "4GiB", "8080", "3 hours ago"},
			[]string{vmBox, "box", "ubuntu:24.04", "failed: no_capacity", "1", "512MiB", "4GiB", "22, 80", "3 hours ago"},
		), h.Out.String())
	})

	t.Run("anybody's, with whose they are", func(t *testing.T) {
		t.Parallel()

		owned := web.With("owner", workloadtest.Object{"uuid": "u1", "name": "Sarah"})
		guest := box.With("metadata.owner_uuid", "guest")
		fake := workloadtest.New().On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(owned, guest))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListVMsCommand(h.Cli), "-A", "--format", "{{.Name}} {{.Owner}} {{.Type}} {{.Slug}} {{.URLs}}"))
		assert.Equal(t, "web Sarah docker web-x1y2 https://web-x1y2-8080.workload.example\nbox guest machine box-x1y2 https://box-x1y2-8080.workload.example\n", h.Out.String())
	})

	t.Run("uuids alone", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListVMsCommand(h.Cli), "-q"))
		assert.Equal(t, vmWeb+"\n"+vmBox+"\n", h.Out.String())
	})

	t.Run("vms that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListVMsCommand(h.Cli)))
		assert.Equal(t, "Error: internal\n", h.Err.String())
	})

	testRequiresArguments(t, newListVMsCommand, `"tarhche ls" accepts no arguments.`, "x")
}

func TestCreateVMCommand(t *testing.T) {
	t.Parallel()

	t.Run("a machine is given its kind's defaults and a made up name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodPost, everyone+"/vms", http.StatusCreated, box)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVMCommand(h.Cli), "--image", "docker.io/library/debian:12@sha256:abc"))
		assert.Equal(t, vmBox+"\n", h.Out.String())

		body := sentJSON(t, fake, http.MethodPost, everyone+"/vms")
		assert.Regexp(t, `^debian-[a-z2-7]{4}$`, body["name"])
		delete(body, "name")
		assert.Equal(t, map[string]any{
			"kind":             "machine",
			"image":            "docker.io/library/debian:12@sha256:abc",
			"resources":        map[string]any{"cpus": float64(1), "memory": float64(128 << 20), "disk": float64(1 << 30)},
			"ports":            []any{},
			"network":          map[string]any{"ingress": "allow", "egress": "allow"},
			"persistent_disk":  true,
			"lifetime_seconds": float64(0),
		}, body)
	})

	t.Run("a docker vm is given its kind's defaults, and what the flags say", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodPost, everyone+"/vms", http.StatusCreated, web)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVMCommand(h.Cli), "--docker", "--name", "web", "--cpus", "2", "-p", "80", "-p", "443,8080", "--egress", "deny", "--lifetime", "2h30m", "--persistent-disk=false"))
		assert.Equal(t, map[string]any{
			"name":             "web",
			"kind":             "docker",
			"resources":        map[string]any{"cpus": float64(2), "memory": float64(512 << 20), "disk": float64(4 << 30)},
			"ports":            []any{float64(80), float64(443), float64(8080)},
			"network":          map[string]any{"ingress": "allow", "egress": "deny"},
			"persistent_disk":  false,
			"lifetime_seconds": float64(9000),
		}, sentJSON(t, fake, http.MethodPost, everyone+"/vms"))
	})

	t.Run("a vm made from a snapshot takes its kind, and its disk unless it is given more", func(t *testing.T) {
		t.Parallel()

		docker := before.With("docker", true)
		fake := workloadtest.New().
			On(http.MethodGet, mine+"/snapshots/"+snapshotBefore, http.StatusOK, docker).
			On(http.MethodPost, everyone+"/vms", http.StatusCreated, web)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVMCommand(h.Cli), "--from-snapshot", snapshotBefore, "-m", "1g"))

		body := sentJSON(t, fake, http.MethodPost, everyone+"/vms")
		assert.Equal(t, "docker", body["kind"])
		assert.Equal(t, snapshotBefore, body["snapshot_uuid"])
		assert.Equal(t, map[string]any{"cpus": float64(1), "memory": float64(1 << 30), "disk": float64(0)}, body["resources"])
		assert.Regexp(t, `^docker-[a-z2-7]{4}$`, body["name"])
	})

	t.Run("a machine from a snapshot of a machine, given a disk", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodPost, everyone+"/vms", http.StatusCreated, box)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVMCommand(h.Cli), "--docker", "--from-snapshot", "before", "--disk", "10g"))

		body := sentJSON(t, fake, http.MethodPost, everyone+"/vms")
		assert.Equal(t, "machine", body["kind"])
		assert.InDelta(t, float64(10<<30), body["resources"].(map[string]any)["disk"], 0)
		assert.Regexp(t, `^machine-[a-z2-7]{4}$`, body["name"])
	})

	t.Run("a snapshot that is not there, or a vm refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page()).
			On(http.MethodPost, everyone+"/vms", http.StatusBadRequest, workloadtest.Refused(map[string]string{"resources.memory": "too small: 128 MiB at least"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateVMCommand(h.Cli), "--from-snapshot", "after"))
		assert.Equal(t, console.ExitFailure, h.Run(newCreateVMCommand(h.Cli), "-m", "1m"))
		assert.Equal(t, "Error: no such snapshot: after\nError: resources.memory: too small: 128 MiB at least\n", h.Err.String())
	})

	t.Run("a size or a port that cannot be read is a usage error", func(t *testing.T) {
		t.Parallel()

		for _, args := range [][]string{{"--memory", "lots"}, {"-p", "http"}, {"-p", "70000"}} {
			fake := workloadtest.New()
			h := harnessOf(t, fake)

			assert.Equal(t, console.ExitUsageError, h.Run(newCreateVMCommand(h.Cli), args...), args)
			assert.Empty(t, fake.Requests())
		}
	})

	testRequiresArguments(t, newCreateVMCommand, `"tarhche create" accepts no arguments.`, "web")
}

func TestRemoveVMsCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveVMsCommand, mine+"/vms", workloadtest.Page(web, box), http.MethodDelete,
		func(uuid string) string { return mine + "/vms/" + uuid },
		map[string]string{"web": vmWeb, "box-x1y2": vmBox})

	t.Run("a vm deleted with what needs it, and one still needed, which is said", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodDelete, everyone+"/vms/"+vmWeb, http.StatusAccepted, `{"dependents":[{"kind":"container","uuid":"c1","name":"nginx","recreates":true}],"recreated":true}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveVMsCommand(h.Cli), "-A", "--cascade", "web"))
		assert.Equal(t, "web\n", h.Out.String())
		assert.Equal(t, "Warning: web is needed by container nginx, which will make it again: delete them with it with --cascade.\n", h.Err.String())

		sent, _ := fake.Sent(http.MethodDelete, everyone+"/vms/"+vmWeb)
		assert.Equal(t, url.Values{"cascade": {"true"}}, sent.Query)
	})

	testRequiresArguments(t, newRemoveVMsCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestStartVMsCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStartVMsCommand, mine+"/vms", workloadtest.Page(web, box), http.MethodPost,
		func(uuid string) string { return mine + "/vms/" + uuid + "/start" },
		map[string]string{"box": vmBox})

	testRequiresArguments(t, newStartVMsCommand, `"tarhche start" requires at least 1 argument.`)
}

func TestStopVMsCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStopVMsCommand, everyone+"/vms", workloadtest.Page(web, box), http.MethodPost,
		func(uuid string) string { return everyone + "/vms/" + uuid + "/stop" },
		map[string]string{vmWeb: vmWeb}, "-A")

	testRequiresArguments(t, newStopVMsCommand, `"tarhche stop" requires at least 1 argument.`)
}

func TestRestartVMsCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRestartVMsCommand, mine+"/vms", workloadtest.Page(web), http.MethodPost,
		func(uuid string) string { return mine + "/vms/" + uuid + "/restart" },
		map[string]string{"web-x1y2": vmWeb})

	testRequiresArguments(t, newRestartVMsCommand, `"tarhche restart" requires at least 1 argument.`)
}

func TestInspectVMsCommand(t *testing.T) {
	t.Parallel()

	t.Run("vms are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectVMsCommand(h.Cli), "web", vmBox))

		var written []map[string]any
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &written))
		require.Len(t, written, 2)
		assert.Equal(t, true, written[0]["docker"])
	})

	t.Run("each through a template, and what is not there on its own", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectVMsCommand(h.Cli), "-A", "-f", "{{.spec.resources.memory}} {{bytes .spec.resources.disk}}", "web", "box"))
		assert.Equal(t, "536870912 4GiB\n", h.Out.String())
		assert.Equal(t, "Error: no such vm: box\n", h.Err.String())
	})

	t.Run("vms that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectVMsCommand(h.Cli), "web"))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectVMsCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestShowVMLogsCommand(t *testing.T) {
	t.Parallel()

	logs := mine + "/vms/" + vmWeb + "/logs"

	t.Run("what a vm wrote, every source of it on the standard output", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","source":"kernel","line":"booting"},{"at":"2026-11-22T10:00:01Z","source":"exec","line":"ls"}]}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowVMLogsCommand(h.Cli), "--tail", "all", vmWeb))
		assert.Equal(t, "booting\nls\n", h.Out.String())
		assert.Empty(t, h.Err.String())
	})

	t.Run("a vm's log is followed until interrupted", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/logs", http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","source":"main","line":"up"}]}`)
		h := harnessOf(t, fake)

		cmd := newShowVMLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, command.ExitInterrupted, runUntil(t, fake, h, cmd, http.MethodGet, everyone+"/vms/"+vmWeb+"/logs", 3, "-A", "-f", "-t", "web"))
		assert.Equal(t, "2026-11-22T10:00:00Z up\n", h.Out.String(), "a line is written once however often it is read")
	})

	t.Run("a vm that is not there, or a bad time, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowVMLogsCommand(h.Cli), "web"))
		assert.Equal(t, "Error: no such vm: web\n", h.Err.String())

		h = harnessOf(t, fake)
		assert.Equal(t, console.ExitUsageError, h.Run(newShowVMLogsCommand(h.Cli), "--since", "yesterday", "web"))
	})

	testRequiresArguments(t, newShowVMLogsCommand, `"tarhche logs" requires exactly 1 argument.`)
}

func TestShowVMStatsCommand(t *testing.T) {
	t.Parallel()

	usage := mine + "/vms/" + vmWeb + "/usage"

	t.Run("what a vm used, a step at a time", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, usage, http.StatusOK, `{"from":"2026-11-22T09:00:00Z","to":"2026-11-22T10:00:00Z","step":60,"points":[
				{"at":"2026-11-22T09:01:00Z","cpu_percent":5,"memory_used":134217728,"memory_limit":536870912,"disk_used":1073741824,"disk_total":4294967296,"network_rx":1024,"network_tx":0}]}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowVMStatsCommand(h.Cli), "--since", "2026-11-22T09:00:00Z", "--until", "2026-11-22T10:00:00Z", "web"))
		assert.Equal(t, table([]string{"TIME", "CPU %", "MEM USAGE / LIMIT", "MEM %", "DISK USAGE / TOTAL", "NET I/O"},
			[]string{"2026-11-22T09:01:00Z", "5.00%", "128MiB / 512MiB", "25.00%", "1GiB / 4GiB", "1KiB / 0B"},
		), h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, usage)
		assert.Equal(t, url.Values{"from": {"2026-11-22T09:00:00Z"}, "to": {"2026-11-22T10:00:00Z"}}, sent.Query)
	})

	t.Run("the last hour by default, through a template", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/usage", http.StatusOK, `{"points":[{"at":"2026-11-22T09:01:00Z","disk_used":1,"disk_total":4}]}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowVMStatsCommand(h.Cli), "-A", "--format", "{{.DiskPerc}}", vmWeb))
		assert.Equal(t, "25.00%\n", h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, everyone+"/vms/"+vmWeb+"/usage")
		assert.Empty(t, sent.Query)
	})

	t.Run("a bad time, a vm that is not there, or usage that cannot be read, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, usage, http.StatusServiceUnavailable, `{"code":"unavailable"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitUsageError, h.Run(newShowVMStatsCommand(h.Cli), "--since", "soon", "web"))
		assert.Equal(t, console.ExitUsageError, h.Run(newShowVMStatsCommand(h.Cli), "--until", "later", "web"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowVMStatsCommand(h.Cli), "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowVMStatsCommand(h.Cli), "web"))
		assert.True(t, strings.HasSuffix(h.Err.String(), "Error: no such vm: box\nError: unavailable\n"), h.Err.String())
	})

	testRequiresArguments(t, newShowVMStatsCommand, `"tarhche stats" requires exactly 1 argument.`)
}

func TestUpdateVMCommand(t *testing.T) {
	t.Parallel()

	t.Run("what is given changes, the rest of the network and the resources as they are", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPatch, mine+"/vms/"+vmWeb, http.StatusOK, web)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newUpdateVMCommand(h.Cli), "--name", "api", "--lifetime", "0", "-p", "80", "--egress", "deny", "-m", "1g", "web"))
		assert.Equal(t, "web\n", h.Out.String())
		assert.Equal(t, map[string]any{
			"name":             "api",
			"lifetime_seconds": float64(0),
			"ports":            []any{float64(80)},
			"network":          map[string]any{"ingress": "allow", "egress": "deny"},
			"resources":        map[string]any{"cpus": float64(1), "memory": float64(1 << 30), "disk": float64(4 << 30)},
		}, sentJSON(t, fake, http.MethodPatch, mine+"/vms/"+vmWeb))
	})

	t.Run("ports taken away, the ingress shut and a disk grown", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodPatch, everyone+"/vms/"+vmWeb, http.StatusOK, web)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newUpdateVMCommand(h.Cli), "-A", "--no-ports", "--ingress", "deny", "--disk", "10g", "--cpus", "2", "--lifetime", "90m", vmWeb))
		assert.Equal(t, map[string]any{
			"lifetime_seconds": float64(5400),
			"ports":            []any{},
			"network":          map[string]any{"ingress": "deny", "egress": "allow"},
			"resources":        map[string]any{"cpus": float64(2), "memory": float64(512 << 20), "disk": float64(10 << 30)},
		}, sentJSON(t, fake, http.MethodPatch, everyone+"/vms/"+vmWeb))
	})

	t.Run("nothing to change, or ports both given and taken away, is a usage error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitUsageError, h.Run(newUpdateVMCommand(h.Cli), "web"))
		assert.Equal(t, usage("update", "nothing to change: give what changes, as --name or --memory"), h.Err.String())

		h = harnessOf(t, fake)
		assert.Equal(t, console.ExitUsageError, h.Run(newUpdateVMCommand(h.Cli), "-p", "80", "--no-ports", "web"))
		assert.Equal(t, usage("update", "--publish and --no-ports cannot be given together"), h.Err.String())
	})

	t.Run("a vm that is not there, or a change refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPatch, mine+"/vms/"+vmWeb, http.StatusBadRequest, workloadtest.Refused(map[string]string{"resources.disk": "only grows"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateVMCommand(h.Cli), "--name", "x", "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newUpdateVMCommand(h.Cli), "--disk", "1g", "web"))
		assert.Equal(t, "Error: no such vm: box\nError: resources.disk: only grows\n", h.Err.String())
	})

	testRequiresArguments(t, newUpdateVMCommand, `"tarhche update" requires exactly 1 argument.`)
}

func TestRestoreVMCommand(t *testing.T) {
	t.Parallel()

	t.Run("a vm's disk is replaced by a snapshot's", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(box)).
			On(http.MethodGet, mine+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodPost, mine+"/vms/"+vmBox+"/restore", http.StatusAccepted, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRestoreVMCommand(h.Cli), "--snapshot", "before", "box"))
		assert.Equal(t, "box\n", h.Out.String())
		assert.Equal(t, map[string]any{"snapshot_uuid": snapshotBefore}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmBox+"/restore"))
	})

	t.Run("a snapshot is required", func(t *testing.T) {
		t.Parallel()

		h := harnessOf(t, workloadtest.New())

		assert.Equal(t, console.ExitUsageError, h.Run(newRestoreVMCommand(h.Cli), "box"))
		assert.Equal(t, usage("restore", "a snapshot is required: give it with --snapshot"), h.Err.String())
	})

	t.Run("a vm or a snapshot that is not there, or a restore refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(box)).
			On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(before)).
			On(http.MethodPost, everyone+"/vms/"+vmBox+"/restore", http.StatusBadRequest, workloadtest.Refused(map[string]string{"snapshot_uuid": "larger than its disk"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRestoreVMCommand(h.Cli), "-A", "--snapshot", "before", "web"))
		assert.Equal(t, console.ExitFailure, h.Run(newRestoreVMCommand(h.Cli), "-A", "--snapshot", "after", "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newRestoreVMCommand(h.Cli), "-A", "--snapshot", "before", "box"))
		assert.Equal(t, "Error: no such vm: web\nError: no such snapshot: after\nError: snapshot_uuid: larger than its disk\n", h.Err.String())
	})

	testRequiresArguments(t, newRestoreVMCommand, `"tarhche restore" requires exactly 1 argument.`)
}

func TestShowVMDependentsCommand(t *testing.T) {
	t.Parallel()

	t.Run("what needs a vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/dependents", http.StatusOK, `{"dependents":[{"kind":"stack","uuid":"s1","name":"blog","recreates":true},{"kind":"container","uuid":"c1","recreates":false}],"recreated":true}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowVMDependentsCommand(h.Cli), "web"))
		assert.Equal(t, table([]string{"KIND", "UUID", "NAME", "RECREATES"},
			[]string{"stack", "s1", "blog", "true"},
			[]string{"container", "c1", "", "false"},
		), h.Out.String())
	})

	t.Run("uuids alone, of anybody's", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/dependents", http.StatusOK, `{"dependents":[{"kind":"stack","uuid":"s1"}]}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowVMDependentsCommand(h.Cli), "-A", "-q", vmWeb))
		assert.Equal(t, "s1\n", h.Out.String())
	})

	t.Run("a vm that is not there, or dependents that cannot be read, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/dependents", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowVMDependentsCommand(h.Cli), "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowVMDependentsCommand(h.Cli), "web"))
		assert.Equal(t, "Error: no such vm: box\nError: internal\n", h.Err.String())
	})

	testRequiresArguments(t, newShowVMDependentsCommand, `"tarhche dependents" requires exactly 1 argument.`)
}
