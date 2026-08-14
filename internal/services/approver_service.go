package services

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ApproverService определяет интерфейс управления утверждающими заявок.
// Авторизация изменяющих/листинговых операций (page.admin) - на роут-middleware
// RequirePermissionV2; история (GetHistory) доступна всем авторизованным.
type ApproverService interface {
	GetAll(ctx context.Context) ([]models.ApplicationApproverWithUser, error)

	// GetRecipients отдаёт принимающих для строки получателей заявки - только
	// отображаемые имена. Отдельно от GetAll по той же причине, что и IsApprover:
	// тот закрыт правом администратора, а заявителю нужно видеть, кому уйдёт заявка.
	GetRecipients(ctx context.Context) ([]models.ApplicationRecipient, error)

	GetAvailableUsers(ctx context.Context) ([]models.AvailableApproverUser, error)
	Create(ctx context.Context, userID int, createdByUsername string) error
	Update(ctx context.Context, id int, displayName *string, actorUsername string) error
	Delete(ctx context.Context, id int, actorUsername string) error
	GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error)

	// IsApprover сообщает, числится ли пользователь принимающим. Отдельно от GetAll:
	// тот отдаёт весь состав с ФИО и организациями и потому закрыт правом администратора,
	// а карточке заявки нужен только ответ про себя - без него кнопки принимающего не
	// показывались бы никому, кроме администраторов (#1685).
	IsApprover(ctx context.Context, username string) (bool, error)

	// IsReviewer сообщает, назначен ли пользователь согласующим хоть где-нибудь.
	// Согласующий живёт per-application (application_responsible_users), поэтому
	// глобального признака у него не было; здесь он выводится из назначений в
	// организациях и компаниях - именно оттуда согласующие попадают в заявку (#1737).
	IsReviewer(ctx context.Context, username string) (bool, error)
}

type approverService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewApproverService создаёт сервис для управления утверждающими заявок.
func NewApproverService(db *gorm.DB) ApproverService {
	return &approverService{db: db, recorder: NewAuditRecorder(db)}
}

// GetAll возвращает список всех утверждающих с информацией о пользователях.
func (s *approverService) GetAll(ctx context.Context) ([]models.ApplicationApproverWithUser, error) {
	result := make([]models.ApplicationApproverWithUser, 0)
	err := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.id, aa.user_id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company, aa.display_name, aa.created_at`).
		Joins("JOIN users u ON u.id = aa.user_id").
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approvers")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range result {
			maskUserParts(masks, result[i].UserID, &result[i].LastName, &result[i].FirstName, &result[i].MiddleName)
		}
	}
	return result, nil
}

// GetRecipients возвращает принимающих для строки получателей заявки: заявителю нужно
// видеть, кому уйдёт заявка, а полный состав с организациями и должностями закрыт правом
// администратора. Имя берётся из маски, если она задана, иначе собирается из ФИО; когда
// работник не дал согласия на обработку персональных данных, вместо ФИО идёт логин - тем
// же правилом, что и в остальных местах.
func (s *approverService) GetRecipients(ctx context.Context) ([]models.ApplicationRecipient, error) {
	var rows []struct {
		UserID      int
		Username    string
		LastName    *string
		FirstName   *string
		MiddleName  *string
		DisplayName *string
	}
	err := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.user_id, u.username, u.last_name, u.first_name, u.middle_name, aa.display_name`).
		Joins("JOIN users u ON u.id = aa.user_id").
		// Супер-администратор в получателях не нужен: он числится принимающим ради
		// доступа, а не как адресат заявки, и в списке только сбивал бы заявителя.
		Where("u.is_active AND NOT u.is_banned AND NOT u.is_super_admin").
		Order("u.last_name, u.first_name").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching recipients")
	}

	masks := loadConsentMasks(ctx, s.db)
	result := make([]models.ApplicationRecipient, 0, len(rows))
	for _, r := range rows {
		if r.DisplayName != nil && strings.TrimSpace(*r.DisplayName) != "" {
			result = append(result, models.ApplicationRecipient{UserID: r.UserID, Name: strings.TrimSpace(*r.DisplayName), Masked: true})
			continue
		}
		last, first, middle := r.LastName, r.FirstName, r.MiddleName
		maskUserParts(masks, r.UserID, &last, &first, &middle)
		parts := make([]string, 0, 3)
		for _, p := range []*string{last, first, middle} {
			if p != nil && strings.TrimSpace(*p) != "" {
				parts = append(parts, strings.TrimSpace(*p))
			}
		}
		name := strings.Join(parts, " ")
		if name == "" {
			name = r.Username
		}
		result = append(result, models.ApplicationRecipient{UserID: r.UserID, Name: name})
	}
	return result, nil
}

// GetAvailableUsers возвращает пользователей, которые ещё не назначены утверждающими.
func (s *approverService) GetAvailableUsers(ctx context.Context) ([]models.AvailableApproverUser, error) {
	result := make([]models.AvailableApproverUser, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company`).
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Where("NOT EXISTS (SELECT 1 FROM application_approvers aa WHERE aa.user_id = u.id)").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching available users")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range result {
			maskUserParts(masks, result[i].ID, &result[i].LastName, &result[i].FirstName, &result[i].MiddleName)
		}
	}
	return result, nil
}

// Create назначает пользователя утверждающим заявок.
func (s *approverService) Create(ctx context.Context, userID int, createdByUsername string) error {
	var userExists int64
	s.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&userExists)
	if userExists == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User not found")
	}

	var alreadyApprover int64
	s.db.WithContext(ctx).Table("application_approvers").Where("user_id = ?", userID).Count(&alreadyApprover)
	if alreadyApprover > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User is already an approver")
	}

	var createdByID int
	if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", createdByUsername).Row().Scan(&createdByID); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Current user not found")
	}

	approver := models.ApplicationApprover{
		UserID:    userID,
		CreatedBy: &createdByID,
	}
	if err := s.db.WithContext(ctx).Create(&approver).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating approver")
	}

	// Снимок имени: approver_name в details (поле было плоской колонкой в старой таблице).
	approverName := s.resolveUserName(ctx, userID)
	s.recorder.Log(ctx, nil, models.AuditEntityApprover, &userID, models.ApproverActionCreated, &createdByID,
		map[string]any{"approver_name": approverName})
	return nil
}

// Update задаёт или снимает маску отображаемого имени принимающего. Пустая строка/только
// пробелы -> NULL (показывается реальное ФИО). Изменение фиксируется в аудите принимающих.
func (s *approverService) Update(ctx context.Context, id int, displayName *string, actorUsername string) error {
	var mask *string
	if displayName != nil {
		if trimmed := strings.TrimSpace(*displayName); trimmed != "" {
			mask = &trimmed
		}
	}

	result := s.db.WithContext(ctx).
		Model(&models.ApplicationApprover{}).
		Where("id = ?", id).
		Update("display_name", mask)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating approver")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Approver not found")
	}

	// Аудит best-effort: снимок user_id и реального имени принимающего + новая маска.
	var userID int
	if err := s.db.WithContext(ctx).Table("application_approvers").Select("user_id").Where("id = ?", id).Row().Scan(&userID); err == nil && userID > 0 {
		var actorID *int
		var aid int
		if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", actorUsername).Row().Scan(&aid); err == nil {
			actorID = &aid
		}
		details := map[string]any{"approver_name": s.resolveUserName(ctx, userID)}
		if mask != nil {
			details["display_name"] = *mask
		}
		s.recorder.Log(ctx, nil, models.AuditEntityApprover, &userID, models.ApproverActionRenamed, actorID, details)
	}
	return nil
}

// Delete удаляет утверждающего по ID.
func (s *approverService) Delete(ctx context.Context, id int, actorUsername string) error {
	// Снимок до удаления: берём user_id и имя принимающего.
	type approverRow struct {
		UserID int
		Name   string
	}
	var snap approverRow
	snapErr := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), ''), u.username, '') AS name`).
		Joins("JOIN users u ON u.id = aa.user_id").
		Where("aa.id = ?", id).
		Row().
		Scan(&snap.UserID, &snap.Name)
	// snapErr не роняет удаление — аудит best-effort.

	result := s.db.WithContext(ctx).Delete(&models.ApplicationApprover{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting approver")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Approver not found")
	}

	if snapErr == nil && snap.UserID > 0 {
		var actorID *int
		var aid int
		if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", actorUsername).Row().Scan(&aid); err == nil {
			actorID = &aid
		}
		s.recorder.Log(ctx, nil, models.AuditEntityApprover, &snap.UserID, models.ApproverActionDeleted, actorID,
			map[string]any{"approver_name": snap.Name})
	}
	return nil
}

// GetHistory возвращает глобальный журнал принимающих (новые сверху).
// Read-switch #870 (F.3): до-cutover строки application_approver_histories подняты в
// audit_log разовым backfill'ом (approver_name из плоской колонки свёрнут в
// details->>'approver_name'), читаем только audit_log. Форму стережёт
// TestApprovers_History_BackfillLegacyIntoAudit. История глобальная (без entity_id).
func (s *approverService) GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.entity_id AS approver_user_id,
			COALESCE(a.details->>'approver_name', '') AS approver_name,
			a.action AS action_type, a.actor_user_id AS actor_user_id,
			` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID             int       `gorm:"column:id"`
		ApproverUserID int       `gorm:"column:approver_user_id"`
		ApproverName   string    `gorm:"column:approver_name"`
		ActionType     string    `gorm:"column:action_type"`
		ActorUserID    *int      `gorm:"column:actor_user_id"`
		ActorName      string    `gorm:"column:actor_name"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityApprover).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approver history")
	}

	// Логин вместо ФИО у обоих участников записи: и у актора, и у принимающего,
	// которого добавили или сняли - это тоже персональные данные работника.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.ApplicationApproverHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.ApplicationApproverHistoryItem{
			ID:             r.ID,
			ApproverUserID: r.ApproverUserID,
			ApproverName:   maskName(masks, &r.ApproverUserID, r.ApproverName),
			ActionType:     r.ActionType,
			ActorUserID:    r.ActorUserID,
			ActorName:      maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:      r.CreatedAt,
		})
	}
	return items, nil
}

// resolveUserName возвращает форматированное имя пользователя по ID
// (фамилия имя отчество или username как фолбэк).
func (s *approverService) resolveUserName(ctx context.Context, userID int) string {
	var name string
	_ = s.db.WithContext(ctx).
		Table("users").
		Select(`COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', last_name, first_name, middle_name)), ''), username, '')`).
		Where("id = ?", userID).
		Row().
		Scan(&name)
	return name
}

// IsApprover проверяет, числится ли пользователь принимающим заявки.
func (s *approverService) IsApprover(ctx context.Context, username string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Joins("JOIN users u ON u.id = aa.user_id").
		Where("u.username = ?", username).
		Count(&count).Error
	if err != nil {
		slog.Error("Ошибка проверки принимающего", "username", username, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error checking approver")
	}
	return count > 0, nil
}

// IsReviewer проверяет, назначен ли пользователь согласующим в организации или компании.
func (s *approverService) IsReviewer(ctx context.Context, username string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM organization_users ou
			JOIN users u ON u.id = ou.user_id
			WHERE u.username = ? AND ou.required_approval = true
		) OR EXISTS (
			SELECT 1 FROM companies_users cu
			JOIN users u ON u.id = cu.user_id
			WHERE u.username = ? AND cu.required_approval = true
		)`
	var isReviewer bool
	if err := s.db.WithContext(ctx).Raw(q, username, username).Scan(&isReviewer).Error; err != nil {
		slog.Error("Ошибка проверки согласующего", "username", username, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error checking reviewer")
	}
	return isReviewer, nil
}
