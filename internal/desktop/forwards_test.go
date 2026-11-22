package desktop_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
)

func TestForwards(t *testing.T) {
	t.Parallel()

	web := desktop.ForwardRequest{Target: desktop.Target{VM: "v-1", Container: "web"}, Port: 80, LocalPort: 8080}
	ssh := desktop.ForwardRequest{Target: desktop.Target{VM: "v-1"}, Port: 22}

	newForwards := func(t *testing.T) (*desktop.Forwards, *mock.Forwarder, *mock.Emitter, *desktop.Workspace) {
		t.Helper()

		w, _ := newWorkspace(t, nil)
		forwarder := &mock.Forwarder{}
		emitter := mock.NewEmitter()
		t.Cleanup(func() { forwarder.AssertExpectations(t) })

		return desktop.NewForwards(context.Background(), w, forwarder, emitter), forwarder, emitter, w
	}

	// eventually waits until there are as many forwards as want.
	eventually := func(t *testing.T, forwards *desktop.Forwards, want int) {
		t.Helper()

		assert.Eventually(t, func() bool { return len(forwards.List()) == want }, 5*time.Second, 5*time.Millisecond)
	}

	t.Run("a port is forwarded through the context the app talks to, and listed", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, _, w := newForwards(t)
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(mock.NewPortForward("127.0.0.1:8080"), nil).Once()
		forwarder.On("Forward", testify.Anything, w.Cli(), ssh).Return(mock.NewPortForward("127.0.0.1:53111"), nil).Once()

		first, err := forwards.Start(web)
		require.NoError(t, err)

		second, err := forwards.Start(ssh)
		require.NoError(t, err)

		assert.Equal(t, desktop.Forward{ID: "1", Context: testContext, Target: web.Target, Port: 80, Address: "127.0.0.1:8080", StartedAt: first.StartedAt}, first)
		assert.WithinDuration(t, time.Now(), first.StartedAt, time.Minute)
		assert.Equal(t, []desktop.Forward{first, second}, forwards.List())
	})

	t.Run("a forward stopped is forgotten, and not told of", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, emitter, w := newForwards(t)
		forward := mock.NewPortForward("127.0.0.1:8080")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(forward, nil).Once()

		started, err := forwards.Start(web)
		require.NoError(t, err)

		require.NoError(t, forwards.Stop(started.ID))

		eventually(t, forwards, 0)
		assert.True(t, forward.IsStopped())
		assert.True(t, emitter.Quiet())
	})

	t.Run("a forward that stops by itself is forgotten and told of", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, emitter, w := newForwards(t)
		forward := mock.NewPortForward("127.0.0.1:8080")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(forward, nil).Once()

		started, err := forwards.Start(web)
		require.NoError(t, err)

		forward.Fail(errors.New("the VM stopped"))

		assert.Equal(t, mock.Event{Name: desktop.EventForwardStopped, Data: desktop.ForwardStopped{ID: started.ID, Error: "the VM stopped"}}, emitter.Next(t))
		assert.Empty(t, forwards.List())
	})

	t.Run("a forward that stops by itself without saying why is told of", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, emitter, w := newForwards(t)
		forward := mock.NewPortForward("127.0.0.1:8080")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(forward, nil).Once()

		started, err := forwards.Start(web)
		require.NoError(t, err)

		forward.Fail(nil)

		assert.Equal(t, mock.Event{Name: desktop.EventForwardStopped, Data: desktop.ForwardStopped{ID: started.ID}}, emitter.Next(t))
	})

	t.Run("a port that cannot be forwarded is not", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, _, w := newForwards(t)
		failure := errors.New("the port is taken")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(nil, failure).Once()

		_, err := forwards.Start(web)

		assert.ErrorIs(t, err, failure)
		assert.Empty(t, forwards.List())
	})

	t.Run("a forward that is not there cannot be stopped", func(t *testing.T) {
		t.Parallel()

		forwards, _, _, _ := newForwards(t)

		assert.EqualError(t, forwards.Stop("9"), `there is no port forward "9"`)
	})

	t.Run("a forward that cannot be closed says why", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, _, w := newForwards(t)
		forward := mock.NewPortForward("127.0.0.1:8080")
		forward.CloseErr = errors.New("already closed")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(forward, nil).Once()

		started, err := forwards.Start(web)
		require.NoError(t, err)

		assert.ErrorIs(t, forwards.Stop(started.ID), forward.CloseErr)
	})

	t.Run("every forward is stopped as the app quits", func(t *testing.T) {
		t.Parallel()

		forwards, forwarder, emitter, w := newForwards(t)
		first, second := mock.NewPortForward("127.0.0.1:8080"), mock.NewPortForward("127.0.0.1:22")
		second.CloseErr = errors.New("already closed")
		forwarder.On("Forward", testify.Anything, w.Cli(), web).Return(first, nil).Once()
		forwarder.On("Forward", testify.Anything, w.Cli(), ssh).Return(second, nil).Once()

		_, err := forwards.Start(web)
		require.NoError(t, err)
		_, err = forwards.Start(ssh)
		require.NoError(t, err)

		assert.ErrorIs(t, forwards.ServiceShutdown(), second.CloseErr)
		assert.True(t, first.IsStopped())
		assert.True(t, second.IsStopped())
		eventually(t, forwards, 0)
		assert.True(t, emitter.Quiet())
	})

	t.Run("a context whose name cannot be said forwards nothing", func(t *testing.T) {
		t.Parallel()

		forwards := desktop.NewForwards(context.Background(), brokenWorkspace(t), &mock.Forwarder{}, mock.NewEmitter())

		_, err := forwards.Start(web)

		assert.Error(t, err)
	})

	t.Run("a forward is started for as long as the app", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		forwarder := &mock.Forwarder{}
		ctx, cancel := context.WithCancel(context.Background())

		var started context.Context
		forwarder.On("Forward", testify.Anything, testify.AnythingOfType("*command.Cli"), web).Run(func(args testify.Arguments) {
			started = args.Get(0).(context.Context)
			assert.IsType(t, &command.Cli{}, args.Get(1))
		}).Return(mock.NewPortForward("127.0.0.1:8080"), nil)

		_, err := desktop.NewForwards(ctx, w, forwarder, mock.NewEmitter()).Start(web)
		require.NoError(t, err)

		require.NoError(t, started.Err())
		cancel()
		<-started.Done()
	})
}
