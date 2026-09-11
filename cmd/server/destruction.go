package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
)

// Журнал уничтожения персональных данных (#2357).
//
// Живёт в том же бинаре, что cleanup, archive и entity, по той же причине: в рабочем
// образе есть только собранные server и seed, компилятора там нет. Веб-интерфейса у
// команды нет намеренно - доступ к ней равен доступу к консоли сервера.

const destructionHelp = `Журнал уничтожения персональных данных.

Использование:
  server destruction show   [-limit=N]                      Показать перечень уничтоженного
  server destruction replay [-apply]                        Применить уничтожения заново после восстановления

Флаги show:
  -limit  Сколько записей показать, свежие раньше. По умолчанию 50

Флаги replay:
  -apply  Снять данные, вернувшиеся из копии. Без флага - только показ

Зачем журнал. Удаление данных из работающей системы не удаляет их из ранее снятых
резервных копий: восстановление вернёт всё как было, вместе с записями, которые оператор
обязан был уничтожить. Держать копии закон позволяет при условии, что после
восстановления удаления применяются заново, - перечень уничтоженного и есть то, по чему
это делается. Персональных данных в нём нет: цель опознаётся идентификатором, номером
заявки и отпечатком документа.

replay проходит по журналу и снимает то, что вернулось из копии: обезличенную заявку
обезличивает снова, уничтоженную уничтожает, человека находит по отпечатку документа и
обезличивает. Снос организации автоматически не повторяется - он идёт только по
проверенному пакету выгрузки, и такие записи команда печатает отдельным предупреждением,
а не пропускает молча. Записи журнала проход не удаляет: следующая копия может оказаться
ещё старше, и то же самое придётся снять ещё раз; растёт только счётчик повторов.

Помнить про replay не нужно - он встроен в scripts/restore.sh отдельным шагом, между
восстановлением базы и запуском системы. Отдельно командой он запускается тогда, когда
базу восстанавливали мимо скрипта.

Примеры:
  server destruction show -limit=20
  server destruction replay
  server destruction replay -apply
`

// runDestruction разбирает подкоманду и возвращает код возврата процесса.
func runDestruction(args []string) int {
	if len(args) == 0 {
		fmt.Print(destructionHelp)
		return 2
	}
	switch args[0] {
	case "help", "-help", "--help":
		fmt.Print(destructionHelp)
		return 0
	case "show":
		return destructionShow(args[1:])
	case "replay":
		return destructionReplay(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "неизвестная подкоманда %q\n\n", args[0])
		fmt.Print(destructionHelp)
		return 2
	}
}

func destructionShow(args []string) int {
	fs := flag.NewFlagSet("destruction show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, destructionHelp) }
	limit := fs.Int("limit", 50, "сколько записей показать")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	records, err := entityarchive.ListDestructionRecords(context.Background(), db, time.Time{}, time.Time{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	if len(records) == 0 {
		fmt.Println("Журнал уничтожения пуст: ничего не уничтожалось.")
		return 0
	}
	// Свежие раньше: перечень читают, чтобы посмотреть последнее.
	fmt.Println(" ", padRight("Когда", 12), padRight("Что", 22), padRight("Действие", 12),
		padRight("Основание", 24), padLeft("Строк", 8), padLeft("Повторов", 9))
	shown := 0
	for i := len(records) - 1; i >= 0 && shown < *limit; i-- {
		r := records[i]
		what := r.EntityType
		if r.ApplicationNumber != "" {
			what = r.ApplicationNumber
		} else if r.EntityID != nil {
			what = fmt.Sprintf("%s #%d", r.EntityType, *r.EntityID)
		}
		fmt.Println(" ",
			padRight(r.CreatedAt.Format("02.01.2006"), 12),
			padRight(what, 22),
			padRight(entityarchive.DestructionActionName(r.Action), 12),
			padRight(entityarchive.DestructionBasisName(r.Basis), 24),
			padLeft(strconv.Itoa(r.Rows), 8),
			padLeft(strconv.Itoa(r.Replays), 9))
		shown++
	}
	fmt.Printf("\nВсего записей: %d\n", len(records))
	return 0
}

// destructionReplay применяет уничтожения заново после восстановления из копии.
func destructionReplay(args []string) int {
	fs := flag.NewFlagSet("destruction replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, destructionHelp) }
	apply := fs.Bool("apply", false, "снять вернувшиеся данные, а не только показать")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	paths := entityarchive.FilePaths{UploadPath: cfg.UploadPath, ArchivePath: cfg.ArchivePath}
	res, err := entityarchive.ReplayDestructions(context.Background(), db,
		services.NewAuditRecorder(db), paths, *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	printReplayResult(res, *apply)
	// Ненулевой код только на настоящих сбоях: запись, требующая рук оператора, - это
	// ожидаемый исход, и валить им шаг восстановления значило бы приучить пропускать
	// его вывод.
	if res.Failed > 0 {
		return 1
	}
	return 0
}

func printReplayResult(res entityarchive.ReplayResult, applied bool) {
	fmt.Println()
	fmt.Printf("Журнал уничтожения: записей %d\n", res.Checked)
	if res.Checked == 0 {
		fmt.Println("Уничтожений не было - применять нечего.")
		return
	}

	for _, out := range res.Outcomes {
		switch out.Status {
		case entityarchive.ReplayApplied:
			if applied {
				fmt.Printf("  снято заново: %s (строк %d, файлов %d)\n", out.Detail, out.Rows, out.Files)
			} else {
				fmt.Printf("  вернулось из копии: %s\n", out.Detail)
			}
		case entityarchive.ReplayManual:
			fmt.Printf("  ТРЕБУЕТ РУК: %s\n", out.Detail)
		case entityarchive.ReplayFailed:
			fmt.Printf("  СБОЙ: %s\n", out.Detail)
		}
	}

	fmt.Println()
	if applied {
		fmt.Printf("Применено заново: %d\n", res.Applied)
	} else {
		fmt.Printf("Вернулось из копии и будет снято: %d\n", res.Applied)
	}
	fmt.Printf("Не возвращалось:  %d\n", res.Gone)
	fmt.Printf("Требует рук:      %d\n", res.Manual)
	fmt.Printf("Сбоев:            %d\n", res.Failed)
	if !applied && res.Applied > 0 {
		fmt.Println("\nНичего не изменено: добавьте -apply.")
	}
}
