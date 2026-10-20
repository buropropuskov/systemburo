package models

import "time"

const (
	ApplicationFileReservationActive  = "active"
	ApplicationFileReservationCleanup = "cleanup_pending"
)

// ApplicationFileReservation accounts for files before ApplicationFile exists.
// Only server-generated names are stored here. Completed reservations are removed.
// UploadedBy deliberately has no cascade: deleting an account must not discard
// the paths and quota needed to clean its unfinished uploads.
type ApplicationFileReservation struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	UploadedBy    int       `gorm:"not null;index:idx_file_reservation_owner_state,priority:1;check:file_reservation_owner_positive,uploaded_by > 0"`
	State         string    `gorm:"size:24;not null;index:idx_file_reservation_owner_state,priority:2;index:idx_file_reservation_state_expiry,priority:1;check:file_reservation_state_valid,state IN ('active','cleanup_pending')"`
	ReservedCount int64     `gorm:"not null;check:file_reservation_count_positive,reserved_count > 0"`
	ReservedBytes int64     `gorm:"not null;check:file_reservation_bytes_valid,reserved_bytes >= 0 AND prepared_bytes >= 0 AND prepared_bytes <= reserved_bytes"`
	PreparedBytes int64     `gorm:"not null;default:0"`
	StoredNames   []string  `gorm:"type:jsonb;serializer:json;not null;default:'[]'"`
	WorkerID      string    `gorm:"type:uuid;not null"`
	CreatedAt     time.Time `gorm:"not null"`
	ExpiresAt     time.Time `gorm:"not null;index:idx_file_reservation_state_expiry,priority:2"`
}
