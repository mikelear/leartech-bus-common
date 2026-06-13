package auth_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestScopes_IsScoped(t *testing.T) {
	testCases := []struct {
		name             string
		currentScopes    auth.Scopes
		requiredScopes   auth.Scopes
		expectedIsScoped bool
	}{
		{
			name:             "NoScopesAtAll",
			currentScopes:    nil,
			requiredScopes:   nil,
			expectedIsScoped: false,
		},
		{
			name:             "EmptyCurrentButWithRequired",
			currentScopes:    auth.Scopes{},
			requiredScopes:   auth.Scopes{auth.ScopeAPI},
			expectedIsScoped: false,
		},
		{
			name:             "NilCurrentButWithRequired",
			currentScopes:    nil,
			requiredScopes:   auth.Scopes{auth.ScopeAPI},
			expectedIsScoped: false,
		},
		{
			name:             "WithCurrentButEmptyRequired",
			currentScopes:    auth.Scopes{auth.ScopeAPI},
			requiredScopes:   auth.Scopes{},
			expectedIsScoped: true,
		},
		{
			name:             "WithCurrentButNilRequired",
			currentScopes:    auth.Scopes{auth.ScopeAPI},
			requiredScopes:   nil,
			expectedIsScoped: true,
		},
		{
			name:             "WithCurrentButNoneRequired",
			currentScopes:    auth.Scopes{auth.ScopeAPI},
			requiredScopes:   auth.Scopes{auth.ScopeInternalServices},
			expectedIsScoped: false,
		},
		{
			name:             "WithCurrentButSomeRequired",
			currentScopes:    auth.Scopes{auth.ScopeAPI, auth.ScopeInternalServices},
			requiredScopes:   auth.Scopes{auth.ScopeInternalServices},
			expectedIsScoped: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.currentScopes.IsScoped(tc.requiredScopes)
			assert.Equal(t, tc.expectedIsScoped, actual)
		})
	}
}

func TestScopes_IsInternalService(t *testing.T) {
	testCases := []struct {
		name               string
		currentScopes      auth.Scopes
		expectedIsInternal bool
	}{
		{
			name:               "NoScopesAtAll",
			currentScopes:      nil,
			expectedIsInternal: false,
		},
		{
			name:               "EmptyCurrentScopes",
			currentScopes:      auth.Scopes{},
			expectedIsInternal: false,
		},
		{
			name:               "WithNonInternalScope",
			currentScopes:      auth.Scopes{auth.ScopeAPI},
			expectedIsInternal: false,
		},
		{
			name:               "WithInternalScope",
			currentScopes:      auth.Scopes{auth.ScopeInternalServices},
			expectedIsInternal: true,
		},
		{
			name:               "WithMultipleScopesIncludingInternal",
			currentScopes:      auth.Scopes{auth.ScopeAPI, auth.ScopeInternalServices},
			expectedIsInternal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.currentScopes.HasInternalService()
			assert.Equal(t, tc.expectedIsInternal, actual)
		})
	}
}
