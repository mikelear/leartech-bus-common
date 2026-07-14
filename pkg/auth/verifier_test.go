//go:build unit

package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newVerifier is a shared setup helper: brings up a signer + JWKS
// server, builds a fully-configured Verifier pointing at it, and
// returns everything for the test to use.
func newVerifier(t *testing.T) (auth.Verifier, *testSigner) {
	t.Helper()
	signer := newTestSigner(t, "test-kid-1", "https://iss.test/leartech")
	srv := signer.serveJWKS(t)
	v, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:   signer.issuer,
		JWKSURL:  srv.URL,
		Audience: "svc-a",
	})
	require.NoError(t, err)
	return v, signer
}

func TestVerify_ValidTokenPasses(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "sub": "user-42"})
	claims, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "user-42", claims.Subject)
	assert.Equal(t, s.issuer, claims.Issuer)
	assert.Contains(t, claims.Audience, "svc-a")
}

func TestVerify_MultiAudienceTokenPassesWhenListContainsAudience(t *testing.T) {
	// aud can be an array per RFC 7519. Presence of the configured
	// audience anywhere in the array is a match.
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": []string{"svc-b", "svc-a", "svc-c"}})
	_, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
}

func TestVerify_WrongAudienceRejected(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-b"})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrAudienceMismatch), "want ErrAudienceMismatch, got %v", err)
}

func TestVerify_MultiAudienceWithoutOursRejected(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": []string{"svc-b", "svc-c"}})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrAudienceMismatch))
}

func TestVerify_MissingAudienceRejected(t *testing.T) {
	// A token with NO aud claim MUST be rejected. This is the
	// central regression the initiative locks in — the sister
	// go-common package treated missing aud as "well, the config
	// didn't require one" and let the request through.
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrAudienceMismatch), "want ErrAudienceMismatch, got %v", err)
}

func TestVerify_EmptyStringAudienceRejected(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": ""})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrAudienceMismatch))
}

func TestVerify_WrongIssuerRejected(t *testing.T) {
	v, s := newVerifier(t)
	// sign a token where iss doesn't match the configured issuer
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "iss": "https://other.test/leartech"})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrIssuerMismatch), "want ErrIssuerMismatch, got %v", err)
}

func TestVerify_ExpiredTokenRejected(t *testing.T) {
	v, s := newVerifier(t)
	past := time.Now().Add(-1 * time.Hour).Unix()
	tok := s.sign(t, jwt.MapClaims{
		"aud": "svc-a",
		"exp": past,
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken), "want ErrInvalidToken, got %v", err)
}

func TestVerify_UnknownKIDRejected(t *testing.T) {
	v, s := newVerifier(t)
	// A signer with the SAME issuer but a DIFFERENT kid that isn't
	// in the JWKS. The verifier must refuse — this is the "key
	// rotation not synchronised" defence.
	other := newTestSigner(t, "attacker-kid", s.issuer)
	tok := other.sign(t, jwt.MapClaims{"aud": "svc-a"})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnknownKID), "want ErrUnknownKID, got %v", err)
}

func TestVerify_WrongSignatureRejected(t *testing.T) {
	// Same kid, different private key. This simulates an attacker
	// who copies the kid but signs with their own key. The JWKS
	// serves the real N, so signature verification must fail.
	v, s := newVerifier(t)
	imposter := newTestSigner(t, s.kid, s.issuer)
	tok := imposter.sign(t, jwt.MapClaims{"aud": "svc-a"})
	_, err := v.Verify(context.Background(), tok)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken), "want ErrInvalidToken, got %v", err)
}

func TestVerify_HS256TokenRejected(t *testing.T) {
	// HS256 is a symmetric algo. Accepting it against a JWKS-served
	// RSA public key is the classic algorithm-confusion attack.
	// Our parser is configured with a whitelist of asymmetric algos
	// only; an HS256 token must be refused up front.
	v, s := newVerifier(t)
	claims := jwt.MapClaims{
		"iss": s.issuer,
		"aud": "svc-a",
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = s.kid
	raw, err := tok.SignedString([]byte("secret"))
	require.NoError(t, err)
	_, err = v.Verify(context.Background(), raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestVerify_EmptyTokenRejected(t *testing.T) {
	v, _ := newVerifier(t)
	_, err := v.Verify(context.Background(), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
	_, err = v.Verify(context.Background(), "   ")
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestVerify_GarbageTokenRejected(t *testing.T) {
	v, _ := newVerifier(t)
	_, err := v.Verify(context.Background(), "not-a-jwt")
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestVerify_MissingKIDHeaderRejected(t *testing.T) {
	// If a token has no kid we cannot pick a key deterministically.
	// Refuse rather than guessing (RFC 7515 §4.1.4 permits absence
	// but our security posture disallows it).
	v, s := newVerifier(t)
	claims := jwt.MapClaims{
		"iss": s.issuer,
		"aud": "svc-a",
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// deliberately not setting tok.Header["kid"]
	raw, err := tok.SignedString(s.priv)
	require.NoError(t, err)
	_, err = v.Verify(context.Background(), raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestVerify_ScopesSurfaceOnClaims(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "scope": "read write admin"})
	claims, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"read", "write", "admin"}, claims.Scopes)
	assert.True(t, claims.HasScope("write"))
	assert.False(t, claims.HasScope("delete"))
}

func TestVerify_ScopesFromArrayClaim(t *testing.T) {
	v, s := newVerifier(t)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "scp": []any{"read", "write"}})
	claims, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"read", "write"}, claims.Scopes)
}

func TestVerify_UnreachableJWKSErrors(t *testing.T) {
	// The constructor must fetch JWKS synchronously — a broken
	// endpoint must be discovered at startup, not at first request.
	// (This overlaps TestNewVerifier_ReturnsErrorWhenJWKSUnreachable
	// in config_test.go; kept here so a future refactor that moves
	// the initial fetch out of the constructor still sees the
	// obligation.)
	_, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:   "https://iss.test",
		JWKSURL:  "http://127.0.0.1:1/jwks",
		Audience: "svc-a",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrJWKSFetch))
}

func TestVerify_JWKSNon2xxErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	_, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:   "https://iss.test",
		JWKSURL:  srv.URL,
		Audience: "svc-a",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrJWKSFetch))
}

func TestVerify_JWKSMalformedBodyErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	t.Cleanup(srv.Close)
	_, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:   "https://iss.test",
		JWKSURL:  srv.URL,
		Audience: "svc-a",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrJWKSFetch))
}
