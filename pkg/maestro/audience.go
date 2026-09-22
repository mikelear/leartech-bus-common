package maestro

// MAESTRO'S AUDIENCE, AND WHY IT IS A CONSTANT HERE.
//
// Every client of this service has so far carried its own idea of the audience,
// as configuration, and at least one of them was wrong:
//
//	// leartech-lighthouse-pr-events/internal/config/config.go
//	AuthAudience string `envconfig:"LEARTECH_AUTH_AUDIENCE" default:"leartech-maestro"`
//
// maestro enforces "leartech-maestro-service". "leartech-maestro" is not an
// audience any service accepts, so that publisher could not authenticate — and
// nothing said so, because Hydra's `audience` field is an ALLOW-LIST rather
// than a default: asking for an audience you are not permitted does not error,
// it returns a token without it. The callee then refuses. That is Hydra's
// behaviour, not an estate setting, and it is the reason a wrong audience
// surfaces as a 401 at the callee rather than a failure at the caller.
//
// THE COUNTS THAT USED TO BE HERE HAVE BEEN REMOVED, because they rotted.
// This comment read "mintable by none of the 62 clients", measured
// 2026-09-12. On 2026-09-22 it was two of 72 — the sentence was false, sat in
// a package with no comment gate, and was still being read as current. A
// count of live cluster state goes stale the moment the cluster changes, so
// here is how to ask instead:
//
//	kubectl -n jx-staging exec deploy/... -- \
//	  curl -s "http://leartech-auth-service-hydra-admin:4445/admin/clients?page_size=500" \
//	  | jq '[.[]|select(.audience|index("leartech-maestro-service"))|.client_id]'
//
// Worth knowing what that answers. An audience ENFORCED by a running service
// but present in no client's allow-list names a service no caller can
// authenticate to. That is worth finding, and it is also where the query's
// false positives live, so triage before raising anything.
//
// The canonical false positive, as of 2026-09-22, is leartech-gate. It
// enforces an audience no client can mint, on both clusters — and that is
// correct. The repo is the GOLDEN GO SERVICE TEMPLATE, and its deployment is
// the template demonstrating itself, auth included. Nothing calls it because
// nothing is meant to: seven days of logs are 75,919 requests, every one a
// kubelet health probe. The estate uses the repo's OTHER binary, gate-cli,
// which Tekton runs by overriding the image entrypoint and which needs no
// token at all.
//
// So read the traffic alongside the allow-list. The allow-list says who could
// call a service; only the traffic says whether anyone meant to.
//
// An audience is not deployment configuration. The issuer is — the two clusters
// run different Hydras — but the audience names the SERVICE, and it is
// identical in both clusters (verified in both GitOps overlays). Making it
// configurable bought nothing and cost a silent outage path, so it is a
// constant, in the shared client, once.
//
// AND IT IS VERIFIED, NOT DOCUMENTED. The previous client said
// `aud=leartech-maestro` in its package comment, confidently and wrongly, for
// as long as it existed. A comment cannot fail. NewProducer now mints one token
// at construction and refuses to build if that token's `aud` does not contain
// Audience — so a misconfigured publisher fails at boot with the reason, rather
// than at the first 401 attributed to the wrong layer.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

// Audience is the RFC 8707 audience maestro enforces on inbound tokens. It is
// deliberately not configurable — see the package comment above.
const Audience = "leartech-maestro-service"

// TokenSource is the minimal surface needed to see what a caller's tokens
// actually carry. It is satisfied by auth.TokenGetter from leartech-go-common
// (and by anything else with the same method), so this package does not take a
// dependency on a particular auth library to perform the check.
type TokenSource interface {
	GetAuthToken(ctx context.Context) (*string, error)
}

// VerifyAudience mints one token from ts and checks that maestro would accept
// it, returning an error naming what the token actually carried.
//
// The token is decoded WITHOUT signature verification, deliberately: this is
// not an authentication decision. We are inspecting a credential we just minted
// for ourselves, to answer "is this addressed to maestro" before relying on it.
// Verifying the signature here would require the issuer's JWKS and prove
// nothing extra — a validly-signed token for the wrong audience is exactly the
// case being caught.
func VerifyAudience(ctx context.Context, ts TokenSource) error {
	if ts == nil {
		return errors.New("maestro: no TokenSource supplied, so the audience its tokens " +
			"carry cannot be checked; pass the service's auth client")
	}

	tok, err := ts.GetAuthToken(ctx)
	if err != nil {
		return errors.Wrap(err, "maestro: could not mint a token to check its audience")
	}
	if tok == nil || *tok == "" {
		return errors.New("maestro: token source returned an empty token")
	}

	auds, err := audiencesOf(*tok)
	if err != nil {
		return err
	}
	for _, a := range auds {
		if a == Audience {
			return nil
		}
	}

	got := "none"
	if len(auds) > 0 {
		got = strings.Join(auds, ", ")
	}
	return errors.Errorf("maestro: this service's tokens are minted for audience [%s], "+
		"but maestro enforces %q. Hydra's `audience` field is an ALLOW-LIST, not a "+
		"default: requesting an audience the client is not permitted returns a token "+
		"WITHOUT it rather than an error, so this would have surfaced as a 401 from "+
		"maestro instead. Fix: set LEARTECH_AUTH_TARGET_AUDIENCE=%s and add %s to the "+
		"client's allow-list in the auth-service provisioning table",
		got, Audience, Audience, Audience)
}

// audiencesOf reads the `aud` claim out of a JWT payload. `aud` is either a
// string or an array of strings (RFC 7519 4.1.3), and both shapes appear in
// practice — Hydra emits an array, hand-rolled fixtures often emit a string.
func audiencesOf(token string) ([]string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.Errorf("maestro: token is not a JWT (%d segments, want 3)", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.Wrap(err, "maestro: decode token payload")
	}

	var claims struct {
		Aud json.RawMessage `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.Wrap(err, "maestro: parse token claims")
	}
	if len(claims.Aud) == 0 {
		return nil, nil
	}

	var many []string
	if err := json.Unmarshal(claims.Aud, &many); err == nil {
		return many, nil
	}
	var one string
	if err := json.Unmarshal(claims.Aud, &one); err == nil {
		if one == "" {
			return nil, nil
		}
		return []string{one}, nil
	}
	return nil, errors.New("maestro: `aud` claim is neither a string nor an array of strings")
}
