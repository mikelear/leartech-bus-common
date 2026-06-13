package auth

const (
	AuthorizationHeaderKey = "Authorization"
	TokenClaimsKey         = "TokenClaims"
)

type HealthResponse struct {
	Status string `json:"status"`
}
