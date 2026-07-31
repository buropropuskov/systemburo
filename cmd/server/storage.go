package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	fmt.Printf("%-34s %12s %12s\n", "Таблица", "Записей", "Размер")
	for _, t := range r.Tables {
		fmt.Printf("%-34s %12d %12s\n", t.Name, t.Rows, humanBytes(t.Bytes))
	}
	if r.OthersBytes > 0 {
		fmt.Printf("%-34s %12s %12s\n", "прочие таблицы", "", humanBytes(r.OthersBytes))
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
	fmt.Printf("%-20s %-12s %10s %12s  %s\n", "Группа", "Старше", "Записей", "Примерно", "Что это")
	var total int64
	for _, t := range database.AllRetentionTargets {
		res, err := database.SweepRetention(ctx, db, t, database.DefaultRetentionCutoff(t, now), false)
		if err != nil {
			return err
		}
		total += res.FreedBytes
		fmt.Printf("%-20s %-12s %10d %12s  %s\n",
			res.Target, res.Cutoff.Format(time.DateOnly), res.Matched,
			humanBytes(res.FreedBytes), res.Description)
	}
	fmt.Println()
	fmt.Printf("Всего можно освободить примерно %s при сроках по умолчанию.\n", humanBytes(total))
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
