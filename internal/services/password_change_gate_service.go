package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PasswordChangeGateService отвечает на единственный вопрос: обязан ли пользователь
// задать свой пароль прямо сейчас (users.must_change_password, #1911).
//
// Флаг читается ИЗ БАЗЫ, а не из полезной нагрузки маркера доступа: маркер живёт до
// JWT_ACCESS_TTL (15 минут по умолчанию) и версии пароля не несёт, поэтому после
// плановой рассылки паролей выданный до неё маркер продолжал бы открывать систему
// целиком. Чтение из базы закрывает это окно сразу.
//
// Кэшируется ТОЛЬКО отрицательный ответ, и это не экономия ради экономии.
// Положительный ответ в кэше пережил бы смену пароля: человек задал новый, флаг в
// базе уже снят, а гейт на весь TTL возвращал бы его в то же окно, где сервис
// отвечает "новый пароль совпадает с текущим" - тупик на ровном месте. Снимать флаг
// умеет не только UpdatePassword (плановая ротация, действия администратора), и
// каждому такому месту пришлось бы помнить про сброс кэша. Отрицательный ответ
// протухает сам: поднятый флаг начинает действовать не позже TTL, как и блокировка
// в BanCheckService.
type PasswordChangeGateService struct {
	db  *gorm.DB
	ttl time.Duration
	// cache: userID -> момент, до которого известно, что смена НЕ требуется.
	cache sync.Map
}

// NewPasswordChangeGateService создаёт сервис с заданным TTL кэша (в проде 30s,
// в тестах 0 - каждый вызов идёт в базу).
func NewPasswordChangeGateService(db *gorm.DB, ttl time.Duration) *PasswordChangeGateService {
	return &PasswordChangeGateService{db: db, ttl: ttl}
}

// Required сообщает, поднят ли у пользователя флаг обязательной смены пароля.
// При ошибке базы возвращает (false, err): вызывающий middleware делает fail-open,
// как проверка блокировки, - временный сбой не должен запирать весь API.
func (s *PasswordChangeGateService) Required(ctx context.Context, userID int) (bool, error) {
	if v, ok := s.cache.Load(userID); ok {
		if time.Now().Before(v.(time.Time)) {
			return false, nil
		}
	}

	var user models.User
	if err := s.db.WithContext(ctx).Select("id, must_change_password").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Несуществующий пользователь - не забота этого гейта: маркер на
			// удалённую запись режет проверка блокировки. Требование сменить пароль
			// из пустоты не выдумываем.
			return false, nil
		}
		return false, err
	}

	if !user.MustChangePassword {
		s.cache.Store(userID, time.Now().Add(s.ttl))
	}
	return user.MustChangePassword, nil
}

// Invalidate сбрасывает отрицательную запись кэша. Нужен там, где флаг ПОДНИМАЮТ и
// ждать TTL не хочется (плановая ротация паролей). Снятия флага не касается -
// положительный ответ не кэшируется вовсе.
func (s *PasswordChangeGateService) Invalidate(userID int) {
	s.cache.Delete(userID)
}
