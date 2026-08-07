package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
  -except    Группы, которые исключить из выбранных. Например: -targets=all -except=audit
  -older-than
             Срок хранения: 30d - сутки, 12m - месяцы. По умолчанию свой у каждой группы
  -from      Начало периода, YYYY-MM-DD. Ограничивает удаление снизу: вместе с
             -older-than задаёт интервал, старше начала ничего не тронет
  -entity    Тип сущности, только для группы audit: car, employee, application и т.д.
  -table     Идентификатор таблицы поста, только для группы snapshots
  -apply     Выполнить удаление. Без него команда только показывает, что удалилось бы
  -help      Эта справка

Группы:
  tokens               Недействительные токены сессий, по умолчанию старше 30d
  notifications        Прочитанные уведомления, по умолчанию старше 30d
  unread-notifications Непрочитанные уведомления, по умолчанию старше 90d
  push-subscriptions   Подписки Web Push без единой успешной доставки, по умолчанию
                       старше 180d
  audit                История сущностей, по умолчанию старше 36m.
                       Записи об удалении (корзина таблиц постов) и последние отметки
                       въезда и выезда сохраняются независимо от срока
  snapshots            Суточные слепки таблиц постов, по умолчанию старше 12m.
                       Снимки, снятые вручную, не трогаются
  request-aggregates   Дневные агрегаты журнала запросов, по умолчанию старше 24m

Примеры:
  server cleanup                                   что удалилось бы из мусорных групп
  server cleanup -apply                            удалить мусор
  server cleanup -targets=audit -older-than=36m    что удалилось бы из истории
  server cleanup -targets=all -except=audit -apply почистить всё, кроме истории
  server cleanup -targets=all -older-than=24m -apply
  server cleanup -targets=audit -entity=car -older-than=24m       история только по машинам
  server cleanup -targets=snapshots -table=3 -apply               слепки одного поста
  server cleanup -targets=audit -from=2023-01-01 -older-than=24m  история за отрезок

Что и сколько занимает места - server storage -help
`

// runCleanup выполняет подкоманду и возвращает код возврата процесса.
func runCleanup(args []string) int {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, cleanupHelp) }
	targets := fs.String("targets", "tokens,notifications", "группы через запятую либо all")
	except := fs.String("except", "", "группы, исключаемые из выбранных")
	from := fs.String("from", "", "начало периода, YYYY-MM-DD")
	entity := fs.String("entity", "", "тип сущности для группы audit")
	tableID := fs.Int("table", 0, "идентификатор таблицы поста для группы snapshots")
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

	list, err := database.SelectRetentionTargets(*targets, *except)
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

	var fromTime *time.Time
	if *from != "" {
		parsed, err := time.Parse(time.DateOnly, *from)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка: начало периода не разобрано (ожидается YYYY-MM-DD)")
			return 2
		}
		fromTime = &parsed
	}
	if *entity != "" {
		if err := database.ValidateEntityType(*entity); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 2
		}
	}
	var tablePtr *int
	if *tableID != 0 {
		tablePtr = tableID
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	results := make([]database.RetentionResult, 0, len(list))
	for _, t := range list {
		res, err := database.SweepRetention(ctx, db, t, database.SweepOptions{
			Cutoff:     cutoffs[t],
			From:       fromTime,
			EntityType: *entity,
			TableID:    tablePtr,
			Apply:      *apply,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 1
		}
		results = append(results, res)
	}

	printCleanupReport(results, *apply)
	return 0
}

func printCleanupReport(results []database.RetentionResult, applied bool) {
	fmt.Println()
	if applied {
		fmt.Println("Очистка выполнена.")
	} else {
		fmt.Println("Ничего не удалено: это предварительный показ. Повторите с флагом -apply.")
	}
	fmt.Println()
	fmt.Println(padRight("Группа", 20), padRight("Период", 25), padLeft("Всего", 10),
		padLeft("Размер", 10), padLeft("Записей", 10), padLeft("Освободится", 12))
	var totalRows, totalBytes int64
	for _, r := range results {
		count := r.Matched
		if applied {
			count = r.Deleted
		}
		totalRows += count
		totalBytes += r.FreedBytes
		fmt.Println(padRight(string(r.Target), 20), padRight(periodLabel(r), 25),
			padLeft(strconv.FormatInt(r.TotalRows, 10), 10), padLeft(humanBytes(r.TableBytes), 10),
			padLeft(strconv.FormatInt(count, 10), 10), padLeft(humanBytes(r.FreedBytes), 12))
	}
	fmt.Println()
	for _, r := range results {
		fmt.Println(" ", padRight(string(r.Target), 20), r.Description)
	}
	fmt.Println()
	switch {
	case totalRows == 0 && applied:
		fmt.Println("Удалять было нечего: под заданные сроки не попала ни одна запись.")
	case totalRows == 0:
		fmt.Println("Удалять нечего: под заданные сроки не попала ни одна запись.")
	case applied:
		fmt.Printf("Удалено: %d %s, освобождено примерно %s\n",
			totalRows, pluralRecords(totalRows), humanBytes(totalBytes))
		fmt.Println("Место остаётся в файлах базы и переиспользуется под новые записи;")
		fmt.Println("операционной системе оно возвращается только полной перепаковкой таблицы.")
	default:
		fmt.Printf("Попадает под удаление: %d %s, примерно %s\n",
			totalRows, pluralRecords(totalRows), humanBytes(totalBytes))
	}
}

// periodLabel описывает, какой отрезок времени попал под условие: только верхняя
// граница или интервал, если задано начало периода.
func periodLabel(r database.RetentionResult) string {
	if r.From != nil {
		return r.From.Format(time.DateOnly) + " - " + r.Cutoff.Format(time.DateOnly)
	}
	return "старше " + r.Cutoff.Format(time.DateOnly)
}

// padRight и padLeft выравнивают колонки ПО РУНАМ. Форматы вида %-20s считают байты,
// поэтому кириллический заголовок занимает вдвое больше и шапка съезжает относительно
// значений - на живом стенде это было видно сразу.
func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// pluralRecords склоняет слово «запись» по числу: строку читает человек, и «1 записей»
// в отчёте выглядит небрежностью.
func pluralRecords(n int64) string {
	if n < 0 {
		n = -n
	}
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return "записей"
	case n%10 == 1:
		return "запись"
	case n%10 >= 2 && n%10 <= 4:
		return "записи"
	default:
		return "записей"
	}
}

// humanBytes переводит байты в привычные человеку единицы: оператор смотрит на вывод
// глазами и решает, стоит ли чистить, а не считает степени двойки.
func humanBytes(n int64) string {
	switch {
	case n <= 0:
		return "-"
	case n < 1024:
		return fmt.Sprintf("%d Б", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f КБ", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f МБ", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f ГБ", float64(n)/(1024*1024*1024))
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
