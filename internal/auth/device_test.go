package auth

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func environment(variables map[string]string) func(string) string {
	return func(name string) string { return variables[name] }
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		variables map[string]string
		language  string
	}{
		{name: "the locale's language and region", variables: map[string]string{"LANG": "en_US.UTF-8"}, language: "en-US"},
		{name: "LC_ALL over the rest", variables: map[string]string{"LC_ALL": "fa_IR.UTF-8", "LC_MESSAGES": "de_DE", "LANG": "en_US"}, language: "fa-IR"},
		{name: "LC_MESSAGES over LANG", variables: map[string]string{"LC_MESSAGES": "de_DE", "LANG": "en_US"}, language: "de-DE"},
		{name: "a modifier left out", variables: map[string]string{"LANG": "de_DE@euro"}, language: "de-DE"},
		{name: "a language alone", variables: map[string]string{"LANG": "fa"}, language: "fa"},
		{name: "none for the C locale", variables: map[string]string{"LANG": "C.UTF-8"}, language: ""},
		{name: "none for the POSIX locale", variables: map[string]string{"LC_ALL": "POSIX"}, language: ""},
		{name: "none without a locale", variables: nil, language: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.language, language(environment(c.variables)))
		})
	}
}

func TestTimezone(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	noLink := func(string) (string, error) { return "", errors.New("no link") }

	t.Run("a zone with a name is called it", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Asia/Tehran", timezone(time.FixedZone("Asia/Tehran", 12600), noLink, now))
	})

	t.Run("the local zone is the one the system's localtime links to", func(t *testing.T) {
		t.Parallel()

		for _, target := range []string{"/usr/share/zoneinfo/Europe/Berlin", "/var/db/timezone/zoneinfo/Europe/Berlin", "../usr/share/zoneinfo/Europe/Berlin"} {
			link := func(path string) (string, error) {
				assert.Equal(t, "/etc/localtime", path)

				return target, nil
			}

			assert.Equal(t, "Europe/Berlin", timezone(time.FixedZone("Local", 3600), link, now), target)
		}
	})

	t.Run("else it is the abbreviation it goes by", func(t *testing.T) {
		t.Parallel()

		elsewhere := func(string) (string, error) { return "/somewhere/else", nil }

		assert.Equal(t, "Local", timezone(time.FixedZone("Local", 3600), noLink, now))
		assert.Equal(t, "Local", timezone(time.FixedZone("Local", 3600), elsewhere, now))
	})
}

func TestManagerDevice(t *testing.T) {
	t.Parallel()

	m := New(nil, nil)
	m.env = environment(map[string]string{"LANG": "fa_IR.UTF-8"})

	got := m.device()

	assert.Equal(t, runtime.GOOS, got.OS)
	assert.Equal(t, Model, got.Model)
	assert.Equal(t, "fa-IR", got.Language)
	assert.NotEmpty(t, got.Timezone)
}
