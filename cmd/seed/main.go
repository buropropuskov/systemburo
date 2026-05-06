package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"systemburo/internal/database"

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

	db, err := gorm.Open(postgres.Open(database.EnsureUTCTimezone(dsn)), &gorm.Config{})
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

	// is_super_admin=true дублирует type_id=6 для поэтапного отказа от хардкода (#187a).
	// Старые проверки по type_id=6 продолжают работать; новые — через is_super_admin.
	result := db.Exec(`
		INSERT INTO users (username, password, organization_id, company_id, type_id, is_super_admin, last_name, first_name)
		VALUES ('buropropuskov', ?, ?, ?, ?, true, ?, ?)
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id,
			is_super_admin = true
	`, hash, orgID, compID, typeID, lastName, firstName)

	if result.Error != nil {
		log.Fatalf("Failed to seed admin: %v", result.Error)
	}

	fmt.Printf("Admin user 'buropropuskov' seeded (password: %s, type_id: %d)\n", password, typeID)

	// Дополнительные e2e-пользователи создаются только по флагу окружения.
	// В production-деплое не вызывается (см. Makefile deploy-seed / staging-seed).
	if os.Getenv("SEED_E2E_USERS") == "true" {
		seedE2EUsers(db, orgID, compID, typeID)
	}

	// Демо-данные для UI-сценариев (объявления, новости, заявки с вложениями, cars_history).
	// По флагу. Идемпотентно — повторный запуск не плодит дубликаты.
	if os.Getenv("SEED_DEMO") == "true" {
		var userID int
		db.Raw("SELECT id FROM users WHERE username = 'buropropuskov' LIMIT 1").Scan(&userID)
		if userID != 0 {
			seedDemoData(db, orgID, compID, userID)
		} else {
			log.Printf("demo seed: buropropuskov user not found, skipping demo data")
		}
	}
}

func seedE2EUsers(db *gorm.DB, orgID, compID, buroTypeID int) {
	const e2ePassword = "testpass123"
	hash := hashPassword(e2ePassword)

	// e2e_admin — тот же type_id что buropropuskov (админ).
	adminResult := db.Exec(`
		INSERT INTO users (username, password, organization_id, company_id, type_id, last_name, first_name)
		VALUES ('e2e_admin', ?, ?, ?, ?, 'E2E', 'Admin')
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id
	`, hash, orgID, compID, buroTypeID)
	if adminResult.Error != nil {
		log.Fatalf("Failed to seed e2e_admin: %v", adminResult.Error)
	}

	// e2e_user — обычный юзер. Берём type_id=1 (первый не-админ тип).
	var userTypeID int
	db.Raw("SELECT id FROM user_types WHERE code != 'buropropuskov' ORDER BY id LIMIT 1").Scan(&userTypeID)
	if userTypeID == 0 {
		userTypeID = 1
	}
	userResult := db.Exec(`
		INSERT INTO users (username, password, organization_id, company_id, type_id, last_name, first_name)
		VALUES ('e2e_user', ?, ?, ?, ?, 'E2E', 'User')
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id
	`, hash, orgID, compID, userTypeID)
	if userResult.Error != nil {
		log.Fatalf("Failed to seed e2e_user: %v", userResult.Error)
	}

	fmt.Printf("E2E users seeded: e2e_admin (type_id=%d), e2e_user (type_id=%d), password=%s\n", buroTypeID, userTypeID, e2ePassword)
}
