package credentials_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

// forgetful is a keychain whose deletions always succeed, whether or not
// there was anything to delete.
type forgetful struct {
	*mock.Keyring
	deletes *int
}

func (f forgetful) Delete(string, string) error {
	*f.deletes++

	return nil
}

func session(accessTokenSize int) credentials.Credentials {
	return credentials.Credentials{
		Method:        credentials.MethodOAuth,
		AccessToken:   strings.Repeat("a", accessTokenSize),
		RefreshToken:  "refresh",
		ClientID:      "client",
		TokenEndpoint: "https://backend.example/oauth/token",
	}
}

func failOn(operation, user string) func(string, string, string) error {
	return func(asked, _, of string) error {
		if asked == operation && of == user {
			return errors.New(operation + " " + user + " failed")
		}

		return nil
	}
}

func TestKeyring(t *testing.T) {
	t.Parallel()

	t.Run("a context nobody signed in to has no credentials", func(t *testing.T) {
		t.Parallel()

		_, err := credentials.NewKeyring(mock.NewKeyring()).Get("prod")

		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("credentials an entry takes are kept in the context's own", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		require.NoError(t, store.Set("prod", session(100)))

		got, err := store.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, session(100), got)
		assert.Equal(t, []string{"prod"}, backend.Users(credentials.Service))
	})

	t.Run("larger credentials are split across entries no larger than one takes, and joined back", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		require.NoError(t, store.Set("prod", session(6000)))

		got, err := store.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, session(6000), got)
		assert.Equal(t, []string{"prod", "prod:1", "prod:2"}, backend.Users(credentials.Service))
	})

	t.Run("what is left over of larger credentials is forgotten", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		require.NoError(t, store.Set("prod", session(9000)))
		require.NoError(t, store.Set("prod", session(4000)))
		assert.Equal(t, []string{"prod", "prod:1"}, backend.Users(credentials.Service))

		require.NoError(t, store.Set("prod", session(10)))
		assert.Equal(t, []string{"prod"}, backend.Users(credentials.Service))

		got, err := store.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, session(10), got)
	})

	t.Run("a context whose name looks like another's part is a context of its own", func(t *testing.T) {
		t.Parallel()

		store := credentials.NewKeyring(mock.NewKeyring())

		require.NoError(t, store.Set("prod", session(6000)))
		require.NoError(t, store.Set("prod.1", session(5)))

		prod, err := store.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, session(6000), prod)

		other, err := store.Get("prod.1")
		require.NoError(t, err)
		assert.Equal(t, session(5), other)
	})

	t.Run("credentials are never split inside a character", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		wide := session(1)
		wide.ClientID = strings.Repeat("ب", 3000)

		require.NoError(t, store.Set("prod", wide))

		for _, user := range backend.Users(credentials.Service) {
			entry, err := backend.Get(credentials.Service, user)
			require.NoError(t, err)
			assert.True(t, utf8.ValidString(entry), user)
		}

		got, err := store.Get("prod")
		require.NoError(t, err)
		assert.Equal(t, wide, got)
	})

	t.Run("credentials too large for any number of entries are refused", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()

		err := credentials.NewKeyring(backend).Set("prod", session(3_000_000))

		assert.ErrorContains(t, err, "more than the keychain can be given")
		assert.Empty(t, backend.Users(credentials.Service))
	})

	t.Run("deleting forgets every entry, and deleting nothing is no error", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		require.NoError(t, store.Set("prod", session(6000)))
		require.NoError(t, store.Set("local", session(10)))

		require.NoError(t, store.Delete("prod"))
		require.NoError(t, store.Delete("prod"))
		require.NoError(t, store.Delete("never"))

		assert.Equal(t, []string{"local"}, backend.Users(credentials.Service))

		_, err := store.Get("prod")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("credentials missing a part say so", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		store := credentials.NewKeyring(backend)

		require.NoError(t, store.Set("prod", session(6000)))
		require.NoError(t, backend.Delete(credentials.Service, "prod:2"))

		_, err := store.Get("prod")

		assert.EqualError(t, err, `the credentials of "prod" are missing part 3 of 3 in the keychain: log in again`)
	})

	for name, head := range map[string]string{
		"not json":              "{",
		"no count":              "tarhche-cli-parts:{}",
		"a count that is none":  "tarhche-cli-parts:x:{}",
		"a count of one part":   "tarhche-cli-parts:1:{}",
		"a count of too many":   "tarhche-cli-parts:5000:{}",
		"a count without parts": "tarhche-cli-parts:-2:{}",
	} {
		t.Run("credentials that are "+name+" are an error", func(t *testing.T) {
			t.Parallel()

			backend := mock.NewKeyring()
			require.NoError(t, backend.Set(credentials.Service, "prod", head))

			_, err := credentials.NewKeyring(backend).Get("prod")

			assert.ErrorContains(t, err, `reading the credentials of "prod" from the keychain`)
		})
	}

	failures := []struct {
		name      string
		size      int
		fail      func(string, string, string) error
		operation func(*credentials.Keyring) error
		said      string
	}{
		{
			name:      "reading the first entry",
			fail:      failOn("get", "prod"),
			operation: func(k *credentials.Keyring) error { _, err := k.Get("prod"); return err },
			said:      `reading the credentials of "prod" from the keychain: get prod failed`,
		},
		{
			name:      "reading a part",
			size:      6000,
			fail:      failOn("get", "prod:1"),
			operation: func(k *credentials.Keyring) error { _, err := k.Get("prod"); return err },
			said:      `reading the credentials of "prod" from the keychain: get prod:1 failed`,
		},
		{
			name:      "keeping the first entry",
			fail:      failOn("set", "prod"),
			operation: func(k *credentials.Keyring) error { return k.Set("prod", session(10)) },
			said:      `keeping the credentials of "prod" in the keychain: set prod failed`,
		},
		{
			name:      "keeping the first entry of larger credentials",
			fail:      failOn("set", "prod"),
			operation: func(k *credentials.Keyring) error { return k.Set("prod", session(6000)) },
			said:      `keeping the credentials of "prod" in the keychain: set prod failed`,
		},
		{
			name:      "keeping a part",
			fail:      failOn("set", "prod:2"),
			operation: func(k *credentials.Keyring) error { return k.Set("prod", session(6000)) },
			said:      `keeping the credentials of "prod" in the keychain: set prod:2 failed`,
		},
		{
			name:      "forgetting what is left over",
			size:      6000,
			fail:      failOn("delete", "prod:1"),
			operation: func(k *credentials.Keyring) error { return k.Set("prod", session(10)) },
			said:      `keeping the credentials of "prod" in the keychain: delete prod:1 failed`,
		},
		{
			name:      "deleting a part",
			size:      6000,
			fail:      failOn("delete", "prod:2"),
			operation: func(k *credentials.Keyring) error { return k.Delete("prod") },
			said:      `removing the credentials of "prod" from the keychain: delete prod:2 failed`,
		},
		{
			name:      "deleting the first entry",
			size:      10,
			fail:      failOn("delete", "prod"),
			operation: func(k *credentials.Keyring) error { return k.Delete("prod") },
			said:      `removing the credentials of "prod" from the keychain: delete prod failed`,
		},
	}

	for _, f := range failures {
		t.Run("a keychain failing "+f.name+" is an error", func(t *testing.T) {
			t.Parallel()

			backend := mock.NewKeyring()
			store := credentials.NewKeyring(backend)

			if f.size > 0 {
				require.NoError(t, store.Set("prod", session(f.size)))
			}

			backend.Fail = f.fail

			assert.EqualError(t, f.operation(store), f.said)
		})
	}

	t.Run("a keychain that never says an entry is missing is not asked forever", func(t *testing.T) {
		t.Parallel()

		backend := forgetful{Keyring: mock.NewKeyring(), deletes: new(int)}

		assert.NoError(t, credentials.NewKeyring(backend).Delete("prod"))
		assert.Greater(t, *backend.deletes, 2)
	})

	t.Run("a keychain refusing an entry as too large is an error", func(t *testing.T) {
		t.Parallel()

		backend := mock.NewKeyring()
		backend.MaxSize = 100

		err := credentials.NewKeyring(backend).Set("prod", session(1000))

		assert.ErrorIs(t, err, keyring.ErrSetDataTooBig)
	})
}

func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("a secret is cut at the size", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"abcd", "efgh", "ij"}, credentials.Split("abcdefghij", 4))
	})

	t.Run("a secret is cut before a character rather than inside it", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"ab", "پc"}, credentials.Split("abپc", 3))
	})

	t.Run("bytes that are no characters are cut at the size", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"\x80\x80", "\x80"}, credentials.Split("\x80\x80\x80", 2))
	})
}
