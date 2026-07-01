package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"gorm.io/gorm"
)

// AllModels returns all GORM models for AutoMigrate.
// Order matters: referenced tables first, then dependents.
func AllModels() []interface{} {
	return []interface{}{
		// Core (no FK dependencies)
		&models.AuditLog{},
		&models.SystemSetting{},
		&models.UserType{},
		&models.Organization{},
		&models.Company{},
		&models.Citizenship{},
		&models.LicensePlateFormat{},
		&models.Mark{},
		&models.VehicleBlacklist{},
		&models.PersonBlacklist{},
		&models.UnloadPlace{},
		&models.SystemTable{},

		// Users (depends on UserType, Organization, Company)
		&models.User{},
		&models.UserBanHistory{},
		&models.RefreshToken{},
		&models.AuthEvent{},
		&models.OrganizationUser{},
		&models.CompaniesUser{},

		// License plate cells (depends on LicensePlateFormat)
		&models.LicensePlateFormatCell{},

		// System tables relations
		&models.SystemTablePhoto{},
		&models.SystemTableTimeSlot{},
		&models.OrganizationTable{},
		&models.CompaniesTable{},
		&models.TableField{},
		&models.TableFieldFact{},

		// Unload places relations
		&models.UnloadPlacePhoto{},
		&models.UnloadPlaceTimeSlot{},
		&models.OrganizationUnloadPlace{},
		&models.CompaniesUnloadPlace{},

		// Bureau (single-owner schedule, no FK)
		&models.BureauTimeSlot{},

		// Applications (depends on User, Organization, Company)
		&models.Application{},
		&models.ApplicationRead{},
		&models.ApplicationStatusHistory{},
		&models.ApplicationResponsibleUser{},
		&models.ApplicationApprover{},
		&models.ApplicationViewer{},

		// Unique records
		&models.UniqueAttachment{},
		&models.UniqueCar{},
		&models.UniqueEmployee{},

		// Attachments (depends on Application, UniqueAttachment)
		&models.Attachment{},

		// Forward attachments (#680: пер-вложенный пересыл; depends on Application, User, Attachment)
		&models.ForwardAttachment{},

		// Доступные мне (#706): привязка мест к охраннику + место разгрузки на уровне вложения
		// (depends on User, UnloadPlace, SystemTable, Attachment)
		&models.SecurityUserUnloadPlace{},
		&models.SecurityUserTable{},
		&models.AttachmentUnloadPlace{},

		// Attachment templates (#183: Excel-бланки)
		&models.AttachmentTemplate{},
		&models.AttachmentTemplateMapping{},
		&models.AttachmentCustomField{},
		&models.AttachmentCustomValue{},
		&models.AttachmentFieldConfig{},

		// Cars (depends on Attachment)
		&models.Car{},
		&models.CarUnloadPlace{},

		// Employees (depends on Attachment, Citizenship)
		&models.Employee{},
		&models.ApplicationEmployee{},
		&models.EmployeeFile{},
		&models.EmployeeTargetTable{},

		// Per-element флаги возможного обхода ЧС (#481): ссылается на cars.id/employees.id
		// без FK (снимок момента подачи, переживает изменение/удаление элемента и записи ЧС).
		&models.ApplicationBlacklistFlag{},
		// Аудит override помеченных элементов (#481, срез 4): кто/когда/коммент "всё равно пропустить".
		&models.ApplicationBlacklistOverride{},

		// Items (depends on Attachment)
		&models.Item{},
		&models.ApplicationItem{},

		// Feedback & notifications
		&models.Feedback{},
		&models.Notification{},
		&models.BugReport{},

		// News
		&models.News{},
		&models.Announcement{},

		// Logging
		&models.RequestLog{},
		// request_logs партиционируется нативно (installLogPartitioning) - вне AutoMigrate.

		// Permissions
		&models.Permission{},
		&models.UserPermission{},

		// New permission system (#229): roles, groups, grants, overrides.
		// PermissionGroup перед Role - RoleDefaultGroup ссылается на обе.
		&models.PermissionGroup{},
		&models.Role{},
		&models.RoleDefaultGroup{},
		&models.PermissionGroupGrant{},
		&models.RolePermissionGrant{},
		&models.UserGroup{},
		&models.UserPermissionOverride{},

		// Access denials journal (#230)
		&models.AccessDenial{},
		&models.AccessDenialArchive{},

		// PD consent & audit (152-FZ)
		&models.PDConsent{},
		// pd_audit_logs партиционируется нативно (installLogPartitioning) - вне AutoMigrate.

		// Documents (#39)
		&models.DocumentGroup{},
		&models.Document{},

		// Разделы руководства (B1): текст по ролям + метаданные PDF
		&models.GuideSection{},

		// Report templates (#632): сохранённые наборы конструктора отчётов
		&models.ReportTemplate{},
		// Дневные пики онлайна пользователей (#632): снимок фонового тикера
		&models.UserOnlinePeak{},
		// Снимок тёплого кэша аналитики дашборда/insights (прогрев после рестарта)
		&models.AnalyticsCache{},
	}
}

// AutoMigrate creates/updates all tables from GORM models.
func AutoMigrate(db *gorm.DB) error {
	slog.Info("running AutoMigrate for all models")
	if err := installExtensions(db); err != nil {
		return err
	}
	if err := installLogPartitioning(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return err
	}
	if err := fixAttachmentTemplateIndex(db); err != nil {
		return err
	}
	if err := relaxApplicationOrgNotNull(db); err != nil {
		return err
	}
	if err := widenFactTableHint(db); err != nil {
		return err
	}
	if err := installSQLFunctions(db); err != nil {
		return err
	}
	if err := EnforceSingleSuperAdmin(db); err != nil {
		return err
	}
	if err := SeedReportTemplates(db); err != nil {
		return err
	}
	if err := createStatisticsIndexes(db); err != nil {
		return err
	}
	if err := createSearchIndexes(db); err != nil {
		return err
	}
	slog.Info("AutoMigrate completed")
	return nil
}

// createStatisticsIndexes добавляет индексы под реальные запросы аналитики (#632):
// фильтр по дате подачи заявок, список машин по статусу на территории. Все CREATE INDEX
// IF NOT EXISTS — идемпотентны и аддитивны, существующие данные/схему не трогают.
// Движок въездов/входов читает audit_log (#870, F.5/F.6) — его покрывает idx_audit_entity
// из модели AuditLog; индексы на дропнутых cars_history/employees_history убраны в F.8.
func createStatisticsIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_applications_sending_datetime ON applications (sending_datetime)`,
		`CREATE INDEX IF NOT EXISTS idx_cars_territory_status ON cars (territory_status)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create statistics index: %w", err)
		}
	}
	return nil
}

// createSearchIndexes добавляет GIN trgm-индексы под мега-поиск заявок (#46): ILIKE
// '%...%' и pg_trgm similarity по полям заявки и вложений ускоряются gin_trgm_ops.
// Все CREATE INDEX IF NOT EXISTS - идемпотентны и аддитивны. Индексы по отдельным
// колонкам (не по concat-выражению: concat_ws не IMMUTABLE, expression-индекс не создать).
func createSearchIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_applications_number_trgm ON applications USING gin (application_number gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_applications_message_trgm ON applications USING gin (message gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_applications_resp_comment_trgm ON applications USING gin (responsible_comment gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_organizations_name_trgm ON organizations USING gin (name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_companies_name_trgm ON companies USING gin (name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_users_last_name_trgm ON users USING gin (last_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_users_first_name_trgm ON users USING gin (first_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_users_middle_name_trgm ON users USING gin (middle_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_cars_number_trgm ON cars USING gin (car_number gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_cars_mark_name_trgm ON cars USING gin (mark_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_employees_last_name_trgm ON employees USING gin (last_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_employees_first_name_trgm ON employees USING gin (first_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_employees_middle_name_trgm ON employees USING gin (middle_name gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_employees_position_trgm ON employees USING gin (position gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_app_resp_users_comment_trgm ON application_responsible_users USING gin (approval_comment gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS idx_unload_places_name_trgm ON unload_places USING gin (name gin_trgm_ops)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create search index: %w", err)
		}
	}
	return nil
}

// SeedReportTemplates идемпотентно создаёт системные пресеты конструктора отчётов
// (#632). Config зеркалит состояние гида и непрозрачен для бэка — его применяет
// фронт. Создаём только отсутствующие по (name, is_system), существующие не трогаем.
// Идемпотентность через Count, а не уникальный индекс: личные шаблоны разных
// пользователей могут совпадать по имени, поэтому DB-level unique на name не ставим.
func SeedReportTemplates(db *gorm.DB) error {
	presets := []models.ReportTemplate{
		{
			Name:        "Сводка за неделю",
			Description: "Поданные заявки по дням за последнюю неделю.",
			Config:      json.RawMessage(`{"mode":"aggregate","metrics":["applications_count"],"dimension":"period","granularity":"day","period_preset":"week"}`),
		},
		{
			Name:        "Проведение работ",
			Description: "Заявки на работы с деталями: организация, наименование, ответственный, период.",
			Config:      json.RawMessage(`{"mode":"list","entity":"work_applications"}`),
		},
		{
			Name:        "Машины по местам",
			Description: "Список машин с организацией, маркой и местом разгрузки.",
			Config:      json.RawMessage(`{"mode":"list","entity":"cars"}`),
		},
		{
			Name:        "Самые популярные места",
			Description: "Места разгрузки по числу въездов машин - самые загруженные сверху.",
			Config:      json.RawMessage(`{"mode":"aggregate","metrics":["car_entries_count"],"dimension":"unload_place"}`),
		},
		{
			Name:        "Проходы людей",
			Description: "Входы людей по дням за последнюю неделю.",
			Config:      json.RawMessage(`{"mode":"aggregate","metrics":["people_entries_count"],"dimension":"period","granularity":"day","period_preset":"week"}`),
		},
	}

	for i := range presets {
		presets[i].IsSystem = true
		var count int64
		if err := db.Model(&models.ReportTemplate{}).
			Where("name = ? AND is_system = ?", presets[i].Name, true).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check system report template %q: %w", presets[i].Name, err)
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&presets[i]).Error; err != nil {
			return fmt.Errorf("seed system report template %q: %w", presets[i].Name, err)
		}
	}
	return nil
}

// EnforceSingleSuperAdmin поддерживает инвариант "ровно один супер-админ".
// Канонический супер-админ выбирается так (приоритет сверху): аккаунт с
// username='buropropuskov' (системный администратор), иначе самый ранний из уже
// существующих супер-админов, иначе первый зарегистрированный пользователь.
// Канонику флаг гарантируется, у всех остальных снимается -- но снятые супера
// становятся обычными администраторами (is_admin), чтобы не потерять доступ
// (admin = всё кроме super-only ключей и личных deny). Имя системного аккаунта
// нормализуется в "Системный Администратор" только если оно пустое (реальное ФИО
// не затирается). Идемпотентна.
//
// Заменяет прежний backfillSuperAdmin, который ошибочно делал супером ВСЕХ
// пользователей типа buropropuskov: при двух+ buro-аккаунтах получалось несколько
// супер-админов, что ломало модель "единственный неудаляемый владелец".
func EnforceSingleSuperAdmin(db *gorm.DB) error {
	var canonicalID int
	if err := db.Raw(`
		SELECT id FROM users
		ORDER BY (username = 'buropropuskov') DESC, is_super_admin DESC, id ASC
		LIMIT 1
	`).Scan(&canonicalID).Error; err != nil {
		return fmt.Errorf("pick canonical super-admin: %w", err)
	}
	if canonicalID == 0 {
		// Пользователей ещё нет (миграция раньше сидов) -- применять нечего.
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		demoted := tx.Exec(`
			UPDATE users SET is_super_admin = false, is_admin = true
			WHERE id <> ? AND is_super_admin = true`, canonicalID)
		if demoted.Error != nil {
			return fmt.Errorf("demote extra super-admins: %w", demoted.Error)
		}
		if demoted.RowsAffected > 0 {
			slog.Warn("demoted extra super-admins to admin",
				"count", demoted.RowsAffected, "kept_super_id", canonicalID)
		}
		if err := tx.Exec(`
			UPDATE users SET is_super_admin = true
			WHERE id = ? AND is_super_admin = false`, canonicalID).Error; err != nil {
			return fmt.Errorf("promote canonical super-admin: %w", err)
		}
		if err := tx.Exec(`
			UPDATE users SET last_name = 'Администратор', first_name = 'Системный'
			WHERE id = ?
			  AND (last_name IS NULL OR last_name = '')
			  AND (first_name IS NULL OR first_name = '')`, canonicalID).Error; err != nil {
			return fmt.Errorf("normalize super-admin name: %w", err)
		}
		return nil
	})
}

// relaxApplicationOrgNotNull снимает NOT NULL с applications.organization_id:
// при подаче заявки достаточно указать организацию ИЛИ компанию, поэтому
// organization_id может быть NULL (company-only подача). Свежий AutoMigrate уже
// создаёт колонку nullable, но на БД из старой "NOT NULL"-эры констрейнт остаётся -
// AutoMigrate существующие колонки не ослабляет. ALTER ... DROP NOT NULL идемпотентен
// (на уже nullable колонке - noop).
func relaxApplicationOrgNotNull(db *gorm.DB) error {
	if err := db.Exec(`ALTER TABLE applications ALTER COLUMN organization_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("relax applications.organization_id NOT NULL: %w", err)
	}
	return nil
}

// widenFactTableHint расширяет system_tables.fact_table_hint с varchar(255) до text.
// Поле редактируется тем же rich-text TextConstructor, что и instruction, - HTML
// форматирования легко переваливает за 255 символов, и запись падает с "value too
// long". AutoMigrate существующие колонки не расширяет, поэтому ALTER явно. Идемпотентен
// (на уже text-колонке - noop).
func widenFactTableHint(db *gorm.DB) error {
	if err := db.Exec(`ALTER TABLE system_tables ALTER COLUMN fact_table_hint TYPE text`).Error; err != nil {
		return fmt.Errorf("widen system_tables.fact_table_hint to text: %w", err)
	}
	return nil
}

// backfillBlacklistNormalized заполняет normalized_number/normalized_fio у записей
// чёрного списка, добавленных до появления этих колонок (#481). Использует ту же
// функцию нормализации, что и сервисы при Create, - иначе эталон в БД разойдётся с
// нормализацией поискового запроса. Идемпотентно: трогает только пустые значения.
func backfillBlacklistNormalized(db *gorm.DB) error {
	var vehicles []models.VehicleBlacklist
	if err := db.Where("normalized_number = '' OR normalized_number IS NULL").Find(&vehicles).Error; err != nil {
		return fmt.Errorf("failed to load vehicle_blacklists for normalization backfill: %w", err)
	}
	for _, v := range vehicles {
		if err := db.Model(&models.VehicleBlacklist{}).Where("id = ?", v.ID).
			Update("normalized_number", normalize.Plate(v.CarNumber)).Error; err != nil {
			return fmt.Errorf("failed to backfill normalized_number for vehicle_blacklist %d: %w", v.ID, err)
		}
	}

	var persons []models.PersonBlacklist
	if err := db.Where("normalized_fio = '' OR normalized_fio IS NULL").Find(&persons).Error; err != nil {
		return fmt.Errorf("failed to load person_blacklists for normalization backfill: %w", err)
	}
	for _, p := range persons {
		middle := ""
		if p.MiddleName != nil {
			middle = *p.MiddleName
		}
		if err := db.Model(&models.PersonBlacklist{}).Where("id = ?", p.ID).
			Update("normalized_fio", normalize.Name(p.LastName, p.FirstName, middle)).Error; err != nil {
			return fmt.Errorf("failed to backfill normalized_fio for person_blacklist %d: %w", p.ID, err)
		}
	}

	if len(vehicles) > 0 || len(persons) > 0 {
		slog.Info("backfilled blacklist normalized fields", "vehicles", len(vehicles), "persons", len(persons))
	}
	return nil
}

// fixAttachmentTemplateIndex заменяет UNIQUE индекс на обычный, если GORM
// создал его при AutoMigrate для belongs-to связи. Несколько шаблонов на
// одно вложение - штатный сценарий (#183).
func fixAttachmentTemplateIndex(db *gorm.DB) error {
	return db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE tablename = 'attachment_templates'
				  AND indexname = 'idx_attachment_templates_unique_attachment_id'
				  AND indexdef LIKE '%UNIQUE%'
			) THEN
				DROP INDEX idx_attachment_templates_unique_attachment_id;
				CREATE INDEX idx_attachment_templates_unique_attachment_id
					ON attachment_templates(unique_attachment_id);
			END IF;
		END $$;
	`).Error
}

// installSQLFunctions создаёт пользовательские SQL-функции, переиспользуемые
// в запросах сервисов. CREATE OR REPLACE безопасен при каждом старте.
func installSQLFunctions(db *gorm.DB) error {
	const formatShortName = `
CREATE OR REPLACE FUNCTION format_short_name(p_last TEXT, p_first TEXT, p_middle TEXT)
RETURNS TEXT AS $$
BEGIN
    IF COALESCE(p_last, '') = '' THEN
        RETURN TRIM(BOTH ' ' FROM
            COALESCE(p_first, '') ||
            CASE WHEN COALESCE(p_middle, '') <> '' THEN ' ' || p_middle ELSE '' END
        );
    END IF;
    RETURN p_last ||
        CASE WHEN COALESCE(p_first, '') <> '' THEN ' ' || LEFT(p_first, 1) || '.' ELSE '' END ||
        CASE WHEN COALESCE(p_middle, '') <> '' THEN LEFT(p_middle, 1) || '.' ELSE '' END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
`
	const formatFullName = `
CREATE OR REPLACE FUNCTION format_full_name(p_last TEXT, p_first TEXT, p_middle TEXT)
RETURNS TEXT AS $$
BEGIN
    RETURN TRIM(BOTH ' ' FROM
        COALESCE(p_last, '') ||
        CASE WHEN COALESCE(p_first, '') <> '' THEN ' ' || p_first ELSE '' END ||
        CASE WHEN COALESCE(p_middle, '') <> '' THEN ' ' || p_middle ELSE '' END
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;
`
	if err := db.Exec(formatShortName).Error; err != nil {
		return err
	}
	if err := db.Exec(formatFullName).Error; err != nil {
		return err
	}
	slog.Info("SQL functions installed: format_short_name, format_full_name")
	return nil
}

// installExtensions включает Postgres-расширения для нечёткого поиска возможного обхода
// ЧС (#481): pg_trgm (триграммные similarity/word_similarity по ФИО) и fuzzystrmatch
// (levenshtein по госномерам). CREATE EXTENSION IF NOT EXISTS идемпотентен и безопасен при
// каждом старте; оба расширения trusted в PG13+, поэтому ставятся владельцем БД без
// суперпользователя. Ставим до AutoMigrate: инфраструктура не зависит от таблиц, а
// FindSimilar-запросы сервисов зависят от расширений.
func installExtensions(db *gorm.DB) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`).Error; err != nil {
		return fmt.Errorf("failed to install pg_trgm extension: %w", err)
	}
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS fuzzystrmatch`).Error; err != nil {
		return fmt.Errorf("failed to install fuzzystrmatch extension: %w", err)
	}
	slog.Info("SQL extensions installed: pg_trgm, fuzzystrmatch")
	return nil
}

// baseRoleGrants -- дефолтный набор прав базовой роли "Пользователь" (ТЗ).
// Строки синхронизированы с ключами permission_catalog.go (избегаем import services,
// чтобы не создавать цикл database->services).
var baseRoleGrants = []string{
	"page.new_application",
	"page.employees",
	"page.cars",
	"page.news",
	"page.personal_cabinet",
	"header.create_application",
	"entity.employees.read",
	"entity.employees.write",
	"entity.employees.delete",
	"entity.cars.read",
	"entity.cars.write",
	"entity.cars.delete",
	"section.registry.organization",
	"section.registry.company",
	"guide.user",
	"detail.open_application",
	"detail.documents",
	// detail.full_history и detail.entry_exit_history НЕ выдаём базовой роли: рядовой
	// пользователь не видит "Полную историю" и "Историю проходов" даже у своих
	// сущностей (админ/супер видят по флагу adminAll/allowAll). Снятие со старых БД -
	// revokeBaseRoleHistoryGrants.
}

// seedBaseRole создаёт неудаляемую базовую роль "Пользователь" и её дефолтные
// grants (идемпотентно). Это фундамент, от которого наследуются новые роли.
func seedBaseRole(db *gorm.DB) error {
	var role models.Role
	err := db.Where("code = ?", "user").First(&role).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		role = models.Role{Code: "user", Name: "Пользователь", IsSystem: true}
		if err := db.Create(&role).Error; err != nil {
			return fmt.Errorf("failed to seed base role: %w", err)
		}
		slog.Info("seeded base role 'Пользователь'")
	case err != nil:
		return fmt.Errorf("failed to check base role: %w", err)
	case !role.IsSystem:
		if err := db.Model(&models.Role{}).Where("id = ?", role.ID).Update("is_system", true).Error; err != nil {
			return fmt.Errorf("failed to mark base role system: %w", err)
		}
	}

	for _, key := range baseRoleGrants {
		grant := models.RolePermissionGrant{RoleID: role.ID, PermissionKey: key, Value: "allow"}
		if err := db.Where("role_id = ? AND permission_key = ?", role.ID, key).
			FirstOrCreate(&grant).Error; err != nil {
			return fmt.Errorf("failed to seed base role grant %s: %w", key, err)
		}
	}
	return nil
}

// revokeBaseRoleHistoryGrants снимает с базовой роли "Пользователь" права
// detail.full_history и detail.entry_exit_history. Их убрали из baseRoleGrants
// (рядовой юзер не должен видеть "Полную историю"/"Историю проходов"), но на уже
// засеянных БД грант остался - seedBaseRole только добавляет. Удаляем разово; delete
// идемпотентен (повторный прогон снимает 0 строк). Затрагивает ТОЛЬКО системную роль
// "user" - гранты кастомных ролей не трогаем. Админ/супер видят историю по флагу.
func RevokeBaseRoleHistoryGrants(db *gorm.DB) error {
	var role models.Role
	if err := db.Where("code = ? AND is_system = ?", "user", true).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // базовая роль ещё не засеяна
		}
		return fmt.Errorf("load base role for history grant revoke: %w", err)
	}
	keys := []string{"detail.full_history", "detail.entry_exit_history"}
	res := db.Where("role_id = ? AND permission_key IN ?", role.ID, keys).
		Delete(&models.RolePermissionGrant{})
	if res.Error != nil {
		return fmt.Errorf("revoke base role history grants: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		slog.Info("revoked history grants from base role", "grants_removed", res.RowsAffected)
	}
	return nil
}

// BackfillBaseRole назначает базовую роль "Пользователь" существующим обычным
// пользователям без роли (role_id IS NULL, не супер-админ). После #187 (Фаза 2)
// навигация и доступ гейтятся правами, а права обычного юзера приходят от роли:
// без роли резолвер отдаёт пустой набор, и юзер увидел бы пустое меню. Супер-админа
// не трогаем (его доступ - allowAll по флагу, роль не нужна). Идемпотентно - берёт
// только role_id IS NULL.
func BackfillBaseRole(db *gorm.DB) error {
	var role models.Role
	if err := db.Where("code = ? AND is_system = ?", "user", true).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // базовая роль ещё не засеяна - нечего назначать
		}
		return fmt.Errorf("load base role for backfill: %w", err)
	}
	res := db.Exec(`
		UPDATE users SET role_id = ?
		WHERE role_id IS NULL AND is_super_admin = false`, role.ID)
	if res.Error != nil {
		return fmt.Errorf("backfill base role: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		slog.Info("backfilled base role for users without role",
			"users_updated", res.RowsAffected, "role_id", role.ID)
	}
	return nil
}

// backfillAdminFromType разово переносит админство с типа на флаг: пользователи
// типа "manager" (кроме супер-админа) становятся is_admin=true. После этого
// авторизация идёт по флагу, а не по user_type (см. эпик прав). Идемпотентно.
func backfillAdminFromType(db *gorm.DB) error {
	res := db.Exec(`
		UPDATE users SET is_admin = true
		WHERE is_admin = false AND is_super_admin = false
		  AND type_id IN (SELECT id FROM user_types WHERE code = 'manager')
	`)
	if res.Error != nil {
		return fmt.Errorf("backfill is_admin from type manager: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		slog.Info("is_admin backfilled from type manager", "users_updated", res.RowsAffected)
	}
	return nil
}

// Seed inserts initial data if tables are empty.
func Seed(db *gorm.DB) error {
	// Миграция: переводим заявки "Непрочитано" -> "В обработке"
	if result := db.Model(&models.Application{}).
		Where("status = ?", models.StatusUnread).
		Update("status", models.StatusProcessing); result.Error != nil {
		slog.Error("failed to migrate unread status", "error", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Info("migrated unread applications to processing", "count", result.RowsAffected)
	}

	// Миграция: переводим feedback "Нерешено" -> "Не решено" (статус с пробелом)
	if result := db.Model(&models.Feedback{}).
		Where("status = ?", "Нерешено").
		Update("status", models.FeedbackOpen); result.Error != nil {
		slog.Error("failed to migrate feedback status", "error", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Info("migrated feedback status to spaced form", "count", result.RowsAffected)
	}

	var count int64
	db.Model(&models.UserType{}).Count(&count)
	if count == 0 {
		slog.Info("seeding user_types")
		types := []models.UserType{
			{Name: "Пользователь", Code: "user", IsSystem: true},
			{Name: "Арендатор", Code: "renter", IsSystem: true},
			{Name: "Подрядчик", Code: "contractor", IsSystem: true},
			{Name: "Охранник", Code: "security", IsSystem: true},
			{Name: "Руководитель", Code: "manager", IsSystem: true},
			{Name: "Бюро пропусков", Code: "buropropuskov", IsSystem: true},
		}
		if err := db.Create(&types).Error; err != nil {
			return err
		}
	}

	// Backfill is_system для уже засеянных БД (staging/prod): эти code используются
	// в авторизации (internal/auth/permissions.go), их нельзя удалять/переименовывать.
	// Идемпотентно: повторный прогон не меняет уже помеченные строки.
	if result := db.Model(&models.UserType{}).
		Where("code IN ? AND is_system = ?",
			[]string{"user", "renter", "contractor", "security", "manager", "buropropuskov"}, false).
		Update("is_system", true); result.Error != nil {
		slog.Error("failed to backfill user_types.is_system", "error", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Info("backfilled user_types.is_system", "count", result.RowsAffected)
	}

	// Разделы руководства (B1): дефолтный контент-черновик по ролям. PDF грузится
	// админом отдельно, поэтому file-поля пустые (в ответе file:null). Идемпотентно:
	// сидим только когда таблица пуста, тексты в DB после этого правятся не сидом.
	var guideCount int64
	db.Model(&models.GuideSection{}).Count(&guideCount)
	if guideCount == 0 {
		slog.Info("seeding guide_sections")
		jsonItems := func(items []string) json.RawMessage {
			b, _ := json.Marshal(items)
			return b
		}
		sections := []models.GuideSection{
			{
				Role:      "user",
				SortOrder: 1,
				Title:     "Руководство пользователя",
				Lead:      "Для сотрудников организаций, которые подают заявки на въезд людей и автомобилей.",
				Items: jsonItems([]string{
					"Подача заявки: типы вложений (автозаявка, проведение работ, разовый пропуск), период и время прохода",
					"Добавление сотрудников и автомобилей, выбор места разгрузки и таблицы прохода",
					"Контроль статуса заявок в Центре заявок, согласующие, копирование номера",
					"Личный кабинет: данные организации, уведомления, список заявок",
				}),
			},
			{
				Role:      "guard",
				SortOrder: 2,
				Title:     "Руководство охранника",
				Lead:      "Для сотрудников охраны на постах: проверка пропусков и отметки въезда/выезда.",
				Items: jsonItems([]string{
					"Раздел «Доступные мне»: поиск, фильтры и сортировка по постам",
					"Отметка въезда и выезда транспорта и людей",
					"Таблицы постов и мест прохода, расписание работы",
					"Действия при несоответствии данных заявки",
				}),
			},
			{
				Role:      "admin",
				SortOrder: 3,
				Title:     "Руководство администратора",
				Lead:      "Для администраторов системы: управление справочниками, пользователями и правами.",
				Items: jsonItems([]string{
					"Учётные записи: создание пользователей, пароли, привязка к организациям",
					"Организации, компании, отделы; места разгрузки и расписания",
					"Конструктор таблиц (посты и места прохода), форматы номеров, отметки",
					"Документы и руководства, объявления и новости",
				}),
			},
		}
		if err := db.Create(&sections).Error; err != nil {
			return fmt.Errorf("failed to seed guide sections: %w", err)
		}
	}

	// Organizations: глобальный uniqueIndex по name заменяем на partial unique
	// (только среди активных), чтобы архивная организация не блокировала создание
	// новой активной с тем же именем (#412). Идемпотентно. Ошибку возвращаем (не
	// логируем молча): от этого индекса зависит уникальность имён - провал должен
	// быть громким.
	if err := db.Exec("DROP INDEX IF EXISTS idx_organizations_name").Error; err != nil {
		return fmt.Errorf("failed to drop idx_organizations_name: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_organizations_name_active ON organizations (name) WHERE is_active = true").Error; err != nil {
		return fmt.Errorf("failed to create partial unique index on organizations.name: %w", err)
	}

	// Companies: то же, что и для organizations (#412).
	if err := db.Exec("DROP INDEX IF EXISTS idx_companies_name").Error; err != nil {
		return fmt.Errorf("failed to drop idx_companies_name: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_companies_name_active ON companies (name) WHERE is_active = true").Error; err != nil {
		return fmt.Errorf("failed to create partial unique index on companies.name: %w", err)
	}

	// Marks: тот же баг, что и #412 - глобальный uniqueIndex по name не давал
	// пересоздать активную марку при наличии архивной с тем же именем (#FF-marks).
	if err := db.Exec("DROP INDEX IF EXISTS idx_marks_name").Error; err != nil {
		return fmt.Errorf("failed to drop idx_marks_name: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_marks_name_active ON marks (name) WHERE is_active = true").Error; err != nil {
		return fmt.Errorf("failed to create partial unique index on marks.name: %w", err)
	}

	// Чёрный список машин: уникальность (car_number, mark_id) только среди активных
	// записей (#443), чтобы снятая запись не мешала повторно добавить ту же машину.
	// Индекс по LOWER(TRIM(car_number)) - чтобы инвариант совпадал с регистронезависимым
	// матчем в Check/каскаде (иначе "A123"/"a123" - два активных дубля). DROP+CREATE,
	// чтобы переопределить старую (raw-column) версию индекса. Идемпотентно. Ошибку
	// возвращаем громко - от индекса зависит инвариант дублей.
	if err := db.Exec("DROP INDEX IF EXISTS uidx_vehicle_blacklists_active").Error; err != nil {
		return fmt.Errorf("failed to drop uidx_vehicle_blacklists_active: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_vehicle_blacklists_active ON vehicle_blacklists (LOWER(TRIM(car_number)), mark_id) WHERE is_active = true").Error; err != nil {
		return fmt.Errorf("failed to create partial unique index on vehicle_blacklists: %w", err)
	}

	// Чёрный список людей: уникальность ФИО только среди активных (#443). COALESCE по
	// отчеству - иначе NULL/пустое отчество плодит дубли (в обычном unique index NULL-ы
	// считаются различными). Нормализация LOWER(TRIM) совпадает с Check/каскадом.
	if err := db.Exec("DROP INDEX IF EXISTS uidx_person_blacklists_active").Error; err != nil {
		return fmt.Errorf("failed to drop uidx_person_blacklists_active: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_person_blacklists_active ON person_blacklists (LOWER(TRIM(last_name)), LOWER(TRIM(first_name)), LOWER(TRIM(COALESCE(middle_name, '')))) WHERE is_active = true").Error; err != nil {
		return fmt.Errorf("failed to create partial unique index on person_blacklists: %w", err)
	}

	// Бэкфилл нормализованных полей ЧС (#481): у записей, добавленных до появления
	// колонок normalized_*, значение пустое. Заполняем той же функцией, что и при
	// Create, иначе нечёткий поиск (#481) не найдёт старые записи. Идемпотентно -
	// берём только пустые. Ошибку возвращаем громко: расхождение нормализации тихо
	// сломало бы поиск возможного обхода ЧС.
	if err := backfillBlacklistNormalized(db); err != nil {
		return err
	}

	// Seed default tab permissions
	if db.Migrator().HasTable("permissions") {
		var permCount int64
		db.Model(&models.Permission{}).Count(&permCount)
		if permCount == 0 {
			slog.Info("seeding default permissions")
			permissions := []models.Permission{
				{Key: "tab.cars.view", Category: "tab", DisplayName: "Автомобили"},
				{Key: "tab.employees.view", Category: "tab", DisplayName: "Сотрудники"},
				{Key: "tab.overview.view", Category: "tab", DisplayName: "Обзор и новости"},
				{Key: "tab.profile.view", Category: "tab", DisplayName: "ЛК"},
			}
			if err := db.Create(&permissions).Error; err != nil {
				return err
			}
		}
	}

	// Базовая роль "Пользователь" с дефолтными правами (фундамент новой системы прав).
	if err := seedBaseRole(db); err != nil {
		return err
	}

	// Снять с базовой роли права на полную историю/историю проходов (рядовой юзер их
	// не видит; админ/супер - по флагу). Разовое снятие для уже засеянных БД.
	if err := RevokeBaseRoleHistoryGrants(db); err != nil {
		return err
	}

	// Разовый перенос админства с типа manager на флаг is_admin.
	if err := backfillAdminFromType(db); err != nil {
		return err
	}

	// Обычным юзерам без роли выдаём базовую "Пользователь" (#187 Фаза 2): без роли
	// резолвер отдаёт пустые права, и гейтинг навигации скрыл бы все вкладки.
	if err := BackfillBaseRole(db); err != nil {
		return err
	}

	// Разовый перенос замороженных *_history строк в общий audit_log (#870, финал):
	// читатели сущностей переведены на audit_log-only, до-cutover история берётся уже
	// из audit_log, а legacy-таблицы затем дропаются DropBackfilledLegacyHistory (ниже).
	if err := BackfillAuditFromLegacy(db); err != nil {
		return err
	}

	// Дроп замороженных legacy *_history таблиц (#870, F.8) - строго ПОСЛЕ backfill:
	// он уже перенёс их строки в audit_log, читатели на audit_log-only, таблицы стали
	// мёртвым бэкапом. Дропаются только подтверждённо-перенесённые (гард-флаг).
	if err := DropBackfilledLegacyHistory(db); err != nil {
		return err
	}

	return nil
}

// auditBackfillSource описывает разовый перенос одной замороженной *_history таблицы
// в общий audit_log (#870, финал). projection - список колонок/выражений источника
// в порядке (entity_id, action, actor_user_id, details, created_at); вместе с
// литералом entity_type он проецируется ровно в колонки INSERT внутри
// BackfillAuditFromLegacy. Для сущностей с плоской схемой (mark old/new, approver
// approver_name, car field_name/...) слот details - это jsonb_build_object(...) в той
// же форме, что пишет recorder сущности, иначе read-switch вернёт не ту историю
// (стережёт golden-тест). ORDER BY created_at, id даёт новым id audit_log тот же
// относительный порядок, что был в legacy (тайбрейкер истории - created_at DESC, id DESC).
//
// ВАЖНО: table и projection конкатенируются в SQL сырыми (не bind-параметры) - это
// допустимо ТОЛЬКО потому, что значения статические литералы из auditBackfillSources().
// Никогда не подставлять сюда данные из запроса/БД/внешнего источника - будет инъекция.
type auditBackfillSource struct {
	entity     string // models.AuditEntity* - и суффикс гард-флага
	table      string // legacy-таблица-источник (probe to_regclass: могла быть дропнута в F.8)
	projection string // entity_id, action, actor_user_id, details, created_at (статические SQL-выражения)
}

// auditBackfillSources - сущности, чьи читатели переведены на audit_log-only
// (#870, финал). Пополняется по мере срезов F.1 (citizenship) -> F.7 (application).
func auditBackfillSources() []auditBackfillSource {
	return []auditBackfillSource{
		{
			entity:     models.AuditEntityCitizenship,
			table:      "citizenship_histories",
			projection: "citizenship_id, action_type, actor_user_id, details, created_at",
		},
		// F.2 ref-батч A: чистые зеркала pilot - details уже jsonb, переносится verbatim.
		{
			entity:     models.AuditEntityCompany,
			table:      "company_histories",
			projection: "company_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityOrganization,
			table:      "organization_histories",
			projection: "organization_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityUserType,
			table:      "user_type_histories",
			projection: "user_type_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityLicensePlateFormat,
			table:      "license_plate_format_histories",
			projection: "format_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityUnloadPlace,
			table:      "unload_place_histories",
			projection: "unload_place_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityUniqueAttachment,
			table:      "unique_attachment_histories",
			projection: "unique_attachment_id, action_type, actor_user_id, details, created_at",
		},
		// F.3 ref-батч B (спец-форма details): user/system_table - details уже jsonb
		// (verbatim); approver/mark/trash - плоские колонки сворачиваются в details в ту
		// же форму, что пишет recorder сущности (иначе read-switch вернёт не ту историю,
		// стережёт golden). actor у этой пятёрки: user/approver - actor_user_id,
		// mark/system_table/trash - user_id.
		{
			entity:     models.AuditEntityUser,
			table:      "user_histories",
			projection: "target_user_id, action_type, actor_user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityApprover,
			table:      "application_approver_histories",
			projection: "approver_user_id, action_type, actor_user_id, jsonb_build_object('approver_name', approver_name), created_at",
		},
		{
			entity:     models.AuditEntityMark,
			table:      "mark_histories",
			projection: "mark_id, action_type, user_id, jsonb_build_object('old_value', old_value, 'new_value', new_value), created_at",
		},
		{
			entity:     models.AuditEntitySystemTable,
			table:      "system_table_histories",
			projection: "system_table_id, action_type, user_id, details, created_at",
		},
		{
			entity:     models.AuditEntitySystemTableTrash,
			table:      "system_table_trash_histories",
			projection: "system_table_id, action_type, user_id, jsonb_build_object('affected_count', affected_count, 'items', COALESCE(details, '[]'::jsonb)), created_at",
		},
		// F.4 blacklist + unique: blacklist - details уже jsonb (каскад reason/cars/
		// employees переносится verbatim), actor в плоской колонке user_id; unique_car/
		// unique_employee - плоские field_name/old/new/comment сворачиваются в details в
		// форме carAuditDetails (как пишет recorder), иначе read-switch вернёт не ту
		// историю (стережёт golden Test*_GetHistory_ReturnsRecords).
		{
			entity:     models.AuditEntityPersonBlacklist,
			table:      "person_blacklist_histories",
			projection: "entity_id, action_type, user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityVehicleBlacklist,
			table:      "vehicle_blacklist_histories",
			projection: "entity_id, action_type, user_id, details, created_at",
		},
		{
			entity:     models.AuditEntityUniqueCar,
			table:      "unique_cars_history",
			projection: "unique_car_id, action_type, user_id, jsonb_build_object('field_name', field_name, 'old_value', old_value, 'new_value', new_value, 'comment', comment), created_at",
		},
		{
			entity:     models.AuditEntityUniqueEmployee,
			table:      "unique_employees_history",
			projection: "unique_employee_id, action_type, user_id, jsonb_build_object('field_name', field_name, 'old_value', old_value, 'new_value', new_value, 'comment', comment), created_at",
		},
		// F.5 car: cars_history - общий журнал событий машины (история/корзина/статус/
		// статистика/отчёты). Плоские поля сворачиваются в details в форму carAuditDetails
		// (field_name/old/new/comment/table_id), как пишет recorder. metadata - отдельным
		// ключом ТОЛЬКО когда колонка не NULL: читатель берёт его через details->'metadata'
		// (jsonb, не ->>'), и пустой ключ дал бы jsonb 'null' вместо отсутствия, разойдясь
		// с recorder (omitempty опускает ключ). table_id/прочие читаются через ->>' и
		// JSON-null = отсутствию ключа, поэтому их CASE не нужен. Стережёт golden
		// TestCars_History_BackfillLegacyIntoAudit + analytics-тесты.
		{
			entity: models.AuditEntityCar,
			table:  "cars_history",
			projection: "car_id, action_type, user_id, " +
				"jsonb_build_object('field_name', field_name, 'old_value', old_value, 'new_value', new_value, 'comment', comment, 'table_id', table_id) " +
				"|| CASE WHEN metadata IS NOT NULL THEN jsonb_build_object('metadata', metadata) ELSE '{}'::jsonb END, " +
				"created_at",
		},
		// F.6 employee: employees_history - колонка-в-колонку зеркало cars_history
		// (общий журнал событий сотрудника). Та же проекция в carAuditDetails-форму,
		// metadata тем же CASE (см. F.5 car выше). Стережёт golden
		// TestEmployees_History_BackfillLegacyIntoAudit + analytics-тесты.
		{
			entity: models.AuditEntityEmployee,
			table:  "employees_history",
			projection: "employee_id, action_type, user_id, " +
				"jsonb_build_object('field_name', field_name, 'old_value', old_value, 'new_value', new_value, 'comment', comment, 'table_id', table_id) " +
				"|| CASE WHEN metadata IS NOT NULL THEN jsonb_build_object('metadata', metadata) ELSE '{}'::jsonb END, " +
				"created_at",
		},
		// F.7 application: НЕ зеркало car/employee - другая плоская схема. Поля
		// action_status/old/new/comment сворачиваются в details (читатель берёт их через
		// ->>', где JSON-null == отсутствию ключа, поэтому CASE им не нужен; нет
		// field_name/table_id. metadata - вложенным jsonb-объектом ТОЛЬКО когда колонка не
		// NULL (тот же квирк, что у car/employee: читатель берёт details->'metadata' через
		// ->, и {"metadata":null} дал бы jsonb-null вместо отсутствия, разойдясь с recorder
		// omitempty). actor=user_id; квирк assigned_responsible/assigned_viewer (user_id =
		// назначенный, не действующий) переносится как есть. action_user_id - мёртвая
		// колонка (нигде не писалась/читалась), не переносится. Стережёт golden
		// TestApplications_HistoryGolden_* + TestApplications_History_BackfillLegacyIntoAudit.
		{
			entity: models.AuditEntityApplication,
			table:  "application_history",
			projection: "application_id, action_type, user_id, " +
				"jsonb_build_object('action_status', action_status, 'old_value', old_value, 'new_value', new_value, 'comment', comment) " +
				"|| CASE WHEN metadata IS NOT NULL THEN jsonb_build_object('metadata', metadata) ELSE '{}'::jsonb END, " +
				"created_at",
		},
	}
}

// BackfillAuditFromLegacy один раз на сущность переносит замороженные *_history строки
// в общий audit_log (#870, финал). До перевода читателя на audit_log-only история
// бралась union'ом legacy+audit_log; backfill копирует legacy-часть в audit_log, после
// чего читатель сущности читает только audit_log, а legacy-таблица дропается
// DropBackfilledLegacyHistory (F.8, тот же Seed, строго после этого переноса).
//
// Идемпотентно: гард-флаг system_settings 'audit_backfilled:<entity>' ставится в той же
// транзакции, что и INSERT, поэтому повторный старт пропускает перенос и дублей не
// создаёт. При параллельном старте двух инстанций оба могут пройти проверку флага, но
// unique constraint на system_settings.key даёт зафиксироваться ровно одной транзакции
// - вторая откатится вместе со своими INSERT'ами, дублей в audit_log не остаётся.
// to_regclass-probe делает перенос no-op, если legacy-таблица уже дропнута (F.8).
// Зовётся из Seed (каждый старт сервера) после AutoMigrate - к первому запросу истории
// audit_log уже наполнен.
func BackfillAuditFromLegacy(db *gorm.DB) error {
	for _, src := range auditBackfillSources() {
		flag := "audit_backfilled:" + src.entity
		var flagged int64
		if err := db.Model(&models.SystemSetting{}).Where("key = ?", flag).Count(&flagged).Error; err != nil {
			return fmt.Errorf("backfill audit %s: check flag: %w", src.entity, err)
		}
		if flagged > 0 {
			continue
		}

		var sourceExists bool
		if err := db.Raw("SELECT to_regclass(?) IS NOT NULL", src.table).Scan(&sourceExists).Error; err != nil {
			return fmt.Errorf("backfill audit %s: probe %s: %w", src.entity, src.table, err)
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			var moved int64
			if sourceExists {
				res := tx.Exec(
					"INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, details, created_at) "+
						"SELECT ?, "+src.projection+" FROM "+src.table+" ORDER BY created_at, id",
					src.entity)
				if res.Error != nil {
					return res.Error
				}
				moved = res.RowsAffected
			}
			if err := tx.Create(&models.SystemSetting{Key: flag, Value: "1", Type: "bool"}).Error; err != nil {
				return err
			}
			// Логируем всегда (в т.ч. rows=0 на свежей установке) - оператору видно,
			// что backfill отработал, а не молча не дошёл сюда.
			slog.Info("audit backfill done", "entity", src.entity, "rows", moved)
			return nil
		}); err != nil {
			return fmt.Errorf("backfill audit %s: %w", src.entity, err)
		}
	}
	return nil
}

// DropBackfilledLegacyHistory дропает замороженные legacy *_history таблицы (#870, F.8)
// ПОСЛЕ того как BackfillAuditFromLegacy перенёс их строки в audit_log, а все читатели
// переведены на audit_log-only. Дропается только таблица, чей перенос подтверждён гард-
// флагом system_settings 'audit_backfilled:<entity>' (его backfill ставит в одной
// транзакции с INSERT - флаг есть ⟺ строки скопированы), иначе таблицу с ещё-не-
// перенесённой до-cutover историей оставляем нетронутой. Зовётся из Seed строго ПОСЛЕ
// BackfillAuditFromLegacy: на первом деплое (backfill+drop разом) копирование
// фиксируется до дропа. Идемпотентно (DROP TABLE IF EXISTS - повторный старт no-op).
// Источник списка - auditBackfillSources() (ровно 19 переведённых сущностей;
// application_status_history и user_ban_histories в него не входят и НЕ дропаются).
//
// ВАЖНО: src.table конкатенируется в DDL сырым - допустимо ТОЛЬКО потому, что это
// статический литерал из auditBackfillSources(), не данные извне (как и в backfill).
func DropBackfilledLegacyHistory(db *gorm.DB) error {
	for _, src := range auditBackfillSources() {
		flag := "audit_backfilled:" + src.entity
		var flagged int64
		if err := db.Model(&models.SystemSetting{}).Where("key = ?", flag).Count(&flagged).Error; err != nil {
			return fmt.Errorf("drop legacy %s: check flag: %w", src.entity, err)
		}
		if flagged == 0 {
			// Перенос ещё не подтверждён - НЕ дропаем (защита до-cutover истории).
			slog.Warn("skip legacy drop: backfill not confirmed", "entity", src.entity, "table", src.table)
			continue
		}
		if err := db.Exec("DROP TABLE IF EXISTS " + src.table).Error; err != nil {
			return fmt.Errorf("drop legacy %s (%s): %w", src.entity, src.table, err)
		}
	}
	return nil
}
