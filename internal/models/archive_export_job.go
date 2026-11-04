package models

import "time"

// ArchiveExportJob хранит намерение обновить файлы заявки без их содержимого.
// FK отсутствует: удаление заявки не должно терять незавершённое задание.
type ArchiveExportJob struct {
	ApplicationID int       `gorm:"primaryKey;autoIncrement:false;index:idx_archive_export_jobs_due,priority:4"`
	Revision      string    `gorm:"type:uuid;not null"`
	State         string    `gorm:"size:16;not null;index:idx_archive_export_jobs_due,priority:1"`
	Reason        string    `gorm:"size:32;not null"`
	QueuedAt      time.Time `gorm:"not null;index:idx_archive_export_jobs_due,priority:3"`
	NextAttemptAt time.Time `gorm:"not null;index:idx_archive_export_jobs_due,priority:2"`
	Attempts      int       `gorm:"not null;default:0"`
}

func (ArchiveExportJob) TableName() string { return "archive_export_jobs" }

const (
	ArchiveExportJobPending    = "pending"
	ArchiveExportJobSuppressed = "suppressed"
)
