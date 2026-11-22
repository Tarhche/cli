package version

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgent(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "tarhche-cli/"+Version+" ("+runtime.GOOS+"; "+runtime.GOARCH+")", UserAgent())
}
