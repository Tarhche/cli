package content

import (
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/resource"
)

func TestFilterFlags(t *testing.T) {
	t.Parallel()

	both := []resource.Audience{resource.AudienceMine, resource.AudienceEveryone}

	t.Run("a flag for each filter of the listings it may list", func(t *testing.T) {
		t.Parallel()

		flags := console.NewFlagSet("tarhche audit-log ls", nil)
		newFilterFlags(lookup("audit-log"), both).define(flags)

		var names []string
		for _, flag := range flags.Flags() {
			names = append(names, flag.Long())
		}

		assert.Equal(t, []string{"kind", "ip", "from", "to", "user-uuid"}, names)
	})

	t.Run("the filters of one audience alone", func(t *testing.T) {
		t.Parallel()

		filters := newFilterFlags(lookup("audit-log"), []resource.Audience{resource.AudienceMine}).offered()

		assert.Len(t, filters, 4)
	})

	t.Run("what each filter takes", func(t *testing.T) {
		t.Parallel()

		filters := newFilterFlags(lookup("audit-log"), both)

		assert.Equal(t, "only the actions of a kind: read, create, update, delete.", filters.explain(lookup("audit-log").Filters[0]))
		assert.Equal(t, "only what was done at this moment or later, a moment in RFC 3339 or a duration before now (1h).", filters.explain(lookup("audit-log").Filters[2]))
		assert.Equal(t, "only what somebody did, among everybody's (--all-users).", filters.explain(lookup("audit-log").Filters[4]))
	})

	t.Run("no word of everybody's where everybody's is all it lists", func(t *testing.T) {
		t.Parallel()

		filters := newFilterFlags(lookup("license"), []resource.Audience{resource.AudienceEveryone})

		assert.Equal(t, "only the licenses somebody holds.", filters.explain(lookup("license").Filters[0]))
	})

	t.Run("the query of the filters given", func(t *testing.T) {
		t.Parallel()

		flags := console.NewFlagSet("tarhche question ls", nil)
		filters := newFilterFlags(lookup("question"), both)
		filters.define(flags)
		require.NoError(t, flags.Parse([]string{"--kind", "text"}))

		query, err := filters.query(flags, resource.AudienceMine)

		require.NoError(t, err)
		assert.Equal(t, "kind=text", query.Encode())
	})
}
