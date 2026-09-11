package entityarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/models"
)

// Свидетельство уничтожения персональных данных (#2357).
//
// Пять точек системы уничтожают или обезличивают персональные данные: обезличивание
// заявки, уничтожение заявки целиком, обезличивание человека, обезличивание
// организации и снос организации по проверенному пакету. Каждая пишет о себе в
// audit_log - и этого мало: журнал чистится по сроку, а у уничтоженной заявки уходит
// вместе с ней самой. Перечень, по которому удаления применяют заново после
// восстановления из копии, обязан пережить и то, и другое, поэтому лежит отдельно
// (models.DestructionRecord).
//
// Основание здесь не украшение записи, а её суть: «уничтожено» без ответа на вопрос
// «на каком основании» - не свидетельство. Поэтому пустое основание при -apply
// отклоняется, а не подставляется умолчанием.

// Основания уничтожения. Значения короткие и латинские - они уходят в базу и в
// аргумент командной строки; человеческие названия берутся из DestructionBasisName.
const (
	// BasisRetention - данные ушли по истечении срока хранения. Решение принято
	// один раз, когда владелец задал срок; дальше его исполняет суточный прогон.
	BasisRetention = "retention"
	// BasisOperator - разовое решение оператора: запись завели по ошибке, не на того
	// человека или дважды.
	BasisOperator = "operator"
	// BasisSubjectRequest - требование субъекта персональных данных.
	BasisSubjectRequest = "subject-request"
)

// Что сделано с данными.
const (
	// DestructionAnonymized - персональные поля затёрты, сущность осталась.
	DestructionAnonymized = "anonymized"
	// DestructionPurged - сущность уничтожена целиком.
	DestructionPurged = "purged"
)

// destructionBasisNames - человеческие названия оснований. Из них собирается и
// подсказка команды, и графа акта уничтожения.
var destructionBasisNames = map[string]string{
	BasisRetention:      "истёк срок хранения",
	BasisOperator:       "решение оператора",
	BasisSubjectRequest: "требование субъекта персональных данных",
}

// destructionActionNames - человеческие названия действий для акта.
var destructionActionNames = map[string]string{
	DestructionAnonymized: "обезличено",
	DestructionPurged:     "уничтожено",
}

// DestructionBasisName - основание словами. Неизвестное значение возвращается как
// есть: запись журнала могла быть сделана версией, которая знала больше оснований,
// и подменять её «неизвестно» значило бы испортить свидетельство.
func DestructionBasisName(basis string) string {
	if name, ok := destructionBasisNames[basis]; ok {
		return name
	}
	return basis
}

// DestructionActionName - действие словами, с тем же правилом про неизвестное значение.
func DestructionActionName(action string) string {
	if name, ok := destructionActionNames[action]; ok {
		return name
	}
	return action
}

// DestructionBases - перечень оснований для подсказки команды, в устойчивом порядке.
func DestructionBases() []string {
	out := make([]string, 0, len(destructionBasisNames))
	for basis := range destructionBasisNames {
		out = append(out, basis)
	}
	sort.Strings(out)
	return out
}

// ParseDestructionBasis разбирает основание из аргумента команды.
func ParseDestructionBasis(s string) (string, error) {
	basis := strings.TrimSpace(s)
	if _, ok := destructionBasisNames[basis]; !ok {
		return "", fmt.Errorf("неизвестное основание %q (доступны: %s)",
			s, strings.Join(DestructionBases(), ", "))
	}
	return basis, nil
}

// DestructionOptions - общие параметры операции, уничтожающей персональные данные.
//
// Собраны в структуру, а не дописаны позиционными аргументами: у обезличивания заявки
// их уже было три, и шестой по счёту аргумент читался бы как загадка на месте вызова.
type DestructionOptions struct {
	// Files - корни загрузок и файлового архива. Нужны там, где уничтожение
	// затрагивает диск (заявка); у организации файлы снимает Purge по пакету.
	Files FilePaths
	// ActorID - учётная запись, если операция идёт через интерфейс. У консольной
	// команды nil: доступ к консоли сервера не равен учётной записи в системе.
	ActorID *int
	// Basis - основание уничтожения, одно из Basis*. Обязательно при Apply.
	Basis string
	// Apply - выполнить. Без него операция только считает объём.
	Apply bool
}

// validate отклоняет уничтожение без основания. Только при Apply: показ объёма -
// это ещё не уничтожение, и требовать основание, чтобы посмотреть числа, значило бы
// толкать оператора вписать первое попавшееся.
func (o DestructionOptions) validate() error {
	if !o.Apply {
		return nil
	}
	if _, ok := destructionBasisNames[o.Basis]; !ok {
		return fmt.Errorf("не указано основание уничтожения (доступны: %s): "+
			"запись журнала без основания не является свидетельством",
			strings.Join(DestructionBases(), ", "))
	}
	return nil
}

// actorNote - «по чьему решению», словами. Выводится из основания и учётной записи,
// а не задаётся вызывающей стороной: одна и та же операция иначе описывалась бы в
// акте по-разному в зависимости от того, откуда её запустили.
func (o DestructionOptions) actorNote() string {
	switch {
	case o.ActorID != nil:
		return fmt.Sprintf("учётная запись #%d", *o.ActorID)
	case o.Basis == BasisRetention:
		return "суточный прогон по сроку хранения"
	default:
		return "оператор сервера (консоль)"
	}
}

// destructionRecord собирает заготовку записи журнала из общих параметров. Поля,
// опознающие цель, дописывает вызывающая сторона - у каждой они свои.
func (o DestructionOptions) destructionRecord(entityType string, entityID *int, action string) models.DestructionRecord {
	return models.DestructionRecord{
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Basis:      o.Basis,
		ActorID:    o.ActorID,
		ActorNote:  o.actorNote(),
	}
}

// recordDestruction кладёт свидетельство в журнал уничтожения.
//
// Вызывается ВНУТРИ той же транзакции, что и само уничтожение, тем же приёмом, что и
// запись в audit_log: не выполнилось действие - не появилось и свидетельство, а
// свидетельства без действия не бывает вовсе.
func recordDestruction(ctx context.Context, tx *gorm.DB, rec models.DestructionRecord) error {
	rec.CreatedAt = time.Now().UTC()
	if err := tx.WithContext(ctx).Create(&rec).Error; err != nil {
		return fmt.Errorf("запись в журнал уничтожения: %w", err)
	}
	return nil
}

// documentDigest - отпечаток свёртки документа для журналов.
//
// Класть в журнал саму свёртку нельзя: при выключенном шифровании crypto.ComputeHMAC
// работает passthrough и возвращает исходное значение, то есть журнал стал бы вторым
// хранилищем паспортов. Отпечаток считается всегда - при включённом шифровании это
// свёртка от свёртки, при выключенном от значения; в обоих случаях детерминированно
// и без открытого документа в записи. Обратный ход (найти вернувшегося человека по
// отпечатку) делает subjectTargetFromDigests пересчётом того же sha256 в базе.
func documentDigest(raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ListDestructionRecords возвращает записи журнала уничтожения за период, старые
// раньше. Нулевые границы означают «без ограничения с этой стороны».
func ListDestructionRecords(ctx context.Context, db *gorm.DB, from, to time.Time) ([]models.DestructionRecord, error) {
	q := db.WithContext(ctx).Model(&models.DestructionRecord{}).Order("created_at, id")
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at < ?", to)
	}
	var out []models.DestructionRecord
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("чтение журнала уничтожения: %w", err)
	}
	return out, nil
}
