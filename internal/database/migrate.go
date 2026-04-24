package database

import (
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
		&models.Organization{},
		&models.Company{},
		&models.Citizenship{},
		&models.LicensePlateFormat{},
		&models.UnloadPlace{},
		&models.SystemTable{},

		// Users (depends on UserType, Organization, Company)
		&models.User{},
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
		&models.UniqueEmployee{},

		// Attachments (depends on Application, UniqueAttachment)
		&models.Attachment{},

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

		// Permissions
		&models.Permission{},
		&models.UserPermission{},

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
	slog.Info("AutoMigrate completed")
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
			{Name: "Пользователь", Code: "user"},
			{Name: "Арендатор", Code: "renter"},
			{Name: "Подрядчик", Code: "contractor"},
			{Name: "Охранник", Code: "security"},
			{Name: "Руководитель", Code: "manager"},
			{Name: "Бюро пропусков", Code: "buropropuskov"},
		}
		if err := db.Create(&types).Error; err != nil {
			return err
		}
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
