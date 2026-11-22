// Package resource describes what the dashboard manages, as the API serves
// it: where each kind of thing is listed, read, created, changed and
// deleted, what identifies one, and how a listing of them is shown. The
// CLI's generic commands and the desktop app's generic pages are both built
// from it.
package resource

import (
	"slices"
	"strings"
)

// Sections group resources in the desktop app's navigation.
const (
	SectionContent        = "Content"
	SectionLearning       = "Learning"
	SectionAdministration = "Administration"
	SectionAccount        = "Account"
)

// Paths are the routes of one audience of a resource: everybody's, which an
// administrator manages, or one's own. A route that is empty is not served.
// Get and Delete name the resource's key fields in braces, as
// "/api/dashboard/articles/{correlation_uuid}/{language_code}".
type Paths struct {
	List   string
	Get    string
	Create string
	Update string
	Delete string
}

// Action is something more than reading, creating, changing and deleting
// that a resource can be asked: a contact message marked read, say.
type Action struct {
	// Name is the command and button it is offered as: "read".
	Name        string
	Description string

	Method string
	// Path names the key fields in braces, as Paths.Get does.
	Path string

	// Mine says it is one of one's own routes rather than everybody's.
	Mine bool
}

// Definition is a kind of thing the dashboard manages.
type Definition struct {
	// Name is the singular the CLI's group is named after: "article".
	Name string
	// Plural is how the API names more than one: "articles".
	Plural string
	// Title is what the desktop app calls them: "Articles".
	Title       string
	Description string
	Section     string

	// Key are the fields that identify one, in the order a command is given
	// them: ["uuid"], or ["correlation_uuid", "language_code"].
	Key []string

	Everyone Paths
	Mine     Paths

	Actions []Action

	// Table is a listing's default table, one template per column separated
	// by tabs, over an item as the API answers it.
	Table string

	// Headers name the columns whose header is not their field's name in
	// capitals, by field.
	Headers map[string]string
}

// Path is path with the key fields in braces replaced by key, in order.
func (d Definition) Path(path string, key []string) string {
	for i, field := range d.Key {
		if i < len(key) {
			path = strings.ReplaceAll(path, "{"+field+"}", key[i])
		}
	}

	return path
}

var definitions []Definition

// All are every resource described, in the order they are shown.
func All() []Definition {
	return slices.Clone(definitions)
}

// Lookup is the resource of that name, singular or plural.
func Lookup(name string) (Definition, bool) {
	for _, d := range definitions {
		if d.Name == name || d.Plural == name {
			return d, true
		}
	}

	return Definition{}, false
}
