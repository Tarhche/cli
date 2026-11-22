package content

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

// impersonateRoute is where a shadow session is opened, a session of its
// opener's in which they are seen as the user it names.
const impersonateRoute = "/api/dashboard/users/{uuid}/impersonate"

// impersonateCommand opens a shadow session as a user and keeps it as a
// context of its own, beside the one it was opened from, so that commands
// given --context NAME act as that user; signing out of the context ends
// the session. It writes the context's name.
type impersonateCommand struct {
	cli   *command.Cli
	group string

	// open is the store sessions are kept in.
	open func(*config.File) (credentials.Store, error)

	flags *console.FlagSet
	name  string
}

var _ console.Command = &impersonateCommand{}

func newImpersonateCommand(cli *command.Cli, group string) *impersonateCommand {
	return &impersonateCommand{cli: cli, group: group, open: credentials.Open}
}

func (c *impersonateCommand) Name() string {
	return "impersonate"
}

func (c *impersonateCommand) Description() string {
	return "Open a shadow session as a user, kept as a context of its own"
}

func (c *impersonateCommand) Usage() string {
	return c.group + " impersonate [OPTIONS] UUID"
}

func (c *impersonateCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.name, console.Long("name"), `the context to keep the session as (default "<context>-as-<username>").`, console.Default(""))
}

func (c *impersonateCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *impersonateCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	from, err := c.cli.Context()
	if err != nil {
		return err
	}

	shadow := config.Context{Name: c.name, API: from.API, Ingress: from.Ingress}

	if len(shadow.Name) > 0 {
		if err := shadow.Validate(); err != nil {
			return usageError(c.flags, "%s", err)
		}
	}

	store, err := c.open(cfg)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var session struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			Username string `json:"username"`
		} `json:"user"`
	}

	if err := client.Post(ctx, user.Path(impersonateRoute, key), nil, &session); err != nil {
		return failed(user, key, err)
	}

	if len(session.AccessToken) == 0 || len(session.RefreshToken) == 0 {
		return errors.New("the api opened no session")
	}

	seenAs := session.User.Username
	if len(seenAs) == 0 {
		seenAs = key[0]
	}

	shadow.Description = fmt.Sprintf("%s, impersonated from %s", seenAs, from.Name)

	if len(shadow.Name) == 0 {
		shadow.Name = contextNameOf(from.Name + "-as-" + seenAs)
	}

	if err := c.keep(cfg, store, shadow, credentials.Credentials{
		Method:       credentials.MethodPassword,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}); err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, shadow.Name)

	return err
}

// keep keeps the session in store and the context it is used through in
// the configuration, and neither when either cannot be kept.
func (c *impersonateCommand) keep(cfg *config.File, store credentials.Store, shadow config.Context, session credentials.Credentials) error {
	if err := cfg.SetContext(shadow); err != nil {
		return err
	}

	if err := store.Set(shadow.Name, session); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return errors.Join(err, store.Delete(shadow.Name))
	}

	return nil
}

// contextNameOf is name as a context may be named: what it may not hold
// is a dash.
func contextNameOf(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("_.+-", r):
			return r
		default:
			return '-'
		}
	}, name)
}
