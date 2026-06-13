package auth_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewTokenClaimsFromMapClaims(t *testing.T) {
	testCases := []struct {
		name      string
		mapClaims map[string]any
		expected  *auth.TokenClaims
		expectErr bool
	}{
		{
			name: "HappyPath",
			mapClaims: map[string]any{
				"sub":   "user123",
				"scope": "scope1 scope2",
				"ext": map[string]any{
					"Permissions": []any{"Permission1", "Permission2"},
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user123",
				Scopes:      auth.Scopes{"scope1", "scope2"},
				Permissions: auth.Permissions{"Permission1", "Permission2"},
			},
			expectErr: false,
		},
		{
			name: "WithPlatformPermissions",
			mapClaims: map[string]any{
				"sub":   "user123",
				"scope": "scope1 scope2",
				"ext": map[string]any{
					"Permissions":         []any{"Permission1"},
					"PlatformPermissions": []any{"SetValues", "ViewBankDetails"},
				},
			},
			expected: &auth.TokenClaims{
				UserID:              "user123",
				Scopes:              auth.Scopes{"scope1", "scope2"},
				Permissions:         auth.Permissions{"Permission1"},
				PlatformPermissions: auth.PlatformPermissions{auth.SetValues, auth.ViewBankDetails},
			},
			expectErr: false,
		},
		{
			name: "PlatformPermissionsOnlyNoPlatformPermissions",
			mapClaims: map[string]any{
				"sub":   "user123",
				"scope": "scope1",
				"ext": map[string]any{
					"Permissions": []any{"Permission1"},
				},
			},
			expected: &auth.TokenClaims{
				UserID:              "user123",
				Scopes:              auth.Scopes{"scope1"},
				Permissions:         auth.Permissions{"Permission1"},
				PlatformPermissions: nil,
			},
			expectErr: false,
		},
		{
			name: "MissingPermissions",
			mapClaims: map[string]any{
				"sub":   "user789",
				"scope": "scopeA scopeB",
			},
			expected: &auth.TokenClaims{
				UserID:      "user789",
				Scopes:      auth.Scopes{"scopeA", "scopeB"},
				Permissions: nil,
			},
			expectErr: false,
		},
		{
			name: "MissingScopes",
			mapClaims: map[string]any{
				"sub": "user321",
				"ext": map[string]any{
					"Permissions": []any{"PermissionX", "PermissionY"},
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user321",
				Scopes:      nil,
				Permissions: auth.Permissions{"PermissionX", "PermissionY"},
			},
			expectErr: true,
		},
		{
			name: "MissingScopesAndPermissions",
			mapClaims: map[string]any{
				"sub": "user456",
			},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTokenClaims, err := auth.NewTokenClaimsFromMapClaims(tc.mapClaims)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actualTokenClaims)
		})
	}
}
