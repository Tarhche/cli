package system

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
)

// contextTable is the default table of a listing of contexts, the one the
// CLI talks to marked.
const contextTable = "{{.Name}}{{if .Current}} *{{end}}\t{{.Description}}\t{{.API}}\t{{.Ingress}}"

// contextView is a context as the context commands show it.
type contextView struct {
	Name        string `json:"Name"`
	Description string `json:"Description"`
	API         string `json:"API"`
	Ingress     string `json:"Ingress"`

	// Current says it is the context the CLI talks to.
	Current bool `json:"Current"`
}

// inspectedContext is a context as inspect shows it: as it is listed, with
// the application the CLI registered with its estate.
type inspectedContext struct {
	contextView

	OAuthClient *oauthClientView `json:"OAuthClient,omitempty"`
}

// oauthClientView is the application the CLI registered with an estate's
// authorization server, and where it is sent back to.
type oauthClientView struct {
	ID          string `json:"ID"`
	RedirectURI string `json:"RedirectURI"`
}

// newContextGroup is docker's context group: the estates the CLI can talk
// to, and which of them it talks to.
func newContextGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("context", "Manage contexts").Register(
		newListContextsCommand(cli),
		command.Alias(newListContextsCommand(cli), "list", command.Name+" context list [OPTIONS]"),
		newCreateContextCommand(cli),
		newUpdateContextCommand(cli),
		newUseContextCommand(cli),
		newRemoveContextsCommand(cli),
		command.Alias(newRemoveContextsCommand(cli), "remove", command.Name+" context remove [OPTIONS] CONTEXT [CONTEXT...]"),
		newInspectContextsCommand(cli),
		newShowContextCommand(cli),
	)
}

// viewOf is c as the context commands show it, current when it is the one
// the CLI talks to.
func viewOf(c config.Context, current string) contextView {
	return contextView{Name: c.Name, Description: c.Description, API: c.API, Ingress: c.Ingress, Current: c.Name == current}
}

// inspectionOf is c as inspect shows it.
func inspectionOf(c config.Context, current string) inspectedContext {
	inspected := inspectedContext{contextView: viewOf(c, current)}
	if c.OAuthClient != nil {
		inspected.OAuthClient = &oauthClientView{ID: c.OAuthClient.ID, RedirectURI: c.OAuthClient.RedirectURI}
	}

	return inspected
}
