//go:build unit

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupRouter wires an /echo endpoint behind auth.Middleware plus
// optional scope-required middlewares. The /echo handler reads
// claims off the context and returns the subject in the body so
// tests can assert on the propagated identity.
func setupRouter(t *testing.T, v auth.Verifier, scopes ...string) *gin.Engine {
	t.Helper()
	r := gin.New()
	group := r.Group("/api", auth.Middleware(v))
	if len(scopes) > 0 {
		group.Use(auth.ScopeRequired(scopes...))
	}
	group.GET("/echo", func(c *gin.Context) {
		cl, ok := auth.FromContext(c.Request.Context())
		if !ok {
			// Should be impossible — the middleware would have
			// aborted before reaching here.
			t.Errorf("handler reached without claims in context")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"sub": cl.Subject})
	})
	return r
}

// doRequest is a tiny helper that sends a request and returns the
// response recorder for assertion. It centralises the boilerplate so
// each test reads as intent, not plumbing.
func doRequest(t *testing.T, r *gin.Engine, method, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestMiddleware_ValidTokenAllows(t *testing.T) {
	v, s := newVerifier(t)
	r := setupRouter(t, v)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "sub": "user-99"})
	w := doRequest(t, r, http.MethodGet, "/api/echo", tok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"sub":"user-99"`)
}

func TestMiddleware_MissingHeaderReturns401(t *testing.T) {
	v, _ := newVerifier(t)
	r := setupRouter(t, v)
	w := doRequest(t, r, http.MethodGet, "/api/echo", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_MalformedHeaderReturns401(t *testing.T) {
	v, _ := newVerifier(t)
	r := setupRouter(t, v)
	// Header present but not "Bearer <token>".
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/echo", nil)
	req.Header.Set("Authorization", "Basic Zm9vOmJhcg==")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_WrongAudienceReturns401(t *testing.T) {
	v, s := newVerifier(t)
	r := setupRouter(t, v)
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-b"})
	w := doRequest(t, r, http.MethodGet, "/api/echo", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_MissingAudienceReturns401(t *testing.T) {
	v, s := newVerifier(t)
	r := setupRouter(t, v)
	tok := s.sign(t, jwt.MapClaims{})
	w := doRequest(t, r, http.MethodGet, "/api/echo", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_ScopeRequiredAllowsWithMatchingScope(t *testing.T) {
	v, s := newVerifier(t)
	r := setupRouter(t, v, "events.publish")
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "scope": "events.publish events.subscribe"})
	w := doRequest(t, r, http.MethodGet, "/api/echo", tok)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_ScopeRequiredRejectsWithoutScope(t *testing.T) {
	v, s := newVerifier(t)
	r := setupRouter(t, v, "events.publish")
	tok := s.sign(t, jwt.MapClaims{"aud": "svc-a", "scope": "events.subscribe"})
	w := doRequest(t, r, http.MethodGet, "/api/echo", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_PanicsOnNilVerifier(t *testing.T) {
	// The one behaviour the middleware factory MUST protect: a nil
	// verifier is a wiring bug that would silently pass every
	// request. Refuse to construct.
	assert.Panics(t, func() {
		auth.Middleware(nil)
	})
}

func TestFromContext_ReturnsFalseWhenAbsent(t *testing.T) {
	// Isolated context (no gin) — must return false, never panic.
	_, ok := auth.FromContext(context.Background())
	assert.False(t, ok)
}

func TestWithClaims_PropagatesThroughPlainContext(t *testing.T) {
	// Background workers spawned off a request should be able to
	// carry claims via WithClaims + FromContext without needing a
	// gin.Context.
	cl := &auth.Claims{Subject: "worker-caller"}
	ctx := auth.WithClaims(context.Background(), cl)
	got, ok := auth.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, "worker-caller", got.Subject)
}
