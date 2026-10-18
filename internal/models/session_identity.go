package models

import "time"

// SessionIdentity is internal authentication context, not a persisted model.
// Effective identity governs object access; owner identifies the browser session.
type SessionIdentity struct {
	EffectiveUserID int
	OwnerUserID     int
	FamilyID        string
	ImpersonatedBy  *int
	AccessExpiresAt time.Time
	Username        string
	OwnerUsername   string
	TypeID          int
	IsSuperAdmin    bool
}
