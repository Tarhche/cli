package auth

import (
	"os"
	"runtime"
	"strings"
	"time"
)

// Model is what the CLI says the device it signs in from is, beside its
// operating system, so that its sessions are told apart from a browser's.
const Model = "Tarhche CLI"

// localtime is the file the system's time zone is a link to, on the systems
// that have one.
const localtime = "/etc/localtime"

// device is what the CLI says of the machine it signs in from, as a browser
// says it of itself.
type device struct {
	OS       string `json:"os,omitempty"`
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

// device is the machine the CLI runs on.
func (m *Manager) device() device {
	return device{
		OS:       runtime.GOOS,
		Model:    Model,
		Language: language(m.env),
		Timezone: timezone(time.Local, os.Readlink, time.Now()),
	}
}

// language is the language the user's locale names, as a language tag:
// "en_US.UTF-8" is "en-US". The C and POSIX locales name none.
func language(getenv func(string) string) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		locale := getenv(name)
		if len(locale) == 0 {
			continue
		}

		locale, _, _ = strings.Cut(locale, ".")
		locale, _, _ = strings.Cut(locale, "@")

		if locale == "C" || locale == "POSIX" {
			return ""
		}

		return strings.ReplaceAll(locale, "_", "-")
	}

	return ""
}

// timezone is the name of the time zone of local: its own when it has one,
// else the zone the system's localtime links to, else the abbreviation it
// goes by now.
func timezone(local *time.Location, readlink func(string) (string, error), now time.Time) string {
	if name := local.String(); name != "Local" && len(name) > 0 {
		return name
	}

	if target, err := readlink(localtime); err == nil {
		if _, zone, found := strings.Cut(target, "zoneinfo/"); found && len(zone) > 0 {
			return zone
		}
	}

	abbreviation, _ := now.In(local).Zone()

	return abbreviation
}
