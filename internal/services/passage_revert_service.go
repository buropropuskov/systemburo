package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// passageRevertWindow - сколько охранник может отменять собственную отметку.
// Константа, а не параметр окружения: новый env тянет за собой замок
// TestEnvWiring_AllConfigVarsReachContainer и правку docker-compose со скриптом
// окружения, а настраивать это окно никто не просил. Администратора окно не
// касается - он отменяет и позже.
const passageRevertWindow = 15 * time.Minute

// RevertPassageRequest - отмена ошибочной отметки прохода (#2437).
//
// Какую именно запись отменять, клиент не выбирает: отменяется последняя
// действительная отметка сущности. Строка таблицы её id не знает, а отмена
// какой-нибудь записи из середины журнала сделала бы бессмысленным откат статуса -
// человек остался бы «на территории» после отмены собственного входа.
//
// TerritoryStatus здесь - направление ОТМЕНЯЕМОЙ отметки, а не новое состояние.
// Его же читает гейт прав RequireTablePassVerb, поэтому отмена гейтится тем же
// правом таблицы, что и сама отметка, без отдельного глагола в каталоге.
type RevertPassageRequest struct {
	TerritoryStatus int    `json:"territory_status"`
	TableID         *int   `json:"table_id"`
	Reason          string `json:"reason"`
	// ActorUserID ставит сервер из токена, а не клиент из тела. У самой отметки
	// прохода автор приходит телом запроса, и повторять это здесь нельзя: на
	// авторстве держится всё правило «своя, последняя, свежая».
	ActorUserID int `json:"-"`
}

// passageRevertDetails - содержимое details записи отмены. reverts_id читают и
// источник истории, и уникальный индекс, запрещающий отменить одно дважды.
type passageRevertDetails struct {
	RevertsID int     `json:"reverts_id"`
	Comment   *string `json:"comment,omitempty"`
	TableID   *int    `json:"table_id,omitempty"`
	// Subject - снимок «о ком отметка» (номер с маркой или ФИО), как в самой отметке:
	// сторно тоже остаётся в журнале после удаления строки справочника (#2485).
	Subject *string `json:"subject,omitempty"`
}

// passageRow - отметка прохода в том виде, в каком её читает механизм отмены.
type passageRow struct {
	ID          int
	Action      string
	ActorUserID *int
	TableID     *int
	CreatedAt   time.Time
}

// passageEntityTable - таблица сущности, чей территориальный статус откатывается.
var passageEntityTable = map[string]string{
	models.AuditEntityCar:      "cars",
	models.AuditEntityEmployee: "employees",
}

// passageActionFor переводит направление в действие журнала.
func passageActionFor(territoryStatus int) (string, error) {
	switch territoryStatus {
	case 1:
		return "entry", nil
	case 2:
		return "exit", nil
	default:
		return "", echo.NewHTTPError(http.StatusBadRequest, "Неизвестное направление прохода")
	}
}

// passageRevertActionFor - действие отмены, парное действию отметки.
func passageRevertActionFor(action string) string {
	if action == "entry" {
		return models.AuditActionEntryRevert
	}
	return models.AuditActionExitRevert
}

// lastLivePassage возвращает последнюю НЕ отменённую отметку сущности.
//
// Вызывается дважды: до записи отмены - чтобы понять, что отменяем, и после - чтобы
// пересчитать статус. Второй раз он уже видит собственную запись отмены (та же
// транзакция) и отдаёт предыдущую отметку, поэтому отдельного «исключить эту»
// параметра не нужно.
func lastLivePassage(ctx context.Context, tx *gorm.DB, entityType string, entityID int, actions ...string) (*passageRow, error) {
	if len(actions) == 0 {
		actions = []string{"entry", "exit"}
	}
	var row passageRow
	err := tx.WithContext(ctx).
		Table("audit_log a").
		Select("a.id, a.action, a.actor_user_id, (a.details->>'table_id')::int AS table_id, a.created_at").
		Where("a.entity_type = ? AND a.entity_id = ?", entityType, entityID).
		Where("a.action IN ?", actions).
		Where(passageRevertNotExists("a")).
		Order("a.created_at DESC, a.id DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load last passage: %w", err)
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

// revertPassage отменяет последнюю отметку прохода сущности и откатывает её
// территориальный статус. Общая часть для машины и человека: различаются они
// только таблицей и тем, кого оповестить об изменении строки.
func revertPassage(ctx context.Context, db *gorm.DB, recorder AuditRecorder, now func() time.Time,
	entityType string, entityID int, req RevertPassageRequest) error {
	table, ok := passageEntityTable[entityType]
	if !ok {
		return fmt.Errorf("revert passage: неизвестный тип сущности %q", entityType)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Укажите причину отмены отметки")
	}
	action, err := passageActionFor(req.TerritoryStatus)
	if err != nil {
		return err
	}
	if req.ActorUserID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}

	admin, err := isPassageRevertAdmin(ctx, db, req.ActorUserID)
	if err != nil {
		return err
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		last, err := lastLivePassage(ctx, tx, entityType, entityID)
		if err != nil {
			return err
		}
		if last == nil {
			return echo.NewHTTPError(http.StatusConflict, "Отменять нечего: отметок прохода нет")
		}
		// Направление разошлось - строку успели отметить заново, пока открыт экран.
		// Молча отменить «то, что есть» нельзя: охранник целился в другое событие.
		if last.Action != action {
			return echo.NewHTTPError(http.StatusConflict,
				"Отметка изменилась, обновите таблицу и повторите")
		}
		// Пост отменяемой отметки обязан совпасть с постом, с которого пришёл запрос.
		// Право проверяется по table_id из тела, и без этой сверки охранник своего КПП
		// мог бы отменить отметку, поставленную на соседнем: право у него есть, а
		// отношения к тому проходу никакого. Записи без поста (до #1036) не сверяем.
		if last.TableID != nil && req.TableID != nil && *last.TableID != *req.TableID {
			return echo.NewHTTPError(http.StatusConflict,
				"Отметка поставлена на другом посту, отменять её нужно там же")
		}
		if !admin {
			if last.ActorUserID == nil || *last.ActorUserID != req.ActorUserID {
				return echo.NewHTTPError(http.StatusForbidden,
					"Отметку поставил другой пользователь, отменить её может администратор")
			}
			if now().Sub(last.CreatedAt) > passageRevertWindow {
				return echo.NewHTTPError(http.StatusForbidden,
					"Отменить свою отметку можно в течение 15 минут, дальше - через администратора")
			}
		}

		details := passageRevertDetails{RevertsID: last.ID, Comment: &reason, TableID: req.TableID}
		if subject := passageEntitySubject(ctx, tx, entityType, entityID); subject != "" {
			details.Subject = &subject
		}
		if err := recorder.Record(ctx, tx, entityType, &entityID,
			passageRevertActionFor(last.Action), &req.ActorUserID, details); err != nil {
			// Уникальный индекс по reverts_id: параллельная отмена той же отметки
			// успела первой. Для человека это не сбой, а «уже отменено».
			if isUniqueViolation(err) {
				return echo.NewHTTPError(http.StatusConflict, "Эта отметка уже отменена")
			}
			return fmt.Errorf("failed to record passage revert: %w", err)
		}

		return rollbackTerritoryStatus(ctx, tx, table, entityType, entityID, now())
	})
	if err != nil {
		return err
	}
	slog.Info("отметка прохода отменена", "entity", entityType, "entity_id", entityID,
		"actor_user_id", req.ActorUserID, "by_admin", admin)
	return nil
}

// rollbackTerritoryStatus приводит строку к состоянию до отменённой отметки.
//
// Состояние считается по журналу, а не по снимку, сохранённому в момент отмены:
// снимок разъезжается с историей, как только отменяют вторую отметку подряд, а
// журнал остаётся единственным источником правды и после цепочки отмен.
func rollbackTerritoryStatus(ctx context.Context, tx *gorm.DB, table, entityType string,
	entityID int, now time.Time) error {
	prev, err := lastLivePassage(ctx, tx, entityType, entityID)
	if err != nil {
		return err
	}
	var status *int
	if prev != nil {
		v := 2
		if prev.Action == "entry" {
			v = 1
		}
		status = &v
	}

	// Время входа берётся от последнего действительного ВХОДА, а не от prev: после
	// отмены выхода prev как раз и есть вход, а после отмены входа предыдущим может
	// оказаться выход, у которого своего времени входа нет.
	lastEntry, err := lastLivePassage(ctx, tx, entityType, entityID, "entry")
	if err != nil {
		return err
	}
	var entryTime *time.Time
	if lastEntry != nil {
		entryTime = &lastEntry.CreatedAt
	}

	if err := tx.WithContext(ctx).Table(table).Where("id = ?", entityID).
		Updates(map[string]interface{}{
			"territory_status":     status,
			"territory_entry_time": entryTime,
			"updated_at":           now,
		}).Error; err != nil {
		return fmt.Errorf("failed to roll back territory status: %w", err)
	}
	return nil
}

// isPassageRevertAdmin отвечает, снимается ли с пользователя ограничение «своя,
// последняя, свежая». Флаги читаются из строки пользователя, а не через
// PermissionResolver: резолвер живёт в middleware и в сервисы машин и людей не
// проведён, а заводить ради одного признака новое ребро графа зависимостей дороже,
// чем один запрос. Забаненный админ сюда не проходит намеренно.
func isPassageRevertAdmin(ctx context.Context, db *gorm.DB, userID int) (bool, error) {
	var allowed bool
	err := db.WithContext(ctx).
		Table("users").
		Select("(is_super_admin OR is_admin) AND is_active AND NOT is_banned").
		Where("id = ?", userID).
		Scan(&allowed).Error
	if err != nil {
		return false, fmt.Errorf("failed to resolve revert rights: %w", err)
	}
	return allowed, nil
}
