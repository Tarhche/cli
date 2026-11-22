// Command tarhche manages Tarhche from a terminal, as docker manages
// containers, or from a desktop app: when it is given --desktop, and when it
// is started as an app is, by a double click, an app bundle or a desktop
// entry, with nothing to run and outside a terminal.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commands"
	"github.com/Tarhche/cli/internal/version"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	args := commandLine(os.Args)
	started := launch{asApp: startedAsApp(args, os.Stdin, os.Stdout, os.Stderr), openDesktop: openDesktop}

	status := run(ctx, args, os.Stdin, os.Stdout, os.Stderr, started)

	cancel()
	os.Exit(status)
}

// run runs what args ask for: the desktop app when --desktop asks for it, or
// when the binary was started as an app and a desktop app can be opened
// here, and the command line otherwise.
func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, started launch) int {
	cli := command.NewCli(in, out, errOut)

	app, err := commands.New(cli)
	if err != nil {
		fmt.Fprintln(errOut, err)

		return 1
	}

	command.ParseGlobals(args[1:], &cli.Globals)

	switch {
	case cli.Globals.Version:
		fmt.Fprintf(out, "Tarhche CLI version %s\n", version.Version)

		return 0
	case cli.Globals.Desktop:
		return cli.Exit(started.openDesktop(ctx, cli))
	case started.asApp:
		if err := started.openDesktop(ctx, cli); !desktopUnavailable(err) {
			return cli.Exit(err)
		}
	}

	return app.Run(ctx, args)
}
