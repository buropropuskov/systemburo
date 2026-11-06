package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"systemburo/internal/models"
)

// PDConsentGateService отвечает на вопрос "нужно ли у этого пользователя спросить
// согласие на обработку ПД" (#1567). Будет вызываться глобальным middleware на
// каждом protected-запросе, поэтому оба слагаемых ответа кэшируются с TTL.
//
// Кэшируется именно ПРИНЯТАЯ пользователем редакция, а не булево "согласия нет":
// иначе после выдачи согласия оставалось бы окно длиной в TTL, где фронт оверлей
// уже снял, а API продолжает отказывать. Сравнение принятой редакции с требуемой
// закрывает и выдачу согласия (Invalidate), и подъём редакции (InvalidateAll).
type PDConsentGateService struct {
	consents ConsentService
	settings SettingsService
	ttl      time.Duration

	mu              sync.RWMutex
	requirement     PDConsentRequirement
	requirementAt   time.Time
	cacheGeneration uint64

	accepted sync.Map // userID(int) -> acceptedEntry
	// Only outstanding reads have an epoch; finished reads leave no extra state.
	acceptedReads map[int]*consentReadEpoch
}

type consentReadEpoch struct {
	generation uint64
	readers    int
}

var errConsentStateChanged = errors.New("consent state changed during read")

const consentReadAttempts = 3

// PDConsentRequirement -- требование системы: что и какой редакции нужно принять.
type PDConsentRequirement struct {
	// Enabled -- запрос согласия реально работает: тумблер включён И текст задан.
	// Пустой текст при включённом тумблере это ошибка настройки, и гейт обязан
	// пропускать (иначе система закрыта, а показать нечего).
	Enabled bool
	// Requested -- сырое состояние тумблера, без учёта текста. Отличает
	// "администратор не включал запрос" от "включил, но текст пуст": второе -
	// ошибка настройки, о которой гейт обязан сказать в журнал.
	Requested bool
	Version   int
	// VersionAt -- когда появилась действующая редакция (RFC3339, пусто у настроек
	// до появления поля).
	VersionAt string
	Text      string
	Hash      string
}

type acceptedEntry struct {
	version   int
	expiresAt time.Time
}

// NewPDConsentGateService создаёт сервис проверки согласия с заданным TTL кэша
// (рекомендуется 30s, как у BanCheckService).
func NewPDConsentGateService(consents ConsentService, settings SettingsService, ttl time.Duration) *PDConsentGateService {
	return &PDConsentGateService{consents: consents, settings: settings, ttl: ttl}
}

// Requirement возвращает текущее требование системы. На cache miss читает настройки
// и считает sha256 текста.
func (s *PDConsentGateService) Requirement(ctx context.Context) (PDConsentRequirement, error) {
	for attempt := 0; attempt < consentReadAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return PDConsentRequirement{}, err
		}
		s.mu.RLock()
		if time.Now().Before(s.requirementAt) {
			req := s.requirement
			s.mu.RUnlock()
			return req, nil
		}
		generation := s.cacheGeneration
		s.mu.RUnlock()

		req, err := s.CurrentRequirement(ctx)
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			return PDConsentRequirement{}, err
		}

		s.mu.Lock()
		fresh := generation == s.cacheGeneration
		if fresh {
			s.requirement = req
			s.requirementAt = time.Now().Add(s.ttl)
		}
		s.mu.Unlock()
		if fresh {
			return req, nil
		}
	}
	return PDConsentRequirement{}, errConsentStateChanged
}

func consentRequirement(settings *models.PDConsentSettings) PDConsentRequirement {
	sum := sha256.Sum256([]byte(settings.Text))
	return PDConsentRequirement{
		Enabled:   settings.Required && hasVisibleText(settings.Text),
		Requested: settings.Required,
		Version:   settings.Version,
		VersionAt: settings.VersionAt,
		Text:      settings.Text,
		Hash:      hex.EncodeToString(sum[:]),
	}
}

// CurrentRequirement читает снимок для предъявления пользователю в обход кеша middleware.
func (s *PDConsentGateService) CurrentRequirement(ctx context.Context) (PDConsentRequirement, error) {
	settings, err := s.settings.GetPDConsentSettings(ctx)
	if err != nil {
		return PDConsentRequirement{}, err
	}
	return consentRequirement(settings), nil
}

// AcceptedVersion возвращает максимальную редакцию действующего согласия
// пользователя на обработку ПД; 0 означает "не соглашался".
func (s *PDConsentGateService) AcceptedVersion(ctx context.Context, userID int) (int, error) {
	for attempt := 0; attempt < consentReadAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.mu.Lock()
		if v, ok := s.accepted.Load(userID); ok {
			entry := v.(acceptedEntry)
			if time.Now().Before(entry.expiresAt) {
				s.mu.Unlock()
				return entry.version, nil
			}
		}
		generation := s.cacheGeneration
		if s.acceptedReads == nil {
			s.acceptedReads = make(map[int]*consentReadEpoch)
		}
		epoch := s.acceptedReads[userID]
		if epoch == nil {
			epoch = &consentReadEpoch{}
			s.acceptedReads[userID] = epoch
		}
		epoch.readers++
		userGeneration := epoch.generation
		s.mu.Unlock()
		version, err := s.consents.ActiveVersion(ctx, userID, ConsentTypePDProcessing)
		if err == nil {
			err = ctx.Err()
		}
		s.mu.Lock()
		epoch.readers--
		if epoch.readers == 0 {
			delete(s.acceptedReads, userID)
		}
		fresh := generation == s.cacheGeneration && userGeneration == epoch.generation
		if err == nil && fresh {
			s.accepted.Store(userID, acceptedEntry{version: version, expiresAt: time.Now().Add(s.ttl)})
		}
		s.mu.Unlock()
		if err != nil {
			return 0, err
		}
		if fresh {
			return version, nil
		}
	}
	return 0, errConsentStateChanged
}

// NeedsConsent сообщает, нужно ли требовать согласие у этого пользователя.
func (s *PDConsentGateService) NeedsConsent(ctx context.Context, userID int) (bool, error) {
	req, err := s.Requirement(ctx)
	if err != nil {
		return false, err
	}
	if !req.Enabled {
		return false, nil
	}
	accepted, err := s.AcceptedVersion(ctx, userID)
	if err != nil {
		return false, err
	}
	return accepted < req.Version, nil
}

// Invalidate сбрасывает кэш принятой редакции пользователя. Обязателен после выдачи
// и отзыва согласия, иначе доступ открывается/закрывается лишь по истечении TTL.
func (s *PDConsentGateService) Invalidate(userID int) {
	s.mu.Lock()
	if epoch := s.acceptedReads[userID]; epoch != nil {
		epoch.generation++
	}
	s.accepted.Delete(userID)
	s.mu.Unlock()
}

// InvalidateAll сбрасывает кэш требования и всех принятых редакций. Нужен, когда
// администратор поднял редакцию или поменял настройки согласия.
func (s *PDConsentGateService) InvalidateAll() {
	s.mu.Lock()
	s.cacheGeneration++
	s.requirement = PDConsentRequirement{}
	s.requirementAt = time.Time{}
	s.accepted.Range(func(key, _ any) bool {
		s.accepted.Delete(key)
		return true
	})
	s.mu.Unlock()
}
