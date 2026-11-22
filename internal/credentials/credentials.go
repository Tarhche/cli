// Package credentials keeps the sessions the CLI signed in with, one per
// context: in the operating system's keychain where there is one, and in a
// file only its owner may read where there is not.
package credentials

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/Tarhche/cli/internal/config"
)

const (
	// MethodOAuth is a session an authorization server handed over, renewed
	// at its token endpoint by the application it was handed to.
	MethodOAuth = "oauth"

	// MethodPassword is a session signed in to with a password, renewed at
	// the API's own refresh endpoint.
	MethodPassword = "password"
)

const (
	// StoreKeychain is the configuration's name for the operating system's
	// keychain.
	StoreKeychain = "keychain"

	// StoreFile is the configuration's name for the file beside it.
	StoreFile = "file"
)

// ErrNotFound is a context nobody signed in to.
var ErrNotFound = errors.New("no credentials")

// Credentials are a session of a context.
type Credentials struct {
	Method       string `json:"method"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`

	// ClientID and TokenEndpoint are what an OAuth session is renewed with.
	ClientID      string `json:"client_id,omitempty"`
	TokenEndpoint string `json:"token_endpoint,omitempty"`
}

// Store keeps credentials by context.
type Store interface {
	// Get is the credentials kept for a context, or ErrNotFound.
	Get(context string) (Credentials, error)

	// Set keeps credentials for a context, replacing what was kept.
	Set(context string, credentials Credentials) error

	// Delete forgets the credentials of a context; forgetting what was
	// never kept is no error.
	Delete(context string) error
}

// Open is the store the configuration says sessions are kept in: the
// keychain, the file, or, when it names neither, the keychain when it works
// on this machine and the file when it does not.
func Open(cfg *config.File) (Store, error) {
	return open(cfg, SystemKeyring{})
}

func open(cfg *config.File, backend Backend) (Store, error) {
	switch cfg.CredentialsStore {
	case StoreFile:
		return NewFile(cfg.Dir()), nil
	case StoreKeychain:
		return NewKeyring(backend), nil
	case "":
		if works(backend) {
			return NewKeyring(backend), nil
		}

		return NewFile(cfg.Dir()), nil
	default:
		return nil, fmt.Errorf("the credentials store %q is neither %q nor %q", cfg.CredentialsStore, StoreKeychain, StoreFile)
	}
}

// works reports whether backend keeps secrets on this machine, by keeping a
// throwaway one and reading it back. Its name is random, so that two
// processes asking at once do not read or remove each other's, and starts
// with a colon, which no context's name does.
func works(backend Backend) bool {
	name, secret := ":probe:"+rand.Text(), "probe"
	if err := backend.Set(Service, name, secret); err != nil {
		return false
	}

	defer func() { _ = backend.Delete(Service, name) }()

	kept, err := backend.Get(Service, name)

	return err == nil && kept == secret
}
