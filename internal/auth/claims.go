package auth

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"time"
)

// claims are what a token says of itself, read without verifying it: the
// estate verifies every token it is handed, and the CLI only needs to know
// whose a session is, which it is, and when its token runs out.
type claims struct {
	Subject   string  `json:"sub"`
	Session   string  `json:"sid"`
	ExpiresAt float64 `json:"exp"`
}

// claimsOf reads the claims of a JWT, and is none for what is not one.
func claimsOf(token string) claims {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims{}
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}
	}

	return c
}

// expiry is when the token runs out, and the zero time when it does not
// say.
func (c claims) expiry() time.Time {
	if c.ExpiresAt <= 0 {
		return time.Time{}
	}

	seconds, fraction := math.Modf(c.ExpiresAt)

	return time.Unix(int64(seconds), int64(fraction*float64(time.Second)))
}

// expiresWithin reports whether token runs out within d. A token that does
// not say when it runs out is taken to last: the API says when it does not.
func expiresWithin(token string, d time.Duration) bool {
	expiry := claimsOf(token).expiry()

	return !expiry.IsZero() && time.Until(expiry) < d
}
