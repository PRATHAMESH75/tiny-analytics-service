package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeSignature returns the lowercase hex encoded HMAC-SHA256 signature for body.
func ComputeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature compares a received signature with a freshly computed one.
func VerifySignature(secret string, body []byte, candidate string) bool {
	expected := ComputeSignature(secret, body)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	candidateBytes, err := hex.DecodeString(candidate)
	if err != nil {
		return false
	}
	return hmac.Equal(expectedBytes, candidateBytes)
}
