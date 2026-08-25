package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	"systemburo/internal/crypto"
)

type Config struct {
	DatabaseURL      string        `env:"DATABASE_URL,required"`
	BindHost         string        `env:"BIND_HOST" envDefault:"0.0.0.0"`
	BindPort         string        `env:"BIND_PORT" envDefault:"8090"`
	JWTSecret        string        `env:"JWT_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	JWTAccessTTL     time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL    time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
	LogLevel         string        `env:"LOG_LEVEL" envDefault:"info"`
	SwaggerEnabled   bool          `env:"SWAGGER_ENABLED" envDefault:"false"`

	// Файловое логирование с ротацией (lumberjack). LogFilePath пустой - пишем только
	// в stdout (как раньше). При заданном пути логи идут и в stdout (docker logs),
	// и в ротируемый файл. LogMaxAgeDays=30 - месячная ротация по времени.
	LogFilePath   string `env:"LOG_FILE_PATH" envDefault:""`
	LogMaxSizeMB  int    `env:"LOG_MAX_SIZE_MB" envDefault:"100"`
	LogMaxAgeDays int    `env:"LOG_MAX_AGE_DAYS" envDefault:"30"`
	LogMaxBackups int    `env:"LOG_MAX_BACKUPS" envDefault:"14"`
	LogCompress   bool   `env:"LOG_COMPRESS" envDefault:"true"`

	CORSAllowedOrigins      []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:8081" envSeparator:","`
	UploadMaxFileSize       int64    `env:"UPLOAD_MAX_FILE_SIZE" envDefault:"10485760"`
	UploadAllowedImageTypes []string `env:"UPLOAD_ALLOWED_IMAGE_TYPES" envDefault:"image/jpeg,image/png,image/webp" envSeparator:","`
	UploadAllowedDocTypes   []string `env:"UPLOAD_ALLOWED_DOC_TYPES" envDefault:"application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" envSeparator:","`

	// Файлы, прикладываемые к заявке (#1721). Размер одного файла берётся из
	// UPLOAD_MAX_FILE_SIZE, здесь - сколько их на заявку и сколько всего. Потолок
	// суммы нужен отдельно от количества: десять файлов по десять мегабайт
	// упрутся в client_max_body_size nginx и оборвутся уже на прокси.
	ApplicationFileMaxCount int   `env:"APPLICATION_FILE_MAX_COUNT" envDefault:"30"`
	ApplicationFileMaxTotal int64 `env:"APPLICATION_FILE_MAX_TOTAL_SIZE" envDefault:"104857600"`
	// ApplicationFileDraftTTL - сколько живёт загруженный, но так и не приложенный
	// к заявке файл: заявитель выбрал файлы и закрыл форму, не отправив её.
	ApplicationFileDraftTTL time.Duration `env:"APPLICATION_FILE_DRAFT_TTL" envDefault:"24h"`
	// Приведение снимков к предсказуемому виду (#1721). Перекодирование заодно
	// срезает EXIF: снимок с телефона несёт координаты съёмки и модель устройства.
	ApplicationFileImageMaxSide int `env:"APPLICATION_FILE_IMAGE_MAX_SIDE" envDefault:"2000"`
	ApplicationFileJPEGQuality  int `env:"APPLICATION_FILE_JPEG_QUALITY" envDefault:"82"`

	DataEncryptionKey  string `env:"DATA_ENCRYPTION_KEY" envDefault:""`
	RequireEncryption  bool   `env:"REQUIRE_ENCRYPTION" envDefault:"false"`
	RateLimitPerMinute int    `env:"RATE_LIMIT_PER_MINUTE" envDefault:"200"`
	RateLimitWindowSec int64  `env:"RATE_LIMIT_WINDOW_SEC" envDefault:"60"`

	// LoginRateLimit ограничивает попытки /login per-IP (защита от brute-force).
	// Дефолт 10/5м: лояльно к опечаткам пароля живых юзеров, но всё ещё
	// блокирует brute-force - Argon2id и так растягивает каждую попытку
	// на 100мс+, 10 попыток за 5 минут это ~17 запросов/час максимум.
	// В CI/e2e ставим LOGIN_RATE_LIMIT_MAX=1000.
	LoginRateLimitMax       int    `env:"LOGIN_RATE_LIMIT_MAX" envDefault:"10"`
	LoginRateLimitWindowSec int64  `env:"LOGIN_RATE_LIMIT_WINDOW_SEC" envDefault:"60"`
	PaginationMaxLimit      int    `env:"PAGINATION_MAX_LIMIT" envDefault:"100"`
	UploadPath              string `env:"UPLOAD_PATH" envDefault:"./uploads"`

	// ArchivePath - корень файлового архива бланков (#1615): заполненные .xlsx
	// раскладываются под ним по годам, месяцам и дням.
	//
	// Каталог обязан лежать ВНЕ UploadPath. Содержимое UploadPath раздаётся
	// статикой до проверки авторизации (router.go, api.Static("/uploads")), а в
	// бланке паспортные данные и патенты - те самые поля, которые в базе хранятся
	// зашифрованными. Архив внутри загрузок означал бы их доступность по прямой
	// ссылке кому угодно. Проверку делает Validate, и она отказывает в старте.
	//
	// На проде монтируется bind-mount-ом с отдельного раздела: путь должен быть
	// предсказуемым, чтобы его можно было зашифровать, отдать в сетевую папку
	// только на чтение и включить в резервное копирование.
	ArchivePath string `env:"ARCHIVE_PATH" envDefault:"./archive"`

	// ArchiveWorkerTick - как часто фоновый воркер разбирает очередь выгрузки.
	// Шифрование файлового архива. Бланки читает внешняя сторона, поэтому они
	// шифруются на её публичный ключ (ARCHIVE_AGE_RECIPIENT), а вторым получателем
	// идёт сама система (ARCHIVE_AGE_IDENTITY) - иначе она не отдаст ZIP по кнопке
	// в карточке заявки. Пустая пара оставляет прежний режим без шифрования.
	ArchiveAgeRecipient string `env:"ARCHIVE_AGE_RECIPIENT" envDefault:""`
	ArchiveAgeIdentity  string `env:"ARCHIVE_AGE_IDENTITY" envDefault:""`

	ArchiveWorkerTick time.Duration `env:"ARCHIVE_WORKER_TICK" envDefault:"15s"`

	// ArchiveSweepInterval - как часто подметаются заявки, для которых очередь
	// потеряна: постановка в неё идёт после коммита и намеренно best-effort, чтобы
	// выгрузка на диск не могла уронить подачу заявки.
	ArchiveSweepInterval time.Duration `env:"ARCHIVE_SWEEP_INTERVAL" envDefault:"5m"`

	// EntityExportPath - корень пакетов консольной выгрузки данных по сущности
	// (server entity export). Пусто по умолчанию намеренно: в пакете лежат все
	// персональные данные организации разом, и подставлять ему каталог «по
	// умолчанию» рядом с кодом нельзя - место хранения выбирает тот, кто
	// разворачивает систему. Пока значение не задано, команда выгрузки отказывает
	// с подсказкой, а не пишет пакет наугад.
	//
	// Каталог обязан лежать вне UploadPath по той же причине, что и ARCHIVE_PATH.
	EntityExportPath string `env:"ENTITY_EXPORT_PATH" envDefault:""`

	// CookieSecure управляет флагом Secure на refresh-cookie. На staging/prod
	// всегда true (HTTPS). На локальной разработке (http://localhost) - false,
	// иначе браузер не отправит cookie.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`

	// Telegram bot для bug-report-ов со страницы Error500. Оба поля опциональные:
	// если пустые - репорты пишутся только в БД, TG-отправка пропускается (warn-лог).
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN" envDefault:""`
	TelegramChatID   string `env:"TELEGRAM_CHAT_ID" envDefault:""`

	// ResetTimezone задаёт часовой пояс для ежедневного сброса территориальных статусов.
	// Используется для расчёта 06:00 локального времени. По умолчанию Europe/Moscow.
	ResetTimezone string `env:"RESET_TIMEZONE" envDefault:"Europe/Moscow"`

	// AnalyticsCacheRefreshSec - интервал обновления тёплого кэша аналитики дашборда
	// (in-memory + снимок в БД для прогрева после рестарта). 0 отключает кэш.
	AnalyticsCacheRefreshSec int `env:"ANALYTICS_CACHE_REFRESH_SEC" envDefault:"60"`

	// Партиционирование request_logs: детально храним RequestLogDetailDays дней
	// (партиции старше сворачиваются в дневные агрегаты и дропаются), партиции
	// создаём на RequestLogPartitionPrecreateDays вперёд.
	RequestLogDetailDays             int `env:"REQUEST_LOG_DETAIL_DAYS" envDefault:"30"`
	RequestLogPartitionPrecreateDays int `env:"REQUEST_LOG_PARTITION_PRECREATE_DAYS" envDefault:"7"`

	// PdAuditRetentionMonths - срок хранения аудита ПД (152-ФЗ): партиции старше
	// дропаются. По умолчанию 36 месяцев (3 года).
	PdAuditRetentionMonths int `env:"PD_AUDIT_RETENTION_MONTHS" envDefault:"36"`

	// Суточная уборка технического мусора (#1614). Считается от момента, когда
	// запись обесценилась: у токена - от истечения или отзыва, у уведомления - от
	// создания при снятой отметке непрочитанного. Остальные журналы автоматика не
	// трогает, для них есть подкоманда cleanup.
	RefreshTokenRetentionDays     int `env:"REFRESH_TOKEN_RETENTION_DAYS" envDefault:"30"`
	ReadNotificationRetentionDays int `env:"READ_NOTIFICATION_RETENTION_DAYS" envDefault:"30"`
	// NotificationRetentionDays - срок непрочитанных уведомлений (#1748, S9). Дольше
	// прочитанных нарочно: непрочитанное не обесценилось само по себе (человек его ещё
	// не видел), поэтому порог заметно мягче, а не совпадает с ReadNotificationRetentionDays.
	NotificationRetentionDays int `env:"NOTIFICATION_RETENTION_DAYS" envDefault:"90"`

	// Web Push (#974): доставка уведомлений в браузер, пока вкладка системы закрыта.
	// Оба ключа пустые - штатный режим "push выключен": подписка на экране настроек
	// всё равно сохраняется (пригодится, если ключи потом появятся), реальная отправка
	// молча пропускается и ошибок не сыплет - стенд без ключей работает как раньше.
	// Пара генерируется командой `server vapid` (cmd/server/vapid.go).
	VAPIDPublicKey  string `env:"VAPID_PUBLIC_KEY" envDefault:""`
	VAPIDPrivateKey string `env:"VAPID_PRIVATE_KEY" envDefault:""`
	// VAPIDSubject - контакт бюро в Sub VAPID-токена (mailto: или https://), по которому
	// push-сервис браузера может связаться с оператором при проблемах с рассылкой.
	// Спецификацией не обязателен, но браузеры настойчиво рекомендуют его задавать.
	VAPIDSubject string `env:"VAPID_SUBJECT" envDefault:""`

	// PushSubscriptionRetentionDays - через сколько дней без единой успешной доставки
	// подписка считается брошенной и подчищается уборкой (#974, database/retention.go).
	// Не совпадает с порогом подряд идущих неудач в push_service.go: тот ловит явно
	// мёртвый endpoint (браузер снял подписку, сервис отвечает не 404/410, а таймаутом),
	// этот - молчаливо забытое устройство, которое ещё отвечает 2xx, но человек им давно
	// не пользуется (ушёл в другой браузер, не отписавшись). 180 дней - половина года без
	// единого успеха, заведомо больше типового цикла смены браузера или устройства.
	PushSubscriptionRetentionDays int `env:"PUSH_SUBSCRIPTION_RETENTION_DAYS" envDefault:"180"`

	// Почтовая рассылка (#1906). Система не поднимает свой почтовый сервер, а
	// подключается клиентом к чужому: Джино, Яндекс 360, почтовый сервер
	// организации - параметры одни и те же. Пустой SMTP_HOST - штатный режим
	// "почта не настроена": письма не ставятся в очередь, а всё, что от неё
	// зависит, отказывается стартовать явно, а не молча копит недоставленное.
	SMTPHost string `env:"SMTP_HOST" envDefault:""`
	SMTPPort int    `env:"SMTP_PORT" envDefault:"587"`
	// SMTPUsername обычно совпадает с полным адресом ящика.
	SMTPUsername string `env:"SMTP_USERNAME" envDefault:""`
	SMTPPassword string `env:"SMTP_PASSWORD" envDefault:""`
	// SMTPFrom обязан совпадать с ящиком аутентификации: почтовые серверы
	// отвергают чужого отправителя ошибкой 550, и настройка выглядит рабочей
	// ровно до первого письма.
	SMTPFrom     string `env:"SMTP_FROM" envDefault:""`
	SMTPFromName string `env:"SMTP_FROM_NAME" envDefault:"Бюро пропусков"`
	// SMTPTLSMode: starttls (587), tls (465, шифрование с первого байта) или none.
	// none оставлен для внутреннего почтового сервера в закрытом контуре, где
	// шифрование снимает сама сеть; наружу так ходить нельзя.
	SMTPTLSMode    string `env:"SMTP_TLS_MODE" envDefault:"starttls"`
	SMTPTimeoutSec int    `env:"SMTP_TIMEOUT_SEC" envDefault:"15"`
	// SMTPRatePerHour - потолок отправки, заведомо ниже лимита провайдера
	// (у Джино 500 писем в час на обычной отправке). Упереться в чужой лимит
	// хуже, чем растянуть рассылку: сервер начинает отвечать отказом всем подряд.
	SMTPRatePerHour int `env:"SMTP_RATE_PER_HOUR" envDefault:"400"`
	// MailRetryAttempts - сколько раз пытаться доставить письмо, прежде чем
	// признать доставку несостоявшейся и позвать администратора.
	MailRetryAttempts int `env:"MAIL_RETRY_ATTEMPTS" envDefault:"5"`
	// MailWorkerTick - как часто разбирается очередь писем.
	MailWorkerTick time.Duration `env:"MAIL_WORKER_TICK" envDefault:"15s"`

	// Пул соединений с базой (database/sql под GORM). Своих значений здесь не было
	// вовсе, а умолчания драйвера под нагрузкой работают против системы: число
	// открытых соединений не ограничено ничем, простаивающих держится два. Первое
	// упирает приложение в max_connections самой базы и превращает всплеск запросов
	// в отказы вместо очереди, второе заставляет открывать соединение заново почти
	// на каждый запрос - Postgres форкает под соединение процесс, и стоит это
	// дороже большинства самих запросов.
	//
	// 50 открытых - треть от max_connections=150 (docker-compose.prod.yml). Остаток
	// не запас на всякий случай: мимо этого пула к базе ходят консольные подкоманды
	// (server cleanup, archive, entity export), pgAdmin, резервное копирование и
	// служебный резерв самой Postgres, и пул во весь предел базы отнимал бы
	// соединения у них - ровно тот отказ, от которого настройка и защищает. Выше
	// поднимать нечего: на 6 vCPU очередь внутри Postgres быстрее не делает, а
	// work_mem=32MB отводится на узел плана в каждом одновременном запросе.
	//
	// 25 простаивающих (половина предела) - чтобы после всплеска соединения
	// оставались горячими: ради этого настройка в первую очередь и делается.
	DBMaxOpenConns int `env:"DB_MAX_OPEN_CONNS" envDefault:"50"`
	DBMaxIdleConns int `env:"DB_MAX_IDLE_CONNS" envDefault:"25"`
	// DBConnMaxLifetime не даёт соединению жить вечно: после смены параметров базы
	// или переключения на реплику вечное соединение продолжает работать со старой
	// стороной. DBConnMaxIdleTime убирает соединения, поднятые под всплеск, чтобы
	// ночью пул опускался обратно к нескольким штукам, а не держал все 25.
	DBConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" envDefault:"1h"`
	DBConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME" envDefault:"10m"`

	// Таймауты HTTP-сервера. Не были заданы ни одного: зависшее соединение держало
	// горутину и дескриптор до перезапуска процесса, а медленная отправка заголовков
	// (slowloris) стоила атакующему одного сокета.
	//
	// Значения согласованы с nginx, а не выбраны из головы. Обычные методы под
	// location /api/ прокси обрывает на 60 секундах (proxy_read_timeout по
	// умолчанию), поэтому 120 секунд на запись физически не могут оборвать то, что
	// не оборвал бы прокси вдвое раньше. Три длинных маршрута nginx освобождает
	// явно - SSE-поток и обе выгрузки файлового архива, - и им приложение снимает
	// срок записи в самом обработчике через httpx.AllowLongResponse. Держать ради
	// них общий таймаут в час значило бы не иметь таймаута вовсе.
	//
	// Чтение тела: nginx буферизует запрос целиком (proxy_request_buffering включена
	// по умолчанию) и сам роняет медленного отправителя по client_body_timeout, так
	// что до приложения тело доезжает со скоростью локальной сети. 120 секунд на
	// 50 МБ потолка (client_max_body_size) - подстраховка на случай запуска без
	// прокси, а не рабочий предел.
	//
	// Ноль в любом из четырёх означает "без ограничения" (семантика net/http) и
	// оставлен как отдушина администратору: обработчик, который на его данных
	// законно идёт дольше, выключается параметром, а не правкой кода.
	HTTPReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"10s"`
	HTTPReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"120s"`
	HTTPWriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"120s"`
	HTTPIdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"120s"`

	// Argon2HashConcurrency ограничивает число одновременных проверок пароля.
	// Проверка намеренно дорогая: каждая держит 19 МБ рабочей памяти и занимает
	// ядро целиком (m=19456, t=2, p=1). Без предела утренний вход смены умножает
	// эти 19 МБ на число одновременных попыток, и защита от подбора пароля
	// становится способом положить сервер - своими же пользователями.
	//
	// 0 - по числу ядер (GOMAXPROCS, на проде зафиксирован в 6). Больше ядер смысла
	// не имеет: вычисление упирается в процессор при p=1, и лишние одновременные
	// проверки только делят то же время между собой, добавляя память. Меньше -
	// оставляет ядра простаивать там, где очередь и так растёт.
	Argon2HashConcurrency int `env:"ARGON2_HASH_CONCURRENCY" envDefault:"0"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate checks configuration values for correctness.
func (c *Config) Validate() error {
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(c.JWTSecret))
	}
	if len(c.JWTRefreshSecret) < 32 {
		return fmt.Errorf("JWT_REFRESH_SECRET must be at least 32 characters (got %d)", len(c.JWTRefreshSecret))
	}
	if !strings.HasPrefix(c.DatabaseURL, "postgres://") && !strings.HasPrefix(c.DatabaseURL, "postgresql://") {
		return fmt.Errorf("DATABASE_URL must be a PostgreSQL connection string")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error (got %q)", c.LogLevel)
	}
	if c.UploadMaxFileSize <= 0 {
		return fmt.Errorf("UPLOAD_MAX_FILE_SIZE must be positive (got %d)", c.UploadMaxFileSize)
	}
	if c.ApplicationFileMaxCount <= 0 {
		return fmt.Errorf("APPLICATION_FILE_MAX_COUNT must be positive (got %d)", c.ApplicationFileMaxCount)
	}
	if c.ApplicationFileMaxTotal < c.UploadMaxFileSize {
		return fmt.Errorf("APPLICATION_FILE_MAX_TOTAL_SIZE (%d) must not be less than UPLOAD_MAX_FILE_SIZE (%d): ни один файл нельзя было бы приложить", c.ApplicationFileMaxTotal, c.UploadMaxFileSize)
	}
	if c.ApplicationFileDraftTTL <= 0 {
		return fmt.Errorf("APPLICATION_FILE_DRAFT_TTL must be positive (got %s)", c.ApplicationFileDraftTTL)
	}
	if c.ApplicationFileImageMaxSide <= 0 {
		return fmt.Errorf("APPLICATION_FILE_IMAGE_MAX_SIDE must be positive (got %d)", c.ApplicationFileImageMaxSide)
	}
	if c.ApplicationFileJPEGQuality < 1 || c.ApplicationFileJPEGQuality > 100 {
		return fmt.Errorf("APPLICATION_FILE_JPEG_QUALITY must be within 1..100 (got %d)", c.ApplicationFileJPEGQuality)
	}
	if c.RequireEncryption && c.DataEncryptionKey == "" {
		return fmt.Errorf("REQUIRE_ENCRYPTION=true but DATA_ENCRYPTION_KEY is empty")
	}
	// Тот же рубильник закрывает и файловый архив. Пустые ключи означают запись
	// открытым текстом, и узнать об этом можно только по именам файлов в каталоге:
	// на staging архив так и писался месяц, пока не хватились. Требование заявлено
	// один раз - выполняться оно должно везде, где данные ложатся на диск.
	if c.RequireEncryption && (c.ArchiveAgeRecipient == "" || c.ArchiveAgeIdentity == "") {
		return fmt.Errorf("REQUIRE_ENCRYPTION=true but ARCHIVE_AGE_RECIPIENT/ARCHIVE_AGE_IDENTITY are empty: archive files would be written unencrypted")
	}
	if c.DataEncryptionKey != "" {
		if _, err := crypto.ParseHexKey(c.DataEncryptionKey); err != nil {
			return fmt.Errorf("DATA_ENCRYPTION_KEY: %w", err)
		}
	}
	if c.RateLimitPerMinute <= 0 {
		return fmt.Errorf("RATE_LIMIT_PER_MINUTE must be positive (got %d)", c.RateLimitPerMinute)
	}
	if c.PaginationMaxLimit <= 0 {
		return fmt.Errorf("PAGINATION_MAX_LIMIT must be positive (got %d)", c.PaginationMaxLimit)
	}
	if c.JWTAccessTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL must be positive (got %s)", c.JWTAccessTTL)
	}
	if c.JWTRefreshTTL <= c.JWTAccessTTL {
		return fmt.Errorf("JWT_REFRESH_TTL (%s) must be greater than JWT_ACCESS_TTL (%s)", c.JWTRefreshTTL, c.JWTAccessTTL)
	}
	if c.ArchiveWorkerTick <= 0 {
		return fmt.Errorf("ARCHIVE_WORKER_TICK must be positive (got %s)", c.ArchiveWorkerTick)
	}
	if c.ArchiveSweepInterval <= 0 {
		return fmt.Errorf("ARCHIVE_SWEEP_INTERVAL must be positive (got %s)", c.ArchiveSweepInterval)
	}
	if err := validateArchiveOutsideUploads(c.ArchivePath, c.UploadPath); err != nil {
		return err
	}
	if err := validateExportOutsideUploads(c.EntityExportPath, c.UploadPath); err != nil {
		return err
	}
	if (c.VAPIDPublicKey == "") != (c.VAPIDPrivateKey == "") {
		return fmt.Errorf("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must be set together (both empty disables push)")
	}
	// Контакт отправителя обязателен, когда доставка вне системы включена: службы
	// push (в первую очередь Google) отвергают уведомления с пустым полем sub, и
	// без этой проверки конфигурация выглядела бы рабочей, а уведомления молча не
	// доходили бы ни до кого.
	if c.VAPIDPublicKey != "" && c.VAPIDSubject == "" {
		return fmt.Errorf("VAPID_SUBJECT is required when push is enabled (mailto: address or https:// site)")
	}
	if c.VAPIDSubject != "" && !strings.HasPrefix(c.VAPIDSubject, "mailto:") && !strings.HasPrefix(c.VAPIDSubject, "https://") {
		return fmt.Errorf("VAPID_SUBJECT must start with mailto: or https:// (got %q)", c.VAPIDSubject)
	}
	if c.PushSubscriptionRetentionDays <= 0 {
		return fmt.Errorf("PUSH_SUBSCRIPTION_RETENTION_DAYS must be positive (got %d)", c.PushSubscriptionRetentionDays)
	}
	if err := c.validateMail(); err != nil {
		return err
	}
	if err := c.validatePoolAndLimits(); err != nil {
		return err
	}
	return nil
}

// validatePoolAndLimits ловит отрицательные пределы пула, таймаутов и параллелизма
// хеширования.
//
// Проверка нужна именно на отрицательные, потому что все три подсистемы трактуют их
// одинаково и молча: database/sql считает неположительный предел соединений
// отсутствием предела, net/http - неположительный таймаут отсутствием срока,
// семафор - неположительное число поводом взять умолчание. То есть опечатка в знаке
// не ломает запуск, а выключает ровно ту защиту, ради которой параметр и появился,
// и заметить это можно только под нагрузкой. Ноль оставлен валидным: он и означает
// осознанное "без ограничения".
func (c *Config) validatePoolAndLimits() error {
	negativeInts := []struct {
		name  string
		value int
	}{
		{"DB_MAX_OPEN_CONNS", c.DBMaxOpenConns},
		{"DB_MAX_IDLE_CONNS", c.DBMaxIdleConns},
		{"ARGON2_HASH_CONCURRENCY", c.Argon2HashConcurrency},
	}
	for _, p := range negativeInts {
		if p.value < 0 {
			return fmt.Errorf("%s must not be negative (got %d)", p.name, p.value)
		}
	}

	negativeDurations := []struct {
		name  string
		value time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", c.DBConnMaxLifetime},
		{"DB_CONN_MAX_IDLE_TIME", c.DBConnMaxIdleTime},
		{"HTTP_READ_HEADER_TIMEOUT", c.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", c.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout},
	}
	for _, p := range negativeDurations {
		if p.value < 0 {
			return fmt.Errorf("%s must not be negative (got %s)", p.name, p.value)
		}
	}

	// Простаивающих больше открытых - не ошибка для database/sql (лишнее он
	// обрежет сам), но почти всегда означает, что предел открытых опустили и забыли
	// про второй параметр. Отказ в старте дешевле пула, который ведёт себя не так,
	// как записано в файле параметров.
	if c.DBMaxOpenConns > 0 && c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS (%d) must not exceed DB_MAX_OPEN_CONNS (%d)", c.DBMaxIdleConns, c.DBMaxOpenConns)
	}
	return nil
}

// validateArchiveOutsideUploads не даёт запуститься с каталогом архива внутри
// каталога загрузок (или наоборот).
//
// Загрузки раздаются статикой до проверки авторизации, поэтому архив внутри них
// сделал бы заполненные бланки с персональными данными доступными по прямой ссылке.
// Это отказ в старте, а не предупреждение в лог: молча работающая система с утечкой
// хуже упавшей, а предупреждение при развёртывании никто не прочитает.
func validateArchiveOutsideUploads(archivePath, uploadPath string) error {
	if archivePath == "" || uploadPath == "" {
		return nil
	}

	archiveAbs, err := resolvePath(archivePath)
	if err != nil {
		return fmt.Errorf("ARCHIVE_PATH: %w", err)
	}
	uploadAbs, err := resolvePath(uploadPath)
	if err != nil {
		return fmt.Errorf("UPLOAD_PATH: %w", err)
	}

	switch {
	case archiveAbs == uploadAbs:
		return fmt.Errorf("ARCHIVE_PATH (%s) must differ from UPLOAD_PATH: uploads are served without authorization", archiveAbs)
	case isInside(archiveAbs, uploadAbs):
		return fmt.Errorf("ARCHIVE_PATH (%s) must not be inside UPLOAD_PATH (%s): uploads are served without authorization, blanks contain personal data", archiveAbs, uploadAbs)
	case isInside(uploadAbs, archiveAbs):
		return fmt.Errorf("UPLOAD_PATH (%s) must not be inside ARCHIVE_PATH (%s)", uploadAbs, archiveAbs)
	}
	return nil
}

// validateExportOutsideUploads держит каталог пакетов выгрузки вне каталога загрузок.
//
// Та же защита, что у архива бланков, и по той же причине: загрузки раздаются статикой
// до проверки авторизации. Разница в цене ошибки - в пакете лежит весь набор данных
// организации сразу, включая файлы заявок, поэтому каталог внутри загрузок означал бы
// выдачу всей выгрузки по прямой ссылке.
func validateExportOutsideUploads(exportPath, uploadPath string) error {
	if exportPath == "" || uploadPath == "" {
		return nil
	}

	exportAbs, err := resolvePath(exportPath)
	if err != nil {
		return fmt.Errorf("ENTITY_EXPORT_PATH: %w", err)
	}
	uploadAbs, err := resolvePath(uploadPath)
	if err != nil {
		return fmt.Errorf("UPLOAD_PATH: %w", err)
	}

	switch {
	case exportAbs == uploadAbs, isInside(exportAbs, uploadAbs):
		return fmt.Errorf("ENTITY_EXPORT_PATH (%s) must be outside UPLOAD_PATH (%s): uploads are served without authorization, an export package holds the whole personal data set of an entity", exportAbs, uploadAbs)
	case isInside(uploadAbs, exportAbs):
		return fmt.Errorf("UPLOAD_PATH (%s) must not be inside ENTITY_EXPORT_PATH (%s)", uploadAbs, exportAbs)
	}
	return nil
}

// resolvePath приводит путь к абсолютному и разворачивает символические ссылки.
//
// Без разворачивания проверка сравнивала бы лексические пути, и каталог архива,
// подложенный ссылкой внутрь загрузок, прошёл бы её - то есть ровно тот случай, от
// которого весь этот код и защищает.
//
// Каталога может ещё не быть: при первом развёртывании приложение стартует до того,
// как оператор создаст его руками. Поэтому разворачиваем ближайшего существующего
// предка и приклеиваем остаток пути.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	rest := ""
	for cur := abs; ; {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			// Развернуть не дали по другой причине (например, нет прав на чтение
			// промежуточного каталога). Сравним хотя бы лексические пути: неполная
			// проверка лучше, чем отказ стартовать из-за особенностей монтирования.
			return abs, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// isInside сообщает, лежит ли child под parent. Оба пути должны быть абсолютными.
func isInside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// MailEnabled сообщает, настроена ли отправка почты. Пустой SMTP_HOST выключает
// её целиком - тем же способом, что пустые VAPID-ключи выключают push.
func (c *Config) MailEnabled() bool {
	return c.SMTPHost != ""
}

// validateMail проверяет параметры почты. Полуготовая настройка (хост есть,
// отправителя нет) не должна доживать до первого письма: там она превратится в
// отказ 550 посреди рассылки, когда пароли уже сменены.
func (c *Config) validateMail() error {
	if !c.MailEnabled() {
		// Почта выключена: остальные параметры не важны, стенд без неё работает.
		return nil
	}
	switch c.SMTPTLSMode {
	case "starttls", "tls", "none":
	default:
		return fmt.Errorf("SMTP_TLS_MODE must be one of: starttls, tls, none (got %q)", c.SMTPTLSMode)
	}
	if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
		return fmt.Errorf("SMTP_PORT must be between 1 and 65535 (got %d)", c.SMTPPort)
	}
	if c.SMTPFrom == "" {
		return fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
	}
	if _, err := mail.ParseAddress(c.SMTPFrom); err != nil {
		return fmt.Errorf("SMTP_FROM must be a valid email address (got %q)", c.SMTPFrom)
	}
	// Пароль без логина и наоборот - почти всегда опечатка в файле параметров:
	// сервер ответит 535, а выглядеть это будет как "письма не приходят".
	if (c.SMTPUsername == "") != (c.SMTPPassword == "") {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be set together (both empty means server without authentication)")
	}
	if c.SMTPTimeoutSec <= 0 {
		return fmt.Errorf("SMTP_TIMEOUT_SEC must be positive (got %d)", c.SMTPTimeoutSec)
	}
	if c.SMTPRatePerHour <= 0 {
		return fmt.Errorf("SMTP_RATE_PER_HOUR must be positive (got %d)", c.SMTPRatePerHour)
	}
	if c.MailRetryAttempts <= 0 {
		return fmt.Errorf("MAIL_RETRY_ATTEMPTS must be positive (got %d)", c.MailRetryAttempts)
	}
	if c.MailWorkerTick <= 0 {
		return fmt.Errorf("MAIL_WORKER_TICK must be positive (got %s)", c.MailWorkerTick)
	}
	return nil
}
