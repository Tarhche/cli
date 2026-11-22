package upgrade

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitOf(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		status  int
		header  map[string]string
		limited bool
		resets  time.Time
	}{
		{name: "a 403 with no request left resets when GitHub says", status: http.StatusForbidden, header: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1795089600"}, limited: true, resets: time.Unix(1795089600, 0)},
		{name: "a 429 resets after the while GitHub says", status: http.StatusTooManyRequests, header: map[string]string{"Retry-After": "60"}, limited: true, resets: now.Add(time.Minute)},
		{name: "a 403 that says to retry after a while is one", status: http.StatusForbidden, header: map[string]string{"Retry-After": "30"}, limited: true, resets: now.Add(30 * time.Second)},
		{name: "a 429 that says nothing resets at no known time", status: http.StatusTooManyRequests, limited: true},
		{name: "a 403 with requests left is no rate limit", status: http.StatusForbidden, header: map[string]string{"X-RateLimit-Remaining": "12"}},
		{name: "a 500 is no rate limit", status: http.StatusInternalServerError, header: map[string]string{"X-RateLimit-Remaining": "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := &http.Response{StatusCode: test.status, Header: http.Header{}}
			for name, value := range test.header {
				response.Header.Set(name, value)
			}

			limit, limited := rateLimitOf(response, now)

			assert.Equal(t, test.limited, limited)
			assert.True(t, test.resets.Equal(limit.resets), "resets at %s, not %s", limit.resets, test.resets)
		})
	}

	t.Run("says when it resets", func(t *testing.T) {
		t.Parallel()

		resets := time.Date(2026, 11, 22, 12, 34, 0, 0, time.UTC)

		assert.EqualError(t, rateLimit{resets: resets}, "GitHub's rate limit was reached: try again after "+resets.Local().Format("15:04"))
		assert.EqualError(t, rateLimit{}, "GitHub's rate limit was reached: try again later")
	})
}

func TestTagOf(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"0.2.0":        "v0.2.0",
		"v0.2.0":       "v0.2.0",
		"0.2":          "v0.2.0",
		"1":            "v1.0.0",
		"0.3.0-rc.1":   "v0.3.0-rc.1",
		" 0.2.0\n":     "v0.2.0",
		"0.2.0+build1": "v0.2.0",
	}

	for asked, tag := range tests {
		t.Run(asked+" is released as "+tag, func(t *testing.T) {
			t.Parallel()

			got, err := tagOf(asked)

			assert.NoError(t, err)
			assert.Equal(t, tag, got)
		})
	}

	for _, asked := range []string{"", "latest", "v", "0.2.0/../../x", "vv0.2.0"} {
		t.Run(asked+" is no version", func(t *testing.T) {
			t.Parallel()

			_, err := tagOf(asked)

			assert.ErrorContains(t, err, "is no version of tarhche")
		})
	}
}

func TestNewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version, current string
		newer            bool
	}{
		{version: "0.3.0", current: "0.2.0", newer: true},
		{version: "v0.3.0", current: "0.2.9", newer: true},
		{version: "0.10.0", current: "0.9.0", newer: true},
		{version: "0.2.0", current: "0.2.0"},
		{version: "0.2.0", current: "0.3.0"},
		{version: "0.3.0", current: "0.3.0-rc.1", newer: true},
		{version: "0.3.0", current: "0.4.0-SNAPSHOT-27101f1"},
		{version: "0.3.0", current: "dev", newer: true},
		{version: "nightly", current: "dev"},
		{version: "nightly", current: "0.2.0"},
	}

	for _, test := range tests {
		t.Run(test.version+" against "+test.current, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.newer, newer(test.version, test.current))
		})
	}
}
