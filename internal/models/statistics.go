package models

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
	TotalApplications   int64                 `json:"total_applications"`
	ByAttachmentType    []AttachmentTypeCount  `json:"by_attachment_type"`
	ByStatus            []StatusCount          `json:"by_status"`
	Processed           int64                 `json:"processed"`
	InWork              int64                 `json:"in_work"`
	CarsEntered         int64                 `json:"cars_entered"`
	PeopleEntered       int64                 `json:"people_entered"`
	AvgCarsPerDay       float64               `json:"avg_cars_per_day"`
	ItemsSum            int64                 `json:"items_sum"`
	CarsOnTerritory     int64                 `json:"cars_on_territory"`
	PeopleOnTerritory   int64                 `json:"people_on_territory"`

	// Система (пользователи, справочники)
	UsersOnline        int64 `json:"users_online"`
	ActiveUsers        int64 `json:"active_users"`
	BannedUsers        int64 `json:"banned_users"`
	OpenFeedback       int64 `json:"open_feedback"`
	ActiveUnloadPlaces int64 `json:"active_unload_places"`
	BlacklistCars      int64 `json:"blacklist_cars"`
	BlacklistPeople    int64 `json:"blacklist_people"`
	UniqueCars         int64 `json:"unique_cars"`
	UniquePeople       int64 `json:"unique_people"`
}

// StatsTimelinePoint — одна точка графика (дата + количество).
// Имя StatsTimelinePoint используется вместо TimelinePoint, т.к.
// TimelinePoint уже занят в request_logs.go.
type StatsTimelinePoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
