package entityarchive

import "fmt"

// Пакет собирает граф данных, связанных с одной сущностью (v1 - организация): какие
// таблицы и сколько строк принадлежат цели. Только чтение: ничего не удаляет и не пишет
// в журналы.
//
// Карта связей ведётся здесь руками, а не выводится из information_schema. Автообход по
// внешним ключам не отличает владение (заявки организации уходят с ней) от ссылки на
// общий справочник (гражданства, марки - остаются): именно это различие и определяет,
// что уносить, а что оставить, и описать его может только человек, читавший модели.
//
// Граф глубокий: организация -> заявки -> вложения -> машины/сотрудники/имущество.
// Тонкости, пойманные проверкой схемы (information_schema), а не чтением одних имён
// полей - у части моделей поле OrganizationID есть только в DTO, не в таблице:
//   - организации напрямую принадлежат ровно 8 таблиц (столбец organization_id):
//     applications, attachments, users, реестры unique_cars/unique_employees и три
//     join-таблицы (organization_users/tables/unload_places);
//   - у cars/employees столбца org НЕТ: они цепляются к организации только через
//     attachment->application. «Ручное добавление» (#1049) кладёт org на само вложение
//     (attachments.organization_id), а не на машину;
//   - system_tables (посты) и unload_places организации НЕ принадлежат - они общие;
//     с организацией уходят лишь их join-строки (organization_tables/unload_places).

// TypeOrganization - единственный поддерживаемый в v1 тип цели.
const TypeOrganization = "organization"

// Node - одна таблица графа: физическое имя и предикат, отбирающий её строки,
// принадлежащие цели. Единственный именованный параметр @org связывается с
// идентификатором организации; в подзапросах он повторяется, но именованный параметр
// не требует считать вхождения (в отличие от позиционного ?).
type Node struct {
	Table string
	Where string
}

// Подзапросы-звенья графа организации. Строятся композицией, чтобы путь до глубокой
// таблицы читался целиком и не расходился между узлами, которые на него опираются.
const (
	orgApps = "SELECT id FROM applications WHERE organization_id = @org"
	orgAtts = "SELECT id FROM attachments WHERE organization_id = @org OR application_id IN (" + orgApps + ")"
	orgCars = "SELECT id FROM cars WHERE attachment_id IN (" + orgAtts + ")"
	orgEmps = "SELECT id FROM employees WHERE attachment_id IN (" + orgAtts + ")"
)

// organizationNodes - граф организации в порядке «дети раньше родителей» (тот же
// порядок, что у удаления в testutil.tables). Для показа порядок косметический, но он
// же понадобится сносу и экспорту, поэтому выдержан сразу.
func organizationNodes() []Node {
	return []Node{
		// Листья машины и сотрудника.
		{"car_unload_places", "car_id IN (" + orgCars + ")"},
		{"car_target_tables", "car_id IN (" + orgCars + ")"},
		{"employee_files", "employee_id IN (" + orgEmps + ")"},
		{"employee_target_tables", "employee_id IN (" + orgEmps + ")"},
		// Элементы вложений (машина/сотрудник/имущество) и их связки - по attachment.
		{"application_employees", "attachment_id IN (" + orgAtts + ")"},
		{"application_items", "attachment_id IN (" + orgAtts + ")"},
		{"items", "attachment_id IN (" + orgAtts + ")"},
		{"cars", "attachment_id IN (" + orgAtts + ")"},
		{"employees", "attachment_id IN (" + orgAtts + ")"},
		{"attachment_unload_places", "attachment_id IN (" + orgAtts + ")"},
		{"application_question_attachments", "attachment_id IN (" + orgAtts + ")"},
		{"attachments", "organization_id = @org OR application_id IN (" + orgApps + ")"},
		// Реестры уникальных машин и сотрудников - per-org (у моделей есть organization_id),
		// поэтому уходят с организацией. Идут после cars/employees: те ссылаются на реестр.
		{"unique_cars", "organization_id = @org"},
		{"unique_employees", "organization_id = @org"},
		// Поддерево заявки - по application_id.
		{"application_answers", "application_id IN (" + orgApps + ")"},
		{"application_question_views", "application_id IN (" + orgApps + ")"},
		{"application_questions", "application_id IN (" + orgApps + ")"},
		{"forward_attachments", "application_id IN (" + orgApps + ")"},
		{"application_blacklist_overrides", "application_id IN (" + orgApps + ")"},
		{"application_blacklist_flags", "application_id IN (" + orgApps + ")"},
		{"application_supplements", "application_id IN (" + orgApps + ")"},
		{"application_files", "application_id IN (" + orgApps + ")"},
		{"application_status_views", "application_id IN (" + orgApps + ")"},
		{"application_reads", "application_id IN (" + orgApps + ")"},
		{"application_viewers", "application_id IN (" + orgApps + ")"},
		{"application_responsible_users", "application_id IN (" + orgApps + ")"},
		{"application_status_history", "application_id IN (" + orgApps + ")"},
		{"applications", "organization_id = @org"},
		// Прямые связки и корень. unload_places/system_tables общие - уходят только
		// их join-строки, сами таблицы не собираются.
		{"organization_unload_places", "organization_id = @org"},
		{"organization_tables", "organization_id = @org"},
		{"organization_users", "organization_id = @org"},
		{"users", "organization_id = @org"},
		{"organizations", "id = @org"},
	}
}

// directOrgRoots - таблицы, чей столбец organization_id делает их прямым корнем графа
// организации. Замок-тест сверяет набор с моделями: у модели есть поле OrganizationID -
// её таблица обязана быть здесь. Иначе новая org-таблица тихо выпадет из выгрузки, а
// заметят это только по неполному реимпорту.
var directOrgRoots = map[string]bool{
	"applications":               true,
	"attachments":                true,
	"unique_cars":                true,
	"unique_employees":           true,
	"users":                      true,
	"organization_users":         true,
	"organization_tables":        true,
	"organization_unload_places": true,
}

// allowedNodeTables - множество имён таблиц, легитимных для графа сущности entityType.
//
// Единственная сверка манифеста пакета со схемой базы (verifySchema в verify.go)
// доказывает лишь «такая таблица и такие колонки где-то в этой базе есть» - подменённый
// манифест может назвать ЛЮБУЮ реальную таблицу схемы (настройки, роли, права) и пройти её
// чисто. Этот список - независимый от схемы якорь: то, что реально входит в граф entityType,
// а не то, что просто существует в базе. Verify и Import обязаны сверяться с ним ОБА
// (см. комментарий пакета в import.go) - каждый как со своим гейтом, не полагаясь на то,
// что второй вызов уже проверил.
func allowedNodeTables(entityType string) (map[string]bool, error) {
	if entityType != TypeOrganization {
		return nil, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}
	set := make(map[string]bool)
	for _, n := range organizationNodes() {
		set[n.Table] = true
	}
	return set, nil
}

// CheckSupportedType сообщает, поддерживается ли entityType, не строя карту таблиц графа -
// команде entity нужен только сам факт. Тонкая обёртка над allowedNodeTables: список
// поддерживаемых типов задаётся там и только там, чтобы не разъезжались две копии одной
// проверки. Вызывается из cmd/server до подключения к базе и настройки шифрования - опечатка
// в -type видна сразу, а не после того, как открылось соединение.
func CheckSupportedType(entityType string) error {
	_, err := allowedNodeTables(entityType)
	return err
}
