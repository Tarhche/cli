package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	t.Parallel()

	t.Run("an override is the directory", func(t *testing.T) {
		t.Parallel()

		dir, err := Dir("/somewhere/else")

		require.NoError(t, err)
		assert.Equal(t, "/somewhere/else", dir)
	})

	t.Run("without one it is in the home directory", func(t *testing.T) {
		t.Parallel()

		home, err := os.UserHomeDir()
		require.NoError(t, err)

		dir, err := Dir("")

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, DirName), dir)
	})
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("a directory without a file is an empty configuration", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		f, err := Load(dir)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, FileName), f.Path())
		assert.Equal(t, dir, f.Dir())
		assert.Empty(t, f.Contexts)
		assert.Equal(t, DefaultContextName, f.Current())
	})

	t.Run("a file is read with its contexts named", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		content := `{"current_context":"local","contexts":{"local":{"api":"http://localhost:8000","ingress":"http://localhost:8030"}},"credentials_store":"file"}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o600))

		f, err := Load(dir)

		require.NoError(t, err)
		assert.Equal(t, "local", f.Current())
		assert.Equal(t, "file", f.CredentialsStore)
		assert.Equal(t, Context{Name: "local", API: "http://localhost:8000", Ingress: "http://localhost:8030"}, f.Contexts["local"])
	})

	t.Run("a file that is not json is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte("{"), 0o600))

		_, err := Load(dir)

		assert.ErrorContains(t, err, FileName)
	})

	t.Run("a file that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, FileName), 0o700))

		_, err := Load(dir)

		assert.ErrorContains(t, err, FileName)
	})
}

func TestFileSave(t *testing.T) {
	t.Parallel()

	t.Run("what is saved is loaded again, readable by its owner alone", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "nested", DirName)

		f, err := Load(dir)
		require.NoError(t, err)

		require.NoError(t, f.SetContext(Context{Name: "local", Description: "my machine", API: "http://localhost:8000"}))
		require.NoError(t, f.Use("local"))
		require.NoError(t, f.Save())

		info, err := os.Stat(f.Path())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		loaded, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, "local", loaded.Current())
		assert.Equal(t, Context{Name: "local", Description: "my machine", API: "http://localhost:8000"}, loaded.Contexts["local"])
	})

	t.Run("a directory that cannot be created is an error", func(t *testing.T) {
		t.Parallel()

		blocker := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(blocker, nil, 0o600))

		f := &File{path: filepath.Join(blocker, "dir", FileName)}

		assert.Error(t, f.Save())
	})

	t.Run("a file that cannot be replaced is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, FileName), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, FileName, "inside"), nil, 0o600))

		f := &File{path: filepath.Join(dir, FileName)}

		assert.ErrorContains(t, f.Save(), FileName)
	})
}

func TestFileContext(t *testing.T) {
	t.Parallel()

	t.Run("the default context is there without being created", func(t *testing.T) {
		t.Parallel()

		f := &File{}

		c, err := f.Context(DefaultContextName)

		require.NoError(t, err)
		assert.Equal(t, DefaultContext(), c)
	})

	t.Run("a default context that was changed is the changed one", func(t *testing.T) {
		t.Parallel()

		f := &File{Contexts: map[string]Context{DefaultContextName: {API: "https://elsewhere.example"}}}

		c, err := f.Context(DefaultContextName)

		require.NoError(t, err)
		assert.Equal(t, Context{Name: DefaultContextName, API: "https://elsewhere.example"}, c)
	})

	t.Run("a context that was never created is an error", func(t *testing.T) {
		t.Parallel()

		_, err := (&File{}).Context("missing")

		assert.ErrorIs(t, err, ErrNoSuchContext)
	})
}

func TestFileNames(t *testing.T) {
	t.Parallel()

	t.Run("the default context is among the names, in order", func(t *testing.T) {
		t.Parallel()

		f := &File{Contexts: map[string]Context{"zeta": {}, "alpha": {}}}

		assert.Equal(t, []string{"alpha", DefaultContextName, "zeta"}, f.Names())
	})

	t.Run("a changed default context is named once", func(t *testing.T) {
		t.Parallel()

		f := &File{Contexts: map[string]Context{DefaultContextName: {}}}

		assert.Equal(t, []string{DefaultContextName}, f.Names())
	})
}

func TestFileSetContext(t *testing.T) {
	t.Parallel()

	t.Run("a valid context is kept", func(t *testing.T) {
		t.Parallel()

		f := &File{}

		require.NoError(t, f.SetContext(Context{Name: "prod", API: "https://backend.example"}))

		assert.Equal(t, Context{Name: "prod", API: "https://backend.example"}, f.Contexts["prod"])
	})

	t.Run("an invalid one is refused", func(t *testing.T) {
		t.Parallel()

		f := &File{}

		assert.ErrorIs(t, f.SetContext(Context{Name: "-bad", API: "https://backend.example"}), ErrInvalidContext)
		assert.Empty(t, f.Contexts)
	})
}

func TestFileRemoveContext(t *testing.T) {
	t.Parallel()

	t.Run("a removed current context leaves the default one current", func(t *testing.T) {
		t.Parallel()

		f := &File{CurrentContext: "prod", Contexts: map[string]Context{"prod": {}}}

		require.NoError(t, f.RemoveContext("prod"))

		assert.Empty(t, f.Contexts)
		assert.Equal(t, DefaultContextName, f.Current())
	})

	t.Run("removing another leaves the current one", func(t *testing.T) {
		t.Parallel()

		f := &File{CurrentContext: "prod", Contexts: map[string]Context{"prod": {}, "staging": {}}}

		require.NoError(t, f.RemoveContext("staging"))

		assert.Equal(t, "prod", f.Current())
	})

	t.Run("the default context cannot be removed", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, (&File{}).RemoveContext(DefaultContextName), ErrDefaultContext)
	})

	t.Run("a context that was never created cannot be removed", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, (&File{}).RemoveContext("missing"), ErrNoSuchContext)
	})
}

func TestFileUse(t *testing.T) {
	t.Parallel()

	t.Run("a context is made current", func(t *testing.T) {
		t.Parallel()

		f := &File{Contexts: map[string]Context{"prod": {}}}

		require.NoError(t, f.Use("prod"))

		assert.Equal(t, "prod", f.Current())
		assert.Equal(t, "prod", f.CurrentContext)
	})

	t.Run("the default context is current when none is named", func(t *testing.T) {
		t.Parallel()

		f := &File{CurrentContext: "prod", Contexts: map[string]Context{"prod": {}}}

		require.NoError(t, f.Use(DefaultContextName))

		assert.Empty(t, f.CurrentContext)
		assert.Equal(t, DefaultContextName, f.Current())
	})

	t.Run("a context that was never created cannot be used", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, (&File{}).Use("missing"), ErrNoSuchContext)
	})
}

func TestContextValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		context Context
		valid   bool
	}{
		{name: "an http api", context: Context{Name: "local", API: "http://localhost:8000"}, valid: true},
		{name: "an https api and ingress", context: Context{Name: "a.b+c_d-e", API: "https://backend.example", Ingress: "https://workload.example"}, valid: true},
		{name: "a website", context: Context{Name: "x", API: "https://backend.example", Web: "https://www.example"}, valid: true},
		{name: "an empty name", context: Context{API: "https://backend.example"}},
		{name: "a name starting with a dash", context: Context{Name: "-x", API: "https://backend.example"}},
		{name: "a name with a slash", context: Context{Name: "a/b", API: "https://backend.example"}},
		{name: "no api", context: Context{Name: "x"}},
		{name: "a relative api", context: Context{Name: "x", API: "/api"}},
		{name: "an api of another scheme", context: Context{Name: "x", API: "ftp://backend.example"}},
		{name: "an api that is no url", context: Context{Name: "x", API: "http://%zz"}},
		{name: "a relative ingress", context: Context{Name: "x", API: "https://backend.example", Ingress: "workload"}},
		{name: "a relative website", context: Context{Name: "x", API: "https://backend.example", Web: "www"}},
		{name: "a website of another scheme", context: Context{Name: "x", API: "https://backend.example", Web: "ftp://www.example"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			err := c.context.Validate()

			if c.valid {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidContext)
			}
		})
	}
}

func TestDefaultContext(t *testing.T) {
	t.Parallel()

	c := DefaultContext()

	assert.Equal(t, DefaultContextName, c.Name)
	assert.Equal(t, DefaultAPI, c.API)
	assert.Equal(t, DefaultIngress, c.Ingress)
	assert.Equal(t, DefaultWeb, c.Web)
	assert.NoError(t, c.Validate())
}
