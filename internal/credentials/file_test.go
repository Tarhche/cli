package credentials_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/credentials"
)

func TestFile(t *testing.T) {
	t.Parallel()

	t.Run("it is the credentials file in the directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		assert.Equal(t, filepath.Join(dir, "credentials.json"), credentials.NewFile(dir).Path())
	})

	t.Run("a context nobody signed in to has no credentials, file or not", func(t *testing.T) {
		t.Parallel()

		store := credentials.NewFile(t.TempDir())

		_, err := store.Get("prod")
		assert.ErrorIs(t, err, credentials.ErrNotFound)

		require.NoError(t, store.Set("local", session(5)))

		_, err = store.Get("prod")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("credentials are kept by context, readable by the owner alone", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "not", "yet")
		store := credentials.NewFile(dir)

		require.NoError(t, store.Set("prod", session(10)))
		require.NoError(t, store.Set("local", session(20)))

		got, err := credentials.NewFile(dir).Get("prod")
		require.NoError(t, err)
		assert.Equal(t, session(10), got)

		info, err := os.Stat(store.Path())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		content, err := os.ReadFile(store.Path())
		require.NoError(t, err)

		var kept map[string]credentials.Credentials
		require.NoError(t, json.Unmarshal(content, &kept))
		assert.Equal(t, map[string]credentials.Credentials{"prod": session(10), "local": session(20)}, kept)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "nothing is left behind but the file")
	})

	t.Run("deleting forgets a context's credentials alone, and deleting nothing is no error", func(t *testing.T) {
		t.Parallel()

		store := credentials.NewFile(t.TempDir())

		require.NoError(t, store.Delete("prod"))

		require.NoError(t, store.Set("prod", session(10)))
		require.NoError(t, store.Set("local", session(20)))

		require.NoError(t, store.Delete("prod"))
		require.NoError(t, store.Delete("prod"))

		_, err := store.Get("prod")
		assert.ErrorIs(t, err, credentials.ErrNotFound)

		local, err := store.Get("local")
		require.NoError(t, err)
		assert.Equal(t, session(20), local)
	})

	t.Run("a file of nothing keeps nothing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, credentials.FileName), []byte("null"), 0o600))

		store := credentials.NewFile(dir)

		_, err := store.Get("prod")
		assert.ErrorIs(t, err, credentials.ErrNotFound)

		require.NoError(t, store.Set("prod", session(10)))
	})

	t.Run("credentials kept at once are all kept", func(t *testing.T) {
		t.Parallel()

		store := credentials.NewFile(t.TempDir())

		var wait sync.WaitGroup
		for i := range 10 {
			wait.Go(func() {
				assert.NoError(t, store.Set(fmt.Sprintf("context-%d", i), session(i)))
			})
		}

		wait.Wait()

		for i := range 10 {
			got, err := store.Get(fmt.Sprintf("context-%d", i))
			require.NoError(t, err)
			assert.Equal(t, session(i), got)
		}
	})

	t.Run("a file that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		for name, prepare := range map[string]func(path string) error{
			"not json":    func(path string) error { return os.WriteFile(path, []byte("{"), 0o600) },
			"a directory": func(path string) error { return os.Mkdir(path, 0o700) },
		} {
			dir := t.TempDir()
			require.NoError(t, prepare(filepath.Join(dir, credentials.FileName)), name)

			store := credentials.NewFile(dir)

			_, err := store.Get("prod")
			assert.Error(t, err, name)
			assert.NotErrorIs(t, err, credentials.ErrNotFound, name)

			assert.Error(t, store.Set("prod", session(1)), name)
			assert.Error(t, store.Delete("prod"), name)
		}
	})

	t.Run("a directory that cannot be made is an error", func(t *testing.T) {
		t.Parallel()

		dangling := filepath.Join(t.TempDir(), "dir")
		require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "nowhere", "dir"), dangling))

		err := credentials.NewFile(dangling).Set("prod", session(1))

		assert.ErrorContains(t, err, "creating")
	})
}
