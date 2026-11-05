package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type argon2Params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// Both the Go generator and its Rust argon2 0.5.3 predecessor use this
// Argon2id-v19 profile. Supporting a new stored profile requires an explicit
// compatibility review: the semaphore bounds concurrent work, not one hash.
const passwordPHCPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
const passwordPHCMaxLength = 97

// Public, synthetic data. Its comparison result is deliberately ignored;
// deriving it still costs exactly one ordinary current-profile verification.
const unknownLoginPHC = passwordPHCPrefix + "AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

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
	invalid := func() ([]byte, []byte, argon2Params, error) {
		return nil, nil, argon2Params{}, fmt.Errorf("invalid PHC format: unsupported or malformed password profile")
	}
	// Bound splitting and base64 allocation before inspecting attacker-corrupted
	// stored data. Never include any part of the stored PHC in an error.
	if len(phc) > passwordPHCMaxLength {
		return invalid()
	}
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return invalid()
	}
	fields := strings.Split(parts[3], ",")
	if len(fields) != 3 {
		return invalid()
	}
	var values [3]uint32
	for i, name := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(fields[i], name) {
			return invalid()
		}
		digits := strings.TrimPrefix(fields[i], name)
		if digits == "" {
			return invalid()
		}
		for _, digit := range digits {
			if digit < '0' || digit > '9' {
				return invalid()
			}
		}
		value, parseErr := strconv.ParseUint(digits, 10, 32)
		if parseErr != nil || strconv.FormatUint(value, 10) != digits {
			return invalid()
		}
		values[i] = uint32(value)
	}
	// Check the whole profile before narrowing parallelism to uint8 or IDKey.
	if values != [3]uint32{19456, 2, 1} || len(parts[4]) != 22 || len(parts[5]) != 43 {
		return invalid()
	}
	salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 || base64RawEncode(salt) != parts[4] {
		return invalid()
	}
	hash, err = base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(hash) != 32 || base64RawEncode(hash) != parts[5] {
		return invalid()
	}
	params = argon2Params{memory: values[0], time: values[1], threads: uint8(values[2])}
	return salt, hash, params, nil
}

// subtleCompare does constant-time comparison.
func subtleCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
