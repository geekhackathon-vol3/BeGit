package crypto

import (
	"strings"
	"testing"
)

// TestEncryptDecrypt は Encrypt → Decrypt で元の文字列が復元されることを確認する
func TestEncryptDecrypt(t *testing.T) {
	// 32バイトのAES-256用キー（hex文字列）
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor() failed: %v", err)
	}

	plaintext := "github_access_token_abc123"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

// TestEncryptDeterministic は同じ入力に対して Encrypt が常に同じ出力を返すことを確認する（決定的暗号化）
func TestEncryptDeterministic(t *testing.T) {
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor() failed: %v", err)
	}

	plaintext := "deterministic_test_token"
	ciphertext1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() #1 failed: %v", err)
	}

	ciphertext2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() #2 failed: %v", err)
	}

	if ciphertext1 != ciphertext2 {
		t.Errorf("Encrypt is not deterministic: %q != %q", ciphertext1, ciphertext2)
	}
}

// TestEncryptDifferentInputs は異なる入力が異なる出力になることを確認する
func TestEncryptDifferentInputs(t *testing.T) {
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor() failed: %v", err)
	}

	ct1, err := enc.Encrypt("token_a")
	if err != nil {
		t.Fatalf("Encrypt() failed: %v", err)
	}

	ct2, err := enc.Encrypt("token_b")
	if err != nil {
		t.Fatalf("Encrypt() failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("different inputs produced the same ciphertext")
	}
}

// TestEncryptOutputFormat は出力フォーマットが hex(nonce) + ":" + hex(ciphertext) であることを確認する
func TestEncryptOutputFormat(t *testing.T) {
	key := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor() failed: %v", err)
	}

	ciphertext, err := enc.Encrypt("test_token")
	if err != nil {
		t.Fatalf("Encrypt() failed: %v", err)
	}

	// ":" で分割して nonce と ciphertext が hex であることを確認
	for _, r := range ciphertext {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || r == ':') {
			t.Errorf("unexpected character %q in ciphertext %q", r, ciphertext)
		}
	}

	// nonce は 12バイト = 24文字の hex
	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 'nonce:ciphertext' format, got %q", ciphertext)
	}
	if len(parts[0]) != 24 {
		t.Errorf("expected nonce length 24 (12 bytes hex), got %d", len(parts[0]))
	}
}

// TestNewEncryptorInvalidKey は無効なキーでエラーが返ることを確認する
func TestNewEncryptorInvalidKey(t *testing.T) {
	_, err := NewEncryptor("tooshort")
	if err == nil {
		t.Error("expected error for too short key")
	}
}
