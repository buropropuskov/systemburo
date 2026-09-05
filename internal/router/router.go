package router

import (
	"net/http"

	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// applicationsBodyLimit - потолок тела запроса на группу /applications (blank-import,
// срез A2A3). Раньше единственным пределом был client_max_body_size 50M в nginx (не в
// этом репозитории) - без него прямой запрос к go-backend в обход nginx (локальная
// разработка, другой reverse-proxy) читал тело неограниченно. Значение зеркалит nginx,
// а не ужесточает его: то, что проходило через nginx, продолжает проходить и здесь.
const applicationsBodyLimit = "50M"

// Dependencies - все хендлеры/сервисы/middleware, нужные для регистрации маршрутов.
// Использование именованных полей вместо длинного списка позиционных параметров
// решает проблему "30+ параметров в Setup": IDE подсказывает имена, диффы при
// добавлении handler-а становятся одной строкой, тесты могут не заполнять
// неиспользуемые поля.
type Dependencies struct {
	// Handlers
	Auth                *handlers.AuthHandler
	UserTypes           *handlers.UserTypesHandler
	Attachments         *handlers.AttachmentHandler
	ManualAttach        *handlers.ManualAttachHandler
	LPF                 *handlers.LicensePlateFormatHandler
	Citizenship         *handlers.CitizenshipHandler
	Organization        *handlers.OrganizationHandler
	Company             *handlers.CompanyHandler
	Users               *handlers.UsersHandler
	Onboarding          *handlers.OnboardingHandler
	Theme               *handlers.ThemeHandler
	UnloadPlace         *handlers.UnloadPlaceHandler
	Cars                *handlers.CarHandler
	Employees           *handlers.EmployeeHandler
	SystemTable         *handlers.SystemTableHandler
	TableSnapshot       *handlers.TableSnapshotHandler
	PassReport          *handlers.PassReportHandler
	UniqueCar           *handlers.UniqueCarHandler
	UniqueEmployee      *handlers.UniqueEmployeeHandler
	Feedback            *handlers.FeedbackHandler
	Application         *handlers.ApplicationHandler
	ApplicationFiles    *handlers.ApplicationFileHandler
	Approver            *handlers.ApproverHandler
	Permissions         *handlers.PermissionHandler
	PermGroups          *handlers.PermissionGroupHandler
	Roles               *handlers.RoleHandler
	AccessDenials       *handlers.AccessDenialHandler
	PDAudit             *handlers.PDAuditHandler
	UserBan             *handlers.UserBanHandler
	Consent             *handlers.ConsentHandler
	Settings            *handlers.SettingsHandler
	News                *handlers.NewsHandler
	Notifications       *handlers.NotificationHandler
	Push                *handlers.PushHandler
	RequestLogs         *handlers.RequestLogsHandler
	EmployeesHistory    *handlers.EmployeesHistoryHandler
	BugReport           *handlers.BugReportHandler
	Maintenance         *handlers.MaintenanceHandler
	Marks               *handlers.MarkHandler
	VehicleBlacklist    *handlers.VehicleBlacklistHandler
	PersonBlacklist     *handlers.PersonBlacklistHandler
	AttachmentTemplates *handlers.AttachmentTemplateHandler
	AttachmentBlanks    *handlers.AttachmentBlankHandler
	AttachmentImport    *handlers.AttachmentImportHandler
	Trash               *handlers.TrashHandler
	DocumentGroups      *handlers.DocumentGroupHandler
	Documents           *handlers.DocumentHandler
	Guide               *handlers.GuideHandler
	Statistics          *handlers.StatisticsHandler
	Reminder            *handlers.ReminderHandler
	Bureau              *handlers.BureauHandler
	WorkModes           *handlers.WorkModesHandler
	Audit               *handlers.AuditHandler
	AuthEvents          *handlers.AuthEventHandler
	Events              *handlers.EventsHandler
	Search              *handlers.SearchHandler
	BlankArchive        *handlers.BlankArchiveHandler
	BlankArchiveStats   *handlers.BlankArchiveStatsHandler
	ArchiveDownload     *handlers.ArchiveDownloadHandler
	Impersonation       *handlers.ImpersonationHandler

	// Services (для middleware и audit)
	PermResolver *services.PermissionResolver
	DenialLog    *services.AccessDenialService

	// Middleware - все опциональны (nil в тестах допустим)
	MaintenanceBlock echo.MiddlewareFunc
	BanCheck         echo.MiddlewareFunc
	LoginLimiter     echo.MiddlewareFunc
	LastSeen         echo.MiddlewareFunc
	// ImportListLimiter - свой rate limit на POST /attachments/:id/import-list
	// (blank-import, C1C2), сверх общего RateLimit в main.go. nil в тестах - разбор
	// .xlsx на несколько подтестов подряд не должен упираться в лимит.
	ImportListLimiter echo.MiddlewareFunc
	// SelfPasswordLimiter - rate limit на PUT /users/me/password. Форма принимает
	// текущий пароль, то есть годится для подбора не хуже страницы входа, а лестница
	// блокировки входа её не прикрывает. nil в тестах - там подряд идут и удачные,
	// и заведомо неудачные попытки одной учёткой.
	SelfPasswordLimiter echo.MiddlewareFunc
	// ConsentGate - PDConsentGate: закрывает API до согласия на обработку ПД
	// (#1567). nil по умолчанию, в том числе в тестах: иначе каждый тест, где
	// согласия нет, начал бы получать 403. Тесты самого гейта поднимают
	// приложение через SetupTestAppWithConsentGate.
	ConsentGate echo.MiddlewareFunc
	// MustChangePassword - mw.MustChangePassword: закрывает protected-API
	// пользователю, обязанному задать свой пароль вместо присланного письмом
	// (#1911). nil по умолчанию и в тестах - по той же причине, что и ConsentGate.
	MustChangePassword echo.MiddlewareFunc
	// TableReportGate - RequireTableVerb(..., "report"): гейт отчётов по проходам
	// правом table.<name>.report. НЕ опционален для роутов pass-report (main и
	// testutil обязаны заполнять) - без гейта отчёт открылся бы любому залогиненному.
	TableReportGate echo.MiddlewareFunc
	// TableVersionsGate/TableTrashGate - RequireTableVerb(..., "versions"/"trash"):
	// снимки версий и корзина таблицы правом table.<name>.versions/.trash. Раньше
	// эти под-роуты гейтил только фронт - любой залогиненный мог снять снимок или
	// чистить корзину любой таблицы. main и testutil обязаны заполнять.
	TableVersionsGate echo.MiddlewareFunc
	TableTrashGate    echo.MiddlewareFunc
	// TablePassGate - RequireTablePassVerb: отметка прохода на КПП правом
	// table.<name>.entry/.exit (направление и таблица берутся из тела). main и
	// testutil обязаны заполнять - без гейта любой залогиненный мог бы отметить
	// проезд/проход любой машины или человека.
	TablePassGate echo.MiddlewareFunc

	// Misc
	JWTSecret []byte
	// JWTRefreshSecret нужен раздаче загруженных файлов: она пускает по cookie
	// продления сеанса, потому что тег <img> заголовок Authorization не шлёт.
	JWTRefreshSecret []byte
	UploadPath       string
}

// Setup регистрирует все маршруты. См. Dependencies для описания полей.
func Setup(e *echo.Echo, d Dependencies) {
	auth := d.Auth
	userTypes := d.UserTypes
	attachments := d.Attachments
	lpf := d.LPF
	cs := d.Citizenship
	org := d.Organization
	comp := d.Company
	users := d.Users
	onboarding := d.Onboarding
	theme := d.Theme
	up := d.UnloadPlace
	cars := d.Cars
	employees := d.Employees
	st := d.SystemTable
	tsnap := d.TableSnapshot
	passReport := d.PassReport
	uc := d.UniqueCar
	ue := d.UniqueEmployee
	fb := d.Feedback
	app := d.Application
	approvers := d.Approver
	permissions := d.Permissions
	permGroups := d.PermGroups
	roles := d.Roles
	accessDenials := d.AccessDenials
	userBan := d.UserBan
	consent := d.Consent
	settings := d.Settings
	news := d.News
	notifications := d.Notifications
	push := d.Push
	requestLogs := d.RequestLogs
	employeesHistory := d.EmployeesHistory
	bugReport := d.BugReport
	maintenance := d.Maintenance
	marks := d.Marks
	vehicleBlacklist := d.VehicleBlacklist
	personBlacklist := d.PersonBlacklist
	attachmentTemplates := d.AttachmentTemplates
	attachmentBlanks := d.AttachmentBlanks
	attachmentImport := d.AttachmentImport
	trash := d.Trash
	docGroups := d.DocumentGroups
	docs := d.Documents
	guide := d.Guide
	statistics := d.Statistics
	reminder := d.Reminder
	bureau := d.Bureau
	audit := d.Audit
	authEvents := d.AuthEvents
	events := d.Events
	archiveDownload := d.ArchiveDownload
	permResolver := d.PermResolver
	denialLog := d.DenialLog
	// requireAdmin — гейт admin-страниц по page.admin (super/admin проходят,
	// обычные — по гранту). Заменяет легаси type-code проверки в сервисах (Ф5).
	requireAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdmin)
	// Справочники раздела /admin/* целиком. Ключ тот же, что у фронтовых страниц:
	// иначе носитель права видит раздел и ловит 403.
	//
	// page.admin этих маршрутов больше не открывает, и наследования «page.admin даёт
	// все page.admin.*» здесь намеренно нет (#1982): вместе со справочниками оно
	// протащило бы журнал обращений к персональным данным, раздел пользователей и
	// настройки - то есть расширило бы права под видом переезда. Действующие
	// администраторы проходят по признаку is_admin (adminAll в резолвере), а не по
	// ключу, поэтому переезд их не задевает.
	requireDirectories := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminDirectories)
	// Конструктор системных таблиц: создание/изменение/удаление структуры и настроек
	// таблиц КПП. Ключ тот же, что у фронтовой страницы /table-constructor.
	requireTablesCtor := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminTablesCtor)
	// Массовый ввод из бланка (blank-import, C1C2): скачивание пустого бланка и приём
	// заполненного гейтятся ОДНИМ правом - иначе пользователь скачивает бланк, заполняет
	// его и упирается в 403 на загрузке (класс "видно, но 403").
	requireImportList := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionImportList)
	// importListLimiter - свой rate limit сверх общего (RateLimit в main.go): приём файла
	// разбирает .xlsx на до 2000 строк, дороже обычной ручки. Опционален и nil в тестах
	// (тот же приём, что у LoginLimiter ниже) - иначе один тестовый файл с полудюжиной
	// подтестов на одну ручку упёрся бы в лимит посреди прогона.
	importListLimiter := d.ImportListLimiter
	maintenanceBlock := d.MaintenanceBlock
	banCheck := d.BanCheck
	consentGate := d.ConsentGate
	mustChangePassword := d.MustChangePassword
	loginLimiter := d.LoginLimiter
	lastSeen := d.LastSeen
	jwtSecret := d.JWTSecret
	// Health check — вне /api, для мониторинга и readiness-проб.
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Все API-роуты под префиксом /api — разделяет API и SPA-роуты (/news, /center
	// и т.д. в Vue router). Nginx проксирует /api/ на backend, остальное — на frontend.
	api := e.Group("/api")

	// Статика загруженных файлов (фото мест разгрузки и системных таблиц).
	// Под /api, чтобы прод-nginx (проксирует /api на backend) раздавал файлы без
	// отдельного location и правок nginx. Доступ закрыт mw.FileAccess: тег <img>
	// не отправляет Authorization, поэтому пропуском служит cookie продления
	// сеанса (#2133). До этого каталог раздавался всем, кто знает адрес файла.
	// Роут регистрируется вручную, а не через api.Group("/uploads").Static: группа
	// echo заводит себе fallback RouteNotFound("/*"), он оказывается точнее
	// статического "/uploads*" и перехватывает запросы файлов на 404.
	if d.UploadPath != "" {
		api.Add(
			http.MethodGet,
			"/uploads*",
			echo.StaticDirectoryHandler(echo.MustSubFS(e.Filesystem, d.UploadPath), false),
			mw.FileAccess(d.JWTSecret, d.JWTRefreshSecret),
		)
	}

	// Public routes. /login опционально защищён per-IP rate limiter-ом.
	loginHandlers := []echo.MiddlewareFunc{}
	if loginLimiter != nil {
		loginHandlers = append(loginHandlers, loginLimiter)
	}
	api.POST("/login", auth.Login, loginHandlers...)
	api.POST("/refresh-token", auth.RefreshToken)
	api.GET("/user-types", auth.GetUserTypes)
	// Публичный статус техработ - без JWT, чтобы страница /maintenance и форма /login
	// могли его опросить.
	api.GET("/settings/maintenance", maintenance.GetPublicStatus)
	// Публичные контакты Бюро пропусков - нужны на логине и в плашке блокировки.
	api.GET("/settings/contacts", settings.GetPublicContacts)

	// Real-time SSE-поток (#840). Публичный роут намеренно: EventSource не шлёт
	// заголовок Authorization. Подключение авторизуется одноразовым билетом из query
	// (выдаётся защищённым POST /events/ticket ниже), а не access-токеном - токен в
	// query утёк бы в журналы. Consume билета внутри Stream.
	if events != nil {
		api.GET("/events", events.Stream)
	}

	// Потоковый ZIP файлового архива за период (#1615, B3). Публичный роут намеренно,
	// как и /events: прямая ссылка/клик по кнопке скачивания не может нести заголовок
	// Authorization. Авторизация - одноразовым билетом из query (выдаётся защищённым
	// POST /file-archive/download-ticket ниже), сам билет несёт и право, и границы
	// периода - consume внутри Download.
	if archiveDownload != nil {
		api.GET("/file-archive/download", archiveDownload.Download)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(mw.JWTAuth(jwtSecret))
	// Maintenance block - после JWTAuth (нужен type_id в context). Super-admin
	// проходит, остальным 503. Передаём nil в тестах чтобы не блочить.
	if maintenanceBlock != nil {
		protected.Use(maintenanceBlock)
	}
	// BanCheck - после JWTAuth (нужен user_id). Забаненный получает 403 даже с
	// валидным access-токеном до истечения exp. Кэш TTL 30s. Инвалидируется при
	// Ban/Unban из UserBanService. nil в тестах чтобы не требовать service.
	if banCheck != nil {
		protected.Use(banCheck)
	}
	// ConsentGate - после BanCheck: забаненный не может дать согласие (проверка
	// бана режет POST), поэтому ему показываем блокировку, а не требование
	// согласия. Супер-админ и роуты из PDConsentWhitelist проходят. nil в тестах.
	if consentGate != nil {
		protected.Use(consentGate)
	}
	// MustChangePassword - после гейта согласия: согласие спрашивается раньше всего
	// остального, а сменить пароль до него всё равно не дают (смены нет в белом
	// списке согласия). Пропускает только MustChangePasswordWhitelist, остальное -
	// 403 с кодом PASSWORD_CHANGE_REQUIRED. nil в тестах: иначе каждый тест, где
	// флаг поднят сидом, начал бы получать 403 вместо своего ответа. Тесты самого
	// гейта поднимают приложение через SetupTestAppWithPasswordGate.
	if mustChangePassword != nil {
		protected.Use(mustChangePassword)
	}
	// LastSeen - после JWTAuth (нужен user_id). Обновляет users.last_seen для
	// учёта онлайна (#632), с in-memory троттлингом и асинхронной записью.
	// nil в тестах, где БД-запись не нужна.
	if lastSeen != nil {
		protected.Use(lastSeen)
	}
	// Гейт опасных действий в режиме «войти как пользователь» (#1912). Без условия
	// и без зависимостей: список закрытого статичен, а необязательный гейт означал бы,
	// что в тестах смена пароля из чужой учётной записи проходит.
	protected.Use(mw.DenyUnderImpersonation())

	protected.POST("/logout", auth.Logout)
	protected.POST("/logout-all", auth.LogoutAll)
	protected.GET("/user-data", auth.GetUserData)
	protected.GET("/users/me", auth.GetCurrentUser)
	protected.GET("/users/current", auth.GetCurrentUser)

	// Онбординг-тур (#657) - self-service статус: любой авторизованный читает и
	// помечает прохождение ДЛЯ СЕБЯ (userID из JWT). Не admin-only.
	protected.GET("/onboarding", onboarding.GetStatus)
	protected.POST("/onboarding/complete", onboarding.MarkComplete)

	// Тема оформления (#1415) - тоже self-service: читаем и пишем СВОЮ тему,
	// userID из JWT. Права не требуются, оформление доступно любому.
	protected.GET("/users/me/theme", theme.GetTheme)
	protected.PUT("/users/me/theme", theme.SetTheme)

	// Смена СВОЕГО пароля (#1915). До этого единственным путём смены был
	// PUT /users/:username/password под page.admin.users - работник не мог сменить
	// свой пароль вообще, только через бюро. Права не требуются, личность
	// подтверждается текущим паролем внутри сервиса.
	if users != nil {
		selfPasswordHandlers := []echo.MiddlewareFunc{}
		if d.SelfPasswordLimiter != nil {
			selfPasswordHandlers = append(selfPasswordHandlers, d.SelfPasswordLimiter)
		}
		protected.PUT("/users/me/password", users.ChangeOwnPassword, selfPasswordHandlers...)
	}

	// Сквозной поиск по разделам. Гейт эндпоинта -- только авторизация, и это
	// намеренно: раздел, на который нет права, отсекается отбором провайдеров, а
	// строки внутри раздела сужает сам провайдер. Права на сам поиск не существует --
	// он не даёт доступа ни к чему, чего нет в листингах.
	if d.Search != nil {
		protected.GET("/search", d.Search.Search)
	}

	// Выдача одноразового билета для SSE-потока (#840). Защищён JWTAuth+banCheck:
	// забаненный/разлогиненный билет не получит, значит и поток не переоткроет.
	if events != nil {
		protected.POST("/events/ticket", events.IssueTicket)
	}

	// Шаблоны вложений (unique_attachments). Изменяющие операции и админ-выборки -
	// page.admin.directories, тем же правом фронт открывает /admin/attachment-types.
	// Активный список и карточка типа остаются всем: их читает форма подачи заявки.
	att := protected.Group("/attachments")
	att.GET("", attachments.GetActive)
	att.GET("/all", attachments.GetAll, requireDirectories)
	att.POST("", attachments.Create, requireDirectories)
	att.PUT("/:id", attachments.Update, requireDirectories)
	att.DELETE("/:id", attachments.Delete, requireDirectories)
	att.PUT("/:id/restore", attachments.Restore, requireDirectories)
	att.GET("/:id/history", attachments.GetHistory, requireDirectories)
	att.GET("/:id", attachments.GetByID)
	// Пустой бланк для заполнения списка участников и приём заполненного - под одним
	// правом action.import.list (blank-import, C1C2): скачивание без права загрузки
	// оставило бы пользователя с заполненным файлом, который загрузить некуда.
	att.GET("/:id/blank-template", attachmentBlanks.DownloadTemplate, requireImportList)
	importListHandlers := []echo.MiddlewareFunc{requireImportList}
	if importListLimiter != nil {
		importListHandlers = append(importListHandlers, importListLimiter)
	}
	att.POST("/:id/import-list", attachmentImport.ImportList, importListHandlers...)
	// Привязка ручного вложения-сироты к заявке (#1049 режим-2): только super/admin.
	// Внимание: :id здесь = экземпляр attachments.id (ручная сирота), а НЕ unique_attachment
	// (шаблон), как в CRUD-маршрутах группы выше. Разные таблицы под одним префиксом.
	att.POST("/:id/attach-to-application", d.ManualAttach.AttachToApplication, requireAdmin)

	// Единый журнал аудита (#870): сводный + история одной сущности через фильтры
	// entity_type/entity_id. Admin-only - кросс-сущностный аудит чувствителен.
	protected.GET("/audit", audit.GetAuditLog, requireAdmin)

	// Управление типами пользователей - справочник раздела (page.admin.directories),
	// тем же правом фронт открывает /admin/user-types.
	utm := protected.Group("/user-types-management", requireDirectories)
	utm.GET("", userTypes.GetAll)
	utm.POST("", userTypes.Create)
	utm.PUT("/:id", userTypes.Update)
	utm.DELETE("/:id", userTypes.Delete)
	utm.GET("/:id/history", userTypes.GetHistory)
	utm.GET("/:id/blocking-users", userTypes.GetBlockingUsers)
	utm.POST("/:id/reassign-users", userTypes.ReassignUsers)

	// Гражданства. Список и история — для всех авторизованных (дропдаун гражданств
	// в форме заявки); изменяющие операции — админ справочников (page.admin.directories).
	csg := protected.Group("/citizenships")
	csg.GET("", cs.GetAll)
	csg.POST("", cs.Create, requireDirectories)
	csg.PUT("/:id", cs.Update, requireDirectories)
	csg.DELETE("/:id", cs.Delete, requireDirectories)
	csg.POST("/:id/restore", cs.Restore, requireDirectories)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	csg.POST("/bulk/archive", cs.BulkArchive, requireDirectories)
	csg.POST("/bulk/restore", cs.BulkRestore, requireDirectories)
	csg.GET("/:id/history", cs.GetHistory)
	csg.POST("/clear-default", cs.ClearDefaults, requireDirectories)

	// Форматы номерных знаков
	lpfGroup := protected.Group("/license-plate-formats")
	// Чтение форматов номеров нужно форме заявки; изменения - только админ справочников.
	lpfGroup.GET("", lpf.GetAll)
	lpfGroup.POST("", lpf.Create, requireDirectories)
	lpfGroup.PUT("/:id", lpf.Update, requireDirectories)
	lpfGroup.DELETE("/:id", lpf.Delete, requireDirectories)
	lpfGroup.POST("/:id/restore", lpf.Restore, requireDirectories)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	lpfGroup.POST("/bulk/archive", lpf.BulkArchive, requireDirectories)
	lpfGroup.POST("/bulk/restore", lpf.BulkRestore, requireDirectories)
	lpfGroup.GET("/:id/history", lpf.GetHistory)

	// Марки автомобилей (#185) - справочник с историчностью.
	marksGroup := protected.Group("/marks")
	// Чтение справочника марок нужно форме заявки (дропдаун) - оставляем открытым;
	// изменения делает только админ из раздела справочников (page.admin.directories).
	marksGroup.GET("", marks.GetAll)
	marksGroup.POST("", marks.Create, requireDirectories)
	marksGroup.PUT("/:id", marks.Update, requireDirectories)
	marksGroup.POST("/:id/archive", marks.Archive, requireDirectories)
	marksGroup.POST("/:id/restore", marks.Restore, requireDirectories)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	marksGroup.POST("/bulk/archive", marks.BulkArchive, requireDirectories)
	marksGroup.POST("/bulk/restore", marks.BulkRestore, requireDirectories)
	marksGroup.GET("/:id/history", marks.GetHistory)

	// Чёрный список машин (#443). POST/DELETE/restore защищены page.admin.blacklist.
	// GET списка/истории и /check открыты любым авторизованным: фронт рендерит
	// страницу даже без права, а /check нужен всем при подаче заявки.
	requireBlacklist := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageBlacklist)
	vblGroup := protected.Group("/vehicle-blacklist")
	// Выгрузка списка и истории ЧС - только под правом (это ПД). Точечную проверку
	// /check оставляем открытой: форме заявки нужно узнать, не в ЧС ли конкретная
	// машина, но без доступа ко всему списку. Пометку реестра теперь даёт сервер
	// полем is_blacklisted (#1528/#1530), список ЧС в браузер больше не грузится.
	vblGroup.GET("", vehicleBlacklist.GetAll, requireBlacklist)
	vblGroup.GET("/check", vehicleBlacklist.Check)
	// Предпросмотр последствий внесения - под правом: в ответе ФИО, номера заявок и посты.
	vblGroup.GET("/impact", vehicleBlacklist.Impact, requireBlacklist)
	vblGroup.GET("/history", vehicleBlacklist.GetAllHistory, requireBlacklist)
	vblGroup.GET("/:id/history", vehicleBlacklist.GetHistory, requireBlacklist)
	vblGroup.POST("", vehicleBlacklist.Create, requireBlacklist)
	vblGroup.PUT("/:id", vehicleBlacklist.Update, requireBlacklist)
	vblGroup.DELETE("/:id", vehicleBlacklist.Delete, requireBlacklist)
	vblGroup.DELETE("/:id/purge", vehicleBlacklist.Purge, requireBlacklist)
	vblGroup.POST("/:id/restore", vehicleBlacklist.Restore, requireBlacklist)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	vblGroup.POST("/bulk/archive", vehicleBlacklist.BulkArchive, requireBlacklist)
	vblGroup.POST("/bulk/restore", vehicleBlacklist.BulkRestore, requireBlacklist)

	// Чёрный список людей (#443). Та же permission page.admin.blacklist (одна страница).
	pblGroup := protected.Group("/person-blacklist")
	// Список и история ЧС людей (ФИО + причины, ПД) - только под правом. /check
	// остаётся открытым: форма проверяет конкретного человека, не выгружая список.
	pblGroup.GET("", personBlacklist.GetAll, requireBlacklist)
	pblGroup.GET("/check", personBlacklist.Check)
	pblGroup.GET("/impact", personBlacklist.Impact, requireBlacklist)
	pblGroup.GET("/history", personBlacklist.GetAllHistory, requireBlacklist)
	pblGroup.GET("/:id/history", personBlacklist.GetHistory, requireBlacklist)
	pblGroup.POST("", personBlacklist.Create, requireBlacklist)
	pblGroup.PUT("/:id", personBlacklist.Update, requireBlacklist)
	pblGroup.DELETE("/:id", personBlacklist.Delete, requireBlacklist)
	pblGroup.DELETE("/:id/purge", personBlacklist.Purge, requireBlacklist)
	pblGroup.POST("/:id/restore", personBlacklist.Restore, requireBlacklist)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	pblGroup.POST("/bulk/archive", personBlacklist.BulkArchive, requireBlacklist)
	pblGroup.POST("/bulk/restore", personBlacklist.BulkRestore, requireBlacklist)

	// Attachment Excel-templates (#183) - вложенные ручки под /attachments/:id/...
	// Настройка бланка целиком админская (page.admin.directories): файл шаблона и привязки
	// ячеек - инструмент справочника, а не форма подачи. Исключение ниже - GET field-config.
	attRoot := protected.Group("/attachments", requireDirectories)
	attRoot.GET("/:id/template", attachmentTemplates.Get)
	attRoot.GET("/:id/templates", attachmentTemplates.ListTemplates)
	attRoot.POST("/:id/template", attachmentTemplates.Upload)
	attRoot.GET("/:id/template/file", attachmentTemplates.DownloadFile)
	attRoot.GET("/:id/template/:tid/file", attachmentTemplates.DownloadFileByID)
	attRoot.PUT("/:id/template/mappings", attachmentTemplates.UpdateMappings)
	// Перенос привязок с другого шаблона: статический /template-sources стоит рядом с
	// /:id/..., у echo статический сегмент приоритетнее параметра.
	attRoot.GET("/template-sources", attachmentTemplates.ListTemplateSources)
	attRoot.POST("/:id/template/copy-mappings", attachmentTemplates.CopyMappings)
	attRoot.PUT("/:id/template/params", attachmentTemplates.UpdateParams)
	attRoot.PUT("/:id/template/:tid/activate", attachmentTemplates.SetActive)
	attRoot.PUT("/:id/template/deactivate", attachmentTemplates.DeactivateAll)
	attRoot.DELETE("/:id/template", attachmentTemplates.Delete)
	attRoot.DELETE("/:id/template/:tid", attachmentTemplates.DeleteByID)
	attRoot.GET("/:id/template-fields", attachmentTemplates.GetFields)
	attRoot.GET("/:id/custom-fields", attachmentTemplates.ListCustomFields)
	attRoot.POST("/:id/custom-fields", attachmentTemplates.CreateCustomField)
	attRoot.PUT("/custom-fields/:fid", attachmentTemplates.UpdateCustomField)
	attRoot.DELETE("/custom-fields/:fid", attachmentTemplates.DeleteCustomField)
	// Настройка полей вложения (feedback-0608-H / #529): видимость/обязательность
	// базовых полей реестра + кастомные поля одним ответом.
	attRoot.PUT("/:id/field-config", attachmentTemplates.SaveFieldConfig)
	// Чтение настройки полей - без права: по ней форма подачи решает, какие поля
	// показывать и требовать (CreateApplication.loadFieldConfig). Регистрируем в группе
	// без гейта, иначе заявитель не соберёт вложение.
	att.GET("/:id/field-config", attachmentTemplates.GetFieldConfig)

	// Организации. Изменяющие операции, история и состав - админ справочников
	// (page.admin.directories, тем же правом фронт открывает /admin/organizations).
	// Открытым остаётся то, без чего не собрать заявку: наименования (GetAll),
	// ответственные (/:id/users), таблицы и места разгрузки - их читают форма подачи
	// и VehicleForm.
	// Подсказки при ручном вводе наименования в заявке (#1437). Гейт - то же право,
	// что разблокирует ручной ввод: без него заявка идёт от своей организации, и
	// подсказывать нечего. Статический сегмент suggest в Echo приоритетнее :id.
	requireOrgOverride := mw.RequirePermissionV2(permResolver, denialLog, services.KeyApplicationOrganizationOverride)

	// Разбор записей «на проверке», заведённых из заявки (#1437). Право отдельное от
	// администрирования справочника: разбирает принимающий, а не только админ.
	requireOrgModerate := mw.RequirePermissionV2(permResolver, denialLog, services.KeyApplicationOrganizationModerate)

	orgg := protected.Group("/organizations")
	orgg.GET("", org.GetAll)
	orgg.GET("/suggest", org.Suggest, requireOrgOverride)
	orgg.POST("/:id/moderation/approve", org.ApproveModeration, requireOrgModerate)
	orgg.PATCH("/:id/moderation/rename", org.RenameModeration, requireOrgModerate)
	orgg.POST("/:id/moderation/merge", org.MergeModeration, requireOrgModerate)
	orgg.POST("", org.Create, requireDirectories)
	orgg.PUT("/:id", org.Update, requireDirectories)
	orgg.DELETE("/:id", org.Delete, requireDirectories)
	orgg.POST("/:id/restore", org.Restore, requireDirectories)
	orgg.GET("/:id/history", org.GetHistory, requireDirectories)
	// Списки для таблицы управления справочником: число работников, тип, архивные
	// записи и статус разбора. Строки этой таблицы больше ничем не отдаются, а
	// открытый /organizations даёт только наименования активных, так что гейт тут
	// закрывает реальную разницу, а не повторяет соседа.
	orgg.GET("/with-users", org.GetWithUsers, requireDirectories)
	orgg.GET("/with-users-extended", org.GetWithUsersExtended, requireDirectories)
	orgg.GET("/:id/users", org.GetOrganizationUsers)
	// Состав ответственных - запись, а не чтение: метод стирает organization_users
	// и пересобирает набор из тела запроса, включая флаги is_primary и
	// required_approval. Второй делает человека согласующим (IsReviewer в
	// approver_service проверяет ровно его) и тянет его в ответственные по заявкам
	// организации, так что без гейта любой работник вписывал себя сам. Право то же,
	// что у соседей по составу - reassign-users и bulk/users.
	orgg.PUT("/:id/users", org.UpdateOrganizationUsers, requireDirectories)
	// Участники - ФИО, должности и логины работников организации. Они же блокируют
	// архивацию: набор active-only, и delete-флоу спрашивает этот же маршрут.
	orgg.GET("/:id/members", org.GetMembers, requireDirectories)
	// Перенос всех участников в другую организацию - гейт как у Delete.
	orgg.POST("/:id/reassign-users", org.ReassignUsers, requireDirectories)
	orgg.GET("/:id/tables", org.GetOrganizationTables)
	orgg.PUT("/:id/tables", org.UpdateOrganizationTables, requireDirectories)
	orgg.GET("/:id/unload-places", org.GetOrganizationUnloadPlaces)
	orgg.PUT("/:id/unload-places", org.UpdateOrganizationUnloadPlaces, requireDirectories)
	// Групповые операции (bulk). Статический сегмент bulk имеет приоритет над
	// param :id в Echo, поэтому /bulk/restore не конфликтует с /:id/restore.
	orgg.POST("/bulk/type", org.BulkUpdateType, requireDirectories)
	orgg.POST("/bulk/unload-places", org.BulkAssignUnloadPlaces, requireDirectories)
	orgg.POST("/bulk/tables", org.BulkAssignTables, requireDirectories)
	orgg.POST("/bulk/users", org.BulkAssignUsers, requireDirectories)
	orgg.POST("/bulk/archive", org.BulkArchive, requireDirectories)
	orgg.POST("/bulk/restore", org.BulkRestore, requireDirectories)
	protected.GET("/get-organization", org.GetMyOrganization)

	// Компании. Зеркало organizations: изменяющие операции, история и состав - админ
	// справочников (page.admin.directories, тем же правом фронт открывает
	// /admin/companies); открыт тот же набор чтений, что нужен форме заявки.
	cg := protected.Group("/companies")
	cg.GET("", comp.GetAll)
	cg.GET("/suggest", comp.Suggest, requireOrgOverride)
	cg.POST("/:id/moderation/approve", comp.ApproveModeration, requireOrgModerate)
	cg.PATCH("/:id/moderation/rename", comp.RenameModeration, requireOrgModerate)
	cg.POST("/:id/moderation/merge", comp.MergeModeration, requireOrgModerate)
	cg.POST("", comp.Create, requireDirectories)
	cg.PUT("/:id", comp.Update, requireDirectories)
	cg.DELETE("/:id", comp.Delete, requireDirectories)
	cg.POST("/:id/restore", comp.Restore, requireDirectories)
	cg.GET("/:id/history", comp.GetHistory, requireDirectories)
	cg.GET("/with-users", comp.GetWithUsers, requireDirectories)
	cg.GET("/with-users-extended", comp.GetWithUsersExtended, requireDirectories)
	cg.GET("/:id/users", comp.GetUsers)
	// Зеркало organizations: запись состава с теми же флагами и тем же следствием
	// для согласования, гейт держим одинаковым.
	cg.PUT("/:id/users", comp.UpdateUsers, requireDirectories)
	cg.GET("/:id/members", comp.GetMembers, requireDirectories)
	// Перенос всех участников в другую компанию - гейт как у Delete.
	cg.POST("/:id/reassign-users", comp.ReassignUsers, requireDirectories)
	cg.GET("/:id/tables", comp.GetTables)
	cg.PUT("/:id/tables", comp.UpdateTables, requireDirectories)
	cg.GET("/:id/unload-places", comp.GetUnloadPlaces)
	cg.PUT("/:id/unload-places", comp.UpdateUnloadPlaces, requireDirectories)
	// Групповые операции (bulk). Статический сегмент bulk имеет приоритет над
	// param :id в Echo, поэтому /bulk/restore не конфликтует с /:id/restore.
	cg.POST("/bulk/type", comp.BulkUpdateType, requireDirectories)
	cg.POST("/bulk/unload-places", comp.BulkAssignUnloadPlaces, requireDirectories)
	cg.POST("/bulk/tables", comp.BulkAssignTables, requireDirectories)
	cg.POST("/bulk/users", comp.BulkAssignUsers, requireDirectories)
	cg.POST("/bulk/archive", comp.BulkArchive, requireDirectories)
	cg.POST("/bulk/restore", comp.BulkRestore, requireDirectories)

	// Места разгрузки
	upg := protected.Group("/unload-places")
	// Чтение мест разгрузки нужно форме заявки (дропдаун, слоты, окна) - открыто;
	// любое изменение справочника - только админ (page.admin.directories).
	upg.GET("", up.GetAll)
	upg.POST("", up.Create, requireDirectories)
	upg.GET("/:id", up.GetByID)
	upg.PUT("/:id", up.Update, requireDirectories)
	upg.DELETE("/:id", up.Delete, requireDirectories)
	upg.POST("/:id/restore", up.Restore, requireDirectories)
	upg.GET("/:id/usage", up.GetUsage)
	upg.POST("/:id/detach-all", up.DetachAll, requireDirectories)
	upg.DELETE("/:id/organizations/:org_id", up.DetachOrganization, requireDirectories)
	upg.DELETE("/:id/companies/:company_id", up.DetachCompany, requireDirectories)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	upg.POST("/bulk/archive", up.BulkArchive, requireDirectories)
	upg.POST("/bulk/restore", up.BulkRestore, requireDirectories)
	upg.GET("/:id/history", up.GetHistory)
	upg.GET("/:id/time-slots", up.GetTimeSlots)
	upg.POST("/:id/time-slots", up.AddTimeSlot, requireDirectories)
	upg.PUT("/:place_id/time-slots/:slot_id", up.UpdateTimeSlot, requireDirectories)
	upg.DELETE("/:place_id/time-slots/:slot_id", up.DeleteTimeSlot, requireDirectories)
	upg.GET("/:id/warning-windows", up.GetWarningWindows)
	upg.POST("/:id/warning-windows", up.AddWarningWindow, requireDirectories)
	upg.PUT("/:place_id/warning-windows/:window_id", up.UpdateWarningWindow, requireDirectories)
	upg.DELETE("/:place_id/warning-windows/:window_id", up.DeleteWarningWindow, requireDirectories)
	upg.POST("/:id/photos", up.UploadPhoto, requireDirectories)
	upg.DELETE("/:place_id/photos/:photo_id", up.DeletePhoto, requireDirectories)
	upg.POST("/:place_id/photos/:photo_id/main", up.SetMainPhoto, requireDirectories)

	// Расписание работы Бюро (single-owner). Чтение -- любой авторизованный
	// (нужно модалке режимов работы), изменения -- админ (раздел «Информация Бюро»).
	bureauGroup := protected.Group("/bureau")
	bureauGroup.GET("/time-slots", bureau.GetTimeSlots)
	bureauGroup.POST("/time-slots", bureau.AddTimeSlot, requireAdmin)
	bureauGroup.PUT("/time-slots/:slot_id", bureau.UpdateTimeSlot, requireAdmin)
	bureauGroup.DELETE("/time-slots/:slot_id", bureau.DeleteTimeSlot, requireAdmin)

	// Режимы работы -- read-only агрегатор расписаний Бюро, мест разгрузки и мест
	// прохода в единой форме слота (для модалки «Режимы работы» в ЛК). Чтение
	// любому авторизованному.
	protected.GET("/work-modes", d.WorkModes.GetWorkModes)

	// Кандидаты в получатели заявки - без права page.admin.users: выбор получателя есть
	// у любого, кто подаёт заявку, а раздача этого списка через админский /users/all
	// отбивала форму подачи 403 у арендатора. Отдаёт узкий срез (коллеги по организации
	// и компании плюс руководители) - не эквивалент списка всех учёток.
	// Статический сегмент объявлен до /users/:username: в роутинге Echo он приоритетнее.
	protected.GET("/users/recipient-candidates", users.GetRecipientCandidates)

	// Управление пользователями - page.admin.users (Ф5, ранее service checkAdmin
	// по type-коду manager/buropropuskov). Тот же ключ, что и у FE-роута раздела.
	requireUsers := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminUsers)
	protected.POST("/users", users.Create, requireUsers)
	protected.GET("/users/all", users.GetAll, requireUsers)
	protected.PUT("/users/:username/type", users.UpdateType, requireUsers)
	protected.PUT("/users/:username/password", users.UpdatePassword, requireUsers)
	// Смена пароля с отправкой письмом (#1910) - под тем же правом, что и ручная
	// установка пароля: это её замена, а не новое полномочие.
	protected.POST("/users/:username/rotate-password", users.RotatePassword, requireUsers)
	protected.PUT("/users/:username/info", users.UpdateInfo, requireUsers)
	protected.PUT("/users/:username/organization", users.UpdateOrganization, requireUsers)
	protected.PUT("/users/:username/company", users.UpdateCompany, requireUsers)
	protected.DELETE("/users/:username", users.Delete, requireUsers)
	protected.POST("/users/:username/restore", users.Restore, requireUsers)
	protected.GET("/users/:username/history", users.GetHistory, requireUsers)
	// Снятие блокировки входа живёт в auth (там политика лока), но гейтится как
	// остальное управление учётками.
	protected.POST("/users/:username/reset-lockout", auth.ResetLockout, requireUsers)
	// Групповые операции над пользователями (username-keyed). Статические сегменты
	// bulk/* приоритетнее /users/:username в роутинге Echo.
	protected.POST("/users/bulk/archive", users.BulkArchive, requireUsers)
	protected.POST("/users/bulk/restore", users.BulkRestore, requireUsers)
	protected.POST("/users/bulk/type", users.BulkUpdateType, requireUsers)
	protected.POST("/users/bulk/organization", users.BulkAssignOrganization, requireUsers)
	protected.POST("/users/bulk/company", users.BulkAssignCompany, requireUsers)
	// История входов пользователя (auth_events): вход/выход/провал/блокировка/сессия.
	protected.GET("/users/:username/auth-events", authEvents.ListForUser, requireUsers)
	// Привязка мест доступа к охраннику (#706)
	protected.GET("/users/:username/unload-places", users.GetUserUnloadPlaces, requireUsers)
	protected.PUT("/users/:username/unload-places", users.SetUserUnloadPlaces, requireUsers)
	protected.GET("/users/:username/tables", users.GetUserTables, requireUsers)
	protected.PUT("/users/:username/tables", users.SetUserTables, requireUsers)

	// Машины (в заявках)
	carsGroup := protected.Group("/cars")
	carsGroup.GET("/active-for-table/:table_id", cars.GetActiveCarsForTable)
	// Ручное добавление машин без заявки (#1049): super/admin проходят авто,
	// остальные - по гранту entity.cars.manual_add.
	carsGroup.POST("/manual", cars.CreateManualCars,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyEntityCarsManualAdd))
	carsGroup.GET("/fact-for-table/:table_id", cars.GetFactCarsForTable)
	carsGroup.GET("/unload-places", cars.GetCarUnloadPlaces)
	carsGroup.GET("/fact-unload-places", cars.GetFactCarUnloadPlaces)
	carsGroup.GET("/check-active", cars.CheckActiveCar)
	carsGroup.GET("/:id/history", cars.GetCarHistory)
	carsGroup.POST("/:id/history", cars.AddCarHistoryEntry)
	carsGroup.GET("/history/all", cars.GetAllCarsHistory)
	carsGroup.GET("/history/table/:table_id", cars.GetCarsHistoryByTable)
	carsGroup.GET("/history/current-status", cars.GetCarsCurrentStatus)
	carsGroup.PUT("/:id/territory-status", cars.UpdateCarTerritoryStatus, d.TablePassGate)
	carsGroup.PUT("/:id/deactivate", cars.DeactivateCar)
	carsGroup.PUT("/:id/activate", cars.ActivateCar)
	carsGroup.GET("/history/unified", cars.GetUnifiedCarHistory)
	carsGroup.PUT("/:id/restore", cars.RestoreCar)
	// Групповые операции над строками таблицы проходной (#1194): перенос/добавление/
	// снятие набора машин с таблиц «Проезд». Права admin - как остальные bulk-операции.
	carsGroup.POST("/bulk/move-table", cars.BulkMoveTable, requireAdmin)
	carsGroup.POST("/bulk/add-table", cars.BulkAddTable, requireAdmin)
	carsGroup.POST("/bulk/unbind-table", cars.BulkUnbindTable, requireAdmin)

	// Сотрудники (в заявках)
	empGroup := protected.Group("/employees")
	empGroup.POST("", employees.CreateEmployee)
	// Ручное добавление сотрудников без заявки (#1049): super/admin проходят авто,
	// остальные - по гранту entity.employees.manual_add.
	empGroup.POST("/manual", employees.CreateManualEmployees,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyEntityEmployeesManualAdd))
	empGroup.GET("/active-for-table/:table_id", employees.GetActiveEmployeesForTable)
	empGroup.PUT("/:id/territory-status", employees.UpdateEmployeeTerritoryStatus, d.TablePassGate)
	empGroup.PUT("/:id/deactivate", employees.DeactivateEmployee)
	empGroup.PUT("/:id/activate", employees.ActivateEmployee)
	empGroup.PUT("/:id/restore", employees.RestoreEmployee)
	// Групповые операции над строками таблицы проходной (#1194): статические
	// сегменты bulk/* приоритетнее /:id в роутинге Echo.
	empGroup.POST("/bulk/move-table", employees.BulkMoveTable, requireAdmin)
	empGroup.POST("/bulk/add-table", employees.BulkAddTable, requireAdmin)
	empGroup.POST("/bulk/unbind-table", employees.BulkUnbindTable, requireAdmin)
	empGroup.GET("/:id/history", employeesHistory.GetByEmployee)
	empGroup.GET("/history/unified", employeesHistory.GetUnified)
	empGroup.GET("/history/all", employeesHistory.GetAll)
	empGroup.GET("/history/current-status", employeesHistory.GetCurrentStatus)
	empGroup.GET("/history/table/:table_id", employeesHistory.GetByTable)

	// Системные таблицы (конструктор таблиц)
	stg := protected.Group("/system-tables")
	// GET-читалки таблиц открыты авторизованному: форма заявки берёт список таблиц КПП
	// для выбора места, доступ к содержимому конкретной таблицы гейтит фронт правом
	// table.<slug>.view. Изменение же структуры и настроек таблицы (создать/переименовать/
	// удалить/поля/слоты/окна/фото) - только конструктор (page.admin.tables_constructor),
	// его F12 не обойдёт. Вызывается всё это из TableConstructor.vue под тем же правом.
	stg.GET("", st.GetAll)
	stg.POST("", st.Create, requireTablesCtor)
	stg.GET("/:id", st.GetByID)
	stg.PUT("/:id", st.Update, requireTablesCtor)
	stg.DELETE("/:id", st.Delete, requireTablesCtor)
	stg.POST("/:id/restore", st.Restore, requireTablesCtor)
	stg.GET("/:id/usage", st.GetUsage)
	stg.POST("/:id/detach-all", st.DetachAll, requireAdmin)
	stg.DELETE("/:id/organizations/:org_id", st.DetachOrganization, requireAdmin)
	stg.DELETE("/:id/companies/:company_id", st.DetachCompany, requireAdmin)
	// param :id в Echo, поэтому /bulk/archive и /bulk/restore не конфликтуют с /:id/restore.
	stg.POST("/bulk/archive", st.BulkArchive, requireTablesCtor)
	stg.POST("/bulk/restore", st.BulkRestore, requireTablesCtor)
	stg.GET("/:id/history", st.GetHistory)
	stg.GET("/name/:name", st.GetByName)
	stg.GET("/:id/time-slots", st.GetTimeSlots)
	stg.POST("/:id/time-slots", st.AddTimeSlot, requireTablesCtor)
	stg.PUT("/:table_id/time-slots/:slot_id", st.UpdateTimeSlot, requireTablesCtor)
	stg.DELETE("/:table_id/time-slots/:slot_id", st.DeleteTimeSlot, requireTablesCtor)
	stg.GET("/:id/warning-windows", st.GetWarningWindows)
	stg.POST("/:id/warning-windows", st.AddWarningWindow, requireTablesCtor)
	stg.PUT("/:table_id/warning-windows/:window_id", st.UpdateWarningWindow, requireTablesCtor)
	stg.DELETE("/:table_id/warning-windows/:window_id", st.DeleteWarningWindow, requireTablesCtor)
	stg.POST("/:id/photos", st.UploadPhoto, requireTablesCtor)
	stg.DELETE("/:table_id/photos/:photo_id", st.DeletePhoto, requireTablesCtor)
	stg.POST("/:table_id/photos/:photo_id/main", st.SetMainPhoto, requireTablesCtor)

	// Версии (слепки) состояния таблицы (#980). Дневной снимок в 06:00 снимает джоба
	// (см. startDailyStatusReset), ручной - POST. Читалки под общей auth-защитой, как
	// соседние sub-роуты system-tables (trash/history): доступ вкладки гейтит фронт
	// правом table.<slug>.versions. Чистка разрушительна - только admin/super.
	stg.POST("/:id/snapshots", tsnap.Create, d.TableVersionsGate)
	stg.GET("/:id/snapshots", tsnap.List)
	stg.GET("/:id/snapshots/:sid", tsnap.Get)
	// Экспорт версии/текущего состояния (xlsx|pdf) файлом на скачивание. Читалка -
	// auth-only, как соседи; sid=current экспортирует текущее состояние таблицы.
	stg.GET("/:id/snapshots/:sid/export", tsnap.Export)
	stg.DELETE("/:id/snapshots", tsnap.Cleanup, requireAdmin)

	// Суточный отчёт охранника по проходам: живое окно [посл. 21:30, now) и
	// история дней. В отличие от соседей гейт НЕ на фронте: право
	// table.<name>.report проверяет BE (d.TableReportGate = RequireTableVerb),
	// FE-кнопка проверяет тот же ключ (#976: FE-гейт и BE-гейт - один набор прав).
	stg.GET("/:id/pass-report/live", passReport.Live, d.TableReportGate)
	stg.GET("/:id/pass-reports", passReport.List, d.TableReportGate)

	// Столбцы таблицы (#345) - изменение структуры, только конструктор.
	stg.PUT("/:id/fields", st.UpdateFields, requireTablesCtor)
	// Столбцы фактовой таблицы (#345)
	stg.PUT("/:id/fact-fields", st.UpdateFactFields, requireTablesCtor)

	// Корзина таблицы (#186) - удалённые элементы с возможностью восстановить
	// или окончательно удалить. Тип элементов определяется по table_type
	// системной таблицы (cars или people).
	stg.GET("/:id/trash", trash.List)
	stg.GET("/:id/trash/history", trash.History)
	stg.POST("/:id/trash/restore", trash.Restore, d.TableTrashGate)
	stg.DELETE("/:id/trash/:item_id", trash.PurgeOne, d.TableTrashGate)
	stg.DELETE("/:id/trash", trash.ClearAll, d.TableTrashGate)

	// Реестр автомобилей (unique_cars)
	ucg := protected.Group("/unique-cars")
	ucg.GET("", uc.GetAll)
	ucg.POST("", uc.Create)
	ucg.POST("/batch", uc.CreateBatch)
	ucg.PUT("/:id", uc.Update)
	ucg.PUT("/by-number", uc.UpdateByNumber)
	ucg.DELETE("/:id", uc.Delete)
	ucg.GET("/ownership-info", uc.GetOwnershipInfo)
	ucg.GET("/lookup", uc.Lookup, requireBlacklist)
	ucg.GET("/:id/history", uc.GetHistory)
	// Журнал всего реестра (включая удалённые записи) - раньше конкретного /:id/history,
	// иначе «history» попало бы в :id и разбор номера вернул бы 400.
	ucg.GET("/history", uc.GetRegistryLog)

	// Реестр сотрудников (unique_employees)
	ueg := protected.Group("/unique-employees")
	ueg.GET("", ue.GetAll)
	ueg.POST("", ue.Create)
	ueg.PUT("/:id", ue.Update)
	ueg.DELETE("/:id", ue.Delete)
	ueg.GET("/ownership-info", ue.GetOwnershipInfo)
	ueg.GET("/lookup", ue.Lookup, requireBlacklist)
	ueg.GET("/:id/history", ue.GetHistory)
	ueg.GET("/history", ue.GetRegistryLog)

	// Обратная связь. Отправка (POST) и свои обращения (GET /my) - любому
	// авторизованному; админ-операции (список/статистика/статус/прочтение) -
	// page.admin.feedback (Ф5, ранее service checkAdmin).
	requireFeedbackAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminFeedback)
	fbg := protected.Group("/feedback")
	fbg.POST("", fb.Create)
	fbg.GET("/all", fb.GetAll, requireFeedbackAdmin)
	fbg.GET("/stats", fb.GetStats, requireFeedbackAdmin)
	fbg.GET("/my", fb.GetMy)
	fbg.PUT("/:id/status", fb.UpdateStatus, requireFeedbackAdmin)
	fbg.PUT("/:id/read", fb.MarkAsRead, requireFeedbackAdmin)
	fbg.PUT("/:id/flag", fb.SetFlag, requireFeedbackAdmin)

	// Заявки
	apg := protected.Group("/applications", echomw.BodyLimit(applicationsBodyLimit))
	apg.GET("", app.GetApplications)
	apg.POST("", app.CreateApplication)
	apg.POST("/submit-complete-application", app.SubmitCompleteApplication)

	// Файлы заявки (#1721). Черновики лежат до подачи, поэтому загрузка и удаление
	// висят на группе заявок без :id. Отдельного права нет: файлы видны тем же,
	// кому видна заявка, проверка доступа - внутри обработчиков.
	if d.ApplicationFiles != nil {
		apg.POST("/files", d.ApplicationFiles.UploadDraft)
		apg.DELETE("/files/:id", d.ApplicationFiles.DeleteDraft)
		apg.GET("/:id/files", d.ApplicationFiles.List)
		apg.GET("/:id/files/:file_id", d.ApplicationFiles.Download)
		// Удаление приложенного файла - под общим админским правом (page.admin), тем
		// же, что открывает раздел администрирования: состав заявки после подачи
		// неизменен, а вычистить приложенное вопреки запрету должен уметь не только
		// супер-администратор.
		apg.DELETE("/:id/files/:file_id", d.ApplicationFiles.DeleteAttached, requireAdmin)
	}
	// Выгрузка реестра заявок (#1832) - под тем же правом, что скачивание бланка:
	// «Экспорт заявок». Роут объявлен до /:id, иначе номер заявки перехватил бы слово
	// export. Обращения пишутся в журнал 152-ФЗ (pdPaths): один файл уносит
	// персональные данные пачкой.
	apg.GET("/export", app.ExportApplications,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionExportApplications))
	apg.GET("/user", app.GetUserApplications)
	apg.GET("/user/status-updates-count", app.GetUserStatusUpdatesCount) // #1349 - счётчик чипа "Обновления" в ЛК
	apg.GET("/unread-count", app.GetUnreadCount)
	apg.GET("/available-attachments", app.GetAvailableAttachments)          // #706 - "Доступные мне" для охранников
	apg.GET("/available-attachments/:id", app.GetAvailableAttachmentDetail) // #706 - деталь вложения
	apg.GET("/attachable", app.GetAttachableApplications, requireAdmin)     // #1049 - заявки для привязки ручного вложения (super/admin)
	apg.GET("/:id", app.GetApplicationByID)
	apg.PUT("/:id", app.UpdateApplication)
	apg.GET("/:id/responsible-users", app.GetApplicationResponsibleUsers)
	apg.GET("/:id/participants", app.GetApplicationParticipants) // все участники заявки с ролями и контактами
	apg.GET("/:id/details", app.GetApplicationDetails)
	apg.GET("/:id/attachments", app.GetApplicationAttachments)
	apg.GET("/:id/blank", attachmentBlanks.Download) // #183 - скачать заполненный .xlsx
	if archiveDownload != nil {
		// ZIP сохранённых бланков заявки из файлового архива (#1615, B3). Гейт тот же,
		// что у скачивания одного бланка (canDownloadBlank) - не выше и не ниже.
		apg.GET("/:id/archive", archiveDownload.Archive)
	}
	apg.POST("/:id/update-items-status", app.UpdateApplicationItemsStatus)
	apg.POST("/:id/forward", app.ForwardApplication)
	apg.GET("/:id/forward-messages", app.GetForwardMessages) // #967 - ветка заявки (пересылки)
	apg.POST("/:id/approve", app.ApproveApplicationByUser)
	apg.POST("/:id/blacklist-overrides", app.OverrideBlacklistFlag)     // #481 - "всё равно пропустить"
	apg.DELETE("/:id/blacklist-overrides", app.DeleteBlacklistOverride) // #481 - отмена подтверждения (срез C)
	apg.GET("/:id/check-approval-status", app.CheckApprovalStatus)
	apg.POST("/:id/take-to-work", app.TakeApplicationToWork)
	// Заметка бюро по заявке. Права нет, проверяет сервис: вести её вправе только
	// принимающий, а принимающий - роль из справочника, а не разрешение.
	apg.PUT("/:id/bureau-note", app.SetBureauNote)
	// #1393 - принимающий доназначает посты и места элементам заявки
	apg.PUT("/:id/elements/tables", app.AssignElementTables)
	// Принимающий убирает человека или машину из поданной заявки: решение для случая,
	// когда пропустить помеченный элемент нельзя, а заявку провести надо.
	apg.DELETE("/:id/elements", app.RemoveApplicationElements)
	apg.PUT("/:id/elements/unload-places", app.AssignCarUnloadPlaces)
	apg.POST("/:id/revoke-from-work", app.RevokeApplicationFromWork)
	apg.POST("/:id/restore-to-work", app.RestoreApplicationToWork)
	apg.POST("/:id/withdraw", app.WithdrawApplication)
	// Дополнение поданной заявки (#1685). Право - продолжение подачи; владение заявкой
	// право не покрывает, его проверяет сервис. Чтение раундов открыто всем, кому видна
	// заявка (CanAccessApplication в handler) - согласующему раунд нужен так же, как автору.
	apg.POST("/:id/supplements", app.CreateSupplement,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionSupplementApplication))
	apg.GET("/:id/supplements", app.GetApplicationSupplements)
	// Голосование по раунду дополнения. Право не требуется, как и у согласования заявки:
	// голосовать вправе только состав раунда, и это проверяет сервис.
	apg.POST("/:id/supplements/:sid/approve", app.ApproveSupplement)
	apg.POST("/:id/supplements/:sid/revoke-approval", app.RevokeSupplementApproval)
	// Решение по раунду. Права тоже нет: принять/отклонить вправе только принимающий,
	// снять - только автор заявки, и обе роли проверяет сервис.
	apg.POST("/:id/supplements/:sid/take-to-work", app.DecideSupplement)
	apg.POST("/:id/supplements/:sid/cancel", app.CancelSupplement)
	apg.GET("/:id/history", app.GetApplicationHistory)
	apg.POST("/:id/revoke-approval", app.RevokeApproval)
	apg.POST("/history", app.AddHistoryEntry)
	apg.GET("/:id/viewers", app.GetApplicationViewers)
	apg.POST("/:id/read", app.MarkAsRead)
	apg.GET("/:id/reads", app.GetReads)

	// Вопросы к заявке (#973: Q&A-топики + тред ответов)
	apg.GET("/:id/questions", app.GetApplicationQuestions)
	apg.POST("/:id/questions", app.CreateApplicationQuestion)
	apg.POST("/:id/questions/seen", app.MarkQuestionsSeen)
	apg.POST("/:id/questions/:questionId/answers", app.CreateApplicationAnswer)
	apg.POST("/:id/questions/:questionId/read", app.MarkQuestionRead)

	// Вложения заявок (cars/employees/items внутри вложений)
	att.GET("/:id/cars", app.GetAttachmentCars)
	att.GET("/:id/employees", app.GetAttachmentEmployees)
	att.GET("/:id/items", app.GetAttachmentItems)

	// Утверждающие заявок. Управление - админ справочников (page.admin.directories,
	// тем же правом фронт открывает /admin/approvers); журнал (history) доступен
	// всем авторизованным (как и раньше - без checkAdmin).
	aag := protected.Group("/application-approvers")
	aag.GET("", approvers.GetAll, requireDirectories)
	// Получатели заявки: только отображаемые имена, поэтому без права на справочники -
	// иначе заявитель не видел бы, кому уходит его заявка.
	aag.GET("/recipients", approvers.GetRecipients)
	aag.GET("/available-users", approvers.GetAvailableUsers, requireDirectories)
	aag.GET("/history", approvers.GetHistory)
	// Ответ про себя доступен любому авторизованному: карточке заявки нужно знать,
	// показывать ли кнопки принимающего, а весь состав ей не нужен и закрыт админом.
	aag.GET("/me", approvers.IsApprover)
	aag.POST("", approvers.Create, requireDirectories)
	aag.PATCH("/:id", approvers.Update, requireDirectories)
	aag.DELETE("/:id", approvers.Delete, requireDirectories)

	// permission.audit.manage = управление системой прав (роли, группы, назначения,
	// индивидуальные права пользователей). super + admin проходят (audit.manage не
	// super-only), обычный - по гранту. auditRead - чтение журнала отказов.
	// Публичные GET-списки групп/ролей остаются открытыми любому авторизованному.
	auditRead := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditRead)
	auditManage := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditManage)
	// Выдача/снятие тумблера "Администратор" -- super-only (ключ action.grant.admin).
	grantAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionGrantAdmin)

	// Разрешения. Чтение/запись чужих прав (effective, override) - auditManage
	// (super + admin). Выдача super-only ключей через override не-суперу запрещена
	// в сервисе. Свои права (/my) - любому авторизованному.
	permGroup := protected.Group("/permissions")
	permGroup.GET("/my", permissions.GetMyPermissions)
	permGroup.GET("/user/:id", permissions.GetUserPermissions, auditManage)
	permGroup.GET("/user/:id/effective", permissions.GetUserEffectivePermissions, auditManage)
	permGroup.PUT("/user/:id", permissions.UpdateUserPermissions, auditManage)
	// Каталог - полный перечень ключей системы с человеческими названиями, то есть
	// карта устройства доступа. Читают его только редакторы прав (модалка прав
	// пользователя, роли, группы), поэтому гейтим как остальную группу (#1967).
	permGroup.GET("/catalog", permissions.GetCatalog, auditManage)
	// Генерация прав для таблицы. Фронт напрямую не дёргает - права создаются
	// автоматически внутри создания таблицы (system_table_service). Прямой роут
	// закрыт тем же правом, что и конструктор, чтобы обычный юзер не мог плодить
	// ключи прав в обход.
	permGroup.POST("/auto-generate", permissions.AutoGenerate, requireTablesCtor)

	// Группы прав (#187a). CRUD защищён permission.audit.manage.
	pgGroup := protected.Group("/permission-groups")
	// Чтение групп прав - это выгрузка карты доступов системы, не справочник для
	// формы. Обычному юзеру не нужно (фронт открывает раздел под audit.manage),
	// поэтому список и карточку гейтим тем же правом, что и запись.
	pgGroup.GET("", permGroups.List, auditManage)
	pgGroup.GET("/:id", permGroups.Get, auditManage)
	pgGroup.POST("", permGroups.Create, auditManage)
	pgGroup.PUT("/:id", permGroups.Update, auditManage)
	pgGroup.DELETE("/:id", permGroups.Delete, auditManage)
	pgGroup.POST("/merge", permGroups.Merge, auditManage)
	protected.GET("/users/:user_id/permission-groups", permGroups.ListForUser)
	protected.POST("/users/:user_id/permission-groups/:group_id", permGroups.AssignToUser, auditManage)
	protected.DELETE("/users/:user_id/permission-groups/:group_id", permGroups.UnassignFromUser, auditManage)
	protected.PUT("/users/:id/role", permGroups.SetUserRole, auditManage)
	protected.PUT("/users/:id/admin", permGroups.SetUserAdmin, grantAdmin)

	// Роли (#187a). CRUD защищён permission.audit.manage.
	rolesGroup := protected.Group("/roles")
	// Список ролей с их грантами - тоже карта доступов, гейтим как запись.
	rolesGroup.GET("", roles.List, auditManage)
	rolesGroup.POST("", roles.Create, auditManage)
	rolesGroup.PUT("/:id", roles.Update, auditManage)
	rolesGroup.DELETE("/:id", roles.Delete, auditManage)
	rolesGroup.PUT("/:id/default-groups", roles.SetDefaultGroups, auditManage)
	rolesGroup.PUT("/:id/permissions", roles.SetPermissions, auditManage)

	// Журнал доступа к персональным данным (152-ФЗ, #1472). Только чтение: записи
	// по закону не удаляются, сроком хранения занимаются партиции таблицы.
	pdAuditRead := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminPDAudit)
	protected.GET("/pd-audit", d.PDAudit.List, pdAuditRead)

	// Журнал отказов в доступе (#230).
	denialsGroup := protected.Group("/access-denials")
	denialsGroup.GET("", accessDenials.List, auditRead)
	denialsGroup.GET("/archive", accessDenials.ListArchive, auditRead)
	denialsGroup.DELETE("", accessDenials.DeleteByFilter, auditManage)
	denialsGroup.POST("/archive", accessDenials.ArchiveOlderThan, auditManage)

	// Бан пользователей (#230). Защищён action.ban.user.
	banUser := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionBanUser)
	protected.POST("/users/:id/ban", userBan.Ban, banUser)
	protected.POST("/users/:id/unban", userBan.Unban, banUser)
	protected.POST("/users/bulk/ban", userBan.BulkBan, banUser)
	protected.POST("/users/bulk/unban", userBan.BulkUnban, banUser)

	// Режим «войти как пользователь» (#1912) - замена практике «администратор знает
	// пароль работника». Вход гейтится правом, возврат в свою учётную запись - нет:
	// его делает тот, кто уже в режиме, и отказать ему значило бы запереть человека
	// в чужой учётной записи до истечения маркера.
	if d.Impersonation != nil {
		requireImpersonate := mw.RequirePermissionV2(permResolver, denialLog, services.KeyUserImpersonate)
		protected.POST("/users/:id/impersonate", d.Impersonation.Start, requireImpersonate)
		protected.POST("/impersonation/stop", d.Impersonation.Stop)
	}

	// Согласие на обработку ПД (152-ФЗ)
	consents := protected.Group("/consents")
	// Состояние согласия и его подтверждение при первом входе (#1567). Доступны
	// любому авторизованному: именно ими кормится окно согласия.
	consents.GET("/gate", consent.GetGate)
	consents.POST("/accept", consent.Accept)
	consents.POST("", consent.Grant)
	consents.DELETE("/:type", consent.Revoke)
	consents.GET("", consent.List)
	consents.GET("/check/:type", consent.Check)

	// Отзыв согласия за работника: он приходит с просьбой к администратору,
	// своей кнопки отзыва у него нет. Право то же, что у раздела работников.
	protected.DELETE("/users/:username/consent", consent.RevokeForUser, requireUsers)

	// Настройки системы. GetAll/Update - под page.admin.settings (#7, ранее
	// checkSuper в settings_service.go): администраторы получают ключ через
	// adminAll (не super-only), точечно снимается личным deny-override.
	requireSettings := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminSettings)
	protected.GET("/settings", settings.GetAll, requireSettings)
	protected.GET("/settings/upload", settings.GetUploadSettings)
	protected.GET("/settings/notifications", settings.GetNotificationSettings)
	protected.GET("/settings/password-policy", settings.GetPasswordPolicy)
	// Почта (#1906): состояние настройки и проверочное письмо. Оба под тем же
	// правом, что и остальные настройки. Конкретные пути объявлены ДО
	// PUT /settings/:key намеренно - иначе echo увидел бы в "mail" значение
	// параметра key и попытался бы сохранить настройку с таким именем.
	protected.GET("/settings/mail/status", settings.GetMailStatus, requireSettings)
	protected.POST("/settings/mail/test", settings.SendTestMail, requireSettings)
	protected.GET("/settings/password-rotation/status", settings.GetPasswordRotationStatus, requireSettings)
	protected.GET("/settings/password-rotation/last", settings.GetPasswordRotationLast, requireSettings)
	// Ручной прогон - под своим правом, а не под настройками: сброс паролей всей
	// организации весит больше, чем правка телефона бюро (#1910).
	protected.POST("/settings/password-rotation/run", settings.RunPasswordRotation,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionRotatePasswords))
	protected.PUT("/settings/:key", settings.Update, requireSettings)

	// Новости. Активные (GET "") - всем авторизованным; управление - админ справочников
	// (page.admin.directories, тем же правом фронт открывает /admin/news).
	ng := protected.Group("/news")
	ng.GET("", news.GetActiveNews)
	ng.GET("/all", news.GetAllNews, requireDirectories)
	ng.POST("", news.CreateNews, requireDirectories)
	ng.PUT("/:id", news.UpdateNews, requireDirectories)
	ng.DELETE("/:id", news.DeleteNews, requireDirectories)

	// Объявления. Активное (GET /active) - всем авторизованным; управление - тем же
	// правом, что новости: это один экран «Новости и объявления».
	ag := protected.Group("/announcements")
	ag.GET("/active", news.GetActiveAnnouncement)
	ag.GET("/all", news.GetAllAnnouncements, requireDirectories)
	ag.POST("", news.CreateAnnouncement, requireDirectories)
	ag.POST("/set-active", news.SetActiveAnnouncement, requireDirectories)
	ag.POST("/:id/hide", news.HideAnnouncement, requireDirectories)
	ag.PUT("/:id", news.UpdateAnnouncement, requireDirectories)
	ag.DELETE("/:id", news.DeleteAnnouncement, requireDirectories)

	// Уведомления. Свои - любому авторизованному; рассылка (Create) - админ
	// (page.admin, Ф5: ранее handler-проверка type_id 5/6 manager/buropropuskov).
	// Подписки (preferences) и «прочитать все» (read-all) - тоже любому авторизованному:
	// это настройка и действие над собственной лентой, не рассылка (#1748).
	notif := protected.Group("/notifications")
	notif.GET("", notifications.GetNotifications)
	notif.GET("/preferences", notifications.GetPreferences)
	notif.PUT("/preferences", notifications.UpdatePreferences)
	notif.PUT("/read-all", notifications.MarkAllRead)
	notif.POST("", notifications.Create, requireAdmin)
	notif.PUT("/:id/read", notifications.MarkRead)
	notif.DELETE("/:id", notifications.Delete)
	notif.DELETE("", notifications.DeleteAll)

	// Web Push (#974): подписка браузера на доставку уведомлений при закрытой вкладке -
	// личная настройка устройства, тот же доступ, что у preferences/read-all выше.
	notif.GET("/push/status", push.GetStatus)
	notif.POST("/push/subscribe", push.Subscribe)
	notif.DELETE("/push/subscribe", push.Unsubscribe)
	// Сводка использования push - НЕ личная настройка, а админский разрез (раздел
	// статистики): гейт page.statistics, как у всей остальной статистики дашборда.
	notif.GET("/push/summary", push.GetSummary, mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageStatistics))

	// Логи запросов (мониторинг) - под page.admin.monitoring (#2125): тем же ключом
	// раздел гейтится в меню и роутере фронта, а требование page.admin отбивало
	// носителя ключа на API. Администраторы проходят через adminAll, личный
	// deny-override на этот ключ раздел закрывает.
	requireMonitoring := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminMonitoring)
	rlg := protected.Group("/request-logs", requireMonitoring)
	rlg.GET("", requestLogs.GetLogs)
	rlg.GET("/users", requestLogs.GetUsers)
	rlg.GET("/stats", requestLogs.GetStats)
	rlg.GET("/realtime", requestLogs.GetRealtime)
	rlg.GET("/timeline", requestLogs.GetTimeline)
	rlg.GET("/history", requestLogs.GetHistory)
	rlg.GET("/export", requestLogs.Export)

	// Bug-report - юзер отправляет со страницы Error500 (POST /api/bug-report)
	protected.POST("/bug-report", bugReport.Submit)

	// Админский toggle maintenance-режима (только type_id=6).
	adminMaint := protected.Group("/admin")
	adminMaint.GET("/maintenance", maintenance.GetAdminStatus)
	adminMaint.PUT("/maintenance", maintenance.ToggleMaintenance)

	// Документы (#39). Admin-операции под page.admin.directories - тем же правом фронт
	// открывает /admin/documents; скачивание и публичный список -- под auth.

	// Сброс онбординг-тура пользователю - админ-действие (после сброса у юзера
	// снова автозапуск). Под page.admin, в отличие от self-эндпоинтов /onboarding.
	protected.POST("/users/:username/onboarding/reset", onboarding.ResetForUser, requireAdmin)
	if docGroups != nil {
		dgGroup := protected.Group("/document-groups")
		dgGroup.GET("", docGroups.List, requireDirectories)
		dgGroup.POST("", docGroups.Create, requireDirectories)
		dgGroup.PUT("/reorder", docGroups.Reorder, requireDirectories)
		dgGroup.PUT("/:id", docGroups.Update, requireDirectories)
		dgGroup.DELETE("/:id", docGroups.Delete, requireDirectories)
	}
	if docs != nil {
		docsGroup := protected.Group("/documents")
		docsGroup.GET("", docs.List, requireDirectories)
		docsGroup.POST("", docs.Upload, requireDirectories)
		docsGroup.PUT("/reorder", docs.Reorder, requireDirectories)
		docsGroup.PUT("/:id", docs.UpdateMeta, requireDirectories)
		docsGroup.PUT("/:id/file", docs.ReplaceFile, requireDirectories)
		docsGroup.DELETE("/:id", docs.Delete, requireDirectories)
		docsGroup.GET("/:id/download", docs.Download)

		protected.GET("/public/documents", docs.GetPublic)
	}

	// Руководство (B1). Любому авторизованному; список отдаёт только разделы, на
	// которые есть право guide.<role>, скачивание гейтит ту же проверку в хендлере
	// (динамический ключ по :role не выражается статическим middleware).
	if guide != nil {
		guideGroup := protected.Group("/guide")
		guideGroup.GET("/sections", guide.ListSections)
		guideGroup.GET("/sections/:role/download", guide.Download)

		// Админ-управление (B1b): правка текста раздела + загрузка/удаление PDF. Гейт page.admin.
		guideGroup.GET("/admin/sections", guide.AdminListSections, requireAdmin)
		guideGroup.PUT("/admin/sections/:role", guide.UpdateSection, requireAdmin)
		guideGroup.PUT("/admin/sections/:role/file", guide.UploadFile, requireAdmin)
		guideGroup.DELETE("/admin/sections/:role/file", guide.DeleteFile, requireAdmin)
	}

	// Документ согласия на обработку данных. Просмотр/скачивание -- любому авторизованному
	// (виден на странице подачи заявки), управление -- под page.admin.
	if settings != nil {
		dpGroup := protected.Group("/settings/data-processing")
		dpGroup.GET("/document/meta", settings.GetDataProcessingMeta)
		dpGroup.GET("/document", settings.ServeDataProcessingDoc)
		dpGroup.POST("/document", settings.UploadDataProcessingDoc, requireAdmin)
		dpGroup.DELETE("/document", settings.DeleteDataProcessingDoc, requireAdmin)

		// Текст согласия на обработку ПД для запроса при первом входе (#1567).
		// Пока только управление под page.admin: пользовательская ручка появится
		// вместе с самим запросом согласия.
		pdcGroup := protected.Group("/settings/pd-consent", requireAdmin)
		pdcGroup.GET("", settings.GetPDConsentSettings)
		pdcGroup.GET("/collection", settings.GetPDConsentCollection)
		pdcGroup.PUT("/text", settings.UpdatePDConsentText)
		pdcGroup.PUT("/required", settings.UpdatePDConsentRequired)
		pdcGroup.POST("/require-again", settings.BumpPDConsentVersion)
	}

	// Файловый архив бланков (#1615): настройки раскладки и живое превью. Просмотр
	// раздела - по page.admin.file_archive, правка настроек - дополнительно по
	// action.manage.file_archive: сменённый шаблон разводит новые файлы мимо тех,
	// что уже лежат на диске, и это действие тяжелее просмотра.
	if blankArchive := d.BlankArchive; blankArchive != nil {
		requireFileArchive := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminFileArchive)
		manageFileArchive := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionManageFileArchive)
		faGroup := protected.Group("/file-archive", requireFileArchive)
		faGroup.GET("/settings", blankArchive.GetSettings)
		// Записи настроек здесь нет намеренно (#1615): раскладку каталогов, пороги
		// места и срок заморозки задаёт тот, кто разворачивает систему, командой
		// server archive на сервере. Корень архива и так живёт в переменной
		// окружения, а сменённый шаблон переносит дерево заявок целиком - держать
		// такое за веб-сессией администратора бюро значит отдавать управление
		// хранилищем персональных данных тому, чья работа - пропуска. В разделе
		// остаётся показ текущих значений (GET выше) и наблюдение.
		// Пересоздание файлов заявки переписывает диск - право то же, что на настройки.
		faGroup.POST("/applications/:id/reexport", blankArchive.Reexport, manageFileArchive)
		// Бэкфилл за период и пересборка типа после правки шаблона (#1615, B4) - тот же
		// уровень доступа: ручка ставит в очередь запись поверх файлов на диске.
		faGroup.POST("/backfill", blankArchive.Backfill, manageFileArchive)
		// Сводка места и квоты (#1615, срез B2) - тот же уровень доступа, что и
		// просмотр настроек: занятое место видит любой, кому виден раздел.
		if stats := d.BlankArchiveStats; stats != nil {
			faGroup.GET("/stats", stats.GetStats)
		}
		// Скачивание из файлового архива (#1615, срез B3). Список и оценка объёма -
		// тот же уровень, что просмотр раздела; фактическая выгрузка байтов (билет,
		// отдельный файл) требует дополнительно action.download.file_archive - право
		// на скачивание тяжелее просмотра сводки, как настройка тяжелее просмотра.
		if archiveDownload != nil {
			downloadFileArchive := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionDownloadFileArchive)
			faGroup.GET("/items", archiveDownload.ListItems)
			faGroup.POST("/estimate", archiveDownload.EstimateDownload)
			faGroup.POST("/download-ticket", archiveDownload.IssueDownloadTicket, downloadFileArchive)
			faGroup.GET("/files/:id", archiveDownload.DownloadFile, downloadFileArchive)
		}
	}

	// Статистика дашборда (#632). Доступ ограничен page.statistics.
	if statistics != nil {
		requireStats := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageStatistics)
		statsGroup := protected.Group("/statistics")
		statsGroup.GET("/summary", statistics.GetSummary, requireStats)
		statsGroup.GET("/processing-summary", statistics.GetProcessingSummary, requireStats)
		statsGroup.GET("/processing-journal", statistics.GetProcessingJournal, requireStats)
		if reminder != nil {
			statsGroup.GET("/stuck-approvals", reminder.GetStuckApprovals, requireStats)
		}
		statsGroup.GET("/timeline", statistics.GetTimeline, requireStats)
		statsGroup.GET("/online-peaks", statistics.GetOnlinePeaks, requireStats)
		statsGroup.GET("/online-users", statistics.GetOnlineUsers, requireStats)
		statsGroup.GET("/recent-passages", statistics.GetRecentPassages, requireStats)
		statsGroup.GET("/metrics", statistics.GetMetrics, requireStats)
		statsGroup.GET("/insights", statistics.GetInsights, requireStats)
		statsGroup.POST("/report", statistics.RunReport, requireStats)
		statsGroup.GET("/report/period", statistics.ReportDataPeriod, requireStats)
		statsGroup.GET("/templates", statistics.ListTemplates, requireStats)
		statsGroup.POST("/templates", statistics.CreateTemplate, requireStats)
		statsGroup.PUT("/templates/:id", statistics.UpdateTemplate, requireStats)
		statsGroup.DELETE("/templates/:id", statistics.DeleteTemplate, requireStats)
	}
}
