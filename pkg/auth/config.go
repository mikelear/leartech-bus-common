package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Config holds the fully-required configuration for building a
// [Verifier]. Every field except HTTPClient and Now is REQUIRED —
// [Config.Validate] returns an error wrapping [ErrMissingConfig] if
// any required field is empty.
//
// There is deliberately no "disable" flag, no "optional audience"
// flag, and no environment-based auto-detection. Callers that cannot
// supply full config must handle the constructor error — they must
// not fall back to "run without auth".
type Config struct {
	// Issuer is the expected `iss` claim in every accepted token
	// AND the base URL of the OIDC provider whose JWKS is fetched.
	// Non-empty is required.
	Issuer string

	// JWKSURL is the JWKS endpoint that serves the public keys used
	// to verify token signatures. Typically
	// `<issuer>/.well-known/jwks.json`. Non-empty is required so the
	// discovery source is always explicit — nothing is auto-derived
	// from Issuer to avoid surprise upgrades.
	JWKSURL string

	// Audience is the required `aud` claim value. A token whose
	// `aud` is empty, does not contain this value, or is set to a
	// different value is rejected. Non-empty is required.
	Audience string

	// RefreshInterval controls how often the JWKS document is
	// re-fetched in the background. Defaults to 10 minutes.
	// On unknown-kid errors the cache is forcibly refreshed once
	// out-of-band regardless of this interval.
	RefreshInterval time.Duration

	// HTTPClient is used to fetch the JWKS document. Nil selects a
	// package-owned client with a 5s timeout.
	HTTPClient *http.Client

	// Now is a clock function used for token expiry checks. Nil
	// selects [time.Now]. Tests can inject a fixed clock.
	Now func() time.Time
}

// Validate returns nil when the Config carries every field required to
// build a fail-closed, audience-bound Verifier. On any missing field
// it returns an error wrapping [ErrMissingConfig] and identifying the
// offending fields.
//
// Validate is intentionally strict: this is the single choke point
// that prevents a partial config from silently downgrading to
// unauthenticated operation.
func (c Config) Validate() error {
	var missing []string
	if strings.TrimSpace(c.Issuer) == "" {
		missing = append(missing, "Issuer")
	}
	if strings.TrimSpace(c.JWKSURL) == "" {
		missing = append(missing, "JWKSURL")
	}
	if strings.TrimSpace(c.Audience) == "" {
		missing = append(missing, "Audience")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingConfig, strings.Join(missing, ", "))
	}
	return nil
}

// defaults returns a Config with unset optional fields populated.
// It does NOT mutate the receiver — callers get a copy.
func (c Config) defaults() Config {
	out := c
	if out.RefreshInterval <= 0 {
		out.RefreshInterval = 10 * time.Minute
	}
	if out.HTTPClient == nil {
		out.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if out.Now == nil {
		out.Now = time.Now
	}
	return out
}

// IsMissingConfig is a convenience for callers that want to
// distinguish "constructor rejected the config" from "runtime error".
// Both cases MUST result in a failed startup, but callers may log the
// two shapes differently.
func IsMissingConfig(err error) bool {
	return errors.Is(err, ErrMissingConfig)
}
