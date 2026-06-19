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
	// TenantID is the optional V6 tenant identifier extracted from ext.tenant_id.
	// Empty when the claim is absent (backward-compatible with V5 tokens).
	TenantID string
	// UserRole is the optional V6 user role extracted from ext.user_role.
	// Empty when the claim is absent (backward-compatible with V5 tokens).
	UserRole string
	// ExternalID is the optional V6 external identifier extracted from ext.external_id.
	// Empty when the claim is absent (backward-compatible with V5 tokens).
	ExternalID string
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

	// V6 ext claims (tenant_id / user_role / external_id) are optional and additive.
	// Absent values produce empty strings — no error — preserving backward compatibility
	// with V5 tokens that don't set these fields. See the auth-service consent handler
	// for the canonical key names.
	tenantID := extractExtStringClaim(mc, "tenant_id")
	userRole := extractExtStringClaim(mc, "user_role")
	externalID := extractExtStringClaim(mc, "external_id")

	return &TokenClaims{
		UserID:              userID,
		Permissions:         permissions,
		PlatformPermissions: platformPermissions,
		Scopes:              scopes,
		TenantID:            tenantID,
		UserRole:            userRole,
		ExternalID:          externalID,
	}, nil
}

// extractExtStringClaim returns the string value of mc["ext"][key], or "" when:
//   - the ext claim is absent
//   - the ext claim is not a map
//   - the key is absent from the ext map
//   - the value is not a string
//
// This is the canonical pattern for optional V6 ext claims and mirrors the
// extractPermissionsFromClaims / extractPlatformPermissionsFromClaims shape.
func extractExtStringClaim(mc jwt.MapClaims, key string) string {
	ext, ok := mc["ext"]
	if !ok {
		return ""
	}

	extMap, ok := ext.(map[string]any)
	if !ok {
		return ""
	}

	valAny, ok := extMap[key]
	if !ok {
		return ""
	}

	val, ok := valAny.(string)
	if !ok {
		return ""
	}

	return val
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
