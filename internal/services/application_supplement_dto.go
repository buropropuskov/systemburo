package services

// Признак дополнения в ответах API (#1685). До этого файла отличить строку, добавленную
// дополнением, от исходного состава подачи снаружи было нечем: и то и другое приезжало
// одинаковыми записями вложения, и интерфейс не мог ни подсветить новое, ни показать, что
// по заявке идёт повторный круг.

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// SupplementMark - принадлежность строки состава вложения раунду дополнения (#1685).
// Встраивается в DTO машин, сотрудников и ТМЦ: набор полей у всех трёх один, а три копии
// разъехались бы на первой же правке.
type SupplementMark struct {
	// SupplementID - раунд, принёсший строку. nil - исходный состав подачи.
	SupplementID *int `json:"supplement_id"`
	// SupplementNumber - номер раунда для подписи «Дополнение №2». nil вместе с SupplementID.
	SupplementNumber *int `json:"supplement_number"`
	// SupplementStatus - статус раунда (models.Supplement*). nil вместе с SupplementID.
	SupplementStatus *string `json:"supplement_status"`
	// IsPending - строка ещё не допущена на КПП.
	IsPending bool `json:"is_pending"`
}

// supplementMark собирает признак дополнения для строки состава.
//
// IsPending - ровно отрицание admittedSupplementCond (application_supplement_guards.go):
// подсветка «ещё не принято» у автора и невидимость строки для охраны обязаны сходиться,
// иначе интерфейс покажет допущенным то, чего на проходной нет. Отсюда же трактовка
// неизвестного статуса: раунд есть, принятым не подтверждён - значит не допущен.
func supplementMark(id *int, number *int, status *string) SupplementMark {
	return SupplementMark{
		SupplementID:     id,
		SupplementNumber: number,
		SupplementStatus: status,
		// Влитая в основной круг добавка (merged) ожидающей не считается: отдельного
		// решения по ней не будет, она идёт вместе с составом заявки. Строгое отрицание
		// допуска - иначе карточка пообещает решение, которого никто не примет.
		IsPending: id != nil && (status == nil ||
			(*status != models.SupplementAccepted && *status != models.SupplementMerged)),
	}
}

// OpenSupplementInfo - незакрытый раунд дополнения в детали заявки (#1685): по нему карточка
// показывает, что идёт повторный круг, и сколько строк он принёс. Голоса согласующих сюда не
// входят - за ними отдельная ручка раундов (GetApplicationSupplements).
type OpenSupplementInfo struct {
	ID              int              `json:"id"`
	Number          int              `json:"number"`
	Status          string           `json:"status"`
	Comment         *string          `json:"comment"`
	CreatedByUserID int              `json:"created_by_user_id"`
	CreatedByName   string           `json:"created_by_name"`
	CreatedAt       time.Time        `json:"created_at"`
	Counts          SupplementCounts `json:"counts"`
}

// quotedSQLList собирает значения в SQL-литерал списка ('a', 'b'). Зовётся только на
// константах пакета models, снаружи в неё ничего не попадает.
func quotedSQLList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, "'"+v+"'")
	}
	return strings.Join(quoted, ", ")
}

// hasOpenSupplementPredicate - у заявки идёт незакрытый раунд дополнения. Условие само по
// себе, без AND: подставляется в SELECT листингов заявок как отдельный столбец.
//
// var, а не const: список статусов берётся из models.OpenSupplementStatuses, чтобы метка в
// списке и снятие раунда в cancelOpenSupplements считали открытым одно и то же.
var hasOpenSupplementPredicate = `EXISTS (SELECT 1 FROM application_supplements sup` +
	` WHERE sup.application_id = a.id AND sup.status IN (` + quotedSQLList(models.OpenSupplementStatuses) + `))`

// loadOpenSupplement возвращает открытый раунд заявки либо nil, если такого нет.
//
// Открытым раунд бывает максимум один (партиальный уникальный индекс
// uidx_app_supplement_open), поэтому берётся первый по убыванию номера, а не список.
func (s *applicationService) loadOpenSupplement(ctx context.Context, applicationID int, masks map[int]string) (*OpenSupplementInfo, error) {
	type openRow struct {
		ID              int
		Number          int
		Status          string
		Comment         *string
		CreatedByUserID int
		CreatedByName   string
		CreatedAt       time.Time
		Vehicles        int
		Employees       int
		Items           int
	}

	var rows []openRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			s.id, s.number, s.status, s.comment,
			s.created_by_user_id,
			-- NULLIF обязателен: format_full_name на пустых ФИО возвращает '', а не NULL,
			-- и без него COALESCE останавливается на пустой строке, не дойдя до логина.
			COALESCE(NULLIF(format_full_name(u.last_name, u.first_name, u.middle_name), ''), u.username, '') AS created_by_name,
			s.created_at,
			(SELECT COUNT(*) FROM cars c WHERE c.supplement_id = s.id) AS vehicles,
			(SELECT COUNT(*) FROM employees e WHERE e.supplement_id = s.id) AS employees,
			(SELECT COUNT(*) FROM items i WHERE i.supplement_id = s.id) AS items
		FROM application_supplements s
		LEFT JOIN users u ON u.id = s.created_by_user_id
		WHERE s.application_id = ? AND s.status IN (`+quotedSQLList(models.OpenSupplementStatuses)+`)
		ORDER BY s.number DESC
		LIMIT 1
	`, applicationID).Scan(&rows).Error
	if err != nil {
		slog.Error("дополнение: не удалось получить открытый раунд", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching open supplement")
	}
	if len(rows) == 0 {
		return nil, nil
	}

	r := rows[0]
	// Маска принимающего и логин вместо ФИО у не давших согласия на обработку ПД - тот же
	// слой, что в остальной детали заявки.
	name := maskName(masks, &r.CreatedByUserID, r.CreatedByName)
	return &OpenSupplementInfo{
		ID:              r.ID,
		Number:          r.Number,
		Status:          r.Status,
		Comment:         r.Comment,
		CreatedByUserID: r.CreatedByUserID,
		CreatedByName:   name,
		CreatedAt:       r.CreatedAt,
		Counts:          SupplementCounts{Vehicles: r.Vehicles, Employees: r.Employees, Items: r.Items},
	}, nil
}

// countSupplements возвращает число раундов дополнения заявки, включая закрытые: карточка
// по нему решает, показывать ли историю дополнений вообще.
func (s *applicationService) countSupplements(ctx context.Context, applicationID int) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM application_supplements WHERE application_id = ?", applicationID,
	).Scan(&count).Error
	if err != nil {
		slog.Error("дополнение: не удалось посчитать раунды", "application_id", applicationID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting supplements")
	}
	return int(count), nil
}
