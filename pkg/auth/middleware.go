package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ContextKey is the gin context key under which verified [Claims]
// are stored. Exported so downstream handlers can pull them out
// via [FromContext] or directly from `c.Get(auth.ContextKey)`.
const ContextKey = "leartech.auth.claims"

// Middleware returns a gin handler that requires a valid Bearer
// token on every request. Behaviour is deliberately fail-closed:
//
//   - v MUST be non-nil. A nil verifier panics at construction time —
//     there is no "auth off" path.
//   - A missing or malformed Authorization header responds 401 with
//     no body-side clue about why (to avoid oracles).
//   - A token that fails verification (bad signature, wrong issuer,
//     wrong audience, expired, unknown kid) responds 401.
//   - On success, [*Claims] is stored under [ContextKey] and the
//     handler chain continues.
//
// This is the ONLY entry point for putting the verifier in front of
// an HTTP handler. There is no companion "MaybeMiddleware" that
// silently no-ops when misconfigured.
func Middleware(v Verifier) gin.HandlerFunc {
	if v == nil {
		// This is a programmer error, not a runtime one. Panicking
		// at wiring time is preferable to silently letting requests
		// through. The main package's construction path is
		// responsible for handling the [NewVerifier] error and
		// aborting startup — this panic exists so any code path that
		// tries to skip that error handling fails loudly.
		panic("auth: Middleware called with nil Verifier — construction error not handled")
	}
	return func(c *gin.Context) {
		raw, ok := extractBearer(c.GetHeader("Authorization"))
		if !ok {
			abort(c, ErrMissingBearer)
			return
		}
		claims, err := v.Verify(c.Request.Context(), raw)
		if err != nil {
			abort(c, err)
			return
		}
		// Store on both the gin Keys map (for c.Get style access)
		// AND the request context (for FromContext + background
		// goroutines spawned from the handler). Keeping both in sync
		// means downstream code can pick either idiom.
		c.Set(ContextKey, claims)
		c.Request = c.Request.WithContext(WithClaims(c.Request.Context(), claims))
		c.Next()
	}
}

// ScopeRequired returns a middleware that must be chained AFTER
// [Middleware]. It rejects requests whose verified claims do not
// carry every scope in `required`.
func ScopeRequired(required ...string) gin.HandlerFunc {
	// Freeze the argument slice into a package-owned copy so the
	// caller cannot mutate the requirement after wiring.
	needed := append([]string(nil), required...)
	return func(c *gin.Context) {
		cl, ok := FromContext(c.Request.Context())
		if !ok {
			// Middleware must run first — if the auth claims aren't
			// present, refuse the request with a stable 401 rather
			// than silently passing.
			abort(c, ErrMissingBearer)
			return
		}
		for _, s := range needed {
			if !cl.HasScope(s) {
				abort(c, ErrScopeMissing)
				return
			}
		}
		c.Next()
	}
}

// FromContext returns the verified claims previously stored by
// [Middleware]. It works with both the gin.Context (which embeds
// context.Context) and a plain context.Context that has been
// annotated by [WithClaims].
func FromContext(ctx context.Context) (*Claims, bool) {
	if ctx == nil {
		return nil, false
	}
	// gin stashes values under a private key on its Keys map; when
	// callers pass c.Request.Context() we need to reach through to
	// the gin context. gin.Context implements context.Context via
	// its Context field; we probe both shapes.
	if v := ctx.Value(claimsCtxKey{}); v != nil {
		if c, ok := v.(*Claims); ok {
			return c, true
		}
	}
	if gc, ok := ctx.(*gin.Context); ok {
		if v, exists := gc.Get(ContextKey); exists {
			if c, ok := v.(*Claims); ok {
				return c, true
			}
		}
	}
	return nil, false
}

// WithClaims returns a context annotated with the given claims.
// Useful in non-gin code paths (e.g. background workers spawned
// from a request) that want to propagate the subject.
func WithClaims(parent context.Context, c *Claims) context.Context {
	return context.WithValue(parent, claimsCtxKey{}, c)
}

// claimsCtxKey is the private key type used by [WithClaims] /
// [FromContext] so the value cannot be overwritten by outside code.
type claimsCtxKey struct{}

// extractBearer parses the Authorization header. It is strict:
// the scheme MUST be "Bearer" (case-insensitive per RFC 6750 §2.1)
// and there MUST be exactly one whitespace-separated token.
func extractBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	if parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// abort short-circuits the request with a 401. The error is logged
// for operators; the response body deliberately does not surface the
// classification (to avoid becoming an oracle for token guessing).
func abort(c *gin.Context, err error) {
	// Log at debug — a compromised deployment could otherwise use
	// info-level logging as a channel to distinguish "wrong sig"
	// from "wrong aud". Debug is off by default in production.
	log.Ctx(c.Request.Context()).Debug().Err(err).Msg("auth: request denied")
	// Set a marker on the context so downstream logging middleware
	// can distinguish auth failures from other 401s if desired.
	c.Set("leartech.auth.deny_reason", classify(err))
	c.AbortWithStatus(http.StatusUnauthorized)
}

// classify returns a short, stable string describing which sentinel
// caused the denial. Used by log middleware, NOT surfaced to the
// client.
func classify(err error) string {
	switch {
	case errors.Is(err, ErrMissingBearer):
		return "missing_bearer"
	case errors.Is(err, ErrAudienceMismatch):
		return "audience_mismatch"
	case errors.Is(err, ErrIssuerMismatch):
		return "issuer_mismatch"
	case errors.Is(err, ErrUnknownKID):
		return "unknown_kid"
	case errors.Is(err, ErrJWKSFetch):
		return "jwks_unreachable"
	case errors.Is(err, ErrScopeMissing):
		return "scope_missing"
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	default:
		return "unknown"
	}
}
