package services

import "sort"

// Единый каталог типов уведомлений (#1748). Источник правды для будущего экрана
// настроек «какие уведомления присылать» и для проверки кодов, которые сервисы
// передают в CreateForUser. По аналогии с каталогом прав (permission_catalog.go)
// коды не сидируются в БД -- в notifications.type лежит просто строка, каталог
// живёт целиком в Go.
//
// Существующие коды (application_created, password_changed и т.д.) уже лежат в
// БД в отправленных ранее уведомлениях -- их менять нельзя, только добавлять новые.

// NotificationCategory группирует типы уведомлений для экрана настроек.
type NotificationCategory string

const (
	NotificationCategoryApplication NotificationCategory = "application"
	NotificationCategorySecurity    NotificationCategory = "security"
	NotificationCategoryPassage     NotificationCategory = "passage"
	NotificationCategoryContent     NotificationCategory = "content"
	NotificationCategorySystem      NotificationCategory = "system"
)

// Приоритет уведомления -- влияет на то, как настойчиво его показывать в будущем
// экране (звук/бейдж), сам факт доставки от приоритета не зависит.
const (
	NotificationPriorityNormal = "normal"
	NotificationPriorityHigh   = "high"
)

// NotificationMeta -- метаданные одного типа уведомления.
type NotificationMeta struct {
	Code     string
	Category NotificationCategory
	// Label -- короткое человеческое название для экрана настроек.
	Label string
	// Description -- одна фраза о том, что за событие порождает уведомление.
	Description string
	// Mandatory -- нельзя отключить (сейчас это все уведомления безопасности).
	Mandatory bool
	// DefaultEnabled -- состояние переключателя, пока пользователь его не менял.
	DefaultEnabled bool
	// Aggregatable -- допустимо схлопывать повторы в одну запись (см. GroupKey
	// в models.Notification).
	Aggregatable bool
	Priority     string
}

// Коды типов уведомлений раздела "application" -- события по заявке от подачи
// до завершения.
const (
	NotificationTypeApplicationCreated           = "application_created"
	NotificationTypeApplicationApprovalRequired  = "application_approval_required"
	NotificationTypeApplicationForwarded         = "application_forwarded"
	NotificationTypeApplicationStatusChanged     = "application_status_changed"
	NotificationTypeApplicationQuestion          = "application_question"
	NotificationTypeApplicationAnswer            = "application_answer"
	NotificationTypeApplicationSupplementReady   = "application_supplement_ready"
	NotificationTypeApplicationSupplementDecided = "application_supplement_decided"
	// NotificationTypeApprovalReminder сохраняет исходное имя (без "Application" в
	// середине) -- переносится из reminder_service.go, где на него уже ссылаются
	// тесты через services.NotificationTypeApprovalReminder.
	NotificationTypeApprovalReminder = "application_approval_reminder"
)

// Коды типов уведомлений раздела "security" -- ни один нельзя отключить.
const (
	NotificationTypePasswordChanged = "password_changed"
	NotificationTypeUserBanned      = "user_banned"
	NotificationTypeUserUnbanned    = "user_unbanned"
	NotificationTypeLoginBlocked    = "login_blocked"
	NotificationTypeRoleChanged     = "role_changed"
)

// Коды типов уведомлений раздела "passage" -- события вокруг прохода по заявке.
const (
	NotificationTypeApplicationExpiring         = "application_expiring"
	NotificationTypeApplicationWithdrawn        = "application_withdrawn"
	NotificationTypeApplicationAcceptorAssigned = "application_acceptor_assigned"
	NotificationTypeApplicationPassageFirst     = "application_passage_first"
)

// Коды типов уведомлений раздела "content" -- новости, документы, обратная связь.
const (
	NotificationTypeNewsPublished     = "news_published"
	NotificationTypeDocumentPublished = "document_published"
	NotificationTypeFeedbackCreated   = "feedback_created"
	NotificationTypeFeedbackAnswered  = "feedback_answered"
)

// Коды типов уведомлений раздела "system" -- обслуживание, архив, справочники.
const (
	NotificationTypeMaintenanceScheduled = "maintenance_scheduled"
	NotificationTypeTrashRestored        = "trash_restored"
	// NotificationTypeArchiveQuotaWarning перенесена из blank_export_quota.go --
	// на неё ссылаются тесты через services.NotificationTypeArchiveQuotaWarning.
	NotificationTypeArchiveQuotaWarning = "archive_quota_warning"
	// NotificationTypeDirectoryPending и NotificationTypeDirectoryResolved перенесены
	// из directory_pending_notify.go -- их имена не следуют схеме "код в PascalCase"
	// (код directory_entry_pending/directory_entry_resolved), но переименовывать
	// нельзя: на них ссылаются handlers-тесты через services.NotificationTypeDirectoryXxx.
	NotificationTypeDirectoryPending  = "directory_entry_pending"
	NotificationTypeDirectoryResolved = "directory_entry_resolved"
)

// notificationCatalog -- полный каталог метаданных, ключ -- код типа.
var notificationCatalog = map[string]NotificationMeta{
	NotificationTypeApplicationCreated: {
		Code: NotificationTypeApplicationCreated, Category: NotificationCategoryApplication,
		Label:       "Заявка отправлена",
		Description: "Ваша заявка принята системой и ожидает согласования.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationApprovalRequired: {
		Code: NotificationTypeApplicationApprovalRequired, Category: NotificationCategoryApplication,
		Label:       "Требуется согласование",
		Description: "Поступила заявка, которая ждёт вашего решения как согласующего.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityHigh,
	},
	NotificationTypeApplicationForwarded: {
		Code: NotificationTypeApplicationForwarded, Category: NotificationCategoryApplication,
		Label:       "Заявка передана для просмотра",
		Description: "Вам открыли доступ к заявке для ознакомления.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationStatusChanged: {
		Code: NotificationTypeApplicationStatusChanged, Category: NotificationCategoryApplication,
		Label:       "Изменился статус заявки",
		Description: "Ваша заявка перешла в новый статус (принята, отклонена, согласована, завершена).",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeApplicationQuestion: {
		Code: NotificationTypeApplicationQuestion, Category: NotificationCategoryApplication,
		Label:       "Новый вопрос по заявке",
		Description: "Согласующий задал вопрос по вашей заявке.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationAnswer: {
		Code: NotificationTypeApplicationAnswer, Category: NotificationCategoryApplication,
		Label:       "Новый ответ на вопрос",
		Description: "На вопрос по заявке, в котором вы участвуете, пришёл ответ.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationSupplementReady: {
		Code: NotificationTypeApplicationSupplementReady, Category: NotificationCategoryApplication,
		Label:       "Дополнение согласовано",
		Description: "Дополнение к заявке прошло согласование и ждёт принятия.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeApplicationSupplementDecided: {
		Code: NotificationTypeApplicationSupplementDecided, Category: NotificationCategoryApplication,
		Label:       "Решение по дополнению",
		Description: "Принимающий вынес решение по вашему дополнению к заявке.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeApprovalReminder: {
		Code: NotificationTypeApprovalReminder, Category: NotificationCategoryApplication,
		Label:       "Напоминание о согласовании",
		Description: "Заявка давно ждёт вашего решения как согласующего.",
		// Не схлопывается: у напоминаний собственный интервал повтора в днях
		// (approval.reminder_repeat_days), и окно схлопывания в минутах для них не
		// работает - зато прячет повтор, если он придёт вскоре после первого.
		Mandatory: false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},

	// security -- отключить нельзя ни один из пяти.
	NotificationTypePasswordChanged: {
		Code: NotificationTypePasswordChanged, Category: NotificationCategorySecurity,
		Label:       "Пароль изменён",
		Description: "Пароль вашей учётной записи был изменён. Отключить такие уведомления нельзя.",
		Mandatory:   true, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeUserBanned: {
		Code: NotificationTypeUserBanned, Category: NotificationCategorySecurity,
		Label:       "Учётная запись заблокирована",
		Description: "Вашу учётную запись заблокировал администратор. Отключить такие уведомления нельзя.",
		Mandatory:   true, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeUserUnbanned: {
		Code: NotificationTypeUserUnbanned, Category: NotificationCategorySecurity,
		Label:       "Учётная запись разблокирована",
		Description: "Блокировку вашей учётной записи сняли. Отключить такие уведомления нельзя.",
		Mandatory:   true, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeLoginBlocked: {
		Code: NotificationTypeLoginBlocked, Category: NotificationCategorySecurity,
		Label:       "Вход временно заблокирован",
		Description: "Слишком много неудачных попыток входа в вашу учётную запись. Отключить такие уведомления нельзя.",
		Mandatory:   true, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeRoleChanged: {
		Code: NotificationTypeRoleChanged, Category: NotificationCategorySecurity,
		Label:       "Изменились роль или права",
		Description: "Администратор изменил вашу роль или права доступа. Отключить такие уведомления нельзя.",
		Mandatory:   true, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},

	// passage
	NotificationTypeApplicationExpiring: {
		Code: NotificationTypeApplicationExpiring, Category: NotificationCategoryPassage,
		Label:       "Срок действия пропуска истекает",
		Description: "Срок действия пропуска по заявке скоро истечёт.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeApplicationWithdrawn: {
		Code: NotificationTypeApplicationWithdrawn, Category: NotificationCategoryPassage,
		Label:       "Заявка отозвана",
		Description: "Заявку отозвали до того, как по ней прошли на территорию.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationAcceptorAssigned: {
		Code: NotificationTypeApplicationAcceptorAssigned, Category: NotificationCategoryPassage,
		Label:       "Назначен принимающий",
		Description: "По заявке назначили принимающего, ответственного за проход.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},
	NotificationTypeApplicationPassageFirst: {
		Code: NotificationTypeApplicationPassageFirst, Category: NotificationCategoryPassage,
		Label:       "Первый проход по заявке",
		Description: "По заявке впервые прошли на территорию.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},

	// content
	NotificationTypeNewsPublished: {
		Code: NotificationTypeNewsPublished, Category: NotificationCategoryContent,
		Label:       "Опубликована новость",
		Description: "На сайте бюро вышла новая новость.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeDocumentPublished: {
		Code: NotificationTypeDocumentPublished, Category: NotificationCategoryContent,
		Label:       "Опубликован документ",
		Description: "В разделе документов появился новый или обновлённый документ.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeFeedbackCreated: {
		Code: NotificationTypeFeedbackCreated, Category: NotificationCategoryContent,
		Label:       "Новое обращение обратной связи",
		Description: "Пришло новое обращение через форму обратной связи.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeFeedbackAnswered: {
		Code: NotificationTypeFeedbackAnswered, Category: NotificationCategoryContent,
		Label:       "Ответ по обращению",
		Description: "На ваше обращение обратной связи пришёл ответ.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},

	// system
	NotificationTypeMaintenanceScheduled: {
		Code: NotificationTypeMaintenanceScheduled, Category: NotificationCategorySystem,
		Label:       "Плановые технические работы",
		Description: "Назначено окно плановых технических работ в системе.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeTrashRestored: {
		Code: NotificationTypeTrashRestored, Category: NotificationCategorySystem,
		Label:       "Запись восстановлена из корзины",
		Description: "Запись, которую вы удаляли, восстановили из корзины.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},
	NotificationTypeArchiveQuotaWarning: {
		Code: NotificationTypeArchiveQuotaWarning, Category: NotificationCategorySystem,
		Label:       "Файловый архив заполняется",
		Description: "Файловый архив приближается к границе выделенного места на диске.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityHigh,
	},
	NotificationTypeDirectoryPending: {
		Code: NotificationTypeDirectoryPending, Category: NotificationCategorySystem,
		Label:       "Запись справочника на проверке",
		Description: "Подача заявки завела в справочнике организацию или компанию, которую нужно разобрать.",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: true, Priority: NotificationPriorityNormal,
	},
	NotificationTypeDirectoryResolved: {
		Code: NotificationTypeDirectoryResolved, Category: NotificationCategorySystem,
		Label:       "Запись справочника разобрана",
		Description: "Запись справочника, которую вы завели, разобрали (подтвердили, исправили или связали с другой).",
		Mandatory:   false, DefaultEnabled: true, Aggregatable: false, Priority: NotificationPriorityNormal,
	},
}

// notificationCategoryOrder -- фиксированный порядок категорий для NotificationCatalog()
// и NotificationCategories(), не зависящий от порядка обхода карты.
var notificationCategoryOrder = []NotificationCategory{
	NotificationCategoryApplication,
	NotificationCategorySecurity,
	NotificationCategoryPassage,
	NotificationCategoryContent,
	NotificationCategorySystem,
}

// NotificationTypeMeta возвращает метаданные типа уведомления по коду.
func NotificationTypeMeta(code string) (NotificationMeta, bool) {
	meta, ok := notificationCatalog[code]
	return meta, ok
}

// NotificationCatalog возвращает весь каталог, отсортированный по категории (в
// порядке notificationCategoryOrder), внутри категории -- по коду. Порядок
// стабилен между вызовами: используется для рендера экрана настроек.
func NotificationCatalog() []NotificationMeta {
	rank := make(map[NotificationCategory]int, len(notificationCategoryOrder))
	for i, cat := range notificationCategoryOrder {
		rank[cat] = i
	}

	out := make([]NotificationMeta, 0, len(notificationCatalog))
	for _, meta := range notificationCatalog {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return rank[out[i].Category] < rank[out[j].Category]
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// NotificationCategories возвращает список категорий в порядке отображения.
func NotificationCategories() []NotificationCategory {
	out := make([]NotificationCategory, len(notificationCategoryOrder))
	copy(out, notificationCategoryOrder)
	return out
}
