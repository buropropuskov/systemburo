package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Резолв организации и компании при подаче заявки (issue #1437).
//
// Контракт: пришёл organization_id - берём его, убедившись, что запись активна;
// пришло только наименование - ищем по ключу дедупликации, а если такого нет,
// заводим запись «на проверке» (moderation_status = pending, тип «Подрядчик»).
//
// Прежний фолбэк «наименование не совпало по точной строке -> organization_id = NULL»
// снят: лишний пробел или другой регистр давали заявку-сироту - её не видели коллеги
// по организации (applyApplicationAccessFilter матчит organization_id), не подтягивались
// согласующие из organization_users, и она выпадала из фильтров Центра и аналитики.
//
// Подмена организации закрыта правом KeyApplicationOrganizationOverride: без него
// заявка привязывается только к своей записи из профиля, с ним - к любой активной или
// к новой, заведённой «на проверке». Право и есть разрешение на чужую организацию,
// поэтому отдельной проверки «id принадлежит своей» тут нет.

// directoryRef описывает справочник, из которого резолвится ссылка заявки.
// Организации и компании ведём одним кодом: расхождение между зеркальными
// сервисами тихое, а поведение подачи должно быть одинаковым для обеих сущностей.
type directoryRef struct {
	table         string
	notFoundMsg   string
	degenerateMsg string
	overrideMsg   string
	// own возвращает запись подающего из его профиля: она разрешена без права.
	own func(applicantScope) *int
	// create заводит запись «на проверке» и возвращает её id.
	create func(tx *gorm.DB, name string, senderID int) (int, error)
}

// directoryResolution - итог резолва: id для заявки и наименование записи, заведённой
// «на проверке». PendingName пуст, когда ссылка легла на существующую запись; непустой
// он значит, что справочник пополнился и запись ждёт разбора - по нему подача зовёт
// принимающих (см. directory_pending_notify.go). Возвращать один лишь id мало: заявка,
// привязанная к чужому черновику из прошлой подачи, нового разбора не требует.
type directoryResolution struct {
	ID          *int
	PendingName string
}

// applicantScope - кто подаёт заявку: сам пользователь, его организация и компания из
// профиля и разрешено ли ему указывать чужие. Собирается в SubmitCompleteApplication
// и CreateApplication, где известен и пользователь, и результат проверки права.
type applicantScope struct {
	userID         int
	organizationID *int
	companyID      *int
	canOverride    bool
}

var organizationRef = directoryRef{
	table:         "organizations",
	notFoundMsg:   "Организация не найдена или находится в архиве",
	degenerateMsg: "Укажите наименование организации",
	overrideMsg:   "Недостаточно прав, чтобы подать заявку от другой организации",
	own:           func(s applicantScope) *int { return s.organizationID },
	create: func(tx *gorm.DB, name string, senderID int) (int, error) {
		orgType := models.OrgTypeContractor
		org := models.Organization{
			Name:             name,
			Type:             &orgType,
			IsActive:         true,
			ModerationStatus: models.ModerationPending,
			CreatedByUserID:  &senderID,
		}
		if err := tx.Create(&org).Error; err != nil {
			return 0, err
		}
		return org.ID, nil
	},
}

var companyRef = directoryRef{
	table:         "companies",
	notFoundMsg:   "Компания не найдена или находится в архиве",
	degenerateMsg: "Укажите наименование компании",
	overrideMsg:   "Недостаточно прав, чтобы подать заявку от другой компании",
	own:           func(s applicantScope) *int { return s.companyID },
	create: func(tx *gorm.DB, name string, senderID int) (int, error) {
		compType := models.OrgTypeContractor
		company := models.Company{
			Name:             name,
			Type:             &compType,
			IsActive:         true,
			ModerationStatus: models.ModerationPending,
			CreatedByUserID:  &senderID,
		}
		if err := tx.Create(&company).Error; err != nil {
			return 0, err
		}
		return company.ID, nil
	},
}

// resolveOrganizationRef возвращает organization_id заявки. ID = nil - организация не
// указана (тогда заявка держится на компании, гейт «укажите одно из двух» проверяется выше).
func (s *applicationService) resolveOrganizationRef(ctx context.Context, tx *gorm.DB, scope applicantScope, id *int, name string) (directoryResolution, error) {
	return s.resolveDirectoryRef(ctx, tx, organizationRef, models.AuditEntityOrganization, models.OrganizationActionCreated, scope, id, name)
}

// resolveCompanyRef - зеркало resolveOrganizationRef для компаний.
func (s *applicationService) resolveCompanyRef(ctx context.Context, tx *gorm.DB, scope applicantScope, id *int, name string) (directoryResolution, error) {
	return s.resolveDirectoryRef(ctx, tx, companyRef, models.AuditEntityCompany, models.CompanyActionCreated, scope, id, name)
}

// ensureDirectoryAllowed пропускает свою запись из профиля всем, а чужую - только по
// праву. Возвращает 403 до проверки существования записи, чтобы перебором id нельзя
// было выяснить состав чужого справочника.
func ensureDirectoryAllowed(ref directoryRef, scope applicantScope, target int) error {
	if scope.canOverride {
		return nil
	}
	if own := ref.own(scope); own != nil && *own == target {
		return nil
	}
	return echo.NewHTTPError(http.StatusForbidden, ref.overrideMsg)
}

func (s *applicationService) resolveDirectoryRef(
	ctx context.Context, tx *gorm.DB, ref directoryRef,
	auditEntity, auditAction string, scope applicantScope, id *int, rawName string,
) (directoryResolution, error) {
	if id != nil {
		if err := ensureDirectoryAllowed(ref, scope, *id); err != nil {
			return directoryResolution{}, err
		}
		var found int
		if err := tx.Raw("SELECT id FROM "+ref.table+" WHERE id = ? AND is_active = true", *id).Scan(&found).Error; err != nil {
			slog.Error("не удалось проверить запись справочника", "table", ref.table, "id", *id, "error", err)
			return directoryResolution{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки справочника")
		}
		if found == 0 {
			return directoryResolution{}, echo.NewHTTPError(http.StatusBadRequest, ref.notFoundMsg)
		}
		return directoryResolution{ID: &found}, nil
	}

	// Канонизируем оформление сразу: дальше это наименование и ищется, и пишется в
	// справочник, и уходит принимающим в уведомление о разборе. Ключ дедупликации от
	// канонизации не меняется, поэтому поиск от неё не зависит (#1437).
	name := normalize.OrgNameDisplay(rawName)
	if name == "" {
		return directoryResolution{}, nil
	}

	key := normalize.OrgName(name)
	existing, err := findActiveDirectoryEntry(tx, ref, name, key)
	if err != nil {
		return directoryResolution{}, err
	}
	if existing != 0 {
		if err := ensureDirectoryAllowed(ref, scope, existing); err != nil {
			return directoryResolution{}, err
		}
		return directoryResolution{ID: &existing}, nil
	}

	// Из наименования без букв и цифр запись не заводим: в справочник уехал бы мусор от
	// опечатки, с которым потом никто ничего не сделает. Пустого ключа для этой проверки
	// недостаточно - «---» его имеет (дефис нормализация оставляет), но содержания в таком
	// наименовании столько же, сколько в «"""».
	if key == "" || normalize.OrgNameMeaningless(name) {
		return directoryResolution{}, echo.NewHTTPError(http.StatusBadRequest, ref.degenerateMsg)
	}

	// Наименования нет в справочнике - значит это заведомо не своя запись из профиля,
	// и завести её может только тот, кому разрешена чужая организация.
	if !scope.canOverride {
		return directoryResolution{}, echo.NewHTTPError(http.StatusForbidden, ref.overrideMsg)
	}

	newID, created, err := createDirectoryEntry(tx, ref, name, key, scope.userID)
	if err != nil {
		return directoryResolution{}, err
	}
	if !created {
		// Ключ заняла параллельная подача - заявка легла на её запись. Разбор той записи
		// уже назначен, второй раз принимающих не зовём (PendingName остаётся пустым).
		// Прав не перепроверяем: до сюда доходит только тот, у кого есть override (гейт
		// выше), а он разрешает любую запись справочника.
		return directoryResolution{ID: &newID}, nil
	}
	slog.Info("запись справочника создана из заявки", "table", ref.table, "id", newID, "name", name, "author", scope.userID)
	author := scope.userID
	s.recorder.Log(ctx, tx, auditEntity, &newID, auditAction, &author, map[string]any{
		"name":              name,
		"type":              models.OrgTypeContractor,
		"moderation_status": models.ModerationPending,
		"source":            "application",
	})
	return directoryResolution{ID: &newID, PendingName: name}, nil
}

// findActiveDirectoryEntry ищет активную запись справочника по наименованию.
//
// Вырожденное наименование (одни кавычки или дефисы) даёт пустой ключ, по которому
// несвязанные записи схлопнулись бы в одну - для них сверяем точную строку, как
// applyNameDuplicateFilter в справочниках.
//
// Partial unique index по ключу (срез 9) оставляет одному ключу не более одной активной
// записи, поэтому выборка однозначна. Порядок «проверенная, затем самая старая» сохранён
// для баз, где индекс ещё не встал из-за неслитых дублей: резолв должен быть
// детерминирован и не цепляться к свежему черновику.
func findActiveDirectoryEntry(tx *gorm.DB, ref directoryRef, name, key string) (int, error) {
	condition, arg := "name_normalized = ?", key
	if key == "" {
		condition, arg = "name = ?", name
	}
	query := fmt.Sprintf(
		"SELECT id FROM %s WHERE is_active = true AND %s ORDER BY (moderation_status = ?) DESC, id ASC LIMIT 1",
		ref.table, condition,
	)
	var existing int
	if err := tx.Raw(query, arg, models.ModerationApproved).Scan(&existing).Error; err != nil {
		slog.Error("не удалось найти запись справочника по наименованию", "table", ref.table, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки справочника")
	}
	return existing, nil
}

// createDirectoryEntry заводит запись «на проверке» и возвращает её id. created=false
// значит, что запись завела параллельная подача с тем же наименованием, а эта
// привязалась к её строке.
//
// INSERT идёт под SAVEPOINT: partial unique index по ключу отбивает второй INSERT в
// гонке, а нарушение уникальности аварийно завершает всю транзакцию Postgres - без
// точки возврата подача упала бы целиком, хотя нужная запись уже существует.
func createDirectoryEntry(tx *gorm.DB, ref directoryRef, name, key string, senderID int) (int, bool, error) {
	const savepoint = "directory_entry_create"
	if err := tx.SavePoint(savepoint).Error; err != nil {
		slog.Error("не удалось поставить точку возврата перед созданием записи справочника", "table", ref.table, "error", err)
		return 0, false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания записи справочника")
	}
	newID, err := ref.create(tx, name, senderID)
	if err == nil {
		return newID, true, nil
	}
	if !isUniqueViolation(err) {
		slog.Error("не удалось создать запись справочника из заявки", "table", ref.table, "name", name, "error", err)
		return 0, false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания записи справочника")
	}
	if rbErr := tx.RollbackTo(savepoint).Error; rbErr != nil {
		slog.Error("не удалось вернуться к точке возврата после конфликта наименований", "table", ref.table, "error", rbErr)
		return 0, false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания записи справочника")
	}
	existing, err := findActiveDirectoryEntry(tx, ref, name, key)
	if err != nil {
		return 0, false, err
	}
	if existing == 0 {
		// Ключ занят, но активной записи по нему нет - модель дедупликации разошлась с
		// предикатом индекса. Молчать нельзя: наименование не привязать ни к чему.
		slog.Error("ключ наименования занят, но активная запись не найдена", "table", ref.table, "name", name, "key", key)
		return 0, false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания записи справочника")
	}
	slog.Info("наименование завела параллельная подача - привязались к её записи",
		"table", ref.table, "id", existing, "name", name)
	return existing, false, nil
}
