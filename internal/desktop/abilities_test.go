package desktop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/desktop"
)

func TestAbilities(t *testing.T) {
	t.Parallel()

	abilitiesOf := func(t *testing.T, permissions ...string) desktop.Abilities {
		t.Helper()

		abilities, err := desktop.NewWorkloads(newWorkspaceGranting(t, nil, permissions...)).Abilities(context.Background())
		require.NoError(t, err)

		return abilities
	}

	t.Run("a session whose roles allow nothing may do nothing", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, desktop.Abilities{}, abilitiesOf(t))
	})

	t.Run("a session whose roles allow all of one's own workload may do all of it", func(t *testing.T) {
		t.Parallel()

		abilities := abilitiesOf(t,
			"self.workload.vms.index", "self.workload.vms.show", "workload.vms.create", "self.workload.vms.manage", "self.workload.vms.delete", "self.workload.vms.logs",
			"self.workload.vms.attach", "self.workload.vms.exec", "self.workload.vms.forward",
			"self.workload.snapshots.index", "self.workload.snapshots.show", "workload.snapshots.create", "self.workload.snapshots.update", "self.workload.snapshots.delete",
			"self.workload.containers.index", "self.workload.containers.show", "workload.containers.create", "self.workload.containers.manage", "self.workload.containers.delete", "self.workload.containers.logs",
			"self.workload.stacks.index", "self.workload.stacks.show", "workload.stacks.create", "self.workload.stacks.manage", "self.workload.stacks.delete",
		)

		assert.Equal(t, desktop.Abilities{
			VMs:        desktop.KindAbilities{List: true, Show: true, Create: true, Start: true, Stop: true, Restart: true, Restore: true, Delete: true, Logs: true, Usage: true, Attach: true, Exec: true, Forward: true},
			Containers: desktop.KindAbilities{List: true, Show: true, Create: true, Start: true, Stop: true, Restart: true, Delete: true, Logs: true, Usage: true, Exec: true},
			Images:     desktop.KindAbilities{List: true, Create: true, Delete: true},
			Networks:   desktop.KindAbilities{List: true, Create: true, Delete: true},
			Volumes:    desktop.KindAbilities{List: true, Create: true, Delete: true},
			Stacks:     desktop.KindAbilities{List: true, Show: true, Create: true, Start: true, Stop: true, Restart: true, Delete: true},
			Snapshots:  desktop.KindAbilities{List: true, Show: true, Create: true, Update: true, Delete: true},
		}, abilities)
	})

	t.Run("a Docker VM's images, networks and volumes are listed only with one's VMs to choose it among", func(t *testing.T) {
		t.Parallel()

		withoutVMs := abilitiesOf(t, "self.workload.containers.index")
		withVMs := abilitiesOf(t, "self.workload.containers.index", "self.workload.vms.index")

		assert.True(t, withoutVMs.Containers.List)
		assert.False(t, withoutVMs.Images.List)
		assert.False(t, withoutVMs.Networks.List)
		assert.False(t, withoutVMs.Volumes.List)
		assert.True(t, withVMs.Images.List)
		assert.True(t, withVMs.Networks.List)
		assert.True(t, withVMs.Volumes.List)
	})

	t.Run("a VM is restored only with its snapshots to choose among", func(t *testing.T) {
		t.Parallel()

		managing := abilitiesOf(t, "self.workload.vms.manage")

		assert.True(t, managing.VMs.Start)
		assert.True(t, managing.VMs.Stop)
		assert.True(t, managing.VMs.Restart)
		assert.False(t, managing.VMs.Restore)
		assert.True(t, abilitiesOf(t, "self.workload.vms.manage", "self.workload.snapshots.index").VMs.Restore)
	})

	t.Run("one's own are created under anybody's permissions", func(t *testing.T) {
		t.Parallel()

		abilities := abilitiesOf(t, "self.workload.vms.create", "self.workload.containers.create")

		assert.False(t, abilities.VMs.Create)
		assert.False(t, abilities.Containers.Create)
	})

	t.Run("a stream is opened under one's own permission or anybody's", func(t *testing.T) {
		t.Parallel()

		abilities := abilitiesOf(t, "workload.vms.attach", "self.workload.vms.exec")

		assert.True(t, abilities.VMs.Attach)
		assert.True(t, abilities.VMs.Exec)
		assert.False(t, abilities.VMs.Forward)
		assert.True(t, abilities.Containers.Exec, "a container is reached through its Docker VM's exec")
		assert.False(t, abilities.Containers.Attach)
		assert.True(t, abilitiesOf(t, "workload.vms.forward").VMs.Forward)
	})

	t.Run("a session that cannot be had may do nothing", func(t *testing.T) {
		t.Parallel()

		_, err := desktop.NewWorkloads(workspaceFailing(t, nil, auth.ErrSessionEnded)).Abilities(context.Background())

		assert.ErrorIs(t, err, auth.ErrSessionEnded)
	})
}
