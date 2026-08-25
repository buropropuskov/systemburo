package handlers

import (
	"context"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AvailableAttachmentDetail - ответ детального эндпоинта вкладки "Доступные мне" (#706):
// заголовок вложения с краткой инфо родительской заявки плюс типизированное содержимое.
// Заполняется только релевантный тип (cars/people/items), остальные опускаются.
type AvailableAttachmentDetail struct {
	Attachment services.AvailableAttachment  `json:"attachment"`
	Cars       []services.CarWithPlaces      `json:"cars,omitempty"`
	Employees  []services.EmployeeWithTables `json:"employees,omitempty"`
	Items      []services.ItemInfo           `json:"items,omitempty"`
}

// requireSecurityOrAdmin - гейт вкладки "Доступные мне" (#706, #976). Доступ имеют: супер-админ,
// обычный админ, любой носитель права page.available и тип "Охранник" (user_types.code='security').
// Второй возврат - "видит всё": true у супер-админа, админа и носителя page.available, который НЕ
// работник поста; у работника поста - всегда false, то есть только его места, даже когда право
// page.available ему выдано. Право открывает страницу (пункт меню и вход), но не снимает фильтр по
// местам: пост не должен видеть чужие. Прочим - 403.
// Набор совпадает с FE-гейтом canViewAccessibleAttachments; рассинхрон давал 403 при видимой вкладке.
func (h *ApplicationHandler) requireSecurityOrAdmin(c echo.Context) (int, bool, error) {
	return securityScope(c, h.service, h.resolver)
}

// securityUserChecker - минимум, нужный securityScope: проверка типа "Охранник".
type securityUserChecker interface {
	IsSecurityUser(ctx context.Context, userID int) (bool, error)
}

// securityScope - тело requireSecurityOrAdmin, вынесенное для повторного использования
// вне ApplicationHandler (скачивание бланка вложения). Возвращает 403 тем, кто не охрана
// и не носитель page.available.
func securityScope(c echo.Context, svc securityUserChecker, resolver *services.PermissionResolver) (int, bool, error) {
	userID := GetUserID(c)
	if IsSuperAdmin(c) {
		return userID, true, nil
	}
	// Работник поста видит только свои места - и тогда, когда право page.available
	// ему выдано. Право открывает саму страницу (пункт меню и вход), а не снимает
	// ограничение по местам: иначе выдача права роли охраны показывала бы посту
	// вложения всех постов сразу.
	isSecurity, err := svc.IsSecurityUser(c.Request().Context(), userID)
	if err != nil {
		return 0, false, err
	}
	if isSecurity {
		return userID, false, nil
	}
	// Has(page.available) истинно для админа (allowAll) и для явного гранта роли/группы/override.
	// Резолвер учитывает бан (у забаненного Has=false) и личные deny-override.
	set, err := resolver.Resolve(c.Request().Context(), userID)
	if err != nil {
		return 0, false, err
	}
	if set.Has(services.KeyPageAvailable) {
		return userID, true, nil
	}
	return 0, false, echo.NewHTTPError(http.StatusForbidden, "Access denied")
}

// GetAvailableAttachments godoc
// @Summary      Доступные мне вложения
// @Description  Плоский список вложений подтверждённых заявок. Супер-админ, обычный админ и носитель права page.available видят все подтверждённые вложения; охранник (тип security) - по совпадению мест разгрузки/прохода. Прочим - 403.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        page            query int    false "Номер страницы"
// @Param        per_page        query int    false "Размер страницы"
// @Param        search          query string false "Поиск по номеру заявки, имени вложения и ФИО отправителя"
// @Param        attachment_type query string false "Тип вложения: cars/people/items"
// @Param        organization_id query int    false "ID организации"
// @Param        company_id      query int    false "ID компании"
// @Param        completed       query bool   false "Только завершённые заявки (по умолчанию скрыты); при активном search игнорируется"
// @Param        night           query bool   false "Только вложения с ночным окном въезда [22:00-06:00); при активном search игнорируется"
// @Success      200 {array}  services.AvailableAttachment
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/available-attachments [get]
func (h *ApplicationHandler) GetAvailableAttachments(c echo.Context) error {
	userID, unrestricted, err := h.requireSecurityOrAdmin(c)
	if err != nil {
		return err
	}

	var filter services.AvailableAttachmentFilters
	if err := c.Bind(&filter); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		params = models.PaginationParams{}
	}
	params.Normalize()

	data, total, err := h.service.GetAvailableAttachmentsForSecurity(
		c.Request().Context(), userID, unrestricted, filter, params.Page, params.PerPage,
	)
	if err != nil {
		return err
	}
	return RespondPaginated(c, data, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
}

// GetAvailableAttachmentDetail godoc
// @Summary      Деталь доступного вложения
// @Description  Заголовок вложения с инфо заявки и типизированное содержимое (автомобили/сотрудники/ТМЦ). Доступ: супер-админ/админ/носитель page.available - любое подтверждённое вложение; охранник - только по совпавшему месту; иначе 403.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID вложения"
// @Success      200 {object} handlers.AvailableAttachmentDetail
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/available-attachments/{id} [get]
func (h *ApplicationHandler) GetAvailableAttachmentDetail(c echo.Context) error {
	userID, unrestricted, err := h.requireSecurityOrAdmin(c)
	if err != nil {
		return err
	}

	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	canView, err := h.service.CanSecurityViewAttachment(ctx, userID, unrestricted, id)
	if err != nil {
		return err
	}
	if !canView {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	header, err := h.service.GetAvailableAttachmentByID(ctx, id)
	if err != nil {
		return err
	}
	if header == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
	}

	detail := AvailableAttachmentDetail{Attachment: *header}
	// Охрана видит только допущенное на КПП: исходный состав подачи и принятые
	// дополнения (#1685). Непринятое дополнение сюда не попадает - в people-вложении
	// едут серия и номер паспорта и номер патента людей, которых ещё не согласовали.
	// SupplementScopeAll в этой ветке - утечка персональных данных, не расширение выдачи.
	switch header.AttachmentType {
	case "cars":
		detail.Cars, err = h.service.GetAttachmentCars(ctx, id, services.SupplementScopeAdmitted)
	case "people":
		detail.Employees, err = h.service.GetAttachmentEmployees(ctx, id, services.SupplementScopeAdmitted)
	case "items":
		detail.Items, err = h.service.GetAttachmentItems(ctx, id, services.SupplementScopeAdmitted)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "Unknown attachment type")
	}
	if err != nil {
		return err
	}
	return RespondSuccess(c, detail)
}
