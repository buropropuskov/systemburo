package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Пути, на которых пользователь видит персональные данные (152-ФЗ). Префикс /api
// обязателен: роутер вешает всё на api := e.Group("/api"), а nginx проксирует без
// среза префикса - без него сверка не совпадала ни разу и журнал стоял пустым (#1472).
var pdPaths = []string{
	"/api/employees", "/api/unique-employees", "/api/attachments",
	// Сводка сбора согласий (#1567) отдаёт поимённый список работников с
	// организациями. Это просмотр персональных данных, пусть и в агрегированном
	// виде, - обращение к нему должно попадать в журнал наравне с прочими.
	"/api/settings/pd-consent/collection",
	// Сквозной поиск возвращает группу сотрудников: ФИО, должность, организация.
	// Данные те же, что в реестре, но вход другой, и без этой строки появился бы
	// способ смотреть персональные данные мимо журнала - ровно та дыра, которую
	// закрывали в #1472, только с другой стороны. Пишем весь поиск, а не только
	// запросы с сотрудниками в выдаче: middleware не разбирает тело ответа, а
	// "запрос был, совпадений не нашлось" - тоже сведения о людях в системе.
	"/api/search",
	// Выгрузка из файлового архива (#1615): один ZIP за период уносит бланки сотен
	// заявок, а в каждом бланке паспорта и патенты открытым текстом. Это самый
	// массовый вынос персональных данных из системы, и он обязан быть в журнале.
	// Префиксом закрыты все входы разом - и поштучный файл, и список, и оценка
	// объёма, и выдача билета; сам поток байтов идёт мимо JWT по одноразовому
	// билету (/api/file-archive/download), поэтому важно, что middleware смотрит
	// на путь, а не на авторизацию.
	"/api/file-archive/download",
	"/api/file-archive/files",
	"/api/file-archive/items",
	"/api/file-archive/estimate",
	// Выгрузка реестра заявок (#1832): один файл уносит ФИО заявителей и
	// принимающих по всей выборке за период. Данные те же, что в списке Центра, но
	// вход другой и объём разовый, поэтому обращение обязано попадать в журнал -
	// как сводка согласий и выгрузка файлового архива выше.
	"/api/applications/export",
	// Выгрузка журнала обращений (#2125): один файл уносит адреса запросов сотен
	// работников за период. Значения параметров в нём затёрты по белому списку, то
	// есть ФИО и номера заявок в файл не попадают, - но сам факт «кто когда куда
	// обращался» остаётся сведениями о людях, и снятие его пачкой владелец решил
	// считать просмотром персональных данных наравне с выгрузкой реестра заявок.
	"/api/request-logs/export",
	// Список пользователей (#2352): та же дыра, что закрывали в #1472, только с
	// другой стороны формы - отдаёт ФИО, должность, почту, телефон и организацию
	// работника. Обнаружено на стенде: запрос к этому пути не сдвигал счётчик
	// журнала, а к /api/unique-employees - сдвигал.
	"/api/users/all",
	// Реестр автомобилей (#2352): зеркало уже учтённого /api/unique-employees, только
	// для машин. UserName в ответе - ФИО владельца записи (отдаётся только
	// администратору, см. maskCarOwners), то есть привязка номера машины к
	// конкретному человеку, а не только номер сам по себе.
	"/api/unique-cars",
	// Кандидаты в получатели заявки (#2352): models.RecipientCandidate несёт ФИО и
	// должность коллег по организации/компании - тот же набор полей, что и в
	// /api/users/all, только для окна пересылки, доступного любому авторизованному.
	"/api/users/recipient-candidates",
}

// auditWriteTimeout - максимальное время на запись лога в БД. Если БД легла или
// горутина зависла, не блокируем graceful shutdown навсегда.
const auditWriteTimeout = 5 * time.Second

func PDAudit(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			path := c.Request().URL.Path
			if !isPDPath(path) {
				return err
			}

			// Снапшот данных запроса ДО горутины: c.Response()/c.RealIP() могут
			// быть невалидны после возврата из handler-а (data race).
			username, _ := c.Get("username").(string)
			userID := pdUserID(c)
			method := c.Request().Method
			ip := c.RealIP()
			statusCode := pdStatusCode(c, err)

			go func() {
				// Отдельный context с таймаутом, не привязанный к request-context
				// (тот отменится сразу после ответа). Защита от висящих горутин
				// при медленной БД.
				ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
				defer cancel()
				log := models.PDAuditLog{
					UserID:     userID,
					Username:   username,
					Action:     methodToAction(method),
					Resource:   pathToResource(path),
					IPAddress:  ip,
					Method:     method,
					Path:       path,
					StatusCode: statusCode,
				}
				if err := db.WithContext(ctx).Create(&log).Error; err != nil {
					slog.Error("failed to write PD audit log", "error", err, "path", path)
				}
			}()

			return err
		}
	}
}

// IsPDPathForRouteCoverage экспортирует isPDPath ради гвард-теста
// TestPDAudit_NoUntriagedGetRoute в internal/handlers: тот сверяет разбор пути с
// реальным роутером (e.Routes()), а строить роутер здесь, в internal/middleware,
// означало бы импортировать internal/testutil - тот и так зависит от middleware
// (тот же приём, что у mw.PDConsentWhitelist в pd_consent.go).
func IsPDPathForRouteCoverage(path string) bool {
	return isPDPath(path)
}

// isPDPath отвечает, ведёт ли запрос к персональным данным. Кроме перечня префиксов
// сюда попадают адреса с идентификатором в середине пути (заявка/id/подраздел,
// пользователь/username/history и т.п.) - строкой в pdPaths их не описать, поэтому
// каждый такой случай разобран отдельной функцией ниже (#1472, #2352).
func isPDPath(path string) bool {
	for _, p := range pdPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return isBlankPath(path) || isAvailableAttachmentPath(path) || isApplicationArchivePath(path) ||
		isApplicationFilePath(path) || isApplicationParticipantsPath(path) ||
		isApplicationListPath(path) || isApplicationDetailPath(path) || isApplicationCardSubPath(path) ||
		isUserHistoryPath(path) || isCarHistoryPath(path) ||
		isSystemTableTrashListPath(path) || isSystemTableSnapshotPayloadPath(path) ||
		isSystemTableHistoryPath(path)
}

// isApplicationListPath - три списка заявок, построенных на одной и той же выборке
// (ApplicationWithDetails): Центр (/api/applications), ЛК (/api/applications/user) и
// список для ручного вложения без формы (/api/applications/attachable, #1049). Все
// три отдают sender_full_name и responsible_full_name - ФИО инициатора и принимающего,
// то самое "имя инициатора", из-за которого завели #2352.
func isApplicationListPath(path string) bool {
	switch path {
	case "/api/applications", "/api/applications/user", "/api/applications/attachable":
		return true
	}
	return false
}

// isApplicationDetailPath - карточка одной заявки (/api/applications/{id}): те же
// sender_full_name/responsible_full_name, что и в списке. Проверка "весь остаток -
// цифры" отделяет числовой id от статических соседей на этом же уровне маршрутизации
// (export, user, attachable, unread-count...), которые персональных данных не отдают.
func isApplicationDetailPath(path string) bool {
	const prefix = "/api/applications/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := path[len(prefix):]
	return rest != "" && isDigits(rest)
}

// applicationCardSuffixes - подпути карточки заявки, где JOIN users отдаёт ФИО:
// инициатора и принимающего (history, details - как в самой карточке), автора
// пересылки (forward-messages), согласующего (responsible-users), смотревшего
// (viewers, reads), автора вопроса или дополнения (questions, supplements). Тот же
// человек, что виден в участниках (закрыто в #1472), только с другой стороны формы.
var applicationCardSuffixes = []string{
	"/history", "/responsible-users", "/forward-messages", "/viewers", "/reads",
	"/questions", "/supplements", "/details",
}

// isApplicationCardSubPath - .../applications/{id}/{суффикс из applicationCardSuffixes}.
func isApplicationCardSubPath(path string) bool {
	const prefix = "/api/applications/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := path[len(prefix):]
	for _, suf := range applicationCardSuffixes {
		id, ok := strings.CutSuffix(rest, suf)
		if ok && id != "" && isDigits(id) {
			return true
		}
	}
	return false
}

// isUserHistoryPath - история изменений учётки (/api/users/{username}/history):
// models.UserHistoryItem несёт ActorName - ФИО того, кто менял запись, тот же
// принцип, что и DeletedByName в корзине таблиц ниже.
func isUserHistoryPath(path string) bool {
	const prefix = "/api/users/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return strings.HasSuffix(path, "/history")
}

// isCarHistoryPath - подпути истории машин, где CarHistoryItemResponse и
// AllCarsHistoryItem несут ФИО охранника, менявшего территориальный статус. Текущий
// статус (history/current-status) сюда НЕ входит - там только car_id/статус/время,
// без единого имени, а сам номер и марка машины субъекта не идентифицируют (см.
// комментарий у UniqueCar.PDConsentAt) - поэтому остальные пути /cars
// (active-for-table, unload-places и т.п.) в журнал тоже не идут.
func isCarHistoryPath(path string) bool {
	switch path {
	case "/api/cars/history/all", "/api/cars/history/unified":
		return true
	// Значения выпадающих списков журнала (#2469) - это перечень тех, кто отмечал
	// проходы, то есть те же ФИО охранников, что и в самой истории. Вход другой,
	// данные те же, и без этой строки появился бы способ прочитать их мимо журнала.
	case "/api/cars/history/filter-options":
		return true
	}
	const prefix = "/api/cars/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := path[len(prefix):]
	if strings.HasPrefix(rest, "history/table/") {
		return true
	}
	return strings.HasSuffix(path, "/history")
}

// isSystemTableTrashListPath - список и журнал корзины таблицы поста (#186):
// models.TrashItem несёт ФИО/номер машины удалённой записи и DeletedByName - кто её
// удалил. Восстановление и очистка (POST/DELETE) отдают только счётчик, а не сами
// записи, - персональных данных не показывают, поэтому в список не идут.
func isSystemTableTrashListPath(path string) bool {
	const prefix = "/api/system-tables/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return strings.HasSuffix(path, "/trash") || strings.HasSuffix(path, "/trash/history")
}

// isSystemTableSnapshotPayloadPath - одна версия слепка таблицы с полным payload
// (машины/люди со статусами, #980) и файл её выгрузки. Список версий (без /{sid})
// отдаёт только метаданные - дату, автора, агрегаты, - без единой строки содержимого,
// поэтому в журнал не идёт.
func isSystemTableSnapshotPayloadPath(path string) bool {
	const prefix = "/api/system-tables/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return strings.Contains(path[len(prefix):], "/snapshots/")
}

// isSystemTableHistoryPath - история изменений СТРУКТУРЫ таблицы поста
// (/api/system-tables/{id}/history, #345): models.SystemTableHistoryItem несёт
// UserName - ФИО администратора, переименовавшего таблицу или менявшего её настройки.
// Не путать с содержимым таблицы (isSystemTableTrashListPath/
// isSystemTableSnapshotPayloadPath выше) - там ФИО того, кто на ней проходит, здесь -
// того, кто её настраивал.
func isSystemTableHistoryPath(path string) bool {
	const prefix = "/api/system-tables/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	id, ok := strings.CutSuffix(path[len(prefix):], "/history")
	return ok && id != "" && isDigits(id)
}

// isDigits отвечает, состоит ли строка целиком из цифр - способ отличить числовой
// id пути от статического сегмента-соседа (export, user, history...).
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isApplicationParticipantsPath - состав участников заявки
// (/api/applications/{id}/participants). Метод отдаёт рабочие контакты каждого:
// почту и телефон. Это те же сведения о людях, что в реестре работников, только
// вход другой - через карточку заявки, - и без этой строки появился бы способ
// собирать контакты мимо журнала, как это было со сквозным поиском до #1472.
func isApplicationParticipantsPath(path string) bool {
	return strings.HasPrefix(path, "/api/applications/") && strings.HasSuffix(path, "/participants")
}

// isApplicationFilePath - файлы, приложенные к заявке (#1721): /api/applications/{id}/files
// и скачивание конкретного файла. Поле общее, «прикрепите документы», и что там
// лежит, система заранее не знает: заявитель кладёт туда разрешение на работу, а
// то и скан паспорта, хотя это запрещено подписью поля. Раз содержимое
// непредсказуемо, обращения считаются просмотром персональных данных.
func isApplicationFilePath(path string) bool {
	const prefix = "/api/applications/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := path[len(prefix):]
	idx := strings.Index(rest, "/files")
	return idx > 0
}

// isApplicationArchivePath - ZIP сохранённых бланков одной заявки
// (/api/applications/:id/archive). Внутри те же паспорта и патенты, что в бланке,
// плюс машиночитаемый слепок заявки со всеми участниками - в журнале это обращение
// обязано быть наравне с одиночным бланком.
func isApplicationArchivePath(path string) bool {
	return strings.HasPrefix(path, "/api/applications/") && strings.HasSuffix(path, "/archive")
}

func isBlankPath(path string) bool {
	return strings.HasPrefix(path, "/api/applications/") && strings.HasSuffix(path, "/blank")
}

func isAvailableAttachmentPath(path string) bool {
	const prefix = "/api/applications/available-attachments/"
	return strings.HasPrefix(path, prefix) && len(path) > len(prefix)
}

// pdStatusCode возвращает код, который увидит клиент. Echo вызывает обработчик
// ошибок уже после цепочки middleware, поэтому у неудачного запроса Response().Status
// здесь ещё дефолтные 200 - и отказ в доступе попадал в журнал как успешный просмотр.
func pdStatusCode(c echo.Context, err error) int {
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Code != 0 {
		return he.Code
	}
	if err != nil && !c.Response().Committed {
		return http.StatusInternalServerError
	}
	return c.Response().Status
}

// pdUserID достаёт идентификатор пользователя из контекста JWT: по одному имени
// запись не привязать к учётке, если пользователя переименовали или архивировали.
func pdUserID(c echo.Context) *int {
	id, ok := c.Get("user_id").(int)
	if !ok || id == 0 {
		return nil
	}
	return &id
}

func methodToAction(method string) string {
	switch method {
	case "GET":
		return "view"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return method
	}
}

func pathToResource(path string) string {
	switch {
	case isBlankPath(path):
		return "attachment_blank"
	case isAvailableAttachmentPath(path):
		return "available_attachment"
	case isApplicationArchivePath(path):
		return "application_archive"
	case isApplicationFilePath(path):
		return "application_file"
	case isApplicationParticipantsPath(path):
		return "application_participants"
	// Список (Центр/ЛК/ручное вложение), деталь и подпути карточки (история,
	// ответственные, пересылки, просмотры, вопросы, дополнения) - один резон
	// "application": для фильтра в журнале это всё "просмотр карточки заявки",
	// дробить на восемь строк смысла не добавляет, а RESOURCE_LABELS во фронте
	// (frontend/src/views/admin/PdAuditLog.vue, TestPDResourceLabeledOnScreen)
	// пришлось бы разрастить на столько же новых подписей.
	case isApplicationCardSubPath(path), isApplicationListPath(path), isApplicationDetailPath(path):
		return "application"
	case isUserHistoryPath(path):
		return "user"
	case isCarHistoryPath(path):
		return "car"
	case isSystemTableTrashListPath(path), isSystemTableSnapshotPayloadPath(path), isSystemTableHistoryPath(path):
		return "system_table_content"
	case strings.HasPrefix(path, "/api/users/all"), strings.HasPrefix(path, "/api/users/recipient-candidates"):
		return "user"
	case strings.HasPrefix(path, "/api/unique-cars"):
		return "unique_car"
	case strings.HasPrefix(path, "/api/unique-employees"):
		return "unique_employee"
	case strings.HasPrefix(path, "/api/employees"):
		return "employee"
	case strings.HasPrefix(path, "/api/attachments"):
		return "attachment"
	case strings.HasPrefix(path, "/api/settings/pd-consent/collection"):
		return "pd_consent_collection"
	case strings.HasPrefix(path, "/api/file-archive/"):
		// Один вид ресурса на все входы выгрузки: разбирать в журнале «список» и
		// «сам ZIP» незачем, отвечать по 152-ФЗ придётся за факт выноса бланков.
		return "file_archive"
	case strings.HasPrefix(path, "/api/applications/export"):
		return "applications_export"
	case strings.HasPrefix(path, "/api/request-logs/export"):
		return "request_logs_export"
	case strings.HasPrefix(path, "/api/search"):
		return "search"
	}
	return "unknown"
}
