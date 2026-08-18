package middleware

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	// requestLogWriteTimeout - таймаут на запись пачки в БД из фонового писателя.
	// Защита от висящей вставки при медленной БД (например, во время shutdown).
	requestLogWriteTimeout = 5 * time.Second

	// requestLogBatchSize - сколько записей копится перед одним INSERT. Раньше
	// каждый запрос порождал горутину и отдельную вставку: на стенде это 350 тысяч
	// транзакций на журнал, который читают несколько раз в неделю.
	requestLogBatchSize = 100

	// requestLogFlushInterval - через сколько неполная пачка всё равно уходит в базу.
	// Лента мониторинга обновляется раз в 5 секунд, поэтому две секунды задержки
	// на экране незаметны.
	requestLogFlushInterval = 2 * time.Second

	// requestLogQueueSize - ёмкость очереди между обработчиками и писателем. Запас
	// на всплеск: при 100 записях в пачке очередь держит два десятка пачек.
	requestLogQueueSize = 2048

	// RequestLogShutdownGrace - сколько main.go ждёт, пока писатель сольёт остаток
	// очереди при остановке сервера.
	RequestLogShutdownGrace = 5 * time.Second
)

// requestLogSkipPaths - адреса, которые не попадают в журнал обращений при успешном
// ответе. Сюда идёт то, что вызывается не действием человека:
//   - /api/search - подсказки сквозного поиска летят на каждый введённый символ и в
//     /admin/requests вытеснили бы всё остальное, ради чего журнал и заведён;
//   - /health - проба готовности, которую дёргает docker и внешний монитор. На стенде
//     это 87 тысяч записей, четверть журнала, и все одинаковые.
//
// Отказы всё равно записываются - см. skipRequestLog. Упавший healthcheck это как раз
// то, что нужно видеть при разборе инцидента, и таких записей единицы.
var requestLogSkipPaths = []string{"/api/search", "/health"}

// requestLogSkipPrefixes - разделы, которые не пишутся целиком, вместе с подпутями.
// Страница мониторинга опрашивает себя сама: лента раз в 5 секунд, график раз в 30, и
// каждая открытая вкладка добавляет свой поток. Журнал, наполовину состоящий из чтения
// журнала, бесполезен.
//
// Совпадение проверяется как «сам раздел или что-то под ним», не как обычный префикс
// строки: иначе замолчал бы и соседний адрес вроде /api/request-logs-summary, причём
// незаметно - пропажу записей в журнале никто не заметит.
var requestLogSkipPrefixes = []string{"/api/request-logs"}

// skipRequestLog решает, пропускать ли запись. Успешный технический запрос не логируем,
// а вот отказ или ошибку - да: именно они интересны при разборе инцидента, и их немного.
// Обращение к персональным данным в поиске это не скрывает: его пишет отдельный
// журнал 152-ФЗ (PDAudit), у которого свой перечень путей.
func skipRequestLog(path string, status int) bool {
	if status >= 400 {
		return false
	}
	for _, p := range requestLogSkipPaths {
		if path == p {
			return true
		}
	}
	for _, p := range requestLogSkipPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// secretQueryKeys - параметры, значение которых нельзя писать в журнал. Билет
// скачивания и подписки на события даёт доступ к данным без заголовка Authorization:
// он одноразовый и живёт меньше минуты, но журнал обращений хранится месяцами, его
// читают через интерфейс и выгружают - секрету там не место. Особенно у файлового
// архива, где билет открывает выгрузку бланков с паспортами (#1615).
var secretQueryKeys = []string{"ticket", "token", "access_token", "key"}

// maskSecretQuery отдаёт адрес запроса с затёртыми значениями секретных параметров.
// Сам факт «пришёл с билетом» в журнале остаётся - пропадает только значение.
func maskSecretQuery(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	masked := false
	for _, key := range secretQueryKeys {
		if q.Has(key) {
			q.Set(key, "***")
			masked = true
		}
	}
	if !masked {
		return u.String()
	}

	clone := *u
	clone.RawQuery = q.Encode()
	return clone.String()
}

// RequestLogWriter копит записи журнала и кладёт их в базу пачками. Создаётся один раз
// на процесс, останавливается через Shutdown вместе с HTTP-сервером.
type RequestLogWriter struct {
	db    *gorm.DB
	queue chan models.RequestLogs

	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	stopped  atomic.Bool

	// dropped - записи, не попавшие в очередь: писатель не успевал за нагрузкой.
	// Считаем и показываем в логе, а не теряем молча.
	dropped  atomic.Int64
	reported int64

	batchSize     int
	flushInterval time.Duration
}

// RequestLogWriterOption настраивает писателя при создании.
type RequestLogWriterOption func(*RequestLogWriter)

// WithRequestLogBatch задаёт размер пачки и срок, после которого неполная пачка всё
// равно уходит в базу. Нужна тестам, чтобы не ждать штатные две секунды.
func WithRequestLogBatch(size int, interval time.Duration) RequestLogWriterOption {
	return func(w *RequestLogWriter) {
		if size > 0 {
			w.batchSize = size
		}
		if interval > 0 {
			w.flushInterval = interval
		}
	}
}

// NewRequestLogWriter поднимает писателя и его фоновую горутину.
func NewRequestLogWriter(db *gorm.DB, opts ...RequestLogWriterOption) *RequestLogWriter {
	w := &RequestLogWriter{
		db:            db,
		queue:         make(chan models.RequestLogs, requestLogQueueSize),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		batchSize:     requestLogBatchSize,
		flushInterval: requestLogFlushInterval,
	}
	for _, opt := range opts {
		opt(w)
	}
	go w.run()
	return w
}

// Enqueue кладёт готовую запись в очередь, минуя HTTP-слой. Отдельный метод нужен
// тестам записи: сам путь через Middleware проверяется отдельно.
func (w *RequestLogWriter) Enqueue(entry models.RequestLogs) {
	w.enqueue(entry)
}

// Middleware записывает HTTP-запросы в таблицу request_logs.
func (w *RequestLogWriter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now().UTC()

			err := next(c)

			if skipRequestLog(c.Request().URL.Path, c.Response().Status) {
				return err
			}

			elapsed := time.Since(start)

			method := c.Request().Method
			reqURL := maskSecretQuery(c.Request().URL)
			status := c.Response().Status
			durationMs := int(elapsed.Milliseconds())
			durationUs := elapsed.Microseconds()

			var userID *int
			var username *string
			if uid, ok := c.Get("user_id").(int); ok && uid > 0 {
				userID = &uid
			}
			if uname, ok := c.Get("username").(string); ok && uname != "" {
				username = &uname
			}

			w.enqueue(models.RequestLogs{
				UserID:         userID,
				Username:       username,
				Method:         &method,
				URL:            &reqURL,
				ResponseStatus: &status,
				DurationMs:     &durationMs,
				DurationUs:     &durationUs,
				CreatedAt:      start,
			})

			return err
		}
	}
}

// Shutdown останавливает писателя и ждёт, пока принятые записи лягут в базу.
// Вызывается после e.Shutdown: новых запросов уже нет, остаётся слить очередь.
func (w *RequestLogWriter) Shutdown(ctx context.Context) {
	w.stopped.Store(true)
	w.stopOnce.Do(func() { close(w.stop) })

	select {
	case <-w.done:
		slog.Info("журнал обращений: очередь записана до остановки сервера")
	case <-ctx.Done():
		slog.Warn("журнал обращений: остановка не дождалась записи очереди - часть обращений в журнал не попадёт")
	}
}

// enqueue кладёт запись в очередь, не блокируя обработку запроса. Журнал обращений
// вспомогательный: заставлять пользователя ждать вставку в него нельзя, поэтому при
// переполнении очереди запись теряется, но не молча - счётчик уходит в лог.
func (w *RequestLogWriter) enqueue(entry models.RequestLogs) {
	if w.stopped.Load() {
		w.dropped.Add(1)
		return
	}
	select {
	case w.queue <- entry:
	default:
		w.dropped.Add(1)
	}
}

// run - фоновый писатель: копит пачку и отдаёт её в базу по наполнению или по таймеру.
func (w *RequestLogWriter) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]models.RequestLogs, 0, w.batchSize)
	for {
		select {
		case entry := <-w.queue:
			batch = append(batch, entry)
			if len(batch) >= w.batchSize {
				batch = w.flush(batch)
			}
		case <-ticker.C:
			batch = w.flush(batch)
		case <-w.stop:
			batch = w.drain(batch)
			w.flush(batch)
			return
		}
	}
}

// drain добирает из очереди всё, что успели принять до остановки. Записи уже отданы
// обработчиками и потерять их на выходе - то же самое, что не записать вовсе.
func (w *RequestLogWriter) drain(batch []models.RequestLogs) []models.RequestLogs {
	for {
		select {
		case entry := <-w.queue:
			batch = append(batch, entry)
			if len(batch) >= w.batchSize {
				batch = w.flush(batch)
			}
		default:
			return batch
		}
	}
}

// flush пишет пачку одним запросом и возвращает опустошённый буфер.
func (w *RequestLogWriter) flush(batch []models.RequestLogs) []models.RequestLogs {
	w.reportDropped()
	if len(batch) == 0 {
		return batch
	}

	// Отдельный context: request-context обработчика давно отменён, а при остановке
	// сервера отведённый на слив срок держит main.go.
	ctx, cancel := context.WithTimeout(context.Background(), requestLogWriteTimeout)
	defer cancel()

	if err := w.db.WithContext(ctx).CreateInBatches(batch, w.batchSize).Error; err != nil {
		slog.Error("журнал обращений: пачка не записана", "error", err, "count", len(batch))
	}
	return batch[:0]
}

// reportDropped сообщает о потерянных записях один раз на пачку, а не на каждую
// потерю: при всплеске нагрузки счётчик растёт тысячами, и построчный лог сам стал бы
// нагрузкой.
func (w *RequestLogWriter) reportDropped() {
	total := w.dropped.Load()
	if total == w.reported {
		return
	}
	slog.Warn("журнал обращений: очередь переполнена, записи отброшены",
		"dropped_total", total, "dropped_since_last", total-w.reported)
	w.reported = total
}
