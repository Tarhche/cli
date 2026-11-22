package workload

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/api"
)

// Audience is whose workloads a client reaches.
type Audience int

const (
	// Mine are one's own, under /api/dashboard/my/workload.
	Mine Audience = iota

	// Everyone is anybody's, which an administrator manages, under
	// /api/dashboard/workload.
	Everyone
)

const (
	minePath     = "/api/dashboard/my/workload"
	everyonePath = "/api/dashboard/workload"

	// PatientTimeout bounds what the API answers only once it is done: a
	// container made, whose image may be pulled and whose VM may be booted
	// first, or an image pulled, which the API waits up to 14 minutes for.
	PatientTimeout = 15 * time.Minute
)

// Client asks the API for the workload: VMs and what lives in them,
// containers, images, networks, volumes and stacks, and the snapshots of
// their disks. Every one of them is answered in its kind's manifest shape.
type Client struct {
	api      *api.Client
	audience Audience
}

// NewClient is a client of the workloads of audience, asking through
// client. What is created is always one's own, whatever the audience.
func NewClient(client *api.Client, audience Audience) *Client {
	return &Client{api: client, audience: audience}
}

// Audience is whose workloads the client reaches.
func (c *Client) Audience() Audience {
	return c.audience
}

// path is the route of the client's audience made of segments.
func (c *Client) path(segments ...string) string {
	if c.audience == Everyone {
		return joined(everyonePath, segments)
	}

	return joined(minePath, segments)
}

// creation is a route things are created on: always anybody's, where what
// is created is one's own.
func creation(segments ...string) string {
	return joined(everyonePath, segments)
}

func joined(base string, segments []string) string {
	return base + "/" + strings.Join(segments, "/")
}

// get reads path into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.api.Get(ctx, path, query, out)
}

// listing is a listing that is not paged: every item at once.
type listing[T any] struct {
	Items []T `json:"items"`
}

// items reads the listing at path, which is not paged.
func items[T any](ctx context.Context, c *Client, path string, query url.Values) ([]T, error) {
	var answer listing[T]
	if err := c.get(ctx, path, query, &answer); err != nil {
		return nil, err
	}

	return answer.Items, nil
}

// act asks path for an action that answers with nothing worth reading.
func (c *Client) act(ctx context.Context, method, path string, query url.Values, body any) error {
	return c.api.Do(ctx, method, path, query, body, nil)
}

// post sends body to path and reads what it answers into out.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.api.Do(ctx, http.MethodPost, path, nil, body, out)
}

// flags is a query of the flags that are set, each as true.
func flags(set map[string]bool) url.Values {
	query := url.Values{}

	for name, on := range set {
		if on {
			query.Set(name, strconv.FormatBool(true))
		}
	}

	return query
}

// narrowed is a query narrowing a listing to one VM's, or nothing when
// vmUUID is empty.
func narrowed(vmUUID string) url.Values {
	if len(vmUUID) == 0 {
		return nil
	}

	return url.Values{"vm": {vmUUID}}
}
