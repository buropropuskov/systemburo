package models

import "time"

// DailyPassReport - суточный отчёт охранника по проходам: счётчики событий
// entry/exit одного пользователя по одной таблице проходной за отчётные сутки.
// report_date = D покрывает окно [D-1 21:30, D 21:30) МСК. Строки пишет крон в
// 21:30 (startDailyPassReportSaver) идемпотентным upsert-ом по
// (report_date, table_id, user_id); источник - audit_log (append-only), поэтому
// повторный прогон лишь перезаписывает те же значения.
//
// UserID NOT NULL DEFAULT 0: сентинел «без автора» вместо NULL - NULL в составном
// unique у Postgres не конфликтует, и upsert через ON CONFLICT плодил бы дубли.
// Без FK на users/system_tables: история отчётов переживает удаление родителя
// (философия audit_log).
type DailyPassReport struct {
	ID            int       `gorm:"primaryKey" json:"id"`
	ReportDate    time.Time `gorm:"type:date;not null;uniqueIndex:idx_daily_pass_report,priority:1" json:"report_date"`
	TableID       int       `gorm:"not null;index;uniqueIndex:idx_daily_pass_report,priority:2" json:"table_id"`
	UserID        int       `gorm:"not null;default:0;uniqueIndex:idx_daily_pass_report,priority:3" json:"user_id"`
	CarEntries    int       `gorm:"not null;default:0" json:"car_entries"`
	CarExits      int       `gorm:"not null;default:0" json:"car_exits"`
	PeopleEntries int       `gorm:"not null;default:0" json:"people_entries"`
	PeopleExits   int       `gorm:"not null;default:0" json:"people_exits"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName задаёт имя таблицы явно, без gorm-плюрализации.
func (DailyPassReport) TableName() string { return "daily_pass_reports" }
