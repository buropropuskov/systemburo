package fakedata

import (
	"context"
	"fmt"
	"sort"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// purgeOrder -- порядок удаления сущностей партии: обратный порядку создания, дети
// раньше родителей. Заявка уносит вложения, машины, сотрудников и имущество каскадом
// (см. constraint:OnDelete:CASCADE в моделях), поэтому отдельными строками они здесь
// не перечислены -- в партии их и не регистрировали.
//
// Название вида берётся из EntityTitle: один список названий на предварительный показ,
// отчёт о наливке и этот отчёт (см. titles.go).
var purgeOrder = []struct {
	entity string
	table  string
}{
	{models.AuditEntityApplication, "applications"},
	{models.AuditEntityApprover, "application_approvers"},
	{models.AuditEntityUniqueCar, "unique_cars"},
	{models.AuditEntityUniqueEmployee, "unique_employees"},
	{models.AuditEntityVehicleBlacklist, "vehicle_blacklists"},
	{models.AuditEntityPersonBlacklist, "person_blacklists"},
	{models.AuditEntityUser, "users"},
	{models.AuditEntitySystemTable, "system_tables"},
	{models.AuditEntityUniqueAttachment, "unique_attachments"},
	{models.AuditEntityLicensePlateFormat, "license_plate_formats"},
	{models.AuditEntityCitizenship, "citizenships"},
	{models.AuditEntityMark, "marks"},
	{models.AuditEntityUnloadPlace, "unload_places"},
	{models.AuditEntityCompany, "companies"},
	{models.AuditEntityOrganization, "organizations"},
}

// PurgeLine -- строка отчёта об удалении: что удалено и что пришлось оставить.
type PurgeLine struct {
	Title string
	// Deleted -- сколько записей удалено (при показе без -apply: сколько удалось бы).
	Deleted int
	// Kept -- сколько оставлено, потому что на них ссылаются данные вне партии.
	Kept int
}

// PurgeResult -- отчёт по всей партии.
type PurgeResult struct {
	Label string
	Lines []PurgeLine
}

// TotalDeleted -- сколько записей удалено всего.
func (r PurgeResult) TotalDeleted() int {
	total := 0
	for _, l := range r.Lines {
		total += l.Deleted
	}
	return total
}

// TotalKept -- сколько записей осталось из-за чужих ссылок.
func (r PurgeResult) TotalKept() int {
	total := 0
	for _, l := range r.Lines {
		total += l.Kept
	}
	return total
}

// PurgeBatch удаляет партию целиком: записи по перечню, их историю и сам перечень.
//
// Без apply ничего не удаляет, а считает, сколько удалилось бы. Считать «сколько
// удалилось бы» точно нельзя не трогая базу: часть записей держат чужие ссылки, и
// узнать это можно только попыткой удаления. Поэтому предварительный показ гоняет тот
// же путь внутри транзакции и откатывает её.
func PurgeBatch(ctx context.Context, db *gorm.DB, label string, apply bool) (PurgeResult, error) {
	batch, err := FindBatch(ctx, db, label)
	if err != nil {
		return PurgeResult{}, err
	}
	result := PurgeResult{Label: batch.Label}

	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return result, fmt.Errorf("удаление партии %q: не удалось начать транзакцию: %w", label, tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	for _, step := range purgeOrder {
		ids, err := batchEntityIDs(ctx, tx, batch.ID, step.entity)
		if err != nil {
			tx.Rollback()
			return result, err
		}
		if len(ids) == 0 {
			continue
		}
		deleted, kept, err := deleteEntities(ctx, tx, step.table, step.entity, ids)
		if err != nil {
			tx.Rollback()
			return result, err
		}
		result.Lines = append(result.Lines, PurgeLine{Title: EntityTitle(step.entity), Deleted: deleted, Kept: kept})
	}

	// Перечень партии снимаем последним: пока он есть, удаление можно повторить с той
	// же точки, если оборвалось.
	if err := tx.Exec(`DELETE FROM fake_batch_items WHERE batch_id = ?`, batch.ID).Error; err != nil {
		tx.Rollback()
		return result, fmt.Errorf("удаление перечня партии %q: %w", label, err)
	}
	if err := tx.Exec(`DELETE FROM fake_batches WHERE id = ?`, batch.ID).Error; err != nil {
		tx.Rollback()
		return result, fmt.Errorf("удаление партии %q: %w", label, err)
	}

	if !apply {
		tx.Rollback()
		return result, nil
	}
	if err := tx.Commit().Error; err != nil {
		return result, fmt.Errorf("удаление партии %q не сохранено: %w", label, err)
	}
	return result, nil
}

func batchEntityIDs(ctx context.Context, tx *gorm.DB, batchID int, entity string) ([]int, error) {
	var ids []int
	err := tx.WithContext(ctx).Raw(
		`SELECT entity_id FROM fake_batch_items WHERE batch_id = ? AND entity = ? ORDER BY entity_id DESC`,
		batchID, entity,
	).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("перечень записей %q партии %d: %w", entity, batchID, err)
	}
	return ids, nil
}

// deleteEntities удаляет записи одного вида и их историю.
//
// Сначала пробует всю пачку разом, и только если пачка не прошла -- идёт по одной.
// Причина в общих записях: справочники, посты и шаблоны вложений заводит та партия, что
// создала их первой, а пользуются ими и последующие. Такую запись держит чужая ссылка,
// и удалять её нельзя -- но и вся пачка из-за неё падать не должна.
func deleteEntities(ctx context.Context, tx *gorm.DB, table, entity string, ids []int) (deleted, kept int, err error) {
	if bulkErr := deleteWithSavepoint(ctx, tx, table, entity, ids); bulkErr == nil {
		return len(ids), 0, nil
	}
	for _, id := range ids {
		if oneErr := deleteWithSavepoint(ctx, tx, table, entity, []int{id}); oneErr != nil {
			kept++
			continue
		}
		deleted++
	}
	return deleted, kept, nil
}

// ownedCleanups -- строки, которые принадлежат самой удаляемой записи и уходят вместе
// с ней. Ключ -- таблица удаляемой записи, значение -- запросы, снимающие принадлежащее
// ей, с единственным параметром «идентификаторы удаляемых».
//
// Список намеренно короткий и ведётся руками. Снимать любые ссылки, найденные по схеме,
// нельзя: на шаблон вложений ссылается вложение чужой партии, и «освобождение» шаблона
// снесло бы её данные. Всё, чего здесь нет, работает наоборот -- ссылка извне оставляет
// запись на месте, и она попадает в отчёт как оставленная.
var ownedCleanups = map[string][]struct{ what, query string }{
	// Членство пользователя в организации или компании существует только ради него:
	// пока строка есть, пользователь не удаляется, а смысла отдельно от него она не имеет.
	"users": {
		{"членство в организациях", `DELETE FROM organization_users WHERE user_id IN (?)`},
		{"членство в компаниях", `DELETE FROM companies_users WHERE user_id IN (?)`},
	},
	// Столбцы, окна и фотографии поста -- части самого поста. Связь объявлена у
	// SystemTable перечнями (Fields/FactFields/TimeSlots/WarningWindows/Photos) без
	// правила удаления, поэтому в базе она без каскада: пост не удалялся, пока живы его
	// собственные столбцы. В отчёте это выглядело как «оставлено: на них ссылаются
	// данные вне партии» -- неправда, ссылались его же строки.
	"system_tables": {
		{"столбцы поста", `DELETE FROM table_fields WHERE table_id IN (?)`},
		{"столбцы фактического прохода", `DELETE FROM table_field_facts WHERE table_id IN (?)`},
		{"временные окна поста", `DELETE FROM system_table_time_slots WHERE table_id IN (?)`},
		{"предупреждения поста", `DELETE FROM system_table_warning_windows WHERE table_id IN (?)`},
		{"фотографии поста", `DELETE FROM system_table_photos WHERE table_id IN (?)`},
		// Привязки видимости к посту -- зеркало строк ниже, но со стороны поста, а не
		// заявки. Пост заводит первая партия, а пользуются им и следующие: удаление
		// первой уносило пост, а привязки живых машин и сотрудников второй оставались
		// смотреть в пустоту -- внешних ключей у них нет, держать пост нечем.
		{"привязки машин к посту", `DELETE FROM car_target_tables WHERE table_id IN (?)`},
		{"привязки сотрудников к посту", `DELETE FROM employee_target_tables WHERE table_id IN (?)`},
		// Права поста (table.<слаг>.<глагол>) заводит SystemTableService.Create вместе с
		// самим постом -- без них пост нельзя выдать ни одной роли. Ссылок на permissions
		// в базе нет ни одной: выдачи и переопределения держат ключ строкой, поэтому
		// удаление поста не задевало ни права, ни выдачи. На стенде это копилось по
		// десять прав на пост с каждой наливкой, и разобрать их через интерфейс уже
		// нельзя -- таблицы, которой они принадлежат, не существует.
		//
		// Порядок обязателен: ссылающиеся строки ищут ключ в permissions, поэтому сами
		// права снимаются последними.
		{"выдачи прав поста ролям", `DELETE FROM role_permission_grants WHERE permission_key IN (
			SELECT key FROM permissions WHERE category = 'table' AND entity_id IN (?))`},
		{"выдачи прав поста группам", `DELETE FROM permission_group_grants WHERE permission_key IN (
			SELECT key FROM permissions WHERE category = 'table' AND entity_id IN (?))`},
		{"переопределения прав поста", `DELETE FROM user_permission_overrides WHERE permission_key IN (
			SELECT key FROM permissions WHERE category = 'table' AND entity_id IN (?))`},
		{"личные права поста", `DELETE FROM user_permissions WHERE permission_key IN (
			SELECT key FROM permissions WHERE category = 'table' AND entity_id IN (?))`},
		{"права поста", `DELETE FROM permissions WHERE category = 'table' AND entity_id IN (?)`},
	},
	// Машины и сотрудники заявки уходят с ней каскадом, а их привязки к постам -- нет:
	// у car_target_tables/employee_target_tables внешних ключей нет вовсе. После
	// удаления партии строки оставались висеть на несуществующих машинах и держали
	// посты той же партии, то есть мусор копился с каждой наливкой.
	"applications": {
		{"привязки машин к постам", `DELETE FROM car_target_tables WHERE car_id IN (
			SELECT c.id FROM cars c JOIN attachments a ON a.id = c.attachment_id
			WHERE a.application_id IN (?))`},
		{"привязки сотрудников к постам", `DELETE FROM employee_target_tables WHERE employee_id IN (
			SELECT e.id FROM employees e JOIN attachments a ON a.id = e.attachment_id
			WHERE a.application_id IN (?))`},
		// Пометки о возможном обходе чёрного списка и решения "всё равно пропустить" по
		// ним -- снимок момента подачи ЭТОЙ заявки. Внешних ключей у них нет намеренно:
		// пометка переживает правку и удаление самого элемента и записи чёрного списка.
		// Но читают их только по application_id, поэтому вместе с заявкой они перестают
		// быть чем-либо, кроме мусора. Решение снимается раньше пометки: оно ссылается
		// на неё.
		{"решения по пометкам чёрного списка", `DELETE FROM application_blacklist_overrides WHERE application_id IN (?)`},
		{"пометки чёрного списка", `DELETE FROM application_blacklist_flags WHERE application_id IN (?)`},
	},
}

// clearOwnedLinks снимает принадлежащее удаляемым записям перед их удалением.
func clearOwnedLinks(ctx context.Context, tx *gorm.DB, table string, ids []int) error {
	for _, cleanup := range ownedCleanups[table] {
		if err := tx.WithContext(ctx).Exec(cleanup.query, ids).Error; err != nil {
			return fmt.Errorf("снятие связки (%s): %w", cleanup.what, err)
		}
	}
	return nil
}

// deleteWithSavepoint выполняет удаление внутри точки сохранения: неудача снимает
// только её, а не всю транзакцию удаления партии.
func deleteWithSavepoint(ctx context.Context, tx *gorm.DB, table, entity string, ids []int) error {
	const savepoint = "fake_purge"
	if err := tx.WithContext(ctx).Exec(`SAVEPOINT ` + savepoint).Error; err != nil {
		return err
	}
	// История живёт без внешнего ключа на сущность (нарочно -- запись переживает
	// удаление того, о ком она), поэтому её приходится снимать явно.
	if err := tx.WithContext(ctx).Exec(
		`DELETE FROM audit_log WHERE entity_type = ? AND entity_id IN (?)`, entity, ids,
	).Error; err != nil {
		tx.WithContext(ctx).Exec(`ROLLBACK TO SAVEPOINT ` + savepoint)
		return err
	}
	if err := clearOwnedLinks(ctx, tx, table, ids); err != nil {
		tx.WithContext(ctx).Exec(`ROLLBACK TO SAVEPOINT ` + savepoint)
		return err
	}
	if err := tx.WithContext(ctx).Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE id IN (?)`, table), ids,
	).Error; err != nil {
		tx.WithContext(ctx).Exec(`ROLLBACK TO SAVEPOINT ` + savepoint)
		return err
	}
	return tx.WithContext(ctx).Exec(`RELEASE SAVEPOINT ` + savepoint).Error
}

// SortedPurgeLines -- строки отчёта в порядке удаления, пустые пропущены.
func SortedPurgeLines(lines []PurgeLine) []PurgeLine {
	out := make([]PurgeLine, 0, len(lines))
	for _, l := range lines {
		if l.Deleted == 0 && l.Kept == 0 {
			continue
		}
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Deleted > out[j].Deleted })
	return out
}
