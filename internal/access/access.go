// Package access says what a session may do, as its estate decides it: the
// permission each route of the API is asked under, and each stream of a VM,
// the permissions a session's token says its roles allow, and, when the API
// or the ingress refuses a request, why: a permission the roles do not
// give, one a role disallows on that one resource, or an account that is
// blocked.
//
// What a token says is a hint, as it is to the web dashboard: the API
// decides every request by the roles as they are when it is made, though
// the node holding a VM decides its streams by what the token says alone.
// The desktop app offers only what the hint allows; a command is never
// hidden, and says why it was refused when it is.
package access

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
)

// ProbePath is a route every signed-in session may read, whatever its roles:
// a session refused it is blocked.
const ProbePath = "/api/dashboard/profile"

// ErrBlocked is an account its estate lets do nothing at all.
var ErrBlocked = errors.New("your account is blocked: the estate refuses every request of yours")

// Requirement is what a route is asked under.
type Requirement struct {
	// Known says the route is one a session is needed for.
	Known bool

	// Permission is the permission it is asked under; empty for a route
	// whoever is signed in may ask.
	Permission string
}

// route is a pattern of the API's, read for matching.
type route struct {
	segments   []string
	literals   int
	permission string
}

// patterns are the routes by method.
var patterns = indexed(routes)

func indexed(table map[string]string) map[string][]route {
	byMethod := make(map[string][]route)

	for key, permission := range table {
		method, path, _ := strings.Cut(key, " ")
		segments := strings.Split(strings.Trim(path, "/"), "/")

		literals := 0
		for _, segment := range segments {
			if !strings.HasPrefix(segment, "{") {
				literals++
			}
		}

		byMethod[method] = append(byMethod[method], route{segments: segments, literals: literals, permission: permission})
	}

	return byMethod
}

// Required is what a request for path is asked under: the route the API
// serves it at, the most specific of those it matches, as the API picks.
func Required(method, path string) Requirement {
	segments := strings.Split(strings.Trim(path, "/"), "/")

	var (
		best  route
		found bool
	)

	for _, candidate := range patterns[method] {
		if matches(candidate.segments, segments) && (!found || candidate.literals > best.literals) {
			best, found = candidate, true
		}
	}

	if !found {
		return Requirement{}
	}

	return Requirement{Known: true, Permission: best.permission}
}

func matches(pattern, segments []string) bool {
	if len(pattern) != len(segments) {
		return false
	}

	for i, segment := range pattern {
		if strings.HasPrefix(segment, "{") {
			if len(segments[i]) == 0 {
				return false
			}

			continue
		}

		if segment != segments[i] {
			return false
		}
	}

	return true
}

// Granted are the permissions a session's access token says its roles allow
// somewhere.
type Granted []string

// GrantedBy are the permissions token says its roles allow.
func GrantedBy(token string) Granted {
	return Granted(auth.PermissionsOf(token))
}

// Has reports whether the roles allow permission somewhere.
func (g Granted) Has(permission string) bool {
	return slices.Contains(g, permission)
}

// Allows reports whether the roles allow what a request for path is asked
// under: a route nobody needs a session for, or one whoever is signed in may
// ask, always.
func (g Granted) Allows(method, path string) bool {
	requirement := Required(method, path)

	return len(requirement.Permission) == 0 || g.Has(requirement.Permission)
}

// DeniedError is a request the estate refused for the roles of whoever asked
// it.
type DeniedError struct {
	// Permission is what the request was asked under; empty when its route
	// is not known.
	Permission string

	// Here says the roles allow it elsewhere, and one of them disallows it
	// on what it was asked of, or it was taken away since the session's
	// token was handed over.
	Here bool

	// Refused is the refusal: the API's answer (an *api.Error), or the
	// ingress's to a stream (a *stream.Error).
	Refused error
}

func (e *DeniedError) Error() string {
	switch {
	case len(e.Permission) == 0:
		return "permission denied: your roles do not allow this"
	case e.Here:
		return fmt.Sprintf("permission denied: your roles do not allow %q on this", e.Permission)
	default:
		return fmt.Sprintf("permission denied: this needs the %q permission, which none of your roles gives you", e.Permission)
	}
}

// Unwrap is the refusal, so that a denial is still one.
func (e *DeniedError) Unwrap() error {
	return e.Refused
}

// BlockedError is a request refused because the account that asked it is
// blocked.
type BlockedError struct {
	Refused *api.Error
}

func (e *BlockedError) Error() string {
	return ErrBlocked.Error()
}

// Unwrap is ErrBlocked and the API's answer.
func (e *BlockedError) Unwrap() []error {
	return []error{ErrBlocked, e.Refused}
}

// Explainer says why the API refused a request: that the account is blocked,
// when even a route every session may read is refused; else the permission
// it needs that the session's token does not give; else that a role
// disallows it on what it was asked of.
type Explainer struct {
	tokens api.TokenSource
	probe  func(ctx context.Context) error
}

var _ api.Explainer = (*Explainer)(nil)

// NewExplainer explains refusals with the tokens of the session, asking probe
// whether it may read ProbePath at all.
func NewExplainer(tokens api.TokenSource, probe func(ctx context.Context) error) *Explainer {
	return &Explainer{tokens: tokens, probe: probe}
}

// Explain is why the API refused a request.
func (e *Explainer) Explain(ctx context.Context, refused *api.Error) error {
	if err := e.probe(api.Unexplained(ctx)); api.IsForbidden(err) {
		return &BlockedError{Refused: refused}
	}

	requirement := Required(refused.Method, refused.Path)

	if len(requirement.Permission) > 0 && e.tokens != nil {
		if token, err := e.tokens.Token(ctx); err == nil && !GrantedBy(token).Has(requirement.Permission) {
			return &DeniedError{Permission: requirement.Permission, Refused: refused}
		}
	}

	return &DeniedError{Permission: requirement.Permission, Here: true, Refused: refused}
}
