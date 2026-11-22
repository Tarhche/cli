package desktop_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/desktop"
)

func TestInvalidError(t *testing.T) {
	t.Parallel()

	t.Run("it says what is wrong with every field, by name", func(t *testing.T) {
		t.Parallel()

		err := desktop.InvalidError{Fields: map[string]string{"name": "is taken", "api": "is no url"}}

		assert.Equal(t, "api: is no url; name: is taken", err.Error())
	})
}

func TestFailureOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want desktop.Failure
	}{
		{
			name: "an error is its message",
			err:  errors.New("it broke"),
			want: desktop.Failure{Message: "it broke"},
		},
		{
			name: "a refusal is its status, its code and its fields",
			err:  fmt.Errorf("creating: %w", &api.Error{StatusCode: http.StatusBadRequest, Code: "too_small", Fields: map[string]string{"name": "required"}}),
			want: desktop.Failure{Message: "creating: too small; name: required", Status: http.StatusBadRequest, Code: "too_small", Fields: map[string]string{"name": "required"}},
		},
		{
			name: "an unauthorized answer is a session gone",
			err:  &api.Error{StatusCode: http.StatusUnauthorized},
			want: desktop.Failure{Message: "unauthorized", Status: http.StatusUnauthorized, LoggedOut: true},
		},
		{
			name: "no session is a session gone",
			err:  fmt.Errorf("listing: %w", auth.ErrNotLoggedIn),
			want: desktop.Failure{Message: "listing: not logged in", LoggedOut: true},
		},
		{
			name: "what the desktop cannot do yet says so",
			err:  fmt.Errorf("terminals are %w", desktop.ErrUnavailable),
			want: desktop.Failure{Message: "terminals are not available yet", Unavailable: true},
		},
		{
			name: "what the desktop refuses is its fields",
			err:  desktop.InvalidError{Fields: map[string]string{"json": "it is not valid JSON"}},
			want: desktop.Failure{Message: "json: it is not valid JSON", Fields: map[string]string{"json": "it is not valid JSON"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, desktop.FailureOf(c.err))
		})
	}
}

func TestMarshalError(t *testing.T) {
	t.Parallel()

	t.Run("an error is written as its failure", func(t *testing.T) {
		t.Parallel()

		var got desktop.Failure
		require.NoError(t, json.Unmarshal(desktop.MarshalError(&api.Error{StatusCode: http.StatusNotFound, Code: "not_found"}), &got))

		assert.Equal(t, desktop.Failure{Message: "not found", Status: http.StatusNotFound, Code: "not_found"}, got)
	})
}
