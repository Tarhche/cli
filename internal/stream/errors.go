package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// maxRefusal bounds how much of a refusal is read; the websocket dialer
// keeps no more than this of it anyway.
const maxRefusal = 1 << 10

// Error is a stream the ingress refused to open, answering plain HTTP
// before upgrading to a websocket.
type Error struct {
	// Stream is what was asked for, as "vms/<uuid>/exec".
	Stream string

	StatusCode int

	// Fields are the reasons a payload was refused, by field.
	Fields map[string]string

	// Message is what the ingress said otherwise.
	Message string
}

// Error says what the refusal means and what the ingress said of it.
func (e *Error) Error() string {
	reason := e.reason()

	if said := e.said(); len(said) > 0 {
		reason += " (" + said + ")"
	}

	return "cannot open " + e.Stream + ": " + reason
}

func (e *Error) reason() string {
	switch e.StatusCode {
	case http.StatusBadRequest:
		return "the ingress refused what it was asked"
	case http.StatusUnauthorized:
		return "the session's token was refused"
	case http.StatusNotFound:
		return "it is not there, or not yours"
	case http.StatusServiceUnavailable:
		return "it cannot be reached now; is it running?"
	}

	if text := http.StatusText(e.StatusCode); len(text) > 0 {
		return fmt.Sprintf("the ingress answered %d %s", e.StatusCode, text)
	}

	return fmt.Sprintf("the ingress answered %d", e.StatusCode)
}

func (e *Error) said() string {
	if len(e.Fields) == 0 {
		return e.Message
	}

	said := make([]string, 0, len(e.Fields))
	for _, field := range slices.Sorted(maps.Keys(e.Fields)) {
		said = append(said, field+": "+e.Fields[field])
	}

	return strings.Join(said, "; ")
}

// IsNotFound reports whether err is a stream refused for a workload that
// is not there, or not the asker's.
func IsNotFound(err error) bool {
	return hasStatus(err, http.StatusNotFound)
}

// IsUnavailable reports whether err is a stream refused for a workload that
// is there but cannot be reached now: one that is not running.
func IsUnavailable(err error) bool {
	return hasStatus(err, http.StatusServiceUnavailable)
}

func hasStatus(err error, status int) bool {
	var e *Error

	return errors.As(err, &e) && e.StatusCode == status
}

// refusal is the error a stream that was not opened is: what the ingress
// answered instead of upgrading, or why it could not be asked.
func refusal(name string, response *http.Response, err error) error {
	if response == nil {
		return fmt.Errorf("cannot open %s: %w", name, err)
	}

	e := &Error{Stream: name, StatusCode: response.StatusCode}

	if response.Body == nil {
		return e
	}

	content, readErr := io.ReadAll(io.LimitReader(response.Body, maxRefusal))
	if readErr != nil {
		return e
	}

	var refused struct {
		Errors map[string]string `json:"errors"`
	}

	if json.Unmarshal(content, &refused) == nil && len(refused.Errors) > 0 {
		e.Fields = refused.Errors
	} else {
		e.Message = strings.TrimSpace(string(content))
	}

	return e
}

// CloseError is a stream the other side closed for a reason other than its
// end: a port that could not be dialled, say.
type CloseError struct {
	Code   int
	Reason string
}

// Error says why the stream was closed.
func (e *CloseError) Error() string {
	if len(e.Reason) == 0 {
		return fmt.Sprintf("the stream was closed (%d)", e.Code)
	}

	return fmt.Sprintf("the stream was closed: %s (%d)", e.Reason, e.Code)
}

// ExitError is a command run to its end that exited with a status other
// than 0.
type ExitError struct {
	Code int

	// Stderr is what it said on its standard error, the first of it.
	Stderr string
}

// Error is what the command said on its standard error, or its status when
// it said nothing.
func (e *ExitError) Error() string {
	if said := strings.TrimSpace(e.Stderr); len(said) > 0 {
		return said
	}

	return fmt.Sprintf("exit status %d", e.Code)
}

// ExitCode is the status the command exited with.
func (e *ExitError) ExitCode() int {
	return e.Code
}
