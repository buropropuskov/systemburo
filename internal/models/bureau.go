package models

import "time"

// BureauTimeSlot -- временной слот расписания работы Бюро. Single-owner: FK к
// родителю нет (Бюро в системе одно), но структура полей совпадает с
// UnloadPlaceTimeSlot/SystemTableTimeSlot, чтобы агрегатор режимов работы (C2)
// приводил все три типа к единой форме слота без преобразований.
type BureauTimeSlot struct {
	ID        int       `json:"id"`
	DayOfWeek int       `json:"day_of_week"` // 0=Пн..6=Вс
	OpenTime  string    `gorm:"size:10" json:"open_time"`
	CloseTime string    `gorm:"size:10" json:"close_time"`
	IsNextDay bool      `gorm:"default:false" json:"is_next_day"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetID реализует контракт timeSlotModel для общего стора слотов.
func (s BureauTimeSlot) GetID() int { return s.ID }
