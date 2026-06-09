package models

import "time"

// ApplicationApproverHistory — аудит-лог добавления и удаления принимающих заявки (#417).
// История глобальная (не per-approver): принимающие hard-delete, поэтому нужен снимок имени.
// FK (ApproverUserID) намеренно без constraint: аудит переживает удаление пользователя.
type ApplicationApproverHistory struct {
	ID             int       `json:"id"`
	ApproverUserID int       `gorm:"index;not null" json:"approver_user_id"`
	ApproverName   string    `gorm:"size:255" json:"approver_name"`
	ActorUserID    *int      `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType     string    `gorm:"size:32;index" json:"action_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// Константы ActionType для ApplicationApproverHistory.
const (
	ApproverActionCreated = "created"
	ApproverActionDeleted = "deleted"
)

// ApplicationApproverHistoryItem — запись аудита с именем актора для API (LEFT JOIN users).
type ApplicationApproverHistoryItem struct {
	ID             int       `json:"id"`
	ApproverUserID int       `json:"approver_user_id"`
	ApproverName   string    `json:"approver_name"`
	ActionType     string    `json:"action_type"`
	ActorUserID    *int      `json:"actor_user_id,omitempty"`
	ActorName      string    `json:"actor_name"`
	CreatedAt      time.Time `json:"created_at"`
}
