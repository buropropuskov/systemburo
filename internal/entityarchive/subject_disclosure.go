package entityarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/models"
)

// Запись выдачи сведений в журнал (#2356).
//
// Выдача без записи в журнал запрещена не соглашением, а порядком вызова: команда
// требует получателя и реквизиты запроса ДО того, как соберёт файлы, а запись идёт в
// одной транзакции с ними. Иначе при проверке оператор не докажет, что раскрытие было
// законным, - а именно ради этого журнал и заводится.

// DisclosureRequest - обязательные сведения о выдаче.
type DisclosureRequest struct {
	Recipient  string
	RequestRef string
	Basis      string
	IssuedBy   string
	// IssuedByUserID заполняется только при выдаче через интерфейс.
	IssuedByUserID *int
}

// Validate проверяет, что выдачу есть чем обосновать.
func (r DisclosureRequest) Validate() error {
	if strings.TrimSpace(r.Recipient) == "" {
		return fmt.Errorf("не указан получатель сведений")
	}
	if strings.TrimSpace(r.RequestRef) == "" {
		return fmt.Errorf("не указаны реквизиты запроса, по которому выдаются сведения")
	}
	if strings.TrimSpace(r.IssuedBy) == "" {
		return fmt.Errorf("не указано, кто выдаёт сведения")
	}
	return nil
}

// RecordDisclosure пишет выдачу в журнал. Возвращает созданную запись.
func RecordDisclosure(ctx context.Context, db *gorm.DB, target SubjectTarget, rep SubjectReport, files []string, req DisclosureRequest) (*models.PDDisclosure, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}

	entry := models.PDDisclosure{
		SubjectHMAC:    DisclosureSubjectKey(target),
		SubjectName:    subjectNameFromReport(rep),
		Recipient:      strings.TrimSpace(req.Recipient),
		RequestRef:     strings.TrimSpace(req.RequestRef),
		Basis:          strings.TrimSpace(req.Basis),
		Scope:          scopeSummary(rep),
		Files:          strings.Join(names, ", "),
		IssuedBy:       strings.TrimSpace(req.IssuedBy),
		IssuedByUserID: req.IssuedByUserID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(&entry).Error; err != nil {
		return nil, fmt.Errorf("запись в журнал выдач: %w", err)
	}
	return &entry, nil
}

// subjectNameFromReport берёт ФИО из первой строки раздела сведений: журнал читает
// человек, и по свёртке документа он ничего не поймёт.
func subjectNameFromReport(rep SubjectReport) string {
	for _, s := range rep.Sections {
		if s.Title != "Сведения" {
			continue
		}
		for _, row := range s.Rows {
			if len(row) > 2 && strings.TrimSpace(row[2]) != "" {
				return row[2]
			}
		}
	}
	return ""
}

// scopeSummary - объём выданного разделами. Строкой: разделы со временем меняются, а
// запись журнала обязана остаться читаемой такой, какой её составили.
func scopeSummary(rep SubjectReport) string {
	parts := make([]string, 0, len(rep.Sections))
	for _, s := range rep.Sections {
		parts = append(parts, fmt.Sprintf("%s: %d", s.Title, len(s.Rows)))
	}
	return strings.Join(parts, ", ")
}

// DisclosureSubjectKey - ключ человека в журнале выдач.
//
// Свёртку документа из цели напрямую сюда класть нельзя: при выключенном шифровании
// crypto.ComputeHMAC работает passthrough и возвращает само значение, то есть журнал
// выдач стал бы вторым хранилищем паспортов - ровно тем, чем он быть не должен.
// Поймано тестом: в записи оказался «4510 161803» вместо свёртки.
//
// Поэтому ключ считается всегда: sha256 поверх того, что пришло. При включённом
// шифровании это свёртка от свёртки, при выключенном - от значения; в обоих случаях
// детерминированно и без открытого документа в журнале.
func DisclosureSubjectKey(target SubjectTarget) string {
	raw := target.PassportHMAC
	if raw == "" {
		raw = target.PatentHMAC
	}
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ListDisclosures возвращает журнал выдач, свежие раньше. Пустой ключ - весь журнал,
// непустой - выдачи по одному человеку (см. DisclosureSubjectKey).
func ListDisclosures(ctx context.Context, db *gorm.DB, subjectHMAC string, limit int) ([]models.PDDisclosure, error) {
	q := db.WithContext(ctx).Model(&models.PDDisclosure{}).Order("created_at DESC")
	if strings.TrimSpace(subjectHMAC) != "" {
		q = q.Where("subject_hmac = ?", subjectHMAC)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []models.PDDisclosure
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("чтение журнала выдач: %w", err)
	}
	return out, nil
}
