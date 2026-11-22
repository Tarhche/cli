// Package mock holds test doubles of what upgrades tarhche.
package mock

import (
	"context"

	testify "github.com/stretchr/testify/mock"

	"github.com/Tarhche/cli/internal/upgrade"
)

// Upgrader reads releases and installs them as a test expects: a test that
// wants to hear how an install goes tells its progress, the third argument,
// what arrived in the call's Run.
type Upgrader struct {
	testify.Mock
}

// Check is called as a test expects.
func (m *Upgrader) Check(ctx context.Context) (upgrade.Check, error) {
	args := m.Called(ctx)

	check, _ := args.Get(0).(upgrade.Check)

	return check, args.Error(1)
}

// Release is called as a test expects.
func (m *Upgrader) Release(ctx context.Context, version string) (upgrade.Release, error) {
	args := m.Called(ctx, version)

	release, _ := args.Get(0).(upgrade.Release)

	return release, args.Error(1)
}

// Target is called as a test expects.
func (m *Upgrader) Target() (string, error) {
	args := m.Called()

	return args.String(0), args.Error(1)
}

// Install is called as a test expects.
func (m *Upgrader) Install(ctx context.Context, release upgrade.Release, progress upgrade.Progress) (upgrade.Installation, error) {
	args := m.Called(ctx, release, progress)

	installation, _ := args.Get(0).(upgrade.Installation)

	return installation, args.Error(1)
}
