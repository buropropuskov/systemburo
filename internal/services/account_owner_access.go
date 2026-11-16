package services

import (
	"context"
	"errors"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

type AccountOwnerAccess struct {
	Verify  bool `json:"verify"`
	Confirm bool `json:"confirm"`
}

// Access requires a direct grant. The generic admin and super-admin grants
// from PermissionSet.Has do not authorize disclosure.
func (s *AccountOwnerStore) Access(ctx context.Context, actorID int) (AccountOwnerAccess, error) {
	identity, ok := SessionIdentityFromContext(ctx)
	if !ok || actorID <= 0 || identity.EffectiveUserID != actorID ||
		identity.OwnerUserID != actorID || identity.ImpersonatedBy != nil {
		return AccountOwnerAccess{}, nil
	}

	var permitted bool
	err := s.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 FROM users u
			JOIN user_permission_overrides p ON p.user_id = u.id
			LEFT JOIN user_types t ON t.id = u.type_id
			WHERE u.id = ? AND u.is_active AND NOT u.is_banned
				AND (u.is_admin OR u.is_super_admin OR t.code = ?)
				AND p.permission_key = ? AND p.value = 'allow'
		)`, actorID, "buropropuskov", KeyActionVerifyAccountOwner).Scan(&permitted).Error
	if err != nil || !permitted {
		return AccountOwnerAccess{}, err
	}

	set, err := NewPermissionResolver(s.db).ResolveFresh(ctx, actorID)
	if err != nil {
		return AccountOwnerAccess{}, err
	}
	if set.IsInactive() || set.IsBanned() {
		return AccountOwnerAccess{}, nil
	}
	return AccountOwnerAccess{Verify: true, Confirm: set.Has(KeyPageAdminUsers)}, nil
}

type AccountOwnerConfirmationStatus struct {
	Revision      int        `json:"revision"`
	EffectiveFrom *time.Time `json:"effective_from,omitempty"`
}

func (s *AccountOwnerStore) ConfirmationStatus(ctx context.Context, userID int) (AccountOwnerConfirmationStatus, error) {
	if userID <= 0 {
		return AccountOwnerConfirmationStatus{}, ErrInvalidAccountOwnerConfirmation
	}
	var user models.User
	if err := s.db.WithContext(ctx).Select("id").Take(&user, "id = ?", userID).Error; err != nil {
		return AccountOwnerConfirmationStatus{}, err
	}
	record, err := latestAccountOwnerConfirmation(s.db.WithContext(ctx), userID)
	if err != nil || record.ID == 0 {
		return AccountOwnerConfirmationStatus{}, err
	}
	return AccountOwnerConfirmationStatus{Revision: record.ID, EffectiveFrom: &record.EffectiveFrom}, nil
}

func IsAccountOwnerRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
