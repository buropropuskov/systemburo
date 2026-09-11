package entityarchive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/export"
	"systemburo/internal/models"
)

// Акт уничтожения персональных данных за период (#2357).
//
// Журнал отвечает машине на вопрос «что снимать заново после восстановления». Акт
// отвечает человеку - проверяющему - на вопрос «чем вы подтверждаете, что данные
// уничтожены». Это один и тот же перечень в двух видах, и второй вид приходится
// собирать отдельно: печатную форму подписывают и прикладывают к ответу, а таблицу
// базы к ответу не приложишь.
//
// Персональных данных в акте нет по той же причине, что и в журнале: документ,
// подтверждающий уничтожение сведений о человеке, не должен эти сведения содержать.
// Опознание идёт номером заявки, идентификатором и основанием.

// actDateLayout - вид дат во всей системе: 01.01.0000. Требование владельца, и акт
// не исключение - он попадает к тем же людям, что и остальные выгрузки.
const actDateLayout = "02.01.2006"

// DestructionAct - собранный акт: что вошло и за какой период.
type DestructionAct struct {
	From time.Time
	To   time.Time
	// Records - записи журнала, вошедшие в акт.
	Records []models.DestructionRecord
	// Sections - готовые таблицы выгрузки: перечень и свод по основаниям.
	Sections []export.Table
	// Rows и Files - сколько строк базы и файлов уничтожено за период.
	Rows  int
	Files int
}

// ParseActDate разбирает границу периода в виде 01.01.2026.
func ParseActDate(s string) (time.Time, error) {
	v, err := time.Parse(actDateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("дата %q не разобрана (ожидается вид 01.01.2026)", s)
	}
	return v.UTC(), nil
}

// BuildDestructionAct собирает акт за период. Обе границы включительные: оператор,
// просящий акт «с 1 января по 31 марта», имеет в виду весь день 31 марта.
func BuildDestructionAct(ctx context.Context, db *gorm.DB, from, to time.Time) (DestructionAct, error) {
	if from.After(to) {
		return DestructionAct{}, fmt.Errorf("начало периода (%s) позже его конца (%s)",
			from.Format(actDateLayout), to.Format(actDateLayout))
	}

	records, err := ListDestructionRecords(ctx, db, from, to.AddDate(0, 0, 1))
	if err != nil {
		return DestructionAct{}, err
	}

	act := DestructionAct{From: from, To: to, Records: records}
	for _, r := range records {
		act.Rows += r.Rows
		act.Files += r.Files
	}
	act.Sections = []export.Table{actListTable(act), actSummaryTable(act)}
	return act, nil
}

// actSubtitle - подпись периода, общая для обеих таблиц акта.
func (a DestructionAct) actSubtitle() string {
	return fmt.Sprintf("за период с %s по %s, записей %d",
		a.From.Format(actDateLayout), a.To.Format(actDateLayout), len(a.Records))
}

// actListTable - основная таблица акта: строка на каждое уничтожение.
func actListTable(a DestructionAct) export.Table {
	t := export.Table{
		Title:    "Уничтожение",
		Subtitle: a.actSubtitle(),
		Headers: []string{
			"Дата", "Что уничтожено", "Действие", "Основание",
			"По чьему решению", "Строк", "Файлов", "Повторов",
		},
	}
	for _, r := range a.Records {
		t.Rows = append(t.Rows, []string{
			r.CreatedAt.Format(actDateLayout),
			DestructionTargetName(r),
			DestructionActionName(r.Action),
			DestructionBasisName(r.Basis),
			r.ActorNote,
			strconv.Itoa(r.Rows),
			strconv.Itoa(r.Files),
			strconv.Itoa(r.Replays),
		})
	}
	return t
}

// actSummaryTable - свод по основаниям. Нужен не ради красоты: проверяющий спрашивает
// «сколько и на каком основании», и считать это по перечню из сотен строк он не станет.
func actSummaryTable(a DestructionAct) export.Table {
	t := export.Table{
		Title:    "Свод по основаниям",
		Subtitle: a.actSubtitle(),
		Headers:  []string{"Основание", "Записей", "Строк", "Файлов"},
	}
	// Порядок фиксированный (DestructionBases), а не по мере встречаемости: акт за
	// разные периоды должен читаться одинаково.
	seen := make(map[string]bool, len(a.Records))
	order := DestructionBases()
	for _, r := range a.Records {
		seen[r.Basis] = true
	}
	for _, r := range a.Records {
		// Основание из записи прежней версии, которого в перечне уже нет, тоже обязано
		// попасть в свод: иначе итог свода разойдётся с перечнем и акт станет ложью.
		if _, known := destructionBasisNames[r.Basis]; !known {
			order = appendUnique(order, r.Basis)
		}
	}

	for _, basis := range order {
		if !seen[basis] {
			continue
		}
		count, rows, files := 0, 0, 0
		for _, r := range a.Records {
			if r.Basis != basis {
				continue
			}
			count++
			rows += r.Rows
			files += r.Files
		}
		t.Rows = append(t.Rows, []string{
			DestructionBasisName(basis),
			strconv.Itoa(count), strconv.Itoa(rows), strconv.Itoa(files),
		})
	}
	t.Rows = append(t.Rows, []string{
		"Итого", strconv.Itoa(len(a.Records)), strconv.Itoa(a.Rows), strconv.Itoa(a.Files),
	})
	return t
}

func appendUnique(list []string, v string) []string {
	for _, item := range list {
		if item == v {
			return list
		}
	}
	return append(list, v)
}

// DestructionTargetName - что уничтожено, словами. Номер заявки предпочтительнее
// идентификатора: и акт, и перечень читает человек, и «№ 20260101/500» говорит ему
// больше, чем 417. Общая на оба вывода: разойдись они, человек в акте назывался бы
// «Сведения о человеке», а в перечне - «unique_employee».
func DestructionTargetName(r models.DestructionRecord) string {
	switch {
	case r.ApplicationNumber != "":
		return "Заявка " + r.ApplicationNumber
	case r.EntityType == models.AuditEntityApplication && r.EntityID != nil:
		return fmt.Sprintf("Заявка #%d (без номера)", *r.EntityID)
	case r.EntityType == models.AuditEntityOrganization && r.EntityID != nil:
		return fmt.Sprintf("Организация #%d", *r.EntityID)
	case r.EntityType == models.AuditEntityUniqueEmployee:
		// Ни имени, ни документа: акт подтверждает уничтожение сведений о человеке и
		// сам этих сведений нести не должен. Отпечаток документа в журнале есть, но
		// в бумагу он не идёт - проверяющему он ничего не даёт.
		return "Сведения о человеке"
	case r.EntityID != nil:
		return fmt.Sprintf("%s #%d", r.EntityType, *r.EntityID)
	default:
		return r.EntityType
	}
}

// WriteDestructionAct кладёт акт в каталог root двумя файлами: .xlsx для работы и .pdf
// для приложения к официальному ответу. Возвращает пути записанных файлов.
//
// Каталог тот же, что у пакетов и справок (ENTITY_EXPORT_PATH): место хранения таких
// документов выбирает владелец системы, а не программа.
func WriteDestructionAct(root string, act DestructionAct) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("не задан каталог выгрузки")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("каталог выгрузки %s: %w", root, err)
	}

	xlsx, err := export.ToXLSXMulti(act.Sections)
	if err != nil {
		return nil, fmt.Errorf("сборка xlsx: %w", err)
	}
	pdf, err := export.ToPDFMulti(act.Sections)
	if err != nil {
		return nil, fmt.Errorf("сборка pdf: %w", err)
	}

	// Имя несёт период, а не момент сборки: акт за один и тот же период, собранный
	// дважды, - это один документ, и разъезжаться по именам он не должен.
	base := filepath.Join(root, fmt.Sprintf("destruction-act-%s-%s",
		act.From.Format("20060102"), act.To.Format("20060102")))

	written := make([]string, 0, 2)
	for _, f := range []struct {
		path string
		data []byte
	}{{base + ".xlsx", xlsx}, {base + ".pdf", pdf}} {
		if err := os.WriteFile(f.path, f.data, 0o600); err != nil {
			return nil, fmt.Errorf("запись %s: %w", f.path, err)
		}
		written = append(written, f.path)
	}
	return written, nil
}
