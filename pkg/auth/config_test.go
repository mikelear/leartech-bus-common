//go:build unit

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file are the acceptance contract for the
// fail-closed / no-noop invariant. Any change to pkg/auth MUST keep
// these green — the initiative is deliberately breaking, but this
// is the ONE promise that must never regress: you cannot construct
// a Verifier without full config, and there is no runtime disable.

func TestConfig_Validate_ErrorsOnMissingIssuer(t *testing.T) {
	cfg := auth.Config{
		JWKSURL:  "https://example.test/.well-known/jwks.json",
		Audience: "svc-a",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig), "want ErrMissingConfig, got %v", err)
	assert.Contains(t, err.Error(), "Issuer")
}

func TestConfig_Validate_ErrorsOnMissingJWKSURL(t *testing.T) {
	cfg := auth.Config{Issuer: "https://iss.test", Audience: "svc-a"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
	assert.Contains(t, err.Error(), "JWKSURL")
}

func TestConfig_Validate_ErrorsOnMissingAudience(t *testing.T) {
	cfg := auth.Config{
		Issuer:  "https://iss.test",
		JWKSURL: "https://example.test/.well-known/jwks.json",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
	assert.Contains(t, err.Error(), "Audience")
}

func TestConfig_Validate_ErrorsOnAllMissing(t *testing.T) {
	err := auth.Config{}.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
	// All three field names must appear so the caller sees which
	// pieces of config to add.
	assert.Contains(t, err.Error(), "Issuer")
	assert.Contains(t, err.Error(), "JWKSURL")
	assert.Contains(t, err.Error(), "Audience")
}

func TestConfig_Validate_TreatsWhitespaceOnlyAsMissing(t *testing.T) {
	// Whitespace-only values are the classic accidental config —
	// caller reads MQUBE_AUTH_AUDIENCE="" from env and stringifies
	// into "   ". These MUST error, not silently pass.
	cfg := auth.Config{Issuer: "  ", JWKSURL: "  ", Audience: "  "}
	err := cfg.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
}

func TestConfig_Validate_PassesWithFullConfig(t *testing.T) {
	cfg := auth.Config{
		Issuer:   "https://iss.test",
		JWKSURL:  "https://iss.test/.well-known/jwks.json",
		Audience: "svc-a",
	}
	require.NoError(t, cfg.Validate())
}

func TestNewVerifier_RejectsEmptyConfig(t *testing.T) {
	// Fail-closed at the constructor: no way to get a *Verifier
	// with nothing configured. The initiative's core promise.
	_, err := auth.NewVerifier(context.Background(), auth.Config{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
	assert.True(t, auth.IsMissingConfig(err))
}

func TestNewVerifier_RejectsMissingAudience(t *testing.T) {
	_, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:  "https://iss.test",
		JWKSURL: "https://iss.test/.well-known/jwks.json",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMissingConfig))
}

func TestNewVerifier_ReturnsErrorWhenJWKSUnreachable(t *testing.T) {
	// Even with full config, if the JWKS endpoint is unreachable we
	// MUST fail the constructor. Callers must not paper over this by
	// falling back to unauthenticated operation.
	_, err := auth.NewVerifier(context.Background(), auth.Config{
		Issuer:   "https://iss.test",
		JWKSURL:  "http://127.0.0.1:1/jwks", // reserved port, guaranteed refused
		Audience: "svc-a",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrJWKSFetch), "want ErrJWKSFetch, got %v", err)
}
