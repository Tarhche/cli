package command

import "github.com/danceable/console"

// alias is a command under another name.
type alias struct {
	console.Command
	name  string
	usage string
}

// Alias is cmd under another name and usage, as docker's ps is container
// ls. The console has no aliases of its own, so the same command is
// registered twice, once under each name.
func Alias(cmd console.Command, name, usage string) console.Command {
	return alias{Command: cmd, name: name, usage: usage}
}

func (a alias) Name() string {
	return a.name
}

func (a alias) Usage() string {
	return a.usage
}
