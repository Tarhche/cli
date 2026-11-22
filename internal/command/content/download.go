package content

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// downloadRoute is where a file's bytes are served, to anybody.
const downloadRoute = "/files/{uuid}"

// downloadCommand writes a file's bytes to the standard output, as docker
// save does, or to a file.
type downloadCommand struct {
	cli   *command.Cli
	group string

	flags  *console.FlagSet
	output string
}

var _ console.Command = &downloadCommand{}

func newDownloadCommand(cli *command.Cli, group string) *downloadCommand {
	return &downloadCommand{cli: cli, group: group}
}

func (c *downloadCommand) Name() string {
	return "download"
}

func (c *downloadCommand) Description() string {
	return "Download a file to the standard output, or to a file"
}

func (c *downloadCommand) Usage() string {
	return c.group + " download [OPTIONS] UUID"
}

func (c *downloadCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.output, console.Long("output"), "writes it to that file rather than to the standard output.", console.Short("o"), console.Default(""))
}

func (c *downloadCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *downloadCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	key, err := uuidKey.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	toOutput := len(c.output) == 0 || c.output == stdin
	if toOutput && command.IsTerminal(c.cli.Out) {
		return usageError(c.flags, "refusing to write a file to a terminal: give -o PATH, or redirect the output.")
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	request, err := client.NewRequest(ctx, http.MethodGet, uuidKey.Path(downloadRoute, key), nil, nil, "")
	if err != nil {
		return err
	}

	request.Header.Set("Accept", "*/*")

	response, err := client.Send(request)
	if err != nil {
		return failed(lookup("file"), key, err)
	}

	defer func() { _ = response.Body.Close() }()

	if toOutput {
		_, err := io.Copy(c.cli.Out, response.Body)

		return err
	}

	return save(c.output, response.Body)
}

// save writes content to the file at path, leaving no file behind when it
// cannot write all of it.
func save(path string, content io.Reader) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, content); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), file.Close(), os.Remove(path))
	}

	return file.Close()
}
