package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

func TestActCommand(t *testing.T) {
	t.Parallel()

	contact := lookup("contact")
	read, unread := actionsByName(contact)[0], actionsByName(contact)[1]

	t.Run("asked of each thing, with its body, writing each", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"PUT /api/dashboard/contact-us/k1/read": answer(`{"uuid":"k1","read_at":"2026-11-22T10:00:00Z"}`),
			"PUT /api/dashboard/contact-us/k2/read": answer(`{"uuid":"k2","read_at":"2026-11-22T10:00:00Z"}`),
		})
		h := harness(t, api)

		status := h.Run(newActCommand(h.Cli, contact, "tarhche contact", read), "k1", "k2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/contact-us/k1/read", "PUT /api/dashboard/contact-us/k2/read"}, api.routes())
		assert.JSONEq(t, `{"read":true}`, api.requests()[0].Body)
		assert.Equal(t, "k1\nk2\n", h.Out.String())
	})

	t.Run("another action of the same route", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/contact-us/k1/read": answer(`{}`)})
		h := harness(t, api)

		status := h.Run(newActCommand(h.Cli, contact, "tarhche contact", unread), "k1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"read":false}`, api.requests()[0].Body)
	})

	t.Run("what it was carried out on, and an error for what it was not", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/contact-us/k1/read": answer(`{}`)})
		h := harness(t, api)

		status := h.Run(newActCommand(h.Cli, contact, "tarhche contact", read), "k9", "k1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "k1\n", h.Out.String())
		assert.Equal(t, "Error: no such contact: k9\n", h.Err.String())
	})

	t.Run("an action without a body sends none", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/exam-attempts/a1/finish": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newActCommand(h.Cli, lookup("exam-attempt"), "tarhche exam-attempt", actionsByName(lookup("exam-attempt"))[0]), "a1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"POST /api/exam-attempts/a1/finish"}, api.routes())
		assert.Empty(t, api.requests()[0].Body)
	})

	t.Run("an action whose route names no key is asked once, of nothing", func(t *testing.T) {
		t.Parallel()

		session := lookup("session")
		api := newFakeAPI(map[string][]reply{"DELETE /api/dashboard/profile/sessions/others": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		others := newActCommand(h.Cli, session, "tarhche profile sessions", actionsByName(session)[0])

		require.Equal(t, console.ExitSuccess, h.Run(others), h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/profile/sessions/others"}, api.routes())
		assert.Empty(t, h.Out.String())
		assert.Equal(t, "tarhche profile sessions rm-others", others.Usage())

		assert.Equal(t, console.ExitUsageError, h.Run(others, "s1"))
	})

	t.Run("an action of one's own and of everybody's", func(t *testing.T) {
		t.Parallel()

		d := resource.Definition{
			Name:  "thing",
			Title: "Things",
			Key:   []string{"uuid"},
			Actions: []resource.Action{
				{Name: "poke", Description: "Poke things", Method: http.MethodPost, Path: "/api/things/{uuid}/poke"},
				{Name: "poke", Description: "Poke things", Method: http.MethodPost, Path: "/api/my/things/{uuid}/poke", Mine: true},
			},
		}
		api := newFakeAPI(map[string][]reply{
			"POST /api/my/things/t1/poke": answered(http.StatusNoContent, ""),
			"POST /api/things/t1/poke":    answered(http.StatusNoContent, ""),
		})
		h := harness(t, api)

		poke := newActCommand(h.Cli, d, "tarhche thing", d.Actions)

		require.Equal(t, console.ExitSuccess, h.Run(poke, "t1"), h.Err.String())
		require.Equal(t, console.ExitSuccess, h.Run(poke, "-A", "t1"), h.Err.String())
		assert.Equal(t, []string{"POST /api/my/things/t1/poke", "POST /api/things/t1/poke"}, api.routes())
		assert.Equal(t, "tarhche thing poke [OPTIONS] UUID [UUID...]", poke.Usage())
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		act := newActCommand(nil, contact, "tarhche contact", read)

		assert.Equal(t, "read", act.Name())
		assert.Equal(t, "Mark one or more messages read", act.Description())
		assert.Equal(t, "tarhche contact read UUID [UUID...]", act.Usage())
	})

	for name, args := range map[string][]string{
		"no key":          nil,
		"a key it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(nil)
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newActCommand(h.Cli, contact, "tarhche contact", read), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("an output that cannot be written", func(t *testing.T) {
		t.Parallel()

		unwritable(t, newFakeAPI(map[string][]reply{"PUT /api/dashboard/contact-us/k1/read": answer(`{}`)}), func(cli *command.Cli) console.Command {
			return newActCommand(cli, contact, "tarhche contact", read)
		}, "k1")
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newActCommand(cli, contact, "tarhche contact", read)
		}, "k1")

		noClient(t, func(cli *command.Cli) console.Command {
			return newActCommand(cli, lookup("session"), "tarhche profile sessions", actionsByName(lookup("session"))[0])
		})
	})
}
