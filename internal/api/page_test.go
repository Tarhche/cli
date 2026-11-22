package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type item struct {
	ID int `json:"id"`
}

func pages(t *testing.T, total int, perPage int) *Client {
	t.Helper()

	server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "x", r.URL.Query().Get("filter"))

		page := 1
		if asked := r.URL.Query().Get("page"); len(asked) > 0 {
			_, _ = fmt.Sscan(asked, &page)
		}

		var items []string
		for i := 0; i < perPage && (page-1)*perPage+i < total*perPage; i++ {
			items = append(items, fmt.Sprintf(`{"id":%d}`, (page-1)*perPage+i))
		}

		_, _ = fmt.Fprintf(rw, `{"items":[%s],"pagination":{"total_pages":%d,"current_page":%d}}`, join(items), total, page)
	})

	c, err := New(server.URL)
	require.NoError(t, err)

	return c
}

func join(items []string) string {
	joined := ""
	for i, item := range items {
		if i > 0 {
			joined += ","
		}

		joined += item
	}

	return joined
}

func TestList(t *testing.T) {
	t.Parallel()

	t.Run("a page is read with its pagination", func(t *testing.T) {
		t.Parallel()

		c := pages(t, 3, 2)

		page, err := List[item](context.Background(), c, "/things", map[string][]string{"filter": {"x"}}, 2)

		require.NoError(t, err)
		assert.Equal(t, []item{{ID: 2}, {ID: 3}}, page.Items)
		assert.Equal(t, Pagination{TotalPages: 3, CurrentPage: 2}, page.Pagination)
	})

	t.Run("page zero is the api's first", func(t *testing.T) {
		t.Parallel()

		c := pages(t, 3, 2)

		page, err := List[item](context.Background(), c, "/things", map[string][]string{"filter": {"x"}}, 0)

		require.NoError(t, err)
		assert.Equal(t, uint(1), page.Pagination.CurrentPage)
	})

	t.Run("an error is that error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusForbidden)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		_, err = List[item](context.Background(), c, "/things", nil, 1)

		assert.True(t, IsForbidden(err))
	})
}

func TestAll(t *testing.T) {
	t.Parallel()

	t.Run("every page is read", func(t *testing.T) {
		t.Parallel()

		c := pages(t, 3, 2)

		items, err := All[item](context.Background(), c, "/things", map[string][]string{"filter": {"x"}})

		require.NoError(t, err)
		assert.Equal(t, []item{{0}, {1}, {2}, {3}, {4}, {5}}, items)
	})

	t.Run("an empty listing is no items", func(t *testing.T) {
		t.Parallel()

		c := pages(t, 0, 2)

		items, err := All[item](context.Background(), c, "/things", map[string][]string{"filter": {"x"}})

		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("an error stops the reading", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		_, err = All[item](context.Background(), c, "/things", nil)

		assert.Error(t, err)
	})
}
