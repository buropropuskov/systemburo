package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"systemburo/internal/entityarchive"
)

// Консольная работа с данными по идентификатору сущности. Как cleanup и archive,
// живёт в этом же бинаре: в рабочем образе есть только собранные server и seed.
//
// Срез 1 - только чтение: показать граф данных, связанных с целью. Экспорт, импорт,
// архивирование и снос добавляются отдельными срезами и здесь ещё не реализованы.
// Веб-интерфейса у команды нет намеренно - как у cleanup и archive: доступ к операции
// равен доступу к консоли сервера, а не к учётной записи в системе.

const entityHelp = `Работа с данными по идентификатору сущности.

Использование:
  server entity show -type=organization -id=N   Показать граф связанных данных

Флаги show:
  -type   Тип сущности. Пока поддерживается только organization
  -id     Идентификатор сущности (> 0)

Команда show только считает и ничего не меняет: печатает, какие таблицы и сколько
строк связаны с указанной организацией (заявки, вложения, машины, сотрудники и т.д.).
Общие справочники и посты, которыми организация лишь пользуется, в граф не входят.

Пример:
  server entity show -type=organization -id=42
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
