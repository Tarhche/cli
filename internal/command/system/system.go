// Package system holds the commands about the CLI itself and the estate it
// talks to: signing in and out, its contexts, its version and upgrading it,
// and who it talks to as whom.
package system

import (
	"io"
	"strings"
	"text/tabwriter"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/upgrade"
	"github.com/Tarhche/cli/internal/version"
)

// formatUsage is what --format is said to take by the commands that show
// one thing.
const formatUsage = "Format output using a custom template: 'json' prints it as JSON, 'TEMPLATE' prints it through the given Go template"

// Register registers the system commands and the context group on c.
func Register(c *console.Console, cli *command.Cli) {
	c.Register(
		newLoginCommand(cli),
		newLogoutCommand(cli),
		newVersionCommand(cli),
		newUpgradeCommand(cli, upgrade.New(upgrade.WithUserAgent(version.UserAgent()))),
		newInfoCommand(cli),
	)

	c.RegisterGroup(newContextGroup(cli))
}

// show writes value as --format asks: through the default template when it
// asks for nothing, as JSON for "json", and through the user's template
// otherwise.
func show(out io.Writer, value any, format, defaultTemplate string) error {
	switch strings.TrimSpace(format) {
	case "":
		return writeAligned(out, value, defaultTemplate)
	case formatter.JSONFormat:
		return formatter.Write(out, []any{value}, formatter.Options{Format: formatter.JSONFormat})
	default:
		return formatter.WriteTemplate(out, value, format)
	}
}

// writeAligned writes value through text, a template whose lines are a
// label and a value separated by a tab, the values aligned as docker aligns
// them.
func writeAligned(out io.Writer, value any, text string) error {
	tmpl, err := formatter.Parse(text)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(out, 20, 1, 1, ' ', 0)
	if err := tmpl.Execute(writer, value); err != nil {
		return err
	}

	return writer.Flush()
}
