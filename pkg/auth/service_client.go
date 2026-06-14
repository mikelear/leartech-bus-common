package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type ServiceClient struct {
	cfg         Config
	jwksKeyFunc keyfunc.Keyfunc
	tokenSource oauth2.TokenSource
	healthURL   string
	httpClient  *http.Client
}

func NewServiceClient(ctx context.Context, cfg Config) (*ServiceClient, error) {
	hydraBaseURL, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse server URL: %w", err)
	}

	// Setup client for OAuth2 Client Credentials flow (service-to-service auth)
	oauth2Config := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     hydraBaseURL.ResolveReference(&url.URL{Path: "/oauth2/token"}).String(),
		Scopes:       []string{string(ScopeInternalServices)},
		AuthStyle:    oauth2.AuthStyleInParams,
	}

	// Setup the JWKS decoder for verifying incoming tokens
	jwksKF, err := keyfunc.NewDefault([]string{hydraBaseURL.ResolveReference(&url.URL{Path: "/.well-known/jwks.json"}).String()})
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS keyfunc: %w", err)
	}

	httpClient := oauth2Config.Client(ctx)
	return &ServiceClient{
		cfg:         cfg,
		jwksKeyFunc: jwksKF,
		tokenSource: oauth2Config.TokenSource(ctx),
		httpClient:  httpClient,
		healthURL:   hydraBaseURL.ResolveReference(&url.URL{Path: "/health/ready"}).String(),
	}, nil
}

func (c *ServiceClient) Middleware(requiredPerms Permissions) gin.HandlerFunc {
	return func(gc *gin.Context) {
		if c.cfg.DisableMiddleware {
			gc.Next()
			return
		}

		tokenClaims, err := c.GetRequestTokenClaimsFromGinContext(gc)
		if err != nil {
			log.Debug().Err(err).Msg("failed to decode/verify the token")
			gc.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if c.isTokenAllowedAccess(requiredPerms, tokenClaims) {
			gc.Set(TokenClaimsKey, tokenClaims)
			gc.Next()
		} else {
			log.Debug().Msg("request's token is not authorised")
			gc.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func (c *ServiceClient) isTokenAllowedAccess(requiredPerms Permissions, tokenClaims *TokenClaims) bool {
	// Either the token is from an internal service, in which case we allow it access.
	// Or it's a User's token with the 'leartechapi' scope and the required permissions.
	return tokenClaims.Scopes.HasInternalService() || (tokenClaims.Scopes.HasAPI() && tokenClaims.Permissions.IsPermitted(requiredPerms))
}

func (c *ServiceClient) PlatformMiddleware(requiredPerms PlatformPermissions) gin.HandlerFunc {
	return func(gc *gin.Context) {
		if c.cfg.DisableMiddleware {
			gc.Next()
			return
		}

		tokenClaims, err := c.GetRequestTokenClaimsFromGinContext(gc)
		if err != nil {
			log.Debug().Err(err).Msg("failed to decode/verify the token")
			gc.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if c.isTokenAllowedPlatformAccess(requiredPerms, tokenClaims) {
			gc.Set(TokenClaimsKey, tokenClaims)
			gc.Next()
		} else {
			log.Debug().Msg("request's token does not have the required platform permissions")
			gc.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func (c *ServiceClient) isTokenAllowedPlatformAccess(required PlatformPermissions, tokenClaims *TokenClaims) bool {
	return tokenClaims.Scopes.HasInternalService() || (tokenClaims.Scopes.HasAPI() && tokenClaims.PlatformPermissions.IsPermitted(required))
}

func (c *ServiceClient) decodeToken(token string) (*TokenClaims, error) {
	jwtToken, err := jwt.Parse(token, c.jwksKeyFunc.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse/verify token: %w", err)
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return nil, errors.New("token is invalid")
	}
	tokenClaims, err := NewTokenClaimsFromMapClaims(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims from token: %w", err)
	}
	return tokenClaims, nil
}

func (c *ServiceClient) GetRequestTokenClaimsFromGinContext(gc *gin.Context) (*TokenClaims, error) {
	tokenClaimsAny, ok := gc.Get(TokenClaimsKey)
	if ok {
		// Previously run middleware may have already set the claims.
		// In that case, just return them.
		if tc, valid := tokenClaimsAny.(*TokenClaims); valid {
			return tc, nil
		}
	}

	// Otherwise, extract and decode the token from the header.
	token, err := c.getTokenFromHeader(gc.GetHeader(AuthorizationHeaderKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get token from header: %w", err)
	}

	tokenClaims, err := c.decodeToken(token)
	if err != nil {
		return nil, err
	}
	return tokenClaims, nil
}

func (c *ServiceClient) getTokenFromHeader(authHeader string) (string, error) {
	if len(authHeader) < 1 {
		return "", ErrAuthorizationHeaderMissing
	}

	split := strings.Split(authHeader, " ")
	if len(split) != 2 {
		return "", ErrAuthorizationHeaderMalformed
	}
	return strings.TrimSpace(split[1]), nil
}

func (c *ServiceClient) GetAuthToken(_ context.Context) (*string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("could not get token: %w", err)
	}
	return &token.AccessToken, nil
}

func (c *ServiceClient) SetAuthHeader(ctx context.Context, req *http.Request) error {
	token, err := c.GetAuthToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}
	if token == nil {
		return errors.New("auth token is nil")
	}
	req.Header.Set(AuthorizationHeaderKey, "Bearer "+*token)
	return nil
}

func (c *ServiceClient) IsDisabled() bool {
	return c.cfg.DisableMiddleware
}

func (c *ServiceClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform ping request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping request returned non-OK status: %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode ping response: %w", err)
	}
	if health.Status != "ok" {
		return fmt.Errorf("ping response status not ok: %s", health.Status)
	}
	return nil
}
