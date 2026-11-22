package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// stdin is the file name that stands for the standard input.
const stdin = "-"

// decode is JSON content as a value, its numbers as they were written, so
// that a template writes 31536000 rather than 3.1536e+07. Nothing at all
// is null.
func decode(content []byte) (any, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	return value, nil
}

// decodeAll are items as decode reads each.
func decodeAll(raws []json.RawMessage) ([]any, error) {
	items := make([]any, len(raws))

	for i, raw := range raws {
		item, err := decode(raw)
		if err != nil {
			return nil, fmt.Errorf("reading the answer: %w", err)
		}

		items[i] = item
	}

	return items, nil
}

// readObject is the JSON object in file, or on the standard input for "-".
// A file holding an array of one object, as inspect writes it, holds that
// object.
func readObject(cli *command.Cli, file string) (map[string]any, error) {
	name := file

	var (
		content []byte
		err     error
	)

	if file == stdin {
		name = "the standard input"
		content, err = io.ReadAll(cli.In)
	} else {
		content, err = os.ReadFile(file)
	}

	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}

	value, err := decode(content)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}

	if array, isArray := value.([]any); isArray && len(array) == 1 {
		value = array[0]
	}

	object, isObject := value.(map[string]any)
	if !isObject {
		return nil, fmt.Errorf("%s holds no JSON object", name)
	}

	return object, nil
}

// withDefaults is body with the fields of defaults it does not say itself.
func withDefaults(body map[string]any, defaults map[string]any) map[string]any {
	for field, value := range defaults {
		if _, said := body[field]; !said {
			body[field] = value
		}
	}

	return body
}

// identify is the key of d the API answered with, its fields joined by
// slashes, or nothing when it answered without one.
func identify(d resource.Definition, answer json.RawMessage) string {
	var fields map[string]any
	if err := json.Unmarshal(answer, &fields); err != nil {
		return ""
	}

	key := make([]string, len(d.Key))

	for i, field := range d.Key {
		value, isString := fields[field].(string)
		if !isString || len(value) == 0 {
			return ""
		}

		key[i] = value
	}

	return strings.Join(key, "/")
}

// failed is err, said of the thing of d that key names: there is no such
// thing when the API found none.
func failed(d resource.Definition, key []string, err error) error {
	if api.IsNotFound(err) {
		return fmt.Errorf("no such %s: %s", singular(d), strings.Join(key, "/"))
	}

	return fmt.Errorf("%s: %w", strings.Join(key, "/"), err)
}

// writeOne writes one thing the API answered with: indented JSON, or as
// format says, "json" on one line or a template of the user's.
func writeOne(out io.Writer, answer json.RawMessage, format string) error {
	if len(bytes.TrimSpace(answer)) == 0 {
		answer = json.RawMessage("null")
	}

	switch format = strings.TrimSpace(format); format {
	case "":
		return formatter.WriteJSON(out, answer)
	case formatter.JSONFormat:
		var line bytes.Buffer
		if err := json.Compact(&line, answer); err != nil {
			return fmt.Errorf("reading the answer: %w", err)
		}

		_, err := fmt.Fprintln(out, line.String())

		return err
	default:
		value, err := decode(answer)
		if err != nil {
			return fmt.Errorf("reading the answer: %w", err)
		}

		return formatter.WriteTemplate(out, value, format)
	}
}
