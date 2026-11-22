package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

const (
	// loginPath is where the API signs somebody in with a password.
	loginPath = "/api/auth/login"

	// refreshPath is where the API renews a session signed in to with a
	// password.
	refreshPath = "/api/auth/token/refresh"
)

// errNoToken is an estate that answered a login or a renewal without a
// token.
var errNoToken = errors.New("the estate answered no access token")

// loginRequest is what the API is asked to sign somebody in with.
type loginRequest struct {
	Identity string `json:"identity"`
	Password string `json:"password"`
	Device   device `json:"device"`
}

// refreshRequest is what the API is asked to renew a session with.
type refreshRequest struct {
	Token string `json:"token"`
}

// LoginWithPassword signs in to a context with an identity (a username or
// an email) and a password, and keeps the session.
func (m *Manager) LoginWithPassword(ctx context.Context, contextName, identity, password string) error {
	release, err := m.hold(ctx, contextName)
	if err != nil {
		return err
	}

	defer release()

	current, err := m.config.Context(contextName)
	if err != nil {
		return err
	}

	client, err := m.client(current.API, "")
	if err != nil {
		return err
	}

	var answer tokens

	err = client.Post(ctx, loginPath, loginRequest{Identity: identity, Password: password, Device: m.device()}, &answer)
	if api.IsValidation(err) {
		return fmt.Errorf("login refused: %w", err)
	} else if err != nil {
		return err
	}

	if len(answer.AccessToken) == 0 {
		return errNoToken
	}

	session := credentials.Credentials{
		Method:       credentials.MethodPassword,
		AccessToken:  answer.AccessToken,
		RefreshToken: answer.RefreshToken,
	}

	if err := m.store.Set(contextName, session); err != nil {
		return err
	}

	m.remember(contextName, session)

	return nil
}

// renewPassword renews a session signed in to with a password, at the API's
// own refresh endpoint, which hands over a new refresh token too.
func (m *Manager) renewPassword(ctx context.Context, current config.Context, kept credentials.Credentials) (credentials.Credentials, error) {
	client, err := m.client(current.API, "")
	if err != nil {
		return credentials.Credentials{}, err
	}

	var answer tokens
	if err := client.Post(ctx, refreshPath, refreshRequest{Token: kept.RefreshToken}, &answer); err != nil {
		return credentials.Credentials{}, err
	}

	return renewed(kept, answer)
}

// renewed is a session as its renewal left it: its new tokens, and the
// refresh token it had when it was not handed a new one.
func renewed(kept credentials.Credentials, answer tokens) (credentials.Credentials, error) {
	if len(answer.AccessToken) == 0 {
		return credentials.Credentials{}, errNoToken
	}

	kept.AccessToken = answer.AccessToken
	if len(answer.RefreshToken) > 0 {
		kept.RefreshToken = answer.RefreshToken
	}

	return kept, nil
}
