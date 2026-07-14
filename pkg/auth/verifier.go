package auth

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

// Verifier verifies a raw JWT string and returns the parsed [Claims]
// on success. Every implementation MUST enforce audience matching
// against a configured value — the interface is intentionally narrow
// so callers cannot ask "is auth on?" (it always is).
type Verifier interface {
	// Verify parses the token, verifies its signature against the
	// JWKS, checks issuer, audience, expiry, and returns the claims.
	// Every failure mode wraps a sentinel error from this package.
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}

// Claims is the verified claim set attached to the gin context after
// successful authentication. It is intentionally opinionated — the
// fields we expose are the ones downstream handlers actually use.
type Claims struct {
	Subject   string
	Issuer    string
	Audience  []string
	Scopes    []string
	IssuedAt  time.Time
	ExpiresAt time.Time
	// Raw is the underlying claims map for anything not surfaced
	// as a first-class field. Handlers should prefer the typed
	// fields where possible.
	Raw jwt.MapClaims
}

// HasScope reports whether the claims include the given scope.
// Scopes are compared exactly (case-sensitive) — no wildcard support.
func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// verifier is the production implementation of [Verifier]. It is
// unexported because the constructor is the only supported entry
// point — this prevents callers from building an instance with a nil
// JWKS client or missing audience.
type verifier struct {
	cfg  Config
	jwks *jwksClient
}

// NewVerifier constructs a fail-closed, audience-bound Verifier.
//
// The constructor calls [Config.Validate] first. On any missing
// required field it returns an error wrapping [ErrMissingConfig] —
// the caller MUST NOT swallow this error as "auth is disabled".
// A missing config means the service cannot safely accept requests.
//
// On success the initial JWKS fetch has already completed, so the
// first request served by the middleware does not pay the fetch
// latency AND a broken JWKS endpoint is discovered at startup rather
// than at first traffic.
func NewVerifier(ctx context.Context, cfg Config) (Verifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg = cfg.defaults()
	jc, err := newJWKSClient(ctx, cfg.JWKSURL, cfg.HTTPClient, cfg.RefreshInterval, cfg.Now)
	if err != nil {
		return nil, err
	}
	return &verifier{cfg: cfg, jwks: jc}, nil
}

// Verify implements [Verifier].
func (v *verifier) Verify(ctx context.Context, raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.Wrap(ErrInvalidToken, "empty token")
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithTimeFunc(v.cfg.Now),
	)

	claims := jwt.MapClaims{}
	tok, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.Wrap(ErrInvalidToken, "token missing kid")
		}
		key, kerr := v.jwks.keyForKID(ctx, kid)
		if kerr != nil {
			return nil, kerr
		}
		return key, nil
	})
	if err != nil {
		// Classify: issuer mismatch → ErrIssuerMismatch; already-a-
		// package-sentinel → propagate; anything else → ErrInvalidToken.
		if errors.Is(err, jwt.ErrTokenInvalidIssuer) {
			return nil, errors.Wrap(ErrIssuerMismatch, err.Error())
		}
		if errors.Is(err, ErrUnknownKID) || errors.Is(err, ErrJWKSFetch) {
			return nil, err
		}
		return nil, errors.Wrap(ErrInvalidToken, err.Error())
	}
	if tok == nil || !tok.Valid {
		return nil, errors.Wrap(ErrInvalidToken, "token not valid")
	}

	// Audience check: REQUIRED, non-empty configured value MUST be
	// present in the token's `aud` claim. This is intentionally
	// re-checked here even though jwt/v5 supports WithAudience —
	// keeping the enforcement inside our package means the sentinel
	// error is stable across jwt-lib upgrades.
	if !audienceMatches(claims, v.cfg.Audience) {
		return nil, ErrAudienceMismatch
	}

	return buildClaims(claims), nil
}

// audienceMatches reports whether the `aud` claim on the map contains
// the required audience. The `aud` claim per RFC 7519 may be either a
// string or an array of strings; both are handled. An empty or missing
// claim is a mismatch.
func audienceMatches(claims jwt.MapClaims, required string) bool {
	if required == "" {
		return false
	}
	aud, ok := claims["aud"]
	if !ok {
		return false
	}
	switch a := aud.(type) {
	case string:
		return a == required
	case []string:
		for _, v := range a {
			if v == required {
				return true
			}
		}
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok && s == required {
				return true
			}
		}
	}
	return false
}

// buildClaims projects a jwt.MapClaims into our opinionated Claims
// shape. Missing / mis-typed fields become zero values — validity
// checks already ran in Verify.
func buildClaims(m jwt.MapClaims) *Claims {
	out := &Claims{Raw: m}
	if s, ok := m["sub"].(string); ok {
		out.Subject = s
	}
	if s, ok := m["iss"].(string); ok {
		out.Issuer = s
	}
	out.Audience = extractAudience(m)
	out.Scopes = extractScopes(m)
	if v, ok := m["iat"]; ok {
		out.IssuedAt = coerceUnix(v)
	}
	if v, ok := m["exp"]; ok {
		out.ExpiresAt = coerceUnix(v)
	}
	return out
}

func extractAudience(m jwt.MapClaims) []string {
	aud, ok := m["aud"]
	if !ok {
		return nil
	}
	switch a := aud.(type) {
	case string:
		return []string{a}
	case []string:
		return append([]string(nil), a...)
	case []any:
		out := make([]string, 0, len(a))
		for _, v := range a {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// extractScopes reads scopes from the two conventions leartech tokens
// use: a space-separated string in `scope` (RFC 8693 style), or an
// array in `scp`. First match wins.
func extractScopes(m jwt.MapClaims) []string {
	if v, ok := m["scope"].(string); ok && v != "" {
		return strings.Fields(v)
	}
	if v, ok := m["scp"].([]any); ok {
		out := make([]string, 0, len(v))
		for _, s := range v {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

// coerceUnix accepts float64 (the default for json.Number decoded
// into any) or int64 and returns the corresponding time.
func coerceUnix(v any) time.Time {
	switch n := v.(type) {
	case float64:
		return time.Unix(int64(n), 0).UTC()
	case int64:
		return time.Unix(n, 0).UTC()
	case int:
		return time.Unix(int64(n), 0).UTC()
	case json.Number:
		// json.Number is present when a caller sets UseNumber; we
		// don't in this package, but tolerate it for
		// interoperability with anything that pre-parses claims.
		if i, err := n.Int64(); err == nil {
			return time.Unix(i, 0).UTC()
		}
	}
	return time.Time{}
}
