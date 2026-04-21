package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/argon2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		log.Fatalf("failed to generate salt: %v", err)
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=19456,t=2,p=1$%s$%s", argon2.Version, saltB64, hashB64)
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/auto_registry?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}

	// Убедимся что организация и компания существуют
	var orgID, compID int
	db.Raw("INSERT INTO organizations (name) VALUES ('Бюро пропусков') ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id").Scan(&orgID)
	db.Raw("INSERT INTO companies (name) VALUES ('Бюро пропусков') ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id").Scan(&compID)

	// type_id=6 = buropropuskov
	var typeID int
	db.Raw("SELECT id FROM user_types WHERE code = 'buropropuskov'").Scan(&typeID)
	if typeID == 0 {
		log.Fatal("user_type 'buropropuskov' not found — run migrations first")
	}

	password := "admin123"
	if len(os.Args) > 1 {
		password = os.Args[1]
	}

	hash := hashPassword(password)

	lastName := "Администратор"
	firstName := "Системный"

	result := db.Exec(`
		INSERT INTO users (username, password, organization_id, company_id, type_id, last_name, first_name)
		VALUES ('buropropuskov', ?, ?, ?, ?, ?, ?)
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id
	`, hash, orgID, compID, typeID, lastName, firstName)

	if result.Error != nil {
		log.Fatalf("Failed to seed admin: %v", result.Error)
	}

	fmt.Printf("Admin user 'buropropuskov' seeded (password: %s, type_id: %d)\n", password, typeID)
}
