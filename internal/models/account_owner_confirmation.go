package models

import "time"

// AccountOwnerConfirmation is a protected revision of the evidence of account
// issuance. It is intentionally independent from mutable user profile fields.
type AccountOwnerConfirmation struct {
	ID            int       `gorm:"index:idx_account_owner_revision,priority:2,sort:desc" json:"-"`
	UserID        int       `gorm:"not null;index:idx_account_owner_revision,priority:1" json:"-"`
	ConfirmedByID int       `gorm:"not null" json:"-"`
	EffectiveFrom time.Time `gorm:"not null" json:"-"`
	Evidence      string    `gorm:"type:text;not null" json:"-"`
	CreatedAt     time.Time `gorm:"not null" json:"-"`
}

// AccountOwnerVerification keeps the evidence supporting an audited lookup
// separate from the public application history.
type AccountOwnerVerification struct {
	ID              int       `json:"-"`
	ApprovalEventID int       `gorm:"not null;index" json:"-"`
	ActorUserID     int       `gorm:"not null" json:"-"`
	ConfirmationID  *int      `json:"-"`
	Reason          string    `gorm:"type:text;not null" json:"-"`
	CreatedAt       time.Time `gorm:"not null" json:"-"`
}
