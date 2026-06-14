package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jarcoal/httpmock"
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const (
	baseAuthorisationURL = "https://hydra.testing.mqube.com"
	tokenURL             = baseAuthorisationURL + "/oauth2/token"
	healthURL            = baseAuthorisationURL + "/health/ready"
	jwksURL              = baseAuthorisationURL + "/.well-known/jwks.json"
)

var (
	mockConfig = auth.Config{
		DisableMiddleware: false,
		ServerURL:         baseAuthorisationURL,
		ClientID:          "SomeClientID",
		ClientSecret:      "SomeClientSecret",
	}

	mockTokenResponse = oauth2.Token{
		AccessToken: "SomeValidToken",
		TokenType:   "Bearer",
	}

	requiredPerms = auth.Permissions{auth.Admin, auth.Broker}
)

func TestAuthClient_Middleware(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	gin.SetMode(gin.TestMode)

	// Generate a test RSA key pair for signing/verifying JWTs
	keyPair := newTestKeyPair()

	testCases := []struct {
		name                string
		tokenClaims         *auth.TokenClaims // nil means use raw userToken instead
		userToken           string            // used when tokenClaims is nil (for malformed token tests)
		requiredPermissions auth.Permissions
		expectedStatusCode  int
	}{
		{
			name: "HappyPath",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{"Admin", "Broker", "BrokerSupport"},
				Scopes:      auth.Scopes{"leartechapi"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusOK,
		},
		{
			name:                "MalformedToken",
			userToken:           "DefinitelyNotTheCorrectSchema",
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusUnauthorized,
		},
		{
			name:                "MissingToken",
			userToken:           "",
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusUnauthorized,
		},
		{
			name:                "InvalidSignature",
			userToken:           "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3Qta2V5LWlkIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1c2VyLTEyMyIsImV4cCI6OTk5OTk5OTk5OSwiaWF0IjoxNjAwMDAwMDAwLCJzY3AiOlsibXF1YmVhcGkiXSwiZXh0Ijp7IlBlcm1pc3Npb25zIjpbIkFkbWluIl19fQ.invalid-signature",
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusUnauthorized,
		},
		{
			name: "EmptyPermissions",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{},
				Scopes:      auth.Scopes{"leartechapi"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusForbidden,
		},
		{
			name: "WithoutRequiredPermissions",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{"CaseRead", "CaseWrite", "Finance"},
				Scopes:      auth.Scopes{"leartechapi"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusForbidden,
		},
		{
			name: "InternalServicesScope",
			tokenClaims: &auth.TokenClaims{
				UserID:      "service-account",
				Permissions: auth.Permissions{}, // No permissions, but has internal services scope
				Scopes:      auth.Scopes{"leartechapi.internal_services"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusOK,
		},
		{
			name: "PermissionsAreNotRequired",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{"SomeOtherPermission"},
				Scopes:      auth.Scopes{"leartechapi"},
			},
			requiredPermissions: nil,
			expectedStatusCode:  http.StatusOK,
		},
		{
			name: "PartialPermissionMatch",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{"Broker"}, // Only one of Admin, Broker required
				Scopes:      auth.Scopes{"leartechapi"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusOK,
		},
		{
			name: "PermissionsMatchButMissingAPIServiceScope",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Permissions: auth.Permissions{"Admin", "Broker"},
				Scopes:      auth.Scopes{"some-other-scope"},
			},
			requiredPermissions: requiredPerms,
			expectedStatusCode:  http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Reset()
			registerMockTokenRequest(mockTokenResponse)
			registerMockJWKSRequest(keyPair.jwksResponse())

			// Determine the token to use
			userToken := tc.userToken
			if tc.tokenClaims != nil {
				userToken = "Bearer " + keyPair.signToken(*tc.tokenClaims)
			}

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			r := gin.Default()
			r.GET("/", setAuthorizationToken(userToken), client.Middleware(tc.requiredPermissions))

			testMiddlewareRequest(t, r, tc.expectedStatusCode)
		})
	}
}

// helper function for setting the authorisation header of a mocked incoming request
func setAuthorizationToken(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token != "" {
			c.Request.Header.Set("Authorization", token)
		}
	}
}

func registerMockTokenRequest(mockTokenResponse oauth2.Token) {
	data, err := json.Marshal(mockTokenResponse)
	if err != nil {
		panic(err)
	}

	httpmock.RegisterResponder(http.MethodPost, tokenURL,
		httpmock.NewBytesResponder(http.StatusOK, data))
}

func registerMockHealthRequest(mockHealth auth.HealthResponse, statusCode int) {
	data, err := json.Marshal(mockHealth)
	if err != nil {
		panic(err)
	}

	httpmock.RegisterResponder(http.MethodGet, healthURL,
		httpmock.NewBytesResponder(statusCode, data))
}

type jwks struct {
	Keys []map[string]interface{} `json:"keys"`
}

// testKeyPair holds an RSA key pair for test JWT signing/verification
type testKeyPair struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
}

// newTestKeyPair generates a new RSA key pair for testing
func newTestKeyPair() *testKeyPair {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return &testKeyPair{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		keyID:      "test-key-id",
	}
}

// jwksResponse returns the JWKS containing this key pair's public key
func (kp *testKeyPair) jwksResponse() jwks {
	return jwks{
		Keys: []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": kp.keyID,
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(kp.publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(kp.publicKey.E)).Bytes()),
			},
		},
	}
}

// signToken creates a signed JWT with the given claims
func (kp *testKeyPair) signToken(claims auth.TokenClaims) string {
	now := time.Now()
	scopesArray := make([]string, len(claims.Scopes))
	for i, s := range claims.Scopes {
		scopesArray[i] = string(s)
	}
	ext := map[string]interface{}{}
	if claims.Permissions != nil {
		ext["Permissions"] = claims.Permissions
	}
	if claims.PlatformPermissions != nil {
		ext["PlatformPermissions"] = claims.PlatformPermissions
	}
	jwtClaims := jwt.MapClaims{
		"sub":   claims.UserID,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"scope": strings.Join(scopesArray, " "),
		"ext":   ext,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = kp.keyID

	signedToken, err := token.SignedString(kp.privateKey)
	if err != nil {
		panic(err)
	}
	return signedToken
}

func registerMockJWKSRequest(mockJWKSResponse jwks) {
	data, err := json.Marshal(mockJWKSResponse)
	if err != nil {
		panic(err)
	}

	httpmock.RegisterResponder(http.MethodGet, jwksURL,
		httpmock.NewBytesResponder(http.StatusOK, data))
}

// registerEmptyJWKSRequest registers an empty JWKS response for tests that don't need token verification
func registerEmptyJWKSRequest() {
	registerMockJWKSRequest(jwks{Keys: []map[string]interface{}{}})
}

// Helper function for testing middleware responds with correct HTTP code
func testMiddlewareRequest(t *testing.T, r *gin.Engine, expectedHTTPCode int) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	testHTTPResponse(t, r, req, func(w *httptest.ResponseRecorder) bool {
		return w.Code == expectedHTTPCode
	})
}

// Helper function to process a request and test its response
func testHTTPResponse(t *testing.T, r *gin.Engine, req *http.Request, f func(w *httptest.ResponseRecorder) bool) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !f(w) {
		t.Fail()
	}
}

func TestAuthClient_GetAuthToken(t *testing.T) {
	testCases := []struct {
		name          string
		expectedToken string
	}{
		{
			name:          "HappyPath",
			expectedToken: "SomeValidToken",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerEmptyJWKSRequest()

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			mockToken := oauth2.Token{
				AccessToken: tc.expectedToken,
				TokenType:   "Bearer",
			}

			registerMockTokenRequest(mockToken)

			token, err := client.GetAuthToken(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expectedToken, *token)
		})
	}
}

func TestAuthClient_IsDisabled(t *testing.T) {
	testCases := []struct {
		name     string
		config   auth.Config
		expected bool
	}{
		{
			name: "DisabledMiddleware",
			config: auth.Config{
				DisableMiddleware: true,
				ServerURL:         baseAuthorisationURL,
			},
			expected: true,
		},
		{
			name: "EnabledMiddleware",
			config: auth.Config{
				DisableMiddleware: false,
				ServerURL:         baseAuthorisationURL,
			},
			expected: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerEmptyJWKSRequest()

			client, err := auth.NewServiceClient(t.Context(), tc.config)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, client.IsDisabled())
		})
	}
}

func TestAuthClient_Ping(t *testing.T) {
	testCases := []struct {
		name                     string
		healthResponse           auth.HealthResponse
		healthResponseStatusCode int
		expectError              bool
	}{
		{
			name:                     "SuccessfulPing",
			healthResponse:           auth.HealthResponse{Status: "ok"},
			healthResponseStatusCode: http.StatusOK,
			expectError:              false,
		},
		{
			name:                     "StatusNotOK",
			healthResponse:           auth.HealthResponse{Status: "ok"},
			healthResponseStatusCode: http.StatusInternalServerError,
			expectError:              true,
		},
		{
			name:                     "UnhealthyStatus",
			healthResponse:           auth.HealthResponse{Status: "unhealthy"},
			healthResponseStatusCode: http.StatusOK,
			expectError:              true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerEmptyJWKSRequest()

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			registerMockTokenRequest(mockTokenResponse)
			registerMockHealthRequest(tc.healthResponse, tc.healthResponseStatusCode)

			err = client.Ping(t.Context())
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthClient_SetAuthHeader(t *testing.T) {
	testCases := []struct {
		name           string
		token          string
		expectedHeader string
	}{
		{
			name:           "ValidToken",
			token:          "SomeValidToken",
			expectedHeader: "Bearer SomeValidToken",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerEmptyJWKSRequest()

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			token := oauth2.Token{
				AccessToken: tc.token,
				TokenType:   "Bearer",
			}
			registerMockTokenRequest(token)

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			err = client.SetAuthHeader(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedHeader, req.Header.Get(auth.AuthorizationHeaderKey))
		})
	}
}

func TestAuthClient_GetAuthToken_TokenIsCached(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	registerEmptyJWKSRequest()

	client, err := auth.NewServiceClient(t.Context(), mockConfig)
	require.NoError(t, err)

	registerMockTokenRequest(mockTokenResponse)

	for i := 0; i < 5; i++ {
		token, err := client.GetAuthToken(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "SomeValidToken", *token)
	}

	callCounts := httpmock.GetCallCountInfo()
	assert.Equal(t, 1, callCounts["POST "+tokenURL], "token endpoint should only be called once")
}

func TestAuthClient_PlatformMiddleware(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	gin.SetMode(gin.TestMode)

	keyPair := newTestKeyPair()

	requiredPlatformPerms := auth.PlatformPermissions{auth.SetValues, auth.ViewBankDetails}

	testCases := []struct {
		name                    string
		tokenClaims             *auth.TokenClaims
		userToken               string
		requiredPlatformPerms   auth.PlatformPermissions
		expectedStatusCode      int
	}{
		{
			name: "HappyPath",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"leartechapi"},
				PlatformPermissions: auth.PlatformPermissions{auth.SetValues, auth.ViewBankDetails, auth.InternalQA},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusOK,
		},
		{
			name: "PartialPlatformPermissionMatch",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"leartechapi"},
				PlatformPermissions: auth.PlatformPermissions{auth.ViewBankDetails},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusOK,
		},
		{
			name: "MissingPlatformPermissions",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"leartechapi"},
				PlatformPermissions: auth.PlatformPermissions{auth.InternalQA},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusForbidden,
		},
		{
			name: "EmptyPlatformPermissions",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"leartechapi"},
				PlatformPermissions: auth.PlatformPermissions{},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusForbidden,
		},
		{
			name: "InternalServiceBypassesPlatformPermissions",
			tokenClaims: &auth.TokenClaims{
				UserID:              "service-account",
				Scopes:              auth.Scopes{"leartechapi.internal_services"},
				PlatformPermissions: auth.PlatformPermissions{},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusOK,
		},
		{
			name: "NoPlatformPermissionsRequired",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"leartechapi"},
				PlatformPermissions: auth.PlatformPermissions{},
			},
			requiredPlatformPerms: nil,
			expectedStatusCode:    http.StatusOK,
		},
		{
			name: "HasPlatformPermissionsButWrongScope",
			tokenClaims: &auth.TokenClaims{
				UserID:              "user-123",
				Scopes:              auth.Scopes{"some-other-scope"},
				PlatformPermissions: auth.PlatformPermissions{auth.SetValues, auth.ViewBankDetails},
			},
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusForbidden,
		},
		{
			name:                  "MalformedToken",
			userToken:             "DefinitelyNotTheCorrectSchema",
			requiredPlatformPerms: requiredPlatformPerms,
			expectedStatusCode:    http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Reset()
			registerMockTokenRequest(mockTokenResponse)
			registerMockJWKSRequest(keyPair.jwksResponse())

			userToken := tc.userToken
			if tc.tokenClaims != nil {
				userToken = "Bearer " + keyPair.signToken(*tc.tokenClaims)
			}

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			r := gin.Default()
			r.GET("/", setAuthorizationToken(userToken), client.PlatformMiddleware(tc.requiredPlatformPerms))

			testMiddlewareRequest(t, r, tc.expectedStatusCode)
		})
	}
}

func TestAuthClient_GetRequestTokenFromGinContext(t *testing.T) {
	// Generate a test RSA key pair for signing/verifying JWTs
	keyPair := newTestKeyPair()

	testCases := []struct {
		name            string
		tokenClaims     *auth.TokenClaims
		contextModifier func(tokenClaims *auth.TokenClaims, gc *gin.Context) *gin.Context
		expectedErr     *error
	}{
		{
			name: "TokenClaimsInContext",
			tokenClaims: &auth.TokenClaims{
				UserID: "user-123",
			},
			contextModifier: func(tokenClaims *auth.TokenClaims, c *gin.Context) *gin.Context {
				c.Set(auth.TokenClaimsKey, tokenClaims)
				return c
			},
			expectedErr: nil,
		},
		{
			name: "DecodeTokenFromHeader",
			tokenClaims: &auth.TokenClaims{
				UserID:      "user-123",
				Scopes:      auth.Scopes{"leartechapi"},
				Permissions: auth.Permissions{"Admin"},
			},
			contextModifier: func(tokenClaims *auth.TokenClaims, c *gin.Context) *gin.Context {
				c.Request.Header.Set(auth.AuthorizationHeaderKey, "Bearer "+keyPair.signToken(*tokenClaims))
				return c
			},
			expectedErr: nil,
		},
		{
			name:        "MissingTokenFromHeader",
			tokenClaims: nil,
			contextModifier: func(tokenClaims *auth.TokenClaims, c *gin.Context) *gin.Context {
				// Don't set token in header or context
				return c
			},
			expectedErr: &auth.ErrAuthorizationHeaderMissing,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			registerMockJWKSRequest(keyPair.jwksResponse())

			client, err := auth.NewServiceClient(t.Context(), mockConfig)
			require.NoError(t, err)

			r := gin.Default()
			r.GET("/", func(c *gin.Context) {
				c = tc.contextModifier(tc.tokenClaims, c)
				claims, err := client.GetRequestTokenClaimsFromGinContext(c)
				if tc.expectedErr != nil {
					assert.ErrorIs(t, err, *tc.expectedErr)
					return
				}
				assert.Equal(t, tc.tokenClaims, claims)
			})

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
