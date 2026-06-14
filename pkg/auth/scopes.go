package auth

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"slices"
	"strings"
)

type Scope string

// Scopes associated with a OAuth2 client
const (
	ScopeAPI              Scope = "leartechapi"
	ScopeInternalServices Scope = "leartechapi.internal_services"
)

type Scopes []Scope

func newScopesFromAny(scopesAny any) (Scopes, error) {
	// 'scope' should be of the format string "scope1 scope2 scope3"
	var scopes Scopes
	switch v := scopesAny.(type) {
	case string:
		for _, s := range strings.Fields(v) {
			scopes = append(scopes, Scope(s))
		}
	default:
		return Scopes{}, fmt.Errorf("invalid scopes type: %T", scopesAny)
	}
	return scopes, nil
}

func (s Scopes) HasInternalService() bool {
	return s.IsScoped(Scopes{ScopeInternalServices})
}

func (s Scopes) HasAPI() bool {
	return s.IsScoped(Scopes{ScopeAPI})
}

func (s Scopes) IsScoped(requiredScopes Scopes) bool {
	if len(s) == 0 {
		// There should always be at least one scope present to be considered scoped
		log.Debug().Msg("scopes is empty")
		return false
	}

	if len(requiredScopes) == 0 {
		// No required scopes, so always scoped
		return true
	}

	// Otherwise check if any of the required scopes are in the scopes array
	for _, haveScope := range s {
		if slices.Contains(requiredScopes, haveScope) {
			return true
		}
	}
	return false
}
