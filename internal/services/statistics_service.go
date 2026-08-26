package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// onlineWindowMinutes — окно "онлайн": пользователь считается онлайн, если его
// last_seen обновлялся за последние N минут. Должно быть >= троттл-окна записи
// last_seen в middleware (60с), с запасом на простой между запросами.
//
// То же окно продублировано на фронте (ONLINE_WINDOW_MINUTES в
// frontend/src/utils/presence.js): таблица пользователей гасит точку присутствия
// по тикающему таймеру, без запроса к бэку. Меняя число здесь, менять и там -
// иначе плитка дашборда и колонка «В сети» дадут разные ответы.
const onlineWindowMinutes = 5

// StatisticsService — интерфейс бизнес-логики статистики дашборда.
type StatisticsService interface {
	GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error)
	// SnapshotOnlinePeak фиксирует текущий онлайн как дневной пик за сегодня
	// (upsert по date, peak_count = MAX(старый, текущий)). Зовётся фоновым тикером.
	SnapshotOnlinePeak(ctx context.Context) error
	// GetOnlinePeaks возвращает серию дневных пиков онлайна за период [from, to]
	// для карточки динамики пользователей. Дни без снимков опускаются.
	GetOnlinePeaks(ctx context.Context, from, to time.Time) ([]models.OnlinePeakPoint, error)
	// GetOnlineUsers возвращает список пользователей онлайн (last_seen в окне) по
	// убыванию свежести активности — для модалки «кто онлайн» на дашборде.
	GetOnlineUsers(ctx context.Context) ([]models.OnlineUser, error)
	// GetProcessingSummary возвращает бандл curated-вкладки «Обработка заявок»:
	// KPI этапов пути заявки со сравнением с прошлым периодом, качество обработки,
	// топ медленных согласующих и разбивку по организациям (#1240).
	GetProcessingSummary(ctx context.Context, from, to time.Time) (*models.ProcessingSummary, error)
	// GetProcessingJournal возвращает страницу сквозной ленты событий обработки
	// (согласования и принятия в работу) за период [from, to] по времени убыванием:
	// limit событий начиная с offset и общее число подходящих событий для постраничной
	// навигации. filter сужает выборку по роли и подстроке номера/актора. Реальное
	// время: без кэша (#1251 S4, страницы — P5b, фильтры и поиск — P5c).
	GetProcessingJournal(ctx context.Context, from, to time.Time, filter ProcessingJournalFilter, limit, offset int) ([]models.ProcessingJournalEntry, int64, error)
	GetTimeline(ctx context.Context, from, to time.Time, metric, granularity string) ([]models.StatsTimelinePoint, error)
	GetRecentPassages(ctx context.Context, limit int) (*models.RecentPassages, error)
	GetReportCatalog(ctx context.Context) (*models.ReportCatalog, error)
	RunReport(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error)
	RunReportList(ctx context.Context, req models.ReportRequest) (*models.ReportListResponse, error)

	GetInsights(ctx context.Context, from, to string) (*models.InsightsResponse, error)

	ListReportTemplates(ctx context.Context, userID int) ([]models.ReportTemplate, error)
	CreateReportTemplate(ctx context.Context, userID int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error)
	UpdateReportTemplate(ctx context.Context, userID, id int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error)
	DeleteReportTemplate(ctx context.Context, userID, id int) error

	// WarmCache прогревает кэш аналитики из БД (вызывать при старте до приёма трафика).
	WarmCache(ctx context.Context)
	// StartCacheRefresh запускает фоновое обновление кэша аналитики до отмены ctx
	// (блокирует, вызывать в горутине). No-op, если кэш отключён.
	StartCacheRefresh(ctx context.Context)
}

type statisticsService struct {
	db              *gorm.DB
	cacheRefresh    time.Duration
	summaryCache    *periodCache[*models.StatsSummary]
	insightsCache   *periodCache[*models.InsightsResponse]
	processingCache *periodCache[*models.ProcessingSummary]
}

// NewStatisticsService создаёт реализацию StatisticsService. cacheRefresh > 0
// включает тёплый кэш дашборда/insights (in-memory + снимок в БД) с обновлением
// раз в cacheRefresh; 0 - кэш отключён, всё считается на каждый запрос.
func NewStatisticsService(db *gorm.DB, cacheRefresh time.Duration) StatisticsService {
	s := &statisticsService{db: db, cacheRefresh: cacheRefresh}
	if cacheRefresh > 0 {
		const evict = time.Hour // период, не запрашиваемый дольше часа, выселяется
		s.summaryCache = newPeriodCache[*models.StatsSummary](db, "summary", evict, s.computeHeavySummary)
		s.insightsCache = newPeriodCache[*models.InsightsResponse](db, "insights", evict,
			func(ctx context.Context, from, to time.Time) (*models.InsightsResponse, error) {
				return s.computeInsights(ctx, from.Format("2006-01-02"), to.Format("2006-01-02"))
			})
		s.processingCache = newPeriodCache[*models.ProcessingSummary](db, "processing", evict, s.computeProcessingSummary)
	}
	return s
}

// WarmCache загружает снимки аналитики из БД в память (прогрев после рестарта).
func (s *statisticsService) WarmCache(ctx context.Context) {
	if s.summaryCache != nil {
		s.summaryCache.warmup(ctx)
	}
	if s.insightsCache != nil {
		s.insightsCache.warmup(ctx)
	}
	if s.processingCache != nil {
		s.processingCache.warmup(ctx)
	}
}

// StartCacheRefresh периодически обновляет кэш аналитики до отмены ctx.
func (s *statisticsService) StartCacheRefresh(ctx context.Context) {
	if s.cacheRefresh <= 0 {
		return
	}
	ticker := time.NewTicker(s.cacheRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.summaryCache != nil {
				s.summaryCache.refresh(ctx)
			}
			if s.insightsCache != nil {
				s.insightsCache.refresh(ctx)
			}
			if s.processingCache != nil {
				s.processingCache.refresh(ctx)
			}
		}
	}
}

// GetSummary возвращает сводную статистику за период [from, to]. Тяжёлые агрегаты
// берутся из тёплого кэша (если включён), realtime-показатели (онлайн, на
// территории) всегда считаются на лету и домешиваются к снимку.
func (s *statisticsService) GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error) {
	var heavy *models.StatsSummary
	var err error
	if s.summaryCache != nil {
		heavy, err = s.summaryCache.get(ctx, from, to)
	} else {
		heavy, err = s.computeHeavySummary(ctx, from, to)
	}
	if err != nil {
		return nil, err
	}
	// Копия, чтобы realtime-поля не мутировали общий кэшированный снимок.
	out := *heavy
	if err := s.computeRealtimeSummary(ctx, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// computeHeavySummary считает дорогие агрегаты за период и медленно меняющиеся
// счётчики - всё, что кэшируется. Realtime-показатели здесь не заполняются, их
// добавляет computeRealtimeSummary.
func (s *statisticsService) computeHeavySummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error) {
	var summary models.StatsSummary

	// total_applications
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ?", from, to).
		Count(&summary.TotalApplications).Error; err != nil {
		return nil, fmt.Errorf("statistics: count applications: %w", err)
	}

	// by_attachment_type: все активные типы из справочника (включая нулевые за
	// период) — раздел дашборда показывает блок на каждый тип системы, поэтому
	// идём от unique_attachments, а не от фактических вложений.
	summary.ByAttachmentType = make([]models.AttachmentTypeCount, 0)
	attachmentName := "COALESCE(ua.display_name, ua.name, ua.title, ua.attachment_type)"
	if err := s.db.WithContext(ctx).
		Table("unique_attachments ua").
		Joins("LEFT JOIN attachments a ON a.unique_attachment_id = ua.id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id AND app.sending_datetime BETWEEN ? AND ?", from, to).
		Where("ua.is_active = true").
		Select(attachmentName + " AS name, COUNT(app.id) AS count").
		Group(attachmentName).
		Order("count DESC, name ASC").
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

	// cars_entered: источник carsHistoryUnion (audit_log[car], #870 F.5 read-switch);
	// до-cutover въезды cars_history перенесены в audit_log backfill'ом.
	if err := s.db.WithContext(ctx).
		Table(carsHistoryUnion+" ch").
		Where("ch.action_type = 'entry' AND ch.created_at BETWEEN ? AND ?", from, to).
		Count(&summary.CarsEntered).Error; err != nil {
		return nil, fmt.Errorf("statistics: cars_entered: %w", err)
	}

	// people_entered: источник employeesHistoryUnion (audit_log[employee], #870 F.6
	// read-switch); до-cutover въезды employees_history перенесены backfill'ом.
	if err := s.db.WithContext(ctx).
		Table(employeesHistoryUnion+" eh").
		Where("eh.action_type = 'entry' AND eh.created_at BETWEEN ? AND ?", from, to).
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

// computeRealtimeSummary заполняет показатели текущего состояния: на территории
// и онлайн. Дёшево (точечные запросы), считается на каждый запрос дашборда.
func (s *statisticsService) computeRealtimeSummary(ctx context.Context, summary *models.StatsSummary) error {
	// cars_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("cars").
		Where("territory_status = 1").
		Count(&summary.CarsOnTerritory).Error; err != nil {
		return fmt.Errorf("statistics: cars_on_territory: %w", err)
	}

	// people_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("employees").
		Where("territory_status = 1").
		Count(&summary.PeopleOnTerritory).Error; err != nil {
		return fmt.Errorf("statistics: people_on_territory: %w", err)
	}

	// users_online: онлайн = активность (last_seen) за последние onlineWindowMinutes.
	online, err := s.countOnline(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("statistics: users_online: %w", err)
	}
	summary.UsersOnline = online

	// users_online_peak_today: пик одновременного онлайна за сегодня из снимков тикера.
	if err := s.db.WithContext(ctx).
		Table("user_online_peaks").
		Where("date = ?", time.Now().UTC().Format("2006-01-02")).
		Select("COALESCE(MAX(peak_count), 0)").
		Scan(&summary.UsersOnlinePeakToday).Error; err != nil {
		return fmt.Errorf("statistics: users_online_peak_today: %w", err)
	}

	return nil
}

// onlineThreshold — граница окна онлайна на момент now: пользователь онлайн, если
// last_seen >= этой границы.
func onlineThreshold(now time.Time) time.Time {
	return now.Add(-onlineWindowMinutes * time.Minute)
}

// onlineUserScope — единый предикат «пользователь онлайн»: активный, не забанен и с
// last_seen в окне на момент now. Колонки не квалифицируем — is_active/is_banned/last_seen
// есть только в users, поэтому предикат работает и при джойнах. Общий для countOnline
// (число на плитке) и GetOnlineUsers (список в модалке), чтобы счётчик и длина списка
// всегда совпадали. Забаненного/архивного отсекаем: BanCheck не даёт ему обновлять
// last_seen, но свежий last_seen до бана держал бы его «онлайн» ещё до окна.
func onlineUserScope(now time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("is_active = true AND is_banned = false").
			Where("last_seen >= ?", onlineThreshold(now))
	}
}

// countOnline считает пользователей онлайн на момент now.
func (s *statisticsService) countOnline(ctx context.Context, now time.Time) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Table("users").
		Scopes(onlineUserScope(now)).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("statistics: count online: %w", err)
	}
	return count, nil
}

// GetOnlineUsers возвращает пользователей онлайн по убыванию last_seen. Тот же предикат
// onlineUserScope, что и в countOnline, поэтому длина списка совпадает с users_online.
// ФИО собирается из частей, роль/тип — имена справочников.
func (s *statisticsService) GetOnlineUsers(ctx context.Context) ([]models.OnlineUser, error) {
	users := make([]models.OnlineUser, 0)
	fullName := "TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name))"
	if err := s.db.WithContext(ctx).
		Table("users u").
		Joins("LEFT JOIN roles r ON r.id = u.role_id").
		Joins("LEFT JOIN user_types ut ON ut.id = u.type_id").
		Scopes(onlineUserScope(time.Now().UTC())).
		Select("u.id AS id, u.username AS login, " +
			fullName + " AS full_name, " +
			"COALESCE(r.name, '') AS role, " +
			"COALESCE(ut.name, '') AS user_type, " +
			"u.last_seen AS last_seen").
		Order("u.last_seen DESC").
		Scan(&users).Error; err != nil {
		return nil, fmt.Errorf("statistics: online users: %w", err)
	}
	// Логин вместо ФИО у тех, кто не давал согласия на обработку данных.
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range users {
			users[i].FullName = maskName(masks, &users[i].ID, users[i].FullName)
		}
	}
	return users, nil
}

// SnapshotOnlinePeak обновляет дневной пик онлайна за сегодня.
// Upsert по date: при конфликте берём MAX(существующий peak_count, текущий онлайн),
// поэтому повторные вызовы за день не плодят строки и пик только растёт.
func (s *statisticsService) SnapshotOnlinePeak(ctx context.Context) error {
	now := time.Now().UTC()
	current, err := s.countOnline(ctx, now)
	if err != nil {
		return err
	}
	today := now.Format("2006-01-02")
	// ON CONFLICT (date) гарантирует одну строку на дату; peak_count монотонно
	// не убывает за день. GREATEST на стороне БД исключает гонку чтение-запись.
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO user_online_peaks (date, peak_count, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (date) DO UPDATE
		SET peak_count = GREATEST(user_online_peaks.peak_count, EXCLUDED.peak_count),
		    updated_at = EXCLUDED.updated_at`,
		today, current, now, now).Error; err != nil {
		return fmt.Errorf("statistics: snapshot online peak: %w", err)
	}
	return nil
}

// GetOnlinePeaks возвращает дневные пики онлайна за период [from, to], по возрастанию даты.
func (s *statisticsService) GetOnlinePeaks(ctx context.Context, from, to time.Time) ([]models.OnlinePeakPoint, error) {
	points := make([]models.OnlinePeakPoint, 0)
	rows := []struct {
		Date string
		Peak int
	}{}
	if err := s.db.WithContext(ctx).
		Table("user_online_peaks").
		Where("date BETWEEN ? AND ?", from.Format("2006-01-02"), to.Format("2006-01-02")).
		Select("to_char(date, 'YYYY-MM-DD') AS date, peak_count AS peak").
		Order("date ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("statistics: online peaks: %w", err)
	}
	for _, r := range rows {
		points = append(points, models.OnlinePeakPoint{Date: r.Date, Peak: r.Peak})
	}
	return points, nil
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
		"applications":   {table: "applications", tsColumn: "sending_datetime", filter: ""},
		"car_entries":    {table: carsHistoryUnion + " ch", tsColumn: "ch.created_at", filter: "ch.action_type='entry'"},
		"people_entries": {table: employeesHistoryUnion + " eh", tsColumn: "eh.created_at", filter: "eh.action_type='entry'"},
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
	// from, to и значения filter передаются через ? плейсхолдеры. Бакетинг — в МСК
	// (tzColumn), иначе сутки режутся по UTC-полуночи и точки «съезжают» на 3 часа.
	tsBucket := tzColumn(src.tsColumn)
	selectExpr := fmt.Sprintf(
		"to_char(date_trunc('%s', %s), 'YYYY-MM-DD') AS date, COUNT(*) AS count",
		unit, tsBucket,
	)
	groupExpr := fmt.Sprintf("date_trunc('%s', %s)", unit, tsBucket)

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
		Table(employeesHistoryUnion+" eh").
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
		Table(carsHistoryUnion+" ch").
		Joins("JOIN cars c ON c.id = ch.car_id").
		Joins("LEFT JOIN attachments a ON a.id = c.attachment_id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN organizations org ON org.id = app.organization_id").
		Joins("LEFT JOIN companies comp ON comp.id = app.company_id").
		Joins("LEFT JOIN system_tables st ON st.id = ch.table_id").
		Where("ch.action_type IN ?", []string{"entry", "exit"}).
		Select("ch.action_type AS action_type, ch.created_at AS created_at, " +
			"c.car_number AS subject, " +
			// mark_name - актуальное поле, car_brand - устаревший fallback (как в trash/blacklist).
			"COALESCE(NULLIF(c.mark_name, ''), c.car_brand, '') AS mark, " +
			"COALESCE(org.name, comp.name, '') AS organization, " +
			"COALESCE(st.display_name, '') AS place").
		Order("ch.created_at DESC").
		Limit(limit).
		Scan(&result.Cars).Error; err != nil {
		return nil, fmt.Errorf("statistics: recent car passages: %w", err)
	}

	return result, nil
}

// GetReportCatalog возвращает каталог конструктора отчётов: whitelist метрик,
// разрезов, фильтров и list-сущностей с подставленными значениями динамических
// справочников. Метаданные берутся из реестров (report_catalog.go), значения
// справочников — из БД.
func (s *statisticsService) GetReportCatalog(ctx context.Context) (*models.ReportCatalog, error) {
	dyn, err := s.loadDynamicReportOptions(ctx)
	if err != nil {
		return nil, err
	}
	catalog := buildReportCatalog(dyn)
	return &catalog, nil
}

// loadDynamicReportOptions подгружает значения справочников для dict-фильтров.
func (s *statisticsService) loadDynamicReportOptions(ctx context.Context) (dynamicReportOptions, error) {
	var dyn dynamicReportOptions

	// load подставляет table/column/where прямо в SQL (GORM не экранирует
	// строковые выражения), поэтому вызывать его допустимо ТОЛЬКО с константами
	// кода - никогда с пользовательским вводом.
	load := func(name, table, column, where string) ([]models.ReportOption, error) {
		var names []string
		tx := s.db.WithContext(ctx).Table(table).
			Distinct(column).
			Where(column + " IS NOT NULL AND " + column + " <> ''")
		if where != "" {
			tx = tx.Where(where)
		}
		if err := tx.Order(column+" ASC").Pluck(column, &names).Error; err != nil {
			return nil, fmt.Errorf("statistics: report catalog %s: %w", name, err)
		}
		opts := make([]models.ReportOption, 0, len(names))
		for _, n := range names {
			opts = append(opts, models.ReportOption{Value: n, Label: n})
		}
		return opts, nil
	}

	var err error
	if dyn.organizations, err = load("organizations", "organizations", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.companies, err = load("companies", "companies", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.attachmentTypes, err = load("attachment_types", "unique_attachments", "display_name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.citizenships, err = load("citizenships", "citizenships", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.unloadPlaces, err = load("unload_places", "unload_places", "name", "is_active = true"); err != nil {
		return dyn, err
	}

	return dyn, nil
}

// RunReport исполняет агрегатный отчёт конструктора (mode=aggregate): одна или
// несколько метрик (Metrics) x разрез x фильтры x период x sort/limit. Каждая
// метрика исполняется отдельным агрегатным запросом (у метрик разные базовые
// таблицы — общий GROUP BY невозможен) и сливается по подписи разреза в Go
// (report_engine.go, чистая логика). Планы собираются buildAggregatePlan из
// whitelist-схем: ввод сверяется по ключам и подставляется только через
// плейсхолдеры. Невалидный запрос -> ErrInvalidReportRequest (400 в handler).
func (s *statisticsService) RunReport(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error) {
	metrics, err := resolveReportMetrics(req)
	if err != nil {
		return nil, err
	}
	multi := len(metrics) > 1

	perMetric := make(map[string][]models.ReportAggregateRow, len(metrics))
	columns := make([]models.ReportMetricColumn, 0, len(metrics))
	for _, m := range metrics {
		mreq := req
		mreq.Metric = m
		plan, perr := buildAggregatePlan(mreq)
		if perr != nil {
			return nil, perr
		}
		// При мультиметриках итоговый лимит применяется после слияния (top-N по
		// сумме метрик), поэтому каждую метрику грузим широко.
		if multi {
			plan.limit = maxReportLimit
		}
		rows, rerr := s.execAggregatePlan(ctx, plan)
		if rerr != nil {
			return nil, rerr
		}
		perMetric[m] = rows
		columns = append(columns, models.ReportMetricColumn{
			Key:   m,
			Label: reportMetricRegistry[m].label,
			Unit:  plan.unit,
			Type:  plan.valueType,
		})
	}

	metricRows, totals := mergeMetricRows(metrics, perMetric, req.Dimension, req.Sort, clampLimit(req.Limit))

	// Cross-tab (G4): при заданном req.Pivot и разрезе period — развернуть значения
	// оси pivot в добавочные колонки-счётчики (отдельной веткой движка). Невалидный
	// pivot (неизвестная ось / неприменима к метрикам / не period) -> 400.
	axis, pivotOn, perr := resolvePivot(req.Pivot, req.Dimension, metrics)
	if perr != nil {
		return nil, perr
	}
	if pivotOn {
		pivotCols, pivotTotals, cerr := s.collectPivotColumns(ctx, metrics, axis, req, metricRows)
		if cerr != nil {
			return nil, cerr
		}
		columns = append(columns, pivotCols...)
		// Итоги pivot-колонок: mergeMetricRows считает totals только по метрикам, а
		// cross-tab-колонки добавляются после. Без этого строка «Итого» показывает 0
		// по колонкам оси (баг: суммы есть в ячейках, но не в итогах).
		for k, v := range pivotTotals {
			totals[k] = v
		}
	}

	// Метрики-средние (avg_*): целые счётчики бинов делятся на число дней бина в Go
	// (постпроцесс), значения переходят в FloatValues. Колонка помечается Float.
	var floatTotals map[string]float64
	for i, m := range metrics {
		if !avgMetrics[m] {
			continue
		}
		total := applyAvgPerDay(metricRows, m, req.Dimension, req.Granularity, req.Filters)
		columns[i].Float = true
		if floatTotals == nil {
			floatTotals = map[string]float64{}
		}
		floatTotals[m] = total
		delete(totals, m)
	}

	// Производные метрики — длительности и доли (#1240): итог — НЕ сумма значений
	// строк, которую посчитал mergeMetricRows (сумма средних/перцентилей/долей по
	// бинам бессмысленна), а тот же агрегат по всему окну отдельным запросом без
	// разреза.
	for _, m := range metrics {
		if !metricTotalNotAdditive(m) || req.Dimension == dimNone {
			continue // без разреза единственная строка уже и есть итог по окну
		}
		total, terr := s.execWindowTotal(ctx, req, m)
		if terr != nil {
			return nil, terr
		}
		totals[m] = total
	}

	// Метрики-доли (#1240, B3): SQL отдаёт их домноженными на rateScale (целое —
	// иначе скан numeric в int64 падает), здесь возвращаем дробь и переносим её в
	// FloatValues — тот же контракт, по которому фронт рисует avg-метрики.
	for i, m := range metrics {
		if !rateMetrics[m] {
			continue
		}
		columns[i].Float = true
		applyRateScale(metricRows, m)
		if floatTotals == nil {
			floatTotals = map[string]float64{}
		}
		floatTotals[m] = round1(float64(totals[m]) / rateScale)
		delete(totals, m)
	}

	// Legacy-поля одиночной метрики (текущий FE читает Rows/Total/Unit): первая
	// метрика как label/value по уже упорядоченным строкам. Unit берём из той же
	// колонки (единый источник — план метрики), чтобы columns[].unit и legacy unit
	// не разъезжались.
	first := metrics[0]
	legacyRows := make([]models.ReportAggregateRow, 0, len(metricRows))
	for _, r := range metricRows {
		legacyRows = append(legacyRows, models.ReportAggregateRow{Label: r.Label, Value: r.Values[first]})
	}

	return &models.ReportResponse{
		Mode:        "aggregate",
		Metric:      first,
		Dimension:   req.Dimension,
		Unit:        columns[0].Unit,
		Rows:        legacyRows,
		Total:       totals[first],
		Columns:     columns,
		MetricRows:  metricRows,
		Totals:      totals,
		FloatTotals: floatTotals,
	}, nil
}

// collectPivotColumns исполняет cross-tab запрос по каждой метрике оси и вписывает
// ячейки в уже упорядоченные строки, возвращая добавочные pivot-колонки и их итоги
// (ключ колонки -> сумма по видимым строкам). Несколько метрик с одной осью дают
// один общий набор pivot-колонок (счётчики складываются — все метрики оси считают
// заявки по тому же выражению).
func (s *statisticsService) collectPivotColumns(ctx context.Context, metrics []string, axis models.ReportPivotInfo, req models.ReportRequest, metricRows []models.ReportMetricRow) ([]models.ReportMetricColumn, map[string]int64, error) {
	var cells []pivotCell
	for _, m := range metrics {
		plan, perr := buildPivotPlan(m, axis.Key, req.Granularity, req.Filters)
		if perr != nil {
			return nil, nil, perr
		}
		mcells, eerr := s.execPivotPlan(ctx, plan)
		if eerr != nil {
			return nil, nil, eerr
		}
		cells = append(cells, mcells...)
	}
	cols, totals := applyPivotCells(metricRows, cells, axis.Label)
	return cols, totals, nil
}

// execPivotPlan исполняет cross-tab запрос: GROUP BY (период-бин, ось pivot) ->
// счётчик. period-подпись совпадает с label обычных period-строк (для слияния).
func (s *statisticsService) execPivotPlan(ctx context.Context, plan *pivotPlan) ([]pivotCell, error) {
	selectStr := fmt.Sprintf("%s AS period, %s AS pivot, %s AS count",
		plan.labelExpr, plan.pivotExpr, plan.aggExpr)
	tx := s.db.WithContext(ctx).Table(plan.table).Select(selectStr)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}

	cells := make([]pivotCell, 0)
	if err := tx.
		Group(plan.periodExpr + ", " + plan.pivotExpr).
		Scan(&cells).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report pivot: %w", err)
	}
	return cells, nil
}

// execWindowTotal считает итог производной метрики (длительность, доля) по всему
// окну фильтров отдельным запросом без разреза. Сложить значения бинов нельзя:
// сумма средних не среднее, перцентили в принципе не складываются, а доля от долей
// не считается — всё это нужно пересчитать по всей выборке. Лимит строк на итог не
// влияет: это агрегат по всем заявкам окна, а не по видимым строкам (у счётчиков
// итог — сумма видимых).
func (s *statisticsService) execWindowTotal(ctx context.Context, req models.ReportRequest, metric string) (int64, error) {
	treq := req
	treq.Metric = metric
	treq.Dimension = dimNone
	plan, err := buildAggregatePlan(treq)
	if err != nil {
		return 0, err
	}
	rows, err := s.execAggregatePlan(ctx, plan)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil // нет заявок с пройденным этапом за период
	}
	return rows[0].Value, nil
}

// execAggregatePlan исполняет один резолвленный план агрегата и возвращает строки
// (подпись разреза + значение метрики). GROUP BY/ORDER применяются только если
// заданы (разрез "none" даёт один итоговый ряд без них).
func (s *statisticsService) execAggregatePlan(ctx context.Context, plan *aggPlan) ([]models.ReportAggregateRow, error) {
	tx := s.db.WithContext(ctx).Table(plan.table).Select(plan.selectStr)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}
	if plan.groupExpr != "" {
		tx = tx.Group(plan.groupExpr)
	}
	if plan.orderStr != "" {
		tx = tx.Order(plan.orderStr)
	}

	rows := make([]models.ReportAggregateRow, 0)
	if err := tx.Limit(plan.limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report: %w", err)
	}
	return rows, nil
}

// RunReportList исполняет list-отчёт конструктора (mode=list): выгрузка строк
// сущности с whitelist-столбцами и фильтрами. План собирается чистой buildListPlan
// из whitelist-схем (report_list_engine.go) — ввод сверяется по ключам и
// подставляется только через плейсхолдеры. Невалидный запрос -> ErrInvalidReportRequest
// (400 в handler). Строки сканируются в []map по алиасам столбцов плана.
func (s *statisticsService) RunReportList(ctx context.Context, req models.ReportRequest) (*models.ReportListResponse, error) {
	// Персональные данные не давших согласия скрыты и в отчётах: колонка принимающего
	// собирает ФИО с телефоном одной строкой, и подменить её после выборки нечем.
	plan, err := buildListPlan(req, pdConsentMaskingActive(ctx, s.db))
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Table(plan.table).Select(plan.selectStr, plan.selectArgs...)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}

	rows := make([]map[string]any, 0)
	if err := tx.
		Order(plan.orderStr).
		Limit(plan.limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report list: %w", err)
	}

	return &models.ReportListResponse{
		Mode:    "list",
		Entity:  req.Entity,
		Columns: plan.columns,
		Rows:    rows,
		Total:   len(rows),
	}, nil
}
