package auth_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPermissions_IsPermitted(t *testing.T) {
	testCases := []struct {
		name                string
		currentPermissions  auth.Permissions
		requiredPermissions auth.Permissions
		expectedIsPermitted bool
	}{
		{
			name:                "NoPermissionsAtAll",
			currentPermissions:  nil,
			requiredPermissions: nil,
			expectedIsPermitted: true,
		},
		{
			name:                "EmptyCurrentButWithRequired",
			currentPermissions:  []auth.Permission{},
			requiredPermissions: []auth.Permission{"Applicant"},
			expectedIsPermitted: false,
		},
		{
			name:                "NilCurrentButWithRequired",
			currentPermissions:  nil,
			requiredPermissions: []auth.Permission{"Applicant"},
			expectedIsPermitted: false,
		},
		{
			name:                "WithCurrentButEmptyRequired",
			currentPermissions:  []auth.Permission{"Applicant"},
			requiredPermissions: []auth.Permission{},
			expectedIsPermitted: true,
		},
		{
			name:                "WithCurrentButNilRequired",
			currentPermissions:  []auth.Permission{"Applicant"},
			requiredPermissions: nil,
			expectedIsPermitted: true,
		},
		{
			name:                "WithCurrentButNoneRequired",
			currentPermissions:  []auth.Permission{"Applicant"},
			requiredPermissions: []auth.Permission{"Broker"},
			expectedIsPermitted: false,
		},
		{
			name:                "WithCurrentButSomeRequired",
			currentPermissions:  []auth.Permission{"Applicant"},
			requiredPermissions: []auth.Permission{"Broker", "Applicant"},
			expectedIsPermitted: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.currentPermissions.IsPermitted(tc.requiredPermissions)
			assert.Equal(t, tc.expectedIsPermitted, actual)
		})
	}
}

func TestPlatformPermissions_IsPermitted(t *testing.T) {
	testCases := []struct {
		name                string
		current             auth.PlatformPermissions
		required            auth.PlatformPermissions
		expectedIsPermitted bool
	}{
		{
			name:                "NoPermissionsAtAll",
			current:             nil,
			required:            nil,
			expectedIsPermitted: true,
		},
		{
			name:                "EmptyCurrentButWithRequired",
			current:             []auth.PlatformPermission{},
			required:            []auth.PlatformPermission{auth.SetValues},
			expectedIsPermitted: false,
		},
		{
			name:                "NilCurrentButWithRequired",
			current:             nil,
			required:            []auth.PlatformPermission{auth.SetValues},
			expectedIsPermitted: false,
		},
		{
			name:                "WithCurrentButEmptyRequired",
			current:             []auth.PlatformPermission{auth.SetValues},
			required:            []auth.PlatformPermission{},
			expectedIsPermitted: true,
		},
		{
			name:                "WithCurrentButNoneMatch",
			current:             []auth.PlatformPermission{auth.SetValues},
			required:            []auth.PlatformPermission{auth.ViewBankDetails},
			expectedIsPermitted: false,
		},
		{
			name:                "WithCurrentAndOneMatches",
			current:             []auth.PlatformPermission{auth.SetValues},
			required:            []auth.PlatformPermission{auth.ViewBankDetails, auth.SetValues},
			expectedIsPermitted: true,
		},
		{
			name:                "MultipleCurrentAndOneMatches",
			current:             []auth.PlatformPermission{auth.InternalQA, auth.ViewCaseServicing},
			required:            []auth.PlatformPermission{auth.ViewCaseServicing},
			expectedIsPermitted: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.current.IsPermitted(tc.required)
			assert.Equal(t, tc.expectedIsPermitted, actual)
		})
	}
}
