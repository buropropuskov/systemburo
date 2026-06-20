package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
)

// securityUserTypeCode - код типа аккаунта охранника ЧОП в user_types. Резолвится по code,
// а не по числовому type_id: id нестабилен между средами (сид может отличаться), code - нет.
const securityUserTypeCode = "security"

// AvailableAttachment - строка вкладки "Доступные мне" (#706): одно вложение подтверждённой
// заявки, доступное охраннику по совпадению мест, плюс короткая инфо родительской заявки для
// карточки. Read-only проекция, без согласований/истории. Places - агрегат имён мест вложения
// (места разгрузки для cars/items, места прохода сотрудников для people).
type AvailableAttachment struct {
	AttachmentID          int        `json:"attachment_id"`
	AttachmentType        string     `json:"attachment_type"`
	AttachmentName        *string    `json:"attachment_name"`
	AttachmentDisplayName *string    `json:"attachment_display_name"`
	EntryDateFrom         *string    `json:"entry_date_from"`
	EntryDateTo           *string    `json:"entry_date_to"`
	EntryTimeFrom         *string    `json:"entry_time_from"`
	EntryTimeTo           *string    `json:"entry_time_to"`
	CreatedAt             *time.Time `json:"created_at"`
	Places                *string    `json:"places"`

	ApplicationID     int        `json:"application_id"`
	ApplicationNumber *string    `json:"application_number"`
	Confirmation      *string    `json:"confirmation"`
	Status            *string    `json:"status"`
	SendingDatetime   *time.Time `json:"sending_datetime"`
	OrganizationName  *string    `json:"organization_name"`
	CompanyName       *string    `json:"company_name"`
	SenderName        *string    `json:"sender_name"`
	SenderFullName    *string    `json:"sender_full_name"`
}

// availableAttachmentSelect - столбцы листинга. Подзапросы places ссылаются на a.id напрямую
// (без плейсхолдеров), поэтому в args после WHERE идут только LIMIT/OFFSET.
const availableAttachmentSelect = `
	a.id as attachment_id,
	a.attachment_type,
	a.attachment_name,
	a.attachment_display_name,
	a.entry_date_from, a.entry_date_to,
	a.entry_time_from, a.entry_time_to,
	a.created_at,
	app.id as application_id,
	app.application_number,
	app.confirmation,
	app.status,
	app.sending_datetime,
	COALESCE(o.name, c.name) as organization_name,
	c.name as company_name,
	format_short_name(su.last_name, su.first_name, su.middle_name) as sender_name,
	format_full_name(su.last_name, su.first_name, su.middle_name) as sender_full_name,
	CASE WHEN a.attachment_type = 'people' THEN (
		SELECT string_agg(DISTINCT st.name, ', ')
		FROM employees e
		JOIN employee_target_tables ett ON ett.employee_id = e.id
		JOIN system_tables st ON st.id = ett.table_id
		WHERE e.attachment_id = a.id
	) ELSE (
		SELECT string_agg(DISTINCT up.name, ', ')
		FROM attachment_unload_places aup
		JOIN unload_places up ON up.id = aup.unload_place_id
		WHERE aup.attachment_id = a.id
	) END as places`

// securityVisibilityWhere строит предикат доступности вложения для вкладки "Доступные мне"
// (ссылается на алиасы a = attachments, app = applications) и его аргументы. Инвариант (#706):
// confirmation = 'Согласовано' И пересечение мест вложения с местами охранника непусто. Места:
// cars/items - attachment_unload_places ∩ security_user_unload_places; people - места прохода
// сотрудников (employee_target_tables) ∩ security_user_tables. Супер-админ отбрасывает предикат
// по местам, оставляя только confirmation-гейт. НЕ смотрит is_active/status места (факт назначения
// = доступ; "обслуживание" места не должно молча скрывать вложение). Не пересекается с
// forward_attachments (#680) - другой источник видимости.
func securityVisibilityWhere(userID int, isSuperAdmin bool) (string, []interface{}) {
	confirm := "app.confirmation = ?"
	args := []interface{}{models.ConfirmationApproved}
	if isSuperAdmin {
		return confirm, args
	}
	place := `(
		(a.attachment_type IN ('cars','items') AND EXISTS (
			SELECT 1 FROM attachment_unload_places aup
			JOIN security_user_unload_places suup ON suup.unload_place_id = aup.unload_place_id
			WHERE aup.attachment_id = a.id AND suup.user_id = ?))
		OR
		(a.attachment_type = 'people' AND EXISTS (
			SELECT 1 FROM employees e
			JOIN employee_target_tables ett ON ett.employee_id = e.id
			JOIN security_user_tables sut ON sut.table_id = ett.table_id
			WHERE e.attachment_id = a.id AND sut.user_id = ?))
	)`
	args = append(args, userID, userID)
	return confirm + " AND " + place, args
}

// IsSecurityUser сообщает, является ли аккаунт типом security (резолв по user_types.code,
// образец CheckBuroByUsername). Несуществующий пользователь -> false без ошибки.
func (s *applicationService) IsSecurityUser(ctx context.Context, userID int) (bool, error) {
	// GORM Scan не возвращает ошибку на 0 строк (в отличие от First): для несуществующего
	// userID Code остаётся пустой строкой -> false.
	var row struct{ Code string }
	err := s.db.WithContext(ctx).
		Table("users").
		Select("user_types.code").
		Joins("JOIN user_types ON user_types.id = users.type_id").
		Where("users.id = ?", userID).
		Scan(&row).Error
	if err != nil {
		return false, fmt.Errorf("failed to resolve user type: %w", err)
	}
	return row.Code == securityUserTypeCode, nil
}

// securityHasAnyPlace - быстрая проверка, назначено ли охраннику хоть одно место (разгрузки или
// прохода). Короткое замыкание для GetAvailableAttachmentsForSecurity: без мест список заведомо
// пуст, основной запрос не нужен.
func (s *applicationService) securityHasAnyPlace(ctx context.Context, userID int) (bool, error) {
	var total int64
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COUNT(*) FROM security_user_unload_places WHERE user_id = ?) +
			(SELECT COUNT(*) FROM security_user_tables WHERE user_id = ?)`,
		userID, userID).Scan(&total).Error
	if err != nil {
		return false, fmt.Errorf("failed to count security places: %w", err)
	}
	return total > 0, nil
}

// GetAvailableAttachmentsForSecurity возвращает страницу вложений подтверждённых заявок, доступных
// охраннику по совпадению мест (#706), и общее количество. Один плоский SQL без N+1, пагинация по
// образцу GetApplicationsPaginated (отдельный COUNT + страница). Супер-админ видит все вложения
// подтверждённых заявок без фильтра по местам. Охранник без назначенных мест -> пустая страница
// без основного запроса.
func (s *applicationService) GetAvailableAttachmentsForSecurity(ctx context.Context, userID int, isSuperAdmin bool, page, perPage int) ([]AvailableAttachment, int64, error) {
	page, perPage = normalizePage(page, perPage)

	if !isSuperAdmin {
		hasPlaces, err := s.securityHasAnyPlace(ctx, userID)
		if err != nil {
			return nil, 0, err
		}
		if !hasPlaces {
			return []AvailableAttachment{}, 0, nil
		}
	}

	where, args := securityVisibilityWhere(userID, isSuperAdmin)

	var total int64
	countSQL := `SELECT COUNT(*) FROM attachments a
		JOIN applications app ON app.id = a.application_id
		WHERE ` + where
	if err := s.db.WithContext(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count available attachments: %w", err)
	}
	if total == 0 {
		return []AvailableAttachment{}, 0, nil
	}

	offset := (page - 1) * perPage
	dataSQL := `SELECT ` + availableAttachmentSelect + `
		FROM attachments a
		JOIN applications app ON app.id = a.application_id
		LEFT JOIN organizations o ON o.id = app.organization_id
		LEFT JOIN companies c ON c.id = app.company_id
		LEFT JOIN users su ON su.id = app.sender_user_id
		WHERE ` + where + `
		ORDER BY app.sending_datetime DESC NULLS LAST, a.id DESC
		LIMIT ? OFFSET ?`
	dataArgs := append(append([]interface{}{}, args...), perPage, offset)

	rows := make([]AvailableAttachment, 0)
	if err := s.db.WithContext(ctx).Raw(dataSQL, dataArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch available attachments: %w", err)
	}
	return rows, total, nil
}

// CanSecurityViewAttachment сообщает, доступно ли конкретное вложение охраннику по тем же правилам,
// что и листинг (#706). Используется детальным эндпоинтом для 403 на чужое вложение. Супер-админ
// проходит фильтр по местам, но confirmation-гейт применяется и к нему.
func (s *applicationService) CanSecurityViewAttachment(ctx context.Context, userID int, isSuperAdmin bool, attachmentID int) (bool, error) {
	where, args := securityVisibilityWhere(userID, isSuperAdmin)
	sql := `SELECT EXISTS (
		SELECT 1 FROM attachments a
		JOIN applications app ON app.id = a.application_id
		WHERE a.id = ? AND ` + where + `)`
	queryArgs := append([]interface{}{attachmentID}, args...)

	var canView bool
	if err := s.db.WithContext(ctx).Raw(sql, queryArgs...).Scan(&canView).Error; err != nil {
		return false, fmt.Errorf("failed to check security attachment access: %w", err)
	}
	return canView, nil
}
