// Package version is what this binary was built as.
package version

import (
	"fmt"
	"runtime"
)

// Version is set when the binary is built:
//
//	go build -ldflags "-X github.com/Tarhche/cli/internal/version.Version=v1.2.3"
var Version = "dev"

// UserAgent is how the CLI introduces itself to the servers it talks to.
func UserAgent() string {
	return fmt.Sprintf("tarhche-cli/%s (%s; %s)", Version, runtime.GOOS, runtime.GOARCH)
}
