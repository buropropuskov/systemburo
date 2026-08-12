package entityarchive

// Срез 5: необратимое обезличивание персональных полей организации консольной командой
// entity anonymize.
//
// Отличие от retire (срез 4, соседний файл) принципиальное, а не косметическое: retire
// гасит is_active и полностью обратим через restore, anonymize НИЧЕГО не гасит и не
// архивирует - связи, история и работа системы остаются как есть, но ФИО и документы
// сотрудников и пользователей организации стираются БЕЗ возможности вернуть. Поэтому у
// anonymize нет restore-команды: откатывать нечего, откатывать было бы нечестно.
// retire и anonymize можно применять к одной организации независимо и в любом порядке -
// они трогают разные столбцы одних и тех же строк.
//
// Перечень полей, что и почему затирается - anonymizeTargets() ниже, она же формирует
// вывод "что будет затёрто" перед -apply (см. printAnonymizeResult в cmd/server/entity.go).
// Главная ловушка среза: вместе со значением закрытого поля (паспорт, патент) обязан
// затираться и его HMAC-отпечаток. Отпечаток детерминирован (crypto.HMACOptional), и по
// нему можно проверить гипотезу "это паспорт такого-то", даже когда само значение стёрто -
// затирание значения без отпечатка обезличиванием не является. Ловушка тем более реальна,
// что хук моделей (Employee/UniqueEmployee/ApplicationEmployee.BeforeSave) пересчитывает
// HMAC ТОЛЬКО когда значение поля не nil (обычная запись данных) - если положиться на
// хук и просто занулить PassportSeriesNumber, отпечаток останется от уже стёртого
// значения. Поэтому оба поля пары (значение + hmac) обнуляются здесь явно, ДО Save(), а
// хук на нулевом значении корректно ничего не делает (тот же приём, что лечит смежный
// урок про nil vs &"" у HMAC-полей, lessons/backend.md).
//
// Поля под ключом системы по-прежнему пишутся через Save() модели, а не сырым UPDATE:
// это не вопрос производительности, а инвариант - модель одна отвечает за то, что
// значение и его HMAC согласованы, и это должно оставаться верным для ЛЮБОЙ записи
// строки, включая эту.
//
// Супер-администратора организации anonymize не трогает - тот же запрет, что у Retire
// ("иначе админ может вырубить владельца"), только здесь он ещё жёстче: обезличивание
// заменяет username на deleted_<id>, а значит вход под прежним логином становится
// невозможен - это НЕОБРАТИМАЯ блокировка, только неочевидная. Организация владельца
// системы вполне может стать целью среза, и тогда одной командой систему лишили бы
// доступа без способа откатить. Пропуск обязан быть виден в результате (SkippedSuperAdmins)
// и в dry-run, и после apply - молчание превратило бы неполное обезличивание в мнимо
// полное.
//
// Обезличенный пользователь теряет вход под прежним логином сразу, но его активная
// сессия (refresh-токен) без отдельного действия продолжила бы жить до истечения
// собственного срока - та же дыра, которую user_service.setActive закрывает при
// архивации одного пользователя. Поэтому apply отзывает активные refresh-токены ровно
// тех пользователей, которых обезличил, той же функцией revokeActiveTokens, что и Retire
// (retire.go), и в той же транзакции.
//
// applications - четвёртая таблица среди целей: у каждой заявки есть шапка подачи
// "Инициатор заявки"/"Телефон" (InitiatorName/ContactPhone, #1454) - ФИО и телефон
// живого человека открытым текстом, и по комментарию модели это МОЖЕТ БЫТЬ ДРУГОЙ
// человек, не отправитель (сам отправитель обезличивается отдельно как users). Без этой
// пары полей ревью нашло дыру: организация числилась бы обезличенной, а в каждой её
// заявке оставалось бы читаемое имя и телефон. У Application нет BeforeSave/AfterFind -
// поля обычные, шифрования и отпечатков не несут.
//
// Ревью тем же взглядом прошло по остальным колонкам графа организации в поисках других
// мест, где имя/телефон/документ лежат открытым текстом. Решения задокументированы у
// каждого места и в отчёте среза, коротко: свободный текст (message, comment, subject,
// notes и т.п.) не трогается - это неструктурированная переписка по рабочему процессу, а
// не выделенное поле идентификации, и его вычитка - другая задача (контентная модерация,
// не полевое обезличивание); attachments.attachment_name/attachment_display_name - имя
// БЛАНКА из справочника, не человека; car_number/mark/item name - имущество, не люди
// (явно исключены и в задаче на срез: "не трогать... номера машин").
//
// application_blacklist_flags/overrides - разбор ПОЛЕ ЗА ПОЛЕМ, а не таблица целиком
// (второй заход ревью поправил первую версию, где обе таблицы были целиком отнесены к
// "истории"):
//   - element_normalized - нормализованная форма (normalize.Name/normalize.Plate)
//     САМОГО ЭЛЕМЕНТА заявки (комментарий модели), то есть ФИО СВОЕГО сотрудника у
//     element_type=employee - такой же идентификатор человека, как имя в employees, и
//     обязан затираться. У element_type=car это номер машины - тот трогать нельзя (см.
//     выше), поэтому обезличивание берёт из этих двух таблиц ТОЛЬКО employee-строки
//     (employeeElementIDs ниже), а не весь узел графа.
//   - matched_value/matched_reason (flags) и matched_value/comment (overrides) - снимок
//     ЗАПИСИ ЧЁРНОГО СПИСКА, с которой сравнили элемент, то есть данные ЧУЖОГО человека,
//     занесённого в ЧС кем-то другим. Он к этой организации отношения не имеет, и стирать
//     чужие данные при обезличивании ЭТОЙ организации нельзя - остаются нетронутыми.
//   - последствие: element_normalized вместе с matched_blacklist_id служит ключом
//     подавления повторных предупреждений ("всё равно пропустить", #481) - после
//     обезличивания подавление по этой паре перестанет работать для обезличенной
//     организации. Для неё самой это не имеет значения (её состав больше не подаётся),
//     но это последствие, а не побочный эффект незамеченный - явно назвать оператору
//     нечем (после zeroing строка просто не совпадёт ни с одним будущим элементом), и
//     оно зафиксировано здесь и в отчёте среза, а не обнаружится потом.
//
// Файловые артефакты вне базы (application_files и слепки бланков заявка.json в архиве
// ARCHIVE_PATH - blank_export_snapshot.go, тот же паспорт/патент, а также ФИО и телефон
// инициатора открытым текстом на момент выпуска бланка) эта команда не трогает физически -
// оба факта названы в выводе (см. anonymizeWarnings), молчать нельзя ни про один из них.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// AnonymizeTableResult - одна таблица графа, которую трогает Anonymize: имя, человеко-
// читаемый перечень полей (используется выводом - см. printAnonymizeResult) и число
// строк - затронутых (apply) или которые будут затронуты (dry-run).
type AnonymizeTableResult struct {
	Table  string
	Fields []string
	Rows   int
}

// AnonymizeResult - что затёр (или затёр бы, при dry-run) Anonymize.
type AnonymizeResult struct {
	Type   string
	ID     int
	Tables []AnonymizeTableResult
	// SkippedSuperAdmins - активные супер-админы организации, которых anonymize НЕ
	// обезличил. Непустое значение оператор обязан увидеть, а не узнать постфактум, что
	// офбординг персональных данных выглядел полным, а владелец системы лишился входа.
	SkippedSuperAdmins []int
	// Warnings - то, что оператор обязан увидеть, но что anonymize сознательно не делает:
	// application_files (сканы документов) и слепки бланков в архиве (заявка.json,
	// ARCHIVE_PATH) в этот срез не входят и остаются читаемыми; matched_value/matched_reason/
	// comment в blacklist-таблицах не затираются (чужая запись) и могут текстуально
	// совпадать с только что обезличенным именем.
	Warnings []string
}

// Total - сколько строк затронуто по всем таблицам.
func (r AnonymizeResult) Total() int {
	n := 0
	for _, t := range r.Tables {
		n += t.Rows
	}
	return n
}

// anonymizeAuditTable / anonymizeDetails - содержимое audit_log.details записи
// anonymize. Только имена таблиц и счётчики строк - никаких значений: смысл действия в
// том, чтобы стереть персональные данные, а не переложить их копию в другой журнал.
// Статический перечень полей (AnonymizeTableResult.Fields) сюда не попадает - это факт
// кода этой версии, а не факт конкретного запуска.
type anonymizeAuditTable struct {
	Table string `json:"table"`
	Rows  int    `json:"rows"`
}

type anonymizeDetails struct {
	Tables []anonymizeAuditTable `json:"tables"`
	// SkippedSuperAdmins - тот же список, что и в AnonymizeResult, зафиксированный на
	// момент действия: история обязана описывать ровно то, что реально произошло,
	// включая то, что было сознательно пропущено (см. retireDetails в retire.go).
	SkippedSuperAdmins []int `json:"skipped_super_admins,omitempty"`
}

// employeeLikeFields - перечень полей, общий для employees/unique_employees/
// application_employees: у всех трёх один и тот же набор из ФИО, документов и их
// отпечатков (см. BeforeSave/AfterFind этих трёх моделей в internal/models/employee.go -
// они почти дословно повторяют друг друга тем же приёмом).
func employeeLikeFields() []string {
	return []string{
		"last_name", "first_name", "middle_name",
		"passport_series_number (и отпечаток passport_series_number_hmac)",
		"patent_number (и отпечаток patent_number_hmac)",
		"other_permission",
	}
}

// anonymizeTargets - таблицы и поля, которые обезличивает Anonymize. Порядок - как в
// задаче: сначала три таблицы сотрудников, потом пользователи организации. ИТОГОВОЕ
// решение о полноте перечня - за владельцем системы; команда обязана печатать его перед
// выполнением (см. printAnonymizeResult), чтобы человек видел, что именно будет затёрто,
// до того как нажмёт -apply.
func anonymizeTargets() []AnonymizeTableResult {
	fields := employeeLikeFields()
	blacklistFields := []string{
		"element_normalized (только у строк своего сотрудника - element_type=employee; " +
			"у element_type=car это номер машины, не трогается)",
	}
	return []AnonymizeTableResult{
		{Table: "employees", Fields: fields},
		{Table: "unique_employees", Fields: fields},
		{Table: "application_employees", Fields: fields},
		{Table: "applications", Fields: []string{
			"initiator_name", "contact_phone",
		}},
		{Table: blacklistFlagsTable, Fields: blacklistFields},
		{Table: blacklistOverridesTable, Fields: blacklistFields},
		{Table: usersTable, Fields: []string{
			"last_name", "first_name", "middle_name", "email", "phone",
			"username (заменяется на псевдоним deleted_<id> - пустым или задвоенным его " +
				"оставить нельзя, на нём уникальность и вход)",
		}},
	}
}

// Anonymize необратимо затирает персональные поля сотрудников и пользователей
// организации. apply=false - только подсчёт того, что попало бы под затирание, база не
// меняется. Связи между записями, audit_log/pd_audit_logs и любая история, должности,
// номера машин, счётчики и даты сущностей не трогаются - меняются только перечисленные
// в anonymizeTargets колонки.
func Anonymize(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, entityType string, id int, actorID *int, apply bool) (AnonymizeResult, error) {
	if entityType != TypeOrganization {
		return AnonymizeResult{}, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}

	if !apply {
		exists, err := orgExists(ctx, db, id)
		if err != nil {
			return AnonymizeResult{}, err
		}
		if !exists {
			return AnonymizeResult{}, fmt.Errorf("организация #%d не найдена", id)
		}
		tables := anonymizeTargets()
		var skipped []int
		for i := range tables {
			ids, sk, err := targetIDsForTable(ctx, db, tables[i].Table, id)
			if err != nil {
				return AnonymizeResult{}, err
			}
			tables[i].Rows = len(ids)
			if sk != nil {
				skipped = sk
			}
		}
		warnings, err := anonymizeWarnings(ctx, db, id)
		if err != nil {
			return AnonymizeResult{}, err
		}
		return AnonymizeResult{Type: entityType, ID: id, Tables: tables, SkippedSuperAdmins: skipped, Warnings: warnings}, nil
	}

	res := AnonymizeResult{Type: entityType, ID: id, Tables: anonymizeTargets()}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Проверка существования - внутри ЭТОЙ ЖЕ транзакции, тем же приёмом, что и
		// Retire: снаружи организация могла бы исчезнуть между проверкой и записью.
		exists, err := orgExists(ctx, tx, id)
		if err != nil {
			return err
		}
		if !exists {
			return errOrgNotFound
		}

		for i := range res.Tables {
			table := res.Tables[i].Table
			ids, skipped, err := targetIDsForTable(ctx, tx, table, id)
			if err != nil {
				return err
			}
			if skipped != nil {
				res.SkippedSuperAdmins = skipped
			}
			n, err := anonymizeRows(ctx, tx, table, ids)
			if err != nil {
				return err
			}
			if table == usersTable {
				// Обезличенный пользователь не может войти под прежним логином, но его
				// уже открытая сессия без этого шага дожила бы до истечения собственного
				// TTL - зеркалит user_service.setActive/Retire (revokeActiveTokens ниже,
				// та же функция, что в retire.go).
				if err := revokeActiveTokens(tx, ids); err != nil {
					return err
				}
			}
			res.Tables[i].Rows = n
		}

		warnings, err := anonymizeWarnings(ctx, tx, id)
		if err != nil {
			return err
		}
		res.Warnings = warnings

		details := make([]anonymizeAuditTable, len(res.Tables))
		for i, t := range res.Tables {
			details[i] = anonymizeAuditTable{Table: t.Table, Rows: t.Rows}
		}
		// Запись аудита - последним шагом транзакции, тем же приёмом, что и Retire: если
		// затирание не выполнилось, метка "сделано" не появится вовсе.
		return recorder.Record(ctx, tx, models.AuditEntityOrganization, &id, models.OrganizationActionAnonymized, actorID,
			anonymizeDetails{Tables: details, SkippedSuperAdmins: res.SkippedSuperAdmins})
	})
	switch {
	case errors.Is(err, errOrgNotFound):
		return AnonymizeResult{}, fmt.Errorf("организация #%d не найдена", id)
	case err != nil:
		return AnonymizeResult{}, fmt.Errorf("anonymize %s #%d: %w", entityType, id, err)
	}
	return res, nil
}

// Имена таблиц, для которых Anonymize не может обойтись голым предикатом графа
// (targetIDs) - у КАЖДОЙ из трёх часть строк графа в цель обезличивания не входит:
//   - usersTable: супер-админ организации (resolveUserTargets);
//   - blacklistFlagsTable/blacklistOverridesTable: строки element_type=car - у них
//     element_normalized несёт номер машины, а не ФИО (employeeElementIDs).
const (
	usersTable              = "users"
	blacklistFlagsTable     = "application_blacklist_flags"
	blacklistOverridesTable = "application_blacklist_overrides"
)

// targetIDsForTable - id строк table, которых КАСАЕТСЯ обезличивание, и (только там, где
// это применимо - пока только users) список пропущенных. Единая точка входа что для
// dry-run, что для apply: три таблицы выше не могут обойтись голым nodeWhere, у остальных
// predicate - это он и есть без изменений.
func targetIDsForTable(ctx context.Context, exec *gorm.DB, table string, id int) (ids []int, skipped []int, err error) {
	switch table {
	case usersTable:
		return resolveUserTargets(ctx, exec, id)
	case blacklistFlagsTable, blacklistOverridesTable:
		ids, err = employeeElementIDs(ctx, exec, table, id)
		return ids, nil, err
	default:
		ids, err = targetIDs(ctx, exec, table, id)
		return ids, nil, err
	}
}

// targetIDs - id строк table, принадлежащих организации id. Предикат берётся из
// organizationNodes() (registry.go) - того же графа, на который опираются show/export/
// retire, а не переписывается заново здесь: иначе список целей обезличивания рано или
// поздно разойдётся с картой связей организации.
func targetIDs(ctx context.Context, exec *gorm.DB, table string, id int) ([]int, error) {
	where, err := nodeWhere(table)
	if err != nil {
		return nil, err
	}
	return idsWithPredicate(ctx, exec, table, where, id)
}

// resolveUserTargets - id пользователей организации, которых обезличивание ЗАТРОНЕТ
// (граф users, но БЕЗ супер-админа), и отдельно id пропущенных супер-админов. Предикат
// графа берётся из nodeWhere("users") и дополняется тем же запретом, что у Retire
// ("иначе админ может вырубить владельца") - см. комментарий пакета. COALESCE обязателен:
// users.is_super_admin в схеме DEFAULT false, но БЕЗ NOT NULL, и "AND is_super_admin" /
// "AND NOT is_super_admin" на NULL дают NULL (SQL three-valued logic) - без COALESCE
// строка с NULL не попала бы НИ в один из двух списков и осталась бы не обезличенной,
// хотя команда рапортовала бы полный охват (тот же дефект уже ловили в retire.go).
func resolveUserTargets(ctx context.Context, exec *gorm.DB, id int) (ids []int, skipped []int, err error) {
	where, err := nodeWhere(usersTable)
	if err != nil {
		return nil, nil, err
	}
	if ids, err = idsWithPredicate(ctx, exec, usersTable, where+" AND NOT COALESCE(is_super_admin, false)", id); err != nil {
		return nil, nil, err
	}
	if skipped, err = idsWithPredicate(ctx, exec, usersTable, where+" AND COALESCE(is_super_admin, false)", id); err != nil {
		return nil, nil, err
	}
	return ids, skipped, nil
}

// employeeElementIDs - id строк application_blacklist_flags/application_blacklist_overrides
// организации, где element_type=employee: element_normalized там несёт нормализованное ФИО
// СВОЕГО сотрудника (normalize.Name), и только эти строки входят в цель обезличивания.
// element_type=car несёт номер машины (normalize.Plate) - такие строки Anonymize не считает
// и не трогает вовсе, как будто их и нет в графе (см. комментарий пакета - номера машин
// исключены самой задачей на срез).
func employeeElementIDs(ctx context.Context, exec *gorm.DB, table string, id int) ([]int, error) {
	where, err := nodeWhere(table)
	if err != nil {
		return nil, err
	}
	return idsWithPredicate(ctx, exec, table, where+" AND element_type = 'employee'", id)
}

// idsWithPredicate - id строк table по ПРОИЗВОЛЬНОМУ предикату (с именованным параметром
// @org), а не обязательно графовому из nodeWhere. Нужен resolveUserTargets, где предикат
// графа дополняется условием на is_super_admin, и targetIDs, для которого predicate - это
// ровно nodeWhere(table) без изменений.
func idsWithPredicate(ctx context.Context, exec *gorm.DB, table, predicate string, id int) ([]int, error) {
	var ids []int
	q := fmt.Sprintf("SELECT id FROM %s WHERE %s ORDER BY id", table, predicate)
	if err := exec.WithContext(ctx).Raw(q, sql.Named("org", id)).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("выборка %s: %w", table, err)
	}
	return ids, nil
}

// nodeWhere ищет предикат таблицы в карте графа организации.
func nodeWhere(table string) (string, error) {
	for _, n := range organizationNodes() {
		if n.Table == table {
			return n.Where, nil
		}
	}
	return "", fmt.Errorf("таблица %s не входит в граф организации", table)
}

// anonymizeWarnings честно говорит про персональные данные, которые остаются читаемыми
// ВНЕ таблиц этого среза - файлы, приложенные к заявкам (application_files), и слепки
// бланков в архиве (заявка.json). Молчать про любой из них нельзя - иначе оператор решит,
// что персональных данных не осталось вовсе, хотя скан паспорта в приложенном файле или
// открытый текст в слепке по-прежнему читаются.
func anonymizeWarnings(ctx context.Context, exec *gorm.DB, id int) ([]string, error) {
	files, err := targetIDs(ctx, exec, "application_files", id)
	if err != nil {
		return nil, err
	}
	filesMsg := fmt.Sprintf("файлы, приложенные к заявкам организации (application_files, сейчас %d шт.) - "+
		"это тоже персональные данные (сканы паспортов и патентов), только в другом виде. Anonymize их "+
		"НЕ трогает: удалить или заменить их - отдельное решение владельца системы", len(files))

	// Число слепков здесь не считается - они лежат в файловом архиве бланков (ARCHIVE_PATH),
	// вне базы и вне этой транзакции, и увидеть их со стороны БД нечем. Предупреждение
	// безусловное: молчание для заявок без бланка не отличить от честного "проверено, слепков
	// нет" снаружи, а второе эта команда доказать не может.
	snapshotMsg := "если для заявок этой организации выпускались бланки - рядом с ними в файловом " +
		"архиве (ARCHIVE_PATH) лежит слепок заявка.json с паспортом, патентом, а также ФИО и " +
		"телефоном инициатора открытым текстом на момент выпуска бланка. Anonymize файлы архива " +
		"НЕ трогает и пересчитать их отсюда не может - решение по ним отдельное, за владельцем системы"

	// matched_value/matched_reason/comment в application_blacklist_flags/overrides - снимок
	// ЧУЖОЙ записи чёрного списка, и обезличивание этой организации его не трогает (см.
	// комментарий пакета). Но именно потому, что предупреждение о совпадении сработало,
	// matched_value в частом случае текстуально ПОХОЖ на element_normalized, который эта же
	// команда только что стёрла - оператор обязан узнать про это здесь же, а не из
	// статической справки, которую читал один раз.
	matchedMsg := "в application_blacklist_flags/application_blacklist_overrides остаётся снимок совпавшей " +
		"записи чёрного списка (matched_value/matched_reason/comment) - он относится к ДРУГОМУ человеку, " +
		"занесённому в список не этой организацией, поэтому не затирается, но текстуально может быть похож " +
		"на только что обезличенное имя своего сотрудника: совпадение с этой записью и было причиной " +
		"предупреждения"

	return []string{filesMsg, snapshotMsg, matchedMsg}, nil
}

// anonymizeRows затирает персональные поля в ids строках table - диспетчер по имени
// таблицы. ids уже отфильтрованы вызывающим (targetIDsForTable) под конкретную таблицу -
// эта функция и функции под ней про то, КАК затереть строку, а не КАКИЕ строки брать.
func anonymizeRows(ctx context.Context, tx *gorm.DB, table string, ids []int) (int, error) {
	switch table {
	case "employees":
		return anonymizeEmployees(ctx, tx, ids)
	case "unique_employees":
		return anonymizeUniqueEmployees(ctx, tx, ids)
	case "application_employees":
		return anonymizeApplicationEmployees(ctx, tx, ids)
	case "applications":
		return anonymizeApplications(ctx, tx, ids)
	case blacklistFlagsTable:
		return anonymizeApplicationBlacklistFlags(ctx, tx, ids)
	case blacklistOverridesTable:
		return anonymizeApplicationBlacklistOverrides(ctx, tx, ids)
	case usersTable:
		return anonymizeUsers(ctx, tx, ids)
	default:
		return 0, fmt.Errorf("обезличивание таблицы %s не реализовано", table)
	}
}

// anonymizeEmployees затирает ФИО и документы сотрудников заявок организации.
// PassportSeriesNumberHMAC/PatentNumberHMAC обнуляются ЗДЕСЬ, рядом со значением, а не
// оставлены хуку: Employee.BeforeSave пересчитывает HMAC только когда значение поля не
// nil (обычная запись), и на уже занулённом значении промолчал бы, оставив старый
// отпечаток - ровно та рассинхронизация, которую этот срез обязан не допустить.
func anonymizeEmployees(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.Employee
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение employees: %w", err)
	}
	for i := range rows {
		rows[i].LastName = nil
		rows[i].FirstName = nil
		rows[i].MiddleName = nil
		rows[i].PassportSeriesNumber = nil
		rows[i].PatentNumber = nil
		rows[i].PassportSeriesNumberHMAC = nil
		rows[i].PatentNumberHMAC = nil
		rows[i].OtherPermission = nil
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание employees #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeUniqueEmployees - зеркало anonymizeEmployees для реестра уникальных
// сотрудников организации (тот же набор полей и та же ловушка HMAC).
func anonymizeUniqueEmployees(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.UniqueEmployee
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение unique_employees: %w", err)
	}
	for i := range rows {
		rows[i].LastName = nil
		rows[i].FirstName = nil
		rows[i].MiddleName = nil
		rows[i].PassportSeriesNumber = nil
		rows[i].PatentNumber = nil
		rows[i].PassportSeriesNumberHMAC = nil
		rows[i].PatentNumberHMAC = nil
		rows[i].OtherPermission = nil
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание unique_employees #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeApplicationEmployees - зеркало anonymizeEmployees для сотрудников,
// зафиксированных в самой заявке (application_employees, снимок на момент подачи).
func anonymizeApplicationEmployees(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.ApplicationEmployee
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение application_employees: %w", err)
	}
	for i := range rows {
		rows[i].LastName = nil
		rows[i].FirstName = nil
		rows[i].MiddleName = nil
		rows[i].PassportSeriesNumber = nil
		rows[i].PatentNumber = nil
		rows[i].PassportSeriesNumberHMAC = nil
		rows[i].PatentNumberHMAC = nil
		rows[i].OtherPermission = nil
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание application_employees #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeApplications затирает ФИО и телефон инициатора из шапки подачи заявки
// (InitiatorName/ContactPhone, #1454) - дефект, найденный ревью: заявка их несёт отдельно
// от отправителя (SenderUserID -> users, уже обезличивается своим путём), и по
// комментарию модели инициатором вполне может быть указан ДРУГОЙ человек. Application не
// несёт HMAC и хуков BeforeSave/AfterFind - обычная запись, как у employees, но без
// ловушки отпечатка.
func anonymizeApplications(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.Application
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение applications: %w", err)
	}
	for i := range rows {
		rows[i].InitiatorName = nil
		rows[i].ContactPhone = nil
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание applications #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeApplicationBlacklistFlags затирает element_normalized - нормализованное ФИО
// СВОЕГО сотрудника у строк element_type=employee (ids уже отфильтрованы вызывающим,
// employeeElementIDs - car сюда не попадает). matched_value/matched_reason НЕ трогаются:
// это снимок ЗАПИСИ чёрного списка, то есть данные ЧУЖОГО человека, попавшего в список не
// этой организацией - обезличивание ЭТОЙ организации его не касается (см. комментарий
// пакета). ElementNormalized - обычный string, не *string: пустая строка, а не nil.
func anonymizeApplicationBlacklistFlags(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.ApplicationBlacklistFlag
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение application_blacklist_flags: %w", err)
	}
	for i := range rows {
		rows[i].ElementNormalized = ""
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание application_blacklist_flags #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeApplicationBlacklistOverrides - зеркало anonymizeApplicationBlacklistFlags для
// таблицы аудита решений "всё равно пропустить": тот же element_normalized своего
// сотрудника затирается, matched_value/comment (данные чужой записи ЧС и текст решения по
// НЕЙ, не по нашему человеку) не трогаются.
func anonymizeApplicationBlacklistOverrides(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.ApplicationBlacklistOverride
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение application_blacklist_overrides: %w", err)
	}
	for i := range rows {
		rows[i].ElementNormalized = ""
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание application_blacklist_overrides #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}

// anonymizeUsers затирает ФИО и контакты пользователей организации. Username не
// зануляется и не пустеет: на нём держится уникальность записи и вход (login ищет
// строку по нему), поэтому вместо nil/"" - псевдоним по идентификатору, который
// гарантированно уникален и никогда не совпадёт с чужим.
func anonymizeUsers(ctx context.Context, tx *gorm.DB, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []models.User
	if err := tx.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("чтение users: %w", err)
	}
	for i := range rows {
		rows[i].LastName = nil
		rows[i].FirstName = nil
		rows[i].MiddleName = nil
		rows[i].Email = nil
		rows[i].Phone = nil
		rows[i].Username = fmt.Sprintf("deleted_%d", rows[i].ID)
		if err := tx.Save(&rows[i]).Error; err != nil {
			return 0, fmt.Errorf("обезличивание users #%d: %w", rows[i].ID, err)
		}
	}
	return len(rows), nil
}
