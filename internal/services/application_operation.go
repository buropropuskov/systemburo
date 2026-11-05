package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"systemburo/internal/models"
)

const ApplicationOperationWindow = 24 * time.Hour
const ApplicationOperationRetention = 25 * time.Hour
const applicationOperationFutureSkew = 5 * time.Minute

var applicationOperationKeyPattern = regexp.MustCompile(`^v1\.[0-9]{13}\.[0-9a-f]{32}$`)

func validateApplicationOperationKey(key string, now time.Time) error {
	if !applicationOperationKeyPattern.MatchString(key) {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный ключ команды")
	}
	stamp, err := strconv.ParseInt(strings.Split(key, ".")[1], 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный ключ команды")
	}
	age := now.Sub(time.UnixMilli(stamp))
	if age > ApplicationOperationWindow || age < -applicationOperationFutureSkew {
		return echo.NewHTTPError(http.StatusConflict, "Срок команды истёк или время устройства неверно. Проверьте результат в списке заявок перед новой отправкой")
	}
	return nil
}

type applicationCommand struct{ row models.ApplicationOperation }

func newApplicationCommand(ctx context.Context, actor *models.User, scope, key string, body any) (*applicationCommand, error) {
	if key == "" {
		return nil, nil
	} // compatibility with old clients
	if actor == nil || actor.ID <= 0 {
		return nil, echo.ErrUnauthorized
	}
	if err := validateApplicationOperationKey(key, time.Now().UTC()); err != nil {
		return nil, err
	}
	if actor.IsBanned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Учётная запись недоступна")
	}
	ownerID := actor.ID
	if identity, ok := SessionIdentityFromContext(ctx); ok {
		if identity.OwnerUserID <= 0 || identity.EffectiveUserID != actor.ID {
			return nil, echo.ErrUnauthorized
		}
		ownerID = identity.OwnerUserID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная команда")
	}
	hash := sha256.Sum256(encoded)
	return &applicationCommand{row: models.ApplicationOperation{
		OwnerUserID: ownerID, EffectiveUserID: actor.ID, Scope: scope, Key: key,
		Fingerprint: hex.EncodeToString(hash[:]), CreatedAt: time.Now().UTC(),
	}}, nil
}

func (c *applicationCommand) query(db *gorm.DB) *gorm.DB {
	return db.Where("owner_user_id = ? AND effective_user_id = ? AND scope = ? AND key = ?", c.row.OwnerUserID, c.row.EffectiveUserID, c.row.Scope, c.row.Key)
}

func (c *applicationCommand) lookup(db *gorm.DB, result any) (bool, error) {
	if c == nil {
		return false, nil
	}
	var row models.ApplicationOperation
	err := c.query(db).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка результата команды временно недоступна")
	}
	if row.Fingerprint != c.row.Fingerprint {
		return false, echo.NewHTTPError(http.StatusConflict, "Ключ команды уже использован с другим содержимым")
	}
	if len(row.Result) == 0 || string(row.Result) == "null" {
		return false, echo.NewHTTPError(http.StatusServiceUnavailable, "Результат команды временно недоступен")
	}
	if err := json.Unmarshal(row.Result, result); err != nil {
		return false, echo.NewHTTPError(http.StatusServiceUnavailable, "Результат команды временно недоступен")
	}
	return true, nil
}

// The unique insert waits for a concurrent command's commit/rollback. No work or
// file attachment happens until the reservation belongs to this transaction.
func (c *applicationCommand) reserve(tx *gorm.DB, result any) (bool, error) {
	if c == nil {
		return false, nil
	}
	inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&c.row)
	if inserted.Error != nil {
		return false, echo.NewHTTPError(http.StatusServiceUnavailable, "Не удалось зарегистрировать команду")
	}
	if inserted.RowsAffected == 1 {
		return false, nil
	}
	found, err := c.lookup(tx, result)
	if err != nil {
		return false, err
	}
	if !found {
		return false, echo.NewHTTPError(http.StatusServiceUnavailable, "Результат команды временно недоступен")
	}
	return true, nil
}

func (c *applicationCommand) save(tx *gorm.DB, result any) error {
	if c == nil {
		return nil
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 4096 {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить результат команды")
	}
	update := c.query(tx.Model(&models.ApplicationOperation{})).Update("result", json.RawMessage(encoded))
	if update.Error != nil || update.RowsAffected != 1 {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить результат команды")
	}
	return nil
}

func (s *applicationService) checkCommandReplayAccess(ctx context.Context, username string, applicationID int) error {
	allowed, err := s.CanAccessApplicationChecked(ctx, applicationID, username, false)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ к результату команды недоступен")
	}
	return nil
}

// ApplicationOperationStatusRequest identifies an existing command, never a new write.
type ApplicationOperationStatusRequest struct {
	Key   string `json:"key" validate:"required,max=80"`
	Scope string `json:"scope" validate:"required,max=80"`
}
type ApplicationOperationStatusResponse struct {
	State  string `json:"state"`
	Result any    `json:"result,omitempty"`
}

// LookupApplicationOperationStatus keeps legacy service adapters source compatible.
func LookupApplicationOperationStatus(ctx context.Context, service ApplicationService, username string, req ApplicationOperationStatusRequest) (*ApplicationOperationStatusResponse, error) {
	checked, ok := service.(interface {
		ApplicationOperationStatus(context.Context, string, ApplicationOperationStatusRequest) (*ApplicationOperationStatusResponse, error)
	})
	if !ok {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка результата команды временно недоступна")
	}
	return checked.ApplicationOperationStatus(ctx, username, req)
}

func (s *applicationService) ApplicationOperationStatus(ctx context.Context, username string, req ApplicationOperationStatusRequest) (*ApplicationOperationStatusResponse, error) {
	if !applicationOperationKeyPattern.MatchString(req.Key) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ключ команды")
	}
	var applicationID int
	if req.Scope != "application.submit" {
		prefix := "application.supplement:"
		if !strings.HasPrefix(req.Scope, prefix) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная область команды")
		}
		id, err := strconv.Atoi(strings.TrimPrefix(req.Scope, prefix))
		if err != nil || id <= 0 || req.Scope != prefix+strconv.Itoa(id) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректная область команды")
		}
		applicationID = id
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	identity, ok := SessionIdentityFromContext(ctx)
	if !ok || identity.OwnerUserID <= 0 || identity.EffectiveUserID != user.ID {
		return nil, echo.ErrUnauthorized
	}
	if user.IsBanned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Учётная запись недоступна")
	}
	if applicationID > 0 {
		var app models.Application
		found := s.db.WithContext(ctx).Select("id,sender_user_id").Where("id = ?", applicationID).First(&app)
		if errors.Is(found.Error, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		if found.Error != nil {
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
		}
		if app.SenderUserID != user.ID && !user.IsSuperAdmin {
			return nil, echo.NewHTTPError(http.StatusForbidden, "Доступ к результату команды недоступен")
		}
		if err := s.checkCommandReplayAccess(ctx, username, applicationID); err != nil {
			return nil, err
		}
	}
	var row models.ApplicationOperation
	err = s.db.WithContext(ctx).Where("owner_user_id = ? AND effective_user_id = ? AND scope = ? AND key = ?", identity.OwnerUserID, user.ID, req.Scope, req.Key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &ApplicationOperationStatusResponse{State: "unknown"}, nil
	}
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка результата команды временно недоступна")
	}
	if len(row.Result) == 0 || string(row.Result) == "null" {
		return &ApplicationOperationStatusResponse{State: "unknown"}, nil
	}
	if applicationID == 0 {
		var result CompleteApplicationResponse
		if err := json.Unmarshal(row.Result, &result); err != nil || result.ApplicationID <= 0 {
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Результат команды временно недоступен")
		}
		var app models.Application
		if err := s.db.WithContext(ctx).Select("id,sender_user_id").Where("id = ? AND sender_user_id = ?", result.ApplicationID, user.ID).First(&app).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, echo.NewHTTPError(http.StatusForbidden, "Доступ к результату команды недоступен")
			}
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
		}
		if err := s.checkCommandReplayAccess(ctx, username, result.ApplicationID); err != nil {
			return nil, err
		}
		return &ApplicationOperationStatusResponse{State: "committed", Result: &result}, nil
	}
	var result CreateSupplementResponse
	if err := json.Unmarshal(row.Result, &result); err != nil || result.SupplementID <= 0 {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "Результат команды временно недоступен")
	}
	return &ApplicationOperationStatusResponse{State: "committed", Result: &result}, nil
}
