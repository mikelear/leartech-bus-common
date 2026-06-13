//go:build localtest

package auth

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestNewServiceClient_LocalTesting is a local test to verify decoding a token with the ServiceClient
func TestNewServiceClient_LocalTesting(t *testing.T) {
	client, err := NewServiceClient(t.Context(), Config{
		DisableMiddleware: false,
		ServerURL:         "InsertYourHydraServerURLHere",
		ClientID:          "InsertYourClientIDHere",
		ClientSecret:      "InsertYourClientSecretHere",
	})
	require.NoError(t, err)

	token := "InsertYourJWTTokenHere"
	claims, err := client.decodeToken(token)
	require.NoError(t, err)
	fmt.Println(claims)
}
