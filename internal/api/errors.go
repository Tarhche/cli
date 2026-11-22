package api

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

// maxErrorBody bounds how much of an answer that is not a success is read.
const maxErrorBody = 64 << 10

// Error is an answer that is not a success, and what the API said of it: a
// failure with a code and a message, or validation errors by field.
type Error struct {
	StatusCode int    `json:"-"`
	Method     string `json:"-"`
	Path       string `json:"-"`

	Code    string            `json:"code,omitempty"`
	Message string            `json:"message,omitempty"`
	Fields  map[string]string `json:"errors,omitempty"`

	// Body is what was answered when it says none of the above.
	Body string `json:"-"`
}

// Error says what the API said, or what the status means when it said
// nothing.
func (e *Error) Error() string {
	var said []string

	if len(e.Message) > 0 {
		said = append(said, e.Message)
	} else if len(e.Code) > 0 {
		said = append(said, strings.ReplaceAll(e.Code, "_", " "))
	}

	for _, field := range slices.Sorted(maps.Keys(e.Fields)) {
		said = append(said, field+": "+e.Fields[field])
	}

	if len(said) == 0 && len(e.Body) > 0 {
		said = append(said, e.Body)
	}

	if len(said) == 0 {
		said = append(said, strings.ToLower(http.StatusText(e.StatusCode)))
	}

	return strings.Join(said, "; ")
}

// IsNotFound reports whether err is an answer saying there is no such thing.
func IsNotFound(err error) bool {
	return hasStatus(err, http.StatusNotFound)
}

// IsUnauthorized reports whether err is an answer refusing who asked.
func IsUnauthorized(err error) bool {
	return hasStatus(err, http.StatusUnauthorized)
}

// IsForbidden reports whether err is an answer saying who asked may not.
func IsForbidden(err error) bool {
	return hasStatus(err, http.StatusForbidden)
}

// IsValidation reports whether err is an answer refusing what was asked,
// field by field.
func IsValidation(err error) bool {
	var e *Error

	return errors.As(err, &e) && len(e.Fields) > 0
}

func hasStatus(err error, status int) bool {
	var e *Error

	return errors.As(err, &e) && e.StatusCode == status
}

func readError(request *http.Request, response *http.Response) error {
	e := &Error{StatusCode: response.StatusCode, Method: request.Method, Path: request.URL.Path}

	content, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if err != nil {
		return fmt.Errorf("%s %s: reading the answer: %w", request.Method, request.URL.Path, err)
	}

	if json.Unmarshal(content, e) != nil || (len(e.Code) == 0 && len(e.Message) == 0 && len(e.Fields) == 0) {
		e.Body = strings.TrimSpace(string(content))
	}

	return e
}
