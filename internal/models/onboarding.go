package models

import "time"

// Ключи онбординг-туров. Каждый тур версионируется и проходится отдельно: один
// человек бывает сразу администратором, принимающим и согласующим, и подъём версии
// одного тура не должен сбрасывать прохождение остальных (#1737).
const (
	TourUser    = "user"
	TourGuard   = "guard"
	TourApprove = "approve"
	TourAccept  = "accept"
	TourAdmin   = "admin"
)

// TourKeys - единственный источник правды о составе туров: по нему валидируется
// ключ из запроса, собирается ответ статуса (все ключи присутствуют, непройденный
// = null) и помечаются тестовые учётки. Ключ приходит от клиента, поэтому без
// белого списка любая опечатка заводила бы новую строку прогресса.
var TourKeys = []string{TourUser, TourGuard, TourApprove, TourAccept, TourAdmin}

// IsValidTourKey сообщает, известен ли ключ тура.
func IsValidTourKey(key string) bool {
	for _, k := range TourKeys {
		if k == key {
			return true
		}
	}
	return false
}

// UserOnboardingProgress - прохождение ОДНОГО тура одним пользователем. Строки нет =
// тур не пройден; хранится per-user (а не в браузере), чтобы прогресс ехал за
// человеком между устройствами и сбрасывался администратором.
type UserOnboardingProgress struct {
	ID      int    `json:"id"`
	UserID  int    `gorm:"not null;uniqueIndex:idx_user_onboarding_tour,priority:1" json:"user_id"`
	User    User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	TourKey string `gorm:"size:32;not null;uniqueIndex:idx_user_onboarding_tour,priority:2" json:"tour_key"`
	// CompletedVersion - версия тура, на которой он был пройден. Только растёт:
	// повторная отметка меньшей версией (устаревшая вкладка) прогресс не понижает.
	CompletedVersion int `gorm:"not null" json:"completed_version"`
	// Finished различает «дошёл до конца» и «закрыл на середине». Строка появляется
	// в обоих случаях - она гасит автозапуск, - но бейдж «Пройден» в меню обучения
	// показывается только дошедшим: иначе пропуск врал бы, что человек всё посмотрел.
	Finished    bool      `gorm:"not null;default:false" json:"finished"`
	CompletedAt time.Time `json:"completed_at"`
}

func (UserOnboardingProgress) TableName() string { return "user_onboarding_progress" }
