package services

import (
	"sort"
	"strings"
)

// Общие типы и хелперы для аудита групповых/одиночных изменений справочников
// (организации, компании). Пишутся в единый audit_log (#870) с деталями
// «было -> стало». Используются Update*-методами organization_service (сейчас);
// company_service подключит их в зеркальном срезе.

// auditUserNameSQL - выражение ФИО актора/ответственного (фолбэк на username),
// то же, что в GetHistory. Ссылается на алиас u таблицы users.
const auditUserNameSQL = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`

// idName - пара id/name для резолва человекочитаемых имён (места, таблицы).
type idName struct {
	ID   int    `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

func idNameMap(rows []idName) map[int]string {
	m := make(map[int]string, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Name
	}
	return m
}

// auditNameDiff - деталь audit для изменения набора именованных привязок
// (места разгрузки, таблицы): что добавили и что убрали.
type auditNameDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

func (d auditNameDiff) empty() bool { return len(d.Added) == 0 && len(d.Removed) == 0 }

// diffIDNames вычисляет added/removed имена по старому набору (id->name) и новому
// набору id (имена из newNames). Порядок детерминирован (по возрастанию id) -
// стабильные тесты и предсказуемый экспорт истории.
func diffIDNames(old map[int]string, newIDs []int, newNames map[int]string) auditNameDiff {
	newSet := make(map[int]bool, len(newIDs))
	orderedNew := make([]int, 0, len(newIDs))
	for _, id := range newIDs {
		if !newSet[id] {
			newSet[id] = true
			orderedNew = append(orderedNew, id)
		}
	}
	sort.Ints(orderedNew)

	var d auditNameDiff
	for _, id := range orderedNew {
		if _, ok := old[id]; !ok {
			d.Added = append(d.Added, newNames[id])
		}
	}
	oldIDs := make([]int, 0, len(old))
	for id := range old {
		oldIDs = append(oldIDs, id)
	}
	sort.Ints(oldIDs)
	for _, id := range oldIDs {
		if !newSet[id] {
			d.Removed = append(d.Removed, old[id])
		}
	}
	return d
}

// --- Ответственные (users) ---

// auditUserState - снимок одного ответственного (для diff «кто был -> кто стал»).
type auditUserState struct {
	Username         string `gorm:"column:username"`
	Name             string `gorm:"column:name"`
	RequiredApproval bool   `gorm:"column:required_approval"`
	IsPrimary        bool   `gorm:"column:is_primary"`
}

// auditPrimaryChange - смена главного ответственного (from/to nil = не было / снят).
type auditPrimaryChange struct {
	From *auditUserRef `json:"from,omitempty"`
	To   *auditUserRef `json:"to,omitempty"`
}

type auditUserAdded struct {
	Username         string `json:"username"`
	Name             string `json:"name"`
	RequiredApproval bool   `json:"required_approval"`
}

type auditUserRef struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

type auditApprovalChange struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	From     bool   `json:"from"`
	To       bool   `json:"to"`
}

// auditUsersDiff - деталь audit для смены ответственных: добавленные, убранные и
// те, у кого сменился флаг обязательного согласования.
type auditUsersDiff struct {
	Added           []auditUserAdded      `json:"added,omitempty"`
	Removed         []auditUserRef        `json:"removed,omitempty"`
	ApprovalChanged []auditApprovalChange `json:"approval_changed,omitempty"`
	PrimaryChanged  *auditPrimaryChange   `json:"primary_changed,omitempty"`
}

func (d auditUsersDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.ApprovalChanged) == 0 && d.PrimaryChanged == nil
}

// primaryRef возвращает ссылку на главного ответственного в наборе (или nil).
func primaryRef(users []auditUserState) *auditUserRef {
	for _, u := range users {
		if u.IsPrimary {
			return &auditUserRef{Username: u.Username, Name: u.Name}
		}
	}
	return nil
}

func samePrimary(a, b *auditUserRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Username == b.Username
}

// diffUsers сравнивает старый и применённый наборы ответственных. Сортировка по
// username - для детерминизма. applied - только реально применённые (несуществующие
// username в UpdateOrganizationUsers пропускаются, в набор не попадают).
func diffUsers(old, applied []auditUserState) auditUsersDiff {
	oldByUser := make(map[string]auditUserState, len(old))
	for _, u := range old {
		oldByUser[u.Username] = u
	}
	appliedByUser := make(map[string]auditUserState, len(applied))
	for _, u := range applied {
		appliedByUser[u.Username] = u
	}

	appliedSorted := append([]auditUserState(nil), applied...)
	sort.Slice(appliedSorted, func(i, j int) bool { return appliedSorted[i].Username < appliedSorted[j].Username })
	oldSorted := append([]auditUserState(nil), old...)
	sort.Slice(oldSorted, func(i, j int) bool { return oldSorted[i].Username < oldSorted[j].Username })

	var d auditUsersDiff
	for _, u := range appliedSorted {
		if o, ok := oldByUser[u.Username]; ok {
			if o.RequiredApproval != u.RequiredApproval {
				d.ApprovalChanged = append(d.ApprovalChanged, auditApprovalChange{
					Username: u.Username, Name: u.Name, From: o.RequiredApproval, To: u.RequiredApproval,
				})
			}
			continue
		}
		d.Added = append(d.Added, auditUserAdded{Username: u.Username, Name: u.Name, RequiredApproval: u.RequiredApproval})
	}
	for _, o := range oldSorted {
		if _, ok := appliedByUser[o.Username]; !ok {
			d.Removed = append(d.Removed, auditUserRef{Username: o.Username, Name: o.Name})
		}
	}
	// Смена главного ответственного (по одному primary на организацию/компанию).
	if op, np := primaryRef(old), primaryRef(applied); !samePrimary(op, np) {
		d.PrimaryChanged = &auditPrimaryChange{From: op, To: np}
	}
	return d
}

// fullName собирает ФИО из указателей на last/first (фолбэк на username), зеркалит
// auditUserNameSQL для случаев, когда ответственный резолвится из модели, а не SQL.
func fullName(last, first *string, username string) string {
	parts := make([]string, 0, 2)
	if last != nil && strings.TrimSpace(*last) != "" {
		parts = append(parts, strings.TrimSpace(*last))
	}
	if first != nil && strings.TrimSpace(*first) != "" {
		parts = append(parts, strings.TrimSpace(*first))
	}
	if n := strings.Join(parts, " "); n != "" {
		return n
	}
	return username
}
