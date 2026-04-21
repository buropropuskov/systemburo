package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

func main() {
	password := "admin123"
	salt := make([]byte, 16)
	rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)
	fmt.Printf("$argon2id$v=%d$m=19456,t=2,p=1$%s$%s\n", argon2.Version, saltB64, hashB64)
}
