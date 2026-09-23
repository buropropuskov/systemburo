package repository

// Модульные тесты слоя репозитория с использованием go-sqlmock.
// Базовые тесты CreateUser/GetUser присутствуют в auth_postgres_test.go репозитория.
// Здесь добавлены негативные сценарии и проверки колонок.

import (
	"testing"

	dokkee "github.com/airvt1x/dokkee-backend"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func newTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock
}

// Создание пользователя — happy path.
func TestCreateUser_OK(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	repo := NewAuthPostgres(db)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	id, err := repo.CreateUser(dokkee.User{
		Username: "ivanov", Password: "hashed",
		FirstName: "И", LastName: "И",
		Email: "ivanov@x.y", Phone: "+79991234567",
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Дубликат email → ошибка уникального ограничения.
//
//возможны изменения// — в текущей версии репозиторий возвращает исходную ошибку pq,
// после рефакторинга планируется маппинг на ErrDuplicateEmail.
func TestCreateUser_DuplicateEmail(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	repo := NewAuthPostgres(db)

	mock.ExpectQuery("INSERT INTO users").
		WillReturnError(assert.AnError)

	_, err := repo.CreateUser(dokkee.User{Email: "a@b.c", Phone: "+79991112233"})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Получение пользователя по email+хэш пароля.
func TestGetUser_Found(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	repo := NewAuthPostgres(db)

	rows := sqlmock.NewRows([]string{"id", "username", "email"}).
		AddRow(42, "ivanov", "ivanov@x.y")

	mock.ExpectQuery("SELECT").
		WithArgs("ivanov@x.y", "somehash").
		WillReturnRows(rows)

	u, err := repo.GetUser("ivanov@x.y", "somehash")
	assert.NoError(t, err)
	assert.Equal(t, 42, u.Id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Пользователя нет в базе → ошибка sql.ErrNoRows.
func TestGetUser_NotFound(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	repo := NewAuthPostgres(db)

	mock.ExpectQuery("SELECT").
		WillReturnError(assert.AnError)

	_, err := repo.GetUser("nobody@x.y", "h")
	assert.Error(t, err)
}
