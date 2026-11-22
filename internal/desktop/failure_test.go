package desktop_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/stream"
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
			name: "a session its estate would not renew is a session gone, which ended",
			err:  fmt.Errorf("listing: %w", auth.ErrSessionEnded),
			want: desktop.Failure{Message: "listing: the session has expired or ended: not logged in", LoggedOut: true, Ended: true},
		},
		{
			name: "an account refused every request is a session gone, blocked",
			err:  &access.BlockedError{Refused: &api.Error{StatusCode: http.StatusForbidden}},
			want: desktop.Failure{Message: "your account is blocked: the estate refuses every request of yours", Status: http.StatusForbidden, LoggedOut: true, Blocked: true},
		},
		{
			name: "an account refused a session is a session gone, blocked for the reason the estate gave",
			err:  fmt.Errorf("renewing: %w", &auth.AccountRefusedError{Reason: "the user is banned"}),
			want: desktop.Failure{Message: "renewing: your account may not sign in: the user is banned", LoggedOut: true, Blocked: true, Reason: "the user is banned"},
		},
		{
			name: "a request the roles do not allow is denied, naming the permission it needs",
			err:  &access.DeniedError{Permission: "workload.vms.create", Refused: &api.Error{StatusCode: http.StatusForbidden}},
			want: desktop.Failure{Message: `permission denied: this needs the "workload.vms.create" permission, which none of your roles gives you`, Status: http.StatusForbidden, Denied: true, Permission: "workload.vms.create"},
		},
		{
			name: "a request a role disallows on what it was asked of is denied",
			err:  &access.DeniedError{Permission: "comments.delete", Here: true, Refused: &api.Error{StatusCode: http.StatusForbidden}},
			want: desktop.Failure{Message: `permission denied: your roles do not allow "comments.delete" on this`, Status: http.StatusForbidden, Denied: true, Permission: "comments.delete"},
		},
		{
			name: "a stream a node refuses for the session's permissions is denied, naming the permission it needs",
			err:  &access.DeniedError{Permission: "self.workload.vms.exec", Refused: &stream.Error{Stream: "vms/v-1/exec", StatusCode: http.StatusForbidden}},
			want: desktop.Failure{Message: `permission denied: this needs the "self.workload.vms.exec" permission, which none of your roles gives you`, Denied: true, Permission: "self.workload.vms.exec"},
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
