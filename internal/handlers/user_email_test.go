package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

// TestUserEmail_RejectsMalformed: адрес с опечаткой не должен сохраняться. До
// #1908 он уходил в базу как есть, и обнаруживалось это только тем, что человеку
// не приходят письма.
func TestUserEmail_RejectsMalformed(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_format_user", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	for _, bad := range []string{"ivanov@", "@example.org", "просто текст", "ivanov@@example.org"} {
		t.Run(bad, func(t *testing.T) {
			err := svc.UpdateInfo(context.Background(), 0, "email_format_user",
				models.UpdateUserInfoRequest{Email: strPtr(bad)})
			require.Error(t, err, "адрес %q должен быть отклонён", bad)
		})
	}

	// Правильный адрес проходит и сохраняется без окружающих пробелов.
	require.NoError(t, svc.UpdateInfo(context.Background(), 0, "email_format_user",
		models.UpdateUserInfoRequest{Email: strPtr("  ivanov@example.org  ")}))

	var saved models.User
	require.NoError(t, db.Where("username = ?", "email_format_user").First(&saved).Error)
	require.NotNil(t, saved.Email)
	assert.Equal(t, "ivanov@example.org", *saved.Email)
}

// TestUserEmail_RejectsNamedForm: «Иванов <ivanov@example.org>» формально
// разбирается как адрес, но в карточке нужен голый ящик - иначе он не совпадёт
// сам с собой при проверке на дубль и уедет в поле «Кому» письма целиком.
func TestUserEmail_RejectsNamedForm(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_named_user", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	err := svc.UpdateInfo(context.Background(), 0, "email_named_user",
		models.UpdateUserInfoRequest{Email: strPtr("Иванов <ivanov@example.org>")})
	assert.Error(t, err)
}

// TestUserEmail_RejectsDuplicate: один ящик на двоих означает, что пароль одного
// работника придёт другому. Сравнение регистронезависимое - на почтовых службах
// адреса, различающиеся регистром, ведут в один ящик.
func TestUserEmail_RejectsDuplicate(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_owner", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: strPtr("shared@example.org"),
	}))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_neighbour", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	err := svc.UpdateInfo(context.Background(), 0, "email_neighbour",
		models.UpdateUserInfoRequest{Email: strPtr("shared@example.org")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "уже указан")

	err = svc.UpdateInfo(context.Background(), 0, "email_neighbour",
		models.UpdateUserInfoRequest{Email: strPtr("SHARED@Example.org")})
	require.Error(t, err, "различие в регистре не делает адрес свободным")

	// Создание с занятым адресом тоже отклоняется.
	err = svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_third", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: strPtr("shared@example.org"),
	})
	assert.Error(t, err)
}

// TestUserEmail_KeepsOwnAddress: сохранение карточки без изменения адреса не
// должно спотыкаться о собственный адрес работника.
func TestUserEmail_KeepsOwnAddress(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_self_keep", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: strPtr("keep@example.org"),
	}))

	require.NoError(t, svc.UpdateInfo(context.Background(), 0, "email_self_keep",
		models.UpdateUserInfoRequest{Email: strPtr("keep@example.org"), Position: strPtr("Инженер")}))

	var saved models.User
	require.NoError(t, db.Where("username = ?", "email_self_keep").First(&saved).Error)
	require.NotNil(t, saved.Email)
	assert.Equal(t, "keep@example.org", *saved.Email)
}

// TestUserEmail_AllowsEmpty: почта не обязательна. Работник без неё просто не
// попадёт в плановую смену паролей, и администратор увидит это в отчёте.
func TestUserEmail_AllowsEmpty(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewUserService(db, services.NewNotificationService(db))
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_empty_user", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: strPtr("temp@example.org"),
	}))

	// Пустая строка очищает адрес, как и прежде.
	require.NoError(t, svc.UpdateInfo(context.Background(), 0, "email_empty_user",
		models.UpdateUserInfoRequest{Email: strPtr("")}))

	var saved models.User
	require.NoError(t, db.Where("username = ?", "email_empty_user").First(&saved).Error)
	require.NotNil(t, saved.Email)
	assert.Empty(t, *saved.Email)

	// Двое без адреса не считаются дублем друг друга.
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "email_empty_second", Password: "password12345678", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))
}
