package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/apperr"

	"gorm.io/gorm"
)

// Application actions retain their existing object/assignment requirements.
// An explicit denial removes that entitlement; an absent grant does not create
// a new assignment and does not remove an existing object-based entitlement.
func applicationActionAllowed(set PermissionSet, key string) bool {
	if key != KeyActionApproveApplication && key != KeyActionForwardApplication {
		return false
	}
	if set.IsBanned() || set.IsInactive() {
		return false
	}
	if set.IsSuperAdmin() {
		return true
	}
	_, denied := set.denies[key]
	return !denied
}

func requireApplicationAction(ctx context.Context, db *gorm.DB, userID int, key string) error {
	allowed, err := applicationActionAvailable(ctx, db, userID, key)
	if err != nil {
		return err
	}
	if !allowed {
		return apperr.Forbidden("Это действие вам недоступно")
	}
	return nil
}

func applicationActionAvailable(ctx context.Context, db *gorm.DB, userID int, key string) (bool, error) {
	set, err := NewPermissionResolver(db).ResolveFresh(ctx, userID)
	if err != nil {
		return false, apperr.New(http.StatusServiceUnavailable, "Не удалось проверить доступность действия", err)
	}
	return applicationActionAllowed(set, key), nil
}

// applicationAssignee is the directory-defined assignment, not a client grant.
type applicationAssignee struct {
	UserID           int
	IsPrimary        bool
	RequiredApproval bool
}

func prepareApplicationAssignees(ctx context.Context, db *gorm.DB, applicationID, actorID int, users []applicationAssignee, primary *int, now time.Time) ([]applicationAssignee, *int, error) {
	eligible := make([]applicationAssignee, 0, len(users))
	for _, user := range users {
		allowed, err := applicationActionAvailable(ctx, db, user.UserID, KeyActionApproveApplication)
		if err != nil {
			return nil, nil, err
		}
		if allowed {
			eligible = append(eligible, user)
			continue
		}
		if user.RequiredApproval {
			return nil, nil, apperr.Validation("Один из обязательных согласующих не может согласовать вашу заявку. Выберите другого согласующего")
		}
		// Preserve the previous visibility without granting a vote.
		if err := db.WithContext(ctx).Exec(`INSERT INTO application_viewers (application_id, user_id, created_at, created_by) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`, applicationID, user.UserID, now, actorID).Error; err != nil {
			return nil, nil, apperr.Internal("Не удалось добавить участника заявки", err)
		}
	}
	var nextPrimary *int
	for _, user := range eligible {
		if primary != nil && user.UserID == *primary {
			return eligible, primary, nil
		}
		if nextPrimary == nil && user.IsPrimary {
			id := user.UserID
			nextPrimary = &id
		}
	}
	return eligible, nextPrimary, nil
}

// Availability is presentation metadata. Actions and assignments always use the
// uncached check above, even when the displayed state was loaded earlier.
func (s *applicationService) applicationDisplayResolver(db *gorm.DB) *PermissionResolver {
	// A transaction must use its own connection and snapshot, even with a one-connection pool.
	if _, inTransaction := db.Statement.ConnPool.(gorm.TxCommitter); inTransaction {
		return NewPermissionResolver(db)
	}
	if s.permissionResolver != nil {
		return s.permissionResolver
	}
	return NewPermissionResolver(db)
}
func applicationApprovalDisplayAvailable(ctx context.Context, resolver *PermissionResolver, userID int) (bool, error) {
	set, err := resolver.Resolve(ctx, userID)
	if err != nil {
		return false, apperr.New(http.StatusServiceUnavailable, "Не удалось загрузить состояние участника", err)
	}
	return applicationActionAllowed(set, KeyActionApproveApplication), nil
}
