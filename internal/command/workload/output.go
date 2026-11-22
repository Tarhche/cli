package workload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// shortID is how long docker shows an id, unless it is asked not to
// truncate it.
const shortID = 12

// writeRows writes rows as a listing: a table by default, or as options say.
func writeRows[T any](out io.Writer, rows []T, options formatter.Options) error {
	items := make([]any, len(rows))
	for i, row := range rows {
		items[i] = row
	}

	return formatter.Write(out, items, options)
}

// writeInspected writes what inspect found as docker does: an array of it
// as the API answered it, or each through format, one per line.
func writeInspected(out io.Writer, found []json.RawMessage, format string) error {
	if len(strings.TrimSpace(format)) == 0 {
		if found == nil {
			found = []json.RawMessage{}
		}

		return formatter.WriteJSON(out, found)
	}

	for _, raw := range found {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()

		var value any
		if err := decoder.Decode(&value); err != nil {
			return err
		}

		if err := formatter.WriteTemplate(out, value, format); err != nil {
			return err
		}
	}

	return nil
}

// stops reports whether err is about every reference a command was given,
// which stops it at once, rather than about the one it was acting on.
func stops(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, auth.ErrNotLoggedIn) ||
		api.IsUnauthorized(err) ||
		api.IsForbidden(err)
}

// failedSome is how a command that said each failure on its own fails once
// it is done.
var failedSome = command.StatusError{Code: console.ExitFailure}

// eachReference acts on every reference in turn, as docker does: it writes
// each it acted on, and says each that failed on its own and goes on, and
// fails once it is done when one did. What fails every one of them stops it
// at once.
func eachReference(cli *command.Cli, references []string, act func(reference string) error) error {
	failed := false

	for _, reference := range references {
		if err := act(reference); err != nil {
			if stops(err) {
				return err
			}

			cli.Exit(err)
			failed = true

			continue
		}

		if _, err := fmt.Fprintln(cli.Out, reference); err != nil {
			return err
		}
	}

	if failed {
		return failedSome
	}

	return nil
}

// inspectEach writes what find finds of every reference, saying each it
// does not find on its own.
func inspectEach(cli *command.Cli, references []string, format string, find func(reference string) (json.RawMessage, error)) error {
	var (
		found  []json.RawMessage
		failed bool
	)

	for _, reference := range references {
		raw, err := find(reference)
		if err != nil {
			if stops(err) {
				return err
			}

			cli.Exit(err)
			failed = true

			continue
		}

		found = append(found, raw)
	}

	if err := writeInspected(cli.Out, found, format); err != nil {
		return err
	}

	if failed {
		return failedSome
	}

	return nil
}

// warnRemoval says what still needs something that was asked to be deleted,
// when anything does.
func warnRemoval(out io.Writer, reference string, removal workload.Removal) error {
	if len(removal.Dependents) == 0 {
		return nil
	}

	needing := make([]string, len(removal.Dependents))
	for i, d := range removal.Dependents {
		needing[i] = d.Kind + " " + orDefault(d.Name, d.UUID)
	}

	if removal.Recreated {
		_, err := fmt.Fprintf(out, "Warning: %s is needed by %s, which will make it again: delete them with it with --cascade.\n", reference, strings.Join(needing, ", "))

		return err
	}

	_, err := fmt.Fprintf(out, "Warning: %s is needed by %s.\n", reference, strings.Join(needing, ", "))

	return err
}

// sayMadeVM says a Docker VM was made for a container or a stack, when one
// was.
func sayMadeVM(out io.Writer, vm workload.ChosenVM) error {
	if !vm.Created {
		return nil
	}

	_, err := fmt.Fprintf(out, "Made Docker VM %s for it.\n", orDefault(vm.Name, vm.UUID))

	return err
}

// truncated is id as docker shows it: its first twelve characters, unless
// untruncated says otherwise.
func truncated(id string, untruncated bool) string {
	if untruncated || len(id) <= shortID {
		return id
	}

	return id[:shortID]
}

// ellipsis is s cut to at most length characters, the last of them an
// ellipsis when it was cut.
func ellipsis(s string, length int) string {
	if utf8.RuneCountInString(s) <= length {
		return s
	}

	return string([]rune(s)[:length-1]) + "…"
}

// orDefault is value, or fallback when it is empty.
func orDefault(value, fallback string) string {
	if len(value) == 0 {
		return fallback
	}

	return value
}

// ownerOf is who owner is, as anybody's listings say.
func ownerOf(owner *workload.Owner, ownerUUID string) string {
	if owner == nil {
		return ownerUUID
	}

	return orDefault(owner.Username, orDefault(owner.Name, owner.UUID))
}
