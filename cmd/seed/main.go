package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/normalize"
	"systemburo/internal/services"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// hashPassword хеширует пароль тем же кодом, что и рабочий сервис (#1907).
// Своя копия параметров Argon2id здесь была источником тихого расхождения:
// правка параметров в auth_service оставляла сид со старыми.
func hashPassword(password string) string {
	return services.HashPassword(password)
}

// defaultPassword - пароль супер-администратора, когда его не задали. Не секрет:
// описан в руководстве по развёртыванию и меняется первым делом после установки.
const defaultPassword = "admin123"

// passwordSource сообщает, откуда взялся пароль, - от этого зависит и предупреждение,
// и что печатать в конце.
type passwordSource int

const (
	passwordDefault passwordSource = iota
	passwordFlag
	passwordPositional
)

// resolvePassword разбирает аргументы командной строки.
//
// Появилась из-за живого инцидента (#1760): пароль брался как os.Args[1] без разбора
// флагов, поэтому `--help` не печатал справку, а молча выставлял учётной записи
// buropropuskov пароль "--help" - на общей базе стенда сразу всем. Теперь разбор идёт
// через flag: справка и неизвестный флаг обрабатываются им, до базы дело не доходит.
//
// Позиционный аргумент остаётся рабочим намеренно: так пароль передают сборка e2e в
// CI и цели Makefile. Ломать их этой правкой нельзя, поэтому старый способ работает,
// но предупреждает. Ключевое отличие от прежнего поведения - в пароль попадает только
// то, что не выглядит флагом.
func resolvePassword(fs *flag.FlagSet, args []string) (string, passwordSource, error) {
	flagPassword := fs.String("password", "", "пароль супер-администратора (по умолчанию "+defaultPassword+")")
	if err := fs.Parse(args); err != nil {
		return "", passwordDefault, err
	}
	if *flagPassword != "" {
		return *flagPassword, passwordFlag, nil
	}
	if fs.NArg() > 0 {
		return fs.Arg(0), passwordPositional, nil
	}
	return defaultPassword, passwordDefault, nil
}

// usage - справка. Перечисляет и переменные окружения: без них не понять, чем
// отличаются make seed, seed-demo и прогон в CI.
func usage(out io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(out, "Создаёт или обновляет учётную запись супер-администратора buropropuskov.")
	fmt.Fprintln(out, "\nИспользование:\n  seed [-password ПАРОЛЬ]")
	fmt.Fprintln(out, "\nФлаги:")
	fs.PrintDefaults()
	fmt.Fprintln(out, "\nПеременные окружения:")
	fmt.Fprintln(out, "  DATABASE_URL       строка подключения к базе")
	fmt.Fprintln(out, "  SEED_DEMO=true     добавить демонстрационные данные")
	fmt.Fprintln(out, "  SEED_E2E_USERS=true  добавить учётные записи для автотестов")
	fmt.Fprintln(out, "\nБез -password ставится пароль по умолчанию: "+defaultPassword)
}

func main() {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	fs.Usage = func() { usage(os.Stderr, fs) }
	password, source, err := resolvePassword(fs, os.Args[1:])
	if err != nil {
		// ExitOnError уже завершил процесс на разборе; ветка нужна для полноты.
		log.Fatalf("не удалось разобрать аргументы: %v", err)
	}
	if source == passwordPositional {
		fmt.Fprintln(os.Stderr, "Предупреждение: пароль позиционным аргументом устарел, используйте -password ПАРОЛЬ")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/auto_registry?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(database.EnsureUTCTimezone(dsn)), &gorm.Config{})
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}

	// Убедимся что организация и компания существуют.
	// У organizations теперь partial unique по name (WHERE is_active=true) - в
	// ON CONFLICT нужно указать тот же предикат, иначе арбитр-индекс не найдётся.
	// name_normalized заполняем здесь же: raw INSERT идёт в обход gorm-модели, её хук
	// BeforeSave не срабатывает, а без ключа запись не участвует в дедупликации
	// наименований (#1437) до следующего запуска сервера с бэкфиллом.
	const seedDirectoryName = "Бюро пропусков"
	seedDirectoryKey := normalize.OrgName(seedDirectoryName)
	var orgID, compID int
	db.Raw("INSERT INTO organizations (name, name_normalized) VALUES (?, ?) ON CONFLICT (name) WHERE is_active = true DO UPDATE SET name_normalized = EXCLUDED.name_normalized RETURNING id",
		seedDirectoryName, seedDirectoryKey).Scan(&orgID)
	db.Raw("INSERT INTO companies (name, name_normalized) VALUES (?, ?) ON CONFLICT (name) WHERE is_active = true DO UPDATE SET name_normalized = EXCLUDED.name_normalized RETURNING id",
		seedDirectoryName, seedDirectoryKey).Scan(&compID)

	// type_id=6 = buropropuskov
	var typeID int
	db.Raw("SELECT id FROM user_types WHERE code = 'buropropuskov'").Scan(&typeID)
	if typeID == 0 {
		log.Fatal("user_type 'buropropuskov' not found — run migrations first")
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

	// Что именно сделано - вслух: раньше команда молчала, и после случайного
	// `--help` было не видно, что пароль подменён (#1760). Заданный пароль в вывод
	// не печатаем: он уходит в журналы CI и в историю терминала. Пароль по умолчанию
	// печатаем - он и так стоит в руководстве, а без него не войти после установки.
	switch source {
	case passwordDefault:
		fmt.Printf("Учётная запись buropropuskov готова, пароль по умолчанию: %s (тип %d)\n", password, typeID)
	default:
		fmt.Printf("Учётная запись buropropuskov готова, пароль изменён на заданный (тип %d)\n", typeID)
	}

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

	// onboardingDoneVersion — заведомо выше любой реальной ONBOARDING_VERSION:
	// E2E-юзерам тур не нужен, а его автозапуск перекрывает UI оверлеем и валит
	// клики чужих тестов (shard 3/4 notifications). Берём с запасом, чтобы не
	// ломаться при росте версии тура.
	const onboardingDoneVersion = 1000

	// e2e_admin — тот же type_id что buropropuskov (админ).
	adminResult := db.Exec(`
		INSERT INTO users (username, password, organization_id, company_id, type_id, last_name, first_name, onboarding_completed_version)
		VALUES ('e2e_admin', ?, ?, ?, ?, 'E2E', 'Admin', ?)
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id,
			onboarding_completed_version = EXCLUDED.onboarding_completed_version
	`, hash, orgID, compID, buroTypeID, onboardingDoneVersion)
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
		INSERT INTO users (username, password, organization_id, company_id, type_id, last_name, first_name, onboarding_completed_version)
		VALUES ('e2e_user', ?, ?, ?, ?, 'E2E', 'User', ?)
		ON CONFLICT (username) DO UPDATE SET
			password = EXCLUDED.password,
			organization_id = EXCLUDED.organization_id,
			company_id = EXCLUDED.company_id,
			type_id = EXCLUDED.type_id,
			onboarding_completed_version = EXCLUDED.onboarding_completed_version
	`, hash, orgID, compID, userTypeID, onboardingDoneVersion)
	if userResult.Error != nil {
		log.Fatalf("Failed to seed e2e_user: %v", userResult.Error)
	}

	// e2e_user должен иметь базовую роль "Пользователь" - иначе резолвер прав
	// отдаёт пустой набор (default-deny) и гейтящиеся по правам элементы (вкладки
	// реестра, кнопка «Добавить») скрыты, что роняет e2e cars/employees. Backfill
	// базовой роли в миграции не цепляет e2e_user: сид выполняется уже после неё.
	var baseRoleID int
	db.Raw("SELECT id FROM roles WHERE code = 'user' AND is_system = true LIMIT 1").Scan(&baseRoleID)
	if baseRoleID != 0 {
		if res := db.Exec(
			`UPDATE users SET role_id = ? WHERE username = 'e2e_user' AND role_id IS NULL`,
			baseRoleID,
		); res.Error != nil {
			log.Fatalf("Failed to assign base role to e2e_user: %v", res.Error)
		}
	} else {
		// Сюда попадаем только при SEED_E2E_USERS=true - без базовой роли e2e_user
		// останется default-deny, и e2e cars/employees точно покраснеют. Кричим явно,
		// чтобы причина была видна в логе сида, а не всплыла загадочным красным e2e.
		log.Printf("WARN: base role 'user' not found - e2e_user без роли, e2e cars/employees упадут на гейтах")
	}

	// buropropuskov логинится в большинстве E2E-сценариев — ему автозапуск тура
	// тоже мешает. Помечаем пройденным ТОЛЬКО в E2E (эта ветка по SEED_E2E_USERS),
	// на staging/prod реальный админ тур увидит.
	if res := db.Exec(
		`UPDATE users SET onboarding_completed_version = ? WHERE username = 'buropropuskov'`,
		onboardingDoneVersion,
	); res.Error != nil {
		log.Fatalf("Failed to mark buropropuskov onboarding done: %v", res.Error)
	}

	markAllToursCompleted(db, onboardingDoneVersion, "e2e_admin", "e2e_user", "buropropuskov")

	fmt.Printf("E2E users seeded: e2e_admin (type_id=%d), e2e_user (type_id=%d), password=%s\n", buroTypeID, userTypeID, e2ePassword)
}

// markAllToursCompleted помечает пройденными ВСЕ туры перечисленных учёток. Туров
// стало пять и прогресс у каждого свой (#1737): пометить один - значит оставить
// остальные непройденными, а любой автозапуск перекрывает интерфейс оверлеем и валит
// клики чужих E2E-тестов (#657). Идемпотентно, версию только поднимает.
func markAllToursCompleted(db *gorm.DB, version int, usernames ...string) {
	const q = `
		INSERT INTO user_onboarding_progress (user_id, tour_key, completed_version, completed_at)
		SELECT id, ?, ?, NOW() FROM users WHERE username = ?
		ON CONFLICT (user_id, tour_key) DO UPDATE
		SET completed_version = EXCLUDED.completed_version,
		    completed_at = EXCLUDED.completed_at
		WHERE user_onboarding_progress.completed_version < EXCLUDED.completed_version`
	for _, username := range usernames {
		for _, tour := range models.TourKeys {
			if res := db.Exec(q, tour, version, username); res.Error != nil {
				log.Fatalf("Failed to mark onboarding tour %q done for %s: %v", tour, username, res.Error)
			}
		}
	}
}
