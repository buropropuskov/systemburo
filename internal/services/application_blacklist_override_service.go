package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OverrideBlacklistFlagRequest - запрос "всё равно пропустить" по конкретному флагу (#481).
type OverrideBlacklistFlagRequest struct {
	FlagID  int    `json:"flag_id" validate:"gte=1"`
	Comment string `json:"comment" validate:"required,min=1,max=1000"`
}

// OverrideBlacklistFlag фиксирует решение ответственного "всё равно пропустить" по
// помеченному элементу заявки (#481, срез 4): пишет аудит-запись (кто/когда/коммент +
// снимок совпавшего значения) и тем самым снимает блокировку согласования по этому флагу.
// Только ответственный по заявке. Идемпотентно: повторный override того же флага не
// плодит дубль (uniqueIndex на flag_id) и возвращает успех.
func (s *applicationService) OverrideBlacklistFlag(ctx context.Context, username string, applicationID int, req OverrideBlacklistFlagRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Комментарий к пропуску обязателен")
	}

	// Подтвердить пропуск может только ответственный по заявке (как и голосовать).
	var responsibleID int
	if err := s.db.WithContext(ctx).Raw(
		"SELECT id FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		applicationID, user.ID,
	).Scan(&responsibleID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки прав на заявку")
	}
	if responsibleID == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "Вы не ответственный по этой заявке")
	}

	// Флаг должен существовать и принадлежать этой заявке.
	var flag models.ApplicationBlacklistFlag
	err = s.db.WithContext(ctx).Where("id = ? AND application_id = ?", req.FlagID, applicationID).First(&flag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Предупреждение не найдено для этой заявки")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения предупреждения")
	}

	var existing int64
	if err := s.db.WithContext(ctx).Model(&models.ApplicationBlacklistOverride{}).
		Where("flag_id = ?", flag.ID).Count(&existing).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки подтверждения пропуска")
	}
	if existing > 0 {
		return nil // уже подтверждён - идемпотентно
	}

	override := models.ApplicationBlacklistOverride{
		FlagID:             flag.ID,
		ApplicationID:      flag.ApplicationID,
		ElementType:        flag.ElementType,
		ElementID:          flag.ElementID,
		MatchedValue:       flag.MatchedValue,
		OverriddenByUserID: user.ID,
		Comment:            comment,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&override).Error; err != nil {
		if isUniqueViolation(err) {
			return nil // гонка двух параллельных override одного флага - не ошибка
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения подтверждения пропуска")
	}
	return nil
}

// DeleteBlacklistOverride снимает ранее подтверждённый пропуск по флагу (#481, срез C):
// удаляет аудит-запись override, чем снова блокирует согласование заявки по этому элементу
// (hasUnoverriddenBlacklistFlags опять вернёт true). Право отмены шире, чем создание: не
// только ответственный по заявке, но и принимающий (глобальный согласующий) - ошибочное
// подтверждение должен мочь снять и тот, кто его не ставил. Override-строку удаляем (иначе
// гейт не разблокируется обратно), поэтому факт отмены пишем отдельно в историю заявки.
func (s *applicationService) DeleteBlacklistOverride(ctx context.Context, username string, applicationID, flagID int) error {
	if flagID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный идентификатор предупреждения")
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	allowed, err := s.canManageBlacklistOverride(ctx, applicationID, user.ID)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для отмены подтверждения пропуска")
	}

	// Флаг должен существовать и принадлежать этой заявке - защита от подмены чужого flag_id.
	var flag models.ApplicationBlacklistFlag
	if err := s.db.WithContext(ctx).Where("id = ? AND application_id = ?", flagID, applicationID).First(&flag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Предупреждение не найдено для этой заявки")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения предупреждения")
	}

	var override models.ApplicationBlacklistOverride
	if err := s.db.WithContext(ctx).Where("flag_id = ? AND application_id = ?", flag.ID, applicationID).First(&override).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Подтверждение пропуска не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения подтверждения пропуска")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.ApplicationBlacklistOverride{}, override.ID).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка отмены подтверждения пропуска")
		}
		meta, _ := json.Marshal(map[string]any{"flag_id": flag.ID, "matched_value": override.MatchedValue})
		if err := tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, comment, metadata, created_at)
			VALUES (?, ?, 'blacklist_override_revoke', ?, ?, ?)
		`, applicationID, user.ID, override.MatchedValue, string(meta), time.Now().UTC()).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории")
		}
		return nil
	})
}

// canManageBlacklistOverride - право отменять подтверждение пропуска (#481, срез C):
// ответственный по этой заявке ИЛИ принимающий (глобальный согласующий). Шире, чем право
// создавать override (только ответственный).
func (s *applicationService) canManageBlacklistOverride(ctx context.Context, applicationID, userID int) (bool, error) {
	var responsibleID int
	if err := s.db.WithContext(ctx).Raw(
		"SELECT id FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		applicationID, userID,
	).Scan(&responsibleID).Error; err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки прав на заявку")
	}
	if responsibleID != 0 {
		return true, nil
	}
	return s.isApprover(ctx, userID)
}

// hasUnoverriddenBlacklistFlags - есть ли у заявки помеченные элементы без override (#481).
// Гейт согласования: пока true, голос "approved" запрещён. Принимает *gorm.DB, чтобы
// вызываться и внутри транзакции согласования, и отдельно.
func hasUnoverriddenBlacklistFlags(ctx context.Context, db *gorm.DB, applicationID int) (bool, error) {
	var cnt int64
	err := db.WithContext(ctx).
		Table("application_blacklist_flags f").
		Where("f.application_id = ?", applicationID).
		Where("NOT EXISTS (SELECT 1 FROM application_blacklist_overrides o WHERE o.flag_id = f.id)").
		Count(&cnt).Error
	if err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки предупреждений ЧС")
	}
	return cnt > 0, nil
}
