package workload

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

var containerHeader = []string{"CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS", "NAMES", "VM"}

func nginxRow() []string {
	return []string{"abcdef123456", "nginx:1.27", `"/docker-entrypoint.…"`, "3 hours ago", "Up 2 hours", "0.0.0.0:8080->80/tcp", "nginx", "web"}
}

func redisRow() []string {
	return []string{"fedcba098765", "redis:7", `"redis-server"`, "3 hours ago", "Exited (0) 5 minutes ago", "", "redis", "web"}
}

func TestListContainersCommand(t *testing.T) {
	t.Parallel()

	t.Run("the running containers across one's docker vms", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(redis, nginx))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListContainersCommand(h.Cli)))
		assert.Equal(t, table(containerHeader, nginxRow()), h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, mine+"/containers")
		assert.Empty(t, sent.Query)
	})

	t.Run("every container with -a, the newest first", func(t *testing.T) {
		t.Parallel()

		older := redis.With("status.created_at", time.Now().Add(-5*time.Hour).UTC().Format(time.RFC3339))
		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(older, nginx))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListContainersCommand(h.Cli), "-a"))

		row := redisRow()
		row[3] = "5 hours ago"
		assert.Equal(t, table(containerHeader, nginxRow(), row), h.Out.String())
	})

	t.Run("ids alone, untruncated, of anybody's in one vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(nginx, redis))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListContainersCommand(h.Cli), "-qa", "--no-trunc", "--vm", vmWeb, "-A"))
		assert.Equal(t, "abcdef1234567890\nfedcba0987654321\n", h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, everyone+"/containers")
		assert.Equal(t, url.Values{"vm": {vmWeb}}, sent.Query)
	})

	t.Run("a container docker has not made yet is shown as it was asked for", func(t *testing.T) {
		t.Parallel()

		pending := workloadtest.Object{
			"kind":     "container",
			"metadata": workloadtest.Object{"uuid": "c-new", "name": "api"},
			"spec": workloadtest.Object{
				"image": "api:1", "entrypoint": []string{"/bin/api"}, "command": []string{"serve"},
				"ports": []workloadtest.Object{{"container_port": 9000, "host_port": 0}},
			},
			"status":  workloadtest.Object{"state": "pending", "reason": "its vm is starting"},
			"vm_uuid": vmWeb,
		}
		failed := pending.
			With("metadata.uuid", "c-bad").
			With("status", workloadtest.Object{"state": "failed", "failure": workloadtest.Object{"code": "conflict", "message": "name in use"}})

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(pending, failed))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListContainersCommand(h.Cli), "--all", "--format", "{{.ID}}|{{.Command}}|{{.Status}}|{{.Ports}}|{{.VM}}"))
		assert.Equal(t, "c-new|\"/bin/api serve\"|pending: its vm is starting|9000/tcp|"+vmWeb+"\nc-bad|\"/bin/api serve\"|failed: name in use|9000/tcp|"+vmWeb+"\n", h.Out.String())
	})

	t.Run("docker's template fields and labels", func(t *testing.T) {
		t.Parallel()

		labelled := nginx.
			With("metadata.labels", workloadtest.Object{"workload.stack.service": "web"}).
			With("status.labels", workloadtest.Object{"maintainer": "nginx"}).
			With("status.mounts", []workloadtest.Object{{"type": "volume", "source": "data", "target": "/data"}})
		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(labelled))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListContainersCommand(h.Cli), "--format", `{{.Label "maintainer"}} {{.Service}} {{.Mounts}} {{.Networks}} {{.State}}`))
		assert.Equal(t, "nginx web data bridge running\n", h.Out.String())
	})

	t.Run("a vm that is not there is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListContainersCommand(h.Cli), "--vm", "nothing"))
		assert.Equal(t, "Error: no such vm: nothing\n", h.Err.String())
	})

	t.Run("what the api refuses is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListContainersCommand(h.Cli)))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	t.Run("it takes no arguments", func(t *testing.T) {
		t.Parallel()

		testRequiresArguments(t, newListContainersCommand, `"tarhche ls" accepts no arguments.`, "web")
	})
}

func TestRunContainerCommand(t *testing.T) {
	t.Parallel()

	created := func(vmCreated bool) workloadtest.Object {
		return workloadtest.Object{"vm": workloadtest.Object{"uuid": vmWeb, "name": "web", "created": vmCreated}, "container": nginx}
	}

	t.Run("a detached container is created as its flags say, in a vm made for it", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		envFile := filepath.Join(dir, "env")
		require.NoError(t, os.WriteFile(envFile, []byte("# a comment\nFROM_FILE=1\n\nLOOKED_UP\nMISSING\n"), 0o600))

		fake := workloadtest.New().On(http.MethodPost, everyone+"/containers", http.StatusCreated, created(true))
		h := harnessOf(t, fake)

		cmd := newRunContainerCommand(h.Cli)
		cmd.lookupEnv = func(key string) (string, bool) {
			if key == "LOOKED_UP" || key == "HOME" {
				return "here", true
			}

			return "", false
		}

		status := h.Run(cmd,
			"-d", "--name", "nginx",
			"-e", "A=b", "--env", "HOME", "--env-file", envFile,
			"-p", "8080:80", "-p", "127.0.0.1:8443:443/udp", "-p", "9000", "-p", "[::1]::53",
			"-v", "data:/data:ro", "-v", "/srv:/srv", "-v", "/cache",
			"--mount", "type=bind,src=/etc,dst=/host/etc,readonly", "--tmpfs", "/run:size=64m",
			"--network", "backend", "--network", "frontend",
			"--restart", "always", "--entrypoint", "/bin/sh", "-u", "1000", "-w", "/srv",
			"--privileged", "--cpus", "0.5", "-m", "256m", "--pids-limit", "100", "--runtime", "runc",
			"--vm-name", "box", "--vm-cpus", "2", "--vm-memory", "1g", "--vm-publish", "80,443", "--vm-egress", "deny",
			"nginx", "nginx", "-g", "daemon off;",
		)

		assert.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "abcdef1234567890\n", h.Out.String())
		assert.Equal(t, "Made Docker VM web for it.\n", h.Err.String())

		sent, _ := fake.Sent(http.MethodPost, everyone+"/containers")
		assert.JSONEq(t, `{
			"vm": {"name": "box", "resources": {"cpus": 2, "memory": 1073741824, "disk": 0}, "ports": [80, 443], "network": {"egress": "deny"}},
			"name": "nginx", "image": "nginx", "command": ["nginx", "-g", "daemon off;"], "entrypoint": ["/bin/sh"],
			"env": ["FROM_FILE=1", "LOOKED_UP=here", "A=b", "HOME=here"], "working_dir": "/srv",
			"ports": [
				{"container_port": 80, "host_port": 8080},
				{"container_port": 443, "host_port": 8443, "host_ip": "127.0.0.1", "protocol": "udp"},
				{"container_port": 9000, "host_port": 0},
				{"container_port": 53, "host_port": 0, "host_ip": "::1"}
			],
			"mounts": [
				{"type": "volume", "source": "data", "target": "/data", "read_only": true},
				{"type": "bind", "source": "/srv", "target": "/srv"},
				{"type": "volume", "target": "/cache"},
				{"type": "bind", "source": "/etc", "target": "/host/etc", "read_only": true},
				{"type": "tmpfs", "target": "/run"}
			],
			"networks": ["backend", "frontend"], "restart_policy": "always", "cpus": 0.5, "memory": 268435456,
			"pids_limit": 100, "privileged": true, "user": "1000", "runtime": "runc"
		}`, sent.Body)
	})

	t.Run("a container goes in the vm --vm names", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box)).
			On(http.MethodPost, everyone+"/containers", http.StatusCreated, created(false))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRunContainerCommand(h.Cli), "-d", "--vm", "web", "nginx"))
		assert.Empty(t, h.Err.String())
		assert.Equal(t, map[string]any{"vm_uuid": vmWeb, "image": "nginx"}, sentJSON(t, fake, http.MethodPost, everyone+"/containers"))
	})

	t.Run("an attached container is followed until it ends, and exits as it exited", func(t *testing.T) {
		t.Parallel()

		ended := nginx.With("status.state", "completed").With("status.docker_status", "Exited (3) 1 second ago")
		logs := mine + "/vms/" + vmWeb + "/containers/c-nginx/logs"

		fake := workloadtest.New().
			On(http.MethodPost, everyone+"/containers", http.StatusCreated, created(false)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","stream":"stdout","line":"starting"}]}`).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","stream":"stdout","line":"starting"},{"at":"2026-11-22T10:00:01Z","stream":"stderr","line":"failing"}]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, nginx).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, ended)
		h := harnessOf(t, fake)

		cmd := newRunContainerCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, 3, h.Run(cmd, "nginx"))
		assert.Equal(t, "abcdef1234567890\nstarting\n", h.Out.String())
		assert.Equal(t, "failing\n", h.Err.String())
	})

	t.Run("an attached container that could not be made is an error", func(t *testing.T) {
		t.Parallel()

		failed := nginx.With("status.state", "failed").With("status.failure", workloadtest.Object{"code": "pull", "message": "no such image"})

		fake := workloadtest.New().
			On(http.MethodPost, everyone+"/containers", http.StatusCreated, created(false)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/logs", http.StatusOK, `{"items":[]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, failed)
		h := harnessOf(t, fake)

		cmd := newRunContainerCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitFailure, h.Run(cmd, "nginx"))
		assert.Equal(t, "Error: container nginx failed: no such image\n", h.Err.String())
	})

	t.Run("an attached container is followed until it is interrupted", func(t *testing.T) {
		t.Parallel()

		logs := mine + "/vms/" + vmWeb + "/containers/c-nginx/logs"
		fake := workloadtest.New().
			On(http.MethodPost, everyone+"/containers", http.StatusCreated, created(false)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, nginx)
		h := harnessOf(t, fake)

		cmd := newRunContainerCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, command.ExitInterrupted, runUntil(t, fake, h, cmd, http.MethodGet, logs, 3, "nginx"))
	})

	t.Run("what it is given wrongly is a usage error", func(t *testing.T) {
		t.Parallel()

		for args, message := range map[string]string{
			"-p 80:http nginx":             `"80:http" is not a port to publish: give [ip:][host port:]container port[/protocol]`,
			"-v a:b:c:d nginx":             `"a:b:c:d" is not a volume to mount: give [source:]target[:ro|rw]`,
			"--mount kind=x nginx":         `"kind=x" is not a mount: "kind" is none of type, source, target and readonly`,
			"--cpus lots nginx":            "",
			"-v data:/data:z nginx":        `"z" is not a volume option: give ro or rw`,
			"--mount ro=maybe,dst=/ nginx": `"ro=maybe,dst=/" is not a mount: readonly is true or false`,
		} {
			fake := workloadtest.New()
			h := harnessOf(t, fake)

			assert.Equal(t, console.ExitUsageError, h.Run(newRunContainerCommand(h.Cli), strings.Fields(args)...), args)
			assert.Contains(t, h.Err.String(), message, args)
			assert.Empty(t, fake.Requests(), args)
		}
	})

	t.Run("an env file that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New()
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRunContainerCommand(h.Cli), "--env-file", filepath.Join(t.TempDir(), "missing"), "nginx"))
		assert.Contains(t, h.Err.String(), "no such file or directory")
	})

	t.Run("a vm that is not there is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRunContainerCommand(h.Cli), "--vm", "nothing", "nginx"))
		assert.Equal(t, "Error: no such vm: nothing\n", h.Err.String())
	})

	t.Run("what the api refuses is said as it said it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodPost, everyone+"/containers", http.StatusBadRequest, workloadtest.Refused(map[string]string{"vm.resources.memory": "too small: 512 MiB at least"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRunContainerCommand(h.Cli), "--vm-memory", "1m", "nginx"))
		assert.Equal(t, "Error: vm.resources.memory: too small: 512 MiB at least\n", h.Err.String())
	})

	t.Run("an image is required", func(t *testing.T) {
		t.Parallel()

		testRequiresArguments(t, newRunContainerCommand, `"tarhche run" requires at least 1 argument.`)
	})
}

func TestStartContainersCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStartContainersCommand, mine+"/containers", workloadtest.Items(nginx, redis), http.MethodPost,
		func(uuid string) string { return mine + "/vms/" + vmWeb + "/containers/" + uuid + "/start" },
		map[string]string{"redis": "c-redis", "abcdef12": "c-nginx"})

	testRequiresArguments(t, newStartContainersCommand, `"tarhche start" requires at least 1 argument.`)
}

func TestStopContainersCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStopContainersCommand, everyone+"/containers", workloadtest.Items(nginx, redis), http.MethodPost,
		func(uuid string) string { return everyone + "/vms/" + vmWeb + "/containers/" + uuid + "/stop" },
		map[string]string{"nginx": "c-nginx"}, "-A")

	testRequiresArguments(t, newStopContainersCommand, `"tarhche stop" requires at least 1 argument.`)
}

func TestRestartContainersCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRestartContainersCommand, mine+"/containers", workloadtest.Items(nginx), http.MethodPost,
		func(uuid string) string { return mine + "/vms/" + vmWeb + "/containers/" + uuid + "/restart" },
		map[string]string{"c-nginx": "c-nginx"})

	testRequiresArguments(t, newRestartContainersCommand, `"tarhche restart" requires at least 1 argument.`)

	t.Run("the containers are looked for in the vm --vm names", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/containers/c-nginx/restart", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRestartContainersCommand(h.Cli), "--vm", vmWeb, "nginx"))

		sent, _ := fake.Sent(http.MethodGet, mine+"/containers")
		assert.Equal(t, vmWeb, sent.Query.Get("vm"))
	})

	t.Run("a vm that is not there is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRestartContainersCommand(h.Cli), "--vm", "web", "nginx"))
		assert.Equal(t, "Error: no such vm: web\n", h.Err.String())
	})
}

func TestRemoveContainersCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveContainersCommand, mine+"/containers", workloadtest.Items(nginx, redis), http.MethodDelete,
		func(uuid string) string { return mine + "/vms/" + vmWeb + "/containers/" + uuid },
		map[string]string{"redis": "c-redis"})

	testRequiresArguments(t, newRemoveContainersCommand, `"tarhche rm" requires at least 1 argument.`)

	t.Run("a running container is removed by force", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodDelete, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveContainersCommand(h.Cli), "-f", "nginx"))
		assert.Equal(t, "nginx\n", h.Out.String())

		sent, _ := fake.Sent(http.MethodDelete, mine+"/vms/"+vmWeb+"/containers/c-nginx")
		assert.Equal(t, url.Values{"force": {"true"}}, sent.Query)
	})
}

func TestShowContainerLogsCommand(t *testing.T) {
	t.Parallel()

	logs := mine + "/vms/" + vmWeb + "/containers/c-nginx/logs"

	t.Run("what a container wrote, stderr on stderr, with when it was written", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","stream":"stdout","line":"up"},{"at":"2026-11-22T10:00:01.5Z","stream":"stderr","line":"oops"}]}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerLogsCommand(h.Cli), "-t", "--tail", "10", "--since", "2026-11-22T09:00:00Z", "--until", "1700000000", "nginx"))
		assert.Equal(t, "2026-11-22T10:00:00Z up\n", h.Out.String())
		assert.Equal(t, "2026-11-22T10:00:01.5Z oops\n", h.Err.String())

		sent, _ := fake.Sent(http.MethodGet, logs)
		assert.Equal(t, url.Values{"tail": {"10"}, "from": {"2026-11-22T09:00:00Z"}, "to": {time.Unix(1700000000, 0).Format(time.RFC3339Nano)}}, sent.Query)
	})

	t.Run("a log followed is read from where it ended, each line once, until the container ends", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","line":"one"},{"at":"2026-11-22T10:00:01Z","line":"two"}]}`).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:01Z","line":"two"},{"at":"2026-11-22T10:00:01Z","line":"two again"},{"at":"2026-11-22T10:00:02Z","line":"three"}]}`).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:02Z","line":"three"}]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusNotFound, `{"code":"not_found"}`)
		h := harnessOf(t, fake)

		cmd := newShowContainerLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitSuccess, h.Run(cmd, "-f", "nginx"))
		assert.Equal(t, "one\ntwo\ntwo again\nthree\n", h.Out.String())

		var froms []string
		for _, r := range fake.Requests() {
			if r.Path == logs {
				froms = append(froms, r.Query.Get("from"))
			}
		}

		assert.Equal(t, []string{"", "2026-11-22T10:00:01Z", "2026-11-22T10:00:02Z", "2026-11-22T10:00:02Z"}, froms)
	})

	t.Run("--tail 0 follows what is written from now on", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","line":"old"}]}`).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[{"at":"2026-11-22T10:00:00Z","line":"old"},{"at":"2026-11-22T10:00:05Z","line":"new"}]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, nginx.With("status.state", "completed"))
		h := harnessOf(t, fake)

		cmd := newShowContainerLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitSuccess, h.Run(cmd, "-f", "-n", "0", "nginx"))
		assert.Equal(t, "new\n", h.Out.String())

		sent := fake.Requests()
		assert.Equal(t, "1", sent[1].Query.Get("tail"), "the last line is read to follow from")
	})

	t.Run("a follow ends at --until", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, nginx)
		h := harnessOf(t, fake)

		cmd := newShowContainerLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitSuccess, h.Run(cmd, "-f", "--until", "1s", "nginx"))
	})

	t.Run("what goes wrong while following is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[]}`).
			On(http.MethodGet, logs, http.StatusInternalServerError, `{"code":"internal"}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusOK, nginx)
		h := harnessOf(t, fake)

		cmd := newShowContainerLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitFailure, h.Run(cmd, "-f", "nginx"))
		assert.Equal(t, "Error: internal\n", h.Err.String())
	})

	t.Run("a container whose state cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusOK, `{"items":[]}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx", http.StatusBadGateway, `{"code":"unreachable"}`)
		h := harnessOf(t, fake)

		cmd := newShowContainerLogsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, console.ExitFailure, h.Run(cmd, "-f", "nginx"))
		assert.Equal(t, "Error: unreachable\n", h.Err.String())
	})

	t.Run("what it is given wrongly is a usage error", func(t *testing.T) {
		t.Parallel()

		for args, message := range map[string]string{
			"--tail many nginx":   `"many" is not a number of lines: give one, or all`,
			"--since soon nginx":  `"soon" is not a time`,
			"--until later nginx": `"later" is not a time`,
		} {
			fake := workloadtest.New()
			h := harnessOf(t, fake)

			assert.Equal(t, console.ExitUsageError, h.Run(newShowContainerLogsCommand(h.Cli), strings.Fields(args)...), args)
			assert.Contains(t, h.Err.String(), message, args)
		}

		testRequiresArguments(t, newShowContainerLogsCommand, `"tarhche logs" requires exactly 1 argument.`)
	})

	t.Run("a container that is not there, or a log that cannot be read, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, logs, http.StatusServiceUnavailable, `{"code":"unavailable"}`).
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerLogsCommand(h.Cli), "redis"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerLogsCommand(h.Cli), "nginx"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerLogsCommand(h.Cli), "--vm", "web", "nginx"))
		assert.Equal(t, "Error: no such container: redis\nError: unavailable\nError: no such vm: web\n", h.Err.String())
	})
}

func TestShowContainerStatsCommand(t *testing.T) {
	t.Parallel()

	header := []string{"CONTAINER ID", "NAME", "CPU %", "MEM USAGE / LIMIT", "MEM %", "NET I/O", "BLOCK I/O", "PIDS"}
	stats := `{"cpu_percent":12.5,"memory_used":52428800,"memory_limit":209715200,"network_rx":1024,"network_tx":2048,"block_read":0,"block_write":4096,"pids":3}`

	t.Run("one sample of every running container", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx, redis)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/stats", http.StatusOK, stats)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerStatsCommand(h.Cli), "--no-stream"))
		assert.Equal(t, table(header, []string{"abcdef123456", "nginx", "12.50%", "50MiB / 200MiB", "25.00%", "1KiB / 2KiB", "0B / 4KiB", "3"}), h.Out.String())
	})

	t.Run("what cannot be sampled is shown as such", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx, redis)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/stats", http.StatusOK, `{"memory_limit":0}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-redis/stats", http.StatusBadRequest, workloadtest.Refused(map[string]string{"state": "not running"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerStatsCommand(h.Cli), "--no-stream", "-a", "--no-trunc"))
		assert.Equal(t, table(header,
			[]string{"abcdef1234567890", "nginx", "0.00%", "0B / 0B", "0.00%", "0B / 0B", "0B / 0B", "0"},
			[]string{"fedcba0987654321", "redis", "--", "--", "--", "--", "--", "--"},
		), h.Out.String())
	})

	t.Run("the containers named, sampled again until interrupted", func(t *testing.T) {
		t.Parallel()

		path := everyone + "/vms/" + vmWeb + "/containers/c-redis/stats"
		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(nginx, redis)).
			On(http.MethodGet, path, http.StatusOK, stats)
		h := harnessOf(t, fake)

		cmd := newShowContainerStatsCommand(h.Cli)
		cmd.every = time.Millisecond

		assert.Equal(t, command.ExitInterrupted, runUntil(t, fake, h, cmd, http.MethodGet, path, 3, "-A", "--format", "{{.Name}} {{.PIDs}}", "redis"))
		assert.True(t, strings.HasPrefix(h.Out.String(), "redis 3\nredis 3\n"), h.Out.String())
	})

	t.Run("what refuses every sample is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/stats", http.StatusUnauthorized, `{"code":"unauthorized"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerStatsCommand(h.Cli), "--no-stream"))
		assert.Equal(t, "Error: unauthorized\n", h.Err.String())
	})

	t.Run("a container that is not there, or containers that cannot be listed, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerStatsCommand(h.Cli), "redis"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerStatsCommand(h.Cli), "--vm", "web"))
		assert.Equal(t, "Error: no such container: redis\nError: no such vm: web\n", h.Err.String())
	})

	t.Run("a bad format is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerStatsCommand(h.Cli), "--format", "{{.Name", "--no-stream"))
		assert.Contains(t, h.Err.String(), "parsing the format")
	})
}

func TestInspectContainersCommand(t *testing.T) {
	t.Parallel()

	t.Run("containers are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx, redis))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectContainersCommand(h.Cli), "nginx", "fedcba"))

		var written []map[string]any
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &written))
		require.Len(t, written, 2)
		assert.Equal(t, "web", written[0]["vm_name"])
		assert.Equal(t, "redis", written[1]["metadata"].(map[string]any)["name"])
		assert.True(t, strings.HasPrefix(h.Out.String(), "[\n    {\n"), "indented as docker's inspect is")
	})

	t.Run("each through a template, and what is not there on its own", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(nginx))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContainersCommand(h.Cli), "-A", "--vm", vmWeb, "-f", "{{.status.id}} {{.spec.ports}}", "nginx", "nothing"))
		assert.Equal(t, "abcdef1234567890 [map[container_port:80 host_port:8080 protocol:tcp]]\n", h.Out.String())
		assert.Equal(t, "Error: no such container: nothing\n", h.Err.String())
	})

	t.Run("nothing found is an empty array", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContainersCommand(h.Cli), "nothing"))
		assert.Equal(t, "[]\n", h.Out.String())
	})

	t.Run("containers that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusUnauthorized, `{"code":"unauthorized"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContainersCommand(h.Cli), "nginx"))
		assert.Equal(t, "Error: unauthorized\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectContainersCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestShowContainerPortsCommand(t *testing.T) {
	t.Parallel()

	published := nginx.With("status.ports", []workloadtest.Object{
		{"container_port": 80, "host_port": 8080, "host_ip": "0.0.0.0", "protocol": "tcp"},
		{"container_port": 80, "host_port": 8080, "host_ip": "::", "protocol": "tcp"},
		{"container_port": 53, "host_port": 5353, "protocol": "udp"},
		{"container_port": 9000, "host_port": 0, "protocol": "tcp"},
	})

	t.Run("every port it publishes", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(published))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerPortsCommand(h.Cli), "nginx"))
		assert.Equal(t, "80/tcp -> 0.0.0.0:8080\n80/tcp -> [::]:8080\n53/udp -> 5353\n", h.Out.String())
	})

	t.Run("what one port is published as", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(published))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerPortsCommand(h.Cli), "nginx", "80"))
		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerPortsCommand(h.Cli), "nginx", "53/udp"))
		assert.Equal(t, "0.0.0.0:8080\n[::]:8080\n5353\n", h.Out.String())
	})

	t.Run("a port it does not publish, or a container docker has not made, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(published, redis))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerPortsCommand(h.Cli), "--vm", vmWeb, "nginx", "9000"))
		assert.Equal(t, console.ExitSuccess, h.Run(newShowContainerPortsCommand(h.Cli), "redis"))
		assert.Equal(t, "Error: no public port \"9000/tcp\" published for nginx\n", h.Err.String())
		assert.Empty(t, h.Out.String())
	})

	t.Run("a container that is not there, or a vm, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerPortsCommand(h.Cli), "nginx"))
		assert.Equal(t, console.ExitFailure, h.Run(newShowContainerPortsCommand(h.Cli), "--vm", "web", "nginx"))
		assert.Equal(t, "Error: no such container: nginx\nError: no such vm: web\n", h.Err.String())
	})

	testRequiresArguments(t, newShowContainerPortsCommand, `"tarhche port" requires at least 1 argument.`)
	testRequiresArguments(t, newShowContainerPortsCommand, `"tarhche port" requires at most 2 arguments.`, "a", "b", "c")
}

func TestConnectContainerCommand(t *testing.T) {
	t.Parallel()

	t.Run("a container is put on a network, reached by its aliases there", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newConnectContainerCommand(h.Cli), "--alias", "web", "--alias", "www", "nginx", "backend"))
		assert.Equal(t, map[string]any{"network": "backend", "aliases": []any{"web", "www"}}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks"))
	})

	t.Run("a container that is not there, or a vm, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newConnectContainerCommand(h.Cli), "-A", "nginx", "backend"))
		assert.Equal(t, console.ExitFailure, h.Run(newConnectContainerCommand(h.Cli), "-A", "--vm", "web", "nginx", "backend"))
		assert.Equal(t, "Error: no such container: nginx\nError: no such vm: web\n", h.Err.String())
	})

	testRequiresArguments(t, newConnectContainerCommand, `"tarhche connect" requires exactly 2 arguments.`, "nginx")
}

func TestDisconnectContainerCommand(t *testing.T) {
	t.Parallel()

	t.Run("a container is taken off a network", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodDelete, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks/backend", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newDisconnectContainerCommand(h.Cli), "nginx", "backend"))

		_, found := fake.Sent(http.MethodDelete, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks/backend")
		assert.True(t, found)
	})

	t.Run("a container that is not there, or a vm, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newDisconnectContainerCommand(h.Cli), "nginx", "backend"))
		assert.Equal(t, console.ExitFailure, h.Run(newDisconnectContainerCommand(h.Cli), "--vm", "web", "nginx", "backend"))
		assert.Equal(t, "Error: no such container: nginx\nError: no such vm: web\n", h.Err.String())
	})

	testRequiresArguments(t, newDisconnectContainerCommand, `"tarhche disconnect" requires exactly 2 arguments.`)
}

func TestContainerEnd(t *testing.T) {
	t.Parallel()

	for status, want := range map[string]int{
		"Exited (0) 1 second ago":          0,
		"Exited (137) 1 second ago":        137,
		"Exited (99999999999999999999) 1s": 0,
		"Up 2 hours":                       0,
	} {
		end := &containerEnd{}
		end.last.Status.DockerStatus = status

		got, err := end.exitStatus()

		require.NoError(t, err, status)
		assert.Equal(t, want, got, status)
	}

	gone := &containerEnd{gone: true}
	got, err := gone.exitStatus()
	require.NoError(t, err)
	assert.Zero(t, got, "a container removed while it was followed ended with nothing to say")
}
