package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// jwk mirrors the shape of a single JSON Web Key entry from a JWKS
// document. Only the fields we consume are decoded — unknown fields
// are ignored.
type jwk struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	Alg string   `json:"alg"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	Crv string   `json:"crv"`
	X   string   `json:"x"`
	Y   string   `json:"y"`
	X5c []string `json:"x5c"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

// keyCache stores parsed public keys keyed by `kid`. The cache is
// safe for concurrent readers and single-flight refreshers.
type keyCache struct {
	mu      sync.RWMutex
	keys    map[string]any
	fetched time.Time
}

// jwksClient fetches and caches keys from a JWKS URL. It refreshes on
// a timer AND on explicit demand (used for unknown-kid tokens).
type jwksClient struct {
	url        string
	httpClient *http.Client
	interval   time.Duration
	now        func() time.Time

	cache        *keyCache
	singleflight singleflightGroup
}

// newJWKSClient constructs a jwksClient. It performs an initial
// synchronous fetch so the caller sees an error at startup rather
// than at first request — this is part of the fail-closed contract.
func newJWKSClient(ctx context.Context, url string, hc *http.Client, interval time.Duration, now func() time.Time) (*jwksClient, error) {
	c := &jwksClient{
		url:        url,
		httpClient: hc,
		interval:   interval,
		now:        now,
		cache:      &keyCache{keys: map[string]any{}},
	}
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// keyForKID returns the parsed public key for the given `kid`. If the
// key is not in cache the client refreshes once — this handles key
// rotation between refresh ticks.
func (c *jwksClient) keyForKID(ctx context.Context, kid string) (any, error) {
	c.cache.mu.RLock()
	k, ok := c.cache.keys[kid]
	c.cache.mu.RUnlock()
	if ok {
		return k, nil
	}
	// Not found — force a refresh, then look again.
	if err := c.singleflight.do(func() error {
		return c.refresh(ctx)
	}); err != nil {
		return nil, err
	}
	c.cache.mu.RLock()
	k, ok = c.cache.keys[kid]
	c.cache.mu.RUnlock()
	if !ok {
		return nil, errors.Wrapf(ErrUnknownKID, "kid=%s", kid)
	}
	return k, nil
}

// refresh fetches the JWKS document and replaces the cache atomically
// on success. On any failure the previous cache is retained.
func (c *jwksClient) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return errors.Wrap(ErrJWKSFetch, err.Error())
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(ErrJWKSFetch, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.Wrapf(ErrJWKSFetch, "status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB safety cap
	if err != nil {
		return errors.Wrap(ErrJWKSFetch, err.Error())
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return errors.Wrap(ErrJWKSFetch, "decode: "+err.Error())
	}
	parsed := make(map[string]any, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			// Reject unkid'd keys — we cannot match them against
			// token headers unambiguously.
			continue
		}
		pk, err := parseJWK(k)
		if err != nil {
			// Individual key parse failures are non-fatal — the JWKS
			// may include keys we don't support. Continue with the
			// rest so a single bad entry doesn't take down the whole
			// document.
			continue
		}
		parsed[k.Kid] = pk
	}
	if len(parsed) == 0 {
		return errors.Wrap(ErrJWKSFetch, "no usable keys in JWKS document")
	}
	c.cache.mu.Lock()
	c.cache.keys = parsed
	c.cache.fetched = c.now()
	c.cache.mu.Unlock()
	return nil
}

// parseJWK converts one JWK entry into a crypto public key.
// Supports RSA (RS256/384/512) and EC (ES256/384/512), which cover
// every JWKS shape leartech providers emit.
func parseJWK(k jwk) (any, error) {
	// Prefer x5c when present — the leaf certificate is
	// unambiguous and skips manual n/e math.
	if len(k.X5c) > 0 {
		der, err := base64.StdEncoding.DecodeString(k.X5c[0])
		if err != nil {
			return nil, fmt.Errorf("decode x5c: %w", err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse x5c cert: %w", err)
		}
		return cert.PublicKey, nil
	}
	switch k.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, fmt.Errorf("decode RSA n: %w", err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, fmt.Errorf("decode RSA e: %w", err)
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		if e == 0 {
			return nil, errors.New("RSA exponent zero")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported EC curve: %s", k.Crv)
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, fmt.Errorf("decode EC x: %w", err)
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, fmt.Errorf("decode EC y: %w", err)
		}
		return &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kty: %s", k.Kty)
	}
}

// singleflightGroup is a tiny coalescing helper so multiple concurrent
// unknown-kid lookups only trigger one JWKS refresh. Kept in-package
// so we don't add a dep on x/sync/singleflight for four lines of code.
type singleflightGroup struct {
	mu       sync.Mutex
	inFlight *singleflightCall
}

type singleflightCall struct {
	done chan struct{}
	err  error
}

func (g *singleflightGroup) do(fn func() error) error {
	g.mu.Lock()
	if g.inFlight != nil {
		call := g.inFlight
		g.mu.Unlock()
		<-call.done
		return call.err
	}
	call := &singleflightCall{done: make(chan struct{})}
	g.inFlight = call
	g.mu.Unlock()

	call.err = fn()
	close(call.done)

	g.mu.Lock()
	g.inFlight = nil
	g.mu.Unlock()
	return call.err
}
