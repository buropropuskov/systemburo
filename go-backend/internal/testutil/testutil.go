package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/handlers"
	"systemburo/internal/router"
	"systemburo/internal/services"
	appvalidator "systemburo/internal/validator"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	TestJWTSecret        = "test-jwt-secret"
	TestJWTRefreshSecret = "test-jwt-refresh-secret"
)

// SetupTestApp creates a fully wired Echo app with real DB, identical to production.
// Returns the Echo instance, DB handle, and a cleanup function.
func SetupTestApp(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()

	db := setupTestDB(t)

	// Run migrations on test DB
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	if err := database.Seed(db); err != nil {
		t.Fatalf("Seed failed: %v", err)
	}

	// Create all services (same wiring as cmd/server/main.go)
	authService := services.NewAuthService(db, TestJWTSecret, TestJWTRefreshSecret, 120*time.Minute, 24*time.Hour)
	userTypeService := services.NewUserTypeService(db)
	lpfService := services.NewLicensePlateFormatService(db)
	attachmentService := services.NewAttachmentService(db)
	citizenshipService := services.NewCitizenshipService(db)
	organizationService := services.NewOrganizationService(db)
	companyService := services.NewCompanyService(db)
	userService := services.NewUserService(db)
	unloadPlaceService := services.NewUnloadPlaceService(db, "./uploads")
	carService := services.NewCarService(db)
	employeeService := services.NewEmployeeService(db)
	permissionService := services.NewPermissionService(db)
	systemTableService := services.NewSystemTableService(db, "./uploads", permissionService)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	feedbackService := services.NewFeedbackService(db)
	applicationService := services.NewApplicationService(db)
	approverService := services.NewApproverService(db)

	// Create all handlers
	authHandler := handlers.NewAuthHandler(authService)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db)
	companyHandler := handlers.NewCompanyHandler(companyService)
	usersHandler := handlers.NewUsersHandler(userService)
	unloadPlaceHandler := handlers.NewUnloadPlaceHandler(unloadPlaceService, "./uploads")
	carHandler := handlers.NewCarHandler(carService)
	employeeHandler := handlers.NewEmployeeHandler(employeeService)
	systemTableHandler := handlers.NewSystemTableHandler(systemTableService)
	uniqueCarHandler := handlers.NewUniqueCarHandler(uniqueCarService)
	uniqueEmployeeHandler := handlers.NewUniqueEmployeeHandler(uniqueEmployeeService)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackService)
	applicationHandler := handlers.NewApplicationHandler(applicationService)
	approverHandler := handlers.NewApproverHandler(approverService)
	permissionHandler := handlers.NewPermissionHandler(permissionService)

	// Setup Echo with routes (no rate limiter, no logger — clean for tests)
	e := echo.New()
	e.HideBanner = true
	e.Validator = appvalidator.New()
	router.Setup(e, authHandler, userTypesHandler, attachmentHandler, lpfHandler,
		citizenshipHandler, organizationHandler, companyHandler, usersHandler,
		unloadPlaceHandler, carHandler, employeeHandler, systemTableHandler,
		uniqueCarHandler, uniqueEmployeeHandler, feedbackHandler,
		applicationHandler, approverHandler, permissionHandler, []byte(TestJWTSecret))

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return e, db, cleanup
}

// CleanDB truncates all tables and re-seeds reference data.
func CleanDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	tables := []string{
		"user_permissions", "permissions",
		"request_log", "request_logs", "notifications", "news", "announcements",
		"feedback", "application_items", "items",
		"employee_target_tables", "employee_files", "application_employees", "employees_history", "employees",
		"car_unload_places", "cars_history", "cars",
		"attachments",
		"unique_employees", "unique_cars", "unique_attachments",
		"application_viewers", "application_approvers", "application_responsible_users",
		"application_status_history", "application_history", "applications",
		"companies_unload_places", "organization_unload_places",
		"unload_place_time_slots", "unload_place_photos", "unload_places",
		"table_fields", "companies_tables", "organization_tables",
		"system_table_time_slots", "system_table_photos", "system_tables",
		"license_plate_format_cells", "license_plate_formats",
		"citizenships",
		"companies_users", "organization_users",
		"refresh_tokens", "users",
		"companies", "organizations", "user_types",
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("CleanDB truncate failed: %v", err)
	}

	// Re-seed user types
	if err := database.Seed(db); err != nil {
		t.Fatalf("CleanDB seed failed: %v", err)
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := getTestDSN()

	// Ensure test database exists
	ensureTestDatabase(t, dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	return db
}

func getTestDSN() string {
	if dsn := os.Getenv("DATABASE_URL_TEST"); dsn != "" {
		return dsn
	}
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		base = "postgres://postgres:123@db/auto_registry"
	}
	return strings.Replace(base, "/auto_registry", "/auto_registry_test", 1)
}

func ensureTestDatabase(t *testing.T, testDSN string) {
	t.Helper()

	// Connect to default postgres database to create test DB
	adminDSN := strings.Replace(testDSN, "/auto_registry_test", "/postgres", 1)
	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect to admin database: %v", err)
	}
	sqlDB, _ := adminDB.DB()
	defer sqlDB.Close()

	// Create test database if not exists
	adminDB.Exec("CREATE DATABASE auto_registry_test")
}
