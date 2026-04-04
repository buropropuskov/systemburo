package models

import "time"

type PDConsent struct {
	ID          int        `json:"id"`
	UserID      int        `gorm:"index;not null" json:"user_id"`
	User        User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ConsentType string     `gorm:"size:50;not null" json:"consent_type"`
	Granted     bool       `gorm:"not null" json:"granted"`
	IPAddress   string     `gorm:"size:45" json:"ip_address"`
	UserAgent   string     `gorm:"type:text" json:"user_agent"`
	GrantedAt   time.Time  `json:"granted_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (PDConsent) TableName() string { return "pd_consents" }

type GrantConsentRequest struct {
	ConsentType string `json:"consent_type" validate:"required,oneof=pd_processing pd_transfer"`
}
