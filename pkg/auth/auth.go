package auth

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

type TokenGetter interface {
	// GetAuthToken returns the service's current token, or generates a new one if it is not set or has expired
	GetAuthToken(ctx context.Context) (*string, error)
	// SetAuthHeader attaches the service's current token to the given request as an Authorization header
	SetAuthHeader(ctx context.Context, req *http.Request) error
}

type ServiceAuthClient interface {
	TokenGetter
	// IsDisabled returns true if the auth httpClient/middleware is disabled
	IsDisabled() bool
	// Middleware checks whether the incoming request is correctly authorised with the relevant permissions
	Middleware(requiredPerms Permissions) gin.HandlerFunc
	// PlatformMiddleware checks whether the incoming request has the required platform permissions
	PlatformMiddleware(requiredPerms PlatformPermissions) gin.HandlerFunc
	// GetRequestTokenClaimsFromGinContext returns the callers token claims from either the gin context directly or by decoding the token from the Authorization header.
	GetRequestTokenClaimsFromGinContext(gc *gin.Context) (*TokenClaims, error)
	// Ping checks that the authorization server is up
	Ping(ctx context.Context) error
}
