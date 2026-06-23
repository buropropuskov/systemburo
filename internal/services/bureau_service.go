package services

import (
	"context"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// BureauService -- бизнес-логика расписания работы Бюро. Бюро в системе одно,
// поэтому это single-owner: слоты не привязаны к родителю.
type BureauService interface {
	GetTimeSlots(ctx context.Context) ([]models.BureauTimeSlot, error)
	AddTimeSlot(ctx context.Context, req CreateTimeSlotRequest) (int, error)
	UpdateTimeSlot(ctx context.Context, slotID int, req UpdateTimeSlotRequest) error
	DeleteTimeSlot(ctx context.Context, slotID int) error
}

type bureauService struct {
	db *gorm.DB
}

// NewBureauService создаёт реализацию BureauService.
func NewBureauService(db *gorm.DB) BureauService {
	return &bureauService{db: db}
}

// timeSlots -- общий стор слотов в single-owner режиме: fkColumn пуст (нет
// партиционирования по родителю), проверка родителя всегда успешна.
func (s *bureauService) timeSlots() timeSlotStore[models.BureauTimeSlot] {
	return timeSlotStore[models.BureauTimeSlot]{
		db:          s.db,
		entity:      "bureau",
		fkColumn:    "",
		checkParent: func(_ context.Context, _ int) error { return nil },
		newSlot: func(_ int, req models.CreateTimeSlotRequest) models.BureauTimeSlot {
			return models.BureauTimeSlot{
				DayOfWeek: req.DayOfWeek,
				OpenTime:  req.OpenTime,
				CloseTime: req.CloseTime,
				IsNextDay: req.IsNextDay != nil && *req.IsNextDay,
				IsActive:  req.IsActive == nil || *req.IsActive,
			}
		},
	}
}

// GetTimeSlots возвращает все слоты расписания Бюро.
func (s *bureauService) GetTimeSlots(ctx context.Context) ([]models.BureauTimeSlot, error) {
	return s.timeSlots().list(ctx, 0)
}

// AddTimeSlot добавляет слот в расписание Бюро.
func (s *bureauService) AddTimeSlot(ctx context.Context, req CreateTimeSlotRequest) (int, error) {
	return s.timeSlots().add(ctx, 0, models.CreateTimeSlotRequest(req))
}

// UpdateTimeSlot обновляет слот расписания Бюро.
func (s *bureauService) UpdateTimeSlot(ctx context.Context, slotID int, req UpdateTimeSlotRequest) error {
	return s.timeSlots().update(ctx, 0, slotID, models.UpdateTimeSlotRequest(req))
}

// DeleteTimeSlot удаляет слот расписания Бюро.
func (s *bureauService) DeleteTimeSlot(ctx context.Context, slotID int) error {
	return s.timeSlots().remove(ctx, 0, slotID)
}
