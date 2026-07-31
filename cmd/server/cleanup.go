package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/database"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Ручная очистка накопленных данных (#1614). Отдельной кнопки в интерфейсе
// намеренно нет: удаление необратимо, а восстановление возможно только из резервной
// копии - операция принадлежит тому, кто эти копии делает.
//
// Без флага -apply команда ничего не удаляет, а только печатает, сколько записей
// попало бы под условие. Порядок работы: сначала посмотреть числа, потом повторить
// с -apply.

const cleanupHelp = `Очистка накопленных данных.

Использование:
  server cleanup [флаги]

Флаги:
  -targets   Группы через запятую, либо all. По умолчанию: tokens,notifications
  -older-than
             Срок хранения: 30d - сутки, 12m - месяцы. По умолчанию свой у каждой группы
  -apply     Выполнить удаление. Без него команда только показывает, что удалилось бы
  -help      Эта справка

Группы:
  tokens              Недействительные токены сессий, по умолчанию старше 30d
  notifications       Прочитанные уведомления, по умолчанию старше 30d
  audit               История сущностей, по умолчанию старше 36m.
                      Записи об удалении (корзина таблиц постов) и последние отметки
                      въезда и выезда сохраняются независимо от срока
  snapshots           Суточные слепки таблиц постов, по умолчанию старше 12m.
                      Снимки, снятые вручную, не трогаются
  request-aggregates  Дневные агрегаты журнала запросов, по умолчанию старше 24m

Примеры:
  server cleanup                                   что удалилось бы из мусорных групп
  server cleanup -apply                            удалить мусор
  server cleanup -targets=audit -older-than=36m    что удалилось бы из истории
  server cleanup -targets=all -older-than=24m -apply
`

// runCleanup выполняет подкоманду и возвращает код возврата процесса.
func runCleanup(args []string) int {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, cleanupHelp) }
	targets := fs.String("targets", "tokens,notifications", "группы через запятую либо all")
	olderThan := fs.String("older-than", "", "срок хранения, например 30d или 12m")
	apply := fs.Bool("apply", false, "выполнить удаление")
	help := fs.Bool("help", false, "справка")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Print(cleanupHelp)
		return 0
	}

	list, err := parseCleanupTargets(*targets)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 2
	}

	now := time.Now().UTC()
	cutoffs := make(map[database.RetentionTarget]time.Time, len(list))
	for _, t := range list {
		if *olderThan == "" {
			cutoffs[t] = database.DefaultRetentionCutoff(t, now)
			continue
		}
		cutoff, err := database.ParseRetentionAge(*olderThan, now)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 2
		}
		cutoffs[t] = cutoff
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	results := make([]database.RetentionResult, 0, len(list))
	for _, t := range list {
		res, err := database.SweepRetention(ctx, db, t, cutoffs[t], *apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 1
		}
		results = append(results, res)
	}

	printCleanupReport(results, *apply)
	return 0
}

// parseCleanupTargets разбирает список групп, сохраняя порядок вывода и отсекая повторы.
func parseCleanupTargets(s string) ([]database.RetentionTarget, error) {
	if strings.TrimSpace(s) == "all" {
		return database.AllRetentionTargets, nil
	}
	seen := make(map[database.RetentionTarget]bool)
	for _, part := range strings.Split(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		t, err := database.ParseRetentionTarget(part)
		if err != nil {
			return nil, err
		}
		seen[t] = true
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("не указана ни одна группа")
	}
	out := make([]database.RetentionTarget, 0, len(seen))
	for _, t := range database.AllRetentionTargets {
		if seen[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

func printCleanupReport(results []database.RetentionResult, applied bool) {
	fmt.Println()
	if applied {
		fmt.Println("Очистка выполнена.")
	} else {
		fmt.Println("Ничего не удалено: это предварительный показ. Повторите с флагом -apply.")
	}
	fmt.Println()
	fmt.Printf("%-20s %-12s %10s  %s\n", "Группа", "Старше", "Записей", "Что это")
	var total int64
	for _, r := range results {
		count := r.Matched
		if applied {
			count = r.Deleted
		}
		total += count
		fmt.Printf("%-20s %-12s %10d  %s\n", r.Target, r.Cutoff.Format(time.DateOnly), count, r.Description)
	}
	fmt.Println()
	if applied {
		fmt.Printf("Удалено записей: %d\n", total)
	} else {
		fmt.Printf("Попадает под удаление: %d\n", total)
	}
}

// openCleanupDB поднимает соединение без AutoMigrate и без сидов: команда обслуживает
// уже развёрнутую базу, менять её схему при уборке нечего.
func openCleanupDB() (*gorm.DB, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("параметры не загружены: %w", err)
	}
	db, err := gorm.Open(postgres.Open(database.EnsureUTCTimezone(cfg.DatabaseURL)), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("нет соединения с базой: %w", err)
	}
	return db, nil
}
