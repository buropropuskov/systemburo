package services

import (
	"context"
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
