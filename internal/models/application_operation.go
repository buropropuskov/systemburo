package models

import (
	"encoding/json"
	"time"
)

// ApplicationOperation stores only command metadata and its minimal committed result.
// No request body or personal data belongs in this table.
type ApplicationOperation struct {
	OwnerUserID     int             `gorm:"primaryKey;autoIncrement:false;not null"`
	EffectiveUserID int             `gorm:"primaryKey;autoIncrement:false;not null"`
	Scope           string          `gorm:"primaryKey;size:80;not null"`
	Key             string          `gorm:"primaryKey;size:80;not null"`
	Fingerprint     string          `gorm:"size:64;not null"`
	Result          json.RawMessage `gorm:"type:jsonb"`
	CreatedAt       time.Time       `gorm:"not null;index:idx_application_operations_created"`
}
