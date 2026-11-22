package workload

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrAmbiguous is a reference that names more than one thing.
	ErrAmbiguous = errors.New("ambiguous reference")

	// ErrNotFound is a reference that names nothing.
	ErrNotFound = errors.New("no such workload")
)

// NotFoundError is a reference that names nothing of a kind.
type NotFoundError struct {
	// Kind is what was looked for: a vm, a container, an image.
	Kind      string
	Reference string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no such %s: %s", e.Kind, e.Reference)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// AmbiguousError is a reference that names more than one thing of a kind.
type AmbiguousError struct {
	Kind      string
	Reference string

	// Matches are what it names, each as it is told apart from the others.
	Matches []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q names more than one %s: %s", e.Reference, e.Kind, strings.Join(e.Matches, ", "))
}

func (e *AmbiguousError) Unwrap() error {
	return ErrAmbiguous
}
