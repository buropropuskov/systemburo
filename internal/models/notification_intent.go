package models

import "time"

// NotificationIntent retains event identifiers only. It is independent of HTTP
// cancellation. A completed technical key prevents re-enqueue duplicates until
// domain deletion through the application FK; no silent TTL sweep drops work.
type NotificationIntent struct {
	EventKey              string      `gorm:"primaryKey;size:120"`
	Kind                  string      `gorm:"size:32;not null"`
	ApplicationID         int         `gorm:"not null;index"`
	Application           Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	SupplementID          *int
	ActorUserID           int  `gorm:"not null"`
	InWork                bool `gorm:"not null;default:false"`
	PendingOrganizationID *int
	PendingCompanyID      *int
	OccurredAt            time.Time `gorm:"not null"`
	State                 string    `gorm:"size:16;not null;default:'pending';index:idx_notification_intent_due,priority:1"`
	NextAttemptAt         time.Time `gorm:"not null;index:idx_notification_intent_due,priority:2"`
	Attempts              int       `gorm:"not null;default:0"`
	CompletedAt           *time.Time
}

func (NotificationIntent) TableName() string { return "notification_intents" }
