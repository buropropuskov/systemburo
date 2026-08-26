package models

import "time"

// Константы ActionType истории принимающих заявки.
const (
	ApproverActionCreated = "created"
	ApproverActionDeleted = "deleted"
	ApproverActionRenamed = "renamed"
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
