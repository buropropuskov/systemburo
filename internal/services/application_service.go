package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var allowedStatuses = map[string]bool{
	"Непрочитано": true, "В обработке": true, "Принята в работу": true,
	"На согласовании": true, "Не согласовано": true, "Согласовано": true,
	"Отклонена": true, "Завершена": true,
}

var allowedConfirmations = map[string]bool{
	"Согласование": true, "Согласовано": true, "Не согласовано": true,
}

// forwardAttachmentVisible ограничивает агрегаты-теги заявки вложениями, видимыми
// просматривающему с учётом пер-вложенного пересыла (#680): отправитель и пользователь
// без строк forward_attachments видят все вложения, получатель со строками - только
// перечисленные. Три плейсхолдера связываются с id просматривающего; ссылается на att.id
// (вложение тег-подзапроса) и a (заявка внешнего запроса).
const forwardAttachmentVisible = `(
		a.sender_user_id = ?
		OR NOT EXISTS (SELECT 1 FROM forward_attachments fa WHERE fa.application_id = a.id AND fa.recipient_user_id = ?)
		OR EXISTS (SELECT 1 FROM forward_attachments fav WHERE fav.application_id = a.id AND fav.recipient_user_id = ? AND fav.attachment_id = att.id)
	)`

// applicationsListSelect - общий список столбцов для листингов заявок (Центр, пагинация,
// заявки пользователя). Теги has_roof_access/has_free_parking учитывают видимость пересыла
// (forwardAttachmentVisible). Плейсхолдеры (?) связываются через applicationsListSelectArgs.
//
// var, а не const: has_open_supplement собирается из models.OpenSupplementStatuses - см.
// hasOpenSupplementPredicate. Плейсхолдеров он не добавляет, порядок аргументов не трогает.
var applicationsListSelect = `
		a.*,
		COALESCE(o.name, c.name) as organization_name,
		c.name as company_name,
		o.moderation_status as organization_moderation_status,
		c.moderation_status as company_moderation_status,
		format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
		format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
		u.is_important as sender_is_important,
		format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
		format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name,
		EXISTS (SELECT 1 FROM attachments att JOIN attachment_templates at2 ON at2.unique_attachment_id = att.unique_attachment_id AND at2.is_active = true WHERE att.application_id = a.id) as has_blank_template,
		EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?) as is_read,
		EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = a.id AND att.roof_access = true AND ` + forwardAttachmentVisible + `) as has_roof_access,
		EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = a.id AND att.free_parking = true AND ` + forwardAttachmentVisible + `) as has_free_parking,
		(SELECT COUNT(*) FROM application_blacklist_flags f
		  WHERE f.application_id = a.id
		    AND NOT EXISTS (SELECT 1 FROM application_blacklist_overrides o WHERE o.flag_id = f.id)
		    -- Снятый из чёрного списка запрет в счётчике не участвует: предупреждать не о чем.
		    AND (
		          (f.element_type = 'car' AND EXISTS (
		             SELECT 1 FROM vehicle_blacklists vb WHERE vb.id = f.matched_blacklist_id AND vb.is_active))
		       OR (f.element_type = 'employee' AND EXISTS (
		             SELECT 1 FROM person_blacklists pb WHERE pb.id = f.matched_blacklist_id AND pb.is_active))
		        )) as blacklist_flags_count,
		(
			EXISTS (SELECT 1 FROM application_questions q WHERE q.application_id = a.id
				AND q.author_user_id <> ?
				AND q.created_at > COALESCE((SELECT r.read_at FROM application_question_reads r WHERE r.question_id = q.id AND r.user_id = ?), to_timestamp(0)))
			OR EXISTS (SELECT 1 FROM application_answers ans WHERE ans.application_id = a.id
				AND ans.author_user_id <> ?
				AND ans.created_at > COALESCE((SELECT r.read_at FROM application_question_reads r WHERE r.question_id = ans.question_id AND r.user_id = ?), to_timestamp(0)))
		) as has_unseen_questions,
		EXISTS (SELECT 1 FROM application_files af WHERE af.application_id = a.id) as has_files,
		` + hasStatusUpdatePredicate + ` as has_status_update,
		` + hasOpenSupplementPredicate + ` as has_open_supplement
	`

// applicationsListSelectArgs связывает плейсхолдеры applicationsListSelect: is_read (1)
// всегда по реальному id просматривающего; теги видимости (по три на каждый из двух) -
// по forwardViewerID, который равен 0 для супер-админа (bypass фильтра пересыла, видит
// все теги - симметрично resolveForwardFilter в detail-пути).
func applicationsListSelectArgs(readUserID, forwardViewerID int) []interface{} {
	return []interface{}{
		readUserID,
		forwardViewerID, forwardViewerID, forwardViewerID,
		forwardViewerID, forwardViewerID, forwardViewerID,
		// has_unseen_questions (#973): per-топик отметка прочтения. На каждый из двух EXISTS -
		// author_user_id <> reader (свой вопрос/ответ не светит) + read по вопросу этого reader.
		readUserID, readUserID, readUserID, readUserID,
		// has_status_update (#1349): seen_at и read_at этого же reader (см. hasStatusUpdatePredicate).
		readUserID, readUserID,
	}
}

// forwardViewerID возвращает id для фильтра видимости тегов: 0 (bypass) для супер-админа,
// иначе реальный id - супер-админ видит все теги независимо от пересыла.
func forwardViewerID(user *models.User) int {
	if user.IsSuperAdmin {
		return 0
	}
	return user.ID
}

// ApplicationService определяет интерфейс бизнес-логики для работы с заявками.
type ApplicationService interface {
	// GetApplications возвращает список заявок для Центра заявок с фильтрацией.
	GetApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)
	GetAttachableApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)

	// GetApplicationsPaginated возвращает страницу заявок с общим количеством.
	GetApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error)

	// GetRegistryExtras добирает к списку заявок то, чего нет в строке Центра, но
	// нужно в выгруженном реестре (#1832): сколько людей и машин в заявке и границы
	// срока действия её вложений. Одним запросом на всю выборку - в списке заявок
	// сотни строк, и подзапрос на каждую превратил бы выгрузку в N+1.
	GetRegistryExtras(ctx context.Context, applicationIDs []int) (map[int]ApplicationRegistryExtras, error)

	// GetUserApplications возвращает заявки текущего пользователя с фильтрацией.
	GetUserApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)

	// GetUserApplicationsPaginated возвращает страницу заявок ЛК с общим количеством (#1158).
	GetUserApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error)

	// GetApplicationByID возвращает заявку по ID с обновлением статуса при первом прочтении.
	GetApplicationByID(ctx context.Context, username string, applicationID int) (map[string]interface{}, error)

	// GetApplicationDetails возвращает расширенную информацию о заявке. username -
	// смотрящий: от него зависит, попадёт ли в ответ заметка бюро (только принимающему,
	// см. application_bureau_note.go).
	GetApplicationDetails(ctx context.Context, username string, applicationID int) (map[string]interface{}, error)

	// SetBureauNote сохраняет заметку бюро по заявке; пустой текст снимает её.
	// Доступно только принимающим.
	SetBureauNote(ctx context.Context, username string, applicationID int, req SetBureauNoteRequest) (*BureauNoteView, error)

	// CreateApplication создаёт новую заявку. canOverrideOrganization - результат проверки
	// права KeyApplicationOrganizationOverride, см. SubmitCompleteApplication.
	CreateApplication(ctx context.Context, username string, req ApplicationCreateRequest, canOverrideOrganization bool) (*ApplicationCreateResponse, error)

	// SubmitCompleteApplication создаёт полную заявку с вложениями.
	// canOverrideOrganization - разрешено ли подающему указывать организацию и компанию,
	// отличные от своих (право KeyApplicationOrganizationOverride, проверяется в handler).
	// Флаг передаётся параметром, а не читается из тела: тело правит клиент.
	SubmitCompleteApplication(ctx context.Context, username string, req CompleteApplicationRequest, canOverrideOrganization bool) (*CompleteApplicationResponse, error)

	// UpdateApplication обновляет данные заявки.
	UpdateApplication(ctx context.Context, username string, applicationID int, req ApplicationUpdateRequest) (*ApplicationUpdateResponse, error)

	// ForwardApplication пересылает заявку ответственным/просматривающим.
	ForwardApplication(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req ForwardApplicationRequest) error

	// ApproveApplicationByUser согласование/отказ заявки пользователем.
	ApproveApplicationByUser(ctx context.Context, username string, applicationID int, req UserApprovalRequest) error
	// OverrideBlacklistFlag фиксирует "всё равно пропустить" по помеченному элементу (#481),
	// снимая блокировку согласования по этому флагу. Только ответственный, идемпотентно.
	OverrideBlacklistFlag(ctx context.Context, username string, applicationID int, req OverrideBlacklistFlagRequest) error

	// DeleteBlacklistOverride снимает ранее подтверждённый пропуск по флагу (#481), снова
	// блокируя согласование по нему. Право шире, чем создание: ответственный ИЛИ принимающий.
	DeleteBlacklistOverride(ctx context.Context, username string, applicationID, flagID int) error

	// CheckApprovalStatus проверяет текущий статус согласования заявки.
	CheckApprovalStatus(ctx context.Context, applicationID int) (*ApprovalStatusResponse, error)

	// TakeApplicationToWork принятие заявки в работу или отказ.
	TakeApplicationToWork(ctx context.Context, username string, applicationID int, req TakeToWorkRequest) error

	// RevokeApplicationFromWork отзыв заявки из работы.
	RevokeApplicationFromWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error

	// RestoreApplicationToWork возврат заявки в обработку.
	RestoreApplicationToWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error

	// WithdrawApplication отзыв своей заявки отправителем (#951).
	WithdrawApplication(ctx context.Context, username string, applicationID int) error

	// CreateSupplement добавляет людей, машины или ТМЦ во вложения уже поданной заявки
	// (#1685). Доступно автору заявки и супер-админу; статусы заявки и голоса основного
	// круга при этом не откатываются - повторный круг живёт отдельным дополнением.
	CreateSupplement(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req CreateSupplementRequest) (*CreateSupplementResponse, error)

	// GetApplicationSupplements возвращает раунды дополнения заявки (новые сверху).
	GetApplicationSupplements(ctx context.Context, applicationID int) ([]SupplementInfo, error)

	// ApproveSupplement - голос согласующего по раунду дополнения (#1685). Пишет итог в
	// application_supplements.status; confirmation и status самой заявки не двигает.
	ApproveSupplement(ctx context.Context, username string, applicationID, supplementID int, req SupplementApprovalRequest) (*SupplementVoteResponse, error)

	// RevokeSupplementApproval возвращает голос по раунду дополнения в pending (#1685).
	RevokeSupplementApproval(ctx context.Context, username string, applicationID, supplementID int, req SupplementRevokeApprovalRequest) (*SupplementVoteResponse, error)

	// DecideSupplement - решение принимающего по согласованному раунду (#1685): принятие
	// поднимает на КПП строки ЭТОГО раунда, отказ оставляет их неактивными навсегда.
	// confirmation и status самой заявки не двигает ни одна ветка.
	DecideSupplement(ctx context.Context, username string, applicationID, supplementID int, req SupplementDecisionRequest) (*SupplementDecisionResponse, error)

	// CancelSupplement снимает незакрытый раунд по воле автора заявки (#1685).
	CancelSupplement(ctx context.Context, username string, applicationID, supplementID int, isSuperAdmin bool, req SupplementCancelRequest) (*SupplementDecisionResponse, error)

	// GetApplicationResponsibleUsers возвращает ответственных пользователей заявки.
	GetApplicationResponsibleUsers(ctx context.Context, applicationID int) ([]ResponsibleUserInfo, error)

	// GetApplicationParticipants возвращает всех участников заявки одним списком:
	// отправителя, принявшего в работу, согласующих, ответственных и читателей - с
	// ролями, контактами и состоянием голоса. Персональные данные маскируются.
	GetApplicationParticipants(ctx context.Context, applicationID int) ([]ApplicationParticipant, error)

	// GetApplicationHistory возвращает историю заявки. username - смотрящий: записи о
	// заметке бюро выдаются только принимающему, остальным лента приходит без них.
	GetApplicationHistory(ctx context.Context, applicationID int, username string) ([]ApplicationHistoryItem, error)

	// GetForwardMessages возвращает ветку заявки (#967) - все пересылки с получателями
	// и сопроводительным текстом (если был), хронологически (старые сверху).
	GetForwardMessages(ctx context.Context, applicationID int) ([]ForwardMessageItem, error)

	// AddHistoryEntry добавляет запись в историю заявки.
	AddHistoryEntry(ctx context.Context, req AddHistoryEntryRequest) error

	// RevokeApproval отзывает ранее данное согласование.
	RevokeApproval(ctx context.Context, username string, applicationID int, req RevokeApprovalRequest) (*RevokeApprovalResponse, error)

	// GetApplicationViewers возвращает просматривающих заявки.
	GetApplicationViewers(ctx context.Context, applicationID int) ([]ViewerWithUser, error)

	// GetApplicationAttachments возвращает вложения заявки, видимые получателю пересылки
	// (viewerUserID; 0 - супер-админ, без фильтра).
	GetApplicationAttachments(ctx context.Context, applicationID, viewerUserID int) ([]AttachmentInfo, error)

	// CanViewAttachment сообщает, доступно ли вложение просматривающему с учётом пересыла.
	CanViewAttachment(ctx context.Context, applicationID, attachmentID, viewerUserID int) (bool, error)

	// GetAttachmentCars возвращает автомобили вложения. scope - см. SupplementScope:
	// охране идёт только допущенное на КПП, автору заявки - весь состав.
	GetAttachmentCars(ctx context.Context, attachmentID int, scope SupplementScope) ([]CarWithPlaces, error)

	// GetAttachmentEmployees возвращает сотрудников вложения. scope - см. SupplementScope.
	GetAttachmentEmployees(ctx context.Context, attachmentID int, scope SupplementScope) ([]EmployeeWithTables, error)

	// GetAttachmentItems возвращает ТМЦ вложения. scope - см. SupplementScope.
	GetAttachmentItems(ctx context.Context, attachmentID int, scope SupplementScope) ([]ItemInfo, error)

	// AssignElementTables назначает или снимает посты проезда/прохода у машин и
	// сотрудников заявки; доступно принимающему, пока заявка не закрыта (#1393).
	AssignElementTables(ctx context.Context, username string, applicationID int, req AssignElementTablesRequest) error
	// AssignCarUnloadPlaces назначает или снимает места разгрузки у машин заявки (#1393).
	AssignCarUnloadPlaces(ctx context.Context, username string, applicationID int, req AssignCarUnloadPlacesRequest) error
	// RemoveApplicationElements убирает людей или машины из поданной заявки; доступно
	// принимающему. Возвращает число реально убранных элементов.
	RemoveApplicationElements(ctx context.Context, username string, applicationID int, req RemoveApplicationElementsRequest) (int, error)
	// UpdateApplicationItemsStatus активирует все машины и сотрудников заявки (status->1) и
	// пишет историю попадания в таблицу проходной. username - актор истории.
	UpdateApplicationItemsStatus(ctx context.Context, applicationID int, username string) error

	// CheckExpiredAttachments проверяет и деактивирует истекшие вложения.
	CheckExpiredAttachments(ctx context.Context) error

	// MarkAsRead фиксирует прочтение заявки пользователем.
	MarkAsRead(ctx context.Context, applicationID int, username string) error

	// MarkStatusSeen помечает текущий статус заявки просмотренным пользователем (#1349):
	// гасит его флаг "статус обновился". Идемпотентно.
	MarkStatusSeen(ctx context.Context, username string, applicationID int) error

	// GetReads возвращает список пользователей, прочитавших заявку.
	GetReads(ctx context.Context, applicationID int) ([]models.ApplicationReadResponse, error)

	// GetUnreadCount возвращает количество непрочитанных заявок для пользователя.
	GetUnreadCount(ctx context.Context, username string) (*models.UnreadCountResponse, error)

	// GetUserStatusUpdatesCount возвращает число заявок ЛК с обновлённым статусом (#1349).
	GetUserStatusUpdatesCount(ctx context.Context, username string) (*models.StatusUpdatesCountResponse, error)

	// CanAccessApplication проверяет, имеет ли пользователь доступ к заявке.
	CanAccessApplication(ctx context.Context, applicationID int, username string, isSuperAdmin bool) bool

	// GetApplicationIDByAttachment возвращает ID заявки по ID вложения. Для manual-вложения
	// без заявки (#1049) возвращает 0 - вызыватели трактуют 0 как "нет заявки".
	GetApplicationIDByAttachment(ctx context.Context, attachmentID int) (int, error)
	// IsApplicationSender - подал ли заявку сам пользователь. Уже, чем
	// CanAccessApplication: доступ есть и у согласующих с получателями пересылки,
	// а сведения документов участников вводил в форму инициатор.
	IsApplicationSender(ctx context.Context, applicationID, userID int) (bool, error)

	// IsSecurityUser сообщает, является ли аккаунт типом security (резолв по user_types.code).
	IsSecurityUser(ctx context.Context, userID int) (bool, error)

	// GetAvailableAttachmentsForSecurity возвращает страницу вложений подтверждённых заявок,
	// доступных охраннику по совпадению мест (#706), и общее количество. unrestricted
	// (super/admin/носитель page.available, #976) - без фильтра по местам. filter (BE-S6)
	// опционально сужает выдачу поверх гейта видимости.
	GetAvailableAttachmentsForSecurity(ctx context.Context, userID int, unrestricted bool, filter AvailableAttachmentFilters, page, perPage int) ([]AvailableAttachment, int64, error)

	// CanSecurityViewAttachment сообщает, доступно ли конкретное вложение охраннику по тем же
	// правилам, что и листинг (#706). Для 403 на чужое вложение в детальном эндпоинте.
	CanSecurityViewAttachment(ctx context.Context, userID int, unrestricted bool, attachmentID int) (bool, error)

	// GetAvailableAttachmentByID возвращает заголовок вложения с инфо заявки для детали
	// "Доступные мне" (#706). nil без ошибки - вложение не найдено. Без проверки доступа,
	// вызывать после CanSecurityViewAttachment.
	GetAvailableAttachmentByID(ctx context.Context, attachmentID int) (*AvailableAttachment, error)

	// GetApplicationQuestions возвращает вопросы к заявке (#973) с вложенными ответами,
	// вложениями и ФИО авторов; вопросы новые сверху, ответы в хронологии треда.
	// forwardViewerID (#680): вложения вопроса скрываются, если недоступны читателю по
	// пер-вложенному пересылу; 0 - супер-админ (видит все). readerUserID - реальный id для
	// флага IsNew (per-топик отметка прочтения).
	GetApplicationQuestions(ctx context.Context, applicationID, forwardViewerID, readerUserID int) ([]QuestionWithAnswers, error)

	// CreateApplicationQuestion создаёт вопрос-топик (#973): гейт canAsk (не чистый sender),
	// история question_created, уведомление инициатору. Возвращает созданный вопрос.
	CreateApplicationQuestion(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req CreateQuestionRequest) (*QuestionWithAnswers, error)

	// CreateApplicationAnswer добавляет ответ в тред вопроса (#973): доступ - любой с доступом
	// к заявке; уведомляет участников обсуждения; в историю не пишет.
	CreateApplicationAnswer(ctx context.Context, username string, applicationID, questionID int, req CreateAnswerRequest) (*AnswerItem, error)

	// MarkQuestionsSeen обновляет last-seen пользователя по Q&A заявки (#973) - гасит маркер
	// "новые вопросы/ответы" в списке.
	MarkQuestionsSeen(ctx context.Context, username string, applicationID int) error

	// MarkQuestionRead помечает конкретный вопрос-топик прочитанным (#973) - гасит его новизну
	// для пользователя (per-топик отметка, недочитанные топики остаются новыми).
	MarkQuestionRead(ctx context.Context, username string, applicationID, questionID int) error

	// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1) -
	// точки изменения заявки ставят её на пересборку бланков после commit.
	SetBlankExportEnqueuer(e BlankExportEnqueuer)
}

// --- DTO: запросы ---

// ApplicationFilter параметры фильтрации списка заявок.
type ApplicationFilter struct {
	SearchQuery    *string `query:"search_query"`
	OrganizationID *int    `query:"organization_id"`
	CompanyID      *int    `query:"company_id"`
	// OrganizationIDs/CompanyIDs - мультивыбор справочников в Центре (#1398): comma-список
	// id -> IN. Живут рядом с одиночными OrganizationID/CompanyID, а не вместо них: те
	// по-прежнему шлёт ЛК (организация пользователя из профиля). Заданные одновременно
	// комбинируются по AND - каждый сужает выборку независимо.
	OrganizationIDs *string `query:"organization_ids"`
	CompanyIDs      *string `query:"company_ids"`
	// UnloadPlaceIDs - мультивыбор мест разгрузки (#1398): заявка проходит, если место
	// хотя бы одного её вложения попало в список. Источник - attachment_unload_places
	// (уровень вложения): для items это единственная привязка, для cars - дедуп-union
	// мест всех машин вложения (#706).
	UnloadPlaceIDs *string `query:"unload_place_ids"`
	// PassageTableIDs - мультивыбор таблиц проходной (#1398, фильтр "Проход"): заявка
	// проходит, если к таблице привязана хотя бы одна её машина (car_target_tables,
	// "Проезд") или сотрудник (employee_target_tables, "Места прохода").
	PassageTableIDs *string `query:"passage_table_ids"`
	Confirmation    *string `query:"confirmation"`
	Status          *string `query:"status"`
	// Unread - псевдо-фильтр "Непрочитано": заявки без записи в application_reads для
	// текущего пользователя (непрочитанность живёт там, а не в колонке a.status). В UI
	// комбинируется со статусами по OR. userID для предиката берётся не отсюда, а из
	// вызывающего сервиса (applyApplicationFilters получает его параметром).
	Unread *bool `query:"unread"`
	// StatusUpdated - псевдо-фильтр "Только с обновлённым статусом" (#1349, чип "Обновления"):
	// заявки, чей статус/подтверждение менялись после последнего просмотра пользователем.
	// requireRead различается по листингу (Центр требует прочтения, ЛК нет), поэтому фильтр
	// применяется не в applyApplicationFilters, а в конкретном листинге через
	// applyStatusUpdatedFilter.
	StatusUpdated *bool   `query:"status_updated"`
	DateFrom      *string `query:"date_from"`
	DateTo        *string `query:"date_to"`
	Archive       *bool   `query:"archive"`
	ActiveToday   *bool   `query:"active_today"`
	// SenderUserID сужает список до заявок, отправленных конкретным пользователем (#1158,
	// вкладка "Мои заявки" в ЛК). Опциональное AND-условие поверх access-фильтра - не
	// расширяет видимость (Центр это поле не использует, для него нейтрально).
	SenderUserID *int `query:"sender_user_id"`
}

// ApplicationCreateRequest тело запроса на создание простой заявки.
type ApplicationCreateRequest struct {
	OrganizationID *int    `json:"organization_id"`
	CompanyID      *int    `json:"company_id"`
	Message        *string `json:"message"`
	DataApproval   bool    `json:"data_approval"`
}

// CompleteApplicationRequest тело запроса на создание полной заявки с вложениями.
type CompleteApplicationRequest struct {
	Message *string `json:"message"`
	// Organization и Company - наименования организации и компании в прежнем контракте
	// подачи. Оставлены рядом с organization_name/company_name (#1437), потому что
	// бандл, уже загруженный в браузере пользователя, продолжает слать именно их:
	// читать значение следует через OrganizationTitle и CompanyTitle.
	Organization string  `json:"organization"`
	Company      *string `json:"company"`
	// OrganizationID и OrganizationName - контракт подачи после #1437: id, когда
	// организация выбрана (своя или из подсказок), наименование - когда введена руками.
	// Заполнять оба не нужно: при заданном id наименование не смотрится.
	OrganizationID   *int    `json:"organization_id"`
	OrganizationName *string `json:"organization_name"`
	// CompanyID и CompanyName - то же для компании.
	CompanyID         *int                 `json:"company_id"`
	CompanyName       *string              `json:"company_name"`
	ResponsiblePerson string               `json:"responsible_person" validate:"required"`
	ContactPhone      string               `json:"contact_phone" validate:"required"`
	DataApproval      bool                 `json:"data_approval"`
	Attachments       []AttachmentData     `json:"attachments"`
	RequiredUsers     *[]RequiredUserInput `json:"required_users"`
	// Readers - получатели-читатели заявки (#884): доступ только на просмотр.
	// Кладутся в application_viewers (как форвард-флоу), без права согласования.
	Readers *[]int `json:"readers"`
	// FileIDs - файлы, загруженные до подачи (#1721). Привязываются к заявке в
	// этой же транзакции: прикрепить файл после подачи нельзя, а неназванный в
	// подаче черновик убирает уборщик.
	FileIDs []int `json:"file_ids"`
}

// OrganizationTitle - введённое наименование организации: поле нового контракта, а при
// его отсутствии - прежнее organization (#1437).
func (r CompleteApplicationRequest) OrganizationTitle() string {
	if r.OrganizationName != nil {
		return *r.OrganizationName
	}
	return r.Organization
}

// CompanyTitle - введённое наименование компании, см. OrganizationTitle.
func (r CompleteApplicationRequest) CompanyTitle() string {
	if r.CompanyName != nil {
		return *r.CompanyName
	}
	if r.Company != nil {
		return *r.Company
	}
	return ""
}

// AttachmentData данные вложения при создании заявки.
type AttachmentData struct {
	AttachmentType        string  `json:"attachment_type"`
	AttachmentName        string  `json:"attachment_name"`
	AttachmentDisplayName string  `json:"attachment_display_name"`
	UniqueAttachmentID    int     `json:"unique_attachment_id"`
	EntryDateFrom         *string `json:"entry_date_from"`
	EntryDateTo           *string `json:"entry_date_to"`
	EntryTimeFrom         *string `json:"entry_time_from"`
	EntryTimeTo           *string `json:"entry_time_to"`
	RoofAccess            bool    `json:"roof_access"`
	FreeParking           bool    `json:"free_parking"`
	// UnloadPlaces - места разгрузки на уровне вложения (#706). Для items-вложений это
	// единственный источник мест; для cars дублирует дедуп-union мест всех машин вложения.
	UnloadPlaces []int                      `json:"unload_places,omitempty"`
	Data         AttachmentContentData      `json:"data"`
	CustomValues *[]models.CustomValueInput `json:"custom_values,omitempty"`
}

// AttachmentContentData содержимое вложения: машины, сотрудники или ТМЦ.
type AttachmentContentData struct {
	Vehicles  *[]VehicleInput  `json:"vehicles"`
	Employees *[]EmployeeInput `json:"employees"`
	Items     *[]ItemInput     `json:"items"`
}

// VehicleInput данные автомобиля при создании.
type VehicleInput struct {
	CarNumber    string  `json:"car_number"`
	CarBrand     string  `json:"car_brand"`
	MarkID       *int    `json:"mark_id"`
	UnloadPlace  *string `json:"unload_place"`
	UnloadPlaces []int   `json:"unload_places"`
	// TargetTables — таблицы «Проезд» (#1036): машина видна только в них. Зеркало
	// EmployeeInput.TargetTables.
	TargetTables []int `json:"passage_tables"`
	// PDConsent - см. EmployeeInput.PDConsent. У машин поле шаблона выключено по
	// умолчанию, флаг приходит только когда администратор его включил.
	PDConsent bool `json:"pd_consent"`
}

// EmployeeInput данные сотрудника при создании.
type EmployeeInput struct {
	LastName             string  `json:"last_name"`
	FirstName            string  `json:"first_name"`
	MiddleName           *string `json:"middle_name"`
	CitizenshipID        int     `json:"citizenship_id"`
	Position             string  `json:"position"`
	PassportSeriesNumber string  `json:"passport_series_number"`
	PatentNumber         *string `json:"patent_number"`
	OtherPermission      *string `json:"other_permission"`
	TargetTables         []int   `json:"target_tables"`
	// PDConsent - заявитель подтвердил, что субъект дал согласие на обработку своих
	// персональных данных. Только флаг: дату и автора отметки ставит сервер.
	PDConsent bool `json:"pd_consent"`
}

// ItemInput данные ТМЦ при создании.
type ItemInput struct {
	Name       string `json:"name"`
	Count      int    `json:"count"`
	OrderIndex int    `json:"order_index"`
}

// RequiredUserInput обязательный пользователь при создании заявки.
type RequiredUserInput struct {
	UserID           int  `json:"user_id"`
	RequiredApproval bool `json:"required_approval"`
}

// ApplicationUpdateRequest тело запроса на обновление заявки.
type ApplicationUpdateRequest struct {
	Confirmation       *string `json:"confirmation"`
	Status             *string `json:"status"`
	ResponsibleComment *string `json:"responsible_comment"`
}

// ForwardApplicationRequest тело запроса на пересылку заявки.
// AttachmentIDs - общий для всех получателей список вложений (#680): пустой -> получатели
// видят все вложения заявки; непустой -> в forward_attachments пишется строка на каждого
// получателя x каждое вложение, и при чтении получатель видит только перечисленные.
// Message - необязательное сопроводительное сообщение (#967): попадает в comment сводной
// записи forwarded, видно всем получателям и принимающим. Пустое после trim -> не пишется.
type ForwardApplicationRequest struct {
	Users         []ForwardUser `json:"users"`
	AttachmentIDs []int         `json:"attachment_ids"`
	Message       string        `json:"message" validate:"max=2000"`
}

// ForwardUser пользователь для пересылки. required_approval и can_view не могут быть оба true.
type ForwardUser struct {
	UserID           int  `json:"user_id"`
	RequiredApproval bool `json:"required_approval"`
	CanView          bool `json:"can_view"`
}

// UserApprovalRequest тело запроса на согласование заявки.
type UserApprovalRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Status  string  `json:"status" validate:"required,oneof=approved rejected"`
	Comment *string `json:"comment"`
}

// TakeToWorkRequest тело запроса на принятие заявки в работу.
type TakeToWorkRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Action  string  `json:"action" validate:"required,oneof=accept reject"`
	Comment *string `json:"comment"`
}

// RevokeFromWorkRequest тело запроса на отзыв заявки из работы.
type RevokeFromWorkRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Comment *string `json:"comment"`
}

// AddHistoryEntryRequest тело запроса на добавление записи в историю.
type AddHistoryEntryRequest struct {
	ApplicationID int              `json:"application_id" validate:"gte=1"`
	UserID        int              `json:"user_id" validate:"gte=1"`
	ActionType    string           `json:"action_type" validate:"required"`
	ActionStatus  *string          `json:"action_status"`
	OldValue      *string          `json:"old_value"`
	NewValue      *string          `json:"new_value"`
	Comment       *string          `json:"comment"`
	Metadata      *json.RawMessage `json:"metadata" swaggertype:"object"`
}

// RevokeApprovalRequest тело запроса на отзыв согласования.
type RevokeApprovalRequest struct {
	Comment *string `json:"comment"`
}

// --- DTO: ответы ---

// ApplicationWithDetails заявка с развёрнутой информацией для списков.
type ApplicationWithDetails struct {
	ID                   int        `json:"id"`
	ApplicationNumber    string     `json:"application_number"`
	Confirmation         string     `json:"confirmation"`
	SendingDatetime      time.Time  `json:"sending_datetime"`
	ReadingDatetime      *time.Time `json:"reading_datetime"`
	ConfirmationDatetime *time.Time `json:"confirmation_datetime"`
	OrganizationID       int        `json:"organization_id"`
	OrganizationName     string     `json:"organization_name"`
	CompanyID            *int       `json:"company_id"`
	CompanyName          string     `json:"company_name"`
	// Статус разбора организации и компании заявки (#1437). nil - записи нет (заявка
	// без компании), pending - наименование заведено подачей и ждёт разбора: по нему
	// деталь заявки показывает плашку принимающему.
	OrganizationModerationStatus *string `json:"organization_moderation_status"`
	CompanyModerationStatus      *string `json:"company_moderation_status"`
	SenderUserID                 int     `json:"sender_user_id"`
	SenderFullName               *string `json:"sender_full_name"`
	SenderName                   string  `json:"sender_name"`
	SenderIsImportant            bool    `json:"sender_is_important"`
	Message                      *string `json:"message"`
	Status                       string  `json:"status"`
	ResponsibleUserID            *int    `json:"responsible_user_id"`
	ResponsibleFullName          *string `json:"responsible_full_name"`
	ResponsibleName              string  `json:"responsible_name"`
	ResponsibleComment           *string `json:"responsible_comment"`
	DataApproval                 bool    `json:"data_approval"`
	HasBlankTemplate             bool    `json:"has_blank_template"`
	IsRead                       bool    `json:"is_read"`
	BlacklistFlagsCount          int     `json:"blacklist_flags_count"`
	HasRoofAccess                bool    `json:"has_roof_access"`
	HasFreeParking               bool    `json:"has_free_parking"`
	HasUnseenQuestions           bool    `json:"has_unseen_questions"`
	// HasFiles - к заявке приложены файлы (#1721). В списке Центра рисуется скрепкой:
	// признак, а не количество - в строке списка важно «есть или нет», состав виден в карточке.
	HasFiles        bool `json:"has_files"`
	HasStatusUpdate bool `json:"has_status_update"`
	// HasOpenSupplement - по заявке идёт незакрытый раунд дополнения (#1685). Статус и
	// согласование самой заявки при этом не откатываются, поэтому без отдельной метки
	// повторный круг в списке ничем себя не выдаёт.
	HasOpenSupplement bool `json:"has_open_supplement"`
}

// ApplicationRegistryExtras - то, чего нет в строке Центра, но нужно в выгруженном
// реестре (#1832): состав заявки числами и границы срока действия её вложений.
//
// Даты вложений хранятся строками (varchar с ISO-датой), поэтому границы берутся
// MIN/MAX по строке: для YYYY-MM-DD лексикографический порядок совпадает с
// хронологическим. Пустая строка - срок не задан.
type ApplicationRegistryExtras struct {
	PeopleCount   int
	CarsCount     int
	EntryDateFrom string
	EntryDateTo   string
}

// ApplicationCreateResponse ответ при создании заявки.
type ApplicationCreateResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	ApplicationID     int    `json:"application_id"`
	ApplicationNumber string `json:"application_number"`
}

// CompleteApplicationResponse ответ при создании полной заявки.
type CompleteApplicationResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	ApplicationID     int    `json:"application_id"`
	ApplicationNumber string `json:"application_number"`
}

// ApplicationUpdateResponse ответ при обновлении заявки.
type ApplicationUpdateResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	RowsAffected int64  `json:"rows_affected"`
}

// ApprovalStatusResponse ответ проверки статуса согласования.
type ApprovalStatusResponse struct {
	Confirmation *string `json:"confirmation"`
	Status       *string `json:"status"`
}

// ResponsibleUserInfo информация об ответственном пользователе с данными согласования.
type ResponsibleUserInfo struct {
	ID               int        `json:"id"`
	Username         string     `json:"username"`
	LastName         *string    `json:"last_name"`
	FirstName        *string    `json:"first_name"`
	MiddleName       *string    `json:"middle_name"`
	Position         *string    `json:"position"`
	IsPrimary        bool       `json:"is_primary"`
	RequiredApproval bool       `json:"required_approval"`
	ApprovalStatus   *string    `json:"approval_status"`
	ApprovalComment  *string    `json:"approval_comment"`
	ApprovalDatetime *time.Time `json:"approval_datetime"`
	// CreatedAt - момент назначения согласующего (не подача заявки: его могли
	// добавить позже). От него карточка заявки считает "не отвечает N дней" (#1315 S3),
	// так же считает ReminderService. ReminderCount - сколько напоминаний уже ушло.
	CreatedAt     time.Time `json:"created_at"`
	ReminderCount int       `json:"reminder_count"`
}

// ApplicationHistoryItem запись истории заявки.
type ApplicationHistoryItem struct {
	ID            int              `json:"id"`
	ApplicationID int              `json:"application_id"`
	UserID        int              `json:"user_id"`
	UserName      string           `json:"user_name"`
	LastName      *string          `json:"last_name"`
	FirstName     *string          `json:"first_name"`
	MiddleName    *string          `json:"middle_name"`
	ActionType    string           `json:"action_type"`
	ActionStatus  *string          `json:"action_status"`
	OldValue      *string          `json:"old_value"`
	NewValue      *string          `json:"new_value"`
	Comment       *string          `json:"comment"`
	CreatedAt     time.Time        `json:"created_at"`
	Metadata      *json.RawMessage `json:"metadata" swaggertype:"object"`
}

// RevokeApprovalResponse ответ при отзыве согласования.
type RevokeApprovalResponse struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	Confirmation *string `json:"confirmation"`
	Status       *string `json:"status"`
}

// ViewerWithUser просматривающий заявки с информацией о пользователе.
type ViewerWithUser struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id"`
	Username   string     `json:"username"`
	LastName   *string    `json:"last_name"`
	FirstName  *string    `json:"first_name"`
	MiddleName *string    `json:"middle_name"`
	Position   *string    `json:"position"`
	CreatedAt  *time.Time `json:"created_at"`
}

// AttachmentInfo информация о вложении заявки.
type AttachmentInfo struct {
	ID                          int        `json:"id"`
	AttachmentType              string     `json:"attachment_type"`
	AttachmentName              string     `json:"attachment_name"`
	AttachmentDisplayName       string     `json:"attachment_display_name"`
	EntryDateFrom               *string    `json:"entry_date_from"`
	EntryDateTo                 *string    `json:"entry_date_to"`
	EntryTimeFrom               *string    `json:"entry_time_from"`
	EntryTimeTo                 *string    `json:"entry_time_to"`
	RoofAccess                  bool       `json:"roof_access"`
	FreeParking                 bool       `json:"free_parking"`
	CreatedAt                   *time.Time `json:"created_at"`
	UniqueAttachmentID          *int       `json:"unique_attachment_id"`
	UniqueAttachmentTitle       *string    `json:"unique_attachment_title"`
	UniqueAttachmentDisplayName *string    `json:"unique_attachment_display_name"`
	HasTemplate                 bool       `json:"has_template"`
	// ArchiveStatus - статус строки реестра файлового архива (blank_exports)
	// для этого вложения (#1615, C6). Пусто, если строки нет вовсе: архив
	// выключен, тумблер типа выключен, либо заявка ещё не выгружалась.
	// DownloadBlanksModal показывает бейдж только на распознанных статусах.
	ArchiveStatus string              `json:"archive_status"`
	CustomValues  []CustomValueDetail `gorm:"-" json:"custom_values,omitempty"`
}

// CustomValueDetail значение кастомного поля для отображения.
type CustomValueDetail struct {
	FieldID int    `json:"field_id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
}

// CarWithPlaces автомобиль с привязанными местами разгрузки.
type CarWithPlaces struct {
	ID             int              `json:"id"`
	CarNumber      string           `json:"car_number"`
	CarBrand       string           `json:"car_brand"`
	UnloadPlace    *string          `json:"unload_place"`
	EntryDateFrom  *string          `json:"entry_date_from"`
	EntryTimeFrom  *string          `json:"entry_time_from"`
	EntryDateTo    *string          `json:"entry_date_to"`
	EntryTimeTo    *string          `json:"entry_time_to"`
	Organization   *string          `json:"organization"`
	OrganizationID *int             `json:"organization_id"`
	Company        *string          `json:"company"`
	CompanyID      *int             `json:"company_id"`
	UnloadPlaces   []UnloadPlaceRef `json:"unload_places"`
	// TargetTables - таблицы «Проезд», выбранные для машины (#1036), зеркало
	// EmployeeWithTables.TargetTables у сотрудников.
	TargetTables []TableInfoRef `json:"target_tables"`
	// BlacklistSimilar - предупреждение о возможном обходе ЧС (#481): заполнено, если
	// номер близок к активной записи ЧС (но не точное совпадение). nil - элемент чист.
	BlacklistSimilar *BlacklistFlagInfo `json:"blacklist_similar,omitempty"`
	// IsBlacklisted - точное попадание в действующий чёрный список; строка остаётся в
	// заявке, но показывается зачёркнутой.
	IsBlacklisted bool `json:"is_blacklisted"`
	// SupplementMark - каким раундом дополнения строка добавлена (#1685).
	SupplementMark
}

// BlacklistFlagInfo - данные per-element предупреждения о возможном обходе ЧС (#481)
// для детали заявки: id флага (для override), что в ЧС похоже, причина, близость [0..1]
// и подтверждён ли уже пропуск (override) - чтобы фронт показал статус и разблокировку.
type BlacklistFlagInfo struct {
	FlagID        int     `json:"flag_id"`
	MatchedValue  string  `json:"matched_value"`
	MatchedReason string  `json:"matched_reason"`
	Similarity    float64 `json:"similarity"`
	Overridden    bool    `json:"overridden"`
}

// UnloadPlaceRef ссылка на место разгрузки.
type UnloadPlaceRef struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// EmployeeWithTables сотрудник с привязанными таблицами.
type EmployeeWithTables struct {
	ID                   int            `json:"id"`
	LastName             string         `json:"last_name"`
	FirstName            string         `json:"first_name"`
	MiddleName           *string        `json:"middle_name"`
	Position             *string        `json:"position"`
	CitizenshipID        *int           `json:"citizenship_id"`
	CitizenshipName      *string        `json:"citizenship_name"`
	PassportSeriesNumber *string        `json:"passport_series_number"`
	PatentNumber         *string        `json:"patent_number"`
	OtherPermission      *string        `json:"other_permission"`
	EntryDateTo          *string        `json:"entry_date_to"`
	PassTime             *string        `json:"pass_time"`
	Organization         *string        `json:"organization"`
	OrganizationID       *int           `json:"organization_id"`
	Company              *string        `json:"company"`
	CompanyID            *int           `json:"company_id"`
	TargetTables         []TableInfoRef `json:"target_tables"`
	// BlacklistSimilar - предупреждение о возможном обходе ЧС (#481): заполнено, если
	// ФИО близко к активной записи ЧС (но не точное совпадение). nil - элемент чист.
	BlacklistSimilar *BlacklistFlagInfo `json:"blacklist_similar,omitempty"`
	// IsBlacklisted - точное попадание в действующий чёрный список. Из заявки строка
	// не исчезает (заявка - документ), но показывается зачёркнутой.
	IsBlacklisted bool `json:"is_blacklisted"`
	// SupplementMark - каким раундом дополнения строка добавлена (#1685).
	SupplementMark
}

// TableInfoRef ссылка на системную таблицу.
type TableInfoRef struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ItemInfo информация о ТМЦ.
type ItemInfo struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Count       int        `json:"count"`
	DateCreated *time.Time `json:"date_created"`
	// SupplementMark - каким раундом дополнения позиция добавлена (#1685).
	SupplementMark
}

// --- Реализация ---

type applicationService struct {
	db                  *gorm.DB
	permissionService   PermissionService
	notificationService NotificationService
	vehicleBlacklist    VehicleBlacklistService
	personBlacklist     PersonBlacklistService
	recorder            AuditRecorder
	realtimePublisher   realtime.Publisher
	tablesProducer      *TablesRefreshPublisher
	availableProducer   *AvailableRefreshPublisher
	permissionResolver  *PermissionResolver
	// blankExports - постановка заявки в очередь на выгрузку в файловый архив
	// (#1615, B1). Сеттер, а не конструкторская опция: BlankExportService поднимается
	// позже applicationService в cmd/server/main.go (зависит от attachmentBlankService).
	blankExports BlankExportEnqueuer
	// files - файлы, приложенные при подаче (#1721). Опциональна: без неё file_ids
	// в подаче не разбираются. Пределы заявки проверяются в момент привязки.
	files        ApplicationFileService
	fileMaxCount int
	fileMaxTotal int64
}

// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1). nil
// безопасен - enqueueArchiveExport становится no-op, раздел настроек архива при
// этом всё равно открывается (архив мог не подняться из-за каталога).
func (s *applicationService) SetBlankExportEnqueuer(e BlankExportEnqueuer) {
	s.blankExports = e
}

// enqueueArchiveExport ставит заявку в очередь на выгрузку бланков в файловый
// архив. Best-effort и синхронный (карта в памяти под мьютексом) - вызывается из
// каждой точки, где заявка реально изменилась после commit.
func (s *applicationService) enqueueArchiveExport(applicationID int, reason string) {
	if s.blankExports == nil {
		return
	}
	s.blankExports.EnqueueApplication(applicationID, reason)
}

// ApplicationServiceOption конфигурирует applicationService при создании.
type ApplicationServiceOption func(*applicationService)

// WithRealtimePublisher включает публикацию real-time сигналов обновления Центра
// заявок (#840). Опционально: без неё сигналы не шлются (тесты, offline).
func WithRealtimePublisher(p realtime.Publisher) ApplicationServiceOption {
	return func(s *applicationService) { s.realtimePublisher = p }
}

// WithApplicationTablesProducer включает публикацию tables.refresh при принятии
// заявки (#840 V2.2): активированные машины/сотрудники появляются в таблицах
// проходной live. Опционально.
func WithApplicationTablesProducer(p *TablesRefreshPublisher) ApplicationServiceOption {
	return func(s *applicationService) { s.tablesProducer = p }
}

// WithApplicationAvailableProducer включает публикацию available.new при переходе
// заявки в "Согласовано" (#840 V3): её вложения появляются в "Доступные мне" охраны.
func WithApplicationAvailableProducer(p *AvailableRefreshPublisher) ApplicationServiceOption {
	return func(s *applicationService) { s.availableProducer = p }
}

// WithApplicationPermissionResolver подключает резолвер прав (#1437, срез 8): по нему
// считается, кому из видящих заявку адресовано уведомление о новой записи справочника
// «на проверке». Без него уведомление не уходит - разбор остаётся доступен через плашку.
func WithApplicationPermissionResolver(r *PermissionResolver) ApplicationServiceOption {
	return func(s *applicationService) { s.permissionResolver = r }
}

// WithApplicationFiles подключает файлы, прикладываемые при подаче (#1721). Без
// неё поле file_ids в подаче игнорируется, и заявка создаётся без вложенных файлов.
func WithApplicationFiles(f ApplicationFileService, maxCount int, maxTotal int64) ApplicationServiceOption {
	return func(s *applicationService) {
		s.files = f
		s.fileMaxCount = maxCount
		s.fileMaxTotal = maxTotal
	}
}

// NewApplicationService создаёт экземпляр сервиса заявок.
func NewApplicationService(db *gorm.DB, permSvc PermissionService, notifSvc NotificationService, vehicleBL VehicleBlacklistService, personBL PersonBlacklistService, recorder AuditRecorder, opts ...ApplicationServiceOption) ApplicationService {
	s := &applicationService{
		db:                  db,
		permissionService:   permSvc,
		notificationService: notifSvc,
		vehicleBlacklist:    vehicleBL,
		personBlacklist:     personBL,
		recorder:            recorder,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// --- Основные методы ---

// GetApplications возвращает список заявок для Центра заявок с фильтрацией.
// maskApplicationNames подменяет в списке заявок ФИО принимающего заданной ему
// маской, а ФИО подавшего и принимающего - логином, если человек не давал согласия
// на обработку персональных данных. No-op, если маскировать некого.
func (s *applicationService) maskApplicationNames(ctx context.Context, rows []ApplicationWithDetails) {
	masks := loadNameMasks(ctx, s.db)
	if masks == nil {
		return
	}
	for i := range rows {
		rows[i].ResponsibleName = maskName(masks, rows[i].ResponsibleUserID, rows[i].ResponsibleName)
		rows[i].ResponsibleFullName = maskNamePtr(masks, rows[i].ResponsibleUserID, rows[i].ResponsibleFullName)
		sender := rows[i].SenderUserID
		rows[i].SenderName = maskName(masks, &sender, rows[i].SenderName)
		rows[i].SenderFullName = maskNamePtr(masks, &sender, rows[i].SenderFullName)
	}
}

// GetRegistryExtras добирает состав и сроки по списку заявок одним запросом.
// Отдельным методом, а не полем ApplicationWithDetails: строке Центра эти числа не
// нужны, а лишний GROUP BY на каждом открытии списка платили бы все.
func (s *applicationService) GetRegistryExtras(ctx context.Context, applicationIDs []int) (map[int]ApplicationRegistryExtras, error) {
	out := make(map[int]ApplicationRegistryExtras, len(applicationIDs))
	if len(applicationIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		ApplicationID int
		PeopleCount   int
		CarsCount     int
		EntryDateFrom string
		EntryDateTo   string
	}
	// COUNT(DISTINCT) обязателен: у заявки несколько вложений, и join людей с
	// машинами размножает строки друг друга (декартово произведение внутри группы).
	err := s.db.WithContext(ctx).
		Table("attachments AS at").
		Select(`at.application_id AS application_id,
			COUNT(DISTINCT e.id) AS people_count,
			COUNT(DISTINCT c.id) AS cars_count,
			COALESCE(MIN(NULLIF(at.entry_date_from, '')), '') AS entry_date_from,
			COALESCE(MAX(NULLIF(at.entry_date_to, '')), '') AS entry_date_to`).
		Joins("LEFT JOIN employees e ON e.attachment_id = at.id").
		Joins("LEFT JOIN cars c ON c.attachment_id = at.id").
		Where("at.application_id IN ?", applicationIDs).
		Group("at.application_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("сводка вложений для реестра заявок: %w", err)
	}

	for _, r := range rows {
		out[r.ApplicationID] = ApplicationRegistryExtras{
			PeopleCount:   r.PeopleCount,
			CarsCount:     r.CarsCount,
			EntryDateFrom: r.EntryDateFrom,
			EntryDateTo:   r.EntryDateTo,
		}
	}
	return out, nil
}

func (s *applicationService) GetApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Table("applications a").
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyApplicationAccessFilter(query, user.ID, isApprover)

	query = applyApplicationFilters(query, filter, true, user.ID)
	query = applyStatusUpdatedFilter(query, user.ID, filter.StatusUpdated, true)
	query = query.Order("a.sending_datetime DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	s.maskApplicationNames(ctx, rows)
	return rows, nil
}

// GetAttachableApplications возвращает активные согласованные заявки для привязки
// ручного вложения (#1049 режим-2). В ОТЛИЧИЕ от GetApplications НЕ применяет
// applyApplicationAccessFilter: super/admin должен видеть ВСЕ заявки для привязки,
// не только свои (автор/ответственный/наблюдатель/принимающий). Гейт page.admin
// стоит на роуте (requireAdmin), поэтому сюда доходит только super/admin - метод
// сам доступ не проверяет. Видимость Центра (GetApplications) при этом не меняется.
// Список жёстко ограничен confirmation='Согласовано' AND status='В работе' (BE-привязка
// принимает только такие цели, loadActiveApprovedApp), фильтр статуса игнорируется.
func (s *applicationService) GetAttachableApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	approved := models.ConfirmationApproved
	inWork := models.StatusInWork
	filter.Confirmation = &approved
	filter.Status = &inWork

	query := s.db.WithContext(ctx).Table("applications a").
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	// Намеренно БЕЗ applyApplicationAccessFilter - привязка это admin-операция.
	query = applyApplicationFilters(query, filter, true, user.ID)
	query = query.Order("a.sending_datetime DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок для привязки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	s.maskApplicationNames(ctx, rows)
	return rows, nil
}

// buildApplicationsBaseQuery строит базовый запрос с джойнами и фильтрами без Select и Order.
func (s *applicationService) buildApplicationsBaseQuery(ctx context.Context, userID int, isApprover bool, filter ApplicationFilter) *gorm.DB {
	query := s.db.WithContext(ctx).Table("applications a").
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyApplicationAccessFilter(query, userID, isApprover)

	query = applyApplicationFilters(query, filter, true, userID)
	// Центр: чип "Обновления" показывает только прочитанные заявки (requireRead=true).
	return applyStatusUpdatedFilter(query, userID, filter.StatusUpdated, true)
}

// GetApplicationsPaginated возвращает страницу заявок с общим количеством.
func (s *applicationService) GetApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := s.buildApplicationsBaseQuery(ctx, user.ID, isApprover, filter)
	if err := countQuery.Count(&total).Error; err != nil {
		slog.Error("Ошибка подсчёта заявок", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	offset := (page - 1) * perPage
	dataQuery := s.buildApplicationsBaseQuery(ctx, user.ID, isApprover, filter)
	dataQuery = dataQuery.
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Order("a.sending_datetime DESC").
		Offset(offset).
		Limit(perPage)

	rows := make([]ApplicationWithDetails, 0)
	if err := dataQuery.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок (paginated)", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	s.maskApplicationNames(ctx, rows)
	return rows, total, nil
}

// applyUserApplicationsAccessFilter ограничивает видимость ЛК: заявки, отправленные
// самим пользователем, ИЛИ заявки его организации (виден "Заявки организации (отдела)").
// Без этого фильтра GetUserApplications отдавал вообще ВСЕ заявки системы (пользователь
// без organization_id видел бы и чужие) - клиент лишь ОТОБРАЖАЛ подмножество через
// currentFilter (my/organization), не ограничивая реальный доступ к данным (IDOR).
// organizationID берётся из БД (user.OrganizationID), не из query - клиент повлиять
// не может. nil organizationID (пользователь без организации) сужает до sender-only.
func applyUserApplicationsAccessFilter(query *gorm.DB, userID int, organizationID *int) *gorm.DB {
	if organizationID != nil {
		return query.Where("a.sender_user_id = ? OR a.organization_id = ?", userID, *organizationID)
	}
	return query.Where("a.sender_user_id = ?", userID)
}

// buildUserApplicationsBaseQuery строит базовый запрос ЛК с джойнами, access-фильтром
// и фильтрами без Select/Order - зеркало buildApplicationsBaseQuery (Центр), но с
// applyUserApplicationsAccessFilter вместо applyApplicationAccessFilter (#1158).
func (s *applicationService) buildUserApplicationsBaseQuery(ctx context.Context, user *models.User, filter ApplicationFilter) *gorm.DB {
	query := s.db.WithContext(ctx).Table("applications a").
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyUserApplicationsAccessFilter(query, user.ID, user.OrganizationID)

	query = applyApplicationFilters(query, filter, true, user.ID)
	// ЛК: у отправителя нет строк application_reads, гейт прочтения не нужен (requireRead=false).
	return applyStatusUpdatedFilter(query, user.ID, filter.StatusUpdated, false)
}

// GetUserApplications возвращает заявки текущего пользователя с фильтрацией (legacy,
// полный список без пагинации - обратная совместимость для вызовов без per_page).
func (s *applicationService) GetUserApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	query := s.buildUserApplicationsBaseQuery(ctx, user, filter).
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Order("a.sending_datetime DESC, a.id DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения пользовательских заявок", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	s.maskApplicationNames(ctx, rows)
	return rows, nil
}

// GetUserApplicationsPaginated возвращает страницу заявок ЛК с общим количеством
// (#1158 срез 4, бесшовная подгрузка). a.id DESC - вторичный ключ сортировки: без
// него офсет-пагинация по неуникальному sending_datetime (несколько заявок в одну
// секунду) могла бы дублировать/пропускать строки между страницами.
func (s *applicationService) GetUserApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := s.buildUserApplicationsBaseQuery(ctx, user, filter)
	if err := countQuery.Count(&total).Error; err != nil {
		slog.Error("Ошибка подсчёта пользовательских заявок", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	offset := (page - 1) * perPage
	dataQuery := s.buildUserApplicationsBaseQuery(ctx, user, filter).
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Order("a.sending_datetime DESC, a.id DESC").
		Offset(offset).
		Limit(perPage)

	rows := make([]ApplicationWithDetails, 0)
	if err := dataQuery.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения пользовательских заявок (paginated)", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	s.maskApplicationNames(ctx, rows)
	return rows, total, nil
}

// GetApplicationByID возвращает заявку по ID с обновлением статуса при первом прочтении.
func (s *applicationService) GetApplicationByID(ctx context.Context, username string, applicationID int) (map[string]interface{}, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем заявку с JOINами
	var row struct {
		models.Application
		OrganizationName    *string `gorm:"column:organization_name"`
		CompanyName         *string `gorm:"column:company_name"`
		SenderFullName      *string `gorm:"column:sender_full_name"`
		SenderName          *string `gorm:"column:sender_name"`
		SenderIsImportant   bool    `gorm:"column:sender_is_important"`
		ResponsibleFullName *string `gorm:"column:responsible_full_name"`
		ResponsibleName     *string `gorm:"column:responsible_name"`
	}

	result := tx.Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
			format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
			u.is_important as sender_is_important,
			format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
			format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id").
		Where("a.id = ?", applicationID).
		First(&row)

	if result.Error != nil {
		tx.Rollback()
		if result.Error == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Обновляем статус при первом прочтении не отправителем. Этот переход НЕ бампает
	// status_updated_at (#1349): смена от факта открытия - шум, а не событие для участников.
	if row.Status != nil && *row.Status == "Непрочитано" && row.SenderUserID != user.ID {
		if err := tx.Exec("UPDATE applications SET status = 'В обработке', reading_datetime = NOW() WHERE id = ?", applicationID).Error; err != nil {
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating application status")
		}
		// Записываем прочтение в историю
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "read", &user.ID,
			applicationAuditDetails{OldValue: ptrString("Непрочитано"), NewValue: ptrString("В обработке")})
	}

	// Просмотр детали гасит флаг "статус обновился" для смотрящего (#1349). Здесь - путь
	// deep-link (?open=); основной путь открытия детали - GET /:id/details (MarkStatusSeen).
	if err := tx.Exec(statusViewUpsert, applicationID, user.ID).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark status seen")
	}

	// Получаем ответственных
	responsibles, err := s.fetchResponsibleUsers(ctx, tx, applicationID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	orgName := ""
	if row.OrganizationName != nil {
		orgName = *row.OrganizationName
	}
	companyName := ""
	if row.CompanyName != nil {
		companyName = *row.CompanyName
	}
	senderName := ""
	if row.SenderName != nil {
		senderName = *row.SenderName
	}
	responsibleName := ""
	if row.ResponsibleName != nil {
		responsibleName = *row.ResponsibleName
	}

	// Маскировка ФИО в детали: заданная маска принимающего и логин вместо ФИО у тех,
	// кто не давал согласия на обработку персональных данных.
	masks := loadNameMasks(ctx, s.db)
	responsibleName = maskName(masks, row.ResponsibleUserID, responsibleName)
	responsibleFullName := maskNamePtr(masks, row.ResponsibleUserID, row.ResponsibleFullName)
	senderName = maskName(masks, &row.SenderUserID, senderName)
	senderFullName := maskNamePtr(masks, &row.SenderUserID, row.SenderFullName)

	response := map[string]interface{}{
		"id":                    row.ID,
		"application_number":    row.ApplicationNumber,
		"confirmation":          row.Confirmation,
		"sending_datetime":      row.SendingDatetime,
		"reading_datetime":      row.ReadingDatetime,
		"confirmation_datetime": row.ConfirmationDatetime,
		"organization_id":       row.OrganizationID,
		"organization_name":     orgName,
		"company_id":            row.CompanyID,
		"company_name":          companyName,
		"sender_user_id":        row.SenderUserID,
		"sender_full_name":      senderFullName,
		"sender_name":           senderName,
		"sender_is_important":   row.SenderIsImportant,
		"message":               row.Message,
		"status":                row.Status,
		"responsible_user_id":   row.ResponsibleUserID,
		"responsible_full_name": responsibleFullName,
		"responsible_name":      responsibleName,
		"responsible_comment":   row.ResponsibleComment,
		"data_approval":         row.DataApproval,
		"responsible_users":     responsibles,
	}

	return response, nil
}

// GetApplicationDetails возвращает расширенную информацию о заявке.
func (s *applicationService) GetApplicationDetails(ctx context.Context, username string, applicationID int) (map[string]interface{}, error) {
	var row struct {
		models.Application
		OrganizationName *string `gorm:"column:organization_name"`
		CompanyName      *string `gorm:"column:company_name"`
		// Статусы разбора (#1437) - те же, что в листингах: деталь перечитывается по
		// live-сигналу, и без них плашка разбора висела бы после чужого решения.
		OrganizationModerationStatus *string `gorm:"column:organization_moderation_status"`
		CompanyModerationStatus      *string `gorm:"column:company_moderation_status"`
		SenderFullName               *string `gorm:"column:sender_full_name"`
		SenderName                   *string `gorm:"column:sender_name"`
		SenderIsImportant            bool    `gorm:"column:sender_is_important"`
		ResponsibleFullName          *string `gorm:"column:responsible_full_name"`
		ResponsibleName              *string `gorm:"column:responsible_name"`
	}

	result := s.db.WithContext(ctx).Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			o.moderation_status as organization_moderation_status,
			c.moderation_status as company_moderation_status,
			format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
			format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
			u.is_important as sender_is_important,
			format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
			format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id").
		Where("a.id = ?", applicationID).
		First(&row)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	responsibles, _ := s.fetchResponsibleUsers(ctx, s.db.WithContext(ctx), applicationID)

	// Заметка бюро едет в ответ только принимающему. Роль глобальная, поэтому смотрящего
	// резолвим здесь, а не полагаемся на CanAccessApplication в хендлере: тот пускает к
	// заявке ещё и заявителя, согласующих и получателей пересылки.
	viewer, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	viewerIsApprover, err := s.isApprover(ctx, viewer.ID)
	if err != nil {
		return nil, err
	}
	var bureauNote *BureauNoteView
	if viewerIsApprover {
		if bureauNote, err = s.loadBureauNote(ctx, applicationID); err != nil {
			return nil, err
		}
	}

	// Зеркало гейта согласования (#481): пока есть помеченные элементы без override,
	// фронт держит кнопку "Согласовать" заблокированной. Источник правды - тот же
	// hasUnoverriddenBlacklistFlags, что блокирует согласование на бэке (409).
	blacklistBlocked, err := hasUnoverriddenBlacklistFlags(ctx, s.db, applicationID)
	if err != nil {
		return nil, err
	}

	// Повторный круг по дополнению (#1685). Статус и согласование заявки он не двигает -
	// без этих двух полей карточка не отличит идущий раунд от его отсутствия.
	masks := loadNameMasks(ctx, s.db)
	openSupplement, err := s.loadOpenSupplement(ctx, applicationID, masks)
	if err != nil {
		return nil, err
	}
	supplementsCount, err := s.countSupplements(ctx, applicationID)
	if err != nil {
		return nil, err
	}

	orgName := ""
	if row.OrganizationName != nil {
		orgName = *row.OrganizationName
	}
	companyName := ""
	if row.CompanyName != nil {
		companyName = *row.CompanyName
	}
	senderName := ""
	if row.SenderName != nil {
		senderName = *row.SenderName
	}
	responsibleName := ""
	if row.ResponsibleName != nil {
		responsibleName = *row.ResponsibleName
	}

	// Маскировка ФИО в детали: заданная маска принимающего и логин вместо ФИО у тех,
	// кто не давал согласия на обработку персональных данных. masks загружены выше -
	// их же получает автор открытого раунда, второй раз справочник не тянем.
	responsibleName = maskName(masks, row.ResponsibleUserID, responsibleName)
	responsibleFullName := maskNamePtr(masks, row.ResponsibleUserID, row.ResponsibleFullName)
	senderName = maskName(masks, &row.SenderUserID, senderName)
	senderFullName := maskNamePtr(masks, &row.SenderUserID, row.SenderFullName)

	response := map[string]interface{}{
		"id":                    row.ID,
		"application_number":    row.ApplicationNumber,
		"confirmation":          row.Confirmation,
		"sending_datetime":      row.SendingDatetime,
		"reading_datetime":      row.ReadingDatetime,
		"confirmation_datetime": row.ConfirmationDatetime,
		"organization_id":       row.OrganizationID,
		"organization_name":     orgName,
		"company_id":            row.CompanyID,
		"company_name":          companyName,
		"sender_user_id":        row.SenderUserID,
		"sender_full_name":      senderFullName,
		"sender_name":           senderName,
		"sender_is_important":   row.SenderIsImportant,
		"message":               row.Message,
		"status":                row.Status,
		"responsible_user_id":   row.ResponsibleUserID,
		"responsible_full_name": responsibleFullName,
		"responsible_name":      responsibleName,
		"responsible_comment":   row.ResponsibleComment,
		"data_approval":         row.DataApproval,
		"responsible_users":     responsibles,

		// Статус разбора наименования (#1437): по нему деталь показывает плашку разбора.
		"organization_moderation_status": row.OrganizationModerationStatus,
		"company_moderation_status":      row.CompanyModerationStatus,

		"has_unoverridden_blacklist_flags": blacklistBlocked,

		// Дополнение заявки (#1685): открытый раунд (null - идущего нет) и общее число
		// раундов, включая закрытые.
		"open_supplement":   openSupplement,
		"supplements_count": supplementsCount,
	}

	applyBureauNoteVisibility(response, bureauNote, viewerIsApprover)

	return response, nil
}

// CreateApplication создаёт новую заявку с назначением ответственных.
func (s *applicationService) CreateApplication(ctx context.Context, username string, req ApplicationCreateRequest, canOverrideOrganization bool) (*ApplicationCreateResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if !req.DataApproval {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Data approval is required")
	}

	// Организация и компания приходят готовыми id, поэтому право override сверяем прямо
	// здесь - тем же гейтом, что и в подаче полной заявки (#1437).
	scope := applicantScope{
		userID:         user.ID,
		organizationID: user.OrganizationID,
		companyID:      user.CompanyID,
		canOverride:    canOverrideOrganization,
	}
	if req.OrganizationID != nil {
		if err := ensureDirectoryAllowed(organizationRef, scope, *req.OrganizationID); err != nil {
			return nil, err
		}
	}
	if req.CompanyID != nil {
		if err := ensureDirectoryAllowed(companyRef, scope, *req.CompanyID); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	applicationNumber := nextApplicationNumber(s.db.WithContext(ctx))

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	app := models.Application{
		ApplicationNumber: &applicationNumber,
		OrganizationID:    safeDerefInt(req.OrganizationID),
		CompanyID:         req.CompanyID,
		SenderUserID:      user.ID,
		Message:           req.Message,
		DataApproval:      ptrString("true"),
		Status:            ptrString("Непрочитано"),
		Confirmation:      ptrString("Согласование"),
		SendingDatetime:   &now,
	}

	if err := tx.Create(&app).Error; err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания заявки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating application")
	}

	// Собираем ответственных из организации и компании
	type respUser struct {
		UserID    int
		IsPrimary bool
	}
	var responsibleUsers []respUser
	var primaryResponsibleID *int

	if req.OrganizationID != nil {
		var orgResp []struct {
			UserID    int  `gorm:"column:user_id"`
			IsPrimary bool `gorm:"column:is_primary"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary FROM organization_users WHERE organization_id = ?", *req.OrganizationID).Scan(&orgResp)
		for _, r := range orgResp {
			responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary})
			if r.IsPrimary {
				primaryResponsibleID = &r.UserID
			}
		}
	}

	if req.CompanyID != nil {
		var compResp []struct {
			UserID    int  `gorm:"column:user_id"`
			IsPrimary bool `gorm:"column:is_primary"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary FROM companies_users WHERE company_id = ?", *req.CompanyID).Scan(&compResp)
		for _, r := range compResp {
			exists := false
			for _, ru := range responsibleUsers {
				if ru.UserID == r.UserID {
					exists = true
					break
				}
			}
			if !exists {
				responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary})
				if r.IsPrimary && primaryResponsibleID == nil {
					primaryResponsibleID = &r.UserID
				}
			}
		}
	}

	if primaryResponsibleID != nil {
		tx.Exec("UPDATE applications SET responsible_user_id = ? WHERE id = ?", *primaryResponsibleID, app.ID)
	}

	for _, ru := range responsibleUsers {
		tx.Exec(`
			INSERT INTO application_responsible_users (application_id, user_id, is_primary, approval_status, created_at)
			VALUES (?, ?, ?, 'pending', NOW())
			ON CONFLICT (application_id, user_id) DO NOTHING
		`, app.ID, ru.UserID, ru.IsPrimary)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return &ApplicationCreateResponse{
		Success:           true,
		Message:           "Application created successfully",
		ApplicationID:     app.ID,
		ApplicationNumber: applicationNumber,
	}, nil
}

// validateBlacklist проверяет машины и людей заявки против активного ЧС (#443).
// Машины матчатся по номеру + mark_id (как и фронтовый /check); машины без mark_id
// ("по факту"/свободная марка) пропускаем - по mark_id в ЧС они попасть не могут.
// Люди - строгое совпадение ФИО. Возвращает 409 при первом совпадении.
//
// Вложения собираются в плоские списки ДО проверки, а не проверяются по одному
// (blank-import, срез A2A3): раньше каждое вложение проверялось отдельно, что на
// многовложенной заявке означало N отдельных загрузок ЧС вместо одной на весь запрос.
// Порядок ошибок (какая строка сообщается первой) не гарантирован при нескольких
// одновременных нарушениях в разных вложениях - гард всё равно отклонит заявку 409.
func (s *applicationService) validateBlacklist(ctx context.Context, req CompleteApplicationRequest) error {
	var vehicles []VehicleInput
	var employees []EmployeeInput
	for _, att := range req.Attachments {
		if att.Data.Vehicles != nil {
			vehicles = append(vehicles, *att.Data.Vehicles...)
		}
		if att.Data.Employees != nil {
			employees = append(employees, *att.Data.Employees...)
		}
	}
	return s.validateBlacklistEntries(ctx, vehicles, employees)
}

// validateBlacklistEntries - тот же гард на плоском наборе строк, без обёртки вложений.
// Дополнение заявки (#1685) добавляет людей и машины в уже существующее вложение и формы
// подачи не собирает, но обходить ЧС не вправе так же, как подача.
//
// На объёме (blank-import, срез A2A3) вход может нести тысячи строк - раньше на каждую
// шёл отдельный SELECT (Check/CheckByName), что превращало ЧС-гард в тысячи round-trip.
// Активные записи ЧС по объёму - десятки/сотни, а не тысячи, поэтому дешевле загрузить
// их ОДНИМ запросом на каждый тип и матчить в памяти, чем гонять запрос на каждую строку
// заявки. Семантика точного совпадения (LOWER(TRIM(...)) =) сохранена один в один.
func (s *applicationService) validateBlacklistEntries(ctx context.Context, vehicles []VehicleInput, employees []EmployeeInput) error {
	if len(vehicles) > 0 {
		idx, err := s.loadVehicleBlacklistIndex(ctx)
		if err != nil {
			return err
		}
		for _, v := range vehicles {
			// Машины из mark-дропдауна приходят с mark_id (строгий матч), выбранные из
			// существующих unique_cars - без mark_id, но с car_brand (имя марки): для них
			// fallback на матч по имени, иначе заблокированная машина прошла бы гард.
			var (
				match models.VehicleBlacklist
				found bool
			)
			switch {
			case v.MarkID != nil:
				match, found = idx.byMarkID[vehicleBlacklistKey(v.CarNumber, strconv.Itoa(*v.MarkID))]
			case strings.TrimSpace(v.CarBrand) != "":
				match, found = idx.byMarkName[vehicleBlacklistKey(v.CarNumber, normalizeBlacklistKey(v.CarBrand))]
			default:
				continue
			}
			if found {
				return echo.NewHTTPError(http.StatusConflict,
					fmt.Sprintf("Машина %s %s в чёрном списке: %s", v.CarNumber, v.CarBrand, match.Reason))
			}
		}
	}
	if len(employees) > 0 {
		idx, err := s.loadPersonBlacklistIndex(ctx)
		if err != nil {
			return err
		}
		for _, e := range employees {
			// Тихая деградация ЧС (#529): если ФИО скрыто конфигом - данных для
			// совпадения нет, матчить нечем, пропускаем (не падаем, не 500).
			if strings.TrimSpace(e.LastName) == "" && strings.TrimSpace(e.FirstName) == "" {
				continue
			}
			middleName := ""
			if e.MiddleName != nil {
				middleName = *e.MiddleName
			}
			match, found := idx[personBlacklistKey(e.LastName, e.FirstName, middleName)]
			if found {
				fio := strings.TrimSpace(fmt.Sprintf("%s %s %s", e.LastName, e.FirstName, middleName))
				return echo.NewHTTPError(http.StatusConflict,
					fmt.Sprintf("Человек %s в чёрном списке: %s", fio, match.Reason))
			}
		}
	}
	return nil
}

// vehicleBlacklistIndex - активные записи ЧС машин, проиндексированные под оба пути
// матчинга Check/CheckByName (mark_id и имя марки), чтобы validateBlacklistEntries не
// ходил в БД на каждую строку заявки.
type vehicleBlacklistIndex struct {
	byMarkID   map[string]models.VehicleBlacklist
	byMarkName map[string]models.VehicleBlacklist
}

// loadVehicleBlacklistIndex грузит ВСЕ активные записи ЧС машин одним запросом и строит
// индекс по обоим ключам матчинга. ORDER BY id ASC + "первый выигрывает" воспроизводит
// поведение исходного Check/CheckByName (First() без явного Order сортирует по PK).
func (s *applicationService) loadVehicleBlacklistIndex(ctx context.Context) (vehicleBlacklistIndex, error) {
	var rows []models.VehicleBlacklist
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("id asc").Find(&rows).Error; err != nil {
		slog.Error("Ошибка загрузки чёрного списка машин", "error", err)
		return vehicleBlacklistIndex{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки чёрного списка")
	}
	idx := vehicleBlacklistIndex{
		byMarkID:   make(map[string]models.VehicleBlacklist, len(rows)),
		byMarkName: make(map[string]models.VehicleBlacklist, len(rows)),
	}
	for _, r := range rows {
		keyID := vehicleBlacklistKey(r.CarNumber, strconv.Itoa(r.MarkID))
		if _, exists := idx.byMarkID[keyID]; !exists {
			idx.byMarkID[keyID] = r
		}
		keyName := vehicleBlacklistKey(r.CarNumber, normalizeBlacklistKey(r.MarkName))
		if _, exists := idx.byMarkName[keyName]; !exists {
			idx.byMarkName[keyName] = r
		}
	}
	return idx, nil
}

// loadPersonBlacklistIndex грузит ВСЕ активные записи ЧС людей одним запросом,
// индексированные по нормализованному ФИО (см. loadVehicleBlacklistIndex).
func (s *applicationService) loadPersonBlacklistIndex(ctx context.Context) (map[string]models.PersonBlacklist, error) {
	var rows []models.PersonBlacklist
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("id asc").Find(&rows).Error; err != nil {
		slog.Error("Ошибка загрузки чёрного списка людей", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки чёрного списка")
	}
	idx := make(map[string]models.PersonBlacklist, len(rows))
	for _, r := range rows {
		middle := ""
		if r.MiddleName != nil {
			middle = *r.MiddleName
		}
		key := personBlacklistKey(r.LastName, r.FirstName, middle)
		if _, exists := idx[key]; !exists {
			idx[key] = r
		}
	}
	return idx, nil
}

// normalizeBlacklistKey - тот же LOWER(TRIM(...)), что использовали Check/CheckByName в
// SQL, но в памяти: ключ индекса ЧС должен быть регистронезависим и без крайних пробелов.
func normalizeBlacklistKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func vehicleBlacklistKey(carNumber, marker string) string {
	return normalizeBlacklistKey(carNumber) + "|" + marker
}

func personBlacklistKey(lastName, firstName, middleName string) string {
	return normalizeBlacklistKey(lastName) + "|" + normalizeBlacklistKey(firstName) + "|" + normalizeBlacklistKey(middleName)
}

// requiredFieldKeys - ключи полей, которые админ ЯВНО настроил обязательными для
// вложения (override visible=true И required=true). Поля без override не валидируются:
// строгая проверка только для настроенных полей, существующие шаблоны не ломаются
// (решение владельца, #529 H-9).
func requiredFieldKeys(overrides []models.AttachmentFieldConfig) map[string]bool {
	req := make(map[string]bool, len(overrides))
	for _, o := range overrides {
		if o.Visible && o.Required {
			req[o.FieldKey] = true
		}
	}
	return req
}

// employeeFieldPresent сообщает, заполнено ли поле сотрудника с данным ключом реестра.
// Незнакомый ключ -> true (не валидируем то, чего не знаем).
func employeeFieldPresent(e EmployeeInput, key string) bool {
	switch key {
	case "last_name":
		return strings.TrimSpace(e.LastName) != ""
	case "first_name":
		return strings.TrimSpace(e.FirstName) != ""
	case "middle_name":
		return e.MiddleName != nil && strings.TrimSpace(*e.MiddleName) != ""
	case "passport":
		return strings.TrimSpace(e.PassportSeriesNumber) != ""
	case "position":
		return strings.TrimSpace(e.Position) != ""
	case "citizenship":
		return e.CitizenshipID > 0
	case "patent":
		// Патент удовлетворяется номером патента ИЛИ иным разрешением (как на фронте).
		return (e.PatentNumber != nil && strings.TrimSpace(*e.PatentNumber) != "") ||
			(e.OtherPermission != nil && strings.TrimSpace(*e.OtherPermission) != "")
	case "work_permission":
		// Делит поле OtherPermission с patent: если оба настроены required,
		// одно заполненное разрешение удовлетворяет обоим (как на фронте).
		return e.OtherPermission != nil && strings.TrimSpace(*e.OtherPermission) != ""
	case "target_tables":
		return len(e.TargetTables) > 0
	case PDConsentFieldKey:
		return e.PDConsent
	}
	return true
}

// vehicleFieldPresent сообщает, заполнено ли поле машины. "По факту" приходит непустым
// sentinel-ом ("По факту") в car_number/car_brand, поэтому by-fact проходит проверку.
func vehicleFieldPresent(v VehicleInput, key string) bool {
	switch key {
	case "number":
		return strings.TrimSpace(v.CarNumber) != ""
	case "mark":
		return v.MarkID != nil || strings.TrimSpace(v.CarBrand) != ""
	case "unloading_places":
		return len(v.UnloadPlaces) > 0
	case "passage_tables":
		return len(v.TargetTables) > 0
	case PDConsentFieldKey:
		return v.PDConsent
	}
	return true
}

// itemFieldPresent сообщает, заполнено ли поле ТМЦ.
func itemFieldPresent(i ItemInput, key string) bool {
	switch key {
	case "item_name":
		return strings.TrimSpace(i.Name) != ""
	case "quantity":
		return i.Count >= 1
	}
	return true
}

// consentAt и consentBy превращают флаг согласия субъекта на обработку персональных
// данных в пару «когда» и «кто». Время и автора ставит сервер: запрос несёт только флаг,
// иначе датой согласия можно было бы прислать что угодно. Флаг снят - обе величины NULL,
// отметки нет.
//
// Где стоит строгость: форма подачи не даёт добавить человека без галочки (поле
// pd_consent в реестре полей видимо и обязательно по умолчанию), а сервер отказывает
// только когда администратор ЯВНО настроил поле обязательным - тем же порядком, что и у
// прочих полей вложения (#529 H-9: строгая серверная проверка включается настройкой,
// иначе существующие шаблоны ломаются). Отдельная точка ввода - карточка реестра
// сотрудников: там согласие требуется всегда, см. uniqueEmployeeService.Create.
func consentAt(granted bool, at time.Time) *time.Time {
	if !granted {
		return nil
	}
	v := at
	return &v
}

func consentBy(granted bool, userID int) *int {
	if !granted {
		return nil
	}
	v := userID
	return &v
}

// validateConfiguredRequiredFields проверяет, что поля, явно настроенные админом
// обязательными (override visible+required), присутствуют в подаче. Скрытые и
// ненастроенные поля не валидируются. Запускается до транзакции (#529 H-9).
func (s *applicationService) validateConfiguredRequiredFields(ctx context.Context, req CompleteApplicationRequest) error {
	for _, att := range req.Attachments {
		if err := s.validateAttachmentRequiredFields(ctx, att.UniqueAttachmentID, att.AttachmentType, att.Data); err != nil {
			return err
		}
	}
	return nil
}

// validateAttachmentRequiredFields - та же проверка для содержимого ОДНОГО вложения.
// Дополнение заявки (#1685) сыплет строки в существующее вложение, а настройка полей
// живёт на его шаблоне: правила обязательности для добавленных строк те же, что при подаче.
func (s *applicationService) validateAttachmentRequiredFields(ctx context.Context, uniqueAttachmentID int, attachmentType string, data AttachmentContentData) error {
	var overrides []models.AttachmentFieldConfig
	if err := s.db.WithContext(ctx).
		Where("unique_attachment_id = ?", uniqueAttachmentID).
		Find(&overrides).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки настройки полей")
	}
	required := requiredFieldKeys(overrides)
	if len(required) == 0 {
		return nil
	}
	labels := fieldDefByKey(attachmentType)
	fail := func(key string) error {
		label := key
		if d, ok := labels[key]; ok {
			label = d.Label
		}
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Поле «%s» обязательно для заполнения", label))
	}

	if data.Employees != nil {
		for _, e := range *data.Employees {
			for key := range required {
				if !employeeFieldPresent(e, key) {
					return fail(key)
				}
			}
		}
	}
	if data.Vehicles != nil {
		for _, v := range *data.Vehicles {
			for key := range required {
				if !vehicleFieldPresent(v, key) {
					return fail(key)
				}
			}
		}
	}
	if data.Items != nil {
		for _, item := range *data.Items {
			for key := range required {
				if !itemFieldPresent(item, key) {
					return fail(key)
				}
			}
		}
	}
	return nil
}

// pendingVehicleFlag - вставленная машина заявки, по которой нужно проверить близость к ЧС
// после коммита (id + номер; FindSimilar матчит по нормализованному номеру, марка не нужна).
type pendingVehicleFlag struct {
	carID     int
	carNumber string
}

// pendingEmployeeFlag - вставленный сотрудник заявки для пост-коммит проверки близости к ЧС.
type pendingEmployeeFlag struct {
	empID      int
	lastName   string
	firstName  string
	middleName string
}

// detectBlacklistSimilarity - мягкий слой предупреждения о возможном обходе ЧС (#481).
// Запускается ПОСЛЕ коммита сабмита и намеренно best-effort: это не блокирующая проверка
// (точное совпадение уже отклонено в validateBlacklist -> 409), а предупреждение. Любая
// ошибка поиска/записи флага логируется и проглатывается - неудача warning-слоя НЕ должна
// валить уже созданную заявку. Вне транзакции сабмита: ошибка здесь не отравит и не откатит её.
// supplementID - каким дополнением пришли проверяемые строки (#1685); nil у исходной подачи.
func (s *applicationService) detectBlacklistSimilarity(ctx context.Context, appID int, supplementID *int, vehicles []pendingVehicleFlag, employees []pendingEmployeeFlag) {
	for _, v := range vehicles {
		matches, err := s.vehicleBlacklist.FindSimilar(ctx, v.carNumber)
		if err != nil {
			slog.Warn("blacklist similarity check failed (vehicle)", "err", err, "app_id", appID, "car_id", v.carID)
			continue
		}
		s.saveBlacklistFlag(ctx, appID, supplementID, models.BlacklistElementCar, v.carID, normalize.Plate(v.carNumber), matches)
	}
	for _, e := range employees {
		matches, err := s.personBlacklist.FindSimilar(ctx, e.lastName, e.firstName, e.middleName)
		if err != nil {
			slog.Warn("blacklist similarity check failed (person)", "err", err, "app_id", appID, "employee_id", e.empID)
			continue
		}
		s.saveBlacklistFlag(ctx, appID, supplementID, models.BlacklistElementEmployee, e.empID, normalize.Name(e.lastName, e.firstName, e.middleName), matches)
	}
}

// saveBlacklistFlag сохраняет ЛУЧШЕЕ совпадение как флаг элемента: matches приходят
// отсортированными по убыванию близости (контракт FindSimilar). Пустой срез - элемент
// чист, флаг не пишем. Ошибку записи логируем и проглатываем (best-effort warning-слой).
func (s *applicationService) saveBlacklistFlag(ctx context.Context, appID int, supplementID *int, elementType string, elementID int, elementNormalized string, matches []models.BlacklistSimilarMatch) {
	if len(matches) == 0 {
		return
	}
	best := matches[0]
	// Если оператор уже подтвердил пропуск этого элемента против этой записи ЧС - не
	// предупреждаем повторно (#481, срез C-followup). Отмена override снова включит флаг.
	if s.isBlacklistSuppressed(ctx, elementType, elementNormalized, best.ID) {
		return
	}
	flag := models.ApplicationBlacklistFlag{
		ApplicationID:      appID,
		SupplementID:       supplementID,
		ElementType:        elementType,
		ElementID:          elementID,
		ElementNormalized:  elementNormalized,
		MatchedBlacklistID: best.ID,
		MatchedValue:       best.MatchedValue,
		MatchedReason:      best.Reason,
		Similarity:         best.Similarity,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&flag).Error; err != nil {
		slog.Warn("blacklist flag save failed", "err", err, "app_id", appID, "element_type", elementType, "element_id", elementID)
	}
}

// isBlacklistSuppressed - оператор ранее нажал "всё равно пропустить" по этому элементу
// против этой записи ЧС, и подтверждение ещё действует (override-строка жива) -> повторно
// не предупреждаем (#481, срез C-followup). Ключ - нормализованная форма элемента + id записи
// ЧС, поэтому переживает пересоздание cars/employees между заявками. Best-effort: при пустом
// ключе или ошибке считаем "не подавлено" (лучше лишний раз предупредить, чем тихо пропустить).
func (s *applicationService) isBlacklistSuppressed(ctx context.Context, elementType, elementNormalized string, matchedBlacklistID int) bool {
	if elementNormalized == "" {
		return false
	}
	var cnt int64
	err := s.db.WithContext(ctx).Model(&models.ApplicationBlacklistOverride{}).
		Where("element_type = ? AND element_normalized = ? AND matched_blacklist_id = ?",
			elementType, elementNormalized, matchedBlacklistID).
		Count(&cnt).Error
	if err != nil {
		slog.Warn("blacklist suppression check failed", "err", err, "element_type", elementType)
		return false
	}
	return cnt > 0
}

// Пакетные потолки подачи (blank-import, срез A2A3): вставка тысяч строк по одной душит
// БД тысячами round-trip, а один INSERT на весь список рискует упереться в лимит числа
// параметров запроса на большом вложении. Числа - компромисс между этими крайностями,
// не бизнес-правило: изменение значения не меняет то, что записывается, только сколько
// запросов на это уходит.
const (
	employeeInsertBatchSize = 500
	carInsertBatchSize      = 500
	bindingInsertBatchSize  = 1000
	auditInsertBatchSize    = 1000
)

// insertCarsBatch вставляет машины одного вложения multi-values INSERT пачками по
// carInsertBatchSize вместо построчного tx.Raw на каждую (blank-import, срез A2A3).
// Raw SQL, не GORM: у Car нет шифрующих хуков (в отличие от Employee), пакетная вставка
// машин раньше и так шла raw построчно - меняется только число round-trip, не механизм.
// RETURNING id для multi-row INSERT возвращает строки в порядке VALUES (гарантия
// Postgres для одного оператора), поэтому i-й id в результате соответствует i-й машине
// входного среза - на этом порядке строится сопоставление с pending-флагами/аудитом/
// привязками ниже.
func insertCarsBatch(tx *gorm.DB, attID int, vehicles []VehicleInput, entryDateFrom, entryTimeFrom, entryDateTo, entryTimeTo *string, actorUserID int, submittedAt time.Time) ([]int, error) {
	carIDs := make([]int, 0, len(vehicles))
	for start := 0; start < len(vehicles); start += carInsertBatchSize {
		end := start + carInsertBatchSize
		if end > len(vehicles) {
			end = len(vehicles)
		}
		chunk := vehicles[start:end]
		placeholders := make([]string, 0, len(chunk))
		args := make([]interface{}, 0, len(chunk)*10)
		for _, v := range chunk {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?::date, ?::time, ?::date, ?::time, 0, ?, ?)")
			args = append(args, attID, v.CarNumber, v.CarBrand, v.UnloadPlace, entryDateFrom, entryTimeFrom, entryDateTo, entryTimeTo,
				consentAt(v.PDConsent, submittedAt), consentBy(v.PDConsent, actorUserID))
		}
		query := "INSERT INTO cars (attachment_id, car_number, car_brand, unload_place, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status, pd_consent_at, pd_consent_by_user_id) VALUES " +
			strings.Join(placeholders, ", ") + " RETURNING id"
		var chunkIDs []int
		if err := tx.Raw(query, args...).Scan(&chunkIDs).Error; err != nil {
			return nil, err
		}
		if len(chunkIDs) != len(chunk) {
			return nil, fmt.Errorf("car insert вернул %d id на %d строк", len(chunkIDs), len(chunk))
		}
		carIDs = append(carIDs, chunkIDs...)
	}
	return carIDs, nil
}

// SubmitCompleteApplication создаёт полную заявку с вложениями, машинами и сотрудниками.
func (s *applicationService) SubmitCompleteApplication(ctx context.Context, username string, req CompleteApplicationRequest, canOverrideOrganization bool) (*CompleteApplicationResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if !req.DataApproval {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Data approval is required")
	}
	if len(req.Attachments) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "At least one attachment is required")
	}
	// Организация и компания взаимозаменяемы: для отправки достаточно одной из двух -
	// выбранной по id либо введённой наименованием (#1437).
	if req.OrganizationID == nil && strings.TrimSpace(req.OrganizationTitle()) == "" &&
		req.CompanyID == nil && strings.TrimSpace(req.CompanyTitle()) == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите организацию или компанию")
	}

	// Серверная валидация настроенных обязательных полей (#529 H-9): поля, помеченные
	// админом required, должны присутствовать; скрытые/ненастроенные пропускаются.
	if err := s.validateConfiguredRequiredFields(ctx, req); err != nil {
		return nil, err
	}

	// Серверный гард ЧС (#443): отклоняем заявку с заблокированной машиной/человеком
	// до старта транзакции - на случай обхода фронтовой проверки.
	if err := s.validateBlacklist(ctx, req); err != nil {
		return nil, err
	}

	// Один человек или одна машина не могут попасть во вложение дважды.
	if err := validateByFactVehicles(req, time.Now()); err != nil {
		return nil, err
	}

	if err := validateNoDuplicates(req); err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	baseTime := time.Now().UTC()
	applicationNumber := nextApplicationNumber(tx)

	// Организация и компания заявки: выбранная по id либо найденная по ключу
	// дедупликации наименования, а незнакомое наименование заводит запись «на
	// проверке» (#1437). NULL остаётся только там, где сущность не указана вовсе.
	// Чужую организацию пропускает только право override, свою из профиля - всегда.
	scope := applicantScope{
		userID:         user.ID,
		organizationID: user.OrganizationID,
		companyID:      user.CompanyID,
		canOverride:    canOverrideOrganization,
	}
	orgRef, err := s.resolveOrganizationRef(ctx, tx, scope, req.OrganizationID, req.OrganizationTitle())
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	companyRefResult, err := s.resolveCompanyRef(ctx, tx, scope, req.CompanyID, req.CompanyTitle())
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	organizationID, companyID := orgRef.ID, companyRefResult.ID
	// Backstop к гейту выше: при текущем резолве оба nil одновременно недостижимы, но
	// заявка без организации И компании не должна создаваться ни при каком его изменении -
	// именно такая сирота и была багом, который срез закрывает.
	if organizationID == nil && companyID == nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите организацию или компанию")
	}

	if err := s.ensureNoActiveByFactApplication(tx, organizationID, hasByFactVehicle(req)); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Создаём заявку
	var appID int
	err = tx.Raw(`
		INSERT INTO applications (application_number, organization_id, company_id, sender_user_id, message, data_approval, status, confirmation, sending_datetime, initiator_name, contact_phone)
		VALUES (?, ?, ?, ?, ?, ?, 'Непрочитано', 'Согласование', ?, NULLIF(?, ''), NULLIF(?, ''))
		RETURNING id
	`, applicationNumber, organizationID, companyID, user.ID, req.Message, fmt.Sprintf("%v", req.DataApproval), baseTime,
		strings.TrimSpace(req.ResponsiblePerson), strings.TrimSpace(req.ContactPhone)).Scan(&appID).Error
	if err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания заявки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating application")
	}

	// Файлы, приложенные при подаче (#1721). Внутри транзакции: не найденный
	// среди своих черновиков файл откатывает подачу, вместо того чтобы создать
	// заявку без документа, который заявитель считает приложенным.
	if s.files != nil {
		if err := s.files.Attach(tx, user.ID, appID, req.FileIDs, s.fileMaxCount, s.fileMaxTotal); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Записываем создание в историю
	metaCreate, _ := json.Marshal(map[string]string{"confirmation": "Согласование", "status": "Непрочитано"})
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &appID, "create", &user.ID,
		applicationAuditDetails{NewValue: &applicationNumber, Metadata: metaCreate})

	// Собираем ответственных
	type respUser struct {
		UserID           int
		IsPrimary        bool
		RequiredApproval bool
	}
	var responsibleUsers []respUser
	var primaryResponsibleID *int

	if organizationID != nil {
		var orgResp []struct {
			UserID           int  `gorm:"column:user_id"`
			IsPrimary        bool `gorm:"column:is_primary"`
			RequiredApproval bool `gorm:"column:required_approval"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary, required_approval FROM organization_users WHERE organization_id = ?", *organizationID).Scan(&orgResp)
		for _, r := range orgResp {
			responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary, r.RequiredApproval})
			if r.IsPrimary {
				primaryResponsibleID = &r.UserID
			}
		}
	}

	if companyID != nil {
		var compResp []struct {
			UserID           int  `gorm:"column:user_id"`
			IsPrimary        bool `gorm:"column:is_primary"`
			RequiredApproval bool `gorm:"column:required_approval"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary, required_approval FROM companies_users WHERE company_id = ?", *companyID).Scan(&compResp)
		for _, r := range compResp {
			exists := false
			for _, ru := range responsibleUsers {
				if ru.UserID == r.UserID {
					exists = true
					break
				}
			}
			if !exists {
				responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary, r.RequiredApproval})
				if r.IsPrimary && primaryResponsibleID == nil {
					primaryResponsibleID = &r.UserID
				}
			}
		}
	}

	// req.RequiredUsers - список из формы подачи, дублирующий required_approval,
	// который уже прочитан выше из organization_users/companies_users. Присланное
	// значение признак не меняет ни в одну сторону (#2037): заявитель не назначает
	// согласующих, он только видит состав организации, поэтому обязательность
	// целиком определяется справочником.
	//
	// Раньше пользователь, не найденный среди responsibleUsers, тихо ДОБАВЛЯЛСЯ в
	// согласующие (#2048): подделанный запрос с чужим user_id заводил в ответственные
	// постороннего из другой организации, тот видел заявку целиком и мог её согласовать.
	// Форма всегда шлёт id из состава уже прочитанной организации/компании (см. функцию
	// отправки в CreateApplication.vue - список берётся из ответа /organizations/{id}/users
	// и /companies/{id}/users), поэтому для легитимной подачи exists здесь истинно всегда,
	// а расхождение - признак подделанного запроса. Отклоняем подачу целиком, а не тихо
	// выбрасываем чужого пользователя: заявитель должен получить внятный отказ, а не
	// заявку, тихо созданную без части того, что он просил.
	if req.RequiredUsers != nil {
		for _, reqUser := range *req.RequiredUsers {
			exists := false
			for _, ru := range responsibleUsers {
				if ru.UserID == reqUser.UserID {
					exists = true
					break
				}
			}
			if !exists {
				tx.Rollback()
				return nil, echo.NewHTTPError(http.StatusBadRequest, "Назначить согласующим можно только ответственного этой организации или компании")
			}
		}
	}

	if primaryResponsibleID != nil {
		tx.Exec("UPDATE applications SET responsible_user_id = ? WHERE id = ?", *primaryResponsibleID, appID)
	}

	for _, ru := range responsibleUsers {
		tx.Exec(`
			INSERT INTO application_responsible_users (application_id, user_id, is_primary, required_approval, approval_status, created_at)
			VALUES (?, ?, ?, ?, 'pending', ?)
			ON CONFLICT (application_id, user_id) DO UPDATE SET is_primary = EXCLUDED.is_primary, required_approval = EXCLUDED.required_approval
		`, appID, ru.UserID, ru.IsPrimary, ru.RequiredApproval, baseTime)

		meta, _ := json.Marshal(map[string]interface{}{
			"required_approval": ru.RequiredApproval,
			"is_primary":        ru.IsPrimary,
		})
		ruUserID := ru.UserID
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &appID, "assigned_responsible", &ruUserID,
			applicationAuditDetails{Metadata: meta})
	}

	// Читатели-получатели заявки (#884): доступ только на просмотр через application_viewers
	// (как форвард-флоу) - CanAccessApplication пускает их на чтение, но не в согласующие.
	// Пропускаем тех, кто уже ответственный (у них доступ и так есть).
	if req.Readers != nil && len(*req.Readers) > 0 {
		// Читателем можно назначить только того, кого форма и предлагала выбрать:
		// иначе подделанный запрос открывал бы заявку любому пользователю системы.
		// Чужие идентификаторы отбрасываем молча - так же, как дубли ответственных
		// строкой ниже; запрос при этом остаётся валидным и заявка подаётся.
		//
		// Тот же список стережёт пересылку (ForwardApplication): второй путь к INSERT
		// в application_viewers обязан пускать тот же круг, иначе закрытая на подаче
		// дыра открывается через /forward.
		allowedReaders, err := recipientCandidateIDs(ctx, tx, *user)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		seenViewer := make(map[int]bool, len(responsibleUsers))
		for _, ru := range responsibleUsers {
			seenViewer[ru.UserID] = true
		}
		for _, readerID := range *req.Readers {
			if readerID <= 0 || seenViewer[readerID] {
				continue
			}
			if _, allowed := allowedReaders[readerID]; !allowed {
				slog.Warn("читатель заявки отброшен: вне списка доступных получателей",
					"application_id", appID, "reader_id", readerID, "author_id", user.ID)
				continue
			}
			seenViewer[readerID] = true
			tx.Exec(`
				INSERT INTO application_viewers (application_id, user_id, created_at, created_by)
				VALUES (?, ?, ?, ?)
			`, appID, readerID, baseTime, user.ID)
		}
	}

	// Вставленные машины/сотрудники для пост-коммит проверки близости к ЧС (#481).
	var pendingVehicleFlags []pendingVehicleFlag
	var pendingEmployeeFlags []pendingEmployeeFlag

	// Создаём вложения
	for _, att := range req.Attachments {
		var attID int
		err := tx.Raw(`
			INSERT INTO attachments (application_id, attachment_type, attachment_name, attachment_display_name, unique_attachment_id, entry_date_from, entry_date_to, entry_time_from, entry_time_to, roof_access, free_parking, status)
			VALUES (?, ?, ?, ?, ?, ?::date, ?::date, ?::time, ?::time, ?, ?, 1)
			RETURNING id
		`, appID, att.AttachmentType, att.AttachmentName, att.AttachmentDisplayName, att.UniqueAttachmentID,
			att.EntryDateFrom, att.EntryDateTo, att.EntryTimeFrom, att.EntryTimeTo, att.RoofAccess, att.FreeParking).Scan(&attID).Error
		if err != nil {
			tx.Rollback()
			slog.Error("Ошибка создания вложения", "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating attachment")
		}

		switch att.AttachmentType {
		case "cars":
			if att.Data.Vehicles != nil {
				vehicles := *att.Data.Vehicles
				// Машины остаются raw SQL (у Car нет шифрующих хуков), но одним пакетным
				// multi-values INSERT вместо построчного (blank-import, срез A2A3).
				carIDs, err := insertCarsBatch(tx, attID, vehicles, att.EntryDateFrom, att.EntryTimeFrom, att.EntryDateTo, att.EntryTimeTo, user.ID, baseTime)
				if err != nil {
					tx.Rollback()
					slog.Error("Ошибка создания машин (batch)", "attachment_id", attID, "error", err)
					return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating car")
				}

				// Дедуп-union мест всех машин вложения для attachment_unload_places (#706).
				// car_unload_places продолжаем писать для read-side и истории.
				carPlacesSet := make(map[int]struct{})
				auditEntries := make([]models.AuditLog, 0, len(vehicles))
				var unloadBindings []models.CarUnloadPlace
				var tableBindings []models.CarTargetTable
				for i, v := range vehicles {
					carID := carIDs[i]
					pendingVehicleFlags = append(pendingVehicleFlags, pendingVehicleFlag{carID: carID, carNumber: v.CarNumber})

					carCreateComment := fmt.Sprintf("Автомобиль %s %s создан", v.CarNumber, v.CarBrand)
					entry, err := buildAuditLogEntry(ctx, models.AuditEntityCar, &carID, "create", &user.ID, carAuditDetails{Comment: &carCreateComment})
					if err != nil {
						slog.Error("не удалось подготовить аудит создания машины (submit)", "car_id", carID, "error", err)
					} else {
						auditEntries = append(auditEntries, entry)
					}

					for _, placeID := range v.UnloadPlaces {
						pid, oneIdx := placeID, 1
						unloadBindings = append(unloadBindings, models.CarUnloadPlace{CarID: carID, UnloadPlaceID: pid, OrderIndex: &oneIdx})
						carPlacesSet[pid] = struct{}{}
					}

					// Таблицы «Проезд» (#1036): машина видна только в выбранных cars-таблицах
					// (зеркало employee_target_tables). Историю попадания в таблицу пишем НЕ здесь,
					// а при активации заявки (status->1) - при подаче машина ещё неактивна (#1085).
					for _, tableID := range v.TargetTables {
						tid, oneIdx := tableID, 1
						tableBindings = append(tableBindings, models.CarTargetTable{CarID: carID, TableID: tid, OrderIndex: &oneIdx, Source: "application"})
					}
				}
				if len(auditEntries) > 0 {
					if err := tx.CreateInBatches(&auditEntries, auditInsertBatchSize).Error; err != nil {
						slog.Error("не удалось записать аудит создания машин (submit)", "attachment_id", attID, "error", err)
					}
				}
				if len(unloadBindings) > 0 {
					if err := tx.CreateInBatches(&unloadBindings, bindingInsertBatchSize).Error; err != nil {
						slog.Error("не удалось привязать машины к местам разгрузки (submit)", "attachment_id", attID, "error", err)
					}
				}
				if len(tableBindings) > 0 {
					if err := tx.CreateInBatches(&tableBindings, bindingInsertBatchSize).Error; err != nil {
						slog.Error("не удалось привязать машины к таблицам (submit)", "attachment_id", attID, "error", err)
					}
				}
				// Пишем дедупированные места в attachment_unload_places (источник для охранника).
				for placeID := range carPlacesSet {
					tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID)
				}
			}

		case "people":
			if att.Data.Employees != nil {
				employeesInput := *att.Data.Employees
				employeeRecords := make([]models.Employee, 0, len(employeesInput))
				for _, e := range employeesInput {
					statusZero := 0
					lastName := e.LastName
					firstName := e.FirstName
					citizenshipID := e.CitizenshipID
					position := e.Position
					employeeRecords = append(employeeRecords, models.Employee{
						AttachmentID:         &attID,
						LastName:             &lastName,
						FirstName:            &firstName,
						MiddleName:           e.MiddleName,
						CitizenshipID:        &citizenshipID,
						Position:             &position,
						PassportSeriesNumber: nilIfBlank(e.PassportSeriesNumber),
						PatentNumber:         nilIfBlankPtr(e.PatentNumber),
						OtherPermission:      e.OtherPermission,
						Status:               &statusZero,
						PDConsentAt:          consentAt(e.PDConsent, baseTime),
						PDConsentByUserID:    consentBy(e.PDConsent, user.ID),
					})
				}
				// CreateInBatches, НЕ raw SQL: Employee.BeforeSave (models/employee.go)
				// шифрует паспорт/патент и пишет HMAC. Хуки срабатывают только через GORM
				// (Create/CreateInBatches), поэтому пакетная вставка обязана остаться на
				// нём - иначе персональные данные легли бы в базу открытым текстом
				// (blank-import, срез A2A3). GORM возвращает id каждой строки в тот же
				// элемент среза (RETURNING на Postgres), порядок с employeesInput совпадает.
				if err := tx.CreateInBatches(&employeeRecords, employeeInsertBatchSize).Error; err != nil {
					tx.Rollback()
					slog.Error("Ошибка создания сотрудников (batch)", "attachment_id", attID, "error", err)
					return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
				}

				auditEntries := make([]models.AuditLog, 0, len(employeeRecords))
				var tableBindings []models.EmployeeTargetTable
				for i, employee := range employeeRecords {
					empID := employee.ID
					e := employeesInput[i]

					empMiddle := ""
					if e.MiddleName != nil {
						empMiddle = *e.MiddleName
					}
					pendingEmployeeFlags = append(pendingEmployeeFlags, pendingEmployeeFlag{
						empID: empID, lastName: e.LastName, firstName: e.FirstName, middleName: empMiddle,
					})
					empComment := fmt.Sprintf("Сотрудник %s создан", strings.TrimSpace(strings.Join([]string{e.LastName, e.FirstName, empMiddle}, " ")))
					entry, err := buildAuditLogEntry(ctx, models.AuditEntityEmployee, &empID, "create", &user.ID, carAuditDetails{Comment: &empComment})
					if err != nil {
						slog.Error("не удалось подготовить аудит создания сотрудника (submit)", "employee_id", empID, "error", err)
					} else {
						auditEntries = append(auditEntries, entry)
					}

					// Историю попадания в таблицу пишем НЕ здесь, а при активации заявки
					// (status->1) - при подаче сотрудник ещё неактивен (#1085).
					for _, tableID := range e.TargetTables {
						tid, oneIdx := tableID, 1
						tableBindings = append(tableBindings, models.EmployeeTargetTable{EmployeeID: empID, TableID: tid, OrderIndex: &oneIdx, Source: "application"})
					}
				}

				if len(auditEntries) > 0 {
					if err := tx.CreateInBatches(&auditEntries, auditInsertBatchSize).Error; err != nil {
						slog.Error("не удалось записать аудит создания сотрудников (submit)", "attachment_id", attID, "error", err)
					}
				}
				if len(tableBindings) > 0 {
					if err := tx.CreateInBatches(&tableBindings, bindingInsertBatchSize).Error; err != nil {
						slog.Error("не удалось привязать сотрудников к таблицам (submit)", "attachment_id", attID, "error", err)
					}
				}

				// Сводная запись в ленте самой заявки (blank-import, срез A2A3): N карточек
				// «Сотрудник ФИО создан» продолжают писаться выше для истории каждого
				// сотрудника (её читает /employees/:id/history) - эта запись их не
				// заменяет, а добавляет ОДНУ строку в историю заявки, чтобы не пролистывать
				// всех сотрудников, чтобы понять сколько их добавлено разом.
				if len(employeeRecords) > 0 {
					summary := fmt.Sprintf("Добавлено сотрудников: %d", len(employeeRecords))
					s.recorder.Log(ctx, tx, models.AuditEntityApplication, &appID, models.AuditActionEmployeesBulkAdded, &user.ID,
						applicationAuditDetails{Comment: &summary})
				}
			}

		case "items":
			if att.Data.Items != nil {
				for _, item := range *att.Data.Items {
					tx.Exec(`
						INSERT INTO items (attachment_id, name, count, date_created)
						VALUES (?, ?, ?, ?)
					`, attID, item.Name, item.Count, baseTime.Format("2006-01-02"))
				}
			}
			// Места разгрузки для items приходят на уровне вложения (#706).
			for _, placeID := range att.UnloadPlaces {
				tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID)
			}

		default:
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment type")
		}

		if att.CustomValues != nil {
			for _, cv := range *att.CustomValues {
				if cv.Value == "" {
					continue
				}
				if err := tx.Create(&models.AttachmentCustomValue{
					AttachmentID:  attID,
					CustomFieldID: cv.CustomFieldID,
					Value:         cv.Value,
				}).Error; err != nil {
					tx.Rollback()
					return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error saving custom field value")
				}
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Файловый архив (#1615, B1): подача - первая точка, где у заявки вообще
	// появляются бланки на выгрузку.
	s.enqueueArchiveExport(appID, BlankExportReasonSubmit)

	// Мягкая проверка возможного обхода ЧС по похожему номеру/ФИО (#481): помечает
	// элементы флагом для предупреждения согласующим. Best-effort, заявку не блокирует.
	// Синхронно (не в горутине) намеренно: флаги должны быть готовы сразу для детали
	// заявки и детерминированны для тестов; сабмит не hot-path, элементов в заявке мало.
	s.detectBlacklistSimilarity(ctx, appID, nil, pendingVehicleFlags, pendingEmployeeFlags)

	// Данные для подробностей уведомления: без них окно показывает один текст, а
	// кнопка перехода к заявке не появляется вовсе - именно на этих двух типах
	// согласующий и заявитель упирались в тупик (#1748).
	// Отправитель кладётся в данные, а не только в текст: окно подробностей строит поля
	// из них, и без организации принимающий видит один номер заявки.
	senderTitle := s.applicationSenderTitle(ctx, organizationID, companyID)
	senderPerson := formatFullName(user.LastName, user.FirstName, user.MiddleName)
	if strings.TrimSpace(senderPerson) == "" {
		senderPerson = user.Username
	}
	submitPayloadFields := map[string]any{
		"application_id":     appID,
		"application_number": applicationNumber,
	}
	if senderTitle != "" {
		submitPayloadFields["organization"] = senderTitle
	}
	if senderPerson != "" {
		submitPayloadFields["sender_name"] = senderPerson
	}
	submitPayloadBytes, _ := json.Marshal(submitPayloadFields)
	submitPayload := string(submitPayloadBytes)

	// Уведомление отправителю о создании заявки
	if s.notificationService != nil {
		if err := s.notificationService.CreateForUser(
			ctx, user.ID,
			NotificationTypeApplicationCreated,
			"Заявка отправлена",
			fmt.Sprintf("Ваша заявка %s отправлена и ожидает согласования.", applicationNumber),
			&submitPayload,
		); err != nil {
			slog.Warn("notification create failed", "err", err, "user_id", user.ID, "app_id", appID)
		}
	}

	// Уведомления ответственным (approvers) о новой заявке
	if s.notificationService != nil {
		for _, ru := range responsibleUsers {
			if !ru.RequiredApproval || ru.UserID == user.ID {
				continue
			}
			if err := s.notificationService.CreateForUser(
				ctx, ru.UserID,
				NotificationTypeApplicationApprovalRequired,
				"Требуется согласование",
				fmt.Sprintf("Поступила новая заявка %s на согласование.", applicationNumber),
				&submitPayload,
			); err != nil {
				slog.Warn("notification create failed", "err", err, "user_id", ru.UserID, "app_id", appID)
			}
		}
	}

	// Принимающим - о заявке, которая легла в Центр и ждёт, что её возьмут в работу.
	// Согласующий и принимающий - разные роли: первый голосует и получает уведомление
	// выше, второй берёт заявку в работу и до этого о подаче не узнавал вообще ничего,
	// хотя именно он ждёт её в Центре.
	if s.notificationService != nil {
		s.notifyApproversAboutNewApplication(ctx, user.ID, appID, pendingAcceptanceNote{
			number:       applicationNumber,
			organization: senderTitle,
			sender:       senderPerson,
			messageText:  optionalString(req.Message),
			fileNames:    s.applicationFileNames(ctx, req.FileIDs),
		}, submitPayload)
	}

	// Подача завела наименование, которого не было в справочнике (#1437): зовём тех,
	// кто может его разобрать. Записи, легшие на существующую организацию или компанию,
	// сюда не попадают - разбирать там нечего.
	var pending []pendingDirectoryNotice
	if orgRef.PendingName != "" {
		pending = append(pending, pendingDirectoryNotice{label: organizationModeration.label, name: orgRef.PendingName})
	}
	if companyRefResult.PendingName != "" {
		pending = append(pending, pendingDirectoryNotice{label: companyModeration.label, name: companyRefResult.PendingName})
	}
	s.notifyDirectoryPending(ctx, appID, applicationNumber, user.ID, pending)

	// Real-time сигнал обновления Центра заявок (#840): тем, у кого новая заявка
	// появляется в списке (автор, ответственные/согласующие, принимающие). Лёгкий
	// сигнал event-then-fetch, best-effort - на успех создания заявки не влияет.
	if s.realtimePublisher != nil {
		audience := s.centerAudience(ctx, appID, user.ID)
		s.realtimePublisher.PublishMany(audience, realtime.Event{Type: "applications.refresh", Scope: "applications-center"})
	}

	return &CompleteApplicationResponse{
		Success:           true,
		Message:           "Application created successfully",
		ApplicationID:     appID,
		ApplicationNumber: applicationNumber,
	}, nil
}

// UpdateApplication обновляет данные заявки (confirmation, status, комментарий).
func (s *applicationService) UpdateApplication(ctx context.Context, username string, applicationID int, req ApplicationUpdateRequest) (*ApplicationUpdateResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	setClauses := []string{}
	args := []interface{}{}

	if req.Confirmation != nil {
		if !allowedConfirmations[*req.Confirmation] {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid confirmation value")
		}
		// Гейт обхода ЧС (#481): прямое выставление "Согласовано" этим путём (минуя
		// поэлементное голосование) тоже блокируем, пока есть помеченные элементы без
		// override - иначе блокировку согласования из ApproveApplicationByUser легко обойти.
		if *req.Confirmation == models.ConfirmationApproved {
			blocked, err := hasUnoverriddenBlacklistFlags(ctx, s.db, applicationID)
			if err != nil {
				return nil, err
			}
			if blocked {
				return nil, echo.NewHTTPError(http.StatusConflict,
					"Заявка содержит элементы, похожие на чёрный список. Подтвердите пропуск каждого ('Всё равно пропустить') перед согласованием")
			}
		}
		setClauses = append(setClauses, "confirmation = ?")
		args = append(args, *req.Confirmation)
		if *req.Confirmation == "Согласовано" || *req.Confirmation == "Не согласовано" {
			setClauses = append(setClauses, "confirmation_datetime = ?")
			args = append(args, now)
		}
	}

	if req.Status != nil {
		if !allowedStatuses[*req.Status] {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid status value")
		}
		setClauses = append(setClauses, "status = ?")
		args = append(args, *req.Status)
		if *req.Status == "В обработке" {
			setClauses = append(setClauses, "reading_datetime = ?")
			args = append(args, now)
		}
	}

	if req.ResponsibleComment != nil {
		setClauses = append(setClauses, "responsible_comment = ?")
		args = append(args, *req.ResponsibleComment)
		setClauses = append(setClauses, "responsible_user_id = ?")
		args = append(args, user.ID)
	}

	if len(setClauses) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "No data to update")
	}

	sqlQuery := fmt.Sprintf("UPDATE applications SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, applicationID)

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Старые значения под блокировкой строки: флаг "статус обновился" бампаем только при
	// реальной смене status/confirmation, а не на каждый PUT (#1349).
	var old struct {
		Status       *string
		Confirmation *string
	}
	oldRes := tx.Raw("SELECT status, confirmation FROM applications WHERE id = ? FOR UPDATE", applicationID).Scan(&old)
	if oldRes.Error != nil {
		tx.Rollback()
		slog.Error("Ошибка чтения заявки перед обновлением", "application_id", applicationID, "error", oldRes.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating application")
	}

	result := tx.Exec(sqlQuery, args...)
	if result.Error != nil {
		tx.Rollback()
		slog.Error("Ошибка обновления заявки", "application_id", applicationID, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating application")
	}

	statusChanged := req.Status != nil && (old.Status == nil || *old.Status != *req.Status)
	confirmationChanged := req.Confirmation != nil && (old.Confirmation == nil || *old.Confirmation != *req.Confirmation)
	if oldRes.RowsAffected > 0 && (statusChanged || confirmationChanged) {
		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Любое изменение заявки этим путём (статус/подтверждение/коммент) участники
	// видят в детали live (#840 V4).
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	// Инициатору - уведомление об исходе согласования, если admin выставил confirmation
	// в финальное значение (Согласовано/Не согласовано) и оно реально сменилось (#1349).
	if confirmationChanged {
		if outcome := confirmationOutcome(req.Confirmation); outcome != "" {
			s.notifyInitiatorStatusChanged(ctx, applicationID, &user.ID, outcome, &statusChangeContext{
				ActorName: formatFullName(user.LastName, user.FirstName, user.MiddleName),
				Comment:   optionalString(req.ResponsibleComment),
			})
		}
	}
	// Прямое выставление "Согласовано" (admin-путь, минуя approve-флоу) делает
	// вложения доступными охране - сигналим обновить "Доступные мне" (#840 V3). Сигнал
	// безданных, лишний при не-переходе безвреден (event-then-fetch, клиент рефетчит).
	if req.Confirmation != nil && *req.Confirmation == models.ConfirmationApproved {
		s.availableProducer.NotifyAvailableChanged(ctx)
	}

	return &ApplicationUpdateResponse{
		Success:      true,
		Message:      "Application updated successfully",
		RowsAffected: result.RowsAffected,
	}, nil
}
