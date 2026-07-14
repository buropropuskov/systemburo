package models

// WarningWindowRequest -- тело запроса на создание/обновление предупреждения по
// временному окну у места (разгрузки/проезда/прохода). Одна структура и для
// create, и для update: PUT перезаписывает запись целиком. У окна есть
// nullable-переключатели (день / время), и partial-обновление по указателю не
// отличило бы "не трогать поле" от "сбросить в NULL", поэтому редактор всегда
// шлёт окно целиком. DayOfWeek nil = каждый день; TimeFrom/TimeTo nil = весь день.
// Формат времени (ЧЧ:ММ) и диапазон дня недели валидируются в сервисном сторе.
type WarningWindowRequest struct {
	DayOfWeek *int    `json:"day_of_week" validate:"omitempty,min=0,max=6"`
	TimeFrom  *string `json:"time_from"`
	TimeTo    *string `json:"time_to"`
	IsNextDay *bool   `json:"is_next_day"`
	Message   string  `json:"message" validate:"required,min=1,max=1000"`
	IsActive  *bool   `json:"is_active"`
}
