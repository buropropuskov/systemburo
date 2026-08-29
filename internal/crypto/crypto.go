package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	hmacpkg "crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

var globalKey []byte

// SetGlobalKey sets the encryption key for GORM hooks. Called once at startup.
func SetGlobalKey(key []byte) { globalKey = key }

// GetGlobalKey returns the current global key (nil = passthrough mode).
func GetGlobalKey() []byte { return globalKey }

// Encrypt encrypts plaintext with AES-256-GCM. Returns base64(nonce+ciphertext).
// If key is nil, returns plaintext unchanged (passthrough).
func Encrypt(plaintext string, key []byte) (string, error) {
	if key == nil {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("rand nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64(nonce+ciphertext) with AES-256-GCM.
// If key is nil, returns ciphertext unchanged (passthrough).
func Decrypt(ciphertext string, key []byte) (string, error) {
	if key == nil {
		return ciphertext, nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("gcm.Open: %w", err)
	}
	return string(plaintext), nil
}

// ComputeHMAC returns HMAC-SHA256 of value as hex string. Deterministic.
// If key is nil, returns value unchanged (passthrough).
func ComputeHMAC(value string, key []byte) string {
	if key == nil {
		return value
	}
	mac := hmacpkg.New(sha256.New, key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// ParseHexKey parses hex-encoded 32-byte key. Returns nil,nil if empty.
func ParseHexKey(hexStr string) ([]byte, error) {
	if hexStr == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (got %d)", len(key))
	}
	return key, nil
}

// EncryptOptional encrypts *string using global key. For GORM hooks.
func EncryptOptional(val *string) (*string, error) {
	if val == nil {
		return nil, nil
	}
	enc, err := Encrypt(*val, globalKey)
	if err != nil {
		return nil, err
	}
	return &enc, nil
}

// DecryptOptional decrypts *string using global key. For GORM hooks.
func DecryptOptional(val *string) *string {
	if val == nil {
		return nil
	}
	dec, err := Decrypt(*val, globalKey)
	if err != nil {
		// Значение возвращается как есть, иначе один сбойный столбец ронял бы
		// выдачу целиком. Но неудача попадает в счётчик и в журнал: молчание тут
		// и приводило к тому, что оператор видел набор символов вместо паспорта,
		// а причину никто не знал.
		noteDecryptFailure()
		return val
	}
	return &dec
}

// HMACOptional computes HMAC of *string using global key. For GORM hooks.
func HMACOptional(val *string) *string {
	if val == nil {
		return nil
	}
	h := ComputeHMAC(*val, globalKey)
	return &h
}
