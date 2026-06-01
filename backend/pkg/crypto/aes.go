// Package crypto は AES-GCM による決定的暗号化ユーティリティを提供する。
// 導出ノンス方式: nonce = SHA-256(key || plaintext)[:12] を使って
// 同じ入力に対して常に同じ暗号文を生成する（DB 検索に利用可能）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Encryptor は決定的 AES-GCM 暗号化・復号インターフェース
type Encryptor interface {
	// Encrypt は plaintext を暗号化する。同一入力に対して常に同じ出力を返す（決定的）。
	// 出力フォーマット: hex(nonce) + ":" + hex(ciphertext)
	Encrypt(plaintext string) (string, error)
	// Decrypt は Encrypt の出力を復号して元の平文を返す
	Decrypt(ciphertext string) (string, error)
}

// aesEncryptor は Encryptor の実装
type aesEncryptor struct {
	key []byte // 32バイト（AES-256）
}

// NewEncryptor は 32バイトの AES-256 キーで Encryptor を生成する。
// key は 64文字の hex 文字列（32バイト）または 32バイトのバイト列として解釈する。
func NewEncryptor(keyHex string) (Encryptor, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		// hex でなければ raw bytes として解釈
		if len(keyHex) != 32 {
			return nil, fmt.Errorf("crypto: key must be 32 bytes (64 hex chars or 32 raw chars), got %d", len(keyHex))
		}
		key = []byte(keyHex)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes for AES-256, got %d", len(key))
	}

	return &aesEncryptor{key: key}, nil
}

// deriveNonce は決定的ノンスを生成する: SHA-256(key || plaintext)[:12]
func (e *aesEncryptor) deriveNonce(plaintext string) []byte {
	h := sha256.New()
	h.Write(e.key)
	h.Write([]byte(plaintext))
	hash := h.Sum(nil)
	return hash[:12] // GCM nonce は 12バイト
}

// Encrypt は plaintext を AES-256-GCM で暗号化する
// 出力: hex(nonce) + ":" + hex(ciphertext)
func (e *aesEncryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	nonce := e.deriveNonce(plaintext)
	encrypted := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return hex.EncodeToString(nonce) + ":" + hex.EncodeToString(encrypted), nil
}

// Decrypt は Encrypt の出力を復号する
func (e *aesEncryptor) Decrypt(ciphertext string) (string, error) {
	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("crypto: invalid ciphertext format, expected 'nonce:ciphertext'")
	}

	nonce, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("crypto: failed to decode nonce: %w", err)
	}

	encrypted, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("crypto: failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decryption failed: %w", err)
	}

	return string(plaintext), nil
}
