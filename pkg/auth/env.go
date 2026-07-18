package auth

import (
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// Environment-variable prefixes for the fail-closed auth configuration.
//
// The leartech engineering org is renaming its historical MQUBE_* prefix
// to LEARTECH_* across every service. This package is the shared entry
// point that every consumer (mcp-servers, maestro, gateways) uses to
// read the auth-related env vars, so migrating the prefix here migrates
// it for the whole fleet — one dependency bump per consumer.
//
// Backward compatibility is REQUIRED during the migration window:
//
//   - The new [EnvPrefix] wins if set.
//   - If only the legacy [LegacyEnvPrefix] is set, its value is
//     returned AND a one-time WARN is logged per variable name (via
//     [LookupEnv] / [Getenv]), so operators can spot which vars still
//     need renaming without spamming logs on every request.
//   - If neither is set, the helpers return the empty string / false
//     exactly as [os.Getenv] / [os.LookupEnv] would.
//
// This keeps behaviour identical when only the old var is set — no
// service breaks on the bus-common bump. Consumers migrate their env
// blocks on their own schedule; the deprecation log is the nudge.
const (
	// EnvPrefix is the current, canonical prefix for every auth
	// environment variable consumed via [LookupEnv] / [Getenv].
	EnvPrefix = "LEARTECH_AUTH_"

	// LegacyEnvPrefix is the historical prefix that predates the
	// LEARTECH rename. Reads fall back to this prefix and emit a
	// one-time deprecation warning per suffix.
	LegacyEnvPrefix = "MQUBE_AUTH_"
)

// Well-known auth env-var suffixes. Callers may reference these
// constants rather than string-literal the suffix at each call site,
// which keeps rename typos out of the codebase.
//
// The values here are the SUFFIX only — [LookupEnv] prepends the
// prefix. So `LookupEnv(EnvSuffixServerURL)` reads
// `LEARTECH_AUTH_SERVER_URL` first, then `MQUBE_AUTH_SERVER_URL`.
const (
	// EnvSuffixServerURL is the base URL of the OIDC provider — the
	// `iss` claim value AND the origin from which the JWKS is
	// discovered. Populates [Config.Issuer] in most consumers.
	EnvSuffixServerURL = "SERVER_URL"

	// EnvSuffixAudience is the required `aud` claim value.
	// Populates [Config.Audience].
	EnvSuffixAudience = "AUDIENCE"

	// EnvSuffixResource is the RFC 8707 resource indicator that
	// s2s clients pass to the token endpoint when minting a bearer
	// for this service. Used by the auth minter, not the verifier.
	EnvSuffixResource = "RESOURCE"

	// EnvSuffixAuthorizationServers is a comma-separated list of
	// authorization-server issuer URLs advertised by this service
	// via /.well-known/oauth-protected-resource (RFC 9728 style).
	EnvSuffixAuthorizationServers = "AUTHORIZATION_SERVERS"

	// EnvSuffixResourceMetadataURL is the fully qualified URL that
	// this service advertises for its RFC 9728 protected-resource
	// metadata document. Consumed by MCP clients to discover the
	// audience + authorization server.
	EnvSuffixResourceMetadataURL = "RESOURCE_METADATA_URL"
)

// deprecationWarnedFor tracks which legacy env-var suffixes have
// already been logged, so the deprecation WARN fires exactly once
// per process per suffix. sync.Map is the right shape here — reads
// dominate writes (the warning fires at most once per key) and we
// need lock-free reads on the common "already warned" path.
var deprecationWarnedFor sync.Map //nolint:gochecknoglobals // process-lifetime warn dedupe

// LookupEnv reads an auth environment variable using the current
// [EnvPrefix]+suffix and falls back to [LegacyEnvPrefix]+suffix if
// the new-prefix variable is unset OR set to the empty string.
//
// Semantics:
//
//   - If `LEARTECH_AUTH_<suffix>` is set (even to "") → returns its
//     value with ok=true. This lets operators explicitly clear an
//     inherited legacy value by exporting the new var empty.
//   - Else if `MQUBE_AUTH_<suffix>` is set (even to "") → returns
//     its value with ok=true AND logs a one-time WARN keyed on
//     the suffix.
//   - Else → returns "" with ok=false.
//
// The one-time WARN uses [sync.Map] to guarantee at-most-once
// logging per suffix per process, so a hot-path caller that reads
// the same var thousands of times per second does not flood logs.
func LookupEnv(suffix string) (string, bool) {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return "", false
	}
	if v, ok := os.LookupEnv(EnvPrefix + suffix); ok {
		return v, true
	}
	legacyName := LegacyEnvPrefix + suffix
	if v, ok := os.LookupEnv(legacyName); ok {
		warnDeprecatedOnce(suffix, legacyName)
		return v, true
	}
	return "", false
}

// Getenv is a convenience wrapper around [LookupEnv] that discards
// the presence bit. It mirrors [os.Getenv]'s shape — returning "" for
// an unset variable.
//
// Prefer [LookupEnv] when the empty string is a meaningful value
// (e.g. "explicitly disable this feature") versus "unset".
func Getenv(suffix string) string {
	v, _ := LookupEnv(suffix)
	return v
}

// ResetDeprecationWarnings clears the per-process dedupe cache used
// by [LookupEnv] so a subsequent legacy-var read re-emits the WARN.
// Exported for tests — production code has no reason to call this.
func ResetDeprecationWarnings() {
	deprecationWarnedFor.Range(func(k, _ any) bool {
		deprecationWarnedFor.Delete(k)
		return true
	})
}

// warnDeprecatedOnce logs the deprecation notice for a legacy-prefix
// env var, emitting exactly once per suffix per process.
//
// The message names both the legacy variable that was consumed AND
// the replacement variable so an operator can grep-and-fix from a
// single log line without cross-referencing docs.
func warnDeprecatedOnce(suffix, legacyName string) {
	if _, loaded := deprecationWarnedFor.LoadOrStore(suffix, struct{}{}); loaded {
		return
	}
	log.Warn().
		Str("legacy_var", legacyName).
		Str("replacement", EnvPrefix+suffix).
		Msg("auth: reading deprecated MQUBE_AUTH_* env var; rename to LEARTECH_AUTH_* — legacy fallback will be removed in a future major version")
}
