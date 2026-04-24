package models

import "time"

// BugReport - репорт о клиентской 500-ошибке, отправленный пользователем со страницы Error500.
// Один юзер может отправить только один репорт на конкретный bug_hash (uniq index),
// при повторе handler возвращает 409. Stack trace не храним - только route, status
// и generic HTTP-message (без содержимого ответа сервера) для защиты от leak архитектуры.
type BugReport struct {
	ID         int       `json:"id"`
	UserID     int       `gorm:"index:idx_bug_user_hash,unique" json:"user_id"`
	BugHash    string    `gorm:"size:16;index:idx_bug_user_hash,unique" json:"bug_hash"`
	Route      string    `gorm:"size:255" json:"route"`
	HTTPStatus int       `json:"http_status"`
	Message    string    `gorm:"size:500" json:"message"`
	UserAgent  string    `gorm:"size:255" json:"user_agent"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// BugReportRequest - входной payload POST /api/bug-report.
// user_id не из body, а из JWT-context. Все поля, которые принимаем от клиента,
// имеют ограниченный размер и валидацию по максимальной длине - чтобы не принять
// огромный stack trace если фронт случайно его пришлёт.
type BugReportRequest struct {
	BugHash    string `json:"bug_hash" validate:"required,min=8,max=16"`
	Route      string `json:"route" validate:"required,max=255"`
	HTTPStatus int    `json:"http_status" validate:"required,gte=400,lte=599"`
	Message    string `json:"message" validate:"max=500"`
}
