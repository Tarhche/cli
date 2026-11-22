package desktop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
	upgrademock "github.com/Tarhche/cli/internal/upgrade/mock"
)

func TestServices(t *testing.T) {
	t.Parallel()

	t.Run("every service is bound, over the workspace", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		reach := desktop.Reach{Terminals: desktop.Unavailable{}, Forwards: desktop.Unavailable{}, Files: desktop.Unavailable{}}

		services := desktop.Services(context.Background(), w, &mock.Authenticator{}, reach, mock.NewEmitter(), &mock.Chooser{}, &upgrademock.Upgrader{}, func() {})

		instances := make([]any, len(services))
		for i, service := range services {
			instances[i] = service.Instance()
		}

		assert.IsType(t, &desktop.Session{}, instances[0])
		assert.IsType(t, &desktop.Resources{}, instances[1])
		assert.IsType(t, &desktop.Workloads{}, instances[2])
		assert.IsType(t, &desktop.Terminals{}, instances[3])
		assert.IsType(t, &desktop.Forwards{}, instances[4])
		assert.IsType(t, &desktop.Files{}, instances[5])
		assert.IsType(t, &desktop.Updates{}, instances[6])
		assert.Len(t, instances, 7)
	})
}
