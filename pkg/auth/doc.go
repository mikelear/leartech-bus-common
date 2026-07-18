// Package auth is the leartech Bearer / JWKS validator shared by every
// service that terminates HTTP for the leartech bus (event maestro,
// mcp-servers — including the PUBLIC Claude-facing MCP surface — and
// anything else that inherits this middleware).
//
// # Design
//
// This package is deliberately FAIL-CLOSED and AUDIENCE-BOUND with NO
// runtime knob to disable auth. If a caller cannot supply full
// configuration — non-empty issuer, JWKS source, and audience — the
// constructor returns a [Config.Validate] error. There is no
// pass-through, no "noop when unset" branch, and no environment
// variable that silently downgrades to unauthenticated operation.
// A public pod that misconfigures its auth MUST fail to start.
//
// The middleware validates every request:
//
//   - Authorization header must be `Bearer <token>` (no query-string,
//     no cookie fallback).
//   - The token signature must verify against a key served by the
//     configured JWKS endpoint (matched by `kid`).
//   - The token issuer (`iss`) must equal the configured issuer.
//   - The token audience (`aud`) must contain the configured audience
//     — matching a JWT with an empty `aud` or a mismatched `aud` is
//     always a 401.
//   - The token must not be expired (`exp`) and, if present, must be
//     usable at the current time (`nbf`, `iat`).
//   - Optional scope requirements are enforced via [ScopeRequired].
//
// On success, the middleware attaches a [*Claims] to the gin context
// under [ContextKey]; downstream handlers pull it out with [FromContext].
//
// # Rationale (why no disable path)
//
// The sister package this replaces (in leartech-go-common) returned a
// no-op "pass-through" client when its auth-server URL or issuer was
// unset. That behaviour makes it possible for a caller that thinks it
// has enabled auth (AUTH_REQUIRED=true) to run unauthenticated because
// a companion setting (LEARTECH_AUTH_SERVER_URL, historically
// MQUBE_AUTH_SERVER_URL — see the env-var migration note below) was
// left empty. The public MCP surface must not be exposed to that
// failure mode. Here there is no way to construct a Verifier that
// silently accepts every request — the type does not exist.
//
// # Environment variables
//
// This package exposes [LookupEnv] / [Getenv] as the shared entry
// point every consumer uses to read the auth-related environment
// variables (SERVER_URL, AUDIENCE, RESOURCE, AUTHORIZATION_SERVERS,
// RESOURCE_METADATA_URL). The current prefix is LEARTECH_AUTH_*.
// A legacy MQUBE_AUTH_* prefix is honoured as a fallback with a
// one-time deprecation WARN per variable so operators can spot and
// rename lingering references without any service breaking on the
// bus-common bump. See [env.go] for the full contract.
package auth
