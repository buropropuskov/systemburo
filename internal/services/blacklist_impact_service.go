package services

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// BlacklistImpactRow - одна строка, которую затронет внесение в чёрный список.
type BlacklistImpactRow struct {
	Label        string   `json:"label"`
	Organization string   `json:"organization,omitempty"`
	Tables       []string `json:"tables"`
	Applications []string `json:"applications"`
}

// BlacklistImpact - предпросмотр последствий внесения записи в чёрный список.
// Отвечает на вопрос «где этот человек или машина сейчас фигурирует»: сколько строк
// перестанет действовать, из каких таблиц постов они уйдут и в каких заявках останутся.
type BlacklistImpact struct {
	Matches int                  `json:"matches"`
	Tables  []string             `json:"tables"`
	Rows    []BlacklistImpactRow `json:"rows"`
}

// personBlacklistImpact считает последствия для людей: отбор совпадений повторяет
// deactivateMatchingEmployees, иначе предпросмотр обещал бы не то, что произойдёт.
func personBlacklistImpact(ctx context.Context, db *gorm.DB, lastName, firstName, middleName string) (*BlacklistImpact, error) {
	last := strings.TrimSpace(lastName)
	first := strings.TrimSpace(firstName)
	if last == "" || first == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нужны фамилия и имя")
	}

	const sql = `
		SELECT
			e.id,
			TRIM(CONCAT_WS(' ', e.last_name, e.first_name, e.middle_name)) AS label,
			COALESCE(o.name, '')                                           AS organization,
			COALESCE(
				(SELECT STRING_AGG(DISTINCT COALESCE(st.display_name, st.name), ' | ')
				   FROM employee_target_tables ett
				   JOIN system_tables st ON st.id = ett.table_id
				  WHERE ett.employee_id = e.id), '') AS tables,
			COALESCE(
				(SELECT STRING_AGG(DISTINCT app.application_number, ' | ')
				   FROM attachments att
				   JOIN applications app ON app.id = att.application_id
				  WHERE att.id = e.attachment_id AND app.application_number IS NOT NULL), '') AS applications
		FROM employees e
		LEFT JOIN attachments att2 ON att2.id = e.attachment_id
		LEFT JOIN applications app2 ON app2.id = att2.application_id
		LEFT JOIN organizations o ON o.id = app2.organization_id
		WHERE LOWER(TRIM(e.last_name)) = LOWER(TRIM(?))
		  AND LOWER(TRIM(e.first_name)) = LOWER(TRIM(?))
		  AND LOWER(TRIM(COALESCE(e.middle_name, ''))) = LOWER(TRIM(COALESCE(?, '')))
		  AND e.status = 1
		  AND e.is_purged = false
		ORDER BY e.id`

	return collectImpact(ctx, db, sql, last, first, strings.TrimSpace(middleName))
}

// vehicleBlacklistImpact считает то же для машин. Отбор повторяет deactivateMatchingCars
// целиком, вместе со сверкой марки: без марки предпросмотр обещал бы деактивацию машин
// с тем же номером и другой маркой, которых внесение в чёрный список не затронет.
func vehicleBlacklistImpact(ctx context.Context, db *gorm.DB, carNumber string, markID int) (*BlacklistImpact, error) {
	number := strings.TrimSpace(carNumber)
	if number == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нужен номер машины")
	}
	if markID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нужна марка машины")
	}

	const sql = `
		SELECT
			c.id,
			COALESCE(c.car_number, '')  AS label,
			COALESCE(o.name, '')        AS organization,
			COALESCE(
				(SELECT STRING_AGG(DISTINCT COALESCE(st.display_name, st.name), ' | ')
				   FROM car_target_tables ctt
				   JOIN system_tables st ON st.id = ctt.table_id
				  WHERE ctt.car_id = c.id), '') AS tables,
			COALESCE(
				(SELECT STRING_AGG(DISTINCT app.application_number, ' | ')
				   FROM attachments att
				   JOIN applications app ON app.id = att.application_id
				  WHERE att.id = c.attachment_id AND app.application_number IS NOT NULL), '') AS applications
		FROM cars c
		LEFT JOIN attachments att2 ON att2.id = c.attachment_id
		LEFT JOIN applications app2 ON app2.id = att2.application_id
		LEFT JOIN organizations o ON o.id = app2.organization_id
		WHERE LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))
		  AND (
		        c.mark_id = ?
		        OR (c.mark_id IS NULL
		            AND LOWER(TRIM(COALESCE(c.mark_name, c.car_brand)))
		              = LOWER(TRIM(COALESCE((SELECT name FROM marks WHERE id = ?), ''))))
		      )
		  AND c.status = 1
		  AND c.is_purged = false
		ORDER BY c.id`

	return collectImpact(ctx, db, sql, number, markID, markID)
}

// collectImpact выполняет запрос предпросмотра и сводит перечень постов по всем строкам:
// администратору важнее видеть «уйдёт из КПП №4», чем разбирать это по каждой строке.
func collectImpact(ctx context.Context, db *gorm.DB, sql string, args ...interface{}) (*BlacklistImpact, error) {
	// Списки собираются в строку через разделитель, а не в массив Postgres: массив
	// пришлось бы сканировать через драйверный тип, а разделитель читается как есть.
	var rows []struct {
		ID           int
		Label        string
		Organization string
		Tables       string
		Applications string
	}
	if err := db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка расчёта последствий")
	}

	impact := &BlacklistImpact{Matches: len(rows), Tables: []string{}, Rows: []BlacklistImpactRow{}}
	seenTable := map[string]bool{}
	for _, r := range rows {
		row := BlacklistImpactRow{
			Label:        strings.TrimSpace(r.Label),
			Organization: strings.TrimSpace(r.Organization),
			Tables:       splitAggregated(r.Tables),
			Applications: splitAggregated(r.Applications),
		}
		for _, t := range row.Tables {
			if !seenTable[t] {
				seenTable[t] = true
				impact.Tables = append(impact.Tables, t)
			}
		}
		impact.Rows = append(impact.Rows, row)
	}
	return impact, nil
}

// splitAggregated разбирает склеенный в SQL перечень обратно в срез.
func splitAggregated(value string) []string {
	out := []string{}
	for _, part := range strings.Split(value, " | ") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
