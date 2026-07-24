package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Граница отчётных суток охранника: 21:30 МСК. report_date = D покрывает окно
// [D-1 21:30, D 21:30) - полуинтервал, событие ровно в 21:30:00 относится к
// НОВОМУ окну. Окно считается в AnalyticsLocation() (МСК), триггер крона - в
// cfg.ResetTimezone (default тот же МСК): при дефолтах зоны совпадают.
const (
	passReportBoundaryHour   = 21
	passReportBoundaryMinute = 30
)

// PassReportCounts - счётчики событий проходов за окно: каждая отметка = +1
// (считаем события, не уникальные машины/люди). Факт-машины включены: их отметки
// пишутся тем же entity_type='car' в audit_log.
type PassReportCounts struct {
	CarEntries    int `json:"car_entries"`
	CarExits      int `json:"car_exits"`
	PeopleEntries int `json:"people_entries"`
	PeopleExits   int `json:"people_exits"`
}

// PassReportRow - счётчики одного пользователя (охранника). UserID=0 - отметки
// без автора (легаси), фронт рисует «Без автора».
type PassReportRow struct {
	UserID   int    `json:"user_id"`
	UserName string `json:"user_name"`
	PassReportCounts
}

// PassReportScope - видимость строк отчёта. Итог по таблице отдаётся всегда
// (охранник видит свои цифры + итог поста); AllUsers - разбивка по всем
// пользователям (админ/super).
type PassReportScope struct {
	UserID   int
	AllUsers bool
}

// PassReportLive - живой отчёт текущего незакрытого окна [последние 21:30, now).
type PassReportLive struct {
	PeriodStart time.Time        `json:"period_start"`
	PeriodEnd   time.Time        `json:"period_end"`
	Rows        []PassReportRow  `json:"rows"`
	Totals      PassReportCounts `json:"totals"`
}

// PassReportDay - сохранённый суточный отчёт одной таблицы за report_date.
type PassReportDay struct {
	ReportDate  string           `json:"report_date"`
	PeriodStart time.Time        `json:"period_start"`
	PeriodEnd   time.Time        `json:"period_end"`
	Rows        []PassReportRow  `json:"rows"`
	Totals      PassReportCounts `json:"totals"`
}

// DailyPassReportService - суточные отчёты охранника по проходам: живое окно
// считается из audit_log на лету, закрытые окна фиксируются кроном в
// daily_pass_reports (история по дням/таблицам/охранникам).
type DailyPassReportService interface {
	// SaveDailyReports агрегирует окно отчётной даты и идемпотентно upsert-ит
	// строки. Пустые группы не пишутся.
	SaveDailyReports(ctx context.Context, reportDate time.Time) error
	// CatchUp дозаписывает пропущенные дни до последнего закрытого окна (сервер
	// мог лежать в 21:30); при пустой таблице делает полный backfill из audit_log.
	CatchUp(ctx context.Context) error
	// Live считает текущее незакрытое окно [последние 21:30, now) на лету.
	Live(ctx context.Context, tableID int, scope PassReportScope) (*PassReportLive, error)
	// ListDays возвращает сохранённые отчёты таблицы за период по report_date
	// (без фильтра - последние listDaysDefaultWindow дней), новые первыми.
	ListDays(ctx context.Context, tableID int, from, to *time.Time, scope PassReportScope) ([]PassReportDay, error)
}

// listDaysDefaultWindow - глубина истории по умолчанию, когда фильтр периода не
// задан: без капа выдача росла бы неограниченно (день = до N строк на охранника).
const listDaysDefaultWindow = 31

type dailyPassReportService struct {
	db  *gorm.DB
	now func() time.Time
}

// NewDailyPassReportService собирает сервис суточных отчётов по проходам.
func NewDailyPassReportService(db *gorm.DB) DailyPassReportService {
	return NewDailyPassReportServiceAt(db, time.Now)
}

// NewDailyPassReportServiceAt - конструктор с инъекцией момента «сейчас» для
// детерминированных тестов границ окна (иначе тест ловит баг только в узкое
// время суток).
func NewDailyPassReportServiceAt(db *gorm.DB, now func() time.Time) DailyPassReportService {
	return &dailyPassReportService{db: db, now: now}
}

// passReportWindow возвращает границы окна отчётной даты d: [d-1 21:30, d 21:30)
// МСК. От d используются только Year/Month/Day.
func passReportWindow(d time.Time) (from, to time.Time) {
	loc := AnalyticsLocation()
	to = time.Date(d.Year(), d.Month(), d.Day(), passReportBoundaryHour, passReportBoundaryMinute, 0, 0, loc)
	from = time.Date(d.Year(), d.Month(), d.Day()-1, passReportBoundaryHour, passReportBoundaryMinute, 0, 0, loc)
	return from, to
}

// latestPassBoundary - последняя граница 21:30 МСК, не позже now. Это и начало
// текущего живого окна, и момент закрытия последнего сохраняемого: report_date
// закрытого окна = дата этой границы.
func latestPassBoundary(now time.Time) time.Time {
	loc := AnalyticsLocation()
	n := now.In(loc)
	b := time.Date(n.Year(), n.Month(), n.Day(), passReportBoundaryHour, passReportBoundaryMinute, 0, 0, loc)
	if n.Before(b) {
		b = b.AddDate(0, 0, -1)
	}
	return b
}

// passReportGracePeriod - сколько после границы 21:30 живой отчёт продолжает
// показывать ТОЛЬКО ЧТО ЗАКРЫТУЮ смену, а не новую пустую. Охранник, открывший
// отчёт сразу после 21:30, растеряется на нулях («где мои проходы за день?») -
// в это окно отдаём его отработанную смену целиком.
const passReportGracePeriod = 15 * time.Minute

// liveWindow - границы живого отчёта. Обычно [последняя граница 21:30, now). Но в
// первые passReportGracePeriod после границы - завершившаяся смена целиком
// [предыдущая граница, последняя граница): даём доработавшему охраннику увидеть
// свою смену, пока он не перешёл на новую. По истечении grace показываем новую
// (изначально пустую) смену.
func liveWindow(now time.Time) (from, to time.Time) {
	b := latestPassBoundary(now)
	if now.Sub(b) < passReportGracePeriod {
		return b.AddDate(0, 0, -1), b
	}
	return b, now
}

// passReportDateOnly нормализует дату к полуночи UTC: колонка report_date имеет
// тип date, произвольный time.Time с зоной дал бы драйверу неоднозначный день.
func passReportDateOnly(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// passAggSelect - счётчики агрегата audit_log. Читаем audit_log напрямую, а не
// через carsHistoryUnion/employeesHistoryUnion: union-проекции стирают
// entity_type, а здесь машины и люди считаются одним проходом COUNT(*) FILTER
// (индекс idx_audit_entity_action покрывает).
const passAggSelect = `
	COALESCE(a.actor_user_id, 0) AS user_id,
	COUNT(*) FILTER (WHERE a.entity_type = 'car' AND a.action = 'entry')      AS car_entries,
	COUNT(*) FILTER (WHERE a.entity_type = 'car' AND a.action = 'exit')       AS car_exits,
	COUNT(*) FILTER (WHERE a.entity_type = 'employee' AND a.action = 'entry') AS people_entries,
	COUNT(*) FILTER (WHERE a.entity_type = 'employee' AND a.action = 'exit')  AS people_exits`

// passAggRow - строка агрегата за окно: одна пара (таблица, пользователь).
type passAggRow struct {
	TableID       int
	UserID        int
	CarEntries    int
	CarExits      int
	PeopleEntries int
	PeopleExits   int
}

// aggregateWindow считает события entry/exit за [from, to) с группировкой по
// (table_id, user_id). Отметки без details.table_id (легаси до #1036) исключаются:
// пост неизвестен, к таблице их не отнести.
func (s *dailyPassReportService) aggregateWindow(ctx context.Context, from, to time.Time, tableID *int) ([]passAggRow, error) {
	q := s.db.WithContext(ctx).
		Table("audit_log a").
		Select("(a.details->>'table_id')::int AS table_id, "+passAggSelect).
		Where("a.entity_type IN ('car', 'employee')").
		Where("a.action IN ('entry', 'exit')").
		Where("a.created_at >= ? AND a.created_at < ?", from, to).
		Where("a.details->>'table_id' IS NOT NULL").
		Group("1, 2")
	if tableID != nil {
		q = q.Where("(a.details->>'table_id')::int = ?", *tableID)
	}

	var rows []passAggRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate pass events: %w", err)
	}
	return rows, nil
}

func (s *dailyPassReportService) SaveDailyReports(ctx context.Context, reportDate time.Time) error {
	from, to := passReportWindow(reportDate)
	agg, err := s.aggregateWindow(ctx, from, to, nil)
	if err != nil {
		return fmt.Errorf("failed to build daily pass report: %w", err)
	}

	day := passReportDateOnly(reportDate)
	rows := make([]models.DailyPassReport, 0, len(agg))
	for _, r := range agg {
		rows = append(rows, models.DailyPassReport{
			ReportDate:    day,
			TableID:       r.TableID,
			UserID:        r.UserID,
			CarEntries:    r.CarEntries,
			CarExits:      r.CarExits,
			PeopleEntries: r.PeopleEntries,
			PeopleExits:   r.PeopleExits,
		})
	}
	return s.upsertReports(ctx, rows)
}

// upsertReports идемпотентно сохраняет строки отчётов: повторный прогон
// перезаписывает те же значения (audit_log append-only, окно закрыто).
func (s *dailyPassReportService) upsertReports(ctx context.Context, rows []models.DailyPassReport) error {
	if len(rows) == 0 {
		return nil // пустое окно - нулевые строки не пишем
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "report_date"}, {Name: "table_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"car_entries", "car_exits", "people_entries", "people_exits", "updated_at"}),
	}).CreateInBatches(rows, 200).Error; err != nil {
		return fmt.Errorf("failed to upsert daily pass reports: %w", err)
	}
	return nil
}

func (s *dailyPassReportService) CatchUp(ctx context.Context) error {
	lastBoundary := latestPassBoundary(s.now())

	var maxDate sql.NullTime
	if err := s.db.WithContext(ctx).
		Model(&models.DailyPassReport{}).
		Select("MAX(report_date)").
		Scan(&maxDate).Error; err != nil {
		return fmt.Errorf("failed to read last pass report date: %w", err)
	}

	if !maxDate.Valid {
		// Первый запуск: полный backfill истории из audit_log одним запросом.
		return s.backfillAll(ctx, lastBoundary)
	}

	last := passReportDateOnly(lastBoundary)
	for d := passReportDateOnly(maxDate.Time).AddDate(0, 0, 1); !d.After(last); d = d.AddDate(0, 0, 1) {
		if err := s.SaveDailyReports(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// backfillAll агрегирует ВСЮ историю audit_log до границы upTo (последнее
// закрытое окно) одним запросом. Бакет отчётной даты - сдвиг +2ч30м: он маппит
// границу 21:30 на полночь следующего дня, т.е. date_trunc('day', msk + 2:30)
// даёт report_date окна события (22.07 21:45 -> 23.07; 22.07 21:15 -> 22.07).
func (s *dailyPassReportService) backfillAll(ctx context.Context, upTo time.Time) error {
	// Поля перечислены плоско: анонимно встроенную структуру gorm при Scan не
	// маппит (нужен тег embedded), счётчики молча остались бы нулями.
	type backfillRow struct {
		ReportDate    time.Time
		TableID       int
		UserID        int
		CarEntries    int
		CarExits      int
		PeopleEntries int
		PeopleExits   int
	}

	sel := fmt.Sprintf(
		"(date_trunc('day', %s + interval '2 hours 30 minutes'))::date AS report_date, (a.details->>'table_id')::int AS table_id, %s",
		tzColumn("a.created_at"), passAggSelect)
	var rows []backfillRow
	if err := s.db.WithContext(ctx).
		Table("audit_log a").
		Select(sel).
		Where("a.entity_type IN ('car', 'employee')").
		Where("a.action IN ('entry', 'exit')").
		Where("a.created_at < ?", upTo).
		Where("a.details->>'table_id' IS NOT NULL").
		Group("1, 2, 3").
		// ORDER BY report_date - страховка водяного знака CatchUp (MAX(report_date)):
		// мульти-батч CreateInBatches у gorm атомарен (обёрнут в транзакцию), но при
		// включении SkipDefaultTransaction частичная запись сохранила бы непрерывный
		// префикс дат снизу, а не дыры в середине истории.
		Order("1").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("failed to backfill daily pass reports: %w", err)
	}

	reports := make([]models.DailyPassReport, 0, len(rows))
	for _, r := range rows {
		reports = append(reports, models.DailyPassReport{
			ReportDate:    passReportDateOnly(r.ReportDate),
			TableID:       r.TableID,
			UserID:        r.UserID,
			CarEntries:    r.CarEntries,
			CarExits:      r.CarExits,
			PeopleEntries: r.PeopleEntries,
			PeopleExits:   r.PeopleExits,
		})
	}
	return s.upsertReports(ctx, reports)
}

func (s *dailyPassReportService) Live(ctx context.Context, tableID int, scope PassReportScope) (*PassReportLive, error) {
	from, to := liveWindow(s.now())
	agg, err := s.aggregateWindow(ctx, from, to, &tableID)
	if err != nil {
		return nil, err
	}

	rows, totals, err := s.scopeRows(ctx, agg, scope)
	if err != nil {
		return nil, err
	}
	return &PassReportLive{PeriodStart: from, PeriodEnd: to, Rows: rows, Totals: totals}, nil
}

func (s *dailyPassReportService) ListDays(ctx context.Context, tableID int, from, to *time.Time, scope PassReportScope) ([]PassReportDay, error) {
	q := s.db.WithContext(ctx).
		Model(&models.DailyPassReport{}).
		Where("table_id = ?", tableID)
	if from == nil && to == nil {
		cutoff := passReportDateOnly(latestPassBoundary(s.now())).AddDate(0, 0, -listDaysDefaultWindow)
		q = q.Where("report_date >= ?", cutoff)
	} else {
		if from != nil {
			q = q.Where("report_date >= ?", passReportDateOnly(*from))
		}
		if to != nil {
			q = q.Where("report_date <= ?", passReportDateOnly(*to))
		}
	}

	var recs []models.DailyPassReport
	if err := q.Order("report_date DESC, user_id ASC").Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("failed to list daily pass reports: %w", err)
	}

	days := make([]PassReportDay, 0)
	for _, rec := range recs {
		key := rec.ReportDate.Format("2006-01-02")
		if len(days) == 0 || days[len(days)-1].ReportDate != key {
			wFrom, wTo := passReportWindow(rec.ReportDate)
			days = append(days, PassReportDay{ReportDate: key, PeriodStart: wFrom, PeriodEnd: wTo})
		}
		d := &days[len(days)-1]
		d.Totals.CarEntries += rec.CarEntries
		d.Totals.CarExits += rec.CarExits
		d.Totals.PeopleEntries += rec.PeopleEntries
		d.Totals.PeopleExits += rec.PeopleExits
		if scope.AllUsers || rec.UserID == scope.UserID {
			d.Rows = append(d.Rows, PassReportRow{
				UserID: rec.UserID,
				PassReportCounts: PassReportCounts{
					CarEntries:    rec.CarEntries,
					CarExits:      rec.CarExits,
					PeopleEntries: rec.PeopleEntries,
					PeopleExits:   rec.PeopleExits,
				},
			})
		}
	}

	if err := s.fillUserNames(ctx, days); err != nil {
		return nil, err
	}
	return days, nil
}

// scopeRows применяет видимость: итог по таблице считается по ВСЕМ строкам окна
// (охранник видит итог поста), в rows не-админу остаются только его строки.
func (s *dailyPassReportService) scopeRows(ctx context.Context, agg []passAggRow, scope PassReportScope) ([]PassReportRow, PassReportCounts, error) {
	var totals PassReportCounts
	rows := make([]PassReportRow, 0, len(agg))
	for _, r := range agg {
		totals.CarEntries += r.CarEntries
		totals.CarExits += r.CarExits
		totals.PeopleEntries += r.PeopleEntries
		totals.PeopleExits += r.PeopleExits
		if scope.AllUsers || r.UserID == scope.UserID {
			rows = append(rows, PassReportRow{
				UserID: r.UserID,
				PassReportCounts: PassReportCounts{
					CarEntries:    r.CarEntries,
					CarExits:      r.CarExits,
					PeopleEntries: r.PeopleEntries,
					PeopleExits:   r.PeopleExits,
				},
			})
		}
	}

	names, err := s.userNames(ctx, collectPassUserIDs(rows))
	if err != nil {
		return nil, PassReportCounts{}, err
	}
	for i := range rows {
		rows[i].UserName = names[rows[i].UserID]
	}
	return rows, totals, nil
}

// fillUserNames резолвит имена по всем строкам списка дней одним запросом.
func (s *dailyPassReportService) fillUserNames(ctx context.Context, days []PassReportDay) error {
	ids := make([]int, 0)
	seen := map[int]bool{}
	for _, d := range days {
		for _, r := range d.Rows {
			if r.UserID > 0 && !seen[r.UserID] {
				seen[r.UserID] = true
				ids = append(ids, r.UserID)
			}
		}
	}
	names, err := s.userNames(ctx, ids)
	if err != nil {
		return err
	}
	for di := range days {
		for ri := range days[di].Rows {
			days[di].Rows[ri].UserName = names[days[di].Rows[ri].UserID]
		}
	}
	return nil
}

// collectPassUserIDs - уникальные положительные user_id строк (0 = «без автора»,
// имя не резолвится).
func collectPassUserIDs(rows []PassReportRow) []int {
	ids := make([]int, 0, len(rows))
	seen := map[int]bool{}
	for _, r := range rows {
		if r.UserID > 0 && !seen[r.UserID] {
			seen[r.UserID] = true
			ids = append(ids, r.UserID)
		}
	}
	return ids
}

// userNames возвращает ФИО пользователей (фолбэк - username), как в
// history-сервисах. Пропавший из users id остаётся без имени - фронт рисует
// «Без автора»/прочерк.
func (s *dailyPassReportService) userNames(ctx context.Context, ids []int) (map[int]string, error) {
	names := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	var rows []struct {
		ID       int
		UserName string
	}
	if err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, COALESCE(NULLIF(TRIM(CONCAT(
			COALESCE(u.last_name, ''),
			CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
			CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
		)), ''), u.username) AS user_name`).
		Where("u.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to resolve pass report user names: %w", err)
	}
	for _, r := range rows {
		names[r.ID] = r.UserName
	}
	return names, nil
}
