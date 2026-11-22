package content

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// listedUnder are the names a help lists under a heading.
func listedUnder(help, heading string) []string {
	_, after, found := strings.Cut(help, heading+"\n\n")
	if !found {
		return nil
	}

	var names []string

	for line := range strings.SplitSeq(after, "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			break
		}

		names = append(names, strings.Fields(line)[0])
	}

	return names
}

// registered is a console with every content command registered, built on
// a harness whose API is api.
func registered(t *testing.T, api http.Handler) (*console.Console, func() (string, string)) {
	t.Helper()

	h := harness(t, api)
	c := console.New(command.Name, "", h.Out, h.Err)
	Register(c, h.Cli)

	return c, func() (string, string) { return h.Out.String(), h.Err.String() }
}

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("a group for every resource, and the commands of the one signed in and the estate", func(t *testing.T) {
		t.Parallel()

		c, output := registered(t, nil)

		require.Equal(t, console.ExitSuccess, c.Run(context.Background(), []string{command.Name, "--help"}))

		help, _ := output()
		assert.Equal(t, []string{
			"article", "attendable-exam", "audit-log", "bookmark", "comment", "config", "contact", "element", "exam",
			"exam-attempt", "file", "language", "license", "license-template", "profile", "question", "role", "subscription", "user",
		}, listedUnder(help, "The command groups are:"))
		assert.Equal(t, []string{"overview", "permissions"}, listedUnder(help, "The commands are:"))
	})

	groups := map[string][]string{
		"article":          {"create", "inspect", "list", "ls", "rm", "update"},
		"attendable-exam":  {"attempts", "inspect", "list", "ls"},
		"audit-log":        {"activity", "list", "ls"},
		"bookmark":         {"list", "ls", "rm"},
		"comment":          {"create", "inspect", "list", "ls", "rm", "update"},
		"config":           {"set", "show"},
		"contact":          {"inspect", "list", "ls", "read", "rm", "unread"},
		"element":          {"create", "inspect", "list", "ls", "rm", "update"},
		"exam":             {"create", "inspect", "list", "ls", "rm", "update"},
		"exam-attempt":     {"answer", "finish", "inspect", "list", "ls", "show", "start"},
		"file":             {"download", "list", "ls", "rm", "upload"},
		"language":         {"create", "inspect", "list", "ls", "rm", "update"},
		"license":          {"create", "inspect", "list", "ls", "rm"},
		"license-template": {"create", "inspect", "list", "ls", "rm", "update"},
		"profile":          {"password", "roles", "show", "subscription", "update"},
		"profile sessions": {"list", "ls", "rm", "rm-others"},
		"question":         {"create", "inspect", "list", "ls", "rm", "update"},
		"role":             {"create", "inspect", "list", "ls", "rm", "update"},
		"subscription":     {"create", "inspect", "list", "ls", "rm", "update"},
		"user":             {"create", "impersonate", "inspect", "list", "ls", "password", "rm", "roles", "sessions", "signout", "subscription", "update"},
	}

	for group, commands := range groups {
		t.Run(group, func(t *testing.T) {
			t.Parallel()

			c, output := registered(t, nil)

			args := append(append([]string{command.Name}, strings.Fields(group)...), "--help")
			require.Equal(t, console.ExitSuccess, c.Run(context.Background(), args))

			help, _ := output()
			assert.Equal(t, commands, listedUnder(help, "The commands are:"))
		})
	}

	t.Run("profile's sessions are a group of their own", func(t *testing.T) {
		t.Parallel()

		c, output := registered(t, nil)

		require.Equal(t, console.ExitSuccess, c.Run(context.Background(), []string{command.Name, "profile", "--help"}))

		help, _ := output()
		assert.Equal(t, []string{"sessions"}, listedUnder(help, "The command groups are:"))
	})

	t.Run("a command's usage is its full name", func(t *testing.T) {
		t.Parallel()

		c, output := registered(t, nil)

		require.Equal(t, console.ExitSuccess, c.Run(context.Background(), []string{command.Name, "article", "list", "--help"}))

		help, _ := output()
		assert.Contains(t, help, "tarhche article list [OPTIONS]")
	})

	for _, test := range []struct {
		args  []string
		route string
	}{
		{args: []string{"article", "ls"}, route: "GET /api/dashboard/my/articles?page=1"},
		{args: []string{"profile", "roles"}, route: "GET /api/dashboard/profile/roles?page=1"},
		{args: []string{"profile", "sessions", "ls"}, route: "GET /api/dashboard/profile/sessions?page=1"},
		{args: []string{"profile", "sessions", "rm-others"}, route: "DELETE /api/dashboard/profile/sessions/others"},
		{args: []string{"permissions"}, route: "GET /api/dashboard/permissions?page=1"},
		{args: []string{"overview"}, route: "GET /api/dashboard/overview"},
		{args: []string{"config", "show"}, route: "GET /api/dashboard/config"},
		{args: []string{"attendable-exam", "attempts", "x1"}, route: "GET /api/exams/x1/attempts"},
		{args: []string{"exam-attempt", "show", "a1"}, route: "GET /api/exam-attempts/a1"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(map[string][]reply{
				strings.Replace(test.route, "?page=1", "", 1): answer(`{"items":[],"pagination":{"total_pages":0,"current_page":1}}`),
			})
			c, output := registered(t, api)

			status := c.Run(context.Background(), append([]string{command.Name}, test.args...))

			_, errOut := output()
			require.Equal(t, console.ExitSuccess, status, errOut)
			assert.Equal(t, []string{test.route}, api.routes())
		})
	}
}

func TestNewGroup(t *testing.T) {
	t.Parallel()

	d := resource.Definition{
		Name:     "thing",
		Plural:   "things",
		Title:    "Things",
		Key:      []string{"uuid"},
		Everyone: resource.Paths{List: "/api/things"},
		Actions:  []resource.Action{{Name: "poke", Description: "Poke things", Method: http.MethodPost, Path: "/api/things/{uuid}/poke"}},
	}

	help := func(t *testing.T, group *console.Group) string {
		t.Helper()

		h := harness(t, nil)
		require.Equal(t, console.ExitSuccess, console.New(command.Name, "", h.Out, h.Err).RegisterGroup(group).Run(context.Background(), []string{command.Name, "thing", "--help"}))

		return h.Out.String()
	}

	t.Run("the commands it serves and its actions", func(t *testing.T) {
		t.Parallel()

		group := newGroup(harness(t, nil).Cli, d, "thing", "tarhche thing")

		assert.Equal(t, []string{"list", "ls", "poke"}, listedUnder(help(t, group), "The commands are:"))
		assert.Contains(t, help(t, group), "Manage things")
	})

	t.Run("a special takes the place of the command of its name, or comes beside them", func(t *testing.T) {
		t.Parallel()

		cli := harness(t, nil).Cli
		group := newGroup(cli, d, "thing", "tarhche thing",
			newShowCommand(cli, "poke", "Poke differently", "tarhche thing poke", "/api/poke"),
			newShowCommand(cli, "extra", "Extra", "tarhche thing extra", "/api/extra"),
		)

		written := help(t, group)
		assert.Equal(t, []string{"extra", "list", "ls", "poke"}, listedUnder(written, "The commands are:"))
		assert.Contains(t, written, "Poke differently")
		assert.NotContains(t, written, "Poke things")
	})
}

func TestLookup(t *testing.T) {
	t.Parallel()

	t.Run("a resource described", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "articles", lookup("article").Plural)
	})

	t.Run("one that is not is a mistake in the wiring", func(t *testing.T) {
		t.Parallel()

		assert.PanicsWithValue(t, `content: no resource is named "nothing"`, func() { lookup("nothing") })
	})
}

func TestActionsByName(t *testing.T) {
	t.Parallel()

	everybody := resource.Action{Name: "activity", Path: "/all"}
	mine := resource.Action{Name: "activity", Path: "/mine", Mine: true}
	other := resource.Action{Name: "other"}

	assert.Equal(t, [][]resource.Action{{everybody, mine}, {other}}, actionsByName(resource.Definition{Actions: []resource.Action{everybody, other, mine}}))
}

func TestPlural(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "license templates", plural(lookup("license-template")))
}

func TestSingular(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "license template", singular(lookup("license-template")))
}

func TestIndefinite(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"article":      "an article",
		"exam-attempt": "an exam attempt",
		"element":      "an element",
		"user":         "a user",
		"role":         "a role",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, indefinite(lookup(name)))
		})
	}
}

func TestFlagName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "language-code", flagName("language_code"))
}

func TestUsageError(t *testing.T) {
	t.Parallel()

	err := usageError(console.NewFlagSet("tarhche thing", nil), "%d things", 2)

	var usage command.UsageError
	require.ErrorAs(t, err, &usage)
	assert.Equal(t, command.UsageError{Command: "tarhche thing", Message: "2 things"}, usage)
}

func TestParseKeys(t *testing.T) {
	t.Parallel()

	article := lookup("article")

	parsed := func(t *testing.T, args ...string) *console.FlagSet {
		t.Helper()

		flags := console.NewFlagSet("tarhche article rm", nil)
		require.NoError(t, flags.Parse(args))

		return flags
	}

	t.Run("each as the resource takes it", func(t *testing.T) {
		t.Parallel()

		keys, err := parseKeys(parsed(t, "c1/en", "c2/fa"), article)

		require.NoError(t, err)
		assert.Equal(t, [][]string{{"c1", "en"}, {"c2", "fa"}}, keys)
	})

	t.Run("one it does not take is a usage error", func(t *testing.T) {
		t.Parallel()

		_, err := parseKeys(parsed(t, "c1/en", "c2"), article)

		var usage command.UsageError
		require.ErrorAs(t, err, &usage)
		assert.Contains(t, usage.Message, `invalid key "c2"`)
	})
}

func TestRequestBody(t *testing.T) {
	t.Parallel()

	t.Run("nothing for no fields", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, requestBody(nil))
		assert.Nil(t, requestBody(map[string]any{}))
	})

	t.Run("the fields", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{"read": true}, requestBody(map[string]any{"read": true}))
	})
}
