package workload

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"text/tabwriter"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const (
	mine     = "/api/dashboard/my/workload"
	everyone = "/api/dashboard/workload"

	vmWeb = "0193a5b2-7c41-7000-8000-000000000001"
	vmBox = "0193a5b2-7c41-7000-8000-000000000002"
)

// web is a Docker VM, and box a machine.
var (
	web = workloadtest.VM(vmWeb, "web", true, "running")
	box = workloadtest.VM(vmBox, "box", false, "stopped")
)

// nginx runs in web, and redis is stopped there.
var (
	nginx = workloadtest.Container("c-nginx", "abcdef1234567890", "nginx", "running", vmWeb, "web")
	redis = workloadtest.Container("c-redis", "fedcba0987654321", "redis", "stopped", vmWeb, "web").
		With("status.docker_state", "exited").
		With("status.docker_status", "Exited (0) 5 minutes ago").
		With("status.image", "redis:7").
		With("status.command", "redis-server").
		With("status.ports", []workloadtest.Object{})
)

// harnessOf is a harness whose API is fake.
func harnessOf(t *testing.T, fake *workloadtest.API) *commandtest.Harness {
	t.Helper()

	return commandtest.New(t, fake)
}

// table is rows as the formatter lays a table out.
func table(rows ...[]string) string {
	var b strings.Builder

	w := tabwriter.NewWriter(&b, 10, 1, 3, ' ', 0)
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	_ = w.Flush()

	return b.String()
}

// sentJSON is the JSON body of the last request on method and path.
func sentJSON(t *testing.T, fake *workloadtest.API, method, path string) map[string]any {
	t.Helper()

	sent, found := fake.Sent(method, path)
	require.True(t, found, "it was sent %v", fake.Routes())

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(sent.Body), &body))

	return body
}

// usage is how the CLI reports a command given what it cannot take.
func usage(name, message string) string {
	return fmt.Sprintf("%s: %s\nSee '%s %s --help'.\n", command.Name, message, command.Name, name)
}

// testRequiresArguments checks that a command given too few or too many
// arguments says so and asks nothing of the API.
func testRequiresArguments[C console.Command](t *testing.T, newCommand func(*command.Cli) C, message string, args ...string) {
	t.Helper()

	fake := workloadtest.New()
	h := harnessOf(t, fake)
	cmd := newCommand(h.Cli)

	assert.Equal(t, console.ExitUsageError, h.Run(cmd, args...))
	assert.Equal(t, usage(cmd.Name(), message), h.Err.String())
	assert.Empty(t, fake.Requests())
}

// testAction checks a command that acts on each reference it is given, as
// docker's start and rm do: each is looked for among what list answers and
// acted on by the route of what it names, written as it was given, and one
// that names nothing is said on its own without stopping the others.
func testAction[C console.Command](t *testing.T, newCommand func(*command.Cli) C, list string, listing workloadtest.Object, method string, acted func(uuid string) string, references map[string]string, flags ...string) {
	t.Helper()

	// fakeOf is an API that answers the listing as listed, and web by its
	// uuid, as --vm names it.
	fakeOf := func(status int, listed any) *workloadtest.API {
		return workloadtest.New().
			On(http.MethodGet, list, status, listed).
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web)
	}

	actedOn := func(fake *workloadtest.API) int {
		count := 0

		for _, r := range fake.Requests() {
			for _, uuid := range references {
				if r.Method == method && r.Path == acted(uuid) {
					count++
				}
			}
		}

		return count
	}

	t.Run("acts on each it is given, and says each that names nothing on its own", func(t *testing.T) {
		t.Parallel()

		fake := fakeOf(http.StatusOK, listing)

		var given []string
		for reference, uuid := range references {
			fake.On(method, acted(uuid), http.StatusNoContent, nil)
			given = append(given, reference)
		}

		h := harnessOf(t, fake)

		status := h.Run(newCommand(h.Cli), slices.Concat(flags, given, []string{"nothing"})...)

		assert.Equal(t, console.ExitFailure, status)

		for reference, uuid := range references {
			assert.Contains(t, h.Out.String(), reference+"\n")

			_, found := fake.Sent(method, acted(uuid))
			assert.True(t, found, "%s was acted on: %v", reference, fake.Routes())
		}

		assert.Contains(t, h.Err.String(), "Error: no such ")
		assert.Contains(t, h.Err.String(), ": nothing\n")
	})

	t.Run("what the api refuses for one is said on its own", func(t *testing.T) {
		t.Parallel()

		fake := fakeOf(http.StatusOK, listing)

		var given string
		for reference, uuid := range references {
			fake.On(method, acted(uuid), http.StatusBadRequest, workloadtest.Refused(map[string]string{"state": "it cannot be now"}))
			given = reference

			break
		}

		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCommand(h.Cli), slices.Concat(flags, []string{given})...))
		assert.Empty(t, h.Out.String())
		assert.Equal(t, "Error: state: it cannot be now\n", h.Err.String())
	})

	t.Run("what refuses every one stops it at once", func(t *testing.T) {
		t.Parallel()

		fake := fakeOf(http.StatusOK, listing)

		var given []string
		for reference, uuid := range references {
			fake.On(method, acted(uuid), http.StatusForbidden, `{"code":"forbidden","message":"not yours to touch"}`)
			given = append(given, reference)
		}

		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCommand(h.Cli), slices.Concat(flags, given)...))
		assert.Equal(t, "Error: not yours to touch\n", h.Err.String())
		assert.Equal(t, 1, actedOn(fake), "only the first was asked for")
	})

	t.Run("what cannot be listed is an error", func(t *testing.T) {
		t.Parallel()

		fake := fakeOf(http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCommand(h.Cli), slices.Concat(flags, []string{"anything"})...))
		assert.Equal(t, "Error: internal\n", h.Err.String())
	})
}

// runUntil runs cmd under a context that ends once the API is sent a
// request on method and path for the times-th time, as a test stops what
// follows the API.
func runUntil(t *testing.T, fake *workloadtest.API, h *commandtest.Harness, cmd console.Command, method, path string, times int32, args ...string) int {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var seen atomic.Int32

	fake.OnEach(func(r workloadtest.Request) {
		if r.Method == method && r.Path == path && seen.Add(1) >= times {
			cancel()
		}
	})

	return h.RunContext(ctx, cmd, args...)
}
