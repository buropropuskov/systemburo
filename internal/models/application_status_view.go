package models

import "time"

// ApplicationStatusView - per-user отметка "видел текущий статус заявки" (#1349) для флага
// "статус обновился". Бинарный application_reads не подходит (залипает после первого
// прочтения); seen_at обновляется при каждом открытии детали заявки и сравнивается с
// applications.status_updated_at. Эталон паттерна - ApplicationQuestionView.
type ApplicationStatusView struct {
	ID            int         `json:"id"`
	ApplicationID int         `gorm:"uniqueIndex:idx_app_user_status_view,priority:1" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID        int         `gorm:"uniqueIndex:idx_app_user_status_view,priority:2" json:"user_id"`
	User          User        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	SeenAt        time.Time   `json:"seen_at"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationStatusView) TableName() string { return "application_status_views" }
