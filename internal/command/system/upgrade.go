package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/danceable/console"
	"golang.org/x/mod/semver"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/upgrade"
)

// upgradeCommand upgrades the CLI to one of its releases: the latest, or the
// one it is told.
type upgradeCommand struct {
	cli      *command.Cli
	upgrader *upgrade.Upgrader
	flags    *console.FlagSet

	check bool
	yes   bool

	interactive func() bool
	terminal    func() bool
	goos        string
}

func newUpgradeCommand(cli *command.Cli, upgrader *upgrade.Upgrader) *upgradeCommand {
	return &upgradeCommand{
		cli:         cli,
		upgrader:    upgrader,
		interactive: cli.Interactive,
		terminal:    func() bool { return command.IsTerminal(cli.Err) },
		goos:        runtime.GOOS,
	}
}

func (c *upgradeCommand) Name() string {
	return "upgrade"
}

func (c *upgradeCommand) Description() string {
	return "Upgrade the Tarhche CLI to its latest release, or to VERSION"
}

func (c *upgradeCommand) Usage() string {
	return command.Name + " upgrade [OPTIONS] [VERSION]"
}

func (c *upgradeCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.check, console.Long("check"), "Only tell whether a newer release is published, and what it says")
	console.Var(flags, &c.yes, console.Long("yes"), "Upgrade without asking", console.Short("y"))
}

func (c *upgradeCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *upgradeCommand) run(ctx context.Context) error {
	if err := command.MaxArgs(c.flags, 1); err != nil {
		return err
	}

	release, installable, err := c.find(ctx)
	if err != nil || !installable || c.check {
		return err
	}

	return c.install(ctx, release)
}

// find reads the release VERSION names, or the latest, and tells what it
// is and says; installable is false when the latest is not newer than the
// version running.
func (c *upgradeCommand) find(ctx context.Context) (upgrade.Release, bool, error) {
	current := c.upgrader.Current()

	if c.flags.NArg() == 1 {
		release, err := c.upgrader.Release(ctx, c.flags.Arg(0))
		if err != nil {
			return upgrade.Release{}, false, err
		}

		fmt.Fprintf(c.cli.Out, "tarhche %s%s (you have %s):\n", release.Version, released(release), current)
		c.describe(release)

		return release, true, nil
	}

	check, err := c.upgrader.Check(ctx)
	if err != nil {
		return upgrade.Release{}, false, err
	}

	latest := check.Latest

	switch {
	case !check.Available && latest.Version == strings.TrimPrefix(current, "v"):
		fmt.Fprintf(c.cli.Out, "tarhche %s is the latest version.\n", current)
	case !check.Available:
		fmt.Fprintf(c.cli.Out, "tarhche %s is newer than its latest release, %s.\n", current, latest.Version)
	case !isRelease(current):
		fmt.Fprintf(c.cli.Out, "This is a development build of tarhche (%s): its latest release is %s%s:\n", current, latest.Version, released(latest))
		c.describe(latest)
	default:
		fmt.Fprintf(c.cli.Out, "tarhche %s is available (you have %s)%s:\n", latest.Version, current, released(latest))
		c.describe(latest)
	}

	return latest, check.Available, nil
}

// describe writes what release says of itself, and where its page is.
func (c *upgradeCommand) describe(release upgrade.Release) {
	if notes := strings.TrimSpace(strings.ReplaceAll(release.Notes, "\r\n", "\n")); len(notes) > 0 {
		fmt.Fprintf(c.cli.Out, "\n%s\n", notes)
	}

	if len(release.URL) > 0 {
		fmt.Fprintf(c.cli.Out, "\n%s\n", release.URL)
	}
}

// install installs release in place of the running binary once whoever is
// at the terminal agrees to, or --yes did.
func (c *upgradeCommand) install(ctx context.Context, release upgrade.Release) error {
	target, err := c.upgrader.Target()
	if err != nil {
		return err
	}

	change := changeOf(release.Version, c.upgrader.Current())

	if !c.yes {
		if !c.interactive() {
			return fmt.Errorf("nobody is at a terminal to agree to it: run '%s --yes%s' to %s tarhche without being asked", c.again(), c.version(), strings.ToLower(change.verb))
		}

		agreed, err := c.ask(fmt.Sprintf("\n%s %s to %s? [y/N] ", change.verb, target, release.Version))
		if err != nil || !agreed {
			return err
		}
	}

	var progress upgrade.Progress

	line := &progressLine{out: c.cli.Err}
	if c.terminal() {
		progress = line.show
	}

	installation, err := c.upgrader.Install(ctx, release, progress)

	line.end()

	if err != nil {
		return c.explain(err)
	}

	fmt.Fprintf(c.cli.Out, "%s %s to %s.\n", change.done, installation.Path, installation.Version)

	return nil
}

// ask asks question where questions are asked and reports whether the
// answer is yes.
func (c *upgradeCommand) ask(question string) (bool, error) {
	fmt.Fprint(c.cli.Err, question)

	answer, err := bufio.NewReader(c.cli.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// explain is err, saying how to install where only an administrator may
// write when it is ErrPermission.
func (c *upgradeCommand) explain(err error) error {
	if !errors.Is(err, upgrade.ErrPermission) {
		return err
	}

	if c.goos == "windows" {
		return fmt.Errorf("%w: run '%s%s' again in a terminal run as administrator", err, c.again(), c.version())
	}

	return fmt.Errorf("%w: run 'sudo %s%s' to upgrade it as root", err, c.again(), c.version())
}

// again is the command that runs upgrade again.
func (c *upgradeCommand) again() string {
	return command.Name + " upgrade"
}

// version is the VERSION upgrade was given, after a space, if it was given
// one.
func (c *upgradeCommand) version() string {
	if c.flags.NArg() == 0 {
		return ""
	}

	return " " + c.flags.Arg(0)
}

// change is what installing a release does to the version running.
type change struct {
	verb string
	done string
}

var (
	upgrading    = change{verb: "Upgrade", done: "Upgraded"}
	downgrading  = change{verb: "Downgrade", done: "Downgraded"}
	reinstalling = change{verb: "Reinstall", done: "Reinstalled"}
)

// changeOf is what installing version does to current: a build of one's
// own is upgraded to any release.
func changeOf(version, current string) change {
	switch compared := semver.Compare(semverOf(version), semverOf(current)); {
	case compared > 0:
		return upgrading
	case compared < 0:
		return downgrading
	default:
		return reinstalling
	}
}

// isRelease reports whether version is a release's, not a build of one's
// own.
func isRelease(version string) bool {
	return semver.IsValid(semverOf(version))
}

// semverOf is version as semver says it: with its v.
func semverOf(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

// released says when release was published, after a comma: nothing when
// that is not known.
func released(release upgrade.Release) string {
	ago := formatter.Ago(release.PublishedAt)
	if len(ago) == 0 {
		return ""
	}

	return ", released " + strings.ToLower(ago[:1]) + ago[1:]
}

// progressLine shows how much of a download arrived on a line of its own,
// rewritten as the download goes on: as a share of it when how much there
// is in all is known, else a mebibyte at a time.
type progressLine struct {
	out   io.Writer
	shown int64
	wrote bool
}

// show shows done of total, when it is worth showing.
func (l *progressLine) show(done, total int64) {
	step := done >> 20
	said := "Downloading: " + formatter.Bytes(done)

	if total > 0 {
		step = done * 100 / total
		said = fmt.Sprintf("Downloading: %d%% (%s of %s)", step, formatter.Bytes(done), formatter.Bytes(total))
	}

	if l.wrote && step == l.shown {
		return
	}

	l.shown, l.wrote = step, true
	fmt.Fprintf(l.out, "\r%-50s", said)
}

// end ends the line, if anything was shown on it.
func (l *progressLine) end() {
	if l.wrote {
		fmt.Fprintln(l.out)
	}
}
