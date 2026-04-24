package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	portalSSOSecretVersion = "v1"
	portalSSOSecretAAD     = "aigateway-portal-sso-config"
)

func EncryptPortalSSOSecret(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	block, err := aes.NewCipher(portalSSOSecretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(trimmed), []byte(portalSSOSecretAAD))
	return fmt.Sprintf(
		"%s:%s:%s",
		portalSSOSecretVersion,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ciphertext),
	), nil
}

func DecryptPortalSSOSecret(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) != 3 {
		return trimmed, nil
	}
	if parts[0] != portalSSOSecretVersion {
		return "", fmt.Errorf("unsupported portal sso secret version: %s", parts[0])
	}

	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(portalSSOSecretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(portalSSOSecretAAD))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func MaskPortalSSOSecret(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return ""
	case len(trimmed) <= 8:
		return "********"
	default:
		return fmt.Sprintf("%s******%s", trimmed[:3], trimmed[len(trimmed)-2:])
	}
}

func portalSSOSecretKey() []byte {
	sum := sha256.Sum256([]byte(portalSSOSecretAAD + ":" + portalSSOSecretVersion))
	return sum[:]
}
