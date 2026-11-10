// Package randtoken generates unguessable, URL-safe opaque tokens for
// public share links (e.g. a project's public feedback link) — distinct
// from a resource's own database ID so a leaked link can be revoked
// (regenerated) without touching the resource's real identifier.
package randtoken

import (
	"crypto/rand"
	"encoding/base64"
)

// Generate returns a URL-safe random token with n bytes of entropy.
func Generate(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
