package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
)

// Консольная работа с данными по идентификатору сущности. Как cleanup и archive,
// живёт в этом же бинаре: в рабочем образе есть только собранные server и seed.
//
// Срез 1 - только чтение: показать граф данных, связанных с целью (show).
// Срез 4 - обратимый офбординг организации: retire гасит is_active у организации и её
// пользователей, restore откатывает ровно то, что погасил последний retire. Экспорт,
// импорт и снос добавляются отдельными срезами и здесь ещё не реализованы.
// Веб-интерфейса у команды нет намеренно - как у cleanup и archive: доступ к операции
// равен доступу к консоли сервера, а не к учётной записи в системе.

const entityHelp = `Работа с данными по идентификатору сущности.

Использование:
  server entity show    -type=organization -id=N            Показать граф связанных данных
  server entity retire  -type=organization -id=N [-apply]    Погасить организацию и её пользователей
  server entity restore -type=organization -id=N [-apply]    Откатить последний retire

Флаги:
  -type   Тип сущности. Пока поддерживается только organization
  -id     Идентификатор сущности (> 0)
  -apply  Только для retire/restore: выполнить изменение. Без флага - только показ

Команда show только считает и ничего не меняет: печатает, какие таблицы и сколько
строк связаны с указанной организацией (заявки, вложения, машины, сотрудники и т.д.).
Общие справочники и посты, которыми организация лишь пользуется, в граф не входят.

retire без -apply показывает, что погасло бы (is_active=false у организации и её
активных пользователей), с -apply - гасит и пишет запись в audit_log. restore без
-apply показывает, что вернул бы последний retire, с -apply - включает ровно те id,
которые он погасил. Без предшествующего retire (или если он уже откачен) restore
отказывает - подряд включать всё неактивное он не умеет и не должен.

Примеры:
  server entity show -type=organization -id=42
  server entity retire -type=organization -id=42 -apply
  server entity restore -type=organization -id=42 -apply
`

// runEntity разбирает подкоманду и возвращает код возврата процесса.
func runEntity(args []string) int {
	if len(args) == 0 {
		fmt.Print(entityHelp)
		return 2
	}
	switch args[0] {
	case "help", "-help", "--help":
		fmt.Print(entityHelp)
		return 0
	case "show":
		return entityShow(args[1:])
	case "retire":
		return entityRetire(args[1:])
	case "restore":
		return entityRestore(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "неизвестная подкоманда %q\n\n", args[0])
		fmt.Print(entityHelp)
		return 2
	}
}

func entityShow(args []string) int {
	fs := flag.NewFlagSet("entity show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	entityType := fs.String("type", entityarchive.TypeOrganization, "тип сущности")
	id := fs.Int("id", 0, "идентификатор сущности")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -id больше нуля")
		return 2
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	graph, err := entityarchive.Collect(context.Background(), db, *entityType, *id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printEntityGraph(graph)
	return 0
}

func printEntityGraph(g entityarchive.Graph) {
	fmt.Println()
	fmt.Printf("Граф данных: %s #%d\n\n", g.Type, g.ID)
	if len(g.Tables) == 0 {
		fmt.Println("Связанных записей не найдено.")
		return
	}
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	for _, t := range g.Tables {
		fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.FormatInt(t.Rows, 10), 10))
	}
	fmt.Println()
	fmt.Printf("Всего строк: %d\n", g.Total())
}

func entityRetire(args []string) int {
	entityType, id, apply, code := parseEntityMutationFlags("entity retire", args)
	if code >= 0 {
		return code
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	res, err := entityarchive.Retire(context.Background(), db, services.NewAuditRecorder(db), entityType, id, nil, apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printRetireResult(res, apply)
	return 0
}

func entityRestore(args []string) int {
	entityType, id, apply, code := parseEntityMutationFlags("entity restore", args)
	if code >= 0 {
		return code
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	res, err := entityarchive.Restore(context.Background(), db, services.NewAuditRecorder(db), entityType, id, nil, apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printRestoreResult(res, apply)
	return 0
}

// parseEntityMutationFlags разбирает флаги, общие для retire и restore (-type, -id,
// -apply). code >= 0 значит "вызывающий обязан вернуть этот код и остановиться" - либо
// уже отпечатана справка/ошибка, либо разбор флагов сам напечатал её (flag.ContinueOnError).
func parseEntityMutationFlags(name string, args []string) (entityType string, id int, apply bool, code int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	t := fs.String("type", entityarchive.TypeOrganization, "тип сущности")
	idFlag := fs.Int("id", 0, "идентификатор сущности")
	applyFlag := fs.Bool("apply", false, "выполнить изменение")
	if err := fs.Parse(args); err != nil {
		return "", 0, false, 2
	}
	if *idFlag <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -id больше нуля")
		return "", 0, false, 2
	}
	return *t, *idFlag, *applyFlag, -1
}

func printRetireResult(res entityarchive.RetireResult, applied bool) {
	fmt.Println()
	if applied {
		fmt.Printf("Погашено: %s #%d\n\n", res.Type, res.ID)
	} else {
		fmt.Printf("Будет погашено (показ, повторите с -apply): %s #%d\n\n", res.Type, res.ID)
	}
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	fmt.Println(" ", padRight("organizations", 34), padLeft(strconv.Itoa(len(res.Organizations)), 10))
	fmt.Println(" ", padRight("users", 34), padLeft(strconv.Itoa(len(res.Users)), 10))
	fmt.Println()
	fmt.Printf("Всего строк: %d\n", res.Total())
	// Молчать нельзя: без этой строки офбординг выглядит полным, а супер-админ
	// организации на самом деле остался с рабочей учётной записью.
	if len(res.SkippedSuperAdmins) > 0 {
		fmt.Println()
		fmt.Printf("Внимание: супер-администратор организации (id %v) НЕ погашен и остаётся "+
			"активным - retire намеренно не трогает учётную запись владельца системы.\n", res.SkippedSuperAdmins)
	}
	if applied {
		fmt.Println("Откат - server entity restore с теми же -type/-id.")
	}
}

func printRestoreResult(res entityarchive.RestoreResult, applied bool) {
	fmt.Println()
	if applied {
		fmt.Printf("Восстановлено: %s #%d\n\n", res.Type, res.ID)
	} else {
		fmt.Printf("Будет восстановлено (показ, повторите с -apply): %s #%d\n\n", res.Type, res.ID)
	}
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	fmt.Println(" ", padRight("organizations", 34), padLeft(strconv.Itoa(len(res.Organizations)), 10))
	fmt.Println(" ", padRight("users", 34), padLeft(strconv.Itoa(len(res.Users)), 10))
	fmt.Println()
	fmt.Printf("Всего строк: %d\n", res.Total())
}
