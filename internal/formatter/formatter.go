// Package formatter writes what commands answer as docker's --format does:
// a table by default, JSON when asked for "json", and a Go template of the
// user's otherwise, which a "table " prefix lays out as a table with
// headers.
package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"
	"text/template/parse"
)

const (
	// TableFormat is the default table of a listing, or a table of the
	// template that follows it.
	TableFormat = "table"

	// JSONFormat is every item as JSON, one per line.
	JSONFormat = "json"
)

// Options say how a listing is written.
type Options struct {
	// Format is what --format was given: empty or "table" for the default
	// table, "json", "table <template>" or a template.
	Format string

	// Table is the default table, as "{{.uuid}}\t{{.name}}": one template
	// per column, separated by tabs.
	Table string

	// Quiet writes the template of ID alone, one item per line.
	Quiet bool

	// ID is the template of what identifies an item, which Quiet writes.
	ID string

	// Headers name the columns whose header is not their field's name in
	// capitals, by field.
	Headers map[string]string
}

// Write writes items as options say.
func Write(out io.Writer, items []any, options Options) error {
	format := strings.TrimSpace(options.Format)

	switch {
	case options.Quiet:
		return writeLines(out, items, options.ID)
	case format == JSONFormat:
		return writeJSONLines(out, items)
	case format == "" || format == TableFormat:
		return writeTable(out, items, options.Table, options.Headers)
	case strings.HasPrefix(format, TableFormat+" "):
		return writeTable(out, items, strings.TrimPrefix(format, TableFormat+" "), options.Headers)
	default:
		return writeLines(out, items, format)
	}
}

// WriteJSON writes value as indented JSON, as inspect writes what it
// inspects.
func WriteJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "    ")
	encoder.SetEscapeHTML(false)

	return encoder.Encode(value)
}

// WriteTemplate writes value through format, as inspect does when given
// --format.
func WriteTemplate(out io.Writer, value any, format string) error {
	return writeLines(out, []any{value}, format)
}

func writeJSONLines(out io.Writer, items []any) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)

	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}

	return nil
}

func writeLines(out io.Writer, items []any, format string) error {
	tmpl, err := Parse(format)
	if err != nil {
		return err
	}

	for _, item := range items {
		var line bytes.Buffer
		if err := tmpl.Execute(&line, item); err != nil {
			return fmt.Errorf("writing %q: %w", format, err)
		}

		if _, err := fmt.Fprintln(out, line.String()); err != nil {
			return err
		}
	}

	return nil
}

func writeTable(out io.Writer, items []any, format string, headers map[string]string) error {
	format = expandTabs(format)

	tmpl, err := Parse(format)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(out, 10, 1, 3, ' ', 0)

	if _, err := fmt.Fprintln(writer, Header(format, headers)); err != nil {
		return err
	}

	for _, item := range items {
		var row bytes.Buffer
		if err := tmpl.Execute(&row, item); err != nil {
			return fmt.Errorf("writing %q: %w", format, err)
		}

		if _, err := fmt.Fprintln(writer, row.String()); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// Parse parses a template of the user's, with the functions docker's
// templates have and the CLI's own.
func Parse(format string) (*template.Template, error) {
	tmpl, err := template.New("format").Option("missingkey=zero").Funcs(Funcs()).Parse(expandTabs(format))
	if err != nil {
		return nil, fmt.Errorf("parsing the format %q: %w", format, err)
	}

	return tmpl, nil
}

// Header is the header row of a table: each column named after the last
// field its template reads, in capitals, unless headers names it.
func Header(format string, headers map[string]string) string {
	columns := strings.Split(expandTabs(format), "\t")
	names := make([]string, len(columns))

	for i, column := range columns {
		names[i] = headerOf(column, headers)
	}

	return strings.Join(names, "\t")
}

func headerOf(column string, headers map[string]string) string {
	field := lastField(column)
	if len(field) == 0 {
		return strings.ToUpper(strings.TrimSpace(column))
	}

	if header, found := headers[field]; found {
		return header
	}

	return strings.ToUpper(strings.NewReplacer("_", " ", "-", " ").Replace(field))
}

func lastField(column string) string {
	trees, err := parse.Parse("column", column, "{{", "}}", Funcs())
	if err != nil {
		return ""
	}

	var found string

	var walk func(parse.Node)
	walk = func(node parse.Node) {
		if len(found) > 0 || node == nil {
			return
		}

		switch n := node.(type) {
		case *parse.ListNode:
			for _, child := range n.Nodes {
				walk(child)
			}
		case *parse.ActionNode:
			walk(n.Pipe)
		case *parse.PipeNode:
			for _, command := range n.Cmds {
				walk(command)
			}
		case *parse.CommandNode:
			for _, argument := range n.Args {
				walk(argument)
			}
		case *parse.FieldNode:
			found = n.Ident[len(n.Ident)-1]
		case *parse.ChainNode:
			if len(n.Field) > 0 {
				found = n.Field[len(n.Field)-1]
			} else {
				walk(n.Node)
			}
		case *parse.IfNode:
			walk(n.Pipe)
		case *parse.RangeNode:
			walk(n.Pipe)
		case *parse.WithNode:
			walk(n.Pipe)
		}
	}

	if tree, exists := trees["column"]; exists {
		walk(tree.Root)
	}

	return found
}

func expandTabs(format string) string {
	return strings.ReplaceAll(format, `\t`, "\t")
}
