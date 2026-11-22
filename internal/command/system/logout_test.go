package system

import (
	"errors"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

func TestLogoutCommand(t *testing.T) {
	t.Parallel()

	t.Run("forgets the session of the current context and ends it on its estate", func(t *testing.T) {
		t.Parallel()

		seen := &visits{}
		store := mock.NewStore(map[string]credentials.Credentials{"test": signedIn(credentials.MethodOAuth)})
		h := newHarness(t, estate(seen, nil), store)

		require.Equal(t, console.ExitSuccess, h.Run(newLogoutCommand(h.Cli)), h.Err.String())

		assert.Equal(t, "Removing login credentials for test\n", h.Out.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/profile/sessions/session-9"}, seen.asked())

		_, err := store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("logs out of nothing without a word from the estate", func(t *testing.T) {
		t.Parallel()

		seen := &visits{}
		h := newHarness(t, estate(seen, nil), emptyStore())

		require.Equal(t, console.ExitSuccess, h.Run(newLogoutCommand(h.Cli)))

		assert.Equal(t, "Removing login credentials for test\n", h.Out.String())
		assert.Empty(t, seen.asked())
	})

	t.Run("fails for a session that cannot be forgotten", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(operation, _ string) error {
			if operation == "delete" {
				return errors.New("keychain locked")
			}

			return nil
		}

		h := newHarness(t, estate(nil, nil), store)

		assert.Equal(t, console.ExitFailure, h.Run(newLogoutCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), "Error: keychain locked\n")
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())
		h.Cli.Globals.Context = "missing"

		assert.Equal(t, console.ExitFailure, h.Run(newLogoutCommand(h.Cli)))
		assert.Empty(t, h.Out.String())
	})

	t.Run("fails for sessions that cannot be kept anywhere", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.CredentialsStore = "vault"

		assert.Equal(t, console.ExitFailure, h.Run(newLogoutCommand(h.Cli)))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newLogoutCommand(unwritable(h))))
	})

	t.Run("refuses an argument", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())

		assert.Equal(t, console.ExitUsageError, h.Run(newLogoutCommand(h.Cli), "default"))
	})
}
