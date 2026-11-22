// Package content holds a group of commands for every resource the
// dashboard manages that is not a workload: articles, comments, users,
// roles and the rest, each built from its description in package resource.
//
// Every group has the commands its resource serves (ls, inspect, create,
// update and rm) and a command for each of its actions; some have commands
// of their own beside them (file upload, user password, ...). A key with
// more than one field, as an article's, is given as one argument, its
// fields joined by slashes: CORRELATION_UUID/LANGUAGE_CODE.
package content

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// Register registers a group of commands for every content resource on c,
// and the commands about the one signed in and the estate itself: profile,
// config, overview and permissions.
func Register(c *console.Console, cli *command.Cli) {
	for _, d := range resource.All() {
		switch d.Name {
		case "session", "permission":
			continue
		}

		path := command.Name + " " + d.Name
		c.RegisterGroup(newGroup(cli, d, d.Name, path, specials(cli, d, path)...))
	}

	c.RegisterGroup(newProfileGroup(cli), newConfigGroup(cli))
	c.Register(
		newShowCommand(cli, "overview", "Display what one has, and as much of the estate as one may see", command.Name+" overview [OPTIONS]", "/api/dashboard/overview"),
		newListCommand(cli, lookup("permission"), command.Name, named("permissions", "List every permission a role may allow or disallow")),
	)
}

// newGroup is the group of commands of a resource, named name and called
// path in full: the commands the resource serves, its actions, and
// specials, each of which takes the place of the command of its name.
func newGroup(cli *command.Cli, d resource.Definition, name, path string, specials ...console.Command) *console.Group {
	var commands []console.Command

	if d.Serves(resource.OperationList) {
		list := newListCommand(cli, d, path)
		commands = append(commands, list, command.Alias(list, "list", path+" list [OPTIONS]"))
	}

	if d.Serves(resource.OperationGet) {
		commands = append(commands, newInspectCommand(cli, d, path))
	}

	if d.Serves(resource.OperationCreate) {
		commands = append(commands, newCreateCommand(cli, d, path))
	}

	if d.Serves(resource.OperationUpdate) {
		commands = append(commands, newUpdateCommand(cli, d, path))
	}

	if d.Serves(resource.OperationDelete) {
		commands = append(commands, newRemoveCommand(cli, d, path))
	}

	for _, actions := range actionsByName(d) {
		commands = append(commands, newActCommand(cli, d, path, actions))
	}

	for _, special := range specials {
		if at := slices.IndexFunc(commands, func(c console.Command) bool { return c.Name() == special.Name() }); at >= 0 {
			commands[at] = special
		} else {
			commands = append(commands, special)
		}
	}

	return console.NewGroup(name, "Manage "+plural(d)).Register(commands...)
}

// specials are the commands a resource's group has beside those built from
// its description, called path in full.
func specials(cli *command.Cli, d resource.Definition, path string) []console.Command {
	switch d.Name {
	case "file":
		return []console.Command{newUploadCommand(cli, path), newDownloadCommand(cli, path)}
	case "user":
		return []console.Command{
			newSetPasswordCommand(cli, path),
			newSetRolesCommand(cli, path),
			newSetSubscriptionCommand(cli, path),
			newUserSessionsCommand(cli, path),
			newSignOutCommand(cli, path),
			newImpersonateCommand(cli, path),
		}
	case "audit-log":
		return []console.Command{newActivityCommand(cli, d, path)}
	case "attendable-exam":
		return []console.Command{
			newShowCommand(cli, "attempts", "Display one's attempts at an exam, and whether one may start another", path+" attempts [OPTIONS] UUID", "/api/exams/{uuid}/attempts"),
		}
	case "exam-attempt":
		return []console.Command{
			newStartAttemptCommand(cli, path),
			newShowCommand(cli, "show", "Display one's attempt as one attends it: its current page, whose time starts as it is shown", path+" show [OPTIONS] UUID", "/api/exam-attempts/{uuid}"),
			newAnswerAttemptCommand(cli, path),
		}
	default:
		return nil
	}
}

// newProfileGroup is the group of commands about the one signed in.
func newProfileGroup(cli *command.Cli) *console.Group {
	path := command.Name + " profile"

	return console.NewGroup("profile", "Manage the profile of the one signed in").
		Register(
			newShowCommand(cli, "show", "Display one's profile", path+" show [OPTIONS]", "/api/dashboard/profile"),
			newSetCommand(cli, "update", "Update one's profile from a JSON file", path+" update -f FILE", "/api/dashboard/profile"),
			newChangePasswordCommand(cli, path),
			newListCommand(cli, lookup("role"), path, named("roles", "List the roles one holds"), forAudience(resource.AudienceMine)),
			newShowCommand(cli, "subscription", "Display the subscription one is on, what one holds of it, and what is past its limits", path+" subscription [OPTIONS]", "/api/dashboard/profile/subscription"),
		).
		RegisterGroup(newGroup(cli, lookup("session"), "sessions", path+" sessions"))
}

// newConfigGroup is the group of commands about the estate's settings.
func newConfigGroup(cli *command.Cli) *console.Group {
	path := command.Name + " config"

	return console.NewGroup("config", "Manage the estate's settings: its default language, roles and subscription").
		Register(
			newShowCommand(cli, "show", "Display the settings", path+" show [OPTIONS]", "/api/dashboard/config"),
			newSetCommand(cli, "set", "Change the settings from a JSON file", path+" set -f FILE", "/api/dashboard/config"),
		)
}

// lookup is the resource of that name, which is described: a resource that
// is not is a mistake in the wiring, not something a user did.
func lookup(name string) resource.Definition {
	d, found := resource.Lookup(name)
	if !found {
		panic(fmt.Sprintf("content: no resource is named %q", name))
	}

	return d
}

// actionsByName are a resource's actions, those of one name together, in
// the order their names first come.
func actionsByName(d resource.Definition) [][]resource.Action {
	var grouped [][]resource.Action

	for _, action := range d.Actions {
		at := slices.IndexFunc(grouped, func(actions []resource.Action) bool { return actions[0].Name == action.Name })
		if at < 0 {
			grouped = append(grouped, []resource.Action{action})
		} else {
			grouped[at] = append(grouped[at], action)
		}
	}

	return grouped
}

// plural is what more than one of a resource is called in a sentence.
func plural(d resource.Definition) string {
	return strings.ToLower(d.Title)
}

// singular is what one of a resource is called in a sentence.
func singular(d resource.Definition) string {
	return strings.ReplaceAll(d.Name, "-", " ")
}

// indefinite is one of a resource, as "an article" or "a user".
func indefinite(d resource.Definition) string {
	if strings.ContainsRune("aeio", rune(d.Name[0])) {
		return "an " + singular(d)
	}

	return "a " + singular(d)
}

// flagName is the flag a query parameter or a field is given as.
func flagName(parameter string) string {
	return strings.ReplaceAll(parameter, "_", "-")
}

// usageError is a command asked for wrongly.
func usageError(flags *console.FlagSet, format string, arguments ...any) error {
	return command.UsageError{Command: flags.Name(), Message: fmt.Sprintf(format, arguments...)}
}

// parseKeys are the keys a command was given, each as d takes it.
func parseKeys(flags *console.FlagSet, d resource.Definition) ([][]string, error) {
	keys := make([][]string, flags.NArg())

	for i, argument := range flags.Args() {
		key, err := d.ParseKey(argument)
		if err != nil {
			return nil, usageError(flags, "%s", err)
		}

		keys[i] = key
	}

	return keys, nil
}

// requestBody is what a request carries: nil, sent as no body, when fields
// say nothing.
func requestBody(fields map[string]any) any {
	if len(fields) == 0 {
		return nil
	}

	return fields
}
