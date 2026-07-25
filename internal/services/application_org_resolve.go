package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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

// resolveOrganizationRef возвращает organization_id заявки. nil - организация не указана
// (тогда заявка держится на компании, гейт «укажите одно из двух» проверяется выше).
func (s *applicationService) resolveOrganizationRef(ctx context.Context, tx *gorm.DB, scope applicantScope, id *int, name string) (*int, error) {
	return s.resolveDirectoryRef(ctx, tx, organizationRef, models.AuditEntityOrganization, models.OrganizationActionCreated, scope, id, name)
}

// resolveCompanyRef - зеркало resolveOrganizationRef для компаний.
func (s *applicationService) resolveCompanyRef(ctx context.Context, tx *gorm.DB, scope applicantScope, id *int, name string) (*int, error) {
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
) (*int, error) {
	if id != nil {
		if err := ensureDirectoryAllowed(ref, scope, *id); err != nil {
			return nil, err
		}
		var found int
		if err := tx.Raw("SELECT id FROM "+ref.table+" WHERE id = ? AND is_active = true", *id).Scan(&found).Error; err != nil {
			slog.Error("не удалось проверить запись справочника", "table", ref.table, "id", *id, "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки справочника")
		}
		if found == 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, ref.notFoundMsg)
		}
		return &found, nil
	}

	name := strings.TrimSpace(rawName)
	if name == "" {
		return nil, nil
	}

	key := normalize.OrgName(name)
	// Вырожденное наименование (одни кавычки или дефисы) даёт пустой ключ, по которому
	// несвязанные записи схлопнулись бы в одну - для них сверяем точную строку, как
	// applyNameDuplicateFilter в справочниках.
	condition := "name_normalized = ?"
	arg := key
	if key == "" {
		condition = "name = ?"
		arg = name
	}
	// Partial unique index по ключу появится последним срезом эпика, поэтому одному ключу
	// пока могут отвечать несколько записей: берём проверенную и самую старую, чтобы
	// резолв был детерминирован и не цеплялся к свежему черновику.
	query := fmt.Sprintf(
		"SELECT id FROM %s WHERE is_active = true AND %s ORDER BY (moderation_status = ?) DESC, id ASC LIMIT 1",
		ref.table, condition,
	)
	var existing int
	if err := tx.Raw(query, arg, models.ModerationApproved).Scan(&existing).Error; err != nil {
		slog.Error("не удалось найти запись справочника по наименованию", "table", ref.table, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки справочника")
	}
	if existing != 0 {
		if err := ensureDirectoryAllowed(ref, scope, existing); err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Из вырожденного наименования запись не заводим: ключа у неё нет, дедупликация по
	// ней работать не будет, а в справочник уехал бы мусор от опечатки.
	if key == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, ref.degenerateMsg)
	}

	// Наименования нет в справочнике - значит это заведомо не своя запись из профиля,
	// и завести её может только тот, кому разрешена чужая организация.
	if !scope.canOverride {
		return nil, echo.NewHTTPError(http.StatusForbidden, ref.overrideMsg)
	}

	newID, err := ref.create(tx, name, scope.userID)
	if err != nil {
		slog.Error("не удалось создать запись справочника из заявки", "table", ref.table, "name", name, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания записи справочника")
	}
	slog.Info("запись справочника создана из заявки", "table", ref.table, "id", newID, "name", name, "author", scope.userID)
	author := scope.userID
	s.recorder.Log(ctx, tx, auditEntity, &newID, auditAction, &author, map[string]any{
		"name":              name,
		"type":              models.OrgTypeContractor,
		"moderation_status": models.ModerationPending,
		"source":            "application",
	})
	return &newID, nil
}
