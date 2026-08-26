package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
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
	// HasBlank - у типа вложения (unique_attachment) есть активный Excel-шаблон, значит для этого
	// вложения генерируется заполненный бланк (#706 S4). По нему фронт показывает "Посмотреть файл".
	HasBlank bool `json:"has_blank"`

	ApplicationID     int        `json:"application_id"`
	ApplicationNumber *string    `json:"application_number"`
	Confirmation      *string    `json:"confirmation"`
	Status            *string    `json:"status"`
	SendingDatetime   *time.Time `json:"sending_datetime"`
	OrganizationName  *string    `json:"organization_name"`
	CompanyName       *string    `json:"company_name"`
	SenderName        *string    `json:"sender_name"`
	SenderFullName    *string    `json:"sender_full_name"`
	// SenderUserID нужен маскировке ФИО подавшего, не давшего согласия на обработку
	// персональных данных: без него строку не с чем сопоставить.
	SenderUserID *int `json:"sender_user_id"`
}

// availableAttachmentFilters - опциональные пользовательские фильтры вкладки "Доступные мне"
// (BE-S6). Сужают листинг ПОВЕРХ инварианта видимости (места ∩ + confirmation='Согласовано' +
// status IN ('В работе','Завершено')), не ослабляя его. Все поля опциональны: nil/пусто = фильтр
// не применяется, выдача как до BE-S6.
// query-теги повторяют стиль ApplicationFilter - биндятся echo c.Bind из query-строки.
type AvailableAttachmentFilters struct {
	Search         *string `query:"search"`
	AttachmentType *string `query:"attachment_type"`
	OrganizationID *int    `query:"organization_id"`
	CompanyID      *int    `query:"company_id"`
	// Completed управляет видимостью завершённых заявок (status='Завершено'): nil/false - скрыть
	// завершённые (дефолт вкладки), true - показать только завершённые. При активном Search не
	// применяется (поиск отдаёт и завершённые, и нет) - см. availableAttachmentFilterWhere.
	Completed *bool `query:"completed"`
	// Night фильтрует по ночному окну въезда: nil/false - не фильтровать, true - только
	// вложения, чьё окно entry_time пересекает ночь [22:00, 06:00). Вычисляется в SQL из строк
	// entry_time_from/to без новых колонок (см. availableAttachmentNightClause). При активном
	// Search не применяется (поиск отдаёт всё) - см. availableAttachmentFilterWhere.
	Night *bool `query:"night"`
}

// availableAttachmentNightClause - предикат "ночного" окна въезда для фильтра night=true. Ночь =
// [22:00, 06:00) (22:00 включительно, 06:00 нет). Окно въезда хранится строками entry_time_from/to
// в формате HH:MM или HH:MM:SS (varchar(20)); substr(...,1,5) приводит обе формы к 'HH:MM', а
// зеро-паддинг часа делает лексикографическое сравнение хронологическим - без ::time-каста, который
// упал бы (500) на нечисловом мусоре в колонке. Окно НЕ через полночь (from<=to) попадает в ночь,
// если конец >= 22:00 или начало < 06:00; окно через полночь (from>to) всегда пересекает ночь
// (содержит 00:00 < 06:00). Пустые/NULL-времена не считаются ночью (нет данных об окне).
const availableAttachmentNightClause = `(
	a.entry_time_from IS NOT NULL AND a.entry_time_to IS NOT NULL
	AND a.entry_time_from <> '' AND a.entry_time_to <> ''
	AND CASE WHEN substr(a.entry_time_from, 1, 5) <= substr(a.entry_time_to, 1, 5)
		THEN substr(a.entry_time_to, 1, 5) >= '22:00' OR substr(a.entry_time_from, 1, 5) < '06:00'
		ELSE true
	END
)`

// availableAttachmentFrom - общий FROM листинга и счётчика "Доступные мне". LEFT JOIN'ы
// organizations/companies/users держат отношение 1:1 к вложению (все по PK), поэтому НЕ меняют
// COUNT(*) и могут стоять в обоих запросах; благодаря им и подзапросы select, и фильтры BE-S6
// (поиск по ФИО отправителя через su) видят алиасы o/c/su. applications - LEFT JOIN (#1049):
// ручное вложение-сирота (application_id NULL, is_manual) НЕ принадлежит заявке; при INNER оно
// выпало бы из выдачи. Для сирот app.* остаются NULL (метка "добавлено вручную"), а org/company
// берутся с самого вложения через COALESCE(app.*, a.*).
const availableAttachmentFrom = `
	FROM attachments a
	LEFT JOIN applications app ON app.id = a.application_id
	LEFT JOIN organizations o ON o.id = COALESCE(app.organization_id, a.organization_id)
	LEFT JOIN companies c ON c.id = COALESCE(app.company_id, a.company_id)
	LEFT JOIN users su ON su.id = app.sender_user_id`

// availableAttachmentSelect - столбцы листинга. Подзапросы places ссылаются на a.id напрямую
// (без плейсхолдеров); аргументы в порядке: видимость -> фильтры BE-S6 -> LIMIT/OFFSET.
// var, а не const: предикат допуска строится той же admittedSupplementCond, что и остальные
// читатели состава - иначе условие пришлось бы продублировать литералом и оно разъехалось бы
// с моделью при первом же переименовании статуса.
var availableAttachmentSelect = `
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
	app.sender_user_id,
	format_short_name(su.last_name, su.first_name, su.middle_name) as sender_name,
	format_full_name(su.last_name, su.first_name, su.middle_name) as sender_full_name,
	CASE WHEN a.attachment_type = 'people' THEN (
		SELECT string_agg(DISTINCT COALESCE(st.display_name, st.name), ', ')
		FROM employees e
		JOIN employee_target_tables ett ON ett.employee_id = e.id
		JOIN system_tables st ON st.id = ett.table_id
		WHERE e.attachment_id = a.id AND ` + admittedSupplementCond("e") + `
	) ELSE (
		SELECT string_agg(DISTINCT up.name, ', ')
		FROM attachment_unload_places aup
		JOIN unload_places up ON up.id = aup.unload_place_id
		WHERE aup.attachment_id = a.id
	) END as places,
	EXISTS (
		SELECT 1 FROM attachment_templates t
		WHERE t.unique_attachment_id = a.unique_attachment_id AND t.is_active = true
	) as has_blank`

// securityVisibilityWhere строит предикат доступности вложения для вкладки "Доступные мне"
// (ссылается на алиасы a = attachments, app = applications) и его аргументы. Инвариант (#706):
// confirmation = 'Согласовано' И status IN ('В работе','Завершено') И пересечение мест вложения с
// местами охранника непусто. Места: cars/items - attachment_unload_places ∩
// security_user_unload_places; people - места прохода сотрудников (employee_target_tables) ∩
// security_user_tables. unrestricted (super/admin/носитель права page.available, #976) отбрасывает
// предикат по местам, оставляя только гейт заявки. НЕ смотрит is_active/status места (факт
// назначения = доступ; "обслуживание" места не должно молча скрывать вложение). Не пересекается с
// forward_attachments (#680) - другой источник видимости.
//
// Статус-гейт (баг: заявка "Отказано" оставалась видна). Согласование (confirmation) и статус
// жизненного цикла - РАЗНЫЕ поля: принимающий отказывает в заявке через reject, который ставит
// status='Отказано', а confirmation='Согласовано' не трогает. #951 закрыл только "Отозвана", но
// "Отказано" (и промежуточная "В обработке" после revoke_from_work) так же не должны попадать в
// допуск охраны. Поэтому гейтим не "confirmation минус чёрный список", а белым списком активных
// статусов: 'В работе' - реальный активный допуск (вкладка по умолчанию), 'Завершено' - для тоггла
// "Завершённые" (availableAttachmentFilterWhere сам выбирает между ними). Всё прочее (Отказано,
// Отозвана, В обработке, Непрочитано, ...) - вне допуска, скрыто.
//
// Ручные вложения (#1049 S6): у сироты (a.is_manual, application_id NULL) заявки нет, поэтому
// гейт заявки к ней неприменим - её видимость несёт ТОЛЬКО пересечение мест (то же, что у
// заявочных: назначен охраннику target-таблица/место -> видит). Ветка a.is_manual снимает
// app-часть, но пересечение мест остаётся - ручные видны наравне с заявочными, но не
// шире (охранник без назначения места ручное не увидит). У сироты app.* при LEFT JOIN NULL:
// ветка a.is_manual=TRUE сама делает OR истинной, а заведомо-NULL app-предикат
// сироту не отсекает (three-valued logic: TRUE OR NULL = TRUE, запрос не падает).
func securityVisibilityWhere(userID int, unrestricted bool) (string, []interface{}) {
	// Заявочные: confirmation='Согласовано' И статус активного допуска ('В работе'/'Завершено').
	// Ручные (a.is_manual) минуют весь app-гейт - заявки нет.
	confirm := "(a.is_manual OR (app.confirmation = ? AND app.status IN (?, ?)))"
	args := []interface{}{models.ConfirmationApproved, models.StatusInWork, models.StatusCompleted}
	if unrestricted {
		return confirm, args
	}
	// Сотрудник непринятого дополнения не открывает вложение охране (#1685). Пост ему
	// назначается сразу при подаче, поэтому без этого условия вложение всплыло бы в
	// "Доступные мне" по человеку, решения по которому ещё нет - и охранник увидел бы
	// заявку, к посту которой ни один ДОПУЩЕННЫЙ сотрудник не относится.
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
			WHERE e.attachment_id = a.id AND sut.user_id = ?
			  AND ` + admittedSupplementCond("e") + `))
	)`
	args = append(args, userID, userID)
	return confirm + " AND " + place, args
}

// availableAttachmentFilterWhere добавляет к предикату видимости опциональные пользовательские
// фильтры (BE-S6) и их аргументы. Поверх инварианта, не вместо: возвращает фрагмент с ведущим
// " AND ", приклеиваемый к результату securityVisibilityWhere - гейт мест/статуса заявки остаётся.
// Пустые/nil-поля игнорируются; attachment_type вне cars/people/items отбрасывается (не сужает).
// Search ищет по номеру заявки, имени и отображаемому имени вложения, ФИО отправителя (ILIKE,
// регистронезависимо). Требует алиасы a/app/su (есть в availableAttachmentFrom).
func availableAttachmentFilterWhere(f AvailableAttachmentFilters) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	// Поиск отдаёт и завершённые, и незавершённые: при активном search статус-предикат не вешаем.
	searchActive := f.Search != nil && strings.TrimSpace(*f.Search) != ""

	if f.AttachmentType != nil {
		switch t := *f.AttachmentType; t {
		case "cars", "people", "items":
			clauses = append(clauses, "a.attachment_type = ?")
			args = append(args, t)
		}
	}
	// COALESCE(app.*, a.*): у ручных вложений (#1049) org/company хранятся на самом вложении,
	// app.* NULL - иначе фильтр по организации молча скрыл бы ручные записи этой же орг.
	if f.OrganizationID != nil {
		clauses = append(clauses, "COALESCE(app.organization_id, a.organization_id) = ?")
		args = append(args, *f.OrganizationID)
	}
	if f.CompanyID != nil {
		clauses = append(clauses, "COALESCE(app.company_id, a.company_id) = ?")
		args = append(args, *f.CompanyID)
	}
	// Базовый гейт уже сузил статус до IN ('В работе','Завершено'); здесь выбираем между ними:
	// дефолт вкладки - только 'В работе' (скрыть завершённые), фильтр "Завершённые" (completed=true) -
	// только 'Завершено'. IS DISTINCT FROM сохранён для симметрии с completed-веткой (NULL-статусов в
	// выдаче уже нет - whitelist их отсёк).
	if !searchActive {
		if f.Completed != nil && *f.Completed {
			clauses = append(clauses, "app.status = ?")
			args = append(args, models.StatusCompleted)
		} else {
			clauses = append(clauses, "app.status IS DISTINCT FROM ?")
			args = append(args, models.StatusCompleted)
		}
		// Ночное окно въезда: предикат без плейсхолдеров (границы 22:00/06:00 - литералы).
		if f.Night != nil && *f.Night {
			clauses = append(clauses, availableAttachmentNightClause)
		}
	}
	if f.Search != nil {
		if s := strings.TrimSpace(*f.Search); s != "" {
			// Мощный поиск как в Центре заявок: варианты (раскладка/номер), ILIKE по полям
			// заявки/вложения/отправителя + EXISTS по машинам/местам разгрузки/сотрудникам
			// этого вложения и согласующим заявки, опечатки через strict_word_similarity.
			variants := buildSearchVariants(s)

			baseCols := []string{
				"app.application_number",
				"COALESCE(o.name, c.name, '')",
				"c.name",
				"app.message",
				"a.attachment_name",
				"a.attachment_display_name",
				"su.last_name", "su.first_name", "su.middle_name",
			}
			baseCond, baseArgs := ilikePatternsArgs(baseCols, variants)

			// Машины этого вложения: номер (+ слитно/раздельно при цифрах), марка, место разгрузки.
			// Обе колонки марки: mark_name заполнен у единиц записей, у остальных марка
			// в устаревшей car_brand (тот же перекос, что в поиске Центра заявок).
			carCond, carArgs := ilikePatternsArgs([]string{"c2.car_number", "c2.mark_name", "c2.car_brand", "c2.unload_place"}, variants)
			platePattern := ""
			if strings.ContainsAny(s, "0123456789") {
				carCond += " OR REPLACE(c2.car_number, ' ', '') ILIKE ?"
				platePattern = "%" + normalize.Plate(s) + "%"
			}
			// Непринятое дополнение в поиске не участвует (#1685): иначе запрос по номеру или
			// фамилии подтверждал бы охране, что человек или машина уже заявлены, хотя решения
			// по ним ещё нет и в составе вложения он их не увидит.
			carSub := `EXISTS(SELECT 1 FROM cars c2 WHERE c2.attachment_id = a.id
				AND ` + admittedSupplementCond("c2") + ` AND (` + carCond + `))`

			// Места разгрузки вложения: в этом view источник - attachment_unload_places
			// (по attachment_id напрямую, #706), как в availableAttachmentSelect, а не через cars.
			upCond, upArgs := ilikePatternsArgs([]string{"up.name"}, variants)
			upSub := `EXISTS(SELECT 1 FROM attachment_unload_places aup
				JOIN unload_places up ON up.id = aup.unload_place_id
				WHERE aup.attachment_id = a.id AND (` + upCond + `))`

			// Сотрудники этого вложения: ФИО/должность + опечатки.
			empCond, empArgs := ilikePatternsArgs([]string{"e.last_name", "e.first_name", "e.middle_name", "e.position"}, variants)
			empSub := `EXISTS(SELECT 1 FROM employees e
				WHERE e.attachment_id = a.id AND ` + admittedSupplementCond("e") + ` AND (` + empCond + `
					OR strict_word_similarity(?, concat_ws(' ', e.last_name, e.first_name, e.middle_name)) > 0.3))`

			// Согласующие заявки: ФИО + комментарий + опечатки.
			apprCond, apprArgs := ilikePatternsArgs([]string{"au.last_name", "au.first_name", "au.middle_name", "aru.approval_comment"}, variants)
			apprSub := `EXISTS(SELECT 1 FROM application_responsible_users aru
				JOIN users au ON au.id = aru.user_id
				WHERE aru.application_id = app.id AND (` + apprCond + `
					OR strict_word_similarity(?, concat_ws(' ', au.last_name, au.first_name, au.middle_name)) > 0.3))`

			clauses = append(clauses, "("+baseCond+" OR "+carSub+" OR "+upSub+" OR "+empSub+" OR "+apprSub+")")
			args = append(args, baseArgs...)
			args = append(args, carArgs...)
			if platePattern != "" {
				args = append(args, platePattern)
			}
			args = append(args, upArgs...)
			args = append(args, empArgs...)
			args = append(args, s) // strict_word_similarity сотрудников
			args = append(args, apprArgs...)
			args = append(args, s) // strict_word_similarity согласующих
		}
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// IsSecurityUser сообщает, является ли аккаунт типом security (резолв по user_types.code).
// Несуществующий пользователь -> false без ошибки.
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
// образцу GetApplicationsPaginated (отдельный COUNT + страница). unrestricted (super/admin/носитель
// page.available, #976) видит все вложения подтверждённых заявок без фильтра по местам. Охранник без
// назначенных мест -> пустая страница без основного запроса.
func (s *applicationService) GetAvailableAttachmentsForSecurity(ctx context.Context, userID int, unrestricted bool, filter AvailableAttachmentFilters, page, perPage int) ([]AvailableAttachment, int64, error) {
	page, perPage = normalizePage(page, perPage)

	if !unrestricted {
		hasPlaces, err := s.securityHasAnyPlace(ctx, userID)
		if err != nil {
			return nil, 0, err
		}
		if !hasPlaces {
			return []AvailableAttachment{}, 0, nil
		}
	}

	where, args := securityVisibilityWhere(userID, unrestricted)
	if filterWhere, filterArgs := availableAttachmentFilterWhere(filter); filterWhere != "" {
		where += filterWhere
		args = append(args, filterArgs...)
	}

	var total int64
	countSQL := `SELECT COUNT(*) ` + availableAttachmentFrom + `
		WHERE ` + where
	if err := s.db.WithContext(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count available attachments: %w", err)
	}
	if total == 0 {
		return []AvailableAttachment{}, 0, nil
	}

	offset := (page - 1) * perPage
	dataSQL := `SELECT ` + availableAttachmentSelect + availableAttachmentFrom + `
		WHERE ` + where + `
		ORDER BY app.sending_datetime DESC NULLS LAST, a.id DESC
		LIMIT ? OFFSET ?`
	dataArgs := append(append([]interface{}{}, args...), perPage, offset)

	rows := make([]AvailableAttachment, 0)
	if err := s.db.WithContext(ctx).Raw(dataSQL, dataArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch available attachments: %w", err)
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range rows {
			rows[i].SenderName = maskNamePtr(masks, rows[i].SenderUserID, rows[i].SenderName)
			rows[i].SenderFullName = maskNamePtr(masks, rows[i].SenderUserID, rows[i].SenderFullName)
		}
	}
	return rows, total, nil
}

// CanSecurityViewAttachment сообщает, доступно ли конкретное вложение охраннику по тем же правилам,
// что и листинг (#706). Используется детальным эндпоинтом для 403 на чужое вложение. unrestricted
// (super/admin/носитель page.available, #976) проходит фильтр по местам, но гейт заявки
// (confirmation='Согласовано' + status IN ('В работе','Завершено')) применяется и к нему. LEFT JOIN
// applications (#1049): без него ручное вложение-сирота
// (application_id NULL) не прошло бы EXISTS и деталь давала бы 403 при видимом в списке - тот же
// набор ролей должен гейтить и список, и деталь.
func (s *applicationService) CanSecurityViewAttachment(ctx context.Context, userID int, unrestricted bool, attachmentID int) (bool, error) {
	where, args := securityVisibilityWhere(userID, unrestricted)
	sql := `SELECT EXISTS (
		SELECT 1 FROM attachments a
		LEFT JOIN applications app ON app.id = a.application_id
		WHERE a.id = ? AND ` + where + `)`
	queryArgs := append([]interface{}{attachmentID}, args...)

	var canView bool
	if err := s.db.WithContext(ctx).Raw(sql, queryArgs...).Scan(&canView).Error; err != nil {
		return false, fmt.Errorf("failed to check security attachment access: %w", err)
	}
	return canView, nil
}

// GetAvailableAttachmentByID возвращает заголовок одного вложения с краткой инфо родительской
// заявки для детального экрана "Доступные мне" (#706). Проекция совпадает с листингом
// (availableAttachmentSelect), без предиката доступа: вызывающий хендлер уже проверил видимость
// через CanSecurityViewAttachment. nil без ошибки означает "вложение не найдено" (явный сигнал
// для 404, реальные ошибки БД пробрасываются).
func (s *applicationService) GetAvailableAttachmentByID(ctx context.Context, attachmentID int) (*AvailableAttachment, error) {
	sql := `SELECT ` + availableAttachmentSelect + availableAttachmentFrom + `
		WHERE a.id = ?`

	var row AvailableAttachment
	if err := s.db.WithContext(ctx).Raw(sql, attachmentID).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch available attachment: %w", err)
	}
	if row.AttachmentID == 0 {
		return nil, nil
	}
	return &row, nil
}
