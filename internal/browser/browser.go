// Package browser opens pages in the user's web browser, with the command
// their operating system opens one with.
package browser

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// Open opens address, an absolute http or https url, in the user's default
// browser.
func Open(address string) error {
	return Opener{OS: runtime.GOOS, Start: Start}.Open(address)
}

// Opener opens pages with the command an operating system opens them with.
type Opener struct {
	// OS is the operating system, as runtime.GOOS names it.
	OS string

	// Start starts a command without waiting for it to end.
	Start func(name string, args ...string) error
}

// Open opens address, an absolute http or https url: nothing else is handed
// to a command that would open a file or run a program as readily as a page.
func (o Opener) Open(address string) error {
	parsed, err := url.Parse(address)
	if err != nil || len(parsed.Host) == 0 || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%q is not an absolute http or https url", address)
	}

	name, args, err := Command(o.OS, address)
	if err != nil {
		return err
	}

	return o.Start(name, args...)
}

// Command is the command that opens address on an operating system: open on
// macOS, the URL protocol handler on Windows, and xdg-open elsewhere.
func Command(goos, address string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{address}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", address}, nil
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly", "illumos", "solaris":
		return "xdg-open", []string{address}, nil
	default:
		return "", nil, fmt.Errorf("opening a browser on %s is not supported", goos)
	}
}

// Start starts a command and lets it run, waiting for it in the background
// so that it does not linger once it ends: xdg-open may stay for as long as
// the browser it started does.
func Start(name string, args ...string) error {
	command := exec.CommandContext(context.Background(), name, args...)
	if err := command.Start(); err != nil {
		return err
	}

	go func() { _ = command.Wait() }()

	return nil
}
