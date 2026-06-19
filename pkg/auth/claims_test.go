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
		// V6 ext claims (tenant_id / user_role / external_id) — additive
		{
			name: "V6ExtClaimsAllPresent",
			mapClaims: map[string]any{
				"sub":   "user-v6",
				"scope": "leartechapi",
				"ext": map[string]any{
					"Permissions": []any{"Admin"},
					"tenant_id":   "tenant-abc",
					"user_role":   "broker",
					"external_id": "ext-xyz",
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user-v6",
				Scopes:      auth.Scopes{"leartechapi"},
				Permissions: auth.Permissions{"Admin"},
				TenantID:    "tenant-abc",
				UserRole:    "broker",
				ExternalID:  "ext-xyz",
			},
			expectErr: false,
		},
		{
			name: "V6ExtClaimsPartial",
			mapClaims: map[string]any{
				"sub":   "user-v6-partial",
				"scope": "leartechapi",
				"ext": map[string]any{
					"Permissions": []any{"Admin"},
					"tenant_id":   "tenant-only",
					// user_role and external_id intentionally absent
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user-v6-partial",
				Scopes:      auth.Scopes{"leartechapi"},
				Permissions: auth.Permissions{"Admin"},
				TenantID:    "tenant-only",
				UserRole:    "",
				ExternalID:  "",
			},
			expectErr: false,
		},
		{
			name: "V6ExtClaimsAbsentLeavesFieldsEmpty",
			mapClaims: map[string]any{
				"sub":   "user-no-v6",
				"scope": "leartechapi",
				"ext": map[string]any{
					"Permissions": []any{"Admin"},
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user-no-v6",
				Scopes:      auth.Scopes{"leartechapi"},
				Permissions: auth.Permissions{"Admin"},
				TenantID:    "",
				UserRole:    "",
				ExternalID:  "",
			},
			expectErr: false,
		},
		{
			name: "V6ExtClaimsNonStringValuesIgnored",
			mapClaims: map[string]any{
				"sub":   "user-bad-types",
				"scope": "leartechapi",
				"ext": map[string]any{
					"Permissions": []any{"Admin"},
					"tenant_id":   42,               // not a string
					"user_role":   []any{"x"},       // not a string
					"external_id": map[string]any{}, // not a string
				},
			},
			expected: &auth.TokenClaims{
				UserID:      "user-bad-types",
				Scopes:      auth.Scopes{"leartechapi"},
				Permissions: auth.Permissions{"Admin"},
				TenantID:    "",
				UserRole:    "",
				ExternalID:  "",
			},
			expectErr: false,
		},
		{
			name: "V6ExtClaimsAlongsidePlatformPermissionsRegression",
			mapClaims: map[string]any{
				"sub":   "user-combined",
				"scope": "leartechapi",
				"ext": map[string]any{
					"Permissions":         []any{"Admin"},
					"PlatformPermissions": []any{"SetValues", "ViewBankDetails"},
					"tenant_id":           "tenant-x",
					"user_role":           "underwriter",
					"external_id":         "ext-99",
				},
			},
			expected: &auth.TokenClaims{
				UserID:              "user-combined",
				Scopes:              auth.Scopes{"leartechapi"},
				Permissions:         auth.Permissions{"Admin"},
				PlatformPermissions: auth.PlatformPermissions{auth.SetValues, auth.ViewBankDetails},
				TenantID:            "tenant-x",
				UserRole:            "underwriter",
				ExternalID:          "ext-99",
			},
			expectErr: false,
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
