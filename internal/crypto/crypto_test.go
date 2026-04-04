package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey() []byte {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plaintext := "4567 890123"
	key := testKey()

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	key := testKey()
	ct1, _ := Encrypt("test", key)
	ct2, _ := Encrypt("test", key)
	assert.NotEqual(t, ct1, ct2, "random nonce should produce different ciphertexts")
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := testKey()
	key2, _ := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	ct, _ := Encrypt("secret", key1)
	_, err := Decrypt(ct, key2)
	assert.Error(t, err)
}

func TestDecrypt_CorruptedData(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!", testKey())
	assert.Error(t, err)
}

func TestHMAC_Deterministic(t *testing.T) {
	key := testKey()
	h1 := ComputeHMAC("4567 890123", key)
	h2 := ComputeHMAC("4567 890123", key)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // hex-encoded SHA256 = 64 chars
}

func TestHMAC_DifferentInputs(t *testing.T) {
	key := testKey()
	h1 := ComputeHMAC("aaa", key)
	h2 := ComputeHMAC("bbb", key)
	assert.NotEqual(t, h1, h2)
}

func TestPassthrough_NilKey(t *testing.T) {
	ct, err := Encrypt("hello", nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", ct)

	pt, err := Decrypt("hello", nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", pt)

	h := ComputeHMAC("hello", nil)
	assert.Equal(t, "hello", h)
}

func TestParseHexKey_Valid(t *testing.T) {
	hexStr := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key, err := ParseHexKey(hexStr)
	require.NoError(t, err)
	assert.Len(t, key, 32)
}

func TestParseHexKey_Empty(t *testing.T) {
	key, err := ParseHexKey("")
	require.NoError(t, err)
	assert.Nil(t, key)
}

func TestParseHexKey_InvalidHex(t *testing.T) {
	_, err := ParseHexKey("not-hex")
	assert.Error(t, err)
}

func TestParseHexKey_WrongLength(t *testing.T) {
	_, err := ParseHexKey("0123456789abcdef") // 8 bytes, not 32
	assert.Error(t, err)
}
