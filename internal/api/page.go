package api

import (
	"context"
	"net/url"
	"strconv"
)

// maxPages bounds how many pages All reads, so that an API that never says
// it is done cannot keep it reading.
const maxPages = 10_000

// Page is one page of a listing, as the API answers every listing.
type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Pagination says which page a page is, of how many.
type Pagination struct {
	TotalPages  uint `json:"total_pages"`
	CurrentPage uint `json:"current_page"`
}

// List reads one page of the listing at path; the first is page 1.
func List[T any](ctx context.Context, c *Client, path string, query url.Values, page uint) (Page[T], error) {
	asked := url.Values{}
	for key, values := range query {
		asked[key] = values
	}

	if page > 0 {
		asked.Set("page", strconv.FormatUint(uint64(page), 10))
	}

	var p Page[T]
	if err := c.Get(ctx, path, asked, &p); err != nil {
		return Page[T]{}, err
	}

	return p, nil
}

// All reads every page of the listing at path.
func All[T any](ctx context.Context, c *Client, path string, query url.Values) ([]T, error) {
	var items []T

	for page := uint(1); page <= maxPages; page++ {
		p, err := List[T](ctx, c, path, query, page)
		if err != nil {
			return nil, err
		}

		items = append(items, p.Items...)

		if len(p.Items) == 0 || page >= p.Pagination.TotalPages {
			break
		}
	}

	return items, nil
}
