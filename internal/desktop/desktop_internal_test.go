package desktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisplayable(t *testing.T) {
	t.Parallel()

	environment := func(variables map[string]string) func(string) string {
		return func(name string) string { return variables[name] }
	}

	cases := []struct {
		name      string
		goos      string
		variables map[string]string
		want      error
	}{
		{name: "macOS always has a display", goos: "darwin"},
		{name: "Windows always has a display", goos: "windows"},
		{name: "X is a display", goos: "linux", variables: map[string]string{"DISPLAY": ":0"}},
		{name: "Wayland is a display", goos: "freebsd", variables: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}},
		{name: "neither is no display", goos: "linux", want: ErrNoDisplay},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, displayable(c.goos, environment(c.variables)))
		})
	}
}
