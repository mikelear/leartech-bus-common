//go:build unit

package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// testSigner bundles a private RSA key with the kid it advertises and
// helpers for minting tokens + serving a matching JWKS document.
//
// It is deliberately kept in the test package (not testutils) — the
// production package must never take a dependency on any code that
// makes it easier to disable auth or produce test tokens outside of
// unit tests. Keeping this file _test.go means it is invisible to
// consumers of pkg/auth.
type testSigner struct {
	kid    string
	priv   *rsa.PrivateKey
	issuer string
}

func newTestSigner(t *testing.T, kid, issuer string) *testSigner {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "generate RSA key for test signer")
	return &testSigner{kid: kid, priv: priv, issuer: issuer}
}

// jwks returns the JWKS document body this signer advertises.
func (s *testSigner) jwks() []byte {
	n := base64.RawURLEncoding.EncodeToString(s.priv.N.Bytes())
	// The exponent E is a small positive int; the JWKS "e" is a
	// base64url of the big-endian minimal-byte representation. For
	// the common exponent 65537 that's "AQAB".
	eBig := big.NewInt(int64(s.priv.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBig)
	doc := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": s.kid,
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

// serveJWKS starts an httptest.Server exposing this signer's JWKS.
func (s *testSigner) serveJWKS(t *testing.T) *httptest.Server {
	t.Helper()
	body := s.jwks()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sign mints an RS256 JWT with the given claim overrides on top of
// sensible defaults (iss/aud/exp).
func (s *testSigner) sign(t *testing.T, overrides jwt.MapClaims) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": s.issuer,
		"sub": "user-1",
		"iat": now.Unix(),
		"exp": now.Add(15 * time.Minute).Unix(),
	}
	for k, v := range overrides {
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = s.kid
	signed, err := tok.SignedString(s.priv)
	require.NoError(t, err, "sign test JWT")
	return signed
}
