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

// isPDPath отвечает, ведёт ли запрос к персональным данным. Кроме перечня префиксов
// сюда попадают два адреса с идентификатором в середине: выгрузка бланка (один .xlsx
// уносит ФИО, паспорта и патенты всех сотрудников заявки) и деталь доступного
// вложения, где охрана видит те же паспортные данные.
func isPDPath(path string) bool {
	for _, p := range pdPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return isBlankPath(path) || isAvailableAttachmentPath(path) || isApplicationArchivePath(path) ||
		isApplicationFilePath(path) || isApplicationParticipantsPath(path)
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
