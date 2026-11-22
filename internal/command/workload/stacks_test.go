package workload

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/workload"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const stackBlog = "0193a5b2-7c41-7000-8000-00000000000a"

var blog = workloadtest.Stack(stackBlog, "blog", vmWeb, "web")

func TestDeployStackCommand(t *testing.T) {
	t.Parallel()

	compose := "services:\n  web:\n    image: nginx\n"
	created := func(vmCreated bool) workloadtest.Object {
		return workloadtest.Object{"vm": workloadtest.Object{"uuid": vmWeb, "name": "web", "created": vmCreated}, "stack": blog}
	}

	t.Run("a stack is deployed from its file into a vm made for it", func(t *testing.T) {
		t.Parallel()

		file := filepath.Join(t.TempDir(), "compose.yaml")
		require.NoError(t, os.WriteFile(file, []byte(compose), 0o600))

		fake := workloadtest.New().On(http.MethodPost, everyone+"/stacks", http.StatusCreated, created(true))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newDeployStackCommand(h.Cli), "-c", file, "--vm-disk", "8g", "--vm-ingress", "deny", "blog"))
		assert.Equal(t, stackBlog+"\n", h.Out.String())
		assert.Equal(t, "Made Docker VM web for it.\n", h.Err.String())
		assert.Equal(t, map[string]any{
			"name":    "blog",
			"compose": compose,
			"vm":      map[string]any{"resources": map[string]any{"cpus": float64(0), "memory": float64(0), "disk": float64(8 << 30)}, "network": map[string]any{"ingress": "deny"}},
		}, sentJSON(t, fake, http.MethodPost, everyone+"/stacks"))
	})

	t.Run("a stack is deployed from the standard input into the vm --vm names", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPost, everyone+"/stacks", http.StatusCreated, created(false))
		h := harnessOf(t, fake)
		h.In.Reset(compose)

		assert.Equal(t, console.ExitSuccess, h.Run(newDeployStackCommand(h.Cli), "--compose-file", "-", "--vm", "web", "blog"))
		assert.Empty(t, h.Err.String())
		assert.Equal(t, map[string]any{"name": "blog", "compose": compose, "vm_uuid": vmWeb}, sentJSON(t, fake, http.MethodPost, everyone+"/stacks"))
	})

	t.Run("a file is required, and has to be there", func(t *testing.T) {
		t.Parallel()

		h := harnessOf(t, workloadtest.New())

		assert.Equal(t, console.ExitUsageError, h.Run(newDeployStackCommand(h.Cli), "blog"))
		assert.Equal(t, usage("deploy", "a compose file is required: give it with -c"), h.Err.String())

		h = harnessOf(t, workloadtest.New())

		assert.Equal(t, console.ExitFailure, h.Run(newDeployStackCommand(h.Cli), "-c", filepath.Join(t.TempDir(), "missing.yaml"), "blog"))
		assert.Contains(t, h.Err.String(), "no such file or directory")
	})

	t.Run("a vm that is not there, or a file refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodPost, everyone+"/stacks", http.StatusBadRequest, workloadtest.Refused(map[string]string{"compose.services.web.build": "not supported"}))
		h := harnessOf(t, fake)
		h.In.Reset(compose)

		assert.Equal(t, console.ExitFailure, h.Run(newDeployStackCommand(h.Cli), "-c", "-", "--vm", "web", "blog"))
		assert.Equal(t, console.ExitFailure, h.Run(newDeployStackCommand(h.Cli), "-c", "-", "blog"))
		assert.Equal(t, "Error: no such vm: web\nError: compose.services.web.build: not supported\n", h.Err.String())
	})

	testRequiresArguments(t, newDeployStackCommand, `"tarhche deploy" requires exactly 1 argument.`)
}

func TestListStacksCommand(t *testing.T) {
	t.Parallel()

	header := []string{"UUID", "NAME", "SERVICES", "VM", "STATE", "CREATED"}

	t.Run("one's stacks", func(t *testing.T) {
		t.Parallel()

		failed := workloadtest.Stack("s-shop", "shop", vmWeb, "").
			With("status", workloadtest.Object{"state": "failed", "failure": workloadtest.Object{"message": "no capacity"}})
		fake := workloadtest.New().On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(blog, failed))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListStacksCommand(h.Cli)))
		assert.Equal(t, table(header,
			[]string{stackBlog, "blog", "1", "web", "running", "3 hours ago"},
			[]string{"s-shop", "shop", "0", vmWeb, "failed: no capacity", "3 hours ago"},
		), h.Out.String())
	})

	t.Run("anybody's in one vm, with whose they are", func(t *testing.T) {
		t.Parallel()

		owned := blog.With("owner", workloadtest.Object{"uuid": "u1", "username": "sarah"})
		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/stacks", http.StatusOK, workloadtest.Page(owned))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListStacksCommand(h.Cli), "-A", "--vm", vmWeb))
		assert.Equal(t, table(append(header, "OWNER"), []string{stackBlog, "blog", "1", "web", "running", "3 hours ago", "sarah"}), h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, everyone+"/stacks")
		assert.Equal(t, url.Values{"vm": {vmWeb}, "page": {"1"}}, sent.Query)
	})

	t.Run("uuids alone", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(blog))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListStacksCommand(h.Cli), "-q"))
		assert.Equal(t, stackBlog+"\n", h.Out.String())
	})

	t.Run("a vm that is not there, or stacks that cannot be listed, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page()).
			On(http.MethodGet, mine+"/stacks", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListStacksCommand(h.Cli), "--vm", "web"))
		assert.Equal(t, console.ExitFailure, h.Run(newListStacksCommand(h.Cli)))
		assert.Equal(t, "Error: no such vm: web\nError: internal\n", h.Err.String())
	})

	testRequiresArguments(t, newListStacksCommand, `"tarhche ls" accepts no arguments.`, "x")
}

func TestListStackContainersCommand(t *testing.T) {
	t.Parallel()

	t.Run("the containers made of a stack's services", func(t *testing.T) {
		t.Parallel()

		ofBlog := nginx.With("metadata.labels", workloadtest.Object{"workload.stack": stackBlog, "workload.stack.service": "web"})
		seenOfBlog := redis.With("status.labels", workloadtest.Object{"workload.stack": stackBlog, "workload.stack.service": "cache"})
		fake := workloadtest.New().
			On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(blog)).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(ofBlog, seenOfBlog, workloadtest.Container("c-other", "0000", "other", "running", vmWeb, "web")))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListStackContainersCommand(h.Cli), "blog"))
		assert.Equal(t, table([]string{"CONTAINER ID", "NAMES", "SERVICE", "IMAGE", "STATUS", "PORTS"},
			[]string{"abcdef123456", "nginx", "web", "nginx:1.27", "Up 2 hours", "0.0.0.0:8080->80/tcp"},
			[]string{"fedcba098765", "redis", "cache", "redis:7", "Exited (0) 5 minutes ago", ""},
		), h.Out.String())

		sent, _ := fake.Sent(http.MethodGet, mine+"/containers")
		assert.Equal(t, vmWeb, sent.Query.Get("vm"))
	})

	t.Run("ids alone, untruncated, of anybody's", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/stacks/"+stackBlog, http.StatusOK, blog).
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(nginx.With("metadata.labels", workloadtest.Object{"workload.stack": stackBlog})))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListStackContainersCommand(h.Cli), "-A", "-q", "--no-trunc", stackBlog))
		assert.Equal(t, "abcdef1234567890\n", h.Out.String())
	})

	t.Run("a stack that is not there, or containers that cannot be listed, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(blog)).
			On(http.MethodGet, mine+"/containers", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListStackContainersCommand(h.Cli), "shop"))
		assert.Equal(t, console.ExitFailure, h.Run(newListStackContainersCommand(h.Cli), "blog"))
		assert.Equal(t, "Error: no such stack: shop\nError: internal\n", h.Err.String())
	})

	testRequiresArguments(t, newListStackContainersCommand, `"tarhche ps" requires exactly 1 argument.`)
}

func TestRemoveStacksCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveStacksCommand, mine+"/stacks", workloadtest.Page(blog), http.MethodDelete,
		func(uuid string) string { return mine + "/stacks/" + uuid },
		map[string]string{"blog": stackBlog})

	testRequiresArguments(t, newRemoveStacksCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestStartStacksCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStartStacksCommand, everyone+"/stacks", workloadtest.Page(blog), http.MethodPost,
		func(uuid string) string { return everyone + "/stacks/" + uuid + "/start" },
		map[string]string{"blog-a1b2": stackBlog}, "-A")

	testRequiresArguments(t, newStartStacksCommand, `"tarhche start" requires at least 1 argument.`)
}

func TestStopStacksCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newStopStacksCommand, mine+"/stacks", workloadtest.Page(blog), http.MethodPost,
		func(uuid string) string { return mine + "/stacks/" + uuid + "/stop" },
		map[string]string{stackBlog: stackBlog})

	testRequiresArguments(t, newStopStacksCommand, `"tarhche stop" requires at least 1 argument.`)
}

func TestRestartStacksCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRestartStacksCommand, mine+"/stacks", workloadtest.Page(blog), http.MethodPost,
		func(uuid string) string { return mine + "/stacks/" + uuid + "/restart" },
		map[string]string{"blog": stackBlog})

	testRequiresArguments(t, newRestartStacksCommand, `"tarhche restart" requires at least 1 argument.`)
}

func TestInspectStacksCommand(t *testing.T) {
	t.Parallel()

	t.Run("stacks are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(blog))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectStacksCommand(h.Cli), "blog"))

		var written []map[string]any
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &written))
		assert.Equal(t, "web", written[0]["vm_name"])
	})

	t.Run("each through a template, and what is not there on its own", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, everyone+"/stacks", http.StatusOK, workloadtest.Page(blog))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectStacksCommand(h.Cli), "-A", "-f", "{{.metadata.slug}}", "blog", "shop"))
		assert.Equal(t, "blog-a1b2\n", h.Out.String())
		assert.Equal(t, "Error: no such stack: shop\n", h.Err.String())
	})

	t.Run("stacks that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/stacks", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectStacksCommand(h.Cli), "blog"))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectStacksCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestStateOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "running", stateOf(workload.Status{State: "running", Reason: "started"}, nil))
	assert.Equal(t, "failed: no_capacity", stateOf(workload.Status{State: "failed", Reason: "no_capacity"}, nil))
	assert.Equal(t, "failed: pull", stateOf(workload.Status{State: "failed", Reason: "no_capacity"}, &workload.Failure{Message: "pull"}))
	assert.Equal(t, "failed", stateOf(workload.Status{State: "failed"}, &workload.Failure{}))
	assert.True(t, strings.HasPrefix(withOwner(stackTable, true), stackTable))
}
