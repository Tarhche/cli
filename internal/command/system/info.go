package system

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// infoTemplate is how info shows what it shows by default.
const infoTemplate = `Context:	{{.Context.Name}}
 API:	{{.Context.API}}
 Ingress:	{{.Context.Ingress}}
 Logged in:	{{if not .LoggedIn}}no{{else if eq .Method "oauth"}}yes, in a browser{{else}}yes, with a password{{end}}
{{- with .User}}

User:	{{.Name}}
 Username:	{{.Username}}
 Email:	{{.Email}}
 UUID:	{{.UUID}}
{{- if .ImpersonatedBy}}
 Seen by:	{{.ImpersonatedBy}}
{{- end}}
 Roles:	{{range $i, $role := $.Roles}}{{if $i}}, {{end}}{{$role.Title}}{{else}}none{{end}}
{{- end}}
{{- with .Subscription}}

Subscription:	{{or .Name "none"}}
 VMs:	{{.VMs}}
 Snapshots:	{{.Snapshots}}
 Files:	{{.Files}}
{{- if .Over}}
 Past its limits:	{{.Over}}
{{- end}}
{{- end}}
`

const (
	profilePath      = "/api/dashboard/profile"
	rolesPath        = "/api/dashboard/profile/roles"
	subscriptionPath = "/api/dashboard/profile/subscription"
)

// information is what info shows: the context the CLI talks to, and who it
// is signed in there as.
type information struct {
	Context      contextInfo       `json:"Context"`
	LoggedIn     bool              `json:"LoggedIn"`
	Method       string            `json:"Method,omitempty"`
	User         *userInfo         `json:"User,omitempty"`
	Roles        []roleInfo        `json:"Roles,omitempty"`
	Subscription *subscriptionInfo `json:"Subscription,omitempty"`
	Warnings     []string          `json:"Warnings,omitempty"`
}

// userInfo is who the session is theirs.
type userInfo struct {
	UUID     string `json:"UUID"`
	Name     string `json:"Name"`
	Username string `json:"Username"`
	Email    string `json:"Email"`

	// ImpersonatedBy is who sees the estate as them, in a shadow session.
	ImpersonatedBy string `json:"ImpersonatedBy,omitempty"`
}

// roleInfo is a role they hold, called what it is called in their language.
type roleInfo struct {
	UUID  string `json:"UUID"`
	Title string `json:"Title"`
}

// subscriptionInfo is the subscription they are on, none when it has no
// name, and how much they hold of what it limits.
type subscriptionInfo struct {
	UUID      string    `json:"UUID,omitempty"`
	Name      string    `json:"Name,omitempty"`
	VMs       usageInfo `json:"VMs"`
	Snapshots usageInfo `json:"Snapshots"`
	Files     usageInfo `json:"Files"`

	// Over is how many of what they hold is past its limits.
	Over int `json:"Over"`
}

// usageInfo is how many of something somebody holds, and how many they may
// hold: as many as the platform allows when Limit is nil.
type usageInfo struct {
	Used  int   `json:"Used"`
	Limit *uint `json:"Limit,omitempty"`
}

func (u usageInfo) String() string {
	if u.Limit == nil {
		return strconv.Itoa(u.Used)
	}

	return fmt.Sprintf("%d of %d", u.Used, *u.Limit)
}

// profileResponse is what the profile route answers.
type profileResponse struct {
	UUID           string `json:"uuid"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Username       string `json:"username"`
	LanguageCode   string `json:"language_code"`
	ImpersonatedBy *struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"impersonated_by"`
}

// rolesResponse is what the route of one's own roles answers.
type rolesResponse struct {
	Items []struct {
		UUID         string `json:"uuid"`
		Translations map[string]struct {
			Title string `json:"title"`
		} `json:"translations"`
	} `json:"items"`
}

// subscriptionResponse is what the route of one's own subscription answers.
type subscriptionResponse struct {
	Subscription *struct {
		UUID   string            `json:"uuid"`
		Names  map[string]string `json:"names"`
		Quotas struct {
			VMs       *quotaResponse `json:"vms"`
			Snapshots *quotaResponse `json:"snapshots"`
			Uploads   *quotaResponse `json:"uploads"`
		} `json:"quotas"`
	} `json:"subscription"`
	Usage struct {
		VMs       int `json:"vms"`
		Snapshots int `json:"snapshots"`
		Files     int `json:"files"`
	} `json:"usage"`
	Over []json.RawMessage `json:"over"`
}

// quotaResponse is how many of something a subscription allows: VMs,
// snapshots, or files, where none is no limit.
type quotaResponse struct {
	Max uint `json:"max"`
}

// infoCommand is docker's info: the context the CLI talks to, and who it is
// signed in there as, with their roles and their subscription.
type infoCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format string
}

func newInfoCommand(cli *command.Cli) *infoCommand {
	return &infoCommand{cli: cli}
}

func (c *infoCommand) Name() string {
	return "info"
}

func (c *infoCommand) Description() string {
	return "Display the context the CLI talks to, and who it is logged in as"
}

func (c *infoCommand) Usage() string {
	return command.Name + " info [OPTIONS]"
}

func (c *infoCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), formatUsage, console.Short("f"))
}

func (c *infoCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *infoCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	current, err := c.cli.Context()
	if err != nil {
		return err
	}

	manager, err := c.cli.Auth()
	if err != nil {
		return err
	}

	status, err := manager.Status(ctx, current.Name)
	if err != nil {
		return err
	}

	info := information{
		Context:  contextInfo{Name: current.Name, API: current.API, Ingress: current.Ingress},
		LoggedIn: status.LoggedIn,
		Method:   status.Method,
	}

	if status.LoggedIn {
		if err := c.readAccount(ctx, &info); err != nil {
			return err
		}
	}

	if err := show(c.cli.Out, info, c.format, infoTemplate); err != nil {
		return err
	}

	if len(strings.TrimSpace(c.format)) == 0 {
		for _, warning := range info.Warnings {
			_, _ = fmt.Fprintf(c.cli.Err, "WARNING: %s\n", warning)
		}
	}

	return nil
}

// readAccount reads who the session is theirs, their roles and their
// subscription into info. A subscription that cannot be read, which the
// estate reads off the workload too, is warned of rather than failing it.
func (c *infoCommand) readAccount(ctx context.Context, info *information) error {
	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var profile profileResponse
	if err := client.Get(ctx, profilePath, nil, &profile); err != nil {
		return err
	}

	info.User = &userInfo{UUID: profile.UUID, Name: profile.Name, Username: profile.Username, Email: profile.Email}
	if seer := profile.ImpersonatedBy; seer != nil {
		info.User.ImpersonatedBy = fmt.Sprintf("%s (%s)", seer.Name, seer.Username)
	}

	languages := []string{c.cli.Globals.Language, profile.LanguageCode, "en"}

	var roles rolesResponse
	if err := client.Get(ctx, rolesPath, nil, &roles); err != nil {
		return err
	}

	for _, role := range roles.Items {
		titles := make(map[string]string, len(role.Translations))
		for language, translation := range role.Translations {
			titles[language] = translation.Title
		}

		info.Roles = append(info.Roles, roleInfo{UUID: role.UUID, Title: cmp.Or(translated(titles, languages...), role.UUID)})
	}

	var subscription subscriptionResponse

	err = client.Get(ctx, subscriptionPath, nil, &subscription)

	switch {
	case err == nil:
		info.Subscription = subscriptionOf(subscription, languages)
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		info.Warnings = append(info.Warnings, "the subscription could not be read: "+err.Error())
	}

	return nil
}

// subscriptionOf is the subscription the API answered, its name in the
// first of languages it has one in.
func subscriptionOf(answered subscriptionResponse, languages []string) *subscriptionInfo {
	held := &subscriptionInfo{
		VMs:       usageInfo{Used: answered.Usage.VMs},
		Snapshots: usageInfo{Used: answered.Usage.Snapshots},
		Files:     usageInfo{Used: answered.Usage.Files},
		Over:      len(answered.Over),
	}

	if s := answered.Subscription; s != nil {
		held.UUID = s.UUID
		held.Name = cmp.Or(translated(s.Names, languages...), s.UUID)

		if q := s.Quotas.VMs; q != nil {
			held.VMs.Limit = &q.Max
		}

		if q := s.Quotas.Snapshots; q != nil {
			held.Snapshots.Limit = &q.Max
		}

		if q := s.Quotas.Uploads; q != nil && q.Max > 0 {
			held.Files.Limit = &q.Max
		}
	}

	return held
}

// translated is what texts says in the first of languages it says anything
// in, else in the first language it says anything in, in alphabetical
// order.
func translated(texts map[string]string, languages ...string) string {
	for _, language := range languages {
		if text := texts[language]; len(text) > 0 && len(language) > 0 {
			return text
		}
	}

	for _, language := range slices.Sorted(maps.Keys(texts)) {
		if text := texts[language]; len(text) > 0 {
			return text
		}
	}

	return ""
}
