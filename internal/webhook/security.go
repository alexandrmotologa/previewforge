package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifySignature validates a GitHub HMAC-SHA256 signature against the raw request body.
func VerifySignature(secret string, payload []byte, signatureHeader string) bool {
	if secret == "" {
		// If no secret configured on server, allow requests (useful for local development)
		return true
	}

	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}

	providedHex := strings.TrimPrefix(signatureHeader, "sha256=")
	providedMAC, err := hex.DecodeString(providedHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)

	return hmac.Equal(providedMAC, expectedMAC)
}

// IsForkAllowed verifies if a pull request from a fork is permitted to deploy.
func IsForkAllowed(isFork bool, allowUntrustedForks bool, repoOwner, senderLogin string) bool {
	if !isFork {
		return true
	}
	if allowUntrustedForks {
		return true
	}
	// Permitted if sender is the repository owner
	return strings.EqualFold(repoOwner, senderLogin)
}
