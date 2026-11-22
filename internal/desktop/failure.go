package desktop

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
)

// Failure is an error as the frontend is told it: what went wrong, what the
// API answered and refused by field, whether the context has to be signed
// in to again and why, and the permission a refusal needed.
type Failure struct {
	Message string `json:"message"`

	// Status is the HTTP status the API answered with, and Code what it said
	// went wrong, when it was the API that refused.
	Status int    `json:"status,omitempty"`
	Code   string `json:"code,omitempty"`

	// Fields are what was refused, by field.
	Fields map[string]string `json:"fields,omitempty"`

	// LoggedOut says the context has no session, or one the API no longer
	// takes, its account's among them: the frontend answers it with its
	// login.
	LoggedOut bool `json:"logged_out,omitempty"`

	// Ended says the session was over: it expired, or was signed out of,
	// and its estate would not renew it.
	Ended bool `json:"ended,omitempty"`

	// Blocked says the estate refuses the account itself, blocked or gone,
	// and Reason is why, when it said why.
	Blocked bool   `json:"blocked,omitempty"`
	Reason  string `json:"reason,omitempty"`

	// Denied says the session's roles do not allow what was asked, and
	// Permission is what it needs, when that is known.
	Denied     bool   `json:"denied,omitempty"`
	Permission string `json:"permission,omitempty"`

	// Unavailable says it is something the desktop cannot do yet.
	Unavailable bool `json:"unavailable,omitempty"`
}

// InvalidError is something asked of the desktop itself that is not right,
// field by field, as the API refuses what it is asked.
type InvalidError struct {
	Fields map[string]string
}

// Error says what is wrong with every field, in the order of their names.
func (e InvalidError) Error() string {
	said := make([]string, 0, len(e.Fields))
	for _, field := range slices.Sorted(maps.Keys(e.Fields)) {
		said = append(said, field+": "+e.Fields[field])
	}

	return strings.Join(said, "; ")
}

// FailureOf is err as the frontend is told it.
func FailureOf(err error) Failure {
	blocked := isBlocked(err)

	failure := Failure{
		Message:     err.Error(),
		LoggedOut:   loggedOut(err) || blocked,
		Ended:       errors.Is(err, auth.ErrSessionEnded),
		Blocked:     blocked,
		Reason:      refusalReason(err),
		Unavailable: errors.Is(err, ErrUnavailable),
	}

	if refusal, isRefusal := errors.AsType[*api.Error](err); isRefusal {
		failure.Status = refusal.StatusCode
		failure.Code = refusal.Code
		failure.Fields = refusal.Fields
	}

	if invalid, isInvalid := errors.AsType[InvalidError](err); isInvalid {
		failure.Fields = invalid.Fields
	}

	if permission, denied := deniedPermission(err); denied {
		failure.Denied = true
		failure.Permission = permission
	}

	return failure
}

// MarshalError is how the app hands the frontend what a service failed
// with: a Failure, which is the cause of the error a call rejects with. A
// Failure holds nothing json cannot write, so it is always written.
func MarshalError(err error) []byte {
	content, _ := json.Marshal(FailureOf(err))

	return content
}

// isBlocked reports whether err says the estate refuses the account itself:
// every request of it, or its session's renewal.
func isBlocked(err error) bool {
	return errors.Is(err, access.ErrBlocked) || errors.Is(err, auth.ErrAccountRefused)
}

// deniedPermission is the permission a request or a stream was refused for,
// when err is such a refusal: the API's, or a node's.
func deniedPermission(err error) (string, bool) {
	if denied, isDenied := errors.AsType[*access.DeniedError](err); isDenied {
		return denied.Permission, true
	}

	return "", false
}
