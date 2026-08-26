package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
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

	mu            sync.RWMutex
	requirement   PDConsentRequirement
	requirementAt time.Time

	accepted sync.Map // userID(int) -> acceptedEntry
}

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
	s.mu.RLock()
	if time.Now().Before(s.requirementAt) {
		req := s.requirement
		s.mu.RUnlock()
		return req, nil
	}
	s.mu.RUnlock()

	settings, err := s.settings.GetPDConsentSettings(ctx)
	if err != nil {
		return PDConsentRequirement{}, err
	}
	sum := sha256.Sum256([]byte(settings.Text))
	req := PDConsentRequirement{
		Enabled:   settings.Required && hasVisibleText(settings.Text),
		Requested: settings.Required,
		Version:   settings.Version,
		VersionAt: settings.VersionAt,
		Text:      settings.Text,
		Hash:      hex.EncodeToString(sum[:]),
	}

	s.mu.Lock()
	s.requirement = req
	s.requirementAt = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return req, nil
}

// AcceptedVersion возвращает максимальную редакцию действующего согласия
// пользователя на обработку ПД; 0 означает "не соглашался".
func (s *PDConsentGateService) AcceptedVersion(ctx context.Context, userID int) (int, error) {
	if v, ok := s.accepted.Load(userID); ok {
		entry := v.(acceptedEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.version, nil
		}
	}
	version, err := s.consents.ActiveVersion(ctx, userID, ConsentTypePDProcessing)
	if err != nil {
		return 0, err
	}
	s.accepted.Store(userID, acceptedEntry{version: version, expiresAt: time.Now().Add(s.ttl)})
	return version, nil
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
	s.accepted.Delete(userID)
}

// InvalidateAll сбрасывает кэш требования и всех принятых редакций. Нужен, когда
// администратор поднял редакцию или поменял настройки согласия.
func (s *PDConsentGateService) InvalidateAll() {
	s.mu.Lock()
	s.requirement = PDConsentRequirement{}
	s.requirementAt = time.Time{}
	s.mu.Unlock()
	s.accepted.Range(func(key, _ any) bool {
		s.accepted.Delete(key)
		return true
	})
}
