package formatter

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("closed")
}

func items() []any {
	return []any{
		map[string]any{"uuid": "1", "name": "web", "created_at": "2026-11-20T10:00:00Z", "metadata": map[string]any{"slug": "web-x"}},
		map[string]any{"uuid": "2", "name": "db", "created_at": "", "metadata": map[string]any{"slug": "db-y"}},
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("the default table, with headers", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, items(), Options{Table: `{{.uuid}}\t{{.name}}\t{{.metadata.slug}}`, Headers: map[string]string{"uuid": "ID"}}))

		assert.Equal(t, "ID        NAME      SLUG\n1         web       web-x\n2         db        db-y\n", out.String())
	})

	t.Run("table is the default table", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, items(), Options{Format: "table", Table: "{{.name}}"}))

		assert.Equal(t, "NAME\nweb\ndb\n", out.String())
	})

	t.Run("a table of the user's", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, items(), Options{Format: `table {{.name}}\t{{.uuid}}`, Table: "{{.uuid}}"}))

		assert.Equal(t, "NAME      UUID\nweb       1\ndb        2\n", out.String())
	})

	t.Run("json, one item per line", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, []any{map[string]string{"a": "<b>"}}, Options{Format: "json"}))

		assert.Equal(t, "{\"a\":\"<b>\"}\n", out.String())
	})

	t.Run("a template of the user's, one item per line", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, items(), Options{Format: `{{.name}}: {{.metadata.slug}}`}))

		assert.Equal(t, "web: web-x\ndb: db-y\n", out.String())
	})

	t.Run("quiet writes what identifies an item", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, Write(&out, items(), Options{Format: "json", Quiet: true, ID: "{{.uuid}}"}))

		assert.Equal(t, "1\n2\n", out.String())
	})

	t.Run("a template that does not parse is an error", func(t *testing.T) {
		t.Parallel()

		assert.ErrorContains(t, Write(&bytes.Buffer{}, items(), Options{Format: "{{.name"}), "parsing the format")
		assert.ErrorContains(t, Write(&bytes.Buffer{}, items(), Options{Table: "{{.name"}), "parsing the format")
	})

	t.Run("a template that fails is an error", func(t *testing.T) {
		t.Parallel()

		assert.ErrorContains(t, Write(&bytes.Buffer{}, items(), Options{Format: "{{.name.nested.deeper}}"}), "writing")
		assert.ErrorContains(t, Write(&bytes.Buffer{}, items(), Options{Format: "table {{.name.nested.deeper}}"}), "writing")
	})

	t.Run("a writer that fails is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, Write(failingWriter{}, items(), Options{Format: "{{.name}}"}))
		assert.Error(t, Write(failingWriter{}, items(), Options{Format: "json"}))
		assert.Error(t, Write(failingWriter{}, items(), Options{Table: "{{.name}}"}))
	})

	t.Run("json that cannot be encoded is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, Write(&bytes.Buffer{}, []any{make(chan int)}, Options{Format: "json"}))
	})
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, WriteJSON(&out, []any{map[string]string{"a": "<b>"}}))

	assert.Equal(t, "[\n    {\n        \"a\": \"<b>\"\n    }\n]\n", out.String())
}

func TestWriteTemplate(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, WriteTemplate(&out, map[string]string{"name": "web"}, "{{.name}}"))

	assert.Equal(t, "web\n", out.String())
}

func TestHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		format string
		header string
	}{
		{name: "fields in capitals", format: `{{.name}}\t{{.created_at}}`, header: "NAME\tCREATED AT"},
		{name: "the last field of a chain", format: "{{.metadata.slug}}", header: "SLUG"},
		{name: "a field through a function", format: "{{ago .created_at}}", header: "CREATED AT"},
		{name: "a field piped into a function", format: "{{.size | bytes}}", header: "SIZE"},
		{name: "a field inside an if", format: "{{if .docker}}docker{{end}}", header: "DOCKER"},
		{name: "a field inside a range", format: "{{range .ports}}{{.}}{{end}}", header: "PORTS"},
		{name: "a field inside a with", format: "{{with .owner}}{{.name}}{{end}}", header: "OWNER"},
		{name: "a field of a parenthesised pipeline", format: "{{(.status).state}}", header: "STATE"},
		{name: "a column that does not parse", format: "{{.name", header: "{{.NAME"},
		{name: "a field through a builtin", format: "{{len .ports}}", header: "PORTS"},
		{name: "a field compared", format: `{{if eq .state "running"}}up{{end}}`, header: "STATE"},
		{name: "a field negated", format: "{{if not .docker}}machine{{end}}", header: "DOCKER"},
		{name: "a key read with index", format: `{{index . "created-at"}}`, header: "CREATED AT"},
		{name: "a key read with index of a field", format: `{{index .labels "workload.stack"}}`, header: "WORKLOAD.STACK"},
		{name: "index of something that is not a string", format: "{{index .ports 0}}", header: "PORTS"},
		{name: "text alone", format: "fixed", header: "FIXED"},
		{name: "an override", format: "{{.uuid}}", header: "ID"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.header, Header(c.format, map[string]string{"uuid": "ID"}))
		})
	}
}

func TestFuncs(t *testing.T) {
	t.Parallel()

	render := func(t *testing.T, format string, data any) string {
		t.Helper()

		var out bytes.Buffer
		require.NoError(t, WriteTemplate(&out, data, format))

		return out.String()
	}

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "{\"a\":1}\n", render(t, "{{json .}}", map[string]int{"a": 1}))
		assert.Error(t, WriteTemplate(&bytes.Buffer{}, map[string]any{"c": make(chan int)}, "{{json .c}}"))
	})

	t.Run("join", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "a,b\n", render(t, `{{join .s ","}}`, map[string]any{"s": []string{"a", "b"}}))
		assert.Equal(t, "1,x\n", render(t, `{{join .s ","}}`, map[string]any{"s": []any{1, "x"}}))
		assert.Equal(t, "\n", render(t, `{{join .s ","}}`, map[string]any{"s": nil}))
		assert.Equal(t, "7\n", render(t, `{{join .s ","}}`, map[string]any{"s": 7}))
	})

	t.Run("lower, upper, title and split", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "ab AB Hello World [a b]\n", render(t, `{{lower "aB"}} {{upper "aB"}} {{title "hello world"}} {{split "a,b" ","}}`, nil))
	})

	t.Run("truncate and pad", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "سلا|abc| x  |\n", render(t, `{{truncate "سلام" 3}}|{{truncate "abc" 5}}|{{pad "x" 1 2}}|`, nil))
		assert.Equal(t, "|\n", render(t, `{{truncate .missing 3}}|`, map[string]any{}))
	})

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "none none x 0\n", render(t, `{{default "none" .missing}} {{default "none" .empty}} {{default "none" .x}} {{default "none" .zero}}`, map[string]any{"empty": "", "x": "x", "zero": 0}))
	})

	t.Run("bytes", func(t *testing.T) {
		t.Parallel()

		cases := map[any]string{
			512:                    "512B",
			int32(2048):            "2KiB",
			int64(1536):            "1.5KiB",
			uint(1 << 20):          "1MiB",
			uint32(1 << 30):        "1GiB",
			uint64(5 << 40):        "5TiB",
			float32(1024):          "1KiB",
			float64(1 << 60):       "1024PiB",
			json.Number("1048576"): "1MiB",
			json.Number("x"):       "",
			"4096":                 "4KiB",
			"x":                    "",
			true:                   "",
			float64(123456789):     "117.74MiB",
		}

		for value, said := range cases {
			assert.Equal(t, said, Bytes(value), "%T %v", value, value)
		}
	})

	t.Run("duration", func(t *testing.T) {
		t.Parallel()

		cases := map[any]string{
			45:                     "45 seconds",
			json.Number("259200"):  "3 days",
			uint64(31536000):       "12 months",
			"3600":                 "About an hour",
			0:                      "",
			-5:                     "",
			"x":                    "",
			json.Number("forever"): "",
		}

		for value, said := range cases {
			assert.Equal(t, said, Duration(value), "%T %v", value, value)
		}

		assert.Equal(t, "Forever\n", render(t, `{{default "Forever" (duration .validity_seconds)}}`, map[string]any{}))
	})
}

func TestAgo(t *testing.T) {
	t.Parallel()

	t.Run("counts from now", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "5 hours ago", Ago(time.Now().Add(-5*time.Hour)))
	})

	moment := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		value any
		said  string
	}{
		{name: "a moment ago", value: moment, said: "Less than a second ago"},
		{name: "one second", value: moment.Add(-1500 * time.Millisecond), said: "1 second ago"},
		{name: "seconds", value: moment.Add(-30 * time.Second), said: "30 seconds ago"},
		{name: "a minute", value: moment.Add(-90 * time.Second), said: "About a minute ago"},
		{name: "minutes", value: moment.Add(-10 * time.Minute), said: "10 minutes ago"},
		{name: "an hour", value: moment.Add(-70 * time.Minute), said: "About an hour ago"},
		{name: "hours", value: moment.Add(-5 * time.Hour), said: "5 hours ago"},
		{name: "days", value: moment.Add(-72 * time.Hour), said: "3 days ago"},
		{name: "weeks", value: moment.Add(-21 * 24 * time.Hour), said: "3 weeks ago"},
		{name: "months", value: moment.Add(-90 * 24 * time.Hour), said: "3 months ago"},
		{name: "years", value: moment.Add(-3 * 365 * 24 * time.Hour), said: "3 years ago"},
		{name: "the future", value: moment.Add(10 * time.Minute), said: "in 10 minutes"},
		{name: "about an hour ahead", value: moment.Add(70 * time.Minute), said: "in about an hour"},
		{name: "a moment ahead, as another clock says it", value: moment.Add(20 * time.Second), said: "Just now"},
		{name: "a pointer", value: new(moment.Add(-10 * time.Minute)), said: "10 minutes ago"},
		{name: "a nil pointer", value: (*time.Time)(nil), said: ""},
		{name: "an rfc 3339 string", value: moment.Add(-5 * time.Hour).Format(time.RFC3339), said: "5 hours ago"},
		{name: "a string that is not a time", value: "yesterday", said: ""},
		{name: "the zero time", value: time.Time{}, said: ""},
		{name: "something else", value: 7, said: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.said, agoFrom(moment, c.value))
		})
	}
}
