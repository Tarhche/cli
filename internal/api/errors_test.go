package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("connection reset")
}

func TestError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  Error
		said string
	}{
		{name: "a message", err: Error{Code: "not_found", Message: "no such vm"}, said: "no such vm"},
		{name: "a code without a message", err: Error{Code: "too_small"}, said: "too small"},
		{name: "validation errors in order", err: Error{Fields: map[string]string{"title": "required", "body": "too long"}}, said: "body: too long; title: required"},
		{name: "a message and fields", err: Error{Message: "refused", Fields: map[string]string{"vms": "exceeds the quota"}}, said: "refused; vms: exceeds the quota"},
		{name: "a body of its own", err: Error{StatusCode: http.StatusBadGateway, Body: "upstream down"}, said: "upstream down"},
		{name: "nothing but a status", err: Error{StatusCode: http.StatusForbidden}, said: "forbidden"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.said, c.err.Error())
		})
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	assert.True(t, IsNotFound(fmt.Errorf("wrapped: %w", &Error{StatusCode: http.StatusNotFound})))
	assert.False(t, IsNotFound(&Error{StatusCode: http.StatusBadRequest}))
	assert.False(t, IsNotFound(errors.New("other")))
}

func TestIsUnauthorized(t *testing.T) {
	t.Parallel()

	assert.True(t, IsUnauthorized(&Error{StatusCode: http.StatusUnauthorized}))
	assert.False(t, IsUnauthorized(&Error{StatusCode: http.StatusForbidden}))
}

func TestIsForbidden(t *testing.T) {
	t.Parallel()

	assert.True(t, IsForbidden(&Error{StatusCode: http.StatusForbidden}))
	assert.False(t, IsForbidden(&Error{StatusCode: http.StatusUnauthorized}))
}

func TestIsValidation(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidation(&Error{StatusCode: http.StatusBadRequest, Fields: map[string]string{"a": "b"}}))
	assert.False(t, IsValidation(&Error{StatusCode: http.StatusBadRequest}))
	assert.False(t, IsValidation(errors.New("other")))
}

func TestReadError(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://backend.example/things", nil)
	require.NoError(t, err)

	t.Run("validation errors are read by field", func(t *testing.T) {
		t.Parallel()

		response := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"errors":{"title":"required"}}`))}

		var apiErr *Error
		require.ErrorAs(t, readError(request, response), &apiErr)
		assert.Equal(t, map[string]string{"title": "required"}, apiErr.Fields)
		assert.Empty(t, apiErr.Body)
	})

	t.Run("json that says none of it is kept as the body", func(t *testing.T) {
		t.Parallel()

		response := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{"other":1}`))}

		var apiErr *Error
		require.ErrorAs(t, readError(request, response), &apiErr)
		assert.Equal(t, `{"other":1}`, apiErr.Body)
	})

	t.Run("a body that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		response := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(failingReader{})}

		err := readError(request, response)

		assert.ErrorContains(t, err, "connection reset")
		assert.ErrorContains(t, err, "GET /things")
	})
}
