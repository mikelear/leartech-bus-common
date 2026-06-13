package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrAuthorizationHeaderMissing is returned when the authorization header is missing from the request.
	ErrAuthorizationHeaderMissing = errors.New("authorization header missing")
	// ErrAuthorizationHeaderMalformed is returned when the authorization header is present but does not conform to the expected format (e.g., "Bearer <token>").
	ErrAuthorizationHeaderMalformed = errors.New("authorization header is malformed")
	// ErrTokenExpired is returned when a token is valid but has expired. Mapping from jwt package for ease of use.
	ErrTokenExpired = jwt.ErrTokenExpired
)
