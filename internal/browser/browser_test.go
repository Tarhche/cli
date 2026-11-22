package browser

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpener(t *testing.T) {
	t.Parallel()

	page := "https://backend.example/oauth/authorize?client_id=a&state=b"

	cases := []struct {
		os   string
		name string
		args []string
	}{
		{os: "darwin", name: "open", args: []string{page}},
		{os: "windows", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", page}},
		{os: "linux", name: "xdg-open", args: []string{page}},
		{os: "freebsd", name: "xdg-open", args: []string{page}},
	}

	for _, c := range cases {
		t.Run("a page is opened on "+c.os+" with "+c.name, func(t *testing.T) {
			t.Parallel()

			var started []string
			opener := Opener{OS: c.os, Start: func(name string, args ...string) error {
				started = append([]string{name}, args...)

				return nil
			}}

			require.NoError(t, opener.Open(page))
			assert.Equal(t, append([]string{c.name}, c.args...), started)
		})
	}

	t.Run("an operating system without a way to open one is an error", func(t *testing.T) {
		t.Parallel()

		opener := Opener{OS: "plan9", Start: func(string, ...string) error {
			t.Fatal("nothing is started")

			return nil
		}}

		assert.EqualError(t, opener.Open(page), "opening a browser on plan9 is not supported")
	})

	for _, address := range []string{"file:///etc/passwd", "C:\\Windows\\notepad.exe", "javascript:alert(1)", "https://", "http://%zz"} {
		t.Run("refuses to open "+address, func(t *testing.T) {
			t.Parallel()

			opener := Opener{OS: "windows", Start: func(string, ...string) error {
				t.Fatal("nothing is started")

				return nil
			}}

			assert.ErrorContains(t, opener.Open(address), "is not an absolute http or https url")
		})
	}

	t.Run("a command that cannot be started is an error", func(t *testing.T) {
		t.Parallel()

		opener := Opener{OS: "linux", Start: func(string, ...string) error { return errors.New("no xdg-open") }}

		assert.EqualError(t, opener.Open(page), "no xdg-open")
	})
}

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("refuses what is not a page before anything is started", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, Open("file:///etc/passwd"))
	})
}

func TestCommand(t *testing.T) {
	t.Parallel()

	name, _, err := Command(runtime.GOOS, "https://backend.example")

	require.NoError(t, err)
	assert.NotEmpty(t, name)
}

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("a command is started", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, Start("true"))
	})

	t.Run("a command that is not there is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, Start("tarhche-no-such-command"))
	})
}
