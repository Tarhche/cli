// Package config reads and writes the CLI's configuration: the contexts it
// can talk to, and which of them it talks to.
//
// A context is an estate of Tarhche: its API, its workload ingress, and the
// application the CLI registered there to sign in through. The default
// context is Tarhche itself and needs no configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

const (
	// DirName is the directory under the home directory the configuration
	// lives in, unless told otherwise.
	DirName = ".tarhche"

	// FileName is the configuration file in that directory.
	FileName = "config.json"

	// DefaultContextName names the context that is there without being
	// created: Tarhche itself.
	DefaultContextName = "default"

	// DefaultAPI and DefaultIngress are where the default context is.
	DefaultAPI     = "https://backend.tarhche.com"
	DefaultIngress = "https://workload.tarhche.com"
)

var (
	// ErrNoSuchContext is a context that was never created.
	ErrNoSuchContext = errors.New("no such context")

	// ErrInvalidContext is a context that cannot be kept as it is.
	ErrInvalidContext = errors.New("invalid context")

	// ErrDefaultContext is something that cannot be done to the default
	// context, such as removing it.
	ErrDefaultContext = errors.New("the default context cannot be changed this way")

	contextName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.+-]*$`)
)

// Context is an estate the CLI talks to.
type Context struct {
	// Name is what the context is called; it is the key it is kept under.
	Name string `json:"-"`

	Description string `json:"description,omitempty"`

	// API is where the estate's API is, as an absolute http(s) URL.
	API string `json:"api"`

	// Ingress is where the estate's workload ingress is, which terminals,
	// commands and forwarded ports are carried through.
	Ingress string `json:"ingress,omitempty"`

	// OAuthClient is the application the CLI registered with the estate's
	// authorization server, kept so it is registered once.
	OAuthClient *OAuthClient `json:"oauth_client,omitempty"`
}

// OAuthClient is an application registered with an authorization server,
// and the address on the loopback interface it is answered at.
type OAuthClient struct {
	ID          string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
}

// File is the configuration file.
type File struct {
	path string

	// CurrentContext is the context commands talk to unless told otherwise.
	CurrentContext string `json:"current_context,omitempty"`

	// Contexts are the contexts that were created, by name.
	Contexts map[string]Context `json:"contexts,omitempty"`

	// CredentialsStore is where sessions are kept: "keychain", "file", or
	// nothing, which is the keychain where there is one and a file where
	// there is not.
	CredentialsStore string `json:"credentials_store,omitempty"`
}

// Dir is the directory the configuration lives in: override when it is
// given, else DirName in the home directory.
func Dir(override string) (string, error) {
	if len(override) > 0 {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory: %w", err)
	}

	return filepath.Join(home, DirName), nil
}

// Load reads the configuration file in dir. A directory with none in it is
// an empty configuration, which is written there once it is saved.
func Load(dir string) (*File, error) {
	f := &File{path: filepath.Join(dir, FileName)}

	content, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.path, err)
	}

	if err := json.Unmarshal(content, f); err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.path, err)
	}

	for name, c := range f.Contexts {
		c.Name = name
		f.Contexts[name] = c
	}

	return f, nil
}

// Path is where the file is.
func (f *File) Path() string {
	return f.path
}

// Dir is the directory the file is in, where whatever else the CLI keeps is
// kept beside it.
func (f *File) Dir() string {
	return filepath.Dir(f.path)
}

// Save writes the file, readable by its owner alone, replacing what was
// there at once rather than a write at a time.
func (f *File) Save() error {
	if err := os.MkdirAll(f.Dir(), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", f.Dir(), err)
	}

	content, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(f.Dir(), FileName+".*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	defer os.Remove(temporary.Name())

	if _, err := temporary.Write(append(content, '\n')); err != nil {
		_ = temporary.Close()

		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()

		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if err := temporary.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if err := os.Rename(temporary.Name(), f.path); err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	return nil
}

// Current is the name of the context commands talk to unless told
// otherwise.
func (f *File) Current() string {
	if len(f.CurrentContext) == 0 {
		return DefaultContextName
	}

	return f.CurrentContext
}

// Context is the context of that name. The default context is there
// whether or not it was ever changed.
func (f *File) Context(name string) (Context, error) {
	if c, found := f.Contexts[name]; found {
		c.Name = name

		return c, nil
	}

	if name == DefaultContextName {
		return DefaultContext(), nil
	}

	return Context{}, fmt.Errorf("%w: %q", ErrNoSuchContext, name)
}

// Names are the names of every context, the default one among them, in
// alphabetical order.
func (f *File) Names() []string {
	names := slices.Collect(maps.Keys(f.Contexts))
	if !slices.Contains(names, DefaultContextName) {
		names = append(names, DefaultContextName)
	}

	slices.Sort(names)

	return names
}

// SetContext keeps c under its name, replacing whatever was kept there.
func (f *File) SetContext(c Context) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if f.Contexts == nil {
		f.Contexts = make(map[string]Context)
	}

	f.Contexts[c.Name] = c

	return nil
}

// RemoveContext forgets the context of that name, and the current context
// goes back to the default one if it was that.
func (f *File) RemoveContext(name string) error {
	if name == DefaultContextName {
		return ErrDefaultContext
	}

	if _, found := f.Contexts[name]; !found {
		return fmt.Errorf("%w: %q", ErrNoSuchContext, name)
	}

	delete(f.Contexts, name)

	if f.CurrentContext == name {
		f.CurrentContext = ""
	}

	return nil
}

// Use makes the context of that name the one commands talk to.
func (f *File) Use(name string) error {
	if _, err := f.Context(name); err != nil {
		return err
	}

	if name == DefaultContextName {
		name = ""
	}

	f.CurrentContext = name

	return nil
}

// DefaultContext is Tarhche itself.
func DefaultContext() Context {
	return Context{
		Name:        DefaultContextName,
		Description: "Tarhche",
		API:         DefaultAPI,
		Ingress:     DefaultIngress,
	}
}

// Validate says what is wrong with a context, if anything.
func (c Context) Validate() error {
	if !contextName.MatchString(c.Name) {
		return fmt.Errorf("%w: %q is not a valid name: it may hold letters, digits, and _.+- after its first character", ErrInvalidContext, c.Name)
	}

	if err := validateURL(c.API); err != nil {
		return fmt.Errorf("%w: its api %w", ErrInvalidContext, err)
	}

	if len(c.Ingress) > 0 {
		if err := validateURL(c.Ingress); err != nil {
			return fmt.Errorf("%w: its ingress %w", ErrInvalidContext, err)
		}
	}

	return nil
}

func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || len(parsed.Host) == 0 || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%q is not an absolute http or https url", raw)
	}

	return nil
}
