package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const AccessTokenVersion = 2

// SessionAccess verifies current server state. RequireLive never memoizes a
// previous check: tickets and long-lived streams must see subsequent revocation.
type SessionAccess interface {
	ResolveAccess(context.Context, *Claims) (models.SessionIdentity, error)
	ResolveRefreshCookie(context.Context, string) (models.SessionIdentity, error)
	ResolveLogout(context.Context, *Claims) (models.SessionIdentity, error)
	RequireLive(context.Context, models.SessionIdentity) (models.SessionIdentity, error)
}

type sessionAccessService struct {
	db            *gorm.DB
	refreshSecret []byte
	now           func() time.Time
}

type SessionAccessOption func(*sessionAccessService)

// WithSessionAccessClock controls expiry checks in deterministic tests.
func WithSessionAccessClock(now func() time.Time) SessionAccessOption {
	return func(s *sessionAccessService) {
		if now != nil {
			s.now = now
		}
	}
}

func NewSessionAccessService(db *gorm.DB, refreshSecret string, opts ...SessionAccessOption) SessionAccess {
	s := &sessionAccessService{db: db, refreshSecret: []byte(refreshSecret), now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type sessionIdentityKey struct{}

func WithSessionIdentity(ctx context.Context, identity models.SessionIdentity) context.Context {
	if identity.ImpersonatedBy != nil {
		actor := *identity.ImpersonatedBy
		identity.ImpersonatedBy = &actor
	}
	return context.WithValue(ctx, sessionIdentityKey{}, identity)
}

func SessionIdentityFromContext(ctx context.Context) (models.SessionIdentity, bool) {
	if ctx == nil {
		return models.SessionIdentity{}, false
	}
	identity, ok := ctx.Value(sessionIdentityKey{}).(models.SessionIdentity)
	return identity, ok
}

func invalidSession() error {
	return apperr.Unauthorized("Сессия завершена. Войдите снова.")
}

func sessionCheckUnavailable(err error) error {
	return apperr.New(http.StatusServiceUnavailable, "Проверка сессии временно недоступна", err)
}

func validSessionBinding(identity models.SessionIdentity) bool {
	if identity.OwnerUserID <= 0 || identity.EffectiveUserID <= 0 || identity.AccessExpiresAt.IsZero() {
		return false
	}
	if _, err := uuid.Parse(identity.FamilyID); err != nil {
		return false
	}
	if identity.ImpersonatedBy == nil {
		return identity.OwnerUserID == identity.EffectiveUserID
	}
	return *identity.ImpersonatedBy == identity.OwnerUserID && identity.OwnerUserID != identity.EffectiveUserID
}

func (s *sessionAccessService) user(ctx context.Context, id int) (*models.User, error) {
	if s.db == nil {
		return nil, sessionCheckUnavailable(nil)
	}
	var user models.User
	err := s.db.WithContext(ctx).Select("id, username, type_id, is_active, is_banned, is_admin, is_super_admin, must_change_password").First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, invalidSession()
	}
	if err != nil {
		return nil, sessionCheckUnavailable(err)
	}
	return &user, nil
}

func (s *sessionAccessService) ResolveAccess(ctx context.Context, claims *Claims) (models.SessionIdentity, error) {
	if claims == nil || claims.TokenVersion != AccessTokenVersion || claims.ExpiresAt == nil || claims.Subject == "" {
		return models.SessionIdentity{}, invalidSession()
	}
	identity := models.SessionIdentity{EffectiveUserID: claims.UserID, OwnerUserID: claims.SessionUserID,
		FamilyID: claims.SessionID, ImpersonatedBy: claims.ImpersonatedBy, AccessExpiresAt: claims.ExpiresAt.Time, Username: claims.Subject}
	return s.verify(ctx, identity)
}

func (s *sessionAccessService) RequireLive(ctx context.Context, identity models.SessionIdentity) (models.SessionIdentity, error) {
	return s.verify(ctx, identity)
}

func (s *sessionAccessService) verify(ctx context.Context, identity models.SessionIdentity) (models.SessionIdentity, error) {
	if !validSessionBinding(identity) || !identity.AccessExpiresAt.After(s.now()) {
		return models.SessionIdentity{}, invalidSession()
	}
	owner, err := s.user(ctx, identity.OwnerUserID)
	if err != nil {
		return models.SessionIdentity{}, err
	}
	if !owner.IsActive {
		if identity.ImpersonatedBy != nil {
			return models.SessionIdentity{}, invalidSession()
		}
		return models.SessionIdentity{}, apperr.Forbidden("Учётная запись отключена")
	}
	var live bool
	err = s.db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM refresh_tokens WHERE user_id = ? AND family_id = ? AND is_revoked = false AND expires_at > ?)`, owner.ID, identity.FamilyID, s.now().UTC()).Scan(&live).Error
	if err != nil {
		return models.SessionIdentity{}, sessionCheckUnavailable(err)
	}
	if !live {
		return models.SessionIdentity{}, invalidSession()
	}
	target := owner
	if identity.ImpersonatedBy != nil {
		target, err = s.user(ctx, identity.EffectiveUserID)
		if err != nil {
			return models.SessionIdentity{}, err
		}
		if !target.IsActive || target.IsBanned || target.MustChangePassword || owner.MustChangePassword {
			return models.SessionIdentity{}, invalidSession()
		}
		if err := authorizeAdminActor(ctx, s.db, owner, KeyUserImpersonate); err != nil {
			return models.SessionIdentity{}, impersonationSessionError(err)
		}
		policy := &impersonationService{db: s.db, resolver: NewPermissionResolver(s.db)}
		if err := policy.checkRank(ctx, owner, target); err != nil {
			return models.SessionIdentity{}, impersonationSessionError(err)
		}
	}
	if identity.Username != "" && identity.Username != target.Username {
		return models.SessionIdentity{}, invalidSession()
	}
	identity.Username, identity.OwnerUsername = target.Username, owner.Username
	identity.TypeID, identity.IsSuperAdmin = target.TypeID, target.IsSuperAdmin
	return identity, nil
}

func impersonationSessionError(err error) error {
	var domain *apperr.Error
	if errors.As(err, &domain) && domain.Code >= 400 && domain.Code < 500 {
		return invalidSession()
	}
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code >= 400 && httpErr.Code < 500 {
		return invalidSession()
	}
	return sessionCheckUnavailable(err)
}

// ResolveLogout authenticates only the identity needed for owner-bound cleanup.
// It grants no live-session access and cannot be reused for logout-all or reads.
func (s *sessionAccessService) ResolveLogout(ctx context.Context, claims *Claims) (models.SessionIdentity, error) {
	if claims == nil || claims.UserID <= 0 || claims.Subject == "" || claims.ExpiresAt == nil || !claims.ExpiresAt.Time.After(s.now()) {
		return models.SessionIdentity{}, invalidSession()
	}
	ownerID := claims.UserID
	if claims.ImpersonatedBy != nil {
		ownerID = *claims.ImpersonatedBy
	}
	if claims.TokenVersion != 0 {
		identity := models.SessionIdentity{EffectiveUserID: claims.UserID, OwnerUserID: claims.SessionUserID, FamilyID: claims.SessionID,
			ImpersonatedBy: claims.ImpersonatedBy, AccessExpiresAt: claims.ExpiresAt.Time}
		if claims.TokenVersion != AccessTokenVersion || !validSessionBinding(identity) {
			return models.SessionIdentity{}, invalidSession()
		}
		ownerID = claims.SessionUserID
	}
	if ownerID <= 0 {
		return models.SessionIdentity{}, invalidSession()
	}
	owner, err := s.user(ctx, ownerID)
	if err != nil {
		return models.SessionIdentity{}, err
	}
	if claims.ImpersonatedBy == nil && owner.Username != claims.Subject {
		return models.SessionIdentity{}, invalidSession()
	}
	return models.SessionIdentity{EffectiveUserID: owner.ID, OwnerUserID: owner.ID, Username: owner.Username, OwnerUsername: owner.Username,
		TypeID: owner.TypeID, IsSuperAdmin: owner.IsSuperAdmin, AccessExpiresAt: claims.ExpiresAt.Time}, nil
}

// ResolveRefreshCookie is read-only: the presented token must itself be live.
// An active descendant never makes an already revoked cookie valid again.
func (s *sessionAccessService) ResolveRefreshCookie(ctx context.Context, raw string) (models.SessionIdentity, error) {
	claims, err := DecodeRefreshToken(raw, s.refreshSecret)
	if err != nil || claims == nil || claims.Subject == "" {
		return models.SessionIdentity{}, invalidSession()
	}
	if s.db == nil {
		return models.SessionIdentity{}, sessionCheckUnavailable(nil)
	}
	var token models.RefreshToken
	err = s.db.WithContext(ctx).Where("token_hash = ? AND is_revoked = false AND expires_at > ?", hashRefreshToken(raw), s.now().UTC()).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.SessionIdentity{}, invalidSession()
	}
	if err != nil {
		return models.SessionIdentity{}, sessionCheckUnavailable(err)
	}
	owner, err := s.user(ctx, token.UserID)
	if err != nil {
		return models.SessionIdentity{}, err
	}
	if !owner.IsActive {
		return models.SessionIdentity{}, apperr.Forbidden("Учётная запись отключена")
	}
	identity := models.SessionIdentity{EffectiveUserID: owner.ID, OwnerUserID: owner.ID, FamilyID: token.FamilyID,
		AccessExpiresAt: token.ExpiresAt, Username: owner.Username, OwnerUsername: owner.Username, TypeID: owner.TypeID, IsSuperAdmin: owner.IsSuperAdmin}
	if owner.Username != claims.Subject || !validSessionBinding(identity) {
		return models.SessionIdentity{}, invalidSession()
	}
	return identity, nil
}
