package workload

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	groups := map[string][]string{
		"container": {"ls", "ps", "list", "run", "start", "stop", "restart", "rm", "logs", "stats", "inspect", "port", "connect", "disconnect"},
		"image":     {"ls", "list", "pull", "rm", "inspect"},
		"network":   {"ls", "list", "create", "rm", "inspect", "connect", "disconnect"},
		"volume":    {"ls", "list", "create", "rm", "inspect"},
		"stack":     {"deploy", "ls", "list", "ps", "rm", "start", "stop", "restart", "inspect"},
		"vm":        {"ls", "list", "create", "rm", "start", "stop", "restart", "inspect", "logs", "stats", "update", "restore", "dependents"},
		"snapshot":  {"ls", "list", "create", "rm", "inspect", "rename"},
	}

	shortcuts := []string{"ps", "run", "start", "stop", "restart", "rm", "logs", "stats", "port", "images", "pull", "rmi", "inspect"}

	// run runs args through a console with the workload registered on it.
	run := func(t *testing.T, fake *workloadtest.API, args ...string) (int, string, string) {
		t.Helper()

		h := harnessOf(t, fake)
		c := console.New(command.Name, "", h.Out, h.Err)
		Register(c, h.Cli)

		status := c.Run(context.Background(), append([]string{command.Name}, args...))

		return status, h.Out.String(), h.Err.String()
	}

	t.Run("every group and shortcut is registered", func(t *testing.T) {
		t.Parallel()

		status, out, _ := run(t, workloadtest.New(), "--help")

		assert.Equal(t, console.ExitSuccess, status)

		for group := range groups {
			assert.Contains(t, out, "\n  "+group+" ", group)
		}

		for _, shortcut := range shortcuts {
			assert.Contains(t, out, "\n  "+shortcut+" ", shortcut)
		}
	})

	t.Run("every command of every group helps", func(t *testing.T) {
		t.Parallel()

		for group, names := range groups {
			status, out, _ := run(t, workloadtest.New(), group, "--help")

			assert.Equal(t, console.ExitSuccess, status, group)

			for _, name := range names {
				assert.Contains(t, out, "\n  "+name+" ", group+" "+name)

				status, out, _ := run(t, workloadtest.New(), group, name, "--help")

				assert.Equal(t, console.ExitSuccess, status, group+" "+name)
				assert.Contains(t, out, command.Name+" "+group+" "+name, group+" "+name)
				assert.True(t, strings.HasSuffix(strings.SplitN(out, "\n", 2)[0], "."), "%s %s is described: %q", group, name, out)
			}
		}
	})

	t.Run("every shortcut helps as itself", func(t *testing.T) {
		t.Parallel()

		for _, shortcut := range shortcuts {
			status, out, _ := run(t, workloadtest.New(), shortcut, "--help")

			assert.Equal(t, console.ExitSuccess, status, shortcut)
			assert.Contains(t, out, "\n  "+command.Name+" "+shortcut+" ", shortcut)
		}
	})

	t.Run("shortcuts and aliases run the commands they stand for", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(box))

		for _, args := range [][]string{{"ps", "-q"}, {"container", "ps", "-q"}, {"container", "list", "-q"}} {
			status, out, _ := run(t, fake, args...)

			assert.Equal(t, console.ExitSuccess, status, args)
			assert.Equal(t, "abcdef123456\n", out, args)
		}

		status, out, _ := run(t, fake, "images", "-q")
		assert.Equal(t, console.ExitSuccess, status)
		assert.Empty(t, out, "a machine holds no images")
	})
}

func TestCommandsWithoutAClient(t *testing.T) {
	t.Parallel()

	cases := map[string]func(cli *command.Cli) (console.Command, []string){
		"container ls": func(cli *command.Cli) (console.Command, []string) { return newListContainersCommand(cli), nil },
		"container run": func(cli *command.Cli) (console.Command, []string) {
			return newRunContainerCommand(cli), []string{"nginx"}
		},
		"container start": func(cli *command.Cli) (console.Command, []string) {
			return newStartContainersCommand(cli), []string{"a"}
		},
		"container stop": func(cli *command.Cli) (console.Command, []string) {
			return newStopContainersCommand(cli), []string{"a"}
		},
		"container restart": func(cli *command.Cli) (console.Command, []string) {
			return newRestartContainersCommand(cli), []string{"a"}
		},
		"container rm": func(cli *command.Cli) (console.Command, []string) {
			return newRemoveContainersCommand(cli), []string{"a"}
		},
		"container logs": func(cli *command.Cli) (console.Command, []string) {
			return newShowContainerLogsCommand(cli), []string{"a"}
		},
		"container stats": func(cli *command.Cli) (console.Command, []string) { return newShowContainerStatsCommand(cli), nil },
		"container inspect": func(cli *command.Cli) (console.Command, []string) {
			return newInspectContainersCommand(cli), []string{"a"}
		},
		"container port": func(cli *command.Cli) (console.Command, []string) {
			return newShowContainerPortsCommand(cli), []string{"a"}
		},
		"container connect": func(cli *command.Cli) (console.Command, []string) {
			return newConnectContainerCommand(cli), []string{"a", "b"}
		},
		"container disconnect": func(cli *command.Cli) (console.Command, []string) {
			return newDisconnectContainerCommand(cli), []string{"a", "b"}
		},
		"image ls":       func(cli *command.Cli) (console.Command, []string) { return newListImagesCommand(cli), nil },
		"image pull":     func(cli *command.Cli) (console.Command, []string) { return newPullImageCommand(cli), []string{"a"} },
		"image rm":       func(cli *command.Cli) (console.Command, []string) { return newRemoveImagesCommand(cli), []string{"a"} },
		"image inspect":  func(cli *command.Cli) (console.Command, []string) { return newInspectImagesCommand(cli), []string{"a"} },
		"network ls":     func(cli *command.Cli) (console.Command, []string) { return newListNetworksCommand(cli), nil },
		"network create": func(cli *command.Cli) (console.Command, []string) { return newCreateNetworkCommand(cli), []string{"a"} },
		"network rm": func(cli *command.Cli) (console.Command, []string) {
			return newRemoveNetworksCommand(cli), []string{"a"}
		},
		"network inspect": func(cli *command.Cli) (console.Command, []string) {
			return newInspectNetworksCommand(cli), []string{"a"}
		},
		"volume ls":     func(cli *command.Cli) (console.Command, []string) { return newListVolumesCommand(cli), nil },
		"volume create": func(cli *command.Cli) (console.Command, []string) { return newCreateVolumeCommand(cli), []string{"a"} },
		"volume rm":     func(cli *command.Cli) (console.Command, []string) { return newRemoveVolumesCommand(cli), []string{"a"} },
		"volume inspect": func(cli *command.Cli) (console.Command, []string) {
			return newInspectVolumesCommand(cli), []string{"a"}
		},
		"stack deploy": func(cli *command.Cli) (console.Command, []string) {
			return newDeployStackCommand(cli), []string{"-c", "-", "a"}
		},
		"stack ls": func(cli *command.Cli) (console.Command, []string) { return newListStacksCommand(cli), nil },
		"stack ps": func(cli *command.Cli) (console.Command, []string) {
			return newListStackContainersCommand(cli), []string{"a"}
		},
		"stack rm":      func(cli *command.Cli) (console.Command, []string) { return newRemoveStacksCommand(cli), []string{"a"} },
		"stack start":   func(cli *command.Cli) (console.Command, []string) { return newStartStacksCommand(cli), []string{"a"} },
		"stack stop":    func(cli *command.Cli) (console.Command, []string) { return newStopStacksCommand(cli), []string{"a"} },
		"stack restart": func(cli *command.Cli) (console.Command, []string) { return newRestartStacksCommand(cli), []string{"a"} },
		"stack inspect": func(cli *command.Cli) (console.Command, []string) { return newInspectStacksCommand(cli), []string{"a"} },
		"vm ls":         func(cli *command.Cli) (console.Command, []string) { return newListVMsCommand(cli), nil },
		"vm create":     func(cli *command.Cli) (console.Command, []string) { return newCreateVMCommand(cli), nil },
		"vm rm":         func(cli *command.Cli) (console.Command, []string) { return newRemoveVMsCommand(cli), []string{"a"} },
		"vm start":      func(cli *command.Cli) (console.Command, []string) { return newStartVMsCommand(cli), []string{"a"} },
		"vm stop":       func(cli *command.Cli) (console.Command, []string) { return newStopVMsCommand(cli), []string{"a"} },
		"vm restart":    func(cli *command.Cli) (console.Command, []string) { return newRestartVMsCommand(cli), []string{"a"} },
		"vm inspect":    func(cli *command.Cli) (console.Command, []string) { return newInspectVMsCommand(cli), []string{"a"} },
		"vm logs":       func(cli *command.Cli) (console.Command, []string) { return newShowVMLogsCommand(cli), []string{"a"} },
		"vm stats":      func(cli *command.Cli) (console.Command, []string) { return newShowVMStatsCommand(cli), []string{"a"} },
		"vm update":     func(cli *command.Cli) (console.Command, []string) { return newUpdateVMCommand(cli), []string{"a"} },
		"vm restore": func(cli *command.Cli) (console.Command, []string) {
			return newRestoreVMCommand(cli), []string{"--snapshot", "s", "a"}
		},
		"vm dependents": func(cli *command.Cli) (console.Command, []string) {
			return newShowVMDependentsCommand(cli), []string{"a"}
		},
		"snapshot ls": func(cli *command.Cli) (console.Command, []string) { return newListSnapshotsCommand(cli), nil },
		"snapshot create": func(cli *command.Cli) (console.Command, []string) {
			return newCreateSnapshotCommand(cli), []string{"a"}
		},
		"snapshot rm": func(cli *command.Cli) (console.Command, []string) {
			return newRemoveSnapshotsCommand(cli), []string{"a"}
		},
		"snapshot inspect": func(cli *command.Cli) (console.Command, []string) {
			return newInspectSnapshotsCommand(cli), []string{"a"}
		},
		"snapshot rename": func(cli *command.Cli) (console.Command, []string) {
			return newRenameSnapshotCommand(cli), []string{"a", "b"}
		},
		"inspect": func(cli *command.Cli) (console.Command, []string) { return newInspectCommand(cli), []string{"a"} },
	}

	for name, of := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Load(t.TempDir())
			require.NoError(t, err)

			var out, errOut bytes.Buffer

			cli := command.NewCli(strings.NewReader(""), &out, &errOut, command.WithConfig(cfg))
			cli.Globals.Context = "missing"

			cmd, args := of(cli)
			status := console.New(command.Name, "", &out, &errOut).Register(cmd).Run(context.Background(), append([]string{command.Name, cmd.Name()}, args...))

			assert.Equal(t, console.ExitFailure, status, errOut.String())
			assert.Contains(t, errOut.String(), "no such context")
		})
	}
}
