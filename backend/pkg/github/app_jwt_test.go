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
	"strings"
	"testing"
	"time"
)

func TestGenerateAppJWT(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() failed: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	token, err := generateAppJWTAt("4886659", string(privateKeyPEM), now)
	if err != nil {
		t.Fatalf("generateAppJWTAt() failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected JWT with 3 parts, got %d", len(parts))
	}

	decode := func(value string, target interface{}) {
		t.Helper()
		data, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("failed to decode JWT part: %v", err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("failed to decode JSON part: %v", err)
		}
	}

	var header map[string]string
	decode(parts[0], &header)
	if header["alg"] != "RS256" || header["typ"] != "JWT" {
		t.Fatalf("unexpected JWT header: %#v", header)
	}

	var claims map[string]interface{}
	decode(parts[1], &claims)
	if claims["iss"] != "4886659" {
		t.Fatalf("unexpected iss claim: %#v", claims["iss"])
	}
	if claims["iat"] != float64(now.Add(-appJWTClockSkew).Unix()) {
		t.Fatalf("unexpected iat claim: %#v", claims["iat"])
	}
	if claims["exp"] != float64(now.Add(appJWTLifetime).Unix()) {
		t.Fatalf("unexpected exp claim: %#v", claims["exp"])
	}

	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("failed to decode JWT signature: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("JWT signature verification failed: %v", err)
	}
}

func TestGenerateAppJWT_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name          string
		appID         string
		privateKeyPEM string
	}{
		{name: "missing app ID", appID: "", privateKeyPEM: "key"},
		{name: "missing private key", appID: "4886659", privateKeyPEM: ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := GenerateAppJWT(test.appID, test.privateKeyPEM); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
