package auth

import (
	"fmt"
	"slices"
)

type Permission string

// Permissions associated with a user
const (
	Applicant           Permission = "Applicant"
	Broker              Permission = "Broker"
	ActivitiesUser      Permission = "ActivitiesUser"
	BrokerSupport       Permission = "BrokerSupport"
	Fallback            Permission = "Fallback"
	Finance             Permission = "Finance"
	Admin               Permission = "Admin"
	CaseRead            Permission = "CaseRead"
	CaseWrite           Permission = "CaseWrite"
	BrokerManagement    Permission = "BrokerManagement"
	SolicitorManagement Permission = "SolicitorManagement"
	Underwriter         Permission = "Underwriter"
	Solicitor           Permission = "Solicitor"
	UserManagement      Permission = "UserManagement"
	ApplicantManagement Permission = "ApplicantManagement"
	InTransition        Permission = "InTransition"
	Completions         Permission = "Completions"
	Customer            Permission = "Customer"
	ContractManagement  Permission = "ContractManagement"
	BranchAdvisor       Permission = "BranchAdvisor"
)

type Permissions []Permission

type PlatformPermission string

const (
	SetValues               PlatformPermission = "SetValues"
	UnlockCases             PlatformPermission = "UnlockCases"
	EditApplicants          PlatformPermission = "EditApplicants"
	EditDocuments           PlatformPermission = "EditDocuments"
	ManageSpecialConditions PlatformPermission = "ManageSpecialConditions"
	LockAnyCase             PlatformPermission = "LockAnyCase"
	ViewBankDetails         PlatformPermission = "ViewBankDetails"
	EditQuestions           PlatformPermission = "EditQuestions"
	PerformAdminTasks       PlatformPermission = "PerformAdminTasks"
	EditSolicitors          PlatformPermission = "EditSolicitors"
	CertifyRules            PlatformPermission = "CertifyRules"
	Workshop                PlatformPermission = "Workshop"
	AssignLeadUnderwriters  PlatformPermission = "AssignLeadUnderwriters"
	SetDocumentViewed       PlatformPermission = "SetDocumentViewed"
	OverrideRestrictedRules PlatformPermission = "OverrideRestrictedRules"
	PerformManagementTasks  PlatformPermission = "PerformManagementTasks"
	InternalQA              PlatformPermission = "InternalQA"
	ViewCaseServicing       PlatformPermission = "ViewCaseServicing"
)

type PlatformPermissions []PlatformPermission

func newPlatformPermissionsFromAny(permsAny any) (PlatformPermissions, error) {
	var permissions PlatformPermissions
	switch v := permsAny.(type) {
	case string:
		permissions = append(permissions, PlatformPermission(v))
	case []string:
		for _, s := range v {
			permissions = append(permissions, PlatformPermission(s))
		}
	case []any:
		for _, s := range v {
			if str, ok := s.(string); ok {
				permissions = append(permissions, PlatformPermission(str))
			} else {
				return PlatformPermissions{}, fmt.Errorf("invalid platform permission type: %T", s)
			}
		}
	default:
		return PlatformPermissions{}, fmt.Errorf("invalid platform permissions type: %T", permsAny)
	}
	return permissions, nil
}

// IsPermitted checks if p has at least one of the required PlatformPermissions
func (p PlatformPermissions) IsPermitted(required PlatformPermissions) bool {
	if len(required) < 1 {
		return true
	}
	if len(p) == 0 {
		return false
	}
	for _, have := range p {
		if slices.Contains(required, have) {
			return true
		}
	}
	return false
}

func newPermissionsFromAny(permsAny any) (Permissions, error) {
	var permissions Permissions
	switch v := permsAny.(type) {
	case string:
		permissions = append(permissions, Permission(v))
	case []string:
		for _, s := range v {
			permissions = append(permissions, Permission(s))
		}
	case []any:
		for _, s := range v {
			if str, ok := s.(string); ok {
				permissions = append(permissions, Permission(str))
			} else {
				return Permissions{}, fmt.Errorf("invalid permission type: %T", s)
			}
		}
	default:
		return Permissions{}, fmt.Errorf("invalid permissions type: %T", permsAny)
	}
	return permissions, nil
}

// IsPermitted checks if p has at least one of the required Permissions
func (p Permissions) IsPermitted(requiredPermissions Permissions) bool {
	if len(requiredPermissions) < 1 {
		// No required permissions, so always permitted
		return true
	}

	if len(p) == 0 {
		// No permissions, so not permitted
		return false
	}

	// Otherwise check if any of the required permissions are in the permissions array
	for _, havePerm := range p {
		if slices.Contains(requiredPermissions, havePerm) {
			return true
		}
	}
	return false
}
