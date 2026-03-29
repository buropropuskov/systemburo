package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
)

type argon2Params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// generateSalt creates a 16-byte random salt.
func generateSalt() []byte {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return salt
}

// base64RawEncode encodes bytes to base64 without padding (PHC standard).
func base64RawEncode(data []byte) string {
	return base64.RawStdEncoding.EncodeToString(data)
}

// parsePHC parses an Argon2 PHC-format hash string.
// Format: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
func parsePHC(phc string) (salt, hash []byte, params argon2Params, err error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 {
		return nil, nil, params, fmt.Errorf("invalid PHC format: expected 6 parts, got %d", len(parts))
	}

	// parts[0] = "" (leading $)
	// parts[1] = "argon2id"
	// parts[2] = "v=19"
	// parts[3] = "m=19456,t=2,p=1"
	// parts[4] = base64(salt)
	// parts[5] = base64(hash)

	if parts[1] != "argon2id" && parts[1] != "argon2i" && parts[1] != "argon2d" {
		return nil, nil, params, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	// Parse params
	var m, t, p uint32
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return nil, nil, params, fmt.Errorf("failed to parse params: %w", err)
	}
	params = argon2Params{memory: m, time: t, threads: uint8(p)}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, params, fmt.Errorf("failed to decode salt: %w", err)
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, params, fmt.Errorf("failed to decode hash: %w", err)
	}

	return salt, hash, params, nil
}

// subtleCompare does constant-time comparison.
func subtleCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
