package unit

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tiny-analytics/internal/auth"
)

func TestHMACVerify(t *testing.T) {
	secret := "super-secret"
	body := []byte(`{"hello":"world"}`)

	sig := auth.ComputeSignature(secret, body)
	require.True(t, auth.VerifySignature(secret, body, sig))
	require.False(t, auth.VerifySignature(secret, body, "deadbeef"))
}
