package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/database"

	"gorm.io/gorm"
)

// Обзор занятого места (#1618). Отвечает на вопросы «что растёт» и «за что браться»
// до того, как оператор возьмётся за cleanup: та команда показывает только выбранные
// группы и только под срок, а здесь видна вся база целиком.
//
// Ничего не изменяет, поэтому её безопасно давать дежурному.

const storageHelp = `Обзор занятого места в базе.

Использование:
  server storage [флаги]

Флаги:
  -top    Сколько крупнейших таблиц показать. По умолчанию 15
  -help   Эта справка

Команда только читает и ничего не удаляет.
Удаление накопленного - server cleanup -help
`

func runStorage(args []string) int {
	fs := flag.NewFlagSet("storage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, storageHelp) }
	top := fs.Int("top", 15, "сколько крупнейших таблиц показать")
	help := fs.Bool("help", false, "справка")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Print(storageHelp)
		return 0
	}
	if *top < 1 {
		fmt.Fprintln(os.Stderr, "Ошибка: -top должен быть положительным числом")
		return 2
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	report, err := database.StorageOverview(ctx, db, *top)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printStorageReport(report)

	if err := printRetentionSummary(ctx, db); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printAutoCleanupNote()
	return 0
}

func printStorageReport(r database.StorageReport) {
	fmt.Println()
	fmt.Printf("База данных занимает %s\n", humanBytes(r.DatabaseBytes))
	fmt.Println()
	fmt.Println(padRight("Таблица", 34), padLeft("Записей", 12), padLeft("Размер", 12))
	for _, t := range r.Tables {
		fmt.Println(padRight(t.Name, 34), padLeft(strconv.FormatInt(t.Rows, 10), 12),
			padLeft(humanBytes(t.Bytes), 12))
	}
	if r.OthersBytes > 0 {
		fmt.Println(padRight("прочие таблицы", 34), padLeft("", 12), padLeft(humanBytes(r.OthersBytes), 12))
	}
}

// printRetentionSummary показывает, сколько из занятого места подлежит очистке по
// срокам хранения. Без этого таблица размеров не подсказывает, за что браться:
// крупная таблица может целиком состоять из записей, удалять которые нельзя.
func printRetentionSummary(ctx context.Context, db *gorm.DB) error {
	now := time.Now().UTC()
	fmt.Println()
	fmt.Println("Из этого объёма подлежит очистке по срокам хранения:")
	fmt.Println()
	fmt.Println(padRight("Группа", 20), padRight("Старше", 12), padLeft("Записей", 10),
		padLeft("Примерно", 12), " Что это")
	var total int64
	for _, t := range database.AllRetentionTargets {
		res, err := database.SweepRetention(ctx, db, t,
			database.SweepOptions{Cutoff: database.DefaultRetentionCutoff(t, now)})
		if err != nil {
			return err
		}
		total += res.FreedBytes
		fmt.Println(padRight(string(res.Target), 20), padRight(res.Cutoff.Format(time.DateOnly), 12),
			padLeft(strconv.FormatInt(res.Matched, 10), 10), padLeft(humanBytes(res.FreedBytes), 12),
			" "+res.Description)
	}
	fmt.Println()
	if total == 0 {
		fmt.Println("При сроках по умолчанию удалять нечего: всё занятое место под данными,")
		fmt.Println("которые ещё не вышли за срок хранения.")
	} else {
		fmt.Printf("Всего можно освободить примерно %s при сроках по умолчанию.\n", humanBytes(total))
	}
	return nil
}

// printAutoCleanupNote напоминает, что часть перечисленного уберётся сама: иначе
// оператор запускает удаление руками там, где достаточно подождать сутки.
func printAutoCleanupNote() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	fmt.Println()
	fmt.Printf("Ежесуточно система удаляет сама: недействительные токены старше %d суток, "+
		"прочитанные уведомления старше %d суток.\n",
		cfg.RefreshTokenRetentionDays, cfg.ReadNotificationRetentionDays)
	fmt.Println("Остальные группы удаляются только командой: server cleanup -help")
}
