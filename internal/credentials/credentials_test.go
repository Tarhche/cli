package credentials_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

// liar is a keychain that keeps what it is given and answers something
// else.
type liar struct {
	*mock.Keyring
}

func (l liar) Get(string, string) (string, error) {
	return "something else", nil
}

func configWith(t *testing.T, store string) *config.File {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	cfg.CredentialsStore = store

	return cfg
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("the file when the configuration names it", func(t *testing.T) {
		t.Parallel()

		cfg := configWith(t, credentials.StoreFile)

		store, err := credentials.Open(cfg)
		require.NoError(t, err)

		file, isFile := store.(*credentials.File)
		require.True(t, isFile)
		assert.Equal(t, filepath.Join(cfg.Dir(), credentials.FileName), file.Path())
	})

	t.Run("the keychain when the configuration names it", func(t *testing.T) {
		t.Parallel()

		store, err := credentials.Open(configWith(t, credentials.StoreKeychain))
		require.NoError(t, err)

		assert.IsType(t, &credentials.Keyring{}, store)
	})

	t.Run("the keychain when it works and the configuration names nothing", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()

		store, err := credentials.OpenWith(configWith(t, ""), backend)
		require.NoError(t, err)

		assert.IsType(t, &credentials.Keyring{}, store)
		assert.Empty(t, backend.Users(credentials.Service), "the throwaway entry is removed")
	})

	failures := map[string]func(*mock.Keyring) credentials.Backend{
		"a keychain that cannot keep anything": func(k *mock.Keyring) credentials.Backend {
			k.Fail = func(operation, _, _ string) error {
				if operation == "set" {
					return errors.New("no keychain here")
				}

				return nil
			}

			return k
		},
		"a keychain that cannot read back what it keeps": func(k *mock.Keyring) credentials.Backend {
			k.Fail = func(operation, _, _ string) error {
				if operation == "get" {
					return errors.New("locked")
				}

				return nil
			}

			return k
		},
		"a keychain that answers something else": func(k *mock.Keyring) credentials.Backend {
			return liar{Keyring: k}
		},
	}

	for name, backendOf := range failures {
		t.Run("the file for "+name, func(t *testing.T) {
			t.Parallel()

			keychain := mock.NewKeyring()
			cfg := configWith(t, "")

			store, err := credentials.OpenWith(cfg, backendOf(keychain))
			require.NoError(t, err)

			file, isFile := store.(*credentials.File)
			require.True(t, isFile)
			assert.Equal(t, filepath.Join(cfg.Dir(), credentials.FileName), file.Path())
			assert.Empty(t, keychain.Users(credentials.Service), "nothing is left behind")
		})
	}

	t.Run("a store that is neither is an error", func(t *testing.T) {
		t.Parallel()

		_, err := credentials.Open(configWith(t, "vault"))

		assert.EqualError(t, err, `the credentials store "vault" is neither "keychain" nor "file"`)
	})
}
