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

Флаги show (нужен ровно один способ указать человека):
  -passport      Серия и номер паспорта как он введён в системе
  -patent        Номер патента
  -registry-id   Идентификатор записи в реестре сотрудников: свёртки берутся из неё

Флаги find:
  -fio           Фамилия Имя Отчество через пробел

Флаги export (человек указывается так же, как в show):
  -apply         Записать файлы. Без него команда только показывает объём справки

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

export собирает справку из четырёх разделов - сведения, заявки, проходы, посты - и
кладёт её в ENTITY_EXPORT_PATH двумя файлами: .xlsx для работы и .pdf для приложения к
официальному ответу. Документы в справке расшифрованы: её и составляют затем, чтобы
ответить органу по существу. Каталог выгрузки выбирает владелец системы - в файлах
лежат персональные данные целиком.

Основание обработки печатается в шапке каждого раздела: законный интерес оператора,
п. 7 ч. 1 ст. 6 152-ФЗ. Отметка уведомления работника и его возражение против обработки
идут отдельными столбцами - именно они меняют режим работы с записью.

Примеры:
  server subject find -fio="Иванов Иван Иванович"
  server subject show -registry-id=416
  server subject show -passport="4510 123456"
  server subject export -registry-id=416
  server subject export -registry-id=416 -apply
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
	fmt.Printf("Найдено записей с таким именем: %d\n", len(candidates))
	fmt.Println("Это НЕ обязательно один человек - однофамильцы существуют.")
	fmt.Println("Собрать данные можно по записи с документом: server subject show -registry-id=N")
	fmt.Println()
	fmt.Println(" ", padRight("Источник", 18), padRight("ID", 8), padRight("ФИО", 40), "Документ")
	for _, c := range candidates {
		doc := "нет"
		if c.HasDocument {
			doc = "есть"
		}
		fmt.Println(" ", padRight(c.Source, 18), padRight(strconv.Itoa(c.ID), 8), padRight(c.FullName, 40), doc)
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
	if err := fs.Parse(args); err != nil {
		return 2
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
	fmt.Println("\nЗаписано:")
	for _, f := range written {
		fmt.Println(" ", f)
	}
	return 0
}
