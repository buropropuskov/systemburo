package models

import "time"

// AttachmentTypeCount — количество вложений по типу за период.
type AttachmentTypeCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// StatusCount — количество заявок по статусу за период.
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// StatsSummary — сводная статистика дашборда за период.
type StatsSummary struct {
	// Данные (заявки, въезды, товары)
	TotalApplications int64                 `json:"total_applications"`
	ByAttachmentType  []AttachmentTypeCount `json:"by_attachment_type"`
	ByStatus          []StatusCount         `json:"by_status"`
	Processed         int64                 `json:"processed"`
	InWork            int64                 `json:"in_work"`
	CarsEntered       int64                 `json:"cars_entered"`
	PeopleEntered     int64                 `json:"people_entered"`
	AvgCarsPerDay     float64               `json:"avg_cars_per_day"`
	ItemsSum          int64                 `json:"items_sum"`
	CarsOnTerritory   int64                 `json:"cars_on_territory"`
	PeopleOnTerritory int64                 `json:"people_on_territory"`

	// Система (пользователи, справочники)
	UsersOnline          int64 `json:"users_online"`
	UsersOnlinePeakToday int64 `json:"users_online_peak_today"`
	ActiveUsers          int64 `json:"active_users"`
	BannedUsers          int64 `json:"banned_users"`
	OpenFeedback         int64 `json:"open_feedback"`
	ActiveUnloadPlaces   int64 `json:"active_unload_places"`
	BlacklistCars        int64 `json:"blacklist_cars"`
	BlacklistPeople      int64 `json:"blacklist_people"`
	UniqueCars           int64 `json:"unique_cars"`
	UniquePeople         int64 `json:"unique_people"`
}

// UserOnlinePeak — дневной пик одновременного онлайна пользователей (#632).
// Снимок пишет фоновый тикер раз в ~1-2 мин: peak_count = MAX(peak_count, текущий
// онлайн). Одна строка на дату (UNIQUE по date), upsert по date.
type UserOnlinePeak struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	Date      time.Time `gorm:"type:date;uniqueIndex;not null" json:"date"`
	PeakCount int       `gorm:"not null;default:0" json:"peak_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OnlinePeakPoint — пик онлайна за один день для серии динамики (#632).
type OnlinePeakPoint struct {
	Date string `json:"date"`
	Peak int    `json:"peak"`
}

// OnlineUser — строка списка «кто онлайн» для модалки дашборда (#632 G7).
// Только пользователи с last_seen в окне онлайна; last_seen отдаём, чтобы фронт
// показал относительное «активен N назад». FullName собран из частей на бэке.
type OnlineUser struct {
	ID       int       `json:"id"`
	Login    string    `json:"login"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	UserType string    `json:"user_type"`
	LastSeen time.Time `json:"last_seen"`
}

// StatsTimelinePoint — одна точка графика (дата + количество).
// Имя StatsTimelinePoint используется вместо TimelinePoint, т.к.
// TimelinePoint уже занят в request_logs.go.
type StatsTimelinePoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// RecentPassage — одна отметка прохода/проезда для живой ленты дашборда.
// ActionType: entry|exit (фронт показывает людям вход/выход, машинам въезд/выезд).
// CreatedAt в UTC; в UTC+3 переводит фронт.
type RecentPassage struct {
	ActionType   string    `json:"action_type"`
	CreatedAt    time.Time `json:"created_at"`
	Subject      string    `json:"subject"`        // ФИО (люди) или гос. номер (машины)
	Mark         string    `json:"mark,omitempty"` // марка машины
	Organization string    `json:"organization"`
	Place        string    `json:"place"` // таблица системы, где отметка
}

// RecentPassages — последние проходы людей и проезды машин для живых лент.
type RecentPassages struct {
	People []RecentPassage `json:"people"`
	Cars   []RecentPassage `json:"cars"`
}
