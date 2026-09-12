package maestro

// The audience, proven rather than documented.
//
// Context: every client of this service carried its own idea of the audience as
// configuration, and leartech-lighthouse-pr-events had
// `default:"leartech-maestro"` — not the "leartech-maestro-service" maestro
// enforces. It could never have authenticated, and nothing said so, because
// Hydra's `audience` field is an ALLOW-LIST rather than a default: asking for an
// audience you are not permitted returns a token WITHOUT it rather than an
// error, and the callee then refuses.
//
// So the tests that matter are the pair around VerifyAudience — a token for the
// right audience is accepted, one for the wrong audience is REFUSED AT
// CONSTRUCTION — plus the `aud`-shape cases, because a decoder that silently
// reads nothing would report every token as wrong-audience and get the check
// switched off.

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// jwtWithAud builds an unsigned JWT carrying the given `aud` claim JSON. The
// signature is never inspected — VerifyAudience deliberately does not verify
// it, because this is not an authentication decision, it is reading a
// credential we minted for ourselves.
func jwtWithAud(audJSON string) string {
	enc := func(s string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(s))
	}
	payload := `{"sub":"svc"}`
	if audJSON != "" {
		payload = `{"sub":"svc","aud":` + audJSON + `}`
	}
	return enc(`{"alg":"none","typ":"JWT"}`) + "." + enc(payload) + ".sig"
}

type tokenSourceFunc func(ctx context.Context) (*string, error)

func (f tokenSourceFunc) GetAuthToken(ctx context.Context) (*string, error) { return f(ctx) }

func staticToken(tok string) TokenSource {
	return tokenSourceFunc(func(context.Context) (*string, error) { return &tok, nil })
}

// ── the pair ────────────────────────────────────────────────────────────────

func TestVerifyAudience_CorrectAudienceIsAccepted(t *testing.T) {
	ts := staticToken(jwtWithAud(`["` + Audience + `"]`))
	if err := VerifyAudience(t.Context(), ts); err != nil {
		t.Fatalf("a token minted for %q was rejected: %v", Audience, err)
	}
}

// THE REGRESSION. This is lighthouse-pr-events' configured default.
func TestVerifyAudience_TheLighthouseDefaultIsRefused(t *testing.T) {
	ts := staticToken(jwtWithAud(`["leartech-maestro"]`))

	err := VerifyAudience(t.Context(), ts)
	if err == nil {
		t.Fatal(`a token minted for "leartech-maestro" was accepted. That is not an ` +
			`audience any service enforces and no Hydra client may mint it — it was ` +
			`leartech-lighthouse-pr-events' code default, and it could never have ` +
			`authenticated to maestro.`)
	}
	// The message has to name what it got AND explain the allow-list semantics,
	// or the reader concludes the token is broken rather than the request for it.
	for _, want := range []string{"leartech-maestro", Audience, "ALLOW-LIST", "LEARTECH_AUTH_TARGET_AUDIENCE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q, so it does not say how to fix it:\n%v", want, err)
		}
	}
}

// ── it must not pass vacuously ──────────────────────────────────────────────

// A token with no `aud` at all is what client_credentials mints when no
// audience is requested — the orchestrator-controller's state on 2026-09-12.
func TestVerifyAudience_MissingAudClaimIsRefused(t *testing.T) {
	if err := VerifyAudience(t.Context(), staticToken(jwtWithAud(""))); err == nil {
		t.Fatal("a token with no `aud` claim was accepted. client_credentials mints " +
			"exactly this when no audience is requested, and maestro refuses it.")
	}
}

func TestVerifyAudience_EmptyAudArrayIsRefused(t *testing.T) {
	if err := VerifyAudience(t.Context(), staticToken(jwtWithAud(`[]`))); err == nil {
		t.Fatal("a token with `aud: []` was accepted; an empty audience set is not " +
			"'unrestricted'")
	}
}

func TestVerifyAudience_NoTokenSourceIsRefused(t *testing.T) {
	if err := VerifyAudience(t.Context(), nil); err == nil {
		t.Fatal("a nil TokenSource passed the check — the audience was not verified at " +
			"all, which is indistinguishable from verifying it successfully")
	}
}

func TestVerifyAudience_MintFailureIsReported(t *testing.T) {
	ts := tokenSourceFunc(func(context.Context) (*string, error) {
		return nil, errors.New("hydra unreachable")
	})
	err := VerifyAudience(t.Context(), ts)
	if err == nil {
		t.Fatal("a failure to mint was treated as a successful audience check")
	}
	if !strings.Contains(err.Error(), "hydra unreachable") {
		t.Errorf("the underlying cause was swallowed: %v", err)
	}
}

func TestVerifyAudience_EmptyTokenIsRefused(t *testing.T) {
	if err := VerifyAudience(t.Context(), staticToken("")); err == nil {
		t.Fatal("an empty token string passed the audience check")
	}
}

func TestVerifyAudience_NonJWTIsReported(t *testing.T) {
	if err := VerifyAudience(t.Context(), staticToken("not-a-jwt")); err == nil {
		t.Fatal("a non-JWT passed the audience check")
	}
}

// ── the `aud` claim has two legal shapes ────────────────────────────────────

// RFC 7519 4.1.3: `aud` is a string OR an array of strings. Hydra emits an
// array; hand-rolled fixtures often emit a string. A decoder handling only one
// shape reports the other as wrong-audience, which is a false failure — and a
// check that cries wolf gets removed.
func TestVerifyAudience_AcceptsBothAudShapes(t *testing.T) {
	if err := VerifyAudience(t.Context(), staticToken(jwtWithAud(`"`+Audience+`"`))); err != nil {
		t.Errorf("a STRING-shaped aud was rejected: %v", err)
	}
	if err := VerifyAudience(t.Context(), staticToken(jwtWithAud(`["`+Audience+`"]`))); err != nil {
		t.Errorf("an ARRAY-shaped aud was rejected: %v", err)
	}
}

// A multi-audience token is legal and valid at each of its audiences.
func TestVerifyAudience_MultiAudienceContainingOursIsAccepted(t *testing.T) {
	tok := jwtWithAud(`["leartech-mcp","` + Audience + `","automated-agent"]`)
	if err := VerifyAudience(t.Context(), staticToken(tok)); err != nil {
		t.Errorf("a multi-audience token containing %q was rejected: %v", Audience, err)
	}
}

// ── construction refuses a bad audience ─────────────────────────────────────

func TestNewProducer_RefusesAWrongAudienceAtConstruction(t *testing.T) {
	_, err := NewProducer("https://maestro.internal", "leartech-lighthouse-pr-events",
		WithAudienceCheck(t.Context(), staticToken(jwtWithAud(`["leartech-maestro"]`))))
	if err == nil {
		t.Fatal("a producer was built on tokens maestro will refuse. Failing at boot is " +
			"the point: the alternative is a 401 on the first announce, attributed to " +
			"the wrong layer.")
	}
}

func TestNewProducer_AcceptsTheRightAudience(t *testing.T) {
	p, err := NewProducer("https://maestro.internal", "leartech-orchestrator-controller",
		WithAudienceCheck(t.Context(), staticToken(jwtWithAud(`["`+Audience+`"]`))))
	if err != nil {
		t.Fatalf("a correctly-audienced producer was refused: %v", err)
	}
	if p == nil {
		t.Fatal("no producer returned")
	}
}

// The audience is a constant on purpose. If it ever needs to differ per
// cluster, that is a design change and this test should be the thing that
// argues about it — not a config key someone sets wrongly.
func TestAudienceIsTheValueMaestroEnforces(t *testing.T) {
	if Audience != "leartech-maestro-service" {
		t.Fatalf("Audience is %q. maestro's chart enforces leartech-maestro-service in "+
			"BOTH clusters (verified in each GitOps overlay); the issuer differs per "+
			"cluster, the audience does not, because it names the service.", Audience)
	}
}
