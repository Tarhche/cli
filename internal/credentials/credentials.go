// Package credentials keeps the sessions the CLI signed in with, one per
// context: in the operating system's keychain where there is one, and in a
// file only its owner may read where there is not.
package credentials

import (
	"errors"

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

// Open is the store the configuration says sessions are kept in.
func Open(cfg *config.File) (Store, error) {
	return nil, errors.New("credentials: no store is implemented yet")
}
