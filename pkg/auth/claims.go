package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// TokenClaims represents the claims extracted from an OAuth2 token.
// This is not exhaustive and only includes the fields we care about.
type TokenClaims struct {
	UserID              string
	Permissions         Permissions
	PlatformPermissions PlatformPermissions
	Scopes              Scopes
}

func NewTokenClaimsFromMapClaims(mc jwt.MapClaims) (claims *TokenClaims, err error) {
	var userID string
	subAny, subjectExists := mc["sub"]
	if subjectExists {
		subjectStr, subjectStrParsed := subAny.(string)
		if subjectStrParsed {
			userID = subjectStr
		}
	}
	if userID == "" {
		// 'sub' claim should always be present in a valid token. If it's missing something is wrong.
		return nil, errors.New("token is missing 'sub' claim")
	}

	var scopes Scopes
	scopeAny, scopeExists := mc["scope"]
	if scopeExists {
		scopes, err = newScopesFromAny(scopeAny)
		if err != nil {
			return nil, fmt.Errorf("failed to parse scopes claim 'scope': %w", err)
		}
	}
	if len(scopes) == 0 {
		// Scopes should always be present in a valid token. If it's missing something is wrong.
		return nil, errors.New("token is missing 'scope' claim")
	}

	permissions, err := extractPermissionsFromClaims(mc)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		log.Debug().Msg("No permissions found in claims")
	}

	platformPermissions, err := extractPlatformPermissionsFromClaims(mc)
	if err != nil {
		return nil, err
	}
	if len(platformPermissions) == 0 {
		log.Debug().Msg("No platform permissions found in claims")
	}

	return &TokenClaims{
		UserID:              userID,
		Permissions:         permissions,
		PlatformPermissions: platformPermissions,
		Scopes:              scopes,
	}, nil
}

// extractPlatformPermissionsFromClaims extracts platform permissions from the "ext.PlatformPermissions" claim.
func extractPlatformPermissionsFromClaims(mc jwt.MapClaims) (PlatformPermissions, error) {
	ext, ok := mc["ext"]
	if !ok {
		return nil, nil
	}

	extMap, ok := ext.(map[string]any)
	if !ok {
		return nil, nil
	}

	permsAny, ok := extMap["PlatformPermissions"]
	if !ok {
		return nil, nil
	}

	platformPermissions, err := newPlatformPermissionsFromAny(permsAny)
	if err != nil {
		return nil, fmt.Errorf("failed to parse permissions from claim 'ext.PlatformPermissions': %w", err)
	}

	return platformPermissions, nil
}

// extractPermissionsFromClaims extracts permissions from the "ext.Permissions" claim.
func extractPermissionsFromClaims(mc jwt.MapClaims) (Permissions, error) {
	ext, ok := mc["ext"]
	if !ok {
		return nil, nil
	}

	extMap, ok := ext.(map[string]any)
	if !ok {
		return nil, nil
	}

	permsAny, ok := extMap["Permissions"]
	if !ok {
		return nil, nil
	}

	permissions, err := newPermissionsFromAny(permsAny)
	if err != nil {
		return nil, fmt.Errorf("failed to parse permissions from claim 'ext.Permissions': %w", err)
	}

	return permissions, nil
}
