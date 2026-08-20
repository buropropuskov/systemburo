package middleware

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"systemburo/internal/logmask"
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
// ответе. Сюда идёт только то, что вызывается не действием человека, а таймером:
//   - /api/search - подсказки сквозного поиска летят на каждый введённый символ и в
//     /admin/requests вытеснили бы всё остальное, ради чего журнал и заведён;
//   - /health - проба готовности, которую дёргает docker и внешний монитор. На стенде
//     это 87 тысяч записей, четверть журнала, и все одинаковые;
//   - три адреса раздела мониторинга - страница опрашивает себя сама: ленту раз в пять
//     секунд, график и метрики шапки раз в тридцать, и каждая открытая вкладка добавляет
//     свой поток. Журнал, наполовину состоящий из чтения журнала, бесполезен.
//
// Остальные адреса раздела (список, выгрузка, история, справочник пользователей) человек
// вызывает руками, и они пишутся: кто и когда выгрузил журнал обращений - как раз то,
// что нужно при разборе.
//
// Сравнение точное, а не по префиксу: префикс заглушил бы и будущий соседний адрес вроде
// /api/search-history, причём молча - пропажу записей в журнале никто не заметит.
// Отказы всё равно записываются - см. skipRequestLog.
var requestLogSkipPaths = []string{
	"/api/search",
	"/health",
	"/api/request-logs/realtime",
	"/api/request-logs/timeline",
	"/api/request-logs/stats",
}

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
	return false
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
	// stopCtx - срок, отведённый на слив очереди при остановке. Пишется до закрытия
	// stop и читается писателем после того, как тот увидел закрытие, поэтому лишней
	// синхронизации не нужно. Без него писатель мог бы возиться с недоступной базой
	// дольше, чем main готов его ждать, и остаток очереди уходил бы вместе с процессом
	// молча.
	stopCtx context.Context

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
// тестам записи: сам путь через Middleware проверяется отдельно. Появится второй
// источник записей - см. условие в Shutdown: он рассчитывает, что к моменту слива
// подавать уже некому.
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
			reqURL := logmask.Query(c.Request().URL)
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
// Вызывается после e.Shutdown, и это условие обязательное: слив вычерпывает очередь до
// пустой, поэтому запись, поданная одновременно со сливом, может не попасть ни в базу,
// ни в счётчик потерь. Пока единственный источник записей - HTTP-обработчики, к этому
// моменту уже завершённые.
func (w *RequestLogWriter) Shutdown(ctx context.Context) {
	w.stopped.Store(true)
	w.stopOnce.Do(func() {
		w.stopCtx = ctx
		close(w.stop)
	})

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

// Dropped - сколько записей не попало в очередь с момента запуска. Нужен диагностике и
// тестам: потеря журнала иначе видна только в логе.
func (w *RequestLogWriter) Dropped() int64 {
	return w.dropped.Load()
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
		// Отведённый на остановку срок вышел: дальше вычерпывать бессмысленно, база
		// всё равно не примет. Что осталось в очереди - потеряно, и об этом скажет
		// ошибка последней пачки.
		if w.stopCtx != nil && w.stopCtx.Err() != nil {
			return batch
		}
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

	// Отдельный context: request-context обработчика давно отменён. При остановке
	// сервера отсчёт идёт от срока, отведённого на слив, - иначе пачки уходили бы по
	// пять секунд каждая и переживали бы то время, что main готов ждать.
	parent := context.Background()
	if w.stopCtx != nil {
		parent = w.stopCtx
	}
	ctx, cancel := context.WithTimeout(parent, requestLogWriteTimeout)
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
