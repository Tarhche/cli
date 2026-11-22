// Package command holds what every command of the CLI is built on: its
// streams, the global flags, the context it talks to, a session there, and
// how it reports what went wrong.
package command

import (
	"context"
	"io"
	"sync"

	"github.com/danceable/console"
	"golang.org/x/term"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/version"
)

// Name is what the CLI is called, in its help and its messages.
const Name = "tarhche"

// DefaultLanguage is the language the API is asked to answer in unless told
// otherwise: the CLI's own.
const DefaultLanguage = "en"

// Globals are the flags given before a command's name.
type Globals struct {
	ConfigDir string `usage:"the directory the configuration is kept in (default ~/.tarhche)." long:"config" env:"TARHCHE_CONFIG"`
	Context   string `usage:"the context to talk to, instead of the current one (see \"tarhche context use\")." long:"context" short:"c" env:"TARHCHE_CONTEXT"`
	Host      string `usage:"the API to talk to, instead of the context's." long:"host" short:"H" env:"TARHCHE_HOST"`
	Ingress   string `usage:"the workload ingress to talk to, instead of the context's." long:"ingress" env:"TARHCHE_INGRESS"`
	Language  string `usage:"the language the API answers in: en or fa." long:"language" env:"TARHCHE_LANGUAGE"`
	Debug     bool   `usage:"says more of what goes wrong." long:"debug" short:"D" env:"TARHCHE_DEBUG"`
	Desktop   bool   `usage:"opens the desktop app instead of running a command." long:"desktop"`
	Version   bool   `usage:"prints the version and exits." long:"version" short:"v"`
}

// Cli is what commands are built on.
type Cli struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	Globals Globals

	lock   sync.Mutex
	config *config.File
	auth   *auth.Manager
	tokens api.TokenSource
	client *api.Client
}

// Option configures a Cli, which is how tests give it what it would
// otherwise build from the configuration.
type Option func(*Cli)

// WithConfig talks to the contexts of cfg.
func WithConfig(cfg *config.File) Option {
	return func(c *Cli) { c.config = cfg }
}

// WithAuth signs in and out through m.
func WithAuth(m *auth.Manager) Option {
	return func(c *Cli) { c.auth = m }
}

// WithTokens signs requests and streams with the tokens of source.
func WithTokens(source api.TokenSource) Option {
	return func(c *Cli) { c.tokens = source }
}

// WithClient talks to the API through client.
func WithClient(client *api.Client) Option {
	return func(c *Cli) { c.client = client }
}

// NewCli is a Cli reading in and writing out and err.
func NewCli(in io.Reader, out, err io.Writer, options ...Option) *Cli {
	c := &Cli{In: in, Out: out, Err: err, Globals: Globals{Language: DefaultLanguage}}
	for _, option := range options {
		option(c)
	}

	return c
}

// Config is the configuration the global flags point at, read once.
func (c *Cli) Config() (*config.File, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.loadConfig()
}

func (c *Cli) loadConfig() (*config.File, error) {
	if c.config != nil {
		return c.config, nil
	}

	dir, err := config.Dir(c.Globals.ConfigDir)
	if err != nil {
		return nil, err
	}

	if c.config, err = config.Load(dir); err != nil {
		return nil, err
	}

	return c.config, nil
}

// ContextName is the name of the context commands talk to: the one the
// global flags name, else the current one.
func (c *Cli) ContextName() (string, error) {
	if len(c.Globals.Context) > 0 {
		return c.Globals.Context, nil
	}

	cfg, err := c.Config()
	if err != nil {
		return "", err
	}

	return cfg.Current(), nil
}

// Context is the context commands talk to, with the API and the ingress the
// global flags name laid over it.
func (c *Cli) Context() (config.Context, error) {
	name, err := c.ContextName()
	if err != nil {
		return config.Context{}, err
	}

	cfg, err := c.Config()
	if err != nil {
		return config.Context{}, err
	}

	current, err := cfg.Context(name)
	if err != nil {
		return config.Context{}, err
	}

	if len(c.Globals.Host) > 0 {
		current.API = c.Globals.Host
	}

	if len(c.Globals.Ingress) > 0 {
		current.Ingress = c.Globals.Ingress
	}

	return current, nil
}

// Auth is what signs the CLI in and out.
func (c *Cli) Auth() (*auth.Manager, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.auth != nil {
		return c.auth, nil
	}

	cfg, err := c.loadConfig()
	if err != nil {
		return nil, err
	}

	store, err := credentials.Open(cfg)
	if err != nil {
		return nil, err
	}

	c.auth = auth.New(cfg, store, auth.WithPrompt(c.Err), auth.WithInteractive(c.Interactive()))

	return c.auth, nil
}

// Tokens is the source of the tokens of the current context's session,
// which signs requests and streams alike.
func (c *Cli) Tokens() (api.TokenSource, error) {
	if c.tokens != nil {
		return c.tokens, nil
	}

	name, err := c.ContextName()
	if err != nil {
		return nil, err
	}

	manager, err := c.Auth()
	if err != nil {
		return nil, err
	}

	return manager.TokenSource(name), nil
}

// Client is a client of the current context's API, signed in.
func (c *Cli) Client(ctx context.Context) (*api.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	current, err := c.Context()
	if err != nil {
		return nil, err
	}

	tokens, err := c.Tokens()
	if err != nil {
		return nil, err
	}

	options := []api.Option{api.WithTokens(tokens), api.WithUserAgent(version.UserAgent())}
	if len(c.Globals.Language) > 0 {
		options = append(options, api.WithLanguage(c.Globals.Language))
	}

	client, err := api.New(current.API, options...)
	if err != nil {
		return nil, err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.client == nil {
		c.client = client
	}

	return c.client, nil
}

// ParseGlobals reads the global flags given before a command's name into
// globals, leaving what it cannot read to the console, which reports it.
// It is how --desktop and --version are acted on without a command.
func ParseGlobals(args []string, globals *Globals) {
	flags := console.NewFlagSet(Name, io.Discard)
	if err := flags.Struct(globals); err != nil {
		return
	}

	_ = flags.Parse(args)
}

// Interactive reports whether somebody is at a terminal to answer: the
// input is one and so are the errors, where questions are asked.
func (c *Cli) Interactive() bool {
	return IsTerminal(c.In) && IsTerminal(c.Err)
}

// IsTerminal reports whether stream is a terminal.
func IsTerminal(stream any) bool {
	file, isFile := stream.(interface{ Fd() uintptr })

	return isFile && term.IsTerminal(int(file.Fd()))
}
