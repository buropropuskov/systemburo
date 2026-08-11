package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// ImpersonationTokenTTL - срок жизни маркера доступа, выданного от чужого имени
// (#1912). Заметно короче обычного сеанса: режим нужен на время разбора одной
// проблемы, а забытая вкладка не должна оставаться чужой учётной записью на
// рабочий день. Продления нет - маркер обновления администратору не выдаётся,
// по истечении срока сеанс сам возвращается в свою учётную запись.
const ImpersonationTokenTTL = 30 * time.Minute

// ImpersonationService - режим «войти как пользователь»: выдача маркера доступа от
// чужого имени и возврат в свою учётную запись (#1912). Заменяет практику, в
// которой администратор знал пароль работника: пароль остаётся только у владельца,
// а журнал перестаёт приписывать чужие действия работнику.
type ImpersonationService interface {
	// Start выдаёт маркер доступа от имени targetUserID. Возвращает ошибку доступа,
	// если у цели прав больше, чем у инициатора, - иначе режим был бы способом
	// поднять себе полномочия.
	Start(ctx context.Context, actorUserID, targetUserID int) (*models.ImpersonationResponse, error)
	// Stop закрывает сеанс записью в журнал. Сам маркер отзывать нечего: он не
	// хранится в базе, а свою учётную запись клиент возвращает обычным обновлением
	// маркера по своей же cookie.
	Stop(ctx context.Context, actorUserID, targetUserID int) error
}

type impersonationService struct {
	db        *gorm.DB
	jwtSecret []byte
	resolver  *PermissionResolver
	audit     AuditRecorder
}

// NewImpersonationService создаёт сервис режима «войти как пользователь».
func NewImpersonationService(db *gorm.DB, jwtSecret string, resolver *PermissionResolver, audit AuditRecorder) ImpersonationService {
	return &impersonationService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		resolver:  resolver,
		audit:     audit,
	}
}

func (s *impersonationService) Start(ctx context.Context, actorUserID, targetUserID int) (*models.ImpersonationResponse, error) {
	if actorUserID == targetUserID {
		return nil, apperr.Validation("Вы уже работаете под этой учётной записью")
	}

	actor, err := s.loadUser(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	target, err := s.loadUser(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	// Отключённая и заблокированная учётные записи не пускают и своего владельца:
	// войти от их имени значило бы получить доступ, которого нет ни у кого.
	if !target.IsActive {
		return nil, apperr.Validation("Учётная запись отключена: войти от её имени нельзя")
	}
	if target.IsBanned {
		return nil, apperr.Validation("Учётная запись заблокирована: войти от её имени нельзя")
	}
	// Учётной записи уже велено сменить пароль (#1911): её собственный доступ закрыт
	// до смены, а смена пароля в режиме запрещена. Сеанс получился бы тупиком - окно
	// смены пароля поверх пустой системы, - поэтому отказываем сразу и словами.
	if target.MustChangePassword {
		return nil, apperr.Validation(
			"Пользователю назначена обязательная смена пароля: до неё войти от его имени нельзя")
	}

	if err := s.checkRank(ctx, actor, target); err != nil {
		return nil, err
	}

	token, expiresAt, err := s.signToken(target, actor.ID)
	if err != nil {
		return nil, err
	}

	// Запись входа в режим критична: без неё режим превращается в тот же безымянный
	// доступ под чужой учётной записью, ради ухода от которого он и заводится.
	// Поэтому Record с проверкой ошибки, а не best-effort Log.
	if err := s.audit.Record(ctx, nil, models.AuditEntityUser, &target.ID,
		models.AuditActionImpersonateStart, &actor.ID,
		models.ImpersonationAuditDetails{
			Comment:        fmt.Sprintf("Вход в систему от имени пользователя %s", displayName(target)),
			ActorUsername:  actor.Username,
			TargetUsername: target.Username,
			ExpiresAt:      &expiresAt,
		}); err != nil {
		return nil, fmt.Errorf("не записан вход в режим от имени пользователя: %w", err)
	}

	return &models.ImpersonationResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		Target: models.ImpersonationTarget{
			ID:       target.ID,
			Username: target.Username,
			FullName: displayName(target),
		},
	}, nil
}

func (s *impersonationService) Stop(ctx context.Context, actorUserID, targetUserID int) error {
	actor, err := s.loadUser(ctx, actorUserID)
	if err != nil {
		return err
	}
	target, err := s.loadUser(ctx, targetUserID)
	if err != nil {
		return err
	}

	if err := s.audit.Record(ctx, nil, models.AuditEntityUser, &target.ID,
		models.AuditActionImpersonateStop, &actor.ID,
		models.ImpersonationAuditDetails{
			Comment:        fmt.Sprintf("Выход из режима работы от имени пользователя %s", displayName(target)),
			ActorUsername:  actor.Username,
			TargetUsername: target.Username,
		}); err != nil {
		return fmt.Errorf("не записан выход из режима от имени пользователя: %w", err)
	}
	return nil
}

// checkRank запрещает вход от имени того, у кого прав не меньше, чем у инициатора.
// Без этой проверки режим - готовый способ поднять себе полномочия: достаточно
// получить право входа от чужого имени и выбрать более полномочного человека.
//
// Порядок проверок идёт от грубого к точному: сначала признаки учётной записи
// (супер-администратор и администратор их не теряют ни при каком наборе прав),
// затем поимённая сверка ключей. Точечная сверка нужна и в паре «обычный - обычный»:
// два рядовых пользователя вполне могут различаться правами вроде разбора
// организаций или журналов.
func (s *impersonationService) checkRank(ctx context.Context, actor, target *models.User) error {
	actorSet, err := s.resolver.Resolve(ctx, actor.ID)
	if err != nil {
		return err
	}
	targetSet, err := s.resolver.Resolve(ctx, target.ID)
	if err != nil {
		return err
	}

	// Супер-администратор - потолок прав: от его имени не входит никто, включая
	// другого супер-администратора (равный уровень ничего не даёт, а спутанность
	// двух полных доступов в журнале дороже удобства).
	if targetSet.IsSuperAdmin() {
		return apperr.Forbidden("Нельзя войти от имени супер-администратора")
	}
	if targetSet.IsAdmin() && !actorSet.IsSuperAdmin() {
		return apperr.Forbidden("Войти от имени администратора может только супер-администратор")
	}
	if actorSet.IsSuperAdmin() {
		return nil
	}
	for _, key := range targetSet.Keys() {
		if !actorSet.Has(key) {
			return apperr.Forbidden(fmt.Sprintf(
				"У пользователя есть право «%s», которого нет у вас: войти от его имени нельзя", key))
		}
	}
	return nil
}

// signToken выпускает маркер доступа от имени target. Личность в маркере - целевая
// (идентификатор, логин, тип), инициатор идёт отдельным полем: проверки доступа
// обязаны считать права по тому, от чьего имени работают, а журнал - знать, кто это
// затеял. Секрет тот же, что у обычного входа, поэтому проверка маркера не меняется.
func (s *impersonationService) signToken(target *models.User, actorUserID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(ImpersonationTokenTTL)
	actor := actorUserID
	claims := Claims{
		UserID:         target.ID,
		TypeID:         target.TypeID,
		IsSuperAdmin:   target.IsSuperAdmin,
		ImpersonatedBy: &actor,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   target.Username,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("подписание маркера доступа от чужого имени: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *impersonationService) loadUser(ctx context.Context, userID int) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, apperr.NotFound("Пользователь не найден")
	}
	return &user, nil
}

// displayName - ФИО для полосы «вы работаете от имени такого-то» и для журнала.
// Считается тем же fullName, что и остальные упоминания людей в аудите, - имя в
// полосе и имя в журнале обязаны совпадать буква в букву.
func displayName(u *models.User) string {
	return fullName(u.LastName, u.FirstName, u.Username)
}
