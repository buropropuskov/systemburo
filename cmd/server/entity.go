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

	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
)

// Консольная работа с данными по идентификатору сущности. Как cleanup и archive,
// живёт в этом же бинаре: в рабочем образе есть только собранные server и seed.
//
// Сейчас команда умеет читать граф данных цели (show), снять с него пакет (export),
// проверить уже снятый пакет (verify), развернуть проверенный пакет на текущий стенд
// (import), обратимо погасить организацию вместе с её пользователями (retire/restore) и
// необратимо затереть их персональные поля (anonymize). Физический снос добавляется
// отдельным срезом и здесь ещё не реализован. Веб-интерфейса у команды нет намеренно -
// как у cleanup и archive: доступ к операции равен доступу к консоли сервера, а не к
// учётной записи в системе.

const entityHelp = `Работа с данными по идентификатору сущности.

Использование:
  server entity show      -type=organization -id=N                      Показать граф связанных данных
  server entity export    -type=organization -id=N [-apply]             Снять пакет с графа
  server entity verify    -pkg=<путь> [-type=... -id=N]                 Проверить снятый пакет
  server entity import    -pkg=<путь> [-apply]                          Развернуть пакет на этот стенд
  server entity retire    -type=organization -id=N [-apply]             Погасить организацию и её пользователей
  server entity restore   -type=organization -id=N [-apply]             Откатить последний retire
  server entity anonymize -type=organization -id=N [-apply]             Необратимо затереть персональные поля
  server entity purge     -type=organization -id=N -pkg=<путь> [-apply] Снести данные по пакету

Общие флаги (show, export, retire, restore, anonymize, purge):
  -type   Тип сущности. Пока поддерживается только organization
  -id     Идентификатор сущности (> 0)
  -apply  Для retire/restore/anonymize/purge: выполнить изменение. Без флага - только показ

Флаги export:
  -apply       Записать пакет. Без него команда только считает
  -plaintext   Разрешить запись без шифрования, когда ключи не заданы

Флаги verify:
  -pkg    Путь к каталогу пакета (тому, что вывела export как "Каталог")
  -type   Необязательно: сверить тип сущности в манифесте. Несовпадение - отказ
  -id     Необязательно: сверить идентификатор сущности в манифесте. Несовпадение - отказ

Флаги purge:
  -pkg    Путь к каталогу пакета - обязан быть зашифрован и проверен под теми же -type/-id
  -apply  Физически удалить строки графа и файлы заявок. Без него команда только проверяет
          пакет и сверяет покрытие текущего состояния

Флаги import:
  -pkg     Путь к каталогу пакета
  -apply   Записать в базу и на диск. Без него команда только проверяет пакет,
           считает занятые идентификаторы и печатает, что случилось бы

show и export базу не меняют: только читают. show печатает, какие таблицы и сколько строк
связаны с организацией (заявки, вложения, машины, сотрудники и т.д.). Общие справочники
и посты, которыми организация лишь пользуется, в граф не входят.

export складывает тот же граф в каталог ENTITY_EXPORT_PATH: опись manifest.json, строки
таблиц в tables/*.jsonl и файлы заявок в files/. Файлы пакета закрываются age-конвертами
на ключах ARCHIVE_AGE_RECIPIENT и ARCHIVE_AGE_IDENTITY - тех же, что у файлового архива.

verify тоже ничего не меняет: ни базу, ни сам пакет. Читает манифест, сверяет отпечаток и
число строк каждого файла с описью, ищет в каталоге лишнее и сверяет колонки каждой
таблицы со схемой текущей базы. С -type/-id дополнительно проверяет, что манифест
принадлежит именно ожидаемой сущности, а не любой внутренне непротиворечивой подмене. Это
гейт, на котором держится инвариант "не сносить, пока копия не снята и не проверена" -
зелёный ответ verify обязателен перед физическим сносом данных из живой системы (команда
сноса появится отдельным срезом и передаёт сюда свои -type/-id, не заводит вторую проверку).

import сам зовёт verify по тому же пакету и отказывается разворачивать что угодно, кроме
целого пакета. Дальше проверяет, не заняты ли на этом стенде идентификаторы из таблиц
пакета - разворот идёт только на чистый стенд, слияние с существующими данными не
поддерживается. Без -apply команда только это и делает: показывает, сколько строк и
файлов уедет и есть ли конфликты, ничего не записывая. С -apply вставляет строки одной
транзакцией, кладёт файлы заявок на диск (перешифровывая ключом ЭТОЙ установки, если
DATA_ENCRYPTION_KEY задан), поправляет последовательности id и пишет след в audit_log.
Если манифест собран с полями под ключом системы (паспорта, патенты), import печатает
предупреждение проверить, тот ли на этом стенде DATA_ENCRYPTION_KEY - определить это точно
нельзя, и молчать об этом нельзя тоже.

retire без -apply показывает, что погасло бы (is_active=false у организации и её активных
пользователей), с -apply - гасит и пишет запись в audit_log. restore без -apply показывает,
что вернул бы последний retire, с -apply - включает ровно те id, которые он погасил. Без
предшествующего retire (или если он уже откачен) restore отказывает - подряд включать всё
неактивное он не умеет и не должен.

anonymize необратимо затирает ФИО, документы (паспорт, патент - вместе с их отпечатками) и
контакты сотрудников и пользователей организации, ФИО и телефон инициатора из шапки подачи
каждой заявки (initiator_name/contact_phone - там может быть указан не отправитель, а другой
человек), а также нормализованное ФИО своего сотрудника в предупреждениях о совпадении с
чёрным списком (application_blacklist_flags/overrides.element_normalized) - но только у
element_type=employee: у element_type=car это номер машины, и его команда не трогает.
Значение записи чёрного списка, с которой сравнили элемент (matched_value/matched_reason/
comment) - данные ЧУЖОГО человека, попавшего в список не этой организацией, и не
затирается. Побочный эффект: после этого подавление повторных предупреждений "всё равно
пропустить" по этой паре элемент/запись перестанет работать - для обезличенной организации
это не важно (её состав больше не подаётся). В отличие от retire связи, история, должности,
номера машин, счётчики и даты сущностей не трогаются - под затирание попадают только
перечисленные поля, и без -apply команда только показывает их список и число затронутых
строк. У anonymize нет restore: действие необратимо, откатывать нечего. Супер-администратора
организации команда не трогает (тот же запрет, что у retire) и отзывает активные
refresh-токены обезличенных пользователей - вход под прежним логином станет невозможен сразу,
а не только когда истечёт срок уже открытой сессии. Файлы, приложенные к заявкам (сканы
документов), и слепки бланков в файловом архиве (заявка.json, ARCHIVE_PATH - тот же
паспорт/патент, а также ФИО и телефон инициатора открытым текстом на момент выпуска бланка)
anonymize не трогает физически и явно предупреждает об обоих - это тоже персональные данные,
но решение по ним отдельное, за владельцем системы.

purge - необратимый физический снос: удаляет строки графа из базы и файлы заявок с диска.
Сначала сам вызывает verify по тому же пакету С УКАЗАНИЕМ -type/-id - пакет для другой
сущности или не прошедший проверку отклоняется сразу. Пакет ОБЯЗАН быть зашифрован: опись
открытого пакета сверяется сама с собой, и снос по такому пакету запрещён без исключений
(флага-обхода, в отличие от export -plaintext, здесь нет). Дальше purge сверяет счётчики
строк из описи пакета с текущим состоянием графа - если данные организации менялись после
снятия копии (что-то добавили или удалили), команда отказывает: копия устарела, и снос по
ней уничтожил бы то, чего в пакете нет. У активной организации это будет срабатывать часто и
по мелочам (открыли заявку - уже появилась отметка прочтения) - это ожидаемо, а не повод
искать обход. Рабочий порядок для необратимого сноса: entity retire -apply (гасит
организацию и её пользователей, дальше её данные не меняются) -> entity export -apply по уже
погашенной организации -> entity verify по свежему пакету -> entity purge -apply по нему же.
Без -apply purge - это всё, что он делает: проверка и подсчёт. С -apply удаление и запись в
audit_log идут одной транзакцией (строки без следа в журнале не считаются удалёнными), файлы
заявок снимаются с диска ПОСЛЕ того, как транзакция зафиксирована. Запись в audit_log
переживает сам снос - журнал не входит в граф организации.

Примеры:
  server entity show    -type=organization -id=42
  server entity export  -type=organization -id=42
  server entity export  -type=organization -id=42 -apply
  server entity verify  -pkg=/var/entity-export/organization-42-20260811-120000
  server entity verify  -pkg=/var/entity-export/organization-42-20260811-120000 -type=organization -id=42
  server entity import  -pkg=/var/entity-export/organization-42-20260811-120000
  server entity import  -pkg=/var/entity-export/organization-42-20260811-120000 -apply
  server entity retire  -type=organization -id=42 -apply
  server entity restore -type=organization -id=42 -apply
  server entity anonymize -type=organization -id=42
  server entity anonymize -type=organization -id=42 -apply
  server entity purge   -type=organization -id=42 -pkg=/var/entity-export/organization-42-20260811-120000
  server entity purge   -type=organization -id=42 -pkg=/var/entity-export/organization-42-20260811-120000 -apply
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
	case "export":
		return entityExport(args[1:])
	case "verify":
		return entityVerify(args[1:])
	case "import":
		return entityImport(args[1:])
	case "retire":
		return entityRetire(args[1:])
	case "restore":
		return entityRestore(args[1:])
	case "anonymize":
		return entityAnonymize(args[1:])
	case "purge":
		return entityPurge(args[1:])
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
	if err := entityarchive.CheckSupportedType(*entityType); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
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

// entityExport снимает пакет с графа цели. Без -apply только считает: выгрузка уносит
// весь набор персональных данных организации разом, и оператор обязан сперва увидеть,
// сколько именно данных ляжет в пакет и куда.
func entityExport(args []string) int {
	fs := flag.NewFlagSet("entity export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	entityType := fs.String("type", entityarchive.TypeOrganization, "тип сущности")
	id := fs.Int("id", 0, "идентификатор сущности")
	apply := fs.Bool("apply", false, "записать пакет, а не только посчитать")
	plaintext := fs.Bool("plaintext", false, "разрешить запись без шифрования")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := entityarchive.CheckSupportedType(*entityType); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 2
	}
	if *id <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -id больше нуля")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	if strings.TrimSpace(cfg.EntityExportPath) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: не задан ENTITY_EXPORT_PATH - каталог, куда складывать пакеты.")
		fmt.Fprintln(os.Stderr, "В пакете лежат все персональные данные организации, поэтому место хранения")
		fmt.Fprintln(os.Stderr, "задаётся явно, а не подставляется каталогом рядом с приложением.")
		return 2
	}

	crypt, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	if err := exportEncryptionGate(crypt.Enabled(), *plaintext); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	// Ключ полей нужен файлам заявок: на диске они закрыты ключом этой установки, а в
	// пакет уезжают открытыми и закрываются заново конвертом получателя.
	encKey, err := crypto.ParseHexKey(cfg.DataEncryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: неверный DATA_ENCRYPTION_KEY:", err)
		return 1
	}
	crypto.SetGlobalKey(encKey)

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	opt := entityarchive.ExportOptions{
		Root:       cfg.EntityExportPath,
		UploadPath: cfg.UploadPath,
		// Тот же приём, что у entityImport: успешная выгрузка обязана оставить след в
		// audit_log, поэтому рецептор собирается сразу с базой, а не отдельно только для
		// -apply - при пробном прогоне Export его всё равно не потребует.
		Recorder: services.NewAuditRecorder(db),
		Now:      time.Now(),
		DryRun:   !*apply,
	}
	if crypt != nil {
		opt.Crypto = crypt
	}

	res, err := entityarchive.Export(context.Background(), db, *entityType, *id, opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printEntityExport(res)
	return 0
}

// entityVerify проверяет уже снятый пакет: ничего не меняет, только читает и отвечает,
// годен ли пакет к развороту. На этом гейте держится инвариант "не сносить, пока копия не
// снята и не проверена" - зелёный код возврата здесь обязателен перед физическим сносом.
func entityVerify(args []string) int {
	fs := flag.NewFlagSet("entity verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	pkg := fs.String("pkg", "", "путь к каталогу пакета")
	entityType := fs.String("type", "", "необязательно: сверить тип сущности в манифесте")
	id := fs.Int("id", 0, "необязательно: сверить идентификатор сущности в манифесте")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*pkg) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -pkg с путём к каталогу пакета")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	crypt, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	// Тот же приём, что в entityExport: nil-интерфейс, а не типизированный nil-указатель,
	// иначе проверка на dec != nil внутри verify всегда была бы истинной.
	var dec entityarchive.Decryptor
	if crypt != nil {
		dec = crypt
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	res, err := entityarchive.Verify(context.Background(), db, *pkg, dec, *entityType, *id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printEntityVerify(res)
	if !res.OK {
		return 1
	}
	return 0
}

// entityImport разворачивает проверенный пакет на текущий стенд. Гейт проверки и гейт
// занятых идентификаторов живут внутри entityarchive.Import - здесь только разбор флагов,
// подключение ключей и печать результата.
func entityImport(args []string) int {
	fs := flag.NewFlagSet("entity import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	pkg := fs.String("pkg", "", "путь к каталогу пакета")
	apply := fs.Bool("apply", false, "записать пакет в базу и на диск, а не только проверить")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*pkg) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -pkg с путём к каталогу пакета")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	crypt, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	// Тот же приём, что в entityVerify: nil-интерфейс, а не типизированный nil-указатель.
	var dec entityarchive.Decryptor
	if crypt != nil {
		dec = crypt
	}

	// Ключ полей нужен файлам заявок: они лягут на диск закрытыми ключом ЭТОЙ установки,
	// если он задан - как обычная загрузка, а не как перенос чужого конверта.
	encKey, err := crypto.ParseHexKey(cfg.DataEncryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: неверный DATA_ENCRYPTION_KEY:", err)
		return 1
	}
	crypto.SetGlobalKey(encKey)

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	opt := entityarchive.ImportOptions{
		UploadPath: cfg.UploadPath,
		Decrypt:    dec,
		Recorder:   services.NewAuditRecorder(db),
		Apply:      *apply,
	}
	res, err := entityarchive.Import(context.Background(), db, *pkg, opt)
	printEntityImport(res)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	return 0
}

// exportEncryptionGate решает, можно ли писать пакет при таких ключах.
//
// Без ключей архива пакет ляжет на диск открытым - со всеми паспортами организации в
// одном каталоге. Отказ по умолчанию: на стенде без шифрования выгрузка нужна изредка и
// осознанно, а «забыли задать ключи» выглядит точно так же, как «решили без них».
func exportEncryptionGate(encryptionOn, plaintext bool) error {
	if encryptionOn || plaintext {
		return nil
	}
	return errors.New("ключи шифрования не заданы, пакет лёг бы на диск открытым.\n" +
		"Задайте ARCHIVE_AGE_RECIPIENT и ARCHIVE_AGE_IDENTITY либо повторите с -plaintext,\n" +
		"если открытый пакет здесь допустим осознанно")
}

func printEntityExport(res entityarchive.ExportResult) {
	fmt.Println()
	if res.DryRun {
		fmt.Println("Пробный прогон, ничего не записано. Повторите с -apply.")
		fmt.Println()
	}
	fmt.Printf("Пакет: %s #%d\n\n", res.Manifest.Type, res.Manifest.ID)
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	for _, t := range res.Manifest.Tables {
		fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.FormatInt(t.Rows, 10), 10))
	}

	fmt.Println()
	fmt.Println(padRight("Всего строк", 34), res.Rows)
	fmt.Println(padRight("Файлов заявок", 34), fmt.Sprintf("%d (%s)", res.Files, humanBytes(res.FileBytes)))
	if res.Manifest.Encrypted {
		fmt.Println(padRight("Шифрование", 34), "age-конверты, ключи архива")
	} else {
		fmt.Println(padRight("Шифрование", 34), "выключено, пакет лежит открытым")
	}
	if res.Manifest.FieldEncryption == "system_key" {
		fmt.Println(padRight("Поля под ключом системы", 34), "паспорта и патенты уедут шифротекстом")
	}
	fmt.Println(padRight("Каталог", 34), res.Dir)

	for _, w := range res.Warnings {
		fmt.Println()
		fmt.Println("Внимание:", w)
	}
	if res.Manifest.FieldEncryption == "system_key" {
		fmt.Println()
		fmt.Println("Развернуть пакет на другом стенде можно только с тем же DATA_ENCRYPTION_KEY:")
		fmt.Println("паспортные поля хранятся закрытыми и в пакет уезжают как есть.")
	}
}

func printEntityVerify(res entityarchive.VerifyResult) {
	fmt.Println()
	if res.Manifest.Type != "" {
		fmt.Printf("Пакет: %s #%d, версия формата %d\n\n", res.Manifest.Type, res.Manifest.ID, res.Manifest.Version)
	}

	if len(res.Files) > 0 {
		fmt.Println(" ", padRight("Файл", 40), padLeft("Строк", 8), "  Состояние")
		for _, f := range res.Files {
			rows := "-"
			if f.Rows > 0 {
				rows = strconv.FormatInt(f.Rows, 10)
			}
			fmt.Println(" ", padRight(f.Name, 40), padLeft(rows, 8), "  "+f.State)
		}
		fmt.Println()
	}

	for _, w := range res.Warnings {
		fmt.Println("Внимание:", w)
	}
	if len(res.Warnings) > 0 {
		fmt.Println()
	}
	for _, p := range res.Problems {
		fmt.Println("Ошибка:", p)
	}
	if len(res.Problems) > 0 {
		fmt.Println()
	}

	if res.OK {
		fmt.Println("Пакет годен.")
	} else {
		fmt.Println("Пакет не годен.")
	}
}

func printEntityImport(res entityarchive.ImportResult) {
	fmt.Println()
	if res.Manifest.Type != "" {
		fmt.Printf("Пакет: %s #%d\n\n", res.Manifest.Type, res.Manifest.ID)
	}

	for _, p := range res.Problems {
		fmt.Println("Ошибка проверки:", p)
	}
	if len(res.Problems) > 0 {
		fmt.Println()
	}

	// Разбивка по таблицам - как у printEntityExport. Это единственное, что оператор
	// видит перед -apply: без неё пробный прогон показывает только суммарные счётчики и
	// слеп ровно к тому, ради чего его обычно и запускают - опознать в составе пакета
	// таблицу, которой там быть не должно.
	if len(res.Manifest.Tables) > 0 {
		fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
		for _, t := range res.Manifest.Tables {
			fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.FormatInt(t.Rows, 10), 10))
		}
		fmt.Println()
	}

	if len(res.Conflicts) > 0 {
		fmt.Println("Заняты идентификаторы:")
		for _, c := range res.Conflicts {
			examples := make([]string, len(c.Examples))
			for i, id := range c.Examples {
				examples[i] = strconv.Itoa(id)
			}
			fmt.Printf("  %s: %d, например %s\n", c.Table, c.Total, strings.Join(examples, ", "))
		}
		fmt.Println()
	}

	if res.Rows > 0 || res.Files > 0 {
		fmt.Println(padRight("Строк в пакете", 34), res.Rows)
		fmt.Println(padRight("Файлов заявок", 34), res.Files)
	}
	for _, w := range res.Warnings {
		fmt.Println()
		fmt.Println("Внимание:", w)
	}

	fmt.Println()
	switch {
	case len(res.Problems) > 0 || len(res.Conflicts) > 0:
		fmt.Println("Импорт не выполнен.")
	case !res.Apply:
		fmt.Println("Пробный прогон, ничего не записано. Повторите с -apply.")
	default:
		fmt.Println("Импорт выполнен.")
	}
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

// entityAnonymize необратимо затирает персональные поля организации. Флаги общие с
// retire/restore (-type/-id/-apply) - тот же parseEntityMutationFlags.
func entityAnonymize(args []string) int {
	entityType, id, apply, code := parseEntityMutationFlags("entity anonymize", args)
	if code >= 0 {
		return code
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	res, err := entityarchive.Anonymize(context.Background(), db, services.NewAuditRecorder(db), entityType, id, nil, apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	printAnonymizeResult(res, apply)
	return 0
}

// printAnonymizeResult печатает перечень затираемых полей ПЕРЕД счётчиками строк -
// оператор обязан увидеть, что именно уйдёт под затирание, до того как решится на
// -apply, а не только сколько строк это затронет.
func printAnonymizeResult(res entityarchive.AnonymizeResult, applied bool) {
	fmt.Println()
	fmt.Printf("Обезличивание: %s #%d\n\n", res.Type, res.ID)
	fmt.Println("Действие НЕОБРАТИМО - затёртые значения не восстанавливаются, у команды нет restore.")
	fmt.Println()

	fmt.Println("Поля, которые будут затёрты:")
	for _, t := range res.Tables {
		fmt.Printf("  %s:\n", t.Table)
		for _, f := range t.Fields {
			fmt.Printf("    - %s\n", f)
		}
	}
	fmt.Println()

	if applied {
		fmt.Println("Затёрто:")
	} else {
		fmt.Println("Будет затёрто (показ, повторите с -apply):")
	}
	fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
	for _, t := range res.Tables {
		fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.Itoa(t.Rows), 10))
	}
	fmt.Println()
	fmt.Printf("Всего строк: %d\n", res.Total())

	// Молчать нельзя: без этой строки обезличивание выглядит полным, а супер-администратор
	// организации на самом деле сохраняет и ФИО, и прежний логин (тот же приём, что у
	// printRetireResult - см. комментарий там).
	if len(res.SkippedSuperAdmins) > 0 {
		fmt.Println()
		fmt.Printf("Внимание: супер-администратор организации (id %v) НЕ обезличен и сохраняет "+
			"прежний логин - anonymize намеренно не трогает учётную запись владельца системы.\n", res.SkippedSuperAdmins)
	}

	for _, w := range res.Warnings {
		fmt.Println()
		fmt.Println("Внимание:", w)
	}
}

// entityPurge физически сносит данные цели по проверенному пакету. Гейты (Verify по
// -type/-id, обязательность шифрования, сверка покрытия текущего состояния) живут внутри
// entityarchive.Purge - здесь только разбор флагов, подключение ключей и печать результата.
func entityPurge(args []string) int {
	fs := flag.NewFlagSet("entity purge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, entityHelp) }
	entityType := fs.String("type", entityarchive.TypeOrganization, "тип сущности")
	id := fs.Int("id", 0, "идентификатор сущности")
	pkg := fs.String("pkg", "", "путь к каталогу пакета")
	apply := fs.Bool("apply", false, "физически удалить данные, а не только проверить")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := entityarchive.CheckSupportedType(*entityType); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 2
	}
	if *id <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -id больше нуля")
		return 2
	}
	if strings.TrimSpace(*pkg) == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите -pkg с путём к каталогу пакета")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	crypt, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	// Тот же приём, что в entityVerify/entityImport: nil-интерфейс, а не типизированный
	// nil-указатель, иначе проверка на dec != nil внутри Verify всегда была бы истинной.
	var dec entityarchive.Decryptor
	if crypt != nil {
		dec = crypt
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	opt := entityarchive.PurgeOptions{
		UploadPath: cfg.UploadPath,
		Decrypt:    dec,
		Recorder:   services.NewAuditRecorder(db),
		Apply:      *apply,
	}
	res, err := entityarchive.Purge(context.Background(), db, *entityType, *id, *pkg, opt)
	printPurgeResult(res)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}
	return 0
}

func printPurgeResult(res entityarchive.PurgeResult) {
	fmt.Println()
	fmt.Printf("Пакет: %s #%d\n\n", res.Type, res.ID)
	if len(res.Tables) > 0 {
		fmt.Println(" ", padRight("Таблица", 34), padLeft("Строк", 10))
		for _, t := range res.Tables {
			fmt.Println(" ", padRight(t.Table, 34), padLeft(strconv.FormatInt(t.Rows, 10), 10))
		}
		fmt.Println()
	}
	fmt.Println(padRight("Всего строк", 34), res.TotalRows())
	if res.DetachedReportTemplates > 0 {
		// Не в счёт "Всего строк": строки физически остались в базе, отвязка - не удаление.
		fmt.Println(padRight("Отвязано общих шаблонов отчётов", 34), res.DetachedReportTemplates)
	}
	fmt.Println(padRight("Файлов заявок", 34), res.Files)
	fmt.Println(padRight("Пакет", 34), res.Package)
	if res.ManifestSHA256 != "" {
		fmt.Println(padRight("Отпечаток манифеста", 34), res.ManifestSHA256)
	}

	for _, w := range res.Warnings {
		fmt.Println()
		fmt.Println("Внимание:", w)
	}

	fmt.Println()
	switch {
	case res.Apply:
		fmt.Println("Снос выполнен. Данные удалены физически и необратимо.")
	default:
		fmt.Println("Пробный прогон, ничего не удалено. Повторите с -apply.")
	}
}
