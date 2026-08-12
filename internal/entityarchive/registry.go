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
//
// Рукописная карта защищает export/verify/покрытие (что БУДЕТ выгружено и сверено), но
// сама по себе не защищает от DELETE: Postgres исполняет ON DELETE CASCADE независимо от
// того, что думает об этом Go-код. Ревью среза purge (12.08) прогнало ПОЛНЫЙ обход
// information_schema от каждого узла графа и нашло 14 таблиц с каскадом от users.id,
// application_questions.id и application_supplements.id, которых в карте не было -
// DELETE FROM users сносил их молча, мимо Collect/экспорта/сверки покрытия/audit_log.
// Все 14 заведены явными узлами ниже, каждая с однострочной причиной. Гарантия держится
// тестом TestOrganizationGraph_NoUnaccountedCascades (internal/handlers/entity_graph_test.go) -
// он живьём обходит information_schema от каждого узла графа и падает на первом каскаде
// без узла и без записи в cascadeExemptions.

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
	// orgUsers - пользователи организации. Опора для личных данных пользователя (найдены
	// каскадом от users.id при аудите 12.08 - см. комментарий пакета выше): их предикат
	// идёт по user_id, а не по organization_id, потому что своей колонки org у них нет.
	orgUsers = "SELECT id FROM users WHERE organization_id = @org"
	// orgQuestions/orgSupplements - опора для находок того же аудита, повисших на
	// поддереве заявки, а не на пользователе: прочтения вопросов и голоса по раунду
	// дополнения принадлежат вопросу/раунду, а не тому, кто читал или голосовал.
	orgQuestions   = "SELECT id FROM application_questions WHERE application_id IN (" + orgApps + ")"
	orgSupplements = "SELECT id FROM application_supplements WHERE application_id IN (" + orgApps + ")"
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
		// application_question_reads - каскад от application_questions.id (найден
		// аудитом 12.08): кто прочитал вопрос, значения не имеет, прочтение
		// принадлежит вопросу - тот же принцип, что у application_answers строкой выше.
		// Обязан идти РАНЬШЕ application_questions (её ребёнок).
		{"application_question_reads", "question_id IN (" + orgQuestions + ")"},
		{"application_question_views", "application_id IN (" + orgApps + ")"},
		{"application_questions", "application_id IN (" + orgApps + ")"},
		{"forward_attachments", "application_id IN (" + orgApps + ")"},
		{"application_blacklist_overrides", "application_id IN (" + orgApps + ")"},
		{"application_blacklist_flags", "application_id IN (" + orgApps + ")"},
		// application_supplement_approvals - каскад от application_supplements.id
		// (тот же аудит): голос согласующего принадлежит раунду дополнения, обязан
		// идти РАНЬШЕ application_supplements.
		{"application_supplement_approvals", "supplement_id IN (" + orgSupplements + ")"},
		{"application_supplements", "application_id IN (" + orgApps + ")"},
		{"application_files", "application_id IN (" + orgApps + ")"},
		{"application_status_views", "application_id IN (" + orgApps + ")"},
		{"application_reads", "application_id IN (" + orgApps + ")"},
		{"application_viewers", "application_id IN (" + orgApps + ")"},
		{"application_responsible_users", "application_id IN (" + orgApps + ")"},
		{"application_status_history", "application_id IN (" + orgApps + ")"},
		{"applications", "organization_id = @org"},
		// Личные данные пользователей организации - остальные 12 находок того же
		// аудита 12.08, все каскад от users.id. Обязаны идти РАНЬШЕ users (её дети).
		// Однострочная причина у каждой - что это и почему уходит с пользователем:
		{"notifications", "user_id IN (" + orgUsers + ")"},                 // личная лента уведомлений
		{"user_onboarding_progress", "user_id IN (" + orgUsers + ")"},      // прогресс обучающих туров
		{"user_notification_preferences", "user_id IN (" + orgUsers + ")"}, // персональные отклонения от каталога уведомлений
		// user_groups/user_permission_overrides - членство и переопределения ПРАВ
		// самого пользователя (не общих ролей/групп - те справочник, остаются).
		{"user_groups", "user_id IN (" + orgUsers + ")"},
		{"user_permission_overrides", "user_id IN (" + orgUsers + ")"},
		// report_templates - шаблоны отчётов, СОЗДАННЫЕ пользователем (owner_user_id).
		// Личные уходят вместе с автором штатным каскадом по предикату ниже. Общие
		// (is_shared) при -apply purge отвязывает от владельца ДО того, как дойти до
		// этого узла (detachSharedReportTemplates в purge.go) - тот же предикат
		// перестаёт их видеть, и они переживают снос вместо того, чтобы пропасть у
		// чужих пользователей, которые ими пользуются. На Collect и экспорт (они не
		// удаляют и не отвязывают ничего) это не влияет - до -apply отвязки ещё не
		// было, и оба видят общие шаблоны как есть, тем же узлом и тем же предикатом.
		{"report_templates", "owner_user_id IN (" + orgUsers + ")"},
		{"security_user_tables", "user_id IN (" + orgUsers + ")"},        // привязка охранника к постам (сами посты - справочник)
		{"security_user_unload_places", "user_id IN (" + orgUsers + ")"}, // привязка охранника к местам разгрузки (справочник)
		// application_approvers - пересмотр решения среза 1 ("глобальный реестр
		// принимающих, в граф не входит"): решение было верным для export/verify
		// (принимающий физически не привязан к организации ни одной колонкой), но FK
		// на users.id каскадный - когда purge удаляет пользователя, Postgres сносит
		// его запись принимающего независимо от того, есть ли для неё узел. Раз снос
		// неизбежен, он обязан быть учтён (счётчик, экспорт, audit_log), а не невидим.
		{"application_approvers", "user_id IN (" + orgUsers + ")"},
		{"feedback", "user_id IN (" + orgUsers + ")"}, // обращения пользователя в поддержку
		// pd_consents - согласия на обработку ПДн. Отдельный ретеншн (доказательство
		// согласия) FK не остановит: он каскадный и физически снесёт строку при
		// удалении пользователя ЛЮБЫМ путём, не только purge. Не заводить узел означало
		// бы потерять эти записи БЕЗ копии в пакете - хуже, чем завести: пакет как раз
		// и есть механизм сохранить копию перед необратимым удалением. Если нужен
		// отдельный более долгий срок хранения именно доказательства согласия - это
		// организационная процедура поверх пакета, вне этого среза.
		{"pd_consents", "user_id IN (" + orgUsers + ")"},
		{"push_subscriptions", "user_id IN (" + orgUsers + ")"}, // подписки на веб-push
		// used_passwords - хеши прежних паролей пользователя, запрет их повторного
		// использования (#1972). В пакете уже лежит users.password - хеш ДЕЙСТВУЮЩЕГО
		// пароля, и хеши прежних той же природы и той же чувствительности, отдельной
		// категории риска не создают. А вот их ОТСУТСТВИЕ в пакете было бы дырой: после
		// разворота на другом стенде запрет повторного использования начал бы работать с
		// чистого листа, и человек смог бы вернуть пароль, который система обязана была
		// помнить как использованный. Найдена ПЯТНАДЦАТОЙ тем же тестом
		// (TestOrganizationGraph_NoUnaccountedCascades), что нашёл 14 таблиц выше при
		// аудите 12.08, только позже и отдельно от него - таблица появилась следующим
		// мержем (#1972) уже после того аудита, узла для неё не завели.
		{"used_passwords", "user_id IN (" + orgUsers + ")"},
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

// GraphTables - имена таблиц графа entityType. Экспортируется ради внешних DB-backed
// замков (TestOrganizationGraph_NoUnaccountedCascades, internal/handlers/entity_graph_test.go):
// такому тесту нужна карта графа, а organizationNodes/allowedNodeTables неэкспортированы -
// заводить для одного теста дублирующую карту было бы худшим вариантом, чем узкий геттер.
func GraphTables(entityType string) ([]string, error) {
	allowed, err := allowedNodeTables(entityType)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(allowed))
	for t := range allowed {
		out = append(out, t)
	}
	return out, nil
}
