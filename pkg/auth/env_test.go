//go:build unit

package auth_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below are the acceptance contract for the backward-compatible
// MQUBE_AUTH_* → LEARTECH_AUTH_* rename. Any change to pkg/auth/env.go
// MUST keep these green — the invariant is that consumers already setting
// only the legacy vars see no behaviour change, only a one-time WARN.

func TestLookupEnv_ReturnsNewPrefixWhenSet(t *testing.T) {
	auth.ResetDeprecationWarnings()
	t.Setenv("LEARTECH_AUTH_SERVER_URL", "https://new.example")
	// Old also set; new must win — no deprecation warning either.
	t.Setenv("MQUBE_AUTH_SERVER_URL", "https://old.example")

	buf, restore := captureLogs(t)
	defer restore()

	v, ok := auth.LookupEnv(auth.EnvSuffixServerURL)
	require.True(t, ok)
	assert.Equal(t, "https://new.example", v)
	assert.NotContains(t, buf.String(), "deprecated",
		"no deprecation warning must fire when new-prefix var is set")
}

func TestLookupEnv_FallsBackToLegacyPrefix(t *testing.T) {
	auth.ResetDeprecationWarnings()
	// Only the legacy var is set — fallback path with one-time WARN.
	t.Setenv("MQUBE_AUTH_AUDIENCE", "legacy-aud")

	buf, restore := captureLogs(t)
	defer restore()

	v, ok := auth.LookupEnv(auth.EnvSuffixAudience)
	require.True(t, ok)
	assert.Equal(t, "legacy-aud", v)

	logged := buf.String()
	assert.Contains(t, logged, "MQUBE_AUTH_AUDIENCE")
	assert.Contains(t, logged, "LEARTECH_AUTH_AUDIENCE")
	assert.Contains(t, logged, "deprecated")
}

func TestLookupEnv_UnsetReturnsEmptyAndFalse(t *testing.T) {
	auth.ResetDeprecationWarnings()
	// Explicitly unset both to defeat any test-parallel leakage.
	// t.Setenv with the caller-owned key is not strictly needed here
	// because os.LookupEnv checks *actual* env, but we clear via
	// Setenv to a placeholder and immediately unset via t.Cleanup —
	// the pragmatic path is just to verify with a suffix no other
	// test uses.
	buf, restore := captureLogs(t)
	defer restore()

	v, ok := auth.LookupEnv("NEVER_SET_SUFFIX_XYZ")
	assert.False(t, ok)
	assert.Equal(t, "", v)
	assert.NotContains(t, buf.String(), "deprecated",
		"no deprecation warning must fire when the legacy var is also unset")
}

func TestLookupEnv_EmptyStringInNewPrefixWins(t *testing.T) {
	// Operators may deliberately clear an inherited legacy value by
	// exporting the new prefix as "". This must be respected —
	// falling back to the legacy value would silently un-clear it.
	auth.ResetDeprecationWarnings()
	t.Setenv("LEARTECH_AUTH_RESOURCE", "")
	t.Setenv("MQUBE_AUTH_RESOURCE", "should-not-be-returned")

	buf, restore := captureLogs(t)
	defer restore()

	v, ok := auth.LookupEnv(auth.EnvSuffixResource)
	require.True(t, ok, "new-prefix var set to empty string must still be ok=true")
	assert.Equal(t, "", v)
	assert.NotContains(t, buf.String(), "deprecated")
}

func TestLookupEnv_EmptyStringInLegacyPrefixReturnsEmptyWithWarn(t *testing.T) {
	auth.ResetDeprecationWarnings()
	t.Setenv("MQUBE_AUTH_RESOURCE_METADATA_URL", "")

	buf, restore := captureLogs(t)
	defer restore()

	v, ok := auth.LookupEnv(auth.EnvSuffixResourceMetadataURL)
	require.True(t, ok, "legacy var set to empty string must still be ok=true")
	assert.Equal(t, "", v)
	assert.Contains(t, buf.String(), "MQUBE_AUTH_RESOURCE_METADATA_URL")
}

func TestLookupEnv_DeprecationWarnsExactlyOncePerSuffix(t *testing.T) {
	auth.ResetDeprecationWarnings()
	t.Setenv("MQUBE_AUTH_AUTHORIZATION_SERVERS", "https://as.example")

	buf, restore := captureLogs(t)
	defer restore()

	// Fire the lookup a few times — the WARN must appear exactly once.
	for i := 0; i < 5; i++ {
		v, ok := auth.LookupEnv(auth.EnvSuffixAuthorizationServers)
		require.True(t, ok)
		assert.Equal(t, "https://as.example", v)
	}
	warnCount := strings.Count(buf.String(), "deprecated")
	assert.Equal(t, 1, warnCount, "deprecation must fire exactly once per suffix per process")
}

func TestLookupEnv_DifferentSuffixesWarnIndependently(t *testing.T) {
	auth.ResetDeprecationWarnings()
	t.Setenv("MQUBE_AUTH_SERVER_URL", "https://iss.example")
	t.Setenv("MQUBE_AUTH_AUDIENCE", "svc-a")

	buf, restore := captureLogs(t)
	defer restore()

	_, _ = auth.LookupEnv(auth.EnvSuffixServerURL)
	_, _ = auth.LookupEnv(auth.EnvSuffixAudience)
	// Repeat — each suffix already warned; no extra warnings.
	_, _ = auth.LookupEnv(auth.EnvSuffixServerURL)
	_, _ = auth.LookupEnv(auth.EnvSuffixAudience)

	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, "MQUBE_AUTH_SERVER_URL"))
	assert.Equal(t, 1, strings.Count(logged, "MQUBE_AUTH_AUDIENCE"))
}

func TestLookupEnv_EmptySuffixReturnsFalseWithoutReadingEnv(t *testing.T) {
	auth.ResetDeprecationWarnings()
	// Guard against accidental prefix-only reads that would resolve to
	// LEARTECH_AUTH_ / MQUBE_AUTH_ (no suffix) — those keys are not a
	// valid identifier and callers must not receive a value for them.
	t.Setenv("LEARTECH_AUTH_", "should-not-be-returned")
	t.Setenv("MQUBE_AUTH_", "should-not-be-returned-either")

	v, ok := auth.LookupEnv("")
	assert.False(t, ok)
	assert.Equal(t, "", v)

	v, ok = auth.LookupEnv("   ")
	assert.False(t, ok, "whitespace-only suffix must be treated as empty")
	assert.Equal(t, "", v)
}

func TestGetenv_MirrorsLookupEnvValue(t *testing.T) {
	auth.ResetDeprecationWarnings()
	t.Setenv("LEARTECH_AUTH_SERVER_URL", "https://from-getenv.example")

	assert.Equal(t, "https://from-getenv.example", auth.Getenv(auth.EnvSuffixServerURL))
	assert.Equal(t, "", auth.Getenv("STILL_UNSET_SUFFIX_XYZ"))
}

// captureLogs redirects the global zerolog logger into an in-memory
// buffer so tests can assert on log output. Returns the buffer plus a
// restore function; the restore function MUST be deferred so subsequent
// tests see the original logger.
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := log.Logger
	log.Logger = zerolog.New(buf).With().Timestamp().Logger()
	return buf, func() { log.Logger = orig }
}
