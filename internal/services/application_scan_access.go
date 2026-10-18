package services

import (
	"context"
	"errors"
	"net/http"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// ApplicationScanAccess отвечает на вопрос раздачи статики: вправе ли вошедший
// пользователь забрать скан заявки, зная только имя файла на диске (#2465).
//
// Имя на диске - UUID, и наружу его не отдаёт ни один ответ API: скачивание идёт
// по идентификатору строки через проверку доступа к заявке. Но знание имени не
// должно заменять эту проверку: адрес файла оседает в журнале запросов
// администратора, в резервной копии и в пакете выгрузки организации, а раздача
// /api/uploads до этого спрашивала только факт входа в систему.
type ApplicationScanAccess struct {
	db   *gorm.DB
	apps ApplicationService
}

// NewApplicationScanAccess собирает гейт поверх той же проверки доступа к заявке,
// какой закрыты остальные её эндпоинты.
func NewApplicationScanAccess(db *gorm.DB, apps ApplicationService) *ApplicationScanAccess {
	return &ApplicationScanAccess{db: db, apps: apps}
}

// CanAccessScan отвечает, открыт ли пользователю файл сканов с таким именем на диске.
//
// Личность берётся по имени учётной записи, а не из полезной нагрузки маркера:
// сюда приходят и запросы с cookie продления сеанса (тег <img> заголовок
// Authorization не шлёт), а в том маркере нет ни идентификатора пользователя, ни
// признака суперадминистратора - есть только subject.
func (a *ApplicationScanAccess) CanAccessScan(ctx context.Context, storedName, username string) (bool, error) {
	if storedName == "" || username == "" {
		return false, nil
	}
	if a.db == nil {
		return false, apperr.New(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
	}

	var file models.ApplicationFile
	err := a.db.WithContext(ctx).
		Select("id, application_id, uploaded_by").
		Where("stored_name = ?", storedName).
		First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, apperr.New(http.StatusServiceUnavailable, "Проверка доступа временно недоступна", err)
	}

	var user models.User
	err = a.db.WithContext(ctx).
		Select("id, username, is_super_admin").
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, apperr.New(http.StatusServiceUnavailable, "Проверка доступа временно недоступна", err)
	}

	// Черновик ещё не привязан к заявке, проверять через неё нечего: файл открыт
	// только тому, кто его загрузил, до самой подачи.
	if file.ApplicationID == nil {
		return file.UploadedBy == user.ID, nil
	}

	return CheckApplicationAccess(ctx, a.apps, *file.ApplicationID, username, user.IsSuperAdmin)
}
