package access

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

// tokenAllowing is an access token whose roles allow permissions.
func tokenAllowing(permissions ...string) string {
	payload, _ := json.Marshal(map[string]any{"sub": "user-1", "permissions": permissions})

	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

type fixedTokens struct {
	token string
	err   error
}

func (f fixedTokens) Token(context.Context) (string, error) { return f.token, f.err }

func (f fixedTokens) Renew(context.Context, string) (string, error) { return f.token, f.err }

func refusal(method, path string) *api.Error {
	return &api.Error{StatusCode: http.StatusForbidden, Method: method, Path: path}
}

func TestRequired(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		method   string
		path     string
		required Requirement
	}{
		{name: "a route a permission is needed for", method: http.MethodGet, path: "/api/dashboard/users", required: Requirement{Known: true, Permission: "users.index"}},
		{name: "a route with a parameter", method: http.MethodDelete, path: "/api/dashboard/users/0190d562-39c0", required: Requirement{Known: true, Permission: "users.delete"}},
		{name: "two parameters", method: http.MethodGet, path: "/api/dashboard/articles/c1/en", required: Requirement{Known: true, Permission: "articles.show"}},
		{name: "a route whose last segment is a word", method: http.MethodPut, path: "/api/dashboard/users/password", required: Requirement{Known: true, Permission: "users.password.update"}},
		{name: "the most specific route of those that match", method: http.MethodDelete, path: "/api/dashboard/profile/sessions/others", required: Requirement{Known: true}},
		{name: "one's own workload", method: http.MethodGet, path: "/api/dashboard/my/workload/vms", required: Requirement{Known: true, Permission: "self.workload.vms.index"}},
		{name: "a route whoever is signed in may ask", method: http.MethodGet, path: "/api/dashboard/profile", required: Requirement{Known: true}},
		{name: "a route the table does not know", method: http.MethodGet, path: "/api/home", required: Requirement{}},
		{name: "a method the route is not served with", method: http.MethodPatch, path: "/api/dashboard/users", required: Requirement{}},
		{name: "a parameter left empty", method: http.MethodDelete, path: "/api/dashboard/users/", required: Requirement{}},
		{name: "a parameter left empty between two words", method: http.MethodGet, path: "/api/dashboard/users//sessions", required: Requirement{}},
		{name: "a path longer than every route", method: http.MethodGet, path: "/api/dashboard/users/u1/sessions/s1/more", required: Requirement{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.required, Required(c.method, c.path))
		})
	}

	t.Run("every route names a known method and a rooted path", func(t *testing.T) {
		t.Parallel()

		for key := range routes {
			method, path, found := strings.Cut(key, " ")

			assert.True(t, found, key)
			assert.Contains(t, []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, method, key)
			assert.True(t, strings.HasPrefix(path, "/"), key)
		}
	})
}

func TestGranted(t *testing.T) {
	t.Parallel()

	granted := GrantedBy(tokenAllowing("users.index", "self.workload.vms.index"))

	t.Run("what the roles allow, as the token says", func(t *testing.T) {
		t.Parallel()

		assert.True(t, granted.Has("users.index"))
		assert.False(t, granted.Has("users.delete"))
		assert.Empty(t, GrantedBy("not a token"))
	})

	t.Run("a route its permission is granted for, or none is needed for", func(t *testing.T) {
		t.Parallel()

		assert.True(t, granted.Allows(http.MethodGet, "/api/dashboard/users"))
		assert.True(t, granted.Allows(http.MethodGet, "/api/dashboard/my/workload/vms"))
		assert.False(t, granted.Allows(http.MethodDelete, "/api/dashboard/users/u1"))
		assert.True(t, granted.Allows(http.MethodGet, "/api/dashboard/profile"), "whoever is signed in")
		assert.True(t, granted.Allows(http.MethodGet, "/api/home"), "nobody needs a session for it")
	})

	t.Run("a stream of one's own vm under its self permission or the anybody's, of anybody's under the anybody's alone", func(t *testing.T) {
		t.Parallel()

		owner := GrantedBy(tokenAllowing("self.workload.vms.exec"))
		administrator := GrantedBy(tokenAllowing("workload.vms.exec"))
		nobody := GrantedBy(tokenAllowing("self.workload.vms.attach", "workload.vms.forward"))

		assert.True(t, owner.AllowsStream(stream.ActionExec, workload.Mine))
		assert.False(t, owner.AllowsStream(stream.ActionExec, workload.Everyone))
		assert.True(t, administrator.AllowsStream(stream.ActionExec, workload.Mine))
		assert.True(t, administrator.AllowsStream(stream.ActionExec, workload.Everyone))
		assert.False(t, nobody.AllowsStream(stream.ActionExec, workload.Mine))
		assert.False(t, nobody.AllowsStream(stream.ActionExec, workload.Everyone))
		assert.True(t, nobody.AllowsStream("logs", workload.Everyone), "an action of no vm's")
	})
}

func TestDeniedError(t *testing.T) {
	t.Parallel()

	refused := refusal(http.MethodGet, "/api/dashboard/users")

	assert.EqualError(t, &DeniedError{Permission: "users.index", Refused: refused}, `permission denied: this needs the "users.index" permission, which none of your roles gives you`)
	assert.EqualError(t, &DeniedError{Permission: "users.index", Here: true, Refused: refused}, `permission denied: your roles do not allow "users.index" on this`)
	assert.EqualError(t, &DeniedError{Here: true, Refused: refused}, "permission denied: your roles do not allow this")
	assert.True(t, api.IsForbidden(&DeniedError{Refused: refused}), "a denial is still a refusal")

	streamRefused := &stream.Error{Stream: "vms/u/exec", Kind: stream.KindVMs, Action: stream.ActionExec, StatusCode: http.StatusForbidden}
	denied := &DeniedError{Permission: "self.workload.vms.exec", Refused: streamRefused}

	assert.EqualError(t, denied, `permission denied: this needs the "self.workload.vms.exec" permission, which none of your roles gives you`)
	assert.True(t, stream.IsForbidden(denied), "a stream's denial is still the ingress's refusal")
	assert.False(t, api.IsForbidden(denied))
	assert.NoError(t, (&DeniedError{}).Unwrap(), "a denial with no refusal unwraps to nothing")
}

func TestBlockedError(t *testing.T) {
	t.Parallel()

	blocked := &BlockedError{Refused: refusal(http.MethodGet, "/api/dashboard/users")}

	assert.EqualError(t, blocked, ErrBlocked.Error())
	assert.ErrorIs(t, blocked, ErrBlocked)
	assert.True(t, api.IsForbidden(blocked), "a blocked request is still a refusal")
}

func TestExplainer(t *testing.T) {
	t.Parallel()

	forbidden := &api.Error{StatusCode: http.StatusForbidden}

	probing := func(answer error, probed *bool) func(context.Context) error {
		return func(ctx context.Context) error {
			*probed = true

			return answer
		}
	}

	t.Run("a permission the roles do not give is what is denied", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(fixedTokens{token: tokenAllowing("users.show")}, probing(nil, &probed))

		err := explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/dashboard/users"))

		var denied *DeniedError
		require.ErrorAs(t, err, &denied)
		assert.Equal(t, "users.index", denied.Permission)
		assert.False(t, denied.Here)
		assert.True(t, probed, "whether the account is blocked is asked first")
	})

	t.Run("an account refused even what every session may read is blocked, whatever its roles", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(fixedTokens{token: tokenAllowing()}, probing(forbidden, &probed))

		err := explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/dashboard/users"))

		assert.ErrorIs(t, err, ErrBlocked)
		assert.True(t, probed)
	})

	t.Run("a route whoever is signed in may ask, refused, is a blocked account", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(fixedTokens{token: tokenAllowing()}, probing(forbidden, &probed))

		assert.ErrorIs(t, explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/dashboard/profile/roles")), ErrBlocked)
	})

	t.Run("a permission the roles give, refused, is disallowed on what it was asked of", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(fixedTokens{token: tokenAllowing("users.delete")}, probing(nil, &probed))

		err := explainer.Explain(context.Background(), refusal(http.MethodDelete, "/api/dashboard/users/u1"))

		var denied *DeniedError
		require.ErrorAs(t, err, &denied)
		assert.Equal(t, "users.delete", denied.Permission)
		assert.True(t, denied.Here)
	})

	t.Run("a token that cannot be had says nothing of the roles", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(fixedTokens{err: errors.New("no session")}, probing(nil, &probed))

		var denied *DeniedError
		require.ErrorAs(t, explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/dashboard/users")), &denied)
		assert.True(t, denied.Here)
		assert.True(t, probed)
	})

	t.Run("a route nobody knows is denied as the api said", func(t *testing.T) {
		t.Parallel()

		var probed bool
		explainer := NewExplainer(nil, probing(nil, &probed))

		var denied *DeniedError
		require.ErrorAs(t, explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/unknown")), &denied)
		assert.Empty(t, denied.Permission)
	})

	t.Run("the probe's own refusal is not explained again", func(t *testing.T) {
		t.Parallel()

		explainer := NewExplainer(fixedTokens{token: tokenAllowing("users.index")}, func(ctx context.Context) error {
			return ctx.Err()
		})

		var denied *DeniedError
		require.ErrorAs(t, explainer.Explain(context.Background(), refusal(http.MethodGet, "/api/dashboard/users")), &denied)
	})
}
