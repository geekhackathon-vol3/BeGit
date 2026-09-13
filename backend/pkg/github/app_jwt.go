package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// appJWTLifetime is kept below GitHub's ten-minute maximum for App JWTs.
	appJWTLifetime  = 9 * time.Minute
	appJWTClockSkew = 1 * time.Minute
)

// GenerateAppJWT creates a short-lived RS256 JWT for authenticating as a
// GitHub App. The private key must be the PEM value downloaded from GitHub.
// The key is never included in returned errors.
func GenerateAppJWT(appID, privateKeyPEM string) (string, error) {
	return generateAppJWTAt(appID, privateKeyPEM, time.Now())
}

func generateAppJWTAt(appID, privateKeyPEM string, now time.Time) (string, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return "", errors.New("github: app ID is required")
	}
	if strings.TrimSpace(privateKeyPEM) == "" {
		return "", errors.New("github: app private key is required")
	}

	privateKey, err := parseRSAPrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return "", fmt.Errorf("github: invalid app private key: %w", err)
	}

	header, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("github: failed to encode JWT header: %w", err)
	}

	issuedAt := now.Add(-appJWTClockSkew).Unix()
	claims, err := json.Marshal(map[string]interface{}{
		"iat": issuedAt,
		"exp": now.Add(appJWTLifetime).Unix(),
		"iss": appID,
	})
	if err != nil {
		return "", fmt.Errorf("github: failed to encode JWT claims: %w", err)
	}

	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("github: failed to sign app JWT: %w", err)
	}

	return unsigned + "." + encode(signature), nil
}

func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("PEM block not found")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("unsupported private key format")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return key, nil
}
