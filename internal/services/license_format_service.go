package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// LicensePlateFormatService определяет интерфейс бизнес-логики форматов номерных знаков.
type LicensePlateFormatService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.LicensePlateFormatWithCells, error)
	Create(ctx context.Context, callerUserID int, req models.CreateLicensePlateFormatRequest) (int, error)
	Update(ctx context.Context, callerUserID, id int, req models.UpdateLicensePlateFormatRequest) error
	Delete(ctx context.Context, callerUserID, id int) error
	Restore(ctx context.Context, callerUserID, id int) error
	BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)
	BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)
	GetHistory(ctx context.Context, id int) ([]models.LicensePlateFormatHistoryItem, error)
}

type licensePlateFormatService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewLicensePlateFormatService создаёт сервис для управления форматами номерных знаков.
func NewLicensePlateFormatService(db *gorm.DB) LicensePlateFormatService {
	return &licensePlateFormatService{db: db, recorder: NewAuditRecorder(db)}
}

// GetAll возвращает форматы номерных знаков с ячейками.
// По умолчанию только активные; includeArchived=true добавляет архивные.
func (s *licensePlateFormatService) GetAll(ctx context.Context, includeArchived bool) ([]models.LicensePlateFormatWithCells, error) {
	formats := make([]models.LicensePlateFormat, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&formats).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching license plate formats")
	}

	result := make([]models.LicensePlateFormatWithCells, 0, len(formats))
	for _, f := range formats {
		cells := make([]models.LicensePlateFormatCell, 0)
		if err := s.db.WithContext(ctx).
			Where("format_id = ?", f.ID).
			Order("cell_order").
			Find(&cells).Error; err != nil {
			continue
		}
		result = append(result, models.LicensePlateFormatWithCells{
			Format: f,
			Cells:  cells,
		})
	}

	return result, nil
}

// Create создаёт формат номерного знака с ячейками в транзакции.
func (s *licensePlateFormatService) Create(ctx context.Context, callerUserID int, req models.CreateLicensePlateFormatRequest) (int, error) {
	var formatID int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := tx.Model(&models.LicensePlateFormat{}).
				Where("is_default = ?", true).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default formats")
			}
		}

		format := models.LicensePlateFormat{
			Name:        req.Name,
			CountryCode: req.CountryCode,
			Icon:        req.Icon,
			IsActive:    true,
			IsDefault:   req.IsDefault != nil && *req.IsDefault,
		}
		if err := tx.Create(&format).Error; err != nil {
			slog.Error("не удалось создать формат номеров", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании формата номеров")
		}
		formatID = format.ID

		for _, c := range req.Cells {
			cell := models.LicensePlateFormatCell{
				FormatID:       formatID,
				CellOrder:      c.CellOrder,
				CellType:       c.CellType,
				MinLength:      c.MinLength,
				MaxLength:      c.MaxLength,
				AllowedLetters: c.AllowedLetters,
				AlphabetType:   c.AlphabetType,
				Language:       c.Language,
				PaddingChar:    ptrOrDefault(c.PaddingChar, "0"),
				PaddingSide:    ptrOrDefault(c.PaddingSide, "left"),
			}
			if err := tx.Create(&cell).Error; err != nil {
				slog.Error("не удалось создать ячейку формата номеров", "format_id", formatID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании ячейки формата номеров")
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}
	slog.Info("формат номеров создан", "id", formatID)
	s.recorder.Log(ctx, nil, models.AuditEntityLicensePlateFormat, &formatID, models.LicensePlateFormatActionCreated, &callerUserID, map[string]any{"name": req.Name})
	return formatID, nil
}

// Update обновляет формат номерного знака и пересоздаёт ячейки.
func (s *licensePlateFormatService) Update(ctx context.Context, callerUserID, id int, req models.UpdateLicensePlateFormatRequest) error {
	var details map[string]any

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Снимок до изменений - для diff в истории. First даёт чистый 404,
		// если формата нет (вместо проверки RowsAffected ниже).
		var prev models.LicensePlateFormat
		if err := tx.First(&prev, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения формата номеров")
		}
		var prevCells []models.LicensePlateFormatCell
		if err := tx.Where("format_id = ?", id).Order("cell_order").Find(&prevCells).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching format cells")
		}

		if req.IsDefault != nil && *req.IsDefault {
			if err := tx.Model(&models.LicensePlateFormat{}).
				Where("is_default = ? AND id != ?", true, id).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default formats")
			}
		}

		isDefault := req.IsDefault != nil && *req.IsDefault
		if err := tx.Model(&models.LicensePlateFormat{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":         req.Name,
				"country_code": req.CountryCode,
				"icon":         req.Icon,
				"is_default":   isDefault,
			}).Error; err != nil {
			slog.Error("не удалось обновить формат номеров", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating license plate format")
		}
		slog.Info("формат номеров обновлён", "id", id)

		if err := tx.Where("format_id = ?", id).Delete(&models.LicensePlateFormatCell{}).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating format cells")
		}

		for _, c := range req.Cells {
			cell := models.LicensePlateFormatCell{
				FormatID:       id,
				CellOrder:      c.CellOrder,
				CellType:       c.CellType,
				MinLength:      c.MinLength,
				MaxLength:      c.MaxLength,
				AllowedLetters: c.AllowedLetters,
				AlphabetType:   c.AlphabetType,
				Language:       c.Language,
				PaddingChar:    ptrOrDefault(c.PaddingChar, "0"),
				PaddingSide:    ptrOrDefault(c.PaddingSide, "left"),
			}
			if err := tx.Create(&cell).Error; err != nil {
				slog.Error("не удалось создать ячейку формата номеров при обновлении", "format_id", id, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании ячейки формата номеров")
			}
		}

		details = buildFormatUpdateDetails(prev, prevCells, req)
		return nil
	})
	if err != nil {
		return err
	}

	// Логируем только если что-то реально изменилось - иначе спам "Изменены данные" без диффа.
	if len(details) > 0 {
		s.recorder.Log(ctx, nil, models.AuditEntityLicensePlateFormat, &id, models.LicensePlateFormatActionUpdated, &callerUserID, details)
	}
	return nil
}

// Delete архивирует формат номерного знака (soft-delete через is_active=false).
// Формат по умолчанию архивировать нельзя - сначала нужно назначить другой дефолтный.
// Формат, используемый машинами (unique_cars.format_id), архивировать можно: машины уже
// созданы и валидацию прошли, архивный формат лишь скрывается из выбора новых.
func (s *licensePlateFormatService) Delete(ctx context.Context, callerUserID, id int) error {
	var format models.LicensePlateFormat
	if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения формата номеров")
	}
	if !format.IsActive {
		return nil // уже в архиве - идемпотентно; проверяем до is_default, иначе архивный дефолт залип бы на 409
	}
	if format.IsDefault {
		return echo.NewHTTPError(http.StatusConflict, "Нельзя архивировать формат по умолчанию. Сначала назначьте другой формат по умолчанию")
	}
	if err := s.db.WithContext(ctx).Model(&format).Update("is_active", false).Error; err != nil {
		slog.Error("не удалось архивировать формат номеров", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка архивации формата номеров")
	}
	slog.Info("формат номеров архивирован", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityLicensePlateFormat, &id, models.LicensePlateFormatActionArchived, &callerUserID, nil)
	return nil
}

// Restore восстанавливает формат номерного знака из архива (is_active=true).
func (s *licensePlateFormatService) Restore(ctx context.Context, callerUserID, id int) error {
	var format models.LicensePlateFormat
	if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения формата номеров")
	}
	if format.IsActive {
		return nil // уже активен - идемпотентно
	}
	// У форматов нет уникального индекса по name (в отличие от marks/organizations),
	// поэтому при восстановлении проверка конфликта имени не нужна.
	if err := s.db.WithContext(ctx).Model(&format).Update("is_active", true).Error; err != nil {
		slog.Error("не удалось восстановить формат номеров", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка восстановления формата номеров")
	}
	slog.Info("формат номеров восстановлен", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityLicensePlateFormat, &id, models.LicensePlateFormatActionRestored, &callerUserID, nil)
	return nil
}

// BulkArchive архивирует набор форматов номеров через Delete (soft-delete). Несуществующие
// -> в Errors (частичный успех 207), не валят операцию. Дубли id дедуплицируются.
func (s *licensePlateFormatService) BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		var format models.LicensePlateFormat
		if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
			res.addError(id, "", "Формат не найден")
			continue
		}
		if err := s.Delete(ctx, callerUserID, id); err != nil {
			res.addError(id, format.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор форматов номеров через Restore.
func (s *licensePlateFormatService) BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		var format models.LicensePlateFormat
		if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
			res.addError(id, "", "Формат не найден")
			continue
		}
		if err := s.Restore(ctx, callerUserID, id); err != nil {
			res.addError(id, format.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// GetHistory возвращает историю изменений формата номеров (новые сверху).
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная license_plate_format_histories дропнута в дроп-sweep (F.8).
// Форму стережёт TestLicenseFormats_History.
func (s *licensePlateFormatService) GetHistory(ctx context.Context, id int) ([]models.LicensePlateFormatHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityLicensePlateFormat, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching license plate format history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.LicensePlateFormatHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.LicensePlateFormatHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}

// ptrOrDefault возвращает указатель на значение или значение по умолчанию.
func ptrOrDefault(p *string, def string) *string {
	if p != nil {
		return p
	}
	return &def
}

// formatCellSnapshot - компактный снимок ячейки для аудита: сравнение old/new
// при Update, чтобы зафиксировать изменение правил валидации номера.
type formatCellSnapshot struct {
	CellOrder      int     `json:"cell_order"`
	CellType       string  `json:"cell_type"`
	MinLength      *int    `json:"min_length,omitempty"`
	MaxLength      *int    `json:"max_length,omitempty"`
	AllowedLetters *string `json:"allowed_letters,omitempty"`
	AlphabetType   *string `json:"alphabet_type,omitempty"`
	Language       *string `json:"language,omitempty"`
	PaddingChar    *string `json:"padding_char,omitempty"`
	PaddingSide    *string `json:"padding_side,omitempty"`
}

// buildFormatUpdateDetails собирает diff изменённых полей формата как {old, new}.
// В результат попадают только реально изменившиеся поля - иначе история засоряется
// "пустыми" записями (см. ui-history: фильтр nil/неизменённого обязателен).
func buildFormatUpdateDetails(prev models.LicensePlateFormat, prevCells []models.LicensePlateFormatCell, req models.UpdateLicensePlateFormatRequest) map[string]any {
	details := map[string]any{}

	if req.Name != prev.Name {
		details["name"] = map[string]any{"old": prev.Name, "new": req.Name}
	}
	if strPtrVal(prev.CountryCode) != strPtrVal(req.CountryCode) {
		details["country_code"] = map[string]any{"old": strPtrVal(prev.CountryCode), "new": strPtrVal(req.CountryCode)}
	}
	if strPtrVal(prev.Icon) != strPtrVal(req.Icon) {
		details["icon"] = map[string]any{"old": strPtrVal(prev.Icon), "new": strPtrVal(req.Icon)}
	}
	newDefault := req.IsDefault != nil && *req.IsDefault
	if newDefault != prev.IsDefault {
		details["is_default"] = map[string]any{"old": prev.IsDefault, "new": newDefault}
	}

	// Нормализуем порядок по cell_order перед сравнением: клиент может прислать
	// ячейки не по порядку, а reflect.DeepEqual слайсов чувствителен к позиции -
	// иначе простая перестановка без изменения данных дала бы ложный diff в истории.
	oldCells := cellSnapshotsFromModel(prevCells)
	newCells := cellSnapshotsFromRequest(req.Cells)
	sort.Slice(oldCells, func(i, j int) bool { return oldCells[i].CellOrder < oldCells[j].CellOrder })
	sort.Slice(newCells, func(i, j int) bool { return newCells[i].CellOrder < newCells[j].CellOrder })
	if !reflect.DeepEqual(oldCells, newCells) {
		details["cells"] = map[string]any{"old": oldCells, "new": newCells}
	}

	return details
}

func cellSnapshotsFromModel(cells []models.LicensePlateFormatCell) []formatCellSnapshot {
	out := make([]formatCellSnapshot, 0, len(cells))
	for _, c := range cells {
		out = append(out, formatCellSnapshot{
			CellOrder: c.CellOrder, CellType: c.CellType,
			MinLength: c.MinLength, MaxLength: c.MaxLength,
			AllowedLetters: c.AllowedLetters, AlphabetType: c.AlphabetType,
			Language: c.Language, PaddingChar: c.PaddingChar, PaddingSide: c.PaddingSide,
		})
	}
	return out
}

// cellSnapshotsFromRequest применяет те же дефолты padding, что и при записи в БД,
// чтобы сравнение old (из БД) и new (из запроса) не давало ложных диффов.
func cellSnapshotsFromRequest(cells []models.UpdateFormatCellRequest) []formatCellSnapshot {
	out := make([]formatCellSnapshot, 0, len(cells))
	for _, c := range cells {
		out = append(out, formatCellSnapshot{
			CellOrder: c.CellOrder, CellType: c.CellType,
			MinLength: c.MinLength, MaxLength: c.MaxLength,
			AllowedLetters: c.AllowedLetters, AlphabetType: c.AlphabetType,
			Language:    c.Language,
			PaddingChar: ptrOrDefault(c.PaddingChar, "0"), PaddingSide: ptrOrDefault(c.PaddingSide, "left"),
		})
	}
	return out
}

// strPtrVal разыменовывает *string, трактуя nil как "" - чтобы nil и пустая строка
// считались одним значением и не плодили ложные диффы в истории.
func strPtrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
