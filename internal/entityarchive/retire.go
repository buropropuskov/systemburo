package entityarchive

// Срез 4: обратимый офбординг организации консольной командой entity retire/restore.
//
// Отличие от архивации организации через админку (organizationService.Delete/Restore):
// та трогает только саму организацию и блокируется активными пользователями (их некуда
// переназначить из активного списка). retire - офбординг контрагента целиком: гасит
// организацию И её пользователей одним действием, без блокировки.
//
// Обратимость держится на одном инварианте: погашать можно ТОЛЬКО реально активные
// строки (WHERE is_active = true ... RETURNING id). Строка, погашенная раньше по другой
// причине (уволенный сотрудник, ручной архив), в список retire не попадёт - иначе
// последующий restore «оживил» бы её вместе с организацией. restore не пересчитывает,
// что сейчас неактивно, а включает РОВНО те id, что записал последний retire (details в
// audit_log) - это и есть контракт «откат ровно того, что было сделано».
//
// Тот же инвариант требует, чтобы retire, которому гасить уже нечего (повторный вызов
// после успешного первого), НЕ писал новую запись в audit_log: пустая запись стала бы
// «последней retire» для restore и стёрла бы доступ к списку id первого вызова -
// организация осталась бы погашенной без штатного способа вернуться.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// RetireResult - что погасил (или погасил бы, при dry-run) retire.
type RetireResult struct {
	Type          string
	ID            int
	Organizations []int
	Users         []int
	// SkippedSuperAdmins - активные супер-админы организации, которых retire НЕ погасил.
	// Пусто в подавляющем большинстве случаев: непустое значение оператор обязан увидеть,
	// а не узнать постфактум, что офбординг выглядел полным, а владелец системы остался
	// с рабочей учёткой.
	SkippedSuperAdmins []int
}

// Total - сколько строк погашено.
func (r RetireResult) Total() int { return len(r.Organizations) + len(r.Users) }

// RestoreResult - что вернул (или вернул бы) restore.
type RestoreResult struct {
	Type          string
	ID            int
	Organizations []int
	Users         []int
}

// Total - сколько строк восстановлено.
func (r RestoreResult) Total() int { return len(r.Organizations) + len(r.Users) }

// retireDetails - содержимое audit_log.details записи retire. restore читает его вместо
// того, чтобы заново искать неактивные строки: список id - источник истины для отката,
// счётчики дублируют len() ради читаемости записи в истории без разбора JSON-массивов.
type retireDetails struct {
	Organizations      []int `json:"organizations"`
	OrganizationsCount int   `json:"organizations_count"`
	Users              []int `json:"users"`
	UsersCount         int   `json:"users_count"`
	// SkippedSuperAdmins - тот же список, что и в RetireResult, зафиксированный на момент
	// действия: история обязана описывать ровно то, что реально произошло, включая то,
	// что было сознательно пропущено.
	SkippedSuperAdmins []int `json:"skipped_super_admins,omitempty"`
}

// errNothingToRetire - сентинел для случая «под гашение ничего не попало» (организация и
// все её обычные пользователи уже неактивны). Транзакция откатывается без записи в
// audit_log: пустая запись стала бы для restore «последней retire» и заслонила бы список
// id настоящего, предыдущего retire.
var errNothingToRetire = errors.New("нечего гасить")

// errOrgNotFound - сентинел для случая, когда организации с таким id не существует вовсе.
// Отдельно от errNothingToRetire: без разделения оператор с опечаткой в -id (или указавший
// организацию, которая никогда не проходила через retire) получил бы то же «уже погашена,
// новая запись не создана» - пошёл бы искать в audit_log запись retired, не нашёл бы её и
// потерял время на ровном месте, приняв «нет такой организации» за «уже офборднута».
var errOrgNotFound = errors.New("организация не найдена")

// orgExists проверяет существование организации отдельно от её активности: is_active у
// несуществующей строки не спросишь, а «нет строки» и «строка есть, но уже неактивна» -
// разные ситуации для оператора.
func orgExists(ctx context.Context, exec *gorm.DB, id int) (bool, error) {
	var exists bool
	if err := exec.WithContext(ctx).Raw("SELECT EXISTS(SELECT 1 FROM organizations WHERE id = ?)", id).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("проверка организации #%d: %w", id, err)
	}
	return exists, nil
}

// Retire гасит организацию и её пользователей (is_active=false), КРОМЕ супер-админа -
// тот же запрет, что и в user_service.setActive ("иначе админ может вырубить владельца"),
// retire обязан соблюдать его, а не обходить сырым SQL по organization_id.
//
// is_super_admin в схеме объявлен с DEFAULT false, но БЕЗ NOT NULL - "AND is_super_admin"
// и "AND NOT is_super_admin" на строке с NULL дают NULL, и такая строка не попадает НИ в
// гашение, НИ в список пропущенных: пользователь остаётся активным, а команда рапортует
// полный офбординг, хотя реально пропустила строку молча. Go читает NULL в bool-поле как
// false (для остальной системы это обычный пользователь), поэтому обе половины предиката
// обёрнуты в COALESCE(is_super_admin, false) - тогда каждая строка попадает ровно в одну
// из двух групп и «пропала строка» становится невозможной.
//
// apply=false - только подсчёт того, что попало бы под гашение, база не меняется.
func Retire(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, entityType string, id int, actorID *int, apply bool) (RetireResult, error) {
	if entityType != TypeOrganization {
		return RetireResult{}, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}

	if !apply {
		// Тот же отказ, что увидел бы оператор при -apply на несуществующий id: пробный
		// прогон обязан говорить правду, иначе она вскроется только на боевом запуске.
		exists, err := orgExists(ctx, db, id)
		if err != nil {
			return RetireResult{}, err
		}
		if !exists {
			return RetireResult{}, fmt.Errorf("организация #%d не найдена", id)
		}
		orgIDs, err := activeIDs(ctx, db, "organizations", "id = ?", id)
		if err != nil {
			return RetireResult{}, err
		}
		userIDs, err := activeIDs(ctx, db, "users", "organization_id = ? AND NOT COALESCE(is_super_admin, false)", id)
		if err != nil {
			return RetireResult{}, err
		}
		skipped, err := activeIDs(ctx, db, "users", "organization_id = ? AND COALESCE(is_super_admin, false)", id)
		if err != nil {
			return RetireResult{}, err
		}
		return RetireResult{Type: entityType, ID: id, Organizations: orgIDs, Users: userIDs, SkippedSuperAdmins: skipped}, nil
	}

	res := RetireResult{Type: entityType, ID: id}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Проверка существования - внутри ЭТОЙ ЖЕ транзакции, что и само гашение: снаружи
		// (отдельным запросом до Transaction) между проверкой и UPDATE организация могла бы
		// исчезнуть, и сообщение снова оказалось бы не про то.
		exists, err := orgExists(ctx, tx, id)
		if err != nil {
			return err
		}
		if !exists {
			return errOrgNotFound
		}
		orgIDs, err := deactivate(tx, "organizations", "id = ?", id)
		if err != nil {
			return err
		}
		userIDs, err := deactivate(tx, "users", "organization_id = ? AND NOT COALESCE(is_super_admin, false)", id)
		if err != nil {
			return err
		}
		// Зеркалим user_service.setActive: обычная архивация одного пользователя рядом с
		// is_active отзывает его активные refresh-токены ("живая сессия гаснет в пределах
		// TTL access-токена"). Дыры без этого шага нет - login и refresh уже блокируются
		// проверкой is_active (auth_service.go) - но у офборднутой организации не должно
		// оставаться отзываемых сессий, которые пришлось бы гасить отдельной командой.
		if err := revokeActiveTokens(tx, userIDs); err != nil {
			return err
		}
		// Супер-админов не гасили - выбираем их отдельно, чтобы отчитаться оператору.
		// Раз они не тронуты, before/after не отличаются, порядок запроса не важен.
		skipped, err := activeIDs(ctx, tx, "users", "organization_id = ? AND COALESCE(is_super_admin, false)", id)
		if err != nil {
			return err
		}
		res.Organizations, res.Users, res.SkippedSuperAdmins = orgIDs, userIDs, skipped

		if len(orgIDs) == 0 && len(userIDs) == 0 {
			return errNothingToRetire
		}
		// Запись аудита - последним шагом транзакции: если гашение не выполнилось, метка
		// «сделано» не появится вовсе, а не появится раньше самого действия.
		return recorder.Record(ctx, tx, models.AuditEntityOrganization, &id, models.OrganizationActionRetired, actorID,
			retireDetails{Organizations: orgIDs, OrganizationsCount: len(orgIDs), Users: userIDs, UsersCount: len(userIDs),
				SkippedSuperAdmins: skipped})
	})
	switch {
	case errors.Is(err, errOrgNotFound):
		return RetireResult{}, fmt.Errorf("организация #%d не найдена", id)
	case errors.Is(err, errNothingToRetire):
		return RetireResult{}, fmt.Errorf("организация #%d уже погашена, новая запись не создана", id)
	case err != nil:
		return RetireResult{}, fmt.Errorf("retire %s #%d: %w", entityType, id, err)
	}
	return res, nil
}

// Restore возвращает организацию и пользователей, погашенных ПОСЛЕДНИМ retire. Если
// такой записи нет, либо последним действием уже был restore - явный отказ: молча
// включить всё, что сейчас неактивно, замаскировало бы разницу между «нечего
// восстанавливать» и «restore нашёл не ту запись».
func Restore(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, entityType string, id int, actorID *int, apply bool) (RestoreResult, error) {
	if entityType != TypeOrganization {
		return RestoreResult{}, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}

	details, err := lastRetireDetails(ctx, db, id)
	if err != nil {
		return RestoreResult{}, err
	}
	res := RestoreResult{Type: entityType, ID: id, Organizations: details.Organizations, Users: details.Users}
	if !apply {
		return res, nil
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := reactivate(tx, "organizations", details.Organizations); err != nil {
			return err
		}
		if err := reactivate(tx, "users", details.Users); err != nil {
			return err
		}
		return recorder.Record(ctx, tx, models.AuditEntityOrganization, &id, models.OrganizationActionRetireRestored, actorID,
			retireDetails{Organizations: details.Organizations, OrganizationsCount: len(details.Organizations),
				Users: details.Users, UsersCount: len(details.Users)})
	})
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore %s #%d: %w", entityType, id, err)
	}
	return res, nil
}

// activeIDs - id строк таблицы, сейчас активных под условием predicate. exec принимает
// как *gorm.DB верхнего уровня (dry-run вне транзакции), так и tx внутри Transaction -
// оба удовлетворяют одному и тому же интерфейсу вызова.
func activeIDs(ctx context.Context, exec *gorm.DB, table, predicate string, arg any) ([]int, error) {
	var ids []int
	q := fmt.Sprintf("SELECT id FROM %s WHERE %s AND is_active = true", table, predicate)
	if err := exec.WithContext(ctx).Raw(q, arg).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("выборка активных %s: %w", table, err)
	}
	return ids, nil
}

// deactivate гасит активные строки таблицы под условием и возвращает погашенные id.
// Фильтр "is_active = true" в WHERE - ядро обратимости: без него UPDATE переключил бы и
// строку, погашенную раньше по другой причине, и restore вернул бы её к жизни вместе с
// организацией, хотя retire её не касался.
func deactivate(tx *gorm.DB, table, predicate string, arg any) ([]int, error) {
	var ids []int
	q := fmt.Sprintf("UPDATE %s SET is_active = false WHERE %s AND is_active = true RETURNING id", table, predicate)
	if err := tx.Raw(q, arg).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("гашение %s: %w", table, err)
	}
	return ids, nil
}

// revokeActiveTokens отзывает активные refresh-токены погашенных пользователей - шаг,
// которого нет в основном флоу is_active, но который делает user_service.setActive для
// одиночной архивации. restore их не возвращает (тот же контракт, что у обычной
// архивации): возврат из офбординга означает новый логин, а не автоматическое
// восстановление старой сессии.
func revokeActiveTokens(tx *gorm.DB, userIDs []int) error {
	if len(userIDs) == 0 {
		return nil
	}
	if err := tx.Model(&models.RefreshToken{}).
		Where("user_id IN ? AND is_revoked = ?", userIDs, false).
		Updates(map[string]any{"is_revoked": true, "revoked_at": time.Now().UTC()}).Error; err != nil {
		return fmt.Errorf("отзыв refresh-токенов: %w", err)
	}
	return nil
}

// reactivate включает ровно переданные id - список, зафиксированный последним retire, а
// не пересчитанный заново по текущему состоянию таблицы.
func reactivate(tx *gorm.DB, table string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Table(table).Where("id IN ?", ids).Update("is_active", true).Error; err != nil {
		return fmt.Errorf("включение %s: %w", table, err)
	}
	return nil
}

// lastRetireDetails читает последнюю запись retire/restore этой организации. Если
// последней по времени была restore - retire уже откатили, восстанавливать нечего.
func lastRetireDetails(ctx context.Context, db *gorm.DB, orgID int) (retireDetails, error) {
	var last models.AuditLog
	err := db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND action IN ?", models.AuditEntityOrganization, orgID,
			[]string{models.OrganizationActionRetired, models.OrganizationActionRetireRestored}).
		Order("created_at DESC, id DESC").
		First(&last).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return retireDetails{}, fmt.Errorf("для организации #%d нет записи retire - восстанавливать нечего", orgID)
	case err != nil:
		return retireDetails{}, fmt.Errorf("чтение истории retire: %w", err)
	case last.Action != models.OrganizationActionRetired:
		return retireDetails{}, fmt.Errorf("организация #%d уже восстановлена (последнее действие - restore)", orgID)
	}

	var details retireDetails
	if err := json.Unmarshal(last.Details, &details); err != nil {
		return retireDetails{}, fmt.Errorf("разбор деталей retire: %w", err)
	}
	return details, nil
}
