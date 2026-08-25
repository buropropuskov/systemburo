package models

import "time"

// ApplicationSupplement - дополнение заявки: партия сущностей, добавленных во вложения уже
// поданной заявки (#1685). Существует отдельно от статусов самой заявки намеренно.
//
// Допуск строки на КПП сегодня производный от заявки: три запроса видимости требуют
// confirmation='Согласовано' И status IN ('В работе','Завершено'). Откат заявки на повторное
// согласование снял бы с КПП всех уже допущенных людей и машины, а этого делать нельзя -
// выданные пропуска должны продолжать работать, пока согласуют добавку. Поэтому
// confirmation/status заявки не откатываются, а повторный круг живёт здесь.
type ApplicationSupplement struct {
	ID int `json:"id"`
	// ApplicationID+Number уникальны: номер раунда считается как max+1, и уникальный индекс
	// не даёт двум одновременным подачам получить одинаковый номер.
	ApplicationID int         `gorm:"uniqueIndex:idx_app_supplement_number,priority:1" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Number        int         `gorm:"uniqueIndex:idx_app_supplement_number,priority:2" json:"number"`
	Status        string      `gorm:"size:20;index" json:"status"`
	// Comment - сопроводительный текст автора: зачем понадобилась добавка.
	Comment         *string   `gorm:"type:text" json:"comment"`
	CreatedByUserID int       `gorm:"index" json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	// ConfirmationDatetime - момент выхода из pending (согласовано либо отклонено), по образцу
	// applications.confirmation_datetime.
	ConfirmationDatetime *time.Time `json:"confirmation_datetime"`
	// DecidedBy/DecisionComment/DecidedAt - кто и когда закрыл раунд решением: принимающий
	// при принятии/отказе либо автор заявки при снятии. Терминальный статус говорит, какое
	// именно это было решение, поэтому отдельной колонки под роль решившего не заведено.
	DecidedByUserID *int       `json:"decided_by_user_id"`
	DecisionComment *string    `gorm:"type:text" json:"decision_comment"`
	DecidedAt       *time.Time `json:"decided_at"`
}

// ApplicationSupplementApproval - голос согласующего по дополнению. Зеркало
// ApplicationResponsibleUser, но отдельной таблицей, а не сбросом голосов основного круга:
// сброс потянул бы updateConfirmationBasedOnApprovals, а тот пишет прямо в
// applications.confirmation и уронил бы допуск уже принятых строк. Состав голосующих -
// снимок ответственных заявки на момент подачи дополнения.
type ApplicationSupplementApproval struct {
	ID           int                   `json:"id"`
	SupplementID int                   `gorm:"uniqueIndex:idx_app_supplement_approval,priority:1" json:"supplement_id"`
	Supplement   ApplicationSupplement `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID       int                   `gorm:"uniqueIndex:idx_app_supplement_approval,priority:2" json:"user_id"`
	User         User                  `json:"-"`
	CreatedAt    time.Time             `json:"created_at"`

	RequiredApproval bool       `gorm:"default:false" json:"required_approval"`
	ApprovalStatus   *string    `gorm:"size:20;default:'pending'" json:"approval_status"`
	ApprovalComment  *string    `gorm:"type:text" json:"approval_comment"`
	ApprovalDatetime *time.Time `json:"approval_datetime"`
}
