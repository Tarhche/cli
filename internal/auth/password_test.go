package auth

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

func TestManagerLoginWithPassword(t *testing.T) {
	t.Parallel()

	t.Run("a password signs in, with what the device says of itself, and the session is kept", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))
		h.env = func(name string) string {
			return map[string]string{"LANG": "fa_IR.UTF-8"}[name]
		}

		require.NoError(t, h.LoginWithPassword(context.Background(), "test", "jane", "secret"))

		kept := h.stored(t)
		assert.Equal(t, credentials.MethodPassword, kept.Method)
		assert.Equal(t, "jane", claimsOf(kept.AccessToken).Subject)
		assert.NotEmpty(t, kept.RefreshToken)
		assert.Empty(t, kept.ClientID)
		assert.Empty(t, kept.TokenEndpoint)

		logins := e.loginRequests()
		require.Len(t, logins, 1)
		assert.Equal(t, "jane", logins[0]["identity"])
		assert.Equal(t, "secret", logins[0]["password"])

		device, isObject := logins[0]["device"].(map[string]any)
		require.True(t, isObject)
		assert.Equal(t, runtime.GOOS, device["os"])
		assert.Equal(t, Model, device["model"])
		assert.Equal(t, "fa-IR", device["language"])
		assert.NotEmpty(t, device["timezone"])
	})

	t.Run("a password the estate refuses is said, and nothing is kept", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		h := newHarness(t, newEstate(t), store)

		err := h.LoginWithPassword(context.Background(), "test", "jane", "wrong")

		assert.EqualError(t, err, "login refused: identity: invalid identity or password")

		_, err = store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("an estate failing to sign in is an error", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t, func(e *estate) { e.fail["POST /api/auth/login"] = failure{status: http.StatusInternalServerError} })
		h := newHarness(t, e, mock.NewStore(nil))

		err := h.LoginWithPassword(context.Background(), "test", "jane", "secret")

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "login refused")
	})

	t.Run("an estate signing in without a token is an error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t, func(e *estate) { e.noAccessToken = true }), mock.NewStore(nil))

		assert.ErrorIs(t, h.LoginWithPassword(context.Background(), "test", "jane", "secret"), errNoToken)
	})

	t.Run("a context that does not exist is an error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		assert.ErrorIs(t, h.LoginWithPassword(context.Background(), "missing", "jane", "secret"), config.ErrNoSuchContext)
	})

	t.Run("an api that is no url is an error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))
		h.config.Contexts["broken"] = config.Context{Name: "broken", API: "not a url"}

		assert.Error(t, h.LoginWithPassword(context.Background(), "broken", "jane", "secret"))
	})

	t.Run("a session that cannot be kept is an error", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(string, string) error { return errors.New("keychain locked") }

		h := newHarness(t, newEstate(t), store)

		assert.EqualError(t, h.LoginWithPassword(context.Background(), "test", "jane", "secret"), "keychain locked")
	})

	t.Run("a login given up on while another is under way stops", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		release, err := h.hold(context.Background(), "test")
		require.NoError(t, err)
		t.Cleanup(release)

		assert.ErrorIs(t, h.LoginWithPassword(canceled(), "test", "jane", "secret"), context.Canceled)
	})
}
