package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/fakedata"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// Наливка вымышленных данных на проверочный стенд (#1682). Кнопки в интерфейсе нет
// намеренно: команда создаёт тысячи записей и принадлежит тому, кто обслуживает стенд.
//
// Без флага -apply команда ничего не пишет, а только показывает, что создалось бы.
// Порядок работы: посмотреть план, потом повторить с -apply.

const fakeHelp = `Наполнение проверочного стенда вымышленными данными.

Использование:
  server fake [флаги]

Флаги:
  -profile     Объём наливки: small, medium, large. По умолчанию: medium
  -orgs        Организаций, переопределяет профиль
  -companies   Компаний
  -users       Пользователей
  -employees   Сотрудников
  -cars        Машин
  -applications
               Заявок
  -blacklists  Записей чёрных списков
  -days-back   На сколько суток назад растянуть даты заявок и проходов
  -user-pass   Пароль создаваемых пользователей. По умолчанию -- пароль,
               проходящий почти любую политику; печатается в сводке.
               Вывод команды содержит его открытым текстом: не сохраняйте его
               в тикетах и постоянных журналах
  -seed        Источник случайности. Тот же seed даёт ту же партию
  -label       Метка партии. По умолчанию собирается из момента запуска
  -apply       Выполнить наливку. Без него команда только показывает план
  -list        Показать созданные партии
  -purge       Удалить партию целиком по её метке. Без -apply только показывает,
               что удалится
  -mark-stand  Отметить экземпляр как проверочный стенд и выйти
  -force-unmarked
               Налить на экземпляр без отметки стенда. Требует -confirm-db
  -confirm-db  Имя базы для подтверждения при -force-unmarked
  -help        Эта справка

Отметка экземпляра:
  Наливка разрешена только там, где стоит отметка проверочного стенда. Признака
  окружения в параметрах нет, поэтому отметка ставится один раз и явно:

    server fake -mark-stand

  На рабочем сервере такую отметку не ставят: вымышленные данные там не наливают.

Примеры:
  server fake                                  что создастся при профиле medium
  server fake -apply                           налить партию
  server fake -profile=large -apply            больший объём
  server fake -applications=2000 -apply        переопределить число заявок
  server fake -seed=12345 -apply               повторить ранее созданную партию
  server fake -list                            показать созданные партии
  server fake -purge=fake-20260806-101500      что удалится из партии
  server fake -purge=fake-20260806-101500 -apply   удалить партию

Что и сколько занимает места - server storage -help
`

// runFake выполняет подкоманду и возвращает код возврата процесса.
func runFake(args []string) int {
	fs := flag.NewFlagSet("fake", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fakeHelp) }
	profileName := fs.String("profile", fakedata.DefaultProfile, "объём наливки")
	orgs := fs.Int("orgs", 0, "организаций")
	companies := fs.Int("companies", 0, "компаний")
	users := fs.Int("users", 0, "пользователей")
	employees := fs.Int("employees", 0, "сотрудников")
	cars := fs.Int("cars", 0, "машин")
	applications := fs.Int("applications", 0, "заявок")
	blacklists := fs.Int("blacklists", 0, "записей чёрных списков")
	daysBack := fs.Int("days-back", 0, "на сколько суток назад растянуть даты")
	userPass := fs.String("user-pass", fakedata.DefaultUserPassword, "пароль создаваемых пользователей")
	seed := fs.Int64("seed", 0, "источник случайности")
	label := fs.String("label", "", "метка партии")
	apply := fs.Bool("apply", false, "выполнить наливку")
	list := fs.Bool("list", false, "показать созданные партии")
	purge := fs.String("purge", "", "удалить партию по метке")
	markStand := fs.Bool("mark-stand", false, "отметить экземпляр как стенд")
	forceUnmarked := fs.Bool("force-unmarked", false, "налить на экземпляр без отметки")
	confirmDB := fs.String("confirm-db", "", "имя базы для подтверждения")
	help := fs.Bool("help", false, "справка")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Print(fakeHelp)
		return 0
	}

	profile, err := fakedata.ProfileByName(*profileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 2
	}
	profile = profile.Apply(fakedata.Overrides{
		Organizations: *orgs,
		Companies:     *companies,
		Users:         *users,
		Employees:     *employees,
		Cars:          *cars,
		Applications:  *applications,
		Blacklists:    *blacklists,
		DaysBack:      *daysBack,
	})

	db, dsn, err := openFakeDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	ctx := context.Background()

	if *markStand {
		if err := fakedata.MarkStand(ctx, db); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 1
		}
		fmt.Printf("Экземпляр отмечен как проверочный стенд (база %q).\n", fakedata.DatabaseName(dsn))
		return 0
	}

	if *list {
		return printFakeBatches(ctx, db)
	}

	if *purge != "" {
		res, err := fakedata.PurgeBatch(ctx, db, *purge, *apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка:", err)
			return 1
		}
		printFakePurge(res, *apply)
		return 0
	}

	if err := fakedata.EnsureStand(ctx, db, dsn, fakedata.GuardOptions{
		ForceUnmarked: *forceUnmarked,
		ConfirmDB:     *confirmDB,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	plan := fakedata.Plan(profile)
	if !*apply {
		printFakePlan(profile, plan)
		return 0
	}

	// Наливка зовёт боевые сервисы, а те пишут в журнал каждую созданную запись. На
	// профиле large это десятки тысяч строк INFO, в которых тонет собственная сводка
	// команды. Оставляем только предупреждения и ошибки: их человек читать обязан.
	slog.SetLogLoggerLevel(slog.LevelWarn)

	if *seed == 0 {
		*seed = time.Now().UTC().UnixNano()
	}

	// Политика паролей стенда читается здесь, а не внутри fakedata: пакет наливки не
	// тянет зависимость на internal/config, а SettingsService нужен свой *config.Config
	// (см. NewSettingsService) -- то же второе чтение параметров, что и в openFakeDB.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	settingsSvc := services.NewSettingsService(db, cfg)
	passwordPolicy := settingsSvc.GetPasswordPolicy()

	consent, err := fakeConsentStamp(ctx, db, settingsSvc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	batch, err := fakedata.OpenBatch(ctx, db, *label, *seed, profile.Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	env := &fakedata.Env{
		DB: db, Batch: batch, Profile: profile, Seed: *seed,
		UserPassword:   *userPass,
		PasswordPolicy: passwordPolicy,
		Consent:        consent,
	}
	if err := fakedata.Run(ctx, env); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		fmt.Fprintf(os.Stderr, "Созданное до сбоя осталось в партии %q.\n", batch.Label())
		return 1
	}

	printFakeResult(batch, *seed, *userPass)
	return 0
}

// fakeConsentStamp -- редакция и отпечаток действующего текста согласия на обработку
// ПД, за которые расписываются создаваемые работники (#1567).
//
// Считает их тот же сервис, что и гейт на входе (PDConsentGateService.Requirement):
// сравнивает-то он именно свои значения, и вторая копия правила «отпечаток -- sha256
// текста настроек» разошлась бы с ним на первой правке. TTL кэша роли не играет --
// значение читается один раз за прогон.
func fakeConsentStamp(ctx context.Context, db *gorm.DB, settings services.SettingsService) (fakedata.ConsentStamp, error) {
	gate := services.NewPDConsentGateService(services.NewConsentService(db), settings, time.Minute)
	req, err := gate.Requirement(ctx)
	if err != nil {
		return fakedata.ConsentStamp{}, fmt.Errorf("настройки согласия на обработку ПД не прочитаны: %w", err)
	}
	return fakedata.ConsentStamp{Version: req.Version, Hash: req.Hash}, nil
}

func printFakePlan(profile fakedata.Profile, plan []fakedata.PlanItem) {
	fmt.Println()
	fmt.Printf("Профиль: %s, период дат: %d сут.\n", profile.Name, profile.DaysBack)
	fmt.Println("Ничего не создано: это предварительный показ. Повторите с флагом -apply.")
	fmt.Println()
	if len(plan) == 0 {
		fmt.Println("Шагов наполнения пока не подключено.")
		return
	}
	fmt.Println(padRight("Что создастся", 30), padLeft("Записей", 10))
	for _, item := range fakedata.PlanRecords(plan) {
		fmt.Println(padRight(item.Title, 30), padLeft(strconv.Itoa(item.Count), 10))
	}
	fmt.Println()
	total := fakedata.PlanTotal(plan)
	fmt.Printf("Всего создастся: %d %s\n", total, pluralRecords(int64(total)))

	// Отметки прохода печатаются отдельно и с оговоркой: они ничего не создают, а их
	// число приблизительное. Пока строки стояли в общей таблице и входили в «всего
	// создастся», отчёт о наливке выглядел так, будто часть работы потерялась.
	if marks := fakedata.PlanMarks(plan); len(marks) > 0 {
		fmt.Println()
		fmt.Println(padRight("Что будет отмечено", 30), padLeft("Примерно", 10))
		for _, item := range marks {
			fmt.Println(padRight(item.Title, 30), padLeft(strconv.Itoa(item.Count), 10))
		}
		fmt.Println()
		fmt.Println("Отметки прохода ставятся на уже созданных машинах и сотрудниках, новых записей не добавляют.")
		fmt.Println("Число приблизительное: отмечаются машины и люди заявок, принятых в работу, а одна запись")
		fmt.Println("реестра попадает в несколько заявок - фактически отмеченных бывает и меньше, и больше.")
	}
}

func printFakeResult(batch *fakedata.Batch, seed int64, userPassword string) {
	fmt.Println()
	fmt.Printf("Партия %q создана.\n", batch.Label())
	fmt.Printf("Источник случайности: %d. Повтор с -seed=%d даст ту же партию.\n", seed, seed)
	fmt.Printf("Пароль созданных пользователей: %s\n", userPassword)
	fmt.Println()
	counts := batch.Counts()
	if len(counts) == 0 {
		fmt.Println("Записей не создано: шагов наполнения пока не подключено.")
		return
	}
	fmt.Println(padRight("Вид записей", 30), padLeft("Создано", 10))
	for _, entity := range fakedata.SortedEntities(counts) {
		fmt.Println(padRight(fakedata.EntityTitle(entity), 30), padLeft(strconv.Itoa(counts[entity]), 10))
	}
	fmt.Println()
	total := batch.Total()
	fmt.Printf("Всего создано: %d %s\n", total, pluralRecords(int64(total)))

	marks := batch.Marks()
	if len(marks) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(padRight("Что отмечено", 30), padLeft("Отмечено", 10))
	for _, entity := range fakedata.SortedEntities(marks) {
		fmt.Println(padRight(fakedata.EntityTitle(entity), 30), padLeft(strconv.Itoa(marks[entity]), 10))
	}
	fmt.Println()
	fmt.Println("Отметки прохода поставлены на уже созданных машинах и сотрудниках: новых записей они не добавляют")
	fmt.Println("и в объём партии не входят.")
}

func printFakeBatches(ctx context.Context, db *gorm.DB) int {
	batches, err := fakedata.ListBatches(ctx, db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	fmt.Println()
	if len(batches) == 0 {
		fmt.Println("Партий вымышленных данных нет.")
		return 0
	}
	fmt.Println(padRight("Метка", 26), padRight("Создана", 20), padRight("Профиль", 10),
		padLeft("Записей", 10), padLeft("Seed", 22))
	for _, batch := range batches {
		total := 0
		for _, n := range fakedata.SummaryCounts(batch) {
			total += n
		}
		fmt.Println(padRight(batch.Label, 26), padRight(batch.CreatedAt.Format("2006-01-02 15:04"), 20),
			padRight(batch.Profile, 10), padLeft(strconv.Itoa(total), 10),
			padLeft(strconv.FormatInt(batch.Seed, 10), 22))
	}
	fmt.Println()
	return 0
}

// openFakeDB поднимает соединение и отдаёт строку подключения: имя базы из неё
// показывается в отказе и требуется для подтверждения при обходе отметки стенда.
func openFakeDB() (*gorm.DB, string, error) {
	db, err := openCleanupDB()
	if err != nil {
		return nil, "", err
	}
	// Параметры читаются второй раз только ради строки подключения: openCleanupDB её
	// не возвращает, а менять её подпись ради одной команды незачем -- на ней сидят
	// ещё cleanup, storage и archive.
	cfg, err := config.Load()
	if err != nil {
		return nil, "", fmt.Errorf("параметры не загружены: %w", err)
	}
	return db, cfg.DatabaseURL, nil
}

func printFakePurge(res fakedata.PurgeResult, applied bool) {
	fmt.Println()
	if applied {
		fmt.Printf("Партия %q удалена.\n", res.Label)
	} else {
		fmt.Printf("Ничего не удалено: это предварительный показ по партии %q. Повторите с флагом -apply.\n", res.Label)
	}
	fmt.Println()
	lines := fakedata.SortedPurgeLines(res.Lines)
	if len(lines) == 0 {
		fmt.Println("Удалять нечего: перечень партии пуст.")
		return
	}
	fmt.Println(padRight("Вид записей", 30), padLeft("Удалено", 10), padLeft("Оставлено", 12))
	for _, l := range lines {
		fmt.Println(padRight(l.Title, 30), padLeft(strconv.Itoa(l.Deleted), 10), padLeft(strconv.Itoa(l.Kept), 12))
	}
	fmt.Println()
	total := res.TotalDeleted()
	fmt.Printf("Всего удалено: %d %s\n", total, pluralRecords(int64(total)))
	if kept := res.TotalKept(); kept > 0 {
		fmt.Printf("Оставлено записей: %d -- на них ссылаются данные вне партии\n", kept)
		fmt.Println("(общие справочники, посты и шаблоны вложений заводит первая партия, а пользуются ими все).")
	}
}
