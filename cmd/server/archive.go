package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/blankpath"
	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Настройка файлового архива бланков из консоли (#1615).
//
// Раскладка каталогов, пороги места и срок заморозки правятся здесь, а не в
// интерфейсе: это конфигурация хранения, её задаёт тот, кто разворачивает систему,
// а не бюро пропусков. Корень архива и так живёт в переменной окружения, и держать
// шаблон каталогов в вебе было непоследовательно - сменённый шаблон переносит дерево
// заявок целиком, то есть по последствиям близок к смене самого корня. Заодно
// захваченная учётная запись администратора перестаёт быть способом увести файлы в
// другой каталог или снять ограничение объёма.
//
// В интерфейсе остаётся наблюдение (занятое место, периоды, ошибки), выгрузка на
// рабочий компьютер и донаполнение за прошлый период - действия, которые отвечают на
// увиденное, но не меняют, где лежат данные.

const archiveHelp = `Настройка файлового архива бланков.

Использование:
  server archive show                     Показать текущие настройки
  server archive preview [-dir Ш] [-file Ш]  Показать путь, который получится
  server archive set [флаги]              Изменить настройки
  server archive on | off                 Включить или выключить выгрузку
  server archive encrypt [-apply]         Закрыть файлы, записанные до включения ключей

Флаги команды set:
  -dir Ш            Шаблон каталогов заявки, уровни через /
  -file Ш           Шаблон имени файла (расширение подставляется само)
  -quota Р          Предельный объём архива, 0 - без ограничения
  -min-free Р       Наименьший остаток свободного места на разделе
  -warn N           Порог предупреждения о заполнении, проценты
  -recheck N        Окно ночной сверки, дни
  -freeze N         Через сколько дней после окончания заявки файлы замораживаются
  -zip-max Р        Потолок одной выгрузки

Размер Р задаётся числом байт либо с единицей: 512M, 2G, 750K.
Каталог архива задаётся переменной ARCHIVE_PATH и здесь не меняется.

Команда encrypt без -apply только считает и ничего не меняет. Нужна один раз, если
ключи ARCHIVE_AGE_RECIPIENT и ARCHIVE_AGE_IDENTITY задали не сразу: файлы, записанные
до них, лежат открытыми и сами не закроются.

Примеры:
  server archive preview -dir '{год}/{дата}/{дата} №{номер} {организация}'
  server archive set -freeze 60 -min-free 4G
  server archive encrypt -apply
`

func runArchive(args []string) int {
	if len(args) == 0 {
		fmt.Print(archiveHelp)
		return 2
	}

	switch args[0] {
	case "help", "-help", "--help":
		fmt.Print(archiveHelp)
		return 0
	case "show":
		return archiveShow()
	case "preview":
		return archivePreview(args[1:])
	case "set":
		return archiveSet(args[1:])
	case "on":
		return archiveSwitch(true)
	case "off":
		return archiveSwitch(false)
	case "encrypt":
		return archiveEncrypt(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Неизвестная команда %q\n\n%s", args[0], archiveHelp)
		return 2
	}
}

// archiveServices поднимает то немногое, что нужно подкоманде: настройки читаются и
// пишутся тем же сервисом, что и в приложении, поэтому проверки шаблонов и границы
// значений здесь ровно те же, а не вторая их копия.
func archiveServices() (*gorm.DB, services.SettingsService, *services.ArchivePathService, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("параметры не загружены: %w", err)
	}
	db, err := openCleanupDB()
	if err != nil {
		return nil, nil, nil, err
	}

	loc, err := time.LoadLocation(cfg.ResetTimezone)
	if err != nil {
		loc = time.UTC
	}
	return db, services.NewSettingsService(db, cfg), services.NewArchivePathService(db, loc), nil
}

func archiveShow() int {
	db, settings, paths, err := archiveServices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	current, err := settings.GetArchiveSettings(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printArchiveSettings(current)

	cfg, err := config.Load()
	if err == nil {
		fmt.Println()
		fmt.Println(padRight("Каталог архива (ARCHIVE_PATH)", 34), cfg.ArchivePath)
	}

	// Пример пути показываем сразу: настройка без него читается как набор строк, а
	// ошибку в шаблоне видно только по готовому пути.
	if preview, err := paths.Preview(ctx, current.DirTemplate, current.FileTemplate, 0); err == nil {
		fmt.Println(padRight("Пример пути", 34), preview.RelPath)
		if preview.Synthetic {
			fmt.Println(strings.Repeat(" ", 35), "(заявок ещё нет, путь собран из образцов)")
		}
	}
	_ = db
	return 0
}

func archivePreview(args []string) int {
	fs := flag.NewFlagSet("archive preview", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, archiveHelp) }
	dir := fs.String("dir", "", "шаблон каталогов")
	file := fs.String("file", "", "шаблон имени файла")
	appID := fs.Int("application", 0, "номер заявки для примера, по умолчанию последняя")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_, settings, paths, err := archiveServices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	current, err := settings.GetArchiveSettings(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	if *dir == "" {
		*dir = current.DirTemplate
	}
	if *file == "" {
		*file = current.FileTemplate
	}

	preview, err := paths.Preview(ctx, *dir, *file, *appID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	fmt.Println("Путь:", preview.RelPath)
	switch {
	case preview.Synthetic:
		fmt.Println("Заявок в базе нет, путь собран из значений-образцов.")
	case preview.ApplicationNumber != "":
		fmt.Println("Пример построен на заявке", preview.ApplicationNumber)
	}
	if len(preview.DirProblems) > 0 || len(preview.FileProblems) > 0 {
		fmt.Println()
		fmt.Println("Замечания к шаблонам:")
		for _, p := range preview.DirProblems {
			fmt.Printf("  каталог, %s: %s\n", p.Token, p.Reason)
		}
		for _, p := range preview.FileProblems {
			fmt.Printf("  имя файла, %s: %s\n", p.Token, p.Reason)
		}
		return 1
	}
	printArchiveTokens()
	return 0
}

func archiveSet(args []string) int {
	fs := flag.NewFlagSet("archive set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, archiveHelp) }
	dir := fs.String("dir", "", "шаблон каталогов заявки")
	file := fs.String("file", "", "шаблон имени файла")
	quota := fs.String("quota", "", "предельный объём архива")
	minFree := fs.String("min-free", "", "наименьший остаток свободного места")
	// -1 как «не задано» у всех числовых флагов: ноль у порогов невалиден, и
	// молча принимать его за «флаг не передан» значит проглатывать опечатку
	// оператора вместо внятного отказа.
	warn := fs.Int("warn", -1, "порог предупреждения, проценты")
	recheck := fs.Int("recheck", -1, "окно ночной сверки, дни")
	freeze := fs.Int("freeze", -1, "срок заморозки, дни")
	zipMax := fs.String("zip-max", "", "потолок одной выгрузки")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	req := models.UpdateArchiveSettingsRequest{}
	touched := false
	if *dir != "" {
		req.DirTemplate, touched = dir, true
	}
	if *file != "" {
		req.FileTemplate, touched = file, true
	}
	if *warn >= 0 {
		req.WarnPercent, touched = warn, true
	}
	if *recheck >= 0 {
		req.RecheckDays, touched = recheck, true
	}
	// Ноль у срока заморозки - осмысленное значение «замораживать сразу»; у порогов
	// ноль невалиден, но отказ о нём должен прозвучать, а не потеряться.
	if *freeze >= 0 {
		req.FreezeAfterDays, touched = freeze, true
	}

	for _, size := range []struct {
		raw  string
		name string
		dst  **int64
	}{
		{*quota, "-quota", &req.QuotaBytes},
		{*minFree, "-min-free", &req.MinFreeBytes},
		{*zipMax, "-zip-max", &req.ZipMaxBytes},
	} {
		if size.raw == "" {
			continue
		}
		v, err := parseSizeArg(size.raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка: %s: %v\n", size.name, err)
			return 2
		}
		*size.dst = &v
		touched = true
	}

	if !touched {
		fmt.Fprintln(os.Stderr, "Нечего менять: не задано ни одного флага.")
		fmt.Fprint(os.Stderr, "\n", archiveHelp)
		return 2
	}
	return applyArchiveSettings(req)
}

func archiveSwitch(on bool) int {
	return applyArchiveSettings(models.UpdateArchiveSettingsRequest{Enabled: &on})
}

// archiveEncrypt закрывает файлы архива, записанные до включения ключей.
//
// По умолчанию только считает: проход переписывает весь каталог, и оператор обязан
// сначала увидеть объём, а уже потом решить. Действие требует явного -apply.
func archiveEncrypt(args []string) int {
	fs := flag.NewFlagSet("archive encrypt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, archiveHelp) }
	apply := fs.Bool("apply", false, "выполнить, а не только посчитать")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	db, settings, paths, err := archiveServices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	crypto, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	writer, err := services.NewArchiveWriter(cfg.ArchivePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: каталог архива недоступен:", err)
		return 1
	}
	writer.SetCrypto(crypto)

	// Сторож места не подключаем: проход не наращивает архив, а заменяет файлы
	// почти того же размера, и упереться в порог ему нечем.
	svc := services.NewBlankExportService(db, nil, paths, writer, settings, nil)
	res, err := svc.EncryptExisting(context.Background(), !*apply)
	if err != nil {
		if errors.Is(err, services.ErrArchiveCryptoDisabled) {
			fmt.Fprintln(os.Stderr, "Ошибка: ключи архива не заданы.")
			fmt.Fprintln(os.Stderr, "Задайте ARCHIVE_AGE_RECIPIENT и ARCHIVE_AGE_IDENTITY, затем повторите.")
			return 1
		}
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println()
	if !*apply {
		fmt.Println("Пробный прогон, ничего не изменено. Повторите с -apply.")
		fmt.Println()
	}
	fmt.Println(padRight("Открытых файлов найдено", 34), res.Candidates)
	fmt.Println(padRight("Закрыто", 34), res.Encrypted)
	if res.Recovered > 0 {
		fmt.Println(padRight("Уже были закрыты, поправлен реестр", 34), res.Recovered)
	}
	if res.Missing > 0 {
		fmt.Println(padRight("Нет на диске", 34), res.Missing)
	}
	if res.Failed > 0 {
		fmt.Println(padRight("Не удалось", 34), res.Failed)
		fmt.Println()
		fmt.Println("Причины каждого отказа - в журнале сервера. Повторный запуск безопасен.")
		return 1
	}
	return 0
}

// applyArchiveSettings сохраняет изменение и пишет его в общий журнал. Запись
// обязательна: правка настроек из интерфейса попадала в журнал с именем
// администратора, и уход в консоль не должен превращать её в невидимую - иначе
// «кто и когда сменил раскладку» останется без ответа.
func applyArchiveSettings(req models.UpdateArchiveSettingsRequest) int {
	db, settings, paths, err := archiveServices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	ctx := context.Background()
	before, err := settings.GetArchiveSettings(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	after, err := settings.UpdateArchiveSettings(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", archiveErrorText(err))
		return 1
	}

	details := archiveConsoleDiff(before, after)
	if len(details) == 0 {
		fmt.Println("Значения не изменились.")
		return 0
	}
	details["источник"] = "консоль сервера"
	if user := os.Getenv("SUDO_USER"); user != "" {
		details["оператор"] = user
	} else if user := os.Getenv("USER"); user != "" {
		details["оператор"] = user
	}
	services.NewAuditRecorder(db).Log(ctx, nil, models.AuditEntityArchiveSettings, nil,
		models.ArchiveSettingsActionUpdated, nil, details)

	fmt.Println("Сохранено.")
	fmt.Println()
	printArchiveSettings(after)

	if preview, err := paths.Preview(ctx, after.DirTemplate, after.FileTemplate, 0); err == nil {
		fmt.Println()
		fmt.Println(padRight("Пример пути", 34), preview.RelPath)
	}
	return 0
}

func printArchiveSettings(s *models.ArchiveSettings) {
	state := "выключена"
	if s.Enabled {
		state = "включена"
	}
	quota := "без ограничения"
	if s.QuotaBytes > 0 {
		quota = humanBytes(s.QuotaBytes)
	}

	fmt.Println()
	fmt.Println(padRight("Выгрузка бланков", 34), state)
	fmt.Println(padRight("Шаблон каталогов", 34), s.DirTemplate)
	fmt.Println(padRight("Шаблон имени файла", 34), s.FileTemplate)
	fmt.Println(padRight("Предельный объём архива", 34), quota)
	fmt.Println(padRight("Остаётся свободным не меньше", 34), humanBytes(s.MinFreeBytes))
	fmt.Println(padRight("Порог предупреждения", 34), strconv.Itoa(s.WarnPercent)+" %")
	fmt.Println(padRight("Окно ночной сверки", 34), strconv.Itoa(s.RecheckDays)+" дн.")
	fmt.Println(padRight("Заморозка через", 34), strconv.Itoa(s.FreezeAfterDays)+" дн. после окончания заявки")
	fmt.Println(padRight("Потолок одной выгрузки", 34), humanBytes(s.ZipMaxBytes))
}

// printArchiveTokens показывает доступные плейсхолдеры: настраивать шаблон, держа их
// в голове, нельзя, а справка в интерфейсе после переноса недоступна.
func printArchiveTokens() {
	fmt.Println()
	fmt.Println("Плейсхолдеры:")
	for _, t := range blankpath.Tokens() {
		where := "каталог и имя файла"
		if t.Scope == blankpath.ScopeFile {
			where = "только имя файла"
		}
		fmt.Printf("  %-16s %-28s %s\n", "{"+t.Key+"}", t.Label, where)
	}
}

// archiveConsoleDiff собирает изменившиеся значения. Сравнивается состояние до и
// после записи, а не присланные флаги: заданное значение может совпасть с текущим.
func archiveConsoleDiff(before, after *models.ArchiveSettings) map[string]any {
	details := map[string]any{}
	add := func(key string, was, now any) {
		if fmt.Sprint(was) != fmt.Sprint(now) {
			details[key] = map[string]any{"old": was, "new": now}
		}
	}
	add("enabled", before.Enabled, after.Enabled)
	add("dir_template", before.DirTemplate, after.DirTemplate)
	add("file_template", before.FileTemplate, after.FileTemplate)
	add("quota_bytes", before.QuotaBytes, after.QuotaBytes)
	add("min_free_bytes", before.MinFreeBytes, after.MinFreeBytes)
	add("warn_percent", before.WarnPercent, after.WarnPercent)
	add("recheck_days", before.RecheckDays, after.RecheckDays)
	add("freeze_after_days", before.FreezeAfterDays, after.FreezeAfterDays)
	add("zip_max_bytes", before.ZipMaxBytes, after.ZipMaxBytes)
	return details
}

// parseSizeArg разбирает размер: голое число байт либо с единицей K, M, G, T.
func parseSizeArg(raw string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	if s == "" {
		return 0, errors.New("пустой размер")
	}

	mult := int64(1)
	switch s[len(s)-1] {
	case 'K':
		mult, s = 1<<10, s[:len(s)-1]
	case 'M':
		mult, s = 1<<20, s[:len(s)-1]
	case 'G':
		mult, s = 1<<30, s[:len(s)-1]
	case 'T':
		mult, s = 1<<40, s[:len(s)-1]
	}

	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("не похоже на размер: %q (примеры: 2G, 512M, 1048576)", raw)
	}
	if n < 0 {
		return 0, errors.New("размер не может быть отрицательным")
	}
	return n * mult, nil
}

// archiveErrorText разворачивает ошибку проверки настроек в человеческий текст.
// Сервис общий с веб-обработчиками и отдаёт отказ ответом echo, поэтому берём
// сообщение по типу, а не выкусываем из строки: формат чужой библиотеки может
// поменяться на следующем обновлении, и тогда оператор увидит "code=400, message=..."
// вместо причины.
func archiveErrorText(err error) string {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		if msg, ok := he.Message.(string); ok && msg != "" {
			return msg
		}
	}
	return err.Error()
}
