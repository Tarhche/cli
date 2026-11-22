package resource

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/formatter"
)

// placeholder is a key field a route names in braces.
var placeholder = regexp.MustCompile(`\{([a-z_]+)\}`)

// listed are items as each listing answers them, by resource and audience,
// in the shapes of the API's responses: what a table is written over.
var listed = map[string]map[Audience]string{
	"article": {
		AudienceEveryone: `[{"correlation_uuid":"c1","corrolated_items":[
			{"cover":"","video":"","title":"Hello\nworld","published_at":"2026-11-20T10:00:00Z","author":{"uuid":"u1","name":"Sarah","avatar":"","username":"sarah"},"language":{"code":"en","name":"English"}},
			{"cover":"","video":"","title":"سلام","published_at":"0001-01-01T00:00:00Z","author":{"uuid":"u1","name":"Sarah","avatar":"","username":"sarah"},"language":{"code":"fa","name":"Persian"}}]}]`,
		AudienceMine: `[{"correlation_uuid":"c2","corrolated_items":[
			{"cover":"","video":"","title":"Mine","published_at":"2026-11-20T10:00:00Z","author":{"uuid":"u1","name":"Sarah","avatar":"","username":"sarah"},"language":{"code":"en","name":"English"}}]}]`,
	},
	"comment": {
		AudienceEveryone: `[{"uuid":"m1","body":"first\nline","author":{"uuid":"u1","name":"Sarah","avatar":"","username":"sarah"},"object_type":"article","object_uuid":"c1","language_code":"en","approved_at":"0001-01-01T00:00:00Z","created_at":"2026-11-20T10:00:00Z"},
			{"uuid":"m2","body":"reply","author":{"uuid":"","name":"","avatar":"","username":""},"parent_uuid":"m1","object_type":"article","object_uuid":"c1","language_code":"en","approved_at":"2026-11-21T10:00:00Z","created_at":"2026-11-20T10:00:00Z"}]`,
		AudienceMine: `[{"uuid":"m1","body":"mine","object_type":"article","object_uuid":"c1","language_code":"en","approved_at":"0001-01-01T00:00:00Z","created_at":"2026-11-20T10:00:00Z"}]`,
	},
	"file": {
		AudienceEveryone: `[{"uuid":"f1","name":"cat.png","size":2048,"owner_uuid":"u1","mimetype":"image/png"}]`,
		AudienceMine:     `[{"uuid":"f2","name":"notes.txt","size":10,"owner_uuid":"u1","mimetype":"text/plain; charset=utf-8"}]`,
	},
	"element": {
		AudienceEveryone: `[{"uuid":"e1","body_type":"cards","venues":["/en/**","/fa/**"],"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-21T10:00:00Z"},
			{"uuid":"e2","body_type":"item","venues":null,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-21T10:00:00Z"}]`,
	},
	"contact": {
		AudienceEveryone: `[{"uuid":"k1","subject":"Hello","body":"Hi","email":"a@example.com","phone":"0912","read_at":"0001-01-01T00:00:00Z","created_at":"2026-11-20T10:00:00Z"},
			{"uuid":"k2","subject":"Bye","body":"Bye","read_at":"2026-11-21T10:00:00Z","created_at":"2026-11-20T10:00:00Z"}]`,
	},
	"question": {
		AudienceEveryone: `[{"uuid":"q1","language_code":"en","kind":"code","prompt":"Print\nhello","difficulty":"easy","time_limit_seconds":60,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
		AudienceMine:     `[{"uuid":"q2","language_code":"fa","kind":"text","prompt":"Why?","time_limit_seconds":0,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
	},
	"exam": {
		AudienceEveryone: `[{"uuid":"x1","language_code":"en","title":"Go","stages":2,"passing_score":70,"max_attempts":3,"license_template_uuid":"t1","attempts":{"total":5,"in_progress":1,"passed":2,"failed":2},"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
		AudienceMine:     `[{"uuid":"x2","language_code":"en","title":"Mine","stages":1,"passing_score":50,"max_attempts":0,"attempts":{"total":0,"in_progress":0,"passed":0,"failed":0},"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
	},
	"attendable-exam": {
		AudienceMine: `[{"uuid":"x1","language_code":"en","title":"Go","description":"","time_limit_seconds":600,"stages":1,"questions":10,"passing_score":70,"issues_license":true,"allows_ai_attempts":false,"attempts_left":null},
			{"uuid":"x2","language_code":"en","title":"Rust","description":"","time_limit_seconds":0,"stages":1,"questions":5,"passing_score":70,"issues_license":false,"allows_ai_attempts":true,"attempts_left":0,"in_progress_uuid":"a1"}]`,
	},
	"exam-attempt": {
		AudienceEveryone: `[{"uuid":"a1","exam_uuid":"x1","exam_title":"Go","user_uuid":"u1","first_name":"Sarah","last_name":"Hosseini","status":"passed","score":80,"passing_score":70,"license_uuid":"l1","started_at":"2026-11-20T10:00:00Z","finished_at":"2026-11-20T11:00:00Z"}]`,
		AudienceMine:     `[{"uuid":"a2","exam_uuid":"x1","exam_title":"Go","language_code":"en","status":"in_progress","passing_score":70,"started_at":"2026-11-20T10:00:00Z","deadline":"2026-11-20T11:00:00Z"}]`,
	},
	"license-template": {
		AudienceEveryone: `[{"uuid":"t1","title":"{topic} for {first_name}","validity_seconds":31536000,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
		AudienceMine:     `[{"uuid":"t2","title":"Mine","validity_seconds":0,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
	},
	"license": {
		AudienceEveryone: `[{"uuid":"l1","user_uuid":"u1","first_name":"Sarah","last_name":"Hosseini","exam_uuid":"x1","topic":"Go","title":"Go for Sarah","issued_at":"2026-11-20T10:00:00Z","expires_at":"2027-11-20T10:00:00Z","expired":false}]`,
		AudienceMine:     `[{"uuid":"l2","exam_uuid":"x1","topic":"Go","title":"Go","issued_at":"2026-11-20T10:00:00Z","expired":false}]`,
	},
	"user": {
		AudienceEveryone: `[{"uuid":"u1","name":"Sarah","avatar":"a.png","email":"s@example.com","username":"sarah"},{"uuid":"u2","email":"x@example.com","username":"x"}]`,
	},
	"role": {
		AudienceEveryone: `[{"uuid":"r1","translations":{"en":{"title":"Admin","description":"all"},"fa":{"title":"مدیر"}},"is_default":true},{"uuid":"r2","translations":{"en":{"title":"Writer"}},"is_default":false}]`,
		AudienceMine:     `[{"uuid":"r1","translations":{"en":{"title":"Admin"}},"statements":[{"effect":"allow","action":"*","resources":["*"]}]}]`,
	},
	"subscription": {
		AudienceEveryone: `[{"uuid":"s1","names":{"en":"Free","fa":"رایگان"},"quotas":{"vms":{"max":2,"cpus":2,"memory":1073741824,"disk":10737418240,"enforcement":{"mode":"soft"}},"snapshots":null,"uploads":{"max_size":104857600,"enforcement":{"mode":"hard","action":"delete","grace_seconds":3600}}},"roles":[],"is_default":true,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"},
			{"uuid":"s2","names":{"en":"Pro"},"quotas":{"vms":null,"snapshots":{"max":5,"enforcement":{"mode":"soft"}},"uploads":null},"roles":["r1"],"is_default":false,"created_at":"2026-11-20T10:00:00Z","updated_at":"2026-11-20T10:00:00Z"}]`,
	},
	"language": {
		AudienceEveryone: `[{"code":"en","name":"English","is_default":true},{"code":"fa","name":"Persian","is_default":false}]`,
	},
	"permission": {
		AudienceEveryone: `[{"name":"list of articles","value":"articles.index"}]`,
	},
	"audit-log": {
		AudienceEveryone: `[{"uuid":"g1","action":"articles.index","kind":"read","entity":{"type":"article"},"payload":{"page":"1"},"status":200,"user":{"uuid":"u1","username":"sarah"},"ip":"10.0.0.1","occurred_at":"2026-11-20T10:00:00Z"},
			{"uuid":"g2","action":"auth.login","kind":"create","entity":{"type":"session"},"status":401,"occurred_at":"2026-11-20T10:00:00Z"}]`,
		AudienceMine: `[{"uuid":"g3","action":"articles.delete","kind":"delete","entity":{"type":"article","id":"c1/en"},"status":204,"session_uuid":"s1","occurred_at":"2026-11-20T10:00:00Z"}]`,
	},
	"bookmark": {
		AudienceMine: `[{"title":"Hello","object_uuid":"c1","object_type":"article","language_code":"en","created_at":"2026-11-20T10:00:00Z"}]`,
	},
	"session": {
		AudienceMine: `[{"uuid":"s1","current":true,"device":{"browser":"Firefox","os":"Linux"},"ip":"10.0.0.1","created_at":"2026-11-20T10:00:00Z","last_activity":{"at":"2026-11-21T10:00:00Z","ip":"10.0.0.1"},"expires_at":"2026-12-20T10:00:00Z"},
			{"uuid":"s2","current":false,"device":{},"client":{"id":"cli"},"impersonated_user":{"uuid":"u2"},"created_at":"2026-11-20T10:00:00Z","last_activity":{"at":"0001-01-01T00:00:00Z"},"expires_at":"2026-12-20T10:00:00Z"}]`,
	},
}

func decode(t *testing.T, content string) []any {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()

	var items []any
	require.NoError(t, decoder.Decode(&items))

	return items
}

func TestPathsRoute(t *testing.T) {
	t.Parallel()

	paths := Paths{List: "/l", Get: "/g", Create: "/c", Update: "/u", Delete: "/d"}

	for op, want := range map[Operation]string{
		OperationList:   "/l",
		OperationGet:    "/g",
		OperationCreate: "/c",
		OperationUpdate: "/u",
		OperationDelete: "/d",
		"unknown":       "",
	} {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, paths.Route(op))
		})
	}
}

func TestActionAudience(t *testing.T) {
	t.Parallel()

	t.Run("one's own", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AudienceMine, Action{Mine: true}.Audience())
	})

	t.Run("everybody's", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, AudienceEveryone, Action{}.Audience())
	})
}

func TestDefinitionPath(t *testing.T) {
	t.Parallel()

	d := Definition{Key: []string{"correlation_uuid", "language_code"}}

	t.Run("every key field is replaced", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/articles/c1/en", d.Path("/articles/{correlation_uuid}/{language_code}", []string{"c1", "en"}))
	})

	t.Run("a field the key does not give is left", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/articles/c1/{language_code}", d.Path("/articles/{correlation_uuid}/{language_code}", []string{"c1"}))
	})
}

func TestDefinitionBodyKey(t *testing.T) {
	t.Parallel()

	d := Definition{Key: []string{"object_uuid", "language_code"}}

	t.Run("the fields the path does not name", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{"object_uuid": "c1", "language_code": "en"}, d.BodyKey("/bookmarks", []string{"c1", "en"}))
	})

	t.Run("none when the path names them all", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, d.BodyKey("/bookmarks/{object_uuid}/{language_code}", []string{"c1", "en"}))
	})

	t.Run("only the fields given", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{"object_uuid": "c1"}, d.BodyKey("/bookmarks", []string{"c1"}))
	})
}

func TestDefinitionPathsOf(t *testing.T) {
	t.Parallel()

	d := Definition{Everyone: Paths{List: "/all"}, Mine: Paths{List: "/my"}}

	t.Run("one's own", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/my", d.PathsOf(AudienceMine).List)
	})

	t.Run("everybody's", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/all", d.PathsOf(AudienceEveryone).List)
	})
}

func TestDefinitionAudiences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    Definition
		want []Audience
	}{
		{name: "both, one's own first", d: Definition{Everyone: Paths{List: "/all"}, Mine: Paths{List: "/my"}}, want: []Audience{AudienceMine, AudienceEveryone}},
		{name: "everybody's alone", d: Definition{Everyone: Paths{List: "/all"}}, want: []Audience{AudienceEveryone}},
		{name: "one's own alone", d: Definition{Mine: Paths{List: "/my"}}, want: []Audience{AudienceMine}},
		{name: "an action is served to its audience", d: Definition{Everyone: Paths{List: "/all"}, Actions: []Action{{Name: "x", Mine: true}}}, want: []Audience{AudienceMine, AudienceEveryone}},
		{name: "an action of everybody's", d: Definition{Actions: []Action{{Name: "x"}}}, want: []Audience{AudienceEveryone}},
		{name: "none", d: Definition{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.d.Audiences())
		})
	}
}

func TestDefinitionServes(t *testing.T) {
	t.Parallel()

	d := Definition{Everyone: Paths{List: "/all"}, Mine: Paths{Get: "/my/{uuid}"}}

	t.Run("served to everybody", func(t *testing.T) {
		t.Parallel()

		assert.True(t, d.Serves(OperationList))
	})

	t.Run("served to oneself", func(t *testing.T) {
		t.Parallel()

		assert.True(t, d.Serves(OperationGet))
	})

	t.Run("not served", func(t *testing.T) {
		t.Parallel()

		assert.False(t, d.Serves(OperationDelete))
	})
}

func TestDefinitionServesBoth(t *testing.T) {
	t.Parallel()

	d := Definition{Everyone: Paths{List: "/all", Create: "/all"}, Mine: Paths{List: "/my"}}

	t.Run("both", func(t *testing.T) {
		t.Parallel()

		assert.True(t, d.ServesBoth(OperationList))
	})

	t.Run("one", func(t *testing.T) {
		t.Parallel()

		assert.False(t, d.ServesBoth(OperationCreate))
	})
}

func TestDefinitionRoute(t *testing.T) {
	t.Parallel()

	d := Definition{
		Everyone: Paths{List: "/all", Create: "/all"},
		Mine:     Paths{List: "/my", Get: "/my/{uuid}"},
	}

	tests := []struct {
		name         string
		op           Operation
		everyone     bool
		wantPath     string
		wantAudience Audience
		wantServed   bool
	}{
		{name: "one's own by default", op: OperationList, wantPath: "/my", wantAudience: AudienceMine, wantServed: true},
		{name: "everybody's when asked", op: OperationList, everyone: true, wantPath: "/all", wantAudience: AudienceEveryone, wantServed: true},
		{name: "everybody's when one's own are not served", op: OperationCreate, wantPath: "/all", wantAudience: AudienceEveryone, wantServed: true},
		{name: "everybody's asked for and not served", op: OperationGet, everyone: true},
		{name: "not served at all", op: OperationDelete},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path, audience, served := d.Route(test.op, test.everyone)

			assert.Equal(t, test.wantPath, path)
			assert.Equal(t, test.wantAudience, audience)
			assert.Equal(t, test.wantServed, served)
		})
	}
}

func TestDefinitionFiltersOf(t *testing.T) {
	t.Parallel()

	d := Definition{Filters: []Filter{
		{Name: "exam_uuid"},
		{Name: "user_uuid", Audiences: []Audience{AudienceEveryone}},
	}}

	t.Run("everybody's listing takes every filter", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, d.Filters, d.FiltersOf(AudienceEveryone))
	})

	t.Run("one's own takes those of every audience", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []Filter{{Name: "exam_uuid"}}, d.FiltersOf(AudienceMine))
	})
}

func TestDefinitionKeyUsage(t *testing.T) {
	t.Parallel()

	t.Run("one field", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "UUID", Definition{Key: []string{"uuid"}}.KeyUsage())
	})

	t.Run("two fields", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "CORRELATION_UUID/LANGUAGE_CODE", Definition{Key: []string{"correlation_uuid", "language_code"}}.KeyUsage())
	})
}

func TestDefinitionParseKey(t *testing.T) {
	t.Parallel()

	d := Definition{Name: "article", Key: []string{"correlation_uuid", "language_code"}}

	t.Run("its fields joined by slashes", func(t *testing.T) {
		t.Parallel()

		key, err := d.ParseKey("c1/en")

		require.NoError(t, err)
		assert.Equal(t, []string{"c1", "en"}, key)
	})

	for _, given := range []string{"c1", "c1/en/x", "c1/", "/en", ".//en", "../en", "c1/."} {
		t.Run("not "+given, func(t *testing.T) {
			t.Parallel()

			_, err := d.ParseKey(given)

			require.ErrorIs(t, err, ErrInvalidKey)
			assert.Contains(t, err.Error(), "CORRELATION_UUID/LANGUAGE_CODE")
		})
	}
}

func TestDefinitionIDTemplate(t *testing.T) {
	t.Parallel()

	t.Run("its own", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "{{.id}}", Definition{Key: []string{"uuid"}, ID: "{{.id}}"}.IDTemplate())
	})

	t.Run("its key fields joined by slashes", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "{{.object_uuid}}/{{.language_code}}", Definition{Key: []string{"object_uuid", "language_code"}}.IDTemplate())
	})
}

func TestDefinitionKeyOf(t *testing.T) {
	t.Parallel()

	d := Definition{Name: "article", Plural: "articles", Key: []string{"correlation_uuid", "language_code"}, ID: "{{.correlation_uuid}}/{{.language.code}}"}

	t.Run("as its id says", func(t *testing.T) {
		t.Parallel()

		key, err := d.KeyOf(map[string]any{"correlation_uuid": "c1", "language": map[string]any{"code": "en"}})

		require.NoError(t, err)
		assert.Equal(t, []string{"c1", "en"}, key)
	})

	t.Run("a row without its key", func(t *testing.T) {
		t.Parallel()

		_, err := d.KeyOf(map[string]any{"correlation_uuid": "c1", "language": map[string]any{}})

		assert.ErrorIs(t, err, ErrInvalidKey)
	})

	t.Run("an id that cannot be read of the row", func(t *testing.T) {
		t.Parallel()

		_, err := d.KeyOf(map[string]any{"correlation_uuid": "c1"})

		assert.ErrorContains(t, err, "identifying the article")
	})

	t.Run("an id that is no template", func(t *testing.T) {
		t.Parallel()

		_, err := Definition{Plural: "things", Key: []string{"uuid"}, ID: "{{"}.KeyOf(map[string]any{})

		assert.ErrorContains(t, err, "parsing the id of things")
	})
}

func TestDefinitionRows(t *testing.T) {
	t.Parallel()

	t.Run("an item a row", func(t *testing.T) {
		t.Parallel()

		items := []any{map[string]any{"uuid": "a"}, map[string]any{"uuid": "b"}}

		assert.Equal(t, items, Definition{}.Rows(items))
	})

	t.Run("a version a row, with the item's fields beside it", func(t *testing.T) {
		t.Parallel()

		d := Definition{Versions: "versions"}
		items := []any{
			map[string]any{"correlation_uuid": "c1", "versions": []any{
				map[string]any{"title": "en", "correlation_uuid": "ignored"},
				map[string]any{"title": "fa"},
				"not a version",
			}},
			map[string]any{"correlation_uuid": "c2"},
			"not an item",
		}

		assert.Equal(t, []any{
			map[string]any{"correlation_uuid": "c1", "title": "en"},
			map[string]any{"correlation_uuid": "c1", "title": "fa"},
			"not an item",
		}, d.Rows(items))
	})
}

func TestAll(t *testing.T) {
	t.Parallel()

	all := All()
	require.NotEmpty(t, all)

	t.Run("it is a copy", func(t *testing.T) {
		t.Parallel()

		copied := All()
		copied[0].Name = "changed"

		assert.NotEqual(t, "changed", All()[0].Name)
	})

	t.Run("names are their own", func(t *testing.T) {
		t.Parallel()

		var names []string
		for _, d := range all {
			names = append(names, d.Name, d.Plural)
		}

		for _, name := range names {
			assert.Equal(t, 1, strings.Count(" "+strings.Join(names, " ")+" ", " "+name+" "), name)
		}
	})

	t.Run("sections come together, in their order", func(t *testing.T) {
		t.Parallel()

		var sections []string
		for _, d := range all {
			if len(sections) == 0 || sections[len(sections)-1] != d.Section {
				sections = append(sections, d.Section)
			}
		}

		assert.Equal(t, []string{SectionContent, SectionLearning, SectionAdministration, SectionAccount}, sections)
	})

	for _, d := range all {
		t.Run(d.Name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, d.Plural)
			assert.NotEmpty(t, d.Title)
			assert.NotEmpty(t, d.Description)
			assert.NotEmpty(t, d.Key)
			assert.True(t, d.Serves(OperationList), "every resource is listed")

			for _, audience := range d.Audiences() {
				paths := d.PathsOf(audience)

				for _, path := range []string{paths.List, paths.Get, paths.Create, paths.Update, paths.Delete} {
					assert.True(t, len(path) == 0 || strings.HasPrefix(path, "/api/"), path)

					for _, field := range placeholder.FindAllStringSubmatch(path, -1) {
						assert.Contains(t, d.Key, field[1], "%s names a field of no key", path)
					}
				}

				if len(paths.Get) > 0 {
					assert.Empty(t, d.BodyKey(paths.Get, d.Key), "%s is read with every key field in its path", paths.Get)
				}

				for _, path := range []string{paths.List, paths.Create} {
					assert.False(t, placeholder.MatchString(path), "%s names no key", path)
				}

				if len(paths.List) > 0 {
					listedAs(t, d, audience)
				}
			}

			for _, action := range d.Actions {
				assert.NotEmpty(t, action.Name)
				assert.NotEmpty(t, action.Description)
				assert.Contains(t, []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, action.Method)
				assert.True(t, strings.HasPrefix(action.Path, "/api/"), action.Path)

				for _, field := range placeholder.FindAllStringSubmatch(action.Path, -1) {
					assert.Contains(t, d.Key, field[1], "%s names a field of no key", action.Path)
				}

				twins := slices.IndexFunc(d.Actions, func(a Action) bool { return a.Name == action.Name && a.Mine == action.Mine })
				assert.Equal(t, action, d.Actions[twins], "an action is named once for each audience")
			}

			for _, filter := range d.Filters {
				assert.NotEmpty(t, filter.Name)
				assert.NotEmpty(t, filter.Description)
				assert.NotEqual(t, d.Cursor, filter.Name)

				for _, audience := range filter.Audiences {
					assert.Contains(t, d.Audiences(), audience)
				}
			}

			_, err := formatter.Parse(d.Table)
			require.NoError(t, err)

			_, err = formatter.Parse(d.IDTemplate())
			require.NoError(t, err)

			defaults := formatter.Header(d.Table, nil)
			for field := range d.Headers {
				assert.Contains(t, strings.Split(defaults, "\t"), strings.ToUpper(strings.NewReplacer("_", " ", "-", " ").Replace(field)), "the header of %s names a column", field)
			}
		})
	}
}

// listedAs checks that the table of d is written over the items the listing
// of audience answers, each a row of its own that identifies itself.
func listedAs(t *testing.T, d Definition, audience Audience) {
	t.Helper()

	fixture, found := listed[d.Name][audience]
	require.True(t, found, "the items %s of %s are listed as", audience, d.Plural)

	rows := d.Rows(decode(t, fixture))
	require.NotEmpty(t, rows)

	var out bytes.Buffer
	require.NoError(t, formatter.Write(&out, rows, formatter.Options{Table: d.Table, Headers: d.Headers}))

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	assert.Len(t, lines, len(rows)+1, out.String())
	assert.NotContains(t, out.String(), "<no value>")
	assert.NotContains(t, out.String(), "<nil>")

	for _, row := range rows {
		key, err := d.KeyOf(row)
		require.NoError(t, err)
		assert.Len(t, key, len(d.Key))
	}
}

func TestLookup(t *testing.T) {
	t.Parallel()

	t.Run("by its singular", func(t *testing.T) {
		t.Parallel()

		d, found := Lookup("license-template")

		require.True(t, found)
		assert.Equal(t, "license-templates", d.Plural)
	})

	t.Run("by its plural", func(t *testing.T) {
		t.Parallel()

		d, found := Lookup("contact-us")

		require.True(t, found)
		assert.Equal(t, "contact", d.Name)
	})

	t.Run("nothing of another name", func(t *testing.T) {
		t.Parallel()

		_, found := Lookup("nothing")

		assert.False(t, found)
	})
}
