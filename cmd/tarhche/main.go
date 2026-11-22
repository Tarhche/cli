// Command tarhche manages Tarhche from a terminal, as docker manages
// containers, or from a desktop app when it is given --desktop.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commands"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/version"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	status := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr)

	cancel()
	os.Exit(status)
}

func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
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
		return cli.Exit(desktop.Run(ctx, cli))
	}

	return app.Run(ctx, args)
}
