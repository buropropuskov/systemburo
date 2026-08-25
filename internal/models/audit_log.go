package models

import (
	"encoding/json"
	"time"
)

// AuditLog - единый журнал аудита (#870): сводит ~21 отдельную *_history таблицу
// в одно место. Одна строка = одно действие над сущностью. Снаружи у каждой
// сущности своя история через фильтр (entity_type, entity_id) - не общая "свалка".
//
// EntityID/ActorUserID намеренно без FK constraint: аудит должен пережить удаление
// родителя или пользователя (как в legacy *History-моделях).
// Details (jsonb) - надмножество всех старых схем: новый паттерн пишет details как
// есть; плоский field_name/old/new/comment/metadata и snapshot-поля маппятся внутрь.
//
// Два составных индекса под разные формы чтения:
//   - idx_audit_entity (entity_type, entity_id, created_at) - история КОНКРЕТНОЙ
//     сущности («что делали с этой заявкой»);
//   - idx_audit_entity_action (entity_type, action, created_at) - выборки по ДЕЙСТВИЮ
//     за период без entity_id: лента журнала обработки (#1251, reject/withdraw по
//     заявкам) и метрики въездов/входов (statistics_service, action='entry' за окно).
//     Для них у idx_audit_entity рабочей остаётся только первая колонка, created_at
//     через пропущенный entity_id не доходит - на 500k строк планировщик уходил в
//     seq scan (38 мс против 9 мс по новому индексу).
type AuditLog struct {
	ID          int             `json:"id"`
	EntityType  string          `gorm:"size:64;not null;index:idx_audit_entity,priority:1;index:idx_audit_entity_action,priority:1" json:"entity_type"`
	EntityID    *int            `gorm:"index:idx_audit_entity,priority:2" json:"entity_id,omitempty"`
	Action      string          `gorm:"size:64;index;index:idx_audit_entity_action,priority:2" json:"action"`
	ActorUserID *int            `gorm:"index" json:"actor_user_id,omitempty"`
	Details     json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt   time.Time       `gorm:"index:idx_audit_entity,priority:3;index:idx_audit_entity_action,priority:3" json:"created_at"`
}

// TableName задаёт имя таблицы явно (singular per #870), без gorm-плюрализации.
func (AuditLog) TableName() string { return "audit_log" }

// AuditEntity* - значения AuditLog.EntityType. Добавляются по мере переноса
// сущностей на audit_log (#870).
const (
	AuditEntityCitizenship        = "citizenship"
	AuditEntityCompany            = "company"
	AuditEntityOrganization       = "organization"
	AuditEntityUserType           = "user_type"
	AuditEntityLicensePlateFormat = "license_plate_format"
	AuditEntityUnloadPlace        = "unload_place"
	AuditEntityUniqueAttachment   = "unique_attachment"
	AuditEntityUser               = "user"
	AuditEntityApprover           = "approver"
	AuditEntityPersonBlacklist    = "person_blacklist"
	AuditEntityVehicleBlacklist   = "vehicle_blacklist"
	AuditEntitySystemTable        = "system_table"
	AuditEntitySystemTableTrash   = "system_table_trash"
	AuditEntityMark               = "mark"
	AuditEntityCar                = "car"
	AuditEntityUniqueCar          = "unique_car"
	AuditEntityEmployee           = "employee"
	AuditEntityUniqueEmployee     = "unique_employee"
	AuditEntityApplication        = "application"
	// AuditEntityArchiveSettings - настройки файлового архива бланков (#1615).
	// EntityID у записей пустой: настройки одни на систему, а не строка справочника.
	AuditEntityArchiveSettings = "archive_settings"
	// AuditEntityArchiveQuota - место и квота файлового архива (#1615, срез B2):
	// переход очереди выгрузки в blocked при пересечении жёсткого порога и снятие
	// блокировки. EntityID пустой - событие относится к очереди в целом, а не к
	// одной заявке.
	AuditEntityArchiveQuota = "archive_quota"
	// AuditEntityRequestLogExport - снятие журнала обращений файлом (#2125).
	// EntityID пустой: выгружается выборка, а не строка справочника. Одна выгрузка
	// уносит адреса обращений сотен пользователей разом, поэтому оставляет след
	// наравне с выгрузкой реестра заявок.
	AuditEntityRequestLogExport = "request_log_export"
)

// RequestLogExportActionExported - журнал обращений выгружен файлом.
const RequestLogExportActionExported = "exported"

// ArchiveSettingsActionUpdated - изменение настроек файлового архива.
const ArchiveSettingsActionUpdated = "updated"

// ArchiveQuotaAction* - действия над очередью выгрузки при пересечении жёсткого
// порога места (#1615, срез B2).
const (
	// ArchiveQuotaActionBlocked - недостатка места хватило, чтобы остановить
	// часть очереди: строки реестра ушли в blocked до появления свободного места.
	ArchiveQuotaActionBlocked = "blocked"
	// ArchiveQuotaActionUnblocked - порог перестал нарушаться, блокировка снята.
	ArchiveQuotaActionUnblocked = "unblocked"
)

// AllAuditEntities - перечень известных типов сущностей. Нужен там, где тип приходит
// снаружи и опечатку в нём надо поймать, а не молча получить пустую выборку: так
// работает фильтр очистки журнала по типу сущности (#1632).
var AllAuditEntities = []string{
	AuditEntityCitizenship, AuditEntityCompany, AuditEntityOrganization,
	AuditEntityUserType, AuditEntityLicensePlateFormat, AuditEntityUnloadPlace,
	AuditEntityUniqueAttachment, AuditEntityUser, AuditEntityApprover,
	AuditEntityPersonBlacklist, AuditEntityVehicleBlacklist, AuditEntitySystemTable,
	AuditEntitySystemTableTrash, AuditEntityMark, AuditEntityCar, AuditEntityUniqueCar,
	AuditEntityEmployee, AuditEntityUniqueEmployee, AuditEntityApplication,
	AuditEntityArchiveSettings, AuditEntityArchiveQuota, AuditEntityRequestLogExport,
}

// AuditAction* - значения AuditLog.Action, вынесенные в константы там, где значение
// используется в нескольких местах записи/чтения (иначе дрейф литерала). Большинство
// действий car/employee остаются строковыми литералами в своих сервисах.
const (
	// AuditActionAddedToTable - машина/сотрудник внесены в таблицу проходной
	// (car_target_tables/employee_target_tables, #1036). Пишется по одной записи на
	// таблицу, details.table_id хранит пост - reader резолвит table_name как у entry/exit.
	AuditActionAddedToTable = "added_to_table"
	// AuditActionUnboundFromTable - машина/сотрудник сняты с таблицы проходной групповой
	// операцией «Убрать» (#1194), но у сущности осталась хотя бы одна другая привязка
	// (иначе см. deactivate/#951). details.table_id - снятая таблица.
	AuditActionUnboundFromTable = "unbound_from_table"
	// AuditActionMovedBetweenTables - машина/сотрудник перенесены групповой операцией
	// «Перенести» (#1194) из одной таблицы проходной в другую(ие) одним действием.
	// details.table_id - таблица-источник (там же появляется событие для фильтра
	// «Место прохода»); таблицы назначения - только в человекочитаемом comment
	// (их может быть несколько, details.table_id - одно поле).
	AuditActionMovedBetweenTables = "moved_between_tables"
	// AuditActionForwarded - сводная запись о пересылке заявки: ОДНА на действие
	// (не на получателя, тех пишут assigned_responsible/assigned_viewer). Читают
	// ветка пересылок в истории заявки (#680) и метрика avg_forwards (#1240).
	AuditActionForwarded = "forwarded"
	// AuditActionTakeToWork - заявка принята в работу (TakeApplicationToWork,
	// action=accept). Пишется на КАЖДОЕ принятие (заявку могли отозвать и принять
	// снова), поэтому принимающего берут как актора ПЕРВОГО такого действия. Читают
	// метрика принимающих (#1251, report_acceptor_metrics.go) и backfill accepted_at.
	AuditActionTakeToWork = "take_to_work"
	// AuditActionReject - ВНИМАНИЕ, значение делят два РАЗНЫХ действия: отказ
	// принимающего (TakeApplicationToWork, action=reject -> статус «Отказано») и
	// несогласование согласующего (ApproveApplication со status=rejected). Различить
	// их можно только по details: у отказа принимающего заполнен new_value статусом
	// «Отказано», у голоса согласующего его нет. Читатели, которым нужен именно отказ
	// принимающего (журнал обработки), обязаны фильтровать по new_value.
	AuditActionReject = "reject"
	// AuditActionWithdraw - заявка отозвана инициатором (WithdrawApplication).
	// Действие терминальное и доступно только отправителю заявки.
	AuditActionWithdraw = "withdraw"
	// AuditActionSupplementCancelled - открытое дополнение заявки снято системой (#1685):
	// заявка закрылась раньше, чем дополнение прошло свой круг. Пишется на заявку
	// (entity_type=application), а не на дополнение - в истории заявки его и ищут;
	// details.comment называет номер снятого раунда.
	AuditActionSupplementCancelled = "supplement_cancelled"
	// AuditActionSupplementApprove / AuditActionSupplementReject - голос согласующего по
	// раунду дополнения (#1685). Отдельные значения от голосов основного круга (approve /
	// reject) намеренно: расклад голосов у кругов разный, а лента истории у заявки одна -
	// без разделения «Согласовал(-а) заявку» встало бы рядом с согласованием добавки и
	// читалось бы как повторное согласование самой заявки.
	AuditActionSupplementApprove = "supplement_approve"
	AuditActionSupplementReject  = "supplement_reject"
	// AuditActionSupplementRevokeApproval - согласующий отозвал свой голос по раунду.
	AuditActionSupplementRevokeApproval = "supplement_revoke_approval"
	// AuditActionSupplementConfirmationChange - сменился ИТОГ раунда дополнения
	// (application_supplements.status). Пишется на заявку, как и остальные события раунда;
	// old_value/new_value - статусы раунда, не заявки: confirmation самой заявки дополнение
	// не двигает ни при каком раскладе голосов.
	AuditActionSupplementConfirmationChange = "supplement_confirmation_change"
	// AuditActionSupplementAccepted - принимающий принял согласованный раунд (#1685): его
	// строки активированы и с этого момента видны на КПП. Отдельно от take_to_work: та
	// запись означает принятие ЗАЯВКИ и меняет её статус, эта не трогает заявку вовсе.
	AuditActionSupplementAccepted = "supplement_accepted"
	// AuditActionSupplementRefused - принимающий отказал согласованному раунду. Строки
	// остаются неактивными навсегда; сама заявка и её допущенный состав не задеты.
	AuditActionSupplementRefused = "supplement_refused"
	// AuditActionSupplementCancelledByAuthor - автор снял собственный незакрытый раунд.
	// Отдельно от AuditActionSupplementCancelled: там раунд снимает система при закрытии
	// заявки (актор пустой, «Система»), здесь - человек своей волей.
	AuditActionSupplementCancelledByAuthor = "supplement_cancelled_by_author"
	// AuditActionImpersonateStart / AuditActionImpersonateStop - вход администратора в
	// режим «войти как пользователь» и возврат в свою учётную запись (#1912). Пишутся на
	// того, от чьего имени открыт сеанс (entity_type=user, entity_id - его id), актор -
	// инициатор. Пара записей задаёт окно, внутри которого действия учётной записи
	// принадлежат не её владельцу; сами действия внутри окна помечены полем
	// details.impersonated_by (его дописывает рекордер аудита).
	AuditActionImpersonateStart = "impersonate_start"
	AuditActionImpersonateStop  = "impersonate_stop"
	// AuditActionEmployeesBulkAdded - сводная запись «добавлено N сотрудников» на заявку
	// (entity_type=application), одна на вложение people при подаче (blank-import, срез
	// A2A3). Каждый сотрудник ПРОДОЛЖАЕТ получать свою собственную запись create
	// (entity_type=employee, entity_id=его id) - её читает история конкретного сотрудника
	// (/employees/:id/history). Эта запись не заменяет их, а даёт заявке одну строку в
	// её собственной ленте вместо необходимости открыть каждого сотрудника по отдельности.
	AuditActionEmployeesBulkAdded = "employees_bulk_added"
)

// AuditLogItem - запись аудита для API с разрезолвленным именем актора
// (LEFT JOIN users). Унифицирует поле актора: старые модели отдавали то actor_name,
// то user_name - generic-ответ всегда actor_name.
type AuditLogItem struct {
	ID          int             `json:"id"`
	EntityType  string          `json:"entity_type"`
	EntityID    *int            `json:"entity_id,omitempty"`
	Action      string          `json:"action"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
