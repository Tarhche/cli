package auth

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClaimsOf(t *testing.T) {
	t.Parallel()

	t.Run("a token says whose session it is, which, and when it runs out", func(t *testing.T) {
		t.Parallel()

		got := claimsOf(token(map[string]any{"sub": "user-1", "sid": "session-1", "exp": 1700000000, "roles": []string{"admin"}}))

		assert.Equal(t, claims{Subject: "user-1", Session: "session-1", ExpiresAt: 1700000000}, got)
		assert.True(t, time.Unix(1700000000, 0).Equal(got.expiry()))
	})

	t.Run("a token says what its roles allow", func(t *testing.T) {
		t.Parallel()

		allowed := token(map[string]any{"sub": "user-1", "permissions": []string{"articles.index", "self.workload.vms.index"}})

		assert.Equal(t, []string{"articles.index", "self.workload.vms.index"}, claimsOf(allowed).Permissions)
		assert.Equal(t, []string{"articles.index", "self.workload.vms.index"}, PermissionsOf(allowed))
		assert.Empty(t, PermissionsOf("not a token"))
	})

	t.Run("a payload with its padding is read too", func(t *testing.T) {
		t.Parallel()

		payload := base64.URLEncoding.EncodeToString([]byte(`{"sub":"u"}`))

		assert.Equal(t, "u", claimsOf("header."+payload+".signature").Subject)
	})

	t.Run("a moment with a fraction of a second is kept to it", func(t *testing.T) {
		t.Parallel()

		got := claimsOf(token(map[string]any{"exp": 1700000000.5}))

		assert.True(t, time.Unix(1700000000, 500_000_000).Equal(got.expiry()))
	})

	for name, value := range map[string]string{
		"nothing":            "",
		"an opaque token":    "opaque",
		"too many parts":     "a.b.c.d",
		"a payload of noise": "header.%%%.signature",
		"a payload not json": "header." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".signature",
	} {
		t.Run(name+" says nothing", func(t *testing.T) {
			t.Parallel()

			got := claimsOf(value)

			assert.Equal(t, claims{}, got)
			assert.True(t, got.expiry().IsZero())
		})
	}
}

func TestExpiresWithin(t *testing.T) {
	t.Parallel()

	assert.True(t, expiresWithin(expiring("s", 10*time.Second), 30*time.Second))
	assert.True(t, expiresWithin(expiring("s", -time.Minute), 30*time.Second))
	assert.False(t, expiresWithin(expiring("s", time.Minute), 30*time.Second))
	assert.False(t, expiresWithin("opaque", 30*time.Second), "a token that does not say is taken to last")
}
