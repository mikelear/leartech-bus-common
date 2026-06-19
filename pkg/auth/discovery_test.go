package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jarcoal/httpmock"
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResourceMetadataHandler exercises the RFC 9728 §3 metadata document
// returned by the discovery helper.
func TestResourceMetadataHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name               string
		cfg                auth.Config
		expectedStatusCode int
		expectedBody       *auth.ProtectedResourceMetadata
	}{
		{
			name: "HappyPath",
			cfg: auth.Config{
				Resource:             "https://api.example.com",
				AuthorizationServers: []string{"https://hydra.example.com"},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody: &auth.ProtectedResourceMetadata{
				Resource:             "https://api.example.com",
				AuthorizationServers: []string{"https://hydra.example.com"},
			},
		},
		{
			name: "MultipleAuthorizationServers",
			cfg: auth.Config{
				Resource: "https://api.example.com",
				AuthorizationServers: []string{
					"https://hydra-primary.example.com",
					"https://hydra-backup.example.com",
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody: &auth.ProtectedResourceMetadata{
				Resource: "https://api.example.com",
				AuthorizationServers: []string{
					"https://hydra-primary.example.com",
					"https://hydra-backup.example.com",
				},
			},
		},
		{
			name:               "DiscoveryDisabledWhenResourceEmpty",
			cfg:                auth.Config{AuthorizationServers: []string{"https://hydra.example.com"}},
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "DiscoveryDisabledWhenAuthorizationServersEmpty",
			cfg:                auth.Config{Resource: "https://api.example.com"},
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "DiscoveryDisabledWhenBothEmpty",
			cfg:                auth.Config{},
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.Default()
			r.GET(auth.ProtectedResourceMetadataPath, auth.ResourceMetadataHandler(tc.cfg))

			req, _ := http.NewRequest(http.MethodGet, auth.ProtectedResourceMetadataPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatusCode, w.Code)

			if tc.expectedStatusCode != http.StatusOK {
				return
			}

			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			var body auth.ProtectedResourceMetadata
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, *tc.expectedBody, body)

			// Spot-check the raw JSON shape — the wire field names matter
			// (RFC 9728 mandates lowercase snake_case).
			var raw map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
			assert.Contains(t, raw, "resource")
			assert.Contains(t, raw, "authorization_servers")
		})
	}
}

// TestNewProtectedResourceMetadata_DefensiveCopy ensures the returned struct
// does not alias the caller's config slice — mutating the result must not
// reach back into Config.AuthorizationServers.
func TestNewProtectedResourceMetadata_DefensiveCopy(t *testing.T) {
	cfg := auth.Config{
		Resource:             "https://api.example.com",
		AuthorizationServers: []string{"https://hydra.example.com"},
	}

	md := auth.NewProtectedResourceMetadata(cfg)
	require.NotNil(t, md)

	md.AuthorizationServers[0] = "MUTATED"

	assert.Equal(t, "https://hydra.example.com", cfg.AuthorizationServers[0],
		"mutating the returned metadata must not bleed into the source config")
}

// TestMiddleware_WWWAuthenticateHint covers the gated RFC 9728 §5.1
// WWW-Authenticate hint on 401 responses. Existing consumers' 401s MUST be
// unchanged when ResourceMetadataURL is not configured.
func TestMiddleware_WWWAuthenticateHint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const metadataURL = "https://api.example.com/.well-known/oauth-protected-resource"

	testCases := []struct {
		name                       string
		cfg                        auth.Config
		expectedWWWAuthenticateHdr string
	}{
		{
			name: "HintEmittedWhenConfigured",
			cfg: auth.Config{
				ServerURL:           baseAuthorisationURL,
				ClientID:            "SomeClientID",
				ClientSecret:        "SomeClientSecret",
				ResourceMetadataURL: metadataURL,
			},
			expectedWWWAuthenticateHdr: `Bearer resource_metadata="` + metadataURL + `"`,
		},
		{
			name: "HintAbsentWhenNotConfiguredBackwardCompat",
			cfg: auth.Config{
				ServerURL:    baseAuthorisationURL,
				ClientID:     "SomeClientID",
				ClientSecret: "SomeClientSecret",
				// ResourceMetadataURL intentionally empty — existing consumers'
				// 401s must be byte-for-byte unchanged.
			},
			expectedWWWAuthenticateHdr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerMockTokenRequest(mockTokenResponse)
			registerEmptyJWKSRequest()

			client, err := auth.NewServiceClient(t.Context(), tc.cfg)
			require.NoError(t, err)

			r := gin.Default()
			// No Authorization header → triggers the 401 path in
			// GetRequestTokenClaimsFromGinContext.
			r.GET("/protected", client.Middleware(auth.Permissions{auth.Admin}))

			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, tc.expectedWWWAuthenticateHdr, w.Header().Get("WWW-Authenticate"))
		})
	}
}

// TestPlatformMiddleware_WWWAuthenticateHint mirrors the assertion for the
// platform-permissions middleware — both 401 paths must honour the gate.
func TestPlatformMiddleware_WWWAuthenticateHint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const metadataURL = "https://api.example.com/.well-known/oauth-protected-resource"

	cfg := auth.Config{
		ServerURL:           baseAuthorisationURL,
		ClientID:            "SomeClientID",
		ClientSecret:        "SomeClientSecret",
		ResourceMetadataURL: metadataURL,
	}

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	registerMockTokenRequest(mockTokenResponse)
	registerEmptyJWKSRequest()

	client, err := auth.NewServiceClient(t.Context(), cfg)
	require.NoError(t, err)

	r := gin.Default()
	r.GET("/platform", client.PlatformMiddleware(auth.PlatformPermissions{auth.SetValues}))

	req, _ := http.NewRequest(http.MethodGet, "/platform", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t,
		`Bearer resource_metadata="`+metadataURL+`"`,
		w.Header().Get("WWW-Authenticate"),
	)
}
