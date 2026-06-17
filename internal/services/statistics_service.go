package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// StatisticsService — интерфейс бизнес-логики статистики дашборда.
type StatisticsService interface {
	GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error)
	GetTimeline(ctx context.Context, from, to time.Time, metric, granularity string) ([]models.StatsTimelinePoint, error)
	GetRecentPassages(ctx context.Context, limit int) (*models.RecentPassages, error)
}

type statisticsService struct {
	db *gorm.DB
}

// NewStatisticsService создаёт реализацию StatisticsService.
func NewStatisticsService(db *gorm.DB) StatisticsService {
	return &statisticsService{db: db}
}

// GetSummary возвращает сводную статистику за период [from, to].
func (s *statisticsService) GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error) {
	var summary models.StatsSummary

	// total_applications
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ?", from, to).
		Count(&summary.TotalApplications).Error; err != nil {
		return nil, fmt.Errorf("statistics: count applications: %w", err)
	}

	// by_attachment_type
	summary.ByAttachmentType = make([]models.AttachmentTypeCount, 0)
	if err := s.db.WithContext(ctx).
		Table("attachments a").
		Joins("JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN unique_attachments ua ON ua.id = a.unique_attachment_id").
		Where("app.sending_datetime BETWEEN ? AND ?", from, to).
		Select("COALESCE(ua.display_name, a.attachment_display_name, a.attachment_type) AS name, COUNT(*) AS count").
		Group("COALESCE(ua.display_name, a.attachment_display_name, a.attachment_type)").
		Order("count DESC").
		Scan(&summary.ByAttachmentType).Error; err != nil {
		return nil, fmt.Errorf("statistics: by_attachment_type: %w", err)
	}

	// by_status
	summary.ByStatus = make([]models.StatusCount, 0)
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ?", from, to).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&summary.ByStatus).Error; err != nil {
		return nil, fmt.Errorf("statistics: by_status: %w", err)
	}

	// processed (терминальные статусы)
	terminalStatuses := []string{
		models.StatusCompleted,
		models.StatusApproved,
		models.StatusRejected,
		models.StatusRefused,
	}
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ? AND status IN ?", from, to, terminalStatuses).
		Count(&summary.Processed).Error; err != nil {
		return nil, fmt.Errorf("statistics: processed: %w", err)
	}

	// in_work (активные статусы)
	inWorkStatuses := []string{
		models.StatusProcessing,
		models.StatusInWork,
		models.StatusApproval,
	}
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ? AND status IN ?", from, to, inWorkStatuses).
		Count(&summary.InWork).Error; err != nil {
		return nil, fmt.Errorf("statistics: in_work: %w", err)
	}

	// cars_entered
	if err := s.db.WithContext(ctx).
		Table("cars_history").
		Where("action_type = 'entry' AND created_at BETWEEN ? AND ?", from, to).
		Count(&summary.CarsEntered).Error; err != nil {
		return nil, fmt.Errorf("statistics: cars_entered: %w", err)
	}

	// people_entered
	if err := s.db.WithContext(ctx).
		Table("employees_history").
		Where("action_type = 'entry' AND created_at BETWEEN ? AND ?", from, to).
		Count(&summary.PeopleEntered).Error; err != nil {
		return nil, fmt.Errorf("statistics: people_entered: %w", err)
	}

	// avg_cars_per_day
	days := int(to.Sub(from).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	summary.AvgCarsPerDay = float64(summary.CarsEntered) / float64(days)

	// items_sum
	var itemsSum struct{ Sum int64 }
	if err := s.db.WithContext(ctx).
		Table("items i").
		Joins("JOIN attachments a ON a.id = i.attachment_id").
		Joins("JOIN applications app ON app.id = a.application_id").
		Where("app.sending_datetime BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(i.count), 0) AS sum").
		Scan(&itemsSum).Error; err != nil {
		return nil, fmt.Errorf("statistics: items_sum: %w", err)
	}
	summary.ItemsSum = itemsSum.Sum

	// cars_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("cars").
		Where("territory_status = 1").
		Count(&summary.CarsOnTerritory).Error; err != nil {
		return nil, fmt.Errorf("statistics: cars_on_territory: %w", err)
	}

	// people_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("employees").
		Where("territory_status = 1").
		Count(&summary.PeopleOnTerritory).Error; err != nil {
		return nil, fmt.Errorf("statistics: people_on_territory: %w", err)
	}

	// users_online: онлайн = вход за последние 15 минут, прокси без сессий.
	onlineThreshold := time.Now().UTC().Add(-15 * time.Minute)
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("last_login_at >= ?", onlineThreshold).
		Count(&summary.UsersOnline).Error; err != nil {
		return nil, fmt.Errorf("statistics: users_online: %w", err)
	}

	// active_users
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_active = true AND is_banned = false").
		Count(&summary.ActiveUsers).Error; err != nil {
		return nil, fmt.Errorf("statistics: active_users: %w", err)
	}

	// banned_users
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_banned = true").
		Count(&summary.BannedUsers).Error; err != nil {
		return nil, fmt.Errorf("statistics: banned_users: %w", err)
	}

	// open_feedback
	if err := s.db.WithContext(ctx).
		Table("feedback").
		Where("status = ?", models.FeedbackOpen).
		Count(&summary.OpenFeedback).Error; err != nil {
		return nil, fmt.Errorf("statistics: open_feedback: %w", err)
	}

	// active_unload_places
	if err := s.db.WithContext(ctx).
		Table("unload_places").
		Where("is_active = true").
		Count(&summary.ActiveUnloadPlaces).Error; err != nil {
		return nil, fmt.Errorf("statistics: active_unload_places: %w", err)
	}

	// blacklist_cars
	if err := s.db.WithContext(ctx).
		Table("vehicle_blacklists").
		Where("is_active = true").
		Count(&summary.BlacklistCars).Error; err != nil {
		return nil, fmt.Errorf("statistics: blacklist_cars: %w", err)
	}

	// blacklist_people
	if err := s.db.WithContext(ctx).
		Table("person_blacklists").
		Where("is_active = true").
		Count(&summary.BlacklistPeople).Error; err != nil {
		return nil, fmt.Errorf("statistics: blacklist_people: %w", err)
	}

	// unique_cars
	if err := s.db.WithContext(ctx).
		Table("unique_cars").
		Count(&summary.UniqueCars).Error; err != nil {
		return nil, fmt.Errorf("statistics: unique_cars: %w", err)
	}

	// unique_people
	if err := s.db.WithContext(ctx).
		Table("unique_employees").
		Count(&summary.UniquePeople).Error; err != nil {
		return nil, fmt.Errorf("statistics: unique_people: %w", err)
	}

	return &summary, nil
}

// timelineSource описывает источник данных для GetTimeline.
type timelineSource struct {
	table    string
	tsColumn string
	filter   string // опциональный WHERE-фрагмент (безопасная константа, не пользовательский ввод)
}

// resolveTimelineSource возвращает параметры запроса по metric/granularity из whitelist.
// Вынесена отдельно для тестируемости без БД.
func resolveTimelineSource(metric, granularity string) (src timelineSource, unit string, err error) {
	metricMap := map[string]timelineSource{
		"applications":    {table: "applications", tsColumn: "sending_datetime", filter: ""},
		"car_entries":     {table: "cars_history", tsColumn: "created_at", filter: "action_type='entry'"},
		"people_entries":  {table: "employees_history", tsColumn: "created_at", filter: "action_type='entry'"},
	}
	granularityMap := map[string]string{
		"day":   "day",
		"week":  "week",
		"month": "month",
	}

	src, ok := metricMap[metric]
	if !ok {
		return timelineSource{}, "", fmt.Errorf("statistics: unknown metric %q", metric)
	}
	unit, ok = granularityMap[granularity]
	if !ok {
		return timelineSource{}, "", fmt.Errorf("statistics: unknown granularity %q", granularity)
	}
	return src, unit, nil
}

// GetTimeline возвращает точки графика за период [from, to].
// metric и granularity проходят через whitelist — конкатенация пользовательского ввода в SQL исключена.
func (s *statisticsService) GetTimeline(ctx context.Context, from, to time.Time, metric, granularity string) ([]models.StatsTimelinePoint, error) {
	src, unit, err := resolveTimelineSource(metric, granularity)
	if err != nil {
		return nil, err
	}

	// table, tsColumn и unit берутся только из whitelist-карт — безопасно подставлять в SQL.
	// from, to и значения filter передаются через ? плейсхолдеры.
	selectExpr := fmt.Sprintf(
		"to_char(date_trunc('%s', %s), 'YYYY-MM-DD') AS date, COUNT(*) AS count",
		unit, src.tsColumn,
	)
	groupExpr := fmt.Sprintf("date_trunc('%s', %s)", unit, src.tsColumn)

	tx := s.db.WithContext(ctx).
		Table(src.table).
		Select(selectExpr).
		Where(fmt.Sprintf("%s BETWEEN ? AND ?", src.tsColumn), from, to)

	if src.filter != "" {
		tx = tx.Where(src.filter)
	}

	points := make([]models.StatsTimelinePoint, 0)
	if err := tx.
		Group(groupExpr).
		Order(groupExpr + " ASC").
		Scan(&points).Error; err != nil {
		return nil, fmt.Errorf("statistics: timeline: %w", err)
	}

	return points, nil
}

// GetRecentPassages возвращает последние отметки проходов людей и проездов машин
// (action_type entry/exit) для живых лент дашборда. Место отметки — таблица системы
// (history.table_id -> system_tables), организация — через вложение/заявку.
func (s *statisticsService) GetRecentPassages(ctx context.Context, limit int) (*models.RecentPassages, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	result := &models.RecentPassages{
		People: make([]models.RecentPassage, 0),
		Cars:   make([]models.RecentPassage, 0),
	}

	if err := s.db.WithContext(ctx).
		Table("employees_history eh").
		Joins("JOIN employees e ON e.id = eh.employee_id").
		Joins("LEFT JOIN attachments a ON a.id = e.attachment_id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN organizations org ON org.id = app.organization_id").
		Joins("LEFT JOIN companies comp ON comp.id = app.company_id").
		Joins("LEFT JOIN system_tables st ON st.id = eh.table_id").
		Where("eh.action_type IN ?", []string{"entry", "exit"}).
		Select("eh.action_type AS action_type, eh.created_at AS created_at, " +
			"TRIM(CONCAT(e.last_name, ' ', e.first_name, ' ', COALESCE(e.middle_name, ''))) AS subject, " +
			"'' AS mark, " +
			"COALESCE(org.name, comp.name, '') AS organization, " +
			"COALESCE(st.display_name, '') AS place").
		Order("eh.created_at DESC").
		Limit(limit).
		Scan(&result.People).Error; err != nil {
		return nil, fmt.Errorf("statistics: recent people passages: %w", err)
	}

	if err := s.db.WithContext(ctx).
		Table("cars_history ch").
		Joins("JOIN cars c ON c.id = ch.car_id").
		Joins("LEFT JOIN attachments a ON a.id = c.attachment_id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN organizations org ON org.id = app.organization_id").
		Joins("LEFT JOIN companies comp ON comp.id = app.company_id").
		Joins("LEFT JOIN system_tables st ON st.id = ch.table_id").
		Where("ch.action_type IN ?", []string{"entry", "exit"}).
		Select("ch.action_type AS action_type, ch.created_at AS created_at, " +
			"c.car_number AS subject, " +
			"COALESCE(c.mark_name, '') AS mark, " +
			"COALESCE(org.name, comp.name, '') AS organization, " +
			"COALESCE(st.display_name, '') AS place").
		Order("ch.created_at DESC").
		Limit(limit).
		Scan(&result.Cars).Error; err != nil {
		return nil, fmt.Errorf("statistics: recent car passages: %w", err)
	}

	return result, nil
}
