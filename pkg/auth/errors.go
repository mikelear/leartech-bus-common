package auth

import "errors"

// Sentinel errors returned by the package. Wrap with [errors.Is] to
// distinguish causes at call sites.
//
// The naming convention follows the errname linter rule: every value
// is prefixed with `Err`.
var (
	// ErrMissingConfig is returned by [Config.Validate] and [NewVerifier]
	// when any required field on [Config] is empty. Callers MUST NOT
	// treat this as "auth disabled" — the constructor deliberately
	// refuses to build a Verifier without a full, enforceable
	// configuration.
	ErrMissingConfig = errors.New("auth: missing required configuration")

	// ErrMissingBearer is returned by the middleware when a request
	// has no `Authorization: Bearer <token>` header, or when the
	// header is malformed.
	ErrMissingBearer = errors.New("auth: missing or malformed Authorization: Bearer header")

	// ErrInvalidToken is returned when a token's signature does not
	// verify, when its issuer does not match, when its expiry has
	// passed, or when any other structural check fails.
	ErrInvalidToken = errors.New("auth: token invalid")

	// ErrAudienceMismatch is returned when a token's `aud` claim does
	// not contain the configured audience. This is a REQUIRED check —
	// there is no way to disable it. A token minted for another
	// service is always rejected.
	ErrAudienceMismatch = errors.New("auth: token audience does not match configured audience")

	// ErrIssuerMismatch is returned when a token's `iss` claim does
	// not exactly match the configured issuer.
	ErrIssuerMismatch = errors.New("auth: token issuer does not match configured issuer")

	// ErrJWKSFetch is returned when the JWKS endpoint cannot be
	// reached, returns a non-2xx status, or serves a body that does
	// not decode as a JWKS. It always denies the request.
	ErrJWKSFetch = errors.New("auth: failed to fetch JWKS")

	// ErrUnknownKID is returned when a token's `kid` header does not
	// match any key served by the current JWKS document (after a
	// forced refresh).
	ErrUnknownKID = errors.New("auth: token kid not present in JWKS")

	// ErrScopeMissing is returned by the [ScopeRequired] middleware
	// when the verified token is missing one of the required scopes.
	ErrScopeMissing = errors.New("auth: token missing required scope")
)
