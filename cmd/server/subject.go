package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// Консольная работа с данными одного человека - субъекта персональных данных (#2356).
//
// Живёт рядом с entity и по тем же правилам: доступ к операции равен доступу к консоли
// сервера, а не к учётной записи. Разница в цели - entity работает с организацией,
// subject отвечает на вопрос «что система знает про этого человека», который задаёт
// государственный орган или сам человек.
//
// Первый срез умеет только показывать граф. Выгрузка сведений, журнал выдач и
// обезличивание идут отдельными срезами.

const subjectHelp = `Данные одного человека - субъекта персональных данных.

Использование:
  server subject show -passport="4510 123456"        Показать, что система знает о человеке
  server subject show -patent="77 1234567"           То же, если документ - патент
  server subject show -registry-id=N                 То же, по записи реестра сотрудников
  server subject find -fio="Иванов Иван Иванович"    Найти записи с таким именем
  server subject export -registry-id=N [-apply]      Снять справку о человеке в файл
  server subject disclosures [-registry-id=N]        Показать журнал выдач сведений
  server subject anonymize -registry-id=N [-apply]   Необратимо обезличить человека

Флаги show (нужен ровно один способ указать человека):
  -passport      Серия и номер паспорта как он введён в системе
  -patent        Номер патента
  -registry-id   Идентификатор записи в реестре сотрудников: свёртки берутся из неё

Флаги find:
  -fio           Фамилия Имя Отчество через пробел

Флаги export (человек указывается так же, как в show):
  -apply         Записать файлы. Без него команда только показывает объём справки
  -recipient     Кому выдаются сведения: наименование органа или «субъекту лично»
  -request       Реквизиты запроса: номер и дата письма, по которому выдаётся
  -basis         Основание своими словами. Необязательно
  -issued-by     Кто выдаёт. По умолчанию - «консоль сервера»

Флаги anonymize (человек указывается так же, как в show):
  -apply         Выполнить затирание. Без него команда только показывает, что затрёт
  -basis         Основание уничтожения для журнала уничтожения. Одно из фиксированных
                 значений: retention (истёк срок хранения), operator (решение оператора),
                 subject-request (требование субъекта). По умолчанию operator. Это НЕ тот
                 -basis, что у export: там основание выдачи пишется своими словами

Флаги disclosures:
  -registry-id   Показать выдачи по одному человеку. Без него - весь журнал
  -limit         Сколько записей показать. По умолчанию 50

show ничего не меняет: считает, сколько строк каждой таблицы относится к человеку -
записи реестра, участие в заявках, привязки к постам, файлы, история и отметки прохода.
Заявки печатаются отдельным списком: они принадлежат организации, а не человеку, но
именно по ним видно, когда и куда он приходил.

Склейка записей идёт по свёртке документа - она детерминирована и заведена ровно для
такого сравнения: находит точное совпадение, не раскрывая значения. По ФИО записи НЕ
склеиваются: однофамильцы существуют, а приписать человеку чужие проходы в ответе
государственному органу - ошибка, которую потом никто не заметит. Для поиска по имени
есть отдельная команда find: она показывает кандидатов, решает человек.

Учётная запись в граф не входит: связи «работник - пользователь системы» в базе нет,
поле user_id у записи реестра означает владельца записи, а не самого работника.

Выдача фиксируется в журнале: -apply без -recipient и -request отказывает. Передача
сведений третьему лицу - это раскрытие персональных данных, и при проверке оператор
обязан показать, кому, когда, по какому запросу и в каком объёме их выдали. Журнал
общей уборкой не чистится: это документ, а не служебная запись.

export собирает справку из четырёх разделов - сведения, заявки, проходы, посты - и
кладёт её в ENTITY_EXPORT_PATH двумя файлами: .xlsx для работы и .pdf для приложения к
официальному ответу. Документы в справке расшифрованы: её и составляют затем, чтобы
ответить органу по существу. Каталог выгрузки выбирает владелец системы - в файлах
лежат персональные данные целиком.

Основание обработки печатается в шапке каждого раздела. Отметка уведомления работника
и его возражение против обработки идут отдельными столбцами - именно они меняют режим
работы с записью.

anonymize необратимо затирает ФИО и документы человека вместе с их отпечатками во всех
трёх таблицах - записи реестра, участники заявок, участники в шапке. Учётную запись не
трогает: связи «работник - пользователь системы» в базе нет, и затирать пользователя по
совпадению имени значило бы обезличить однофамильца.

ПОСЛЕ обезличивания человек перестаёт находиться: свёртка документа стирается вместе со
значением, и собрать по нему сведения больше нельзя. Это не побочный эффект, а смысл
операции - поэтому справку, если она нужна, снимают ДО.

Журнал истории и проходов команда не трогает: он доказывает, кто и когда был на
объекте. Имя человека в пояснениях к проходам остаётся - команда говорит, сколько таких
записей, чтобы решение принимал человек, а не молчание программы.

Примеры:
  server subject find -fio="Иванов Иван Иванович"
  server subject show -registry-id=416
  server subject show -passport="4510 123456"
  server subject export -registry-id=416
  server subject export -registry-id=416 -apply -recipient="УМВД по г. Москве" -request="исх. 12/345 от 08.09.2026"
  server subject disclosures -registry-id=416
  server subject anonymize -registry-id=416
  server subject anonymize -registry-id=416 -apply
`

func runSubject(args []string) int {
	if len(args) == 0 {
		fmt.Print(subjectHelp)
		return 2
	}
	switch args[0] {
	case "help", "-help", "--help":
		fmt.Print(subjectHelp)
		return 0
	case "show":
		return subjectShow(args[1:])
	case "find":
		return subjectFind(args[1:])
	case "export":
		return subjectExport(args[1:])
	case "disclosures":
		return subjectDisclosures(args[1:])
	case "anonymize":
		return subjectAnonymize(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "неизвестная подкоманда %q\n\n", args[0])
		fmt.Print(subjectHelp)
		return 2
	}
}

// subjectPrepare открывает базу и включает ключ шифрования: без него свёртка документа
// считается passthrough и не совпадёт ни с чем в базе, где данные зашифрованы.
func subjectPrepare() (*gorm.DB, int) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return nil, 1
	}
	encKey, err := crypto.ParseHexKey(cfg.DataEncryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: DATA_ENCRYPTION_KEY:", err)
		return nil, 1
	}
	crypto.SetGlobalKey(encKey)

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return nil, 1
	}
	return db, -1
}

func subjectShow(args []string) int {
	fs := flag.NewFlagSet("subject show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, subjectHelp) }
	passport := fs.String("passport", "", "серия и номер паспорта")
	patent := fs.String("patent", "", "номер патента")
	registryID := fs.Int("registry-id", 0, "идентификатор записи реестра")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	byDocument := strings.TrimSpace(*passport) != "" || strings.TrimSpace(*patent) != ""
	if byDocument == (*registryID > 0) {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите либо -passport/-patent, либо -registry-id")
		return 2
	}

	db, code := subjectPrepare()
	if code >= 0 {
		return code
	}

	ctx := context.Background()
	target, code := subjectResolveTarget(ctx, db, *passport, *patent, *registryID)
	if code >= 0 {
		return code
	}

	graph, err := entityarchive.CollectSubject(ctx, db, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	apps, err := entityarchive.SubjectApplications(ctx, db, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	fmt.Printf("Данные о человеке, найденные %s\n\n", target.Origin)
	if len(graph.Tables) == 0 {
		fmt.Println("Записей не найдено.")
		return 0
	}
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	for _, t := range graph.Tables {
		fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.FormatInt(t.Rows, 10), 10))
	}
	fmt.Println()
	fmt.Printf("Всего строк: %d\n", graph.Total())

	fmt.Printf("\nЗаявки с его участием: %d\n", len(apps))
	for _, a := range apps {
		number, status := "без номера", ""
		if a.ApplicationNumber != nil {
			number = *a.ApplicationNumber
		}
		if a.Status != nil {
			status = *a.Status
		}
		fmt.Printf("  %s  %s\n", padRight(number, 20), status)
	}
	return 0
}

func subjectFind(args []string) int {
	fs := flag.NewFlagSet("subject find", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, subjectHelp) }
	fio := fs.String("fio", "", "Фамилия Имя Отчество")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	parts := strings.Fields(*fio)
	if len(parts) < 2 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -fio хотя бы из фамилии и имени")
		return 2
	}
	middle := ""
	if len(parts) > 2 {
		middle = strings.Join(parts[2:], " ")
	}

	db, code := subjectPrepare()
	if code >= 0 {
		return code
	}

	candidates, err := entityarchive.FindSubjectCandidatesByFIO(context.Background(), db, parts[0], parts[1], middle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	if len(candidates) == 0 {
		fmt.Println("Записей с таким именем не найдено.")
		return 0
	}
	fmt.Printf("Найдено людей с таким именем: %d\n", len(candidates))
	fmt.Println("Это НЕ обязательно один человек - однофамильцы существуют. Строки склеены")
	fmt.Println("по документу: сколько за человеком записей, видно в столбцах справа.")
	fmt.Println()
	fmt.Println(" ", padRight("ФИО", 34), padRight("Реестр", 9), padRight("Заявки", 9), "Собрать")
	for _, c := range candidates {
		how := "нет документа"
		switch {
		case !c.HasDocument:
		case c.RegistryID > 0:
			how = fmt.Sprintf("-registry-id=%d", c.RegistryID)
		default:
			how = fmt.Sprintf("-employee-id=%d", c.EmployeeID)
		}
		fmt.Println(" ", padRight(c.FullName, 34),
			padRight(strconv.Itoa(c.RegistryRows), 9),
			padRight(strconv.Itoa(c.ApplicationRows), 9), how)
	}
	return 0
}

// subjectResolveTarget разбирает общие для show и export флаги указания человека.
func subjectResolveTarget(ctx context.Context, db *gorm.DB, passport, patent string, registryID int) (entityarchive.SubjectTarget, int) {
	if registryID > 0 {
		target, err := entityarchive.SubjectTargetFromRegistry(ctx, db, registryID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return entityarchive.SubjectTarget{}, 1
		}
		if target.Empty() {
			fmt.Fprintf(os.Stderr, "Ошибка: у записи реестра %d нет ни паспорта, ни патента - склеить по ней нечего\n", registryID)
			return entityarchive.SubjectTarget{}, 1
		}
		return target, -1
	}
	return entityarchive.SubjectTargetFromDocuments(passport, patent), -1
}

// subjectExport снимает справку. Без -apply только считает: справка уносит все
// персональные данные человека разом, и оператор обязан сперва увидеть её объём.
func subjectExport(args []string) int {
	fs := flag.NewFlagSet("subject export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, subjectHelp) }
	passport := fs.String("passport", "", "серия и номер паспорта")
	patent := fs.String("patent", "", "номер патента")
	registryID := fs.Int("registry-id", 0, "идентификатор записи реестра")
	apply := fs.Bool("apply", false, "записать файлы, а не только посчитать")
	recipient := fs.String("recipient", "", "кому выдаются сведения")
	request := fs.String("request", "", "реквизиты запроса")
	basis := fs.String("basis", "", "основание выдачи")
	issuedBy := fs.String("issued-by", "консоль сервера", "кто выдаёт сведения")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Обязательность получателя и реквизитов проверяется ДО сборки справки: собрать
	// персональные данные человека и только потом отказать - значит сделать лишнюю
	// работу с самыми чувствительными данными в системе.
	disclosure := entityarchive.DisclosureRequest{
		Recipient: *recipient, RequestRef: *request, Basis: *basis, IssuedBy: *issuedBy,
	}
	if *apply {
		if err := disclosure.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			fmt.Fprintln(os.Stderr, "Выдача сведений третьему лицу фиксируется в журнале, и без этих")
			fmt.Fprintln(os.Stderr, "сведений запись бессмысленна: при проверке нечем показать, что")
			fmt.Fprintln(os.Stderr, "раскрытие было законным.")
			return 2
		}
	}

	byDocument := strings.TrimSpace(*passport) != "" || strings.TrimSpace(*patent) != ""
	if byDocument == (*registryID > 0) {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите либо -passport/-patent, либо -registry-id")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	if strings.TrimSpace(cfg.EntityExportPath) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: не задан ENTITY_EXPORT_PATH - каталог, куда складывать справки.")
		fmt.Fprintln(os.Stderr, "В справке лежат персональные данные человека целиком, поэтому место хранения")
		fmt.Fprintln(os.Stderr, "выбирает владелец системы, а не программа.")
		return 2
	}

	db, code := subjectPrepare()
	if code >= 0 {
		return code
	}

	ctx := context.Background()
	target, code := subjectResolveTarget(ctx, db, *passport, *patent, *registryID)
	if code >= 0 {
		return code
	}

	rep, err := entityarchive.BuildSubjectReport(ctx, db, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	fmt.Printf("Справка о человеке, найденном %s\n\n", rep.Origin)
	for _, s := range rep.Sections {
		fmt.Println(" ", padRight(s.Title, 24), padLeft(strconv.Itoa(len(s.Rows)), 8), "строк")
	}
	fmt.Printf("\nВсего строк: %d\n", entityarchive.SubjectReportRowCount(rep))

	if !*apply {
		fmt.Println("\nФайлы не записаны: добавьте -apply.")
		return 0
	}

	written, err := entityarchive.WriteSubjectReport(cfg.EntityExportPath, rep)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	entry, err := entityarchive.RecordDisclosure(ctx, db, target, rep, written, disclosure)
	if err != nil {
		// Файлы уже на диске, а записи о выдаче нет - молчать об этом нельзя:
		// незафиксированная выдача при проверке неотличима от утечки.
		fmt.Fprintln(os.Stderr, "Ошибка: файлы записаны, но выдача НЕ попала в журнал:", err)
		fmt.Fprintln(os.Stderr, "Файлы:", strings.Join(written, ", "))
		return 1
	}

	fmt.Println("\nЗаписано:")
	for _, f := range written {
		fmt.Println(" ", f)
	}
	fmt.Printf("\nВыдача внесена в журнал под номером %d: %s, %s\n",
		entry.ID, entry.Recipient, entry.RequestRef)
	return 0
}

// subjectDisclosures печатает журнал выдач - целиком или по одному человеку.
func subjectDisclosures(args []string) int {
	fs := flag.NewFlagSet("subject disclosures", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, subjectHelp) }
	registryID := fs.Int("registry-id", 0, "идентификатор записи реестра")
	limit := fs.Int("limit", 50, "сколько записей показать")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	db, code := subjectPrepare()
	if code >= 0 {
		return code
	}

	ctx := context.Background()
	hmac := ""
	if *registryID > 0 {
		target, err := entityarchive.SubjectTargetFromRegistry(ctx, db, *registryID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 1
		}
		hmac = entityarchive.DisclosureSubjectKey(target)
	}

	entries, err := entityarchive.ListDisclosures(ctx, db, hmac, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	if len(entries) == 0 {
		fmt.Println("Выдач не зарегистрировано.")
		return 0
	}
	fmt.Printf("Выдач в журнале: %d\n\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s  %s\n", e.CreatedAt.Format("02.01.2006 15:04"), e.SubjectName)
		fmt.Printf("    кому: %s\n", e.Recipient)
		fmt.Printf("    запрос: %s\n", e.RequestRef)
		if e.Basis != "" {
			fmt.Printf("    основание: %s\n", e.Basis)
		}
		fmt.Printf("    объём: %s\n", e.Scope)
		fmt.Printf("    выдал: %s\n\n", e.IssuedBy)
	}
	return 0
}

// subjectAnonymize необратимо затирает персональные поля человека. Без -apply только
// показывает, что затёр бы: операция необратима, и увидеть её объём человек обязан
// до, а не после.
func subjectAnonymize(args []string) int {
	fs := flag.NewFlagSet("subject anonymize", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, subjectHelp) }
	passport := fs.String("passport", "", "серия и номер паспорта")
	patent := fs.String("patent", "", "номер патента")
	registryID := fs.Int("registry-id", 0, "идентификатор записи реестра")
	apply := fs.Bool("apply", false, "выполнить затирание, а не только показать")
	basisFlag := fs.String("basis", entityarchive.BasisOperator,
		"основание уничтожения: "+strings.Join(entityarchive.DestructionBases(), ", "))
	if err := fs.Parse(args); err != nil {
		return 2
	}
	basis, err := entityarchive.ParseDestructionBasis(*basisFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 2
	}

	byDocument := strings.TrimSpace(*passport) != "" || strings.TrimSpace(*patent) != ""
	if byDocument == (*registryID > 0) {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите либо -passport/-patent, либо -registry-id")
		return 2
	}

	db, code := subjectPrepare()
	if code >= 0 {
		return code
	}

	ctx := context.Background()
	target, code := subjectResolveTarget(ctx, db, *passport, *patent, *registryID)
	if code >= 0 {
		return code
	}

	res, err := entityarchive.AnonymizeSubject(ctx, db, services.NewAuditRecorder(db), target,
		entityarchive.DestructionOptions{Basis: basis, Apply: *apply})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	if *apply {
		fmt.Printf("Обезличен человек, найденный %s\n\n", res.Origin)
	} else {
		fmt.Printf("Будет обезличен человек, найденный %s\n\n", res.Origin)
	}
	fmt.Println("Затираются поля:")
	for _, f := range res.Tables[0].Fields {
		fmt.Println("  -", f)
	}
	fmt.Println()
	for _, tbl := range res.Tables {
		fmt.Println(" ", padRight(tbl.Table, 24), padLeft(strconv.Itoa(tbl.Rows), 8), "строк")
	}
	fmt.Printf("\nВсего строк: %d\n", res.Total())

	fmt.Println("\nОстаётся после обезличивания:")
	for _, w := range res.Warnings {
		fmt.Println("  -", w)
	}

	if !*apply {
		fmt.Println("\nНичего не изменено: добавьте -apply. Операция необратима.")
	}
	return 0
}
