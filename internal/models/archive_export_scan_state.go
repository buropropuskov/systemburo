package models

import "time"

// ArchiveExportScanState — единственный сохраняемый курсор ночной сверки.
type ArchiveExportScanState struct {
	ID      int       `gorm:"primaryKey;autoIncrement:false"`
	Cutoff  time.Time `gorm:"not null"`
	UpperID int       `gorm:"not null;default:0"`
	AfterID int       `gorm:"not null;default:0"`
	Active  bool      `gorm:"not null;default:false"`
}

func (ArchiveExportScanState) TableName() string { return "archive_export_scan_states" }
