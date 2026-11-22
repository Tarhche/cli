package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/command"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("its numbers as they were written", func(t *testing.T) {
		t.Parallel()

		value, err := decode([]byte(`{"size":31536000}`))

		require.NoError(t, err)
		assert.Equal(t, map[string]any{"size": json.Number("31536000")}, value)
	})

	t.Run("nothing at all is null", func(t *testing.T) {
		t.Parallel()

		value, err := decode([]byte(" \n"))

		require.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("what is not JSON", func(t *testing.T) {
		t.Parallel()

		_, err := decode([]byte(`{`))

		assert.Error(t, err)
	})
}

func TestDecodeAll(t *testing.T) {
	t.Parallel()

	t.Run("each item", func(t *testing.T) {
		t.Parallel()

		items, err := decodeAll([]json.RawMessage{json.RawMessage(`1`), json.RawMessage(`"a"`)})

		require.NoError(t, err)
		assert.Equal(t, []any{json.Number("1"), "a"}, items)
	})

	t.Run("an item that is not JSON", func(t *testing.T) {
		t.Parallel()

		_, err := decodeAll([]json.RawMessage{json.RawMessage(`{`)})

		assert.ErrorContains(t, err, "reading the answer")
	})
}

func TestReadObject(t *testing.T) {
	t.Parallel()

	cli := func(in string) *command.Cli {
		return command.NewCli(strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{})
	}

	t.Run("the object on the standard input", func(t *testing.T) {
		t.Parallel()

		object, err := readObject(cli(`{"a":1}`), "-")

		require.NoError(t, err)
		assert.Equal(t, map[string]any{"a": json.Number("1")}, object)
	})

	t.Run("a standard input that holds none", func(t *testing.T) {
		t.Parallel()

		_, err := readObject(cli(`[]`), "-")

		assert.EqualError(t, err, "the standard input holds no JSON object")
	})

	t.Run("a standard input that is not JSON", func(t *testing.T) {
		t.Parallel()

		_, err := readObject(cli(`{`), "-")

		assert.ErrorContains(t, err, "reading the standard input")
	})

	t.Run("a standard input that cannot be read", func(t *testing.T) {
		t.Parallel()

		_, err := readObject(command.NewCli(failingReader{}, nil, nil), "-")

		assert.ErrorContains(t, err, "reading the standard input: broken")
	})
}

func TestWithDefaults(t *testing.T) {
	t.Parallel()

	body := withDefaults(map[string]any{"object_type": "video", "body": "Hi"}, map[string]any{"object_type": "article", "approved": false})

	assert.Equal(t, map[string]any{"object_type": "video", "body": "Hi", "approved": false}, body)
}

func TestIdentify(t *testing.T) {
	t.Parallel()

	article := lookup("article")

	t.Run("the key answered", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "c1/en", identify(article, json.RawMessage(`{"correlation_uuid":"c1","language_code":"en"}`)))
	})

	for name, answer := range map[string]string{
		"nothing":                 ``,
		"no object":               `[]`,
		"part of the key":         `{"correlation_uuid":"c1"}`,
		"an empty field":          `{"correlation_uuid":"c1","language_code":""}`,
		"a field of another type": `{"correlation_uuid":"c1","language_code":1}`,
	} {
		t.Run("none of "+name, func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, identify(article, json.RawMessage(answer)))
		})
	}
}

func TestFailed(t *testing.T) {
	t.Parallel()

	t.Run("no such thing when the api found none", func(t *testing.T) {
		t.Parallel()

		err := failed(lookup("license-template"), []string{"t1"}, &api.Error{StatusCode: 404})

		assert.EqualError(t, err, "no such license template: t1")
	})

	t.Run("anything else, said of the thing", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("broken")
		err := failed(lookup("article"), []string{"c1", "en"}, cause)

		assert.EqualError(t, err, "c1/en: broken")
		assert.ErrorIs(t, err, cause)
	})
}

func TestWriteOne(t *testing.T) {
	t.Parallel()

	t.Run("an answer that is not JSON, on a line", func(t *testing.T) {
		t.Parallel()

		err := writeOne(&bytes.Buffer{}, json.RawMessage(`{`), "json")

		assert.ErrorContains(t, err, "reading the answer")
	})

	t.Run("an answer that is not JSON, through a template", func(t *testing.T) {
		t.Parallel()

		err := writeOne(&bytes.Buffer{}, json.RawMessage(`{`), "{{.}}")

		assert.ErrorContains(t, err, "reading the answer")
	})

	t.Run("nothing is null, through a template too", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, writeOne(&out, nil, "json"))

		assert.Equal(t, "null\n", out.String())
	})
}

// failingReader is a reader that cannot be read.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("broken")
}
