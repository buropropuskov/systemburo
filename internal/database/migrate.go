package database

import (
	"fmt"
	"log/slog"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AllModels returns all GORM models for AutoMigrate.
// Order matters: referenced tables first, then dependents.
func AllModels() []interface{} {
	return []interface{}{
		// Core (no FK dependencies)
		&models.SystemSetting{},
		&models.UserType{},
		&models.UserTypeHistory{},
		&models.Organization{},
		&models.OrganizationHistory{},
		&models.Company{},
		&models.CompanyHistory{},
		&models.Citizenship{},
		&models.LicensePlateFormat{},
		&models.Mark{},
		&models.MarkHistory{},
		&models.UnloadPlace{},
		&models.SystemTable{},
		&models.SystemTableHistory{},

		// Users (depends on UserType, Organization, Company)
		&models.User{},
		&models.UserHistory{},
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

		// Applications (depends on User, Organization, Company)
		&models.Application{},
		&models.ApplicationRead{},
		&models.ApplicationHistory{},
		&models.ApplicationStatusHistory{},
		&models.ApplicationResponsibleUser{},
		&models.ApplicationApprover{},
		&models.ApplicationViewer{},

		// Unique records
		&models.UniqueAttachment{},
		&models.UniqueCar{},
		&models.UniqueCarHistory{},
		&models.UniqueEmployee{},
		&models.UniqueEmployeeHistory{},

		// Attachments (depends on Application, UniqueAttachment)
		&models.Attachment{},

		// Attachment templates (#183: Excel-бланки)
		&models.AttachmentTemplate{},
		&models.AttachmentTemplateMapping{},
		&models.AttachmentCustomField{},
		&models.AttachmentCustomValue{},

		// Cars (depends on Attachment)
		&models.Car{},
		&models.CarHistory{},
		&models.CarUnloadPlace{},

		// Employees (depends on Attachment, Citizenship)
		&models.Employee{},
		&models.EmployeeHistory{},
		&models.ApplicationEmployee{},
		&models.EmployeeFile{},
		&models.EmployeeTargetTable{},

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
		&models.RequestLogs{},
		// Trash history для system_tables (#186)
		&models.SystemTableTrashHistory{},

		// Permissions
		&models.Permission{},
		&models.UserPermission{},

		// New permission system (#229): roles, groups, grants, overrides.
		// PermissionGroup перед Role - RoleDefaultGroup ссылается на обе.
		&models.PermissionGroup{},
		&models.Role{},
		&models.RoleDefaultGroup{},
		&models.PermissionGroupGrant{},
		&models.UserGroup{},
		&models.UserPermissionOverride{},

		// Access denials journal (#230)
		&models.AccessDenial{},
		&models.AccessDenialArchive{},

		// PD consent & audit (152-FZ)
		&models.PDConsent{},
		&models.PDAuditLog{},
	}
}

// AutoMigrate creates/updates all tables from GORM models.
func AutoMigrate(db *gorm.DB) error {
	slog.Info("running AutoMigrate for all models")
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return err
	}
	if err := fixAttachmentTemplateIndex(db); err != nil {
		return err
	}
	if err := installSQLFunctions(db); err != nil {
		return err
	}
	if err := backfillSuperAdmin(db); err != nil {
		return err
	}
	slog.Info("AutoMigrate completed")
	return nil
}

// backfillSuperAdmin проставляет is_super_admin=true пользователям с type_id,
// соответствующим коду 'buropropuskov' в user_types (#231).
// Безопасна для повторного запуска: WHERE not (already true) делает её noop.
// После полного отказа от type_id=6 проверки эту миграцию можно удалить.
func backfillSuperAdmin(db *gorm.DB) error {
	res := db.Exec(`
		UPDATE users SET is_super_admin = true
		WHERE is_super_admin = false
		  AND type_id IN (SELECT id FROM user_types WHERE code = 'buropropuskov')
	`)
	if res.Error != nil {
		return fmt.Errorf("backfill is_super_admin: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		slog.Info("super-admin backfilled", "users_updated", res.RowsAffected)
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

	return nil
}
