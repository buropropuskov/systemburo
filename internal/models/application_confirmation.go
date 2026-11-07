package models

import "time"

// ApplicationConfirmationDocument retains the exact text associated with a confirmation.
type ApplicationConfirmationDocument struct {
	Version int    `gorm:"primaryKey;autoIncrement:false"`
	Hash    string `gorm:"primaryKey;size:64"`
	Text    string `gorm:"type:text;not null"`
}

type ApplicationConfirmedRow struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}

// ApplicationConfirmation records the sender's declaration for one operation.
type ApplicationConfirmation struct {
	ID              int
	ApplicationID   int                    `gorm:"not null;index;uniqueIndex:idx_application_initial_confirmation,where:supplement_id IS NULL"`
	Application     Application            `gorm:"constraint:OnDelete:CASCADE"`
	SupplementID    *int                   `gorm:"uniqueIndex"`
	Supplement      *ApplicationSupplement `gorm:"constraint:OnDelete:CASCADE"`
	ActorUserID     *int
	Actor           *User                           `gorm:"foreignKey:ActorUserID;constraint:OnDelete:SET NULL"`
	ConfirmedAt     time.Time                       `gorm:"not null"`
	DocumentVersion int                             `gorm:"not null"`
	DocumentHash    string                          `gorm:"size:64;not null"`
	Document        ApplicationConfirmationDocument `gorm:"foreignKey:DocumentVersion,DocumentHash;references:Version,Hash;constraint:OnDelete:RESTRICT"`
	Rows            []ApplicationConfirmedRow       `gorm:"type:jsonb;serializer:json;not null;default:'[]'"`
}
