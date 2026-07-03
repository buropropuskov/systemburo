package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/export"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// BuildSnapshotExport собирает табличные данные версии (или текущего состояния при
// snapshotID == nil) в формат-нейтральную export.Table для рендера в Excel/PDF.
// Строки берутся из payload снимка (для версии) либо свежим collectRows (для текущего) -
// ровно тот набор, что показывает/снимает страница таблицы.
func (s *tableSnapshotService) BuildSnapshotExport(ctx context.Context, tableID int, snapshotID *int) (export.Table, string, error) {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).
		Select("id", "name", "display_name", "table_type", "show_fact_table").
		First(&table, tableID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return export.Table{}, "", echo.NewHTTPError(http.StatusNotFound, "Table not found")
		}
		return export.Table{}, "", fmt.Errorf("failed to load table %d for export: %w", tableID, err)
	}

	payloadRaw, takenAt, err := s.resolveExportPayload(ctx, table, snapshotID)
	if err != nil {
		return export.Table{}, "", err
	}

	var payload models.SnapshotPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return export.Table{}, "", fmt.Errorf("failed to parse snapshot payload for export: %w", err)
	}

	tbl := export.Table{Title: snapshotTableTitle(table)}
	mskAt := takenAt.In(moscowWorkModeLoc)
	if snapshotID == nil {
		tbl.Subtitle = "Текущее состояние на " + mskAt.Format("02.01.2006 15:04")
	} else {
		tbl.Subtitle = "Версия от " + mskAt.Format("02.01.2006 15:04")
	}

	switch payload.TableType {
	case models.TableTypeCars:
		var rows []snapshotCarRow
		if err := json.Unmarshal(payload.Rows, &rows); err != nil {
			return export.Table{}, "", fmt.Errorf("failed to parse car rows for export: %w", err)
		}
		tbl.Headers, tbl.Rows = carExportRows(rows)
	case models.TableTypePeople:
		var rows []snapshotEmployeeRow
		if err := json.Unmarshal(payload.Rows, &rows); err != nil {
			return export.Table{}, "", fmt.Errorf("failed to parse employee rows for export: %w", err)
		}
		tbl.Headers, tbl.Rows = employeeExportRows(rows)
	default:
		return export.Table{}, "", echo.NewHTTPError(http.StatusUnprocessableEntity, "Unsupported table type for export")
	}

	return tbl, snapshotExportFilename(tbl.Title, snapshotID, mskAt), nil
}

// resolveExportPayload отдаёт сырой payload и момент состояния: для версии - из БД,
// для текущего (snapshotID == nil) - свежий слепок без записи.
func (s *tableSnapshotService) resolveExportPayload(ctx context.Context, table models.SystemTable, snapshotID *int) (json.RawMessage, time.Time, error) {
	if snapshotID == nil {
		rows, _, err := s.collectRows(ctx, table)
		if err != nil {
			return nil, time.Time{}, err
		}
		// Fields здесь не собираем: экспорт (carExportRows/employeeExportRows) рендерит
		// колонки по жёсткой шапке, payload.Fields не читает - лишний SELECT ни к чему.
		payloadJSON, err := json.Marshal(models.SnapshotPayload{TableType: table.TableType, Rows: rows})
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to marshal current payload for export: %w", err)
		}
		return payloadJSON, time.Now().UTC(), nil
	}

	snap, err := s.GetSnapshot(ctx, table.ID, *snapshotID)
	if err != nil {
		return nil, time.Time{}, err
	}
	return snap.Payload, snap.TakenAt, nil
}

// snapshotTableTitle - человекочитаемое имя таблицы для заголовка выгрузки.
func snapshotTableTitle(t models.SystemTable) string {
	if t.DisplayName != nil && strings.TrimSpace(*t.DisplayName) != "" {
		return *t.DisplayName
	}
	return t.Name
}

// snapshotExportFilename строит человекочитаемую базу имени файла (без расширения),
// напр. «Машины - версия 03.07.2026». Кириллица - handler отдаёт её в
// Content-Disposition через filename* (RFC 5987) с ASCII-фолбэком.
func snapshotExportFilename(title string, snapshotID *int, at time.Time) string {
	kind := "версия"
	if snapshotID == nil {
		kind = "текущее состояние"
	}
	return fmt.Sprintf("%s - %s %s", title, kind, at.Format("02.01.2006"))
}

// carExportRows раскладывает строки-машины снимка в шапку и ячейки выгрузки. Колонка
// «По факту» добавляется только если в слепке есть строки блока «по факту».
func carExportRows(rows []snapshotCarRow) ([]string, [][]string) {
	hasFact := false
	for _, r := range rows {
		if r.IsFact {
			hasFact = true
			break
		}
	}

	headers := []string{"№ машины", "Марка", "Организация", "Компания", "Места разгрузки", "Заявка №", "Статус на территории", "Время въезда"}
	if hasFact {
		headers = append(headers, "По факту")
	}

	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells := []string{
			r.CarNumber,
			r.CarBrand,
			derefStr(r.Organization),
			derefStr(r.Company),
			carUnloadPlaces(r),
			derefStr(r.ApplicationNumber),
			territoryStatusLabel(r.TerritoryStatus),
			derefStr(r.TerritoryEntryTime),
		}
		if hasFact {
			cells = append(cells, boolLabel(r.IsFact))
		}
		out = append(out, cells)
	}
	return headers, out
}

// employeeExportRows раскладывает строки-сотрудники снимка в шапку и ячейки выгрузки.
func employeeExportRows(rows []snapshotEmployeeRow) ([]string, [][]string) {
	headers := []string{"Фамилия", "Имя", "Отчество", "Организация", "Компания", "Гражданство", "Должность", "Места прохода", "Заявка №", "Статус на территории"}

	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, []string{
			r.LastName,
			r.FirstName,
			derefStr(r.MiddleName),
			derefStr(r.Organization),
			derefStr(r.Company),
			derefStr(r.CitizenshipName),
			derefStr(r.Position),
			derefStr(r.PassPlaces),
			derefStr(r.ApplicationNumber),
			territoryStatusLabel(r.TerritoryStatus),
		})
	}
	return headers, out
}

// carUnloadPlaces - список мест разгрузки машины через запятую (страница показывает
// их множеством); fallback на единичное поле, если множественное пустое.
func carUnloadPlaces(r snapshotCarRow) string {
	if len(r.UnloadPlaces) > 0 {
		return strings.Join(r.UnloadPlaces, ", ")
	}
	return derefStr(r.UnloadPlace)
}

// territoryStatusLabel - русская подпись территориального статуса, как на фронте:
// 1=На территории, 2=Выехал, 0/nil=Не въезжал.
func territoryStatusLabel(s *int) string {
	switch {
	case s != nil && *s == 1:
		return "На территории"
	case s != nil && *s == 2:
		return "Выехал"
	default:
		return "Не въезжал"
	}
}

func boolLabel(b bool) string {
	if b {
		return "Да"
	}
	return ""
}
