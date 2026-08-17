package fakedata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// appWorkHourStart, appWorkHourEnd -- диапазон часов, в которые "отправляется" заявка.
// Полночь-к-полуночи выглядела бы неправдоподобно для бюро пропусков -- реальные заявки
// подают в рабочее время.
const (
	appWorkHourStart = 8
	appWorkHourEnd   = 19
)

// appAttachmentMaxItems -- сколько максимум машин/сотрудников попадает в одно вложение
// и сколько строк ТМЦ в одно вложение items. Небольшое число: цель -- показать, что
// вложение умеет нести несколько записей, а не нагрузочное тестирование одной заявки.
const appAttachmentMaxItems = 3

// appValidityMinDays, appValidityMaxDays -- на сколько дней вперёд от даты подачи
// действует пропуск (entry_date_to - entry_date_from). Часть заявок с таким окном,
// сдвинутым в прошлое (см. shiftApplicationDates), окажется просроченной уже сегодня --
// это тоже честная картина: на живом стенде старые пропуска именно так и выглядят.
const (
	appValidityMinDays = 7
	appValidityMaxDays = 90
)

// Типы вложений заявки (Attachment.AttachmentType) -- зеркало строковых констант
// application_service.go, там они не вынесены в именованные константы.
const (
	appAttachmentCars   = "cars"
	appAttachmentPeople = "people"
	appAttachmentItems  = "items"
)

// appAttachmentTypeOrder -- фиксированный порядок трёх типов вложений. По нему
// applicationsStep.Run гарантированно проводит каждый тип через хотя бы одну заявку
// (см. buildApplication) -- без ротации при небольшом Profile.Applications и невезении
// генератора мог бы не выпасть, например, items ни разу.
var appAttachmentTypeOrder = []string{appAttachmentCars, appAttachmentPeople, appAttachmentItems}

// appTemplateSpec -- шаблон вложения (unique_attachments), которого наливке не хватает
// для подачи заявок. По одному на тип: submit-complete-application требует
// unique_attachment_id у каждого вложения, а на свежем стенде без ручной настройки
// администратора или SEED_DEMO таких шаблонов вообще нет (cmd/seed/demo.go заводит свои
// cars_demo/people_demo/items_demo, но эта команда для стенда наливки не гарантирована).
type appTemplateSpec struct {
	attachmentType string
	name           string
	displayName    string
	title          string
}

var appTemplateSpecs = []appTemplateSpec{
	{appAttachmentCars, "fake_cars_template", "Автомобили (наливка)", "АВТОМОБИЛИ"},
	{appAttachmentPeople, "fake_people_template", "Сотрудники (наливка)", "СОТРУДНИКИ"},
	{appAttachmentItems, "fake_items_template", "Имущество (наливка)", "ИМУЩЕСТВО"},
}

// appMessages -- правдоподобные тексты заявки, см. vehicleBlacklistReasons в
// blacklists.go -- один текст на всю партию сразу выдал бы вымышленные данные.
var appMessages = []string{
	"Прошу оформить пропуск для выполнения работ на территории.",
	"Заявка на въезд для планового обслуживания оборудования.",
	"Требуется допуск для проведения погрузочно-разгрузочных работ.",
	"Заявка на пропуск в рамках договора поставки.",
	"Прошу согласовать въезд для монтажных работ.",
	"Плановый визит для технического осмотра инженерных систем.",
	"Заявка на пропуск сотрудников подрядной организации.",
	"Доставка груза по накладной, требуется допуск на территорию.",
	"Прошу согласовать доступ для проведения инвентаризации.",
	"Заявка на въезд транспорта для вывоза отходов производства.",
}

// appItemNames -- правдоподобные наименования ТМЦ во вложении items.
var appItemNames = []string{
	"Ноутбук служебный", "Комплект инструмента", "Паллета с товаром",
	"Коробки с документами", "Стройматериалы", "Оборудование для монтажа",
	"Офисная мебель", "Оргтехника", "Канцелярские товары", "Образцы продукции",
	"Кабельная продукция", "Запасные части", "Спецодежда", "Измерительные приборы",
}

// applicationsStep наливает заявки с вложениями всех трёх типов (#1682, том 6) через
// applicationService.SubmitCompleteApplication -- тот же путь, которым идёт живой
// пользователь. Прямая запись в applications/attachments/cars/employees/items дала бы
// заявку без согласующих, без истории и без флагов близости к чёрному списку, которые
// расставляет сервис.
//
// Идёт последним в Steps(): использует пользователей (usersStep), реестры сотрудников и
// машин (registriesStep), чёрные списки (blacklistsStep) и таблицы постов (postsStep) --
// все они уже должны существовать в базе.
type applicationsStep struct{}

func (applicationsStep) Name() string { return "заявки с вложениями" }

func (applicationsStep) Plan(p Profile) []PlanItem {
	if p.Applications <= 0 {
		// Шаг при нулевом профиле не создаёт ничего, включая шаблоны: предпоказ обязан
		// говорить то же самое. Через флаги команды сюда не попасть (ноль означает
		// «не задано»), но Profile собирают и напрямую.
		return nil
	}
	return []PlanItem{
		// Верхняя граница -- на чистой базе шаблонов ещё нет, шаг заведёт все три
		// (см. ensureAttachmentTemplates); Plan по контракту пакета базу не читает.
		{Entity: models.AuditEntityUniqueAttachment, Title: EntityTitle(models.AuditEntityUniqueAttachment), Count: len(appTemplateSpecs)},
		{Entity: models.AuditEntityApplication, Title: EntityTitle(models.AuditEntityApplication), Count: p.Applications},
	}
}

func (applicationsStep) Run(ctx context.Context, env *Env) error {
	if env.Profile.Applications <= 0 {
		return nil
	}

	// Реестры машин/сотрудников читаются от имени администратора (см. actorUsername в
	// registries.go): их создал он, а не заявители, и без него читать нечего -- то же
	// рассуждение, что в registriesStep/blacklistsStep.
	adminUsername, err := actorUsername(ctx, env.DB, env.ActorUserID)
	if err != nil {
		return err
	}
	if adminUsername == "" {
		return fmt.Errorf("на стенде нет ни одного администратора: заявки читают реестры машин и " +
			"сотрудников от его имени. Заведите учётную запись администратора (make staging-seed) и повторите")
	}

	templates, err := ensureAttachmentTemplates(ctx, env)
	if err != nil {
		return err
	}

	applicants, err := loadApplicantPool(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return err
	}
	if len(applicants) == 0 {
		return fmt.Errorf("партия не налила ни одного активного пользователя -- заявки некому подавать " +
			"(шаг пользователей должен был выполниться раньше и оставить хотя бы одного активного, не " +
			"заблокированного и не заархивированного)")
	}

	cars, err := services.NewUniqueCarService(env.DB).GetAll(ctx, adminUsername, "")
	if err != nil {
		return fmt.Errorf("реестр машин для заявок: %w", err)
	}
	vehiclePool := appVehicleCandidates(cars)
	if len(vehiclePool) == 0 {
		return fmt.Errorf("реестр машин пуст или целиком совпал с активными записями чёрного списка -- " +
			"заявкам не из чего собрать вложение с машинами")
	}

	employees, err := services.NewUniqueEmployeeService(env.DB).GetAll(ctx, adminUsername, "")
	if err != nil {
		return fmt.Errorf("реестр сотрудников для заявок: %w", err)
	}
	employeePool := appEmployeeCandidates(employees)
	if len(employeePool) == 0 {
		return fmt.Errorf("реестр сотрудников пуст или целиком совпал с активными записями чёрного " +
			"списка -- заявкам не из чего собрать вложение с людьми")
	}

	tables, err := loadAppTargetTables(ctx, env.DB)
	if err != nil {
		return err
	}
	placeIDs, err := loadAppUnloadPlaceIDs(ctx, env.DB)
	if err != nil {
		return err
	}

	recorder := services.NewAuditRecorder(env.DB)
	appSvc := services.NewApplicationService(
		env.DB,
		services.NewPermissionService(env.DB),
		services.NewNotificationService(env.DB),
		services.NewVehicleBlacklistService(env.DB, recorder),
		services.NewPersonBlacklistService(env.DB, recorder),
		recorder,
		// Без опций: наливка не публикатор real-time сигналов и не продюсер обновления
		// таблиц/доступного охране -- у неё нет живых подключённых клиентов, которым эти
		// сигналы адресованы (см. задание -- "минимальный набор без real-time публикаторов").
	)

	streams := newApplicationStreams(env.Seed)
	now := time.Now().UTC()
	created := make([]shiftedApplication, 0, env.Profile.Applications)

	for i := 0; i < env.Profile.Applications; i++ {
		applicant := Pick(streams.applicant, applicants)
		req, sentAt := buildApplication(env.Profile, applicant, templates, vehiclePool, employeePool, tables, placeIDs, streams, now, i)

		// canOverrideOrganization=false: заявка всегда идёт от организации/компании самого
		// заявителя (drawAffiliation уже гарантировал ему хотя бы одну) -- так подаёт
		// заявку обычный пользователь, без права application.organization.override.
		resp, err := appSvc.SubmitCompleteApplication(ctx, applicant.Username, req, false)
		if err != nil {
			return fmt.Errorf("заявка %d/%d (пользователь %s): %w", i+1, env.Profile.Applications, applicant.Username, err)
		}
		// Регистрация в партии сразу после создания -- сбой между Create и Add оставил бы
		// заявку без записи в партии (см. тот же приём в других шагах).
		if err := env.Batch.Add(ctx, models.AuditEntityApplication, resp.ApplicationID); err != nil {
			return fmt.Errorf("регистрация заявки %d в партии: %w", resp.ApplicationID, err)
		}
		created = append(created, shiftedApplication{id: resp.ApplicationID, sentAt: sentAt})
	}

	return shiftApplicationDates(ctx, env, created)
}

// ensureAttachmentTemplates заводит недостающие шаблоны вложений (см. appTemplateSpecs),
// пропуская уже существующие по имени -- идемпотентно, как lookupsStep/postsStep.
func ensureAttachmentTemplates(ctx context.Context, env *Env) (map[string]int, error) {
	svc := services.NewAttachmentService(env.DB)
	existing, err := svc.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("шаблоны вложений: %w", err)
	}
	byName := make(map[string]int, len(existing))
	for _, ua := range existing {
		if ua.Name != nil {
			byName[*ua.Name] = ua.ID
		}
	}

	ids := make(map[string]int, len(appTemplateSpecs))
	for _, spec := range appTemplateSpecs {
		if id, ok := byName[spec.name]; ok {
			ids[spec.attachmentType] = id
			continue
		}
		resp, err := svc.Create(ctx, env.ActorUserID, models.CreateUniqueAttachmentRequest{
			AttachmentType: spec.attachmentType,
			Name:           spec.name,
			DisplayName:    spec.displayName,
			Title:          spec.title,
		})
		if err != nil {
			return nil, fmt.Errorf("шаблон вложения %q: %w", spec.name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityUniqueAttachment, resp.ID); err != nil {
			return nil, fmt.Errorf("регистрация шаблона вложения %d в партии: %w", resp.ID, err)
		}
		ids[spec.attachmentType] = resp.ID
	}
	return ids, nil
}

// applicantRef -- то немногое о заявителе, что нужно для подачи: логин (SubmitCompleteApplication
// принимает его, не id) и собственная организация/компания из профиля (drawAffiliation в
// users.go гарантировал хотя бы одну ненулевой).
type applicantRef struct {
	Username       string
	OrganizationID *int
	CompanyID      *int
}

// loadApplicantPool читает пользователей, которых в ЭТОЙ партии налил usersStep (по
// перечню партии, а не по имени/маске) -- задание прямо требует подавать заявки "от
// пользователей, налитых шагом пользователей", а не от произвольных учётных записей на
// стенде. Заблокированные и заархивированные исключены: под ними не проходит вход, и
// живой пользователь заявку подать не смог бы.
func loadApplicantPool(ctx context.Context, db *gorm.DB, batchID int) ([]applicantRef, error) {
	var rows []applicantRef
	err := db.WithContext(ctx).Raw(`
		SELECT u.username, u.organization_id, u.company_id
		FROM users u
		JOIN fake_batch_items fbi ON fbi.entity_id = u.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND u.is_active = true AND u.is_banned = false
		ORDER BY u.id`, batchID, models.AuditEntityUser).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("пользователи-заявители для заявок: %w", err)
	}
	return rows, nil
}

// appVehicleCandidate -- то немногое из записи реестра машин, что нужно вложению cars.
type appVehicleCandidate struct {
	number, mark string
}

// appVehicleCandidates отбирает из реестра машины с заполненными номером и маркой, ИСКЛЮЧАЯ
// те, что реестр уже пометил IsBlacklisted (совпадение с активной записью ЧС по номеру и
// марке, см. carsListSelect в unique_car_service.go -- нормализация 1:1 с
// vehicleBlacklistService.CheckByName). blacklistsStep выполняется раньше и часть записей ЧС
// намеренно похожа на записи реестра, но НЕ совпадает с ними точно (mutatePlateLetter меняет
// букву) -- отбор по этому флагу защищает не от предсказанной похожести, а от любого
// реального совпадения, включая маловероятное случайное: validateBlacklist в
// SubmitCompleteApplication отклонил бы заявку с такой машиной.
func appVehicleCandidates(cars []services.UniqueCarWithRelations) []appVehicleCandidate {
	out := make([]appVehicleCandidate, 0, len(cars))
	for _, c := range cars {
		if c.IsBlacklisted || c.Number == nil || c.Mark == nil {
			continue
		}
		number := strings.TrimSpace(*c.Number)
		mark := strings.TrimSpace(*c.Mark)
		if number == "" || mark == "" {
			continue
		}
		out = append(out, appVehicleCandidate{number: number, mark: mark})
	}
	return out
}

// appEmployeeCandidate -- то немногое из записи реестра сотрудников, что нужно вложению
// people.
type appEmployeeCandidate struct {
	lastName, firstName, middleName string
	citizenshipID                   int
	position                        string
	passport                        string
	patentNumber, otherPermission   *string
}

// appEmployeeCandidates -- зеркало appVehicleCandidates для сотрудников (IsBlacklisted --
// нормализация 1:1 с personBlacklistService.Check, см. employeesListSelect).
func appEmployeeCandidates(employees []services.UniqueEmployeeWithRelations) []appEmployeeCandidate {
	out := make([]appEmployeeCandidate, 0, len(employees))
	for _, e := range employees {
		if e.IsBlacklisted || e.LastName == nil || e.FirstName == nil || e.CitizenshipID == nil {
			continue
		}
		last := strings.TrimSpace(*e.LastName)
		first := strings.TrimSpace(*e.FirstName)
		if last == "" || first == "" {
			continue
		}
		middle := ""
		if e.MiddleName != nil {
			middle = strings.TrimSpace(*e.MiddleName)
		}
		position := ""
		if e.Position != nil {
			position = strings.TrimSpace(*e.Position)
		}
		passport := ""
		if e.PassportSeriesNumber != nil {
			passport = strings.TrimSpace(*e.PassportSeriesNumber)
		}
		out = append(out, appEmployeeCandidate{
			lastName: last, firstName: first, middleName: middle,
			citizenshipID: *e.CitizenshipID, position: position, passport: passport,
			patentNumber: e.PatentNumber, otherPermission: e.OtherPermission,
		})
	}
	return out
}

// appTableRefs -- id таблиц постов по типу, на которые заявка привязывает машины/сотрудников
// (Attachment.TargetTables, #1036).
type appTableRefs struct {
	carsTableIDs   []int
	peopleTableIDs []int
}

// loadAppTargetTables читает активные таблицы постов через SystemTableService (тот же
// список, что видит пользователь в форме) и делит по типу. permSvc не нужен для чтения
// (GetAll его не трогает), поэтому передан nil.
func loadAppTargetTables(ctx context.Context, db *gorm.DB) (appTableRefs, error) {
	tables, err := services.NewSystemTableService(db, "", 0, nil).GetAll(ctx, false)
	if err != nil {
		return appTableRefs{}, fmt.Errorf("таблицы постов для заявок: %w", err)
	}
	var refs appTableRefs
	for _, t := range tables {
		switch t.Table.TableType {
		case models.TableTypeCars:
			refs.carsTableIDs = append(refs.carsTableIDs, t.Table.ID)
		case models.TableTypePeople:
			refs.peopleTableIDs = append(refs.peopleTableIDs, t.Table.ID)
		}
	}
	if len(refs.carsTableIDs) == 0 || len(refs.peopleTableIDs) == 0 {
		return refs, fmt.Errorf("в базе нет активных таблиц постов обоих типов -- заявкам не к чему " +
			"привязать машины/сотрудников (шаг таблиц постов должен был выполниться раньше)")
	}
	return refs, nil
}

// loadAppUnloadPlaceIDs читает активные места разгрузки через UnloadPlaceService.
func loadAppUnloadPlaceIDs(ctx context.Context, db *gorm.DB) ([]int, error) {
	places, err := services.NewUnloadPlaceService(db).GetAll(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("места разгрузки для заявок: %w", err)
	}
	if len(places) == 0 {
		return nil, fmt.Errorf("в базе нет ни одного активного места разгрузки -- заявкам не к чему " +
			"привязать разгрузку")
	}
	ids := make([]int, 0, len(places))
	for _, p := range places {
		ids = append(ids, p.ID)
	}
	return ids, nil
}

// applicationStreams -- независимые потоки случайности наливки заявок, см. userStreams в
// users.go: каждый вид решения получает свой поток, чтобы правка одного не сдвигала
// остальные при повторе с тем же -seed. Потоки machine/employee/item сгруппированы по типу
// вложения -- внутри одного типа сохранён общий поток на счётчик+привязки (места/таблицы),
// а не отдельный на каждую мелочь: типов решений внутри одного вложения слишком много,
// чтобы разводить их поодиночке без явной пользы.
type applicationStreams struct {
	applicant    *Stream // кто из пула заявителей подаёт эту заявку
	daysBack     *Stream // на сколько суток в прошлое сдвигается заявка
	timeOfDay    *Stream // час/минута/секунда отправки в пределах суток
	attachMix    *Stream // сколько доп. типов вложений сверх базового и какие именно
	validityDays *Stream // на сколько дней вперёд от даты подачи действует пропуск
	message      *Stream
	responsible  *Stream // ФИО ответственного лица в шапке подачи
	phone        *Stream // контактный телефон в шапке подачи

	vehicleCount *Stream // сколько машин во вложении cars
	vehiclePick  *Stream // какие машины реестра
	vehicleAssoc *Stream // места разгрузки и таблицы поста для каждой машины

	employeeCount *Stream
	employeePick  *Stream
	employeeAssoc *Stream // таблицы поста для каждого сотрудника

	itemCount *Stream // сколько строк ТМЦ во вложении items
	itemName  *Stream
	itemQty   *Stream
	itemAssoc *Stream // места разгрузки вложения items
}

func newApplicationStreams(seed int64) *applicationStreams {
	return &applicationStreams{
		applicant:     NewStream(seed, "application-applicant"),
		daysBack:      NewStream(seed, "application-days-back"),
		timeOfDay:     NewStream(seed, "application-time-of-day"),
		attachMix:     NewStream(seed, "application-attach-mix"),
		validityDays:  NewStream(seed, "application-validity-days"),
		message:       NewStream(seed, "application-message"),
		responsible:   NewStream(seed, "application-responsible"),
		phone:         NewStream(seed, "application-phone"),
		vehicleCount:  NewStream(seed, "application-vehicle-count"),
		vehiclePick:   NewStream(seed, "application-vehicle-pick"),
		vehicleAssoc:  NewStream(seed, "application-vehicle-assoc"),
		employeeCount: NewStream(seed, "application-employee-count"),
		employeePick:  NewStream(seed, "application-employee-pick"),
		employeeAssoc: NewStream(seed, "application-employee-assoc"),
		itemCount:     NewStream(seed, "application-item-count"),
		itemName:      NewStream(seed, "application-item-name"),
		itemQty:       NewStream(seed, "application-item-qty"),
		itemAssoc:     NewStream(seed, "application-item-assoc"),
	}
}

// pickDistinct возвращает n различных элементов items в случайном порядке без повторов --
// частичная тасовка Фишера-Йетса на копии среза (исходный не трогаем: повторный вызов на
// одном словаре не должен видеть перемешанный порядок предыдущего). n больше len(items)
// обрезается до всего среза -- нужна вложению, чтобы не насажать одну и ту же машину или
// одного и того же сотрудника дважды (validateNoDuplicates в SubmitCompleteApplication
// отклонил бы такую заявку).
func pickDistinct[T any](s *Stream, items []T, n int) []T {
	if n > len(items) {
		n = len(items)
	}
	if n <= 0 {
		return nil
	}
	cp := make([]T, len(items))
	copy(cp, items)
	for i := 0; i < n; i++ {
		j := IntRange(s, i, len(cp)-1)
		cp[i], cp[j] = cp[j], cp[i]
	}
	return cp[:n]
}

// otherAppTypes возвращает два типа вложений, отличных от base, в фиксированном порядке.
func otherAppTypes(base string) []string {
	out := make([]string, 0, len(appAttachmentTypeOrder)-1)
	for _, t := range appAttachmentTypeOrder {
		if t != base {
			out = append(out, t)
		}
	}
	return out
}

// buildApplication собирает запрос на подачу одной заявки и дату, на которую её потом
// сдвинет shiftApplicationDates.
//
// Дата вычисляется ЗДЕСЬ, до вызова SubmitCompleteApplication, а не только в
// shiftApplicationDates: окно вложения (entry_date_from/entry_date_to) строится от той же
// даты, что дальше станет sending_datetime. Посчитай их порознь -- получилась бы заявка
// "отправлена год назад" с окном допуска, начинающимся сегодня, и это было бы первое, что
// бросается в глаза на стенде.
//
// seqIndex -- порядковый номер заявки в партии (0-based): по нему appAttachmentTypeOrder
// ротируется так, что база вложения перебирает cars/people/items по кругу, и при
// Profile.Applications >= 3 в партии гарантированно есть вложение каждого типа -- не
// понадеявшись на то, что все три выпадут случайно.
func buildApplication(
	profile Profile, applicant applicantRef, templates map[string]int,
	vehiclePool []appVehicleCandidate, employeePool []appEmployeeCandidate,
	tables appTableRefs, placeIDs []int, s *applicationStreams, now time.Time, seqIndex int,
) (services.CompleteApplicationRequest, time.Time) {
	daysBack := 0
	if profile.DaysBack > 0 {
		daysBack = IntRange(s.daysBack, 0, profile.DaysBack)
	}
	hour := IntRange(s.timeOfDay, appWorkHourStart, appWorkHourEnd)
	minute := IntRange(s.timeOfDay, 0, 59)
	second := IntRange(s.timeOfDay, 0, 59)
	shiftedDay := now.AddDate(0, 0, -daysBack)
	sentAt := time.Date(shiftedDay.Year(), shiftedDay.Month(), shiftedDay.Day(), hour, minute, second, 0, time.UTC)
	// Заявка «сегодня» может выпасть на рабочий час, который ещё не наступил: наливку
	// запускают и утром, и ночью. Дата подачи в будущем ломает всё, что считается от неё
	// следом -- переходы по стадиям обязаны быть позже подачи и не позже «сейчас», а при
	// подаче из будущего эти два условия несовместимы, и стадии датируются раньше подачи.
	if sentAt.After(now) {
		sentAt = now.Add(-time.Duration(IntRange(s.timeOfDay, 1, 90)) * time.Minute)
	}

	entryFrom := sentAt.Format("2006-01-02")
	validity := IntRange(s.validityDays, appValidityMinDays, appValidityMaxDays)
	entryTo := sentAt.AddDate(0, 0, validity).Format("2006-01-02")
	timeFrom, timeTo := "08:00", "18:00"

	baseType := appAttachmentTypeOrder[seqIndex%len(appAttachmentTypeOrder)]
	extraN := IntRange(s.attachMix, 0, len(appAttachmentTypeOrder)-1)
	extraTypes := pickDistinct(s.attachMix, otherAppTypes(baseType), extraN)
	types := append([]string{baseType}, extraTypes...)

	attachments := make([]services.AttachmentData, 0, len(types))
	for _, t := range types {
		switch t {
		case appAttachmentCars:
			attachments = append(attachments, buildVehicleAttachment(
				templates[appAttachmentCars], vehiclePool, tables.carsTableIDs, placeIDs, entryFrom, entryTo, timeFrom, timeTo, s))
		case appAttachmentPeople:
			attachments = append(attachments, buildEmployeeAttachment(
				templates[appAttachmentPeople], employeePool, tables.peopleTableIDs, entryFrom, entryTo, timeFrom, timeTo, s))
		case appAttachmentItems:
			attachments = append(attachments, buildItemAttachment(
				templates[appAttachmentItems], placeIDs, entryFrom, entryTo, timeFrom, timeTo, s))
		}
	}

	name := RandomFullName(s.responsible)
	responsiblePerson := strings.TrimSpace(strings.Join([]string{name.LastName, name.FirstName, name.MiddleName}, " "))
	phone := Phone(s.phone)
	message := Pick(s.message, appMessages)

	req := services.CompleteApplicationRequest{
		Message:           &message,
		OrganizationID:    applicant.OrganizationID,
		CompanyID:         applicant.CompanyID,
		ResponsiblePerson: responsiblePerson,
		ContactPhone:      phone,
		DataApproval:      true,
		Attachments:       attachments,
	}
	return req, sentAt
}

func buildVehicleAttachment(templateID int, pool []appVehicleCandidate, tableIDs, placeIDs []int, entryFrom, entryTo, timeFrom, timeTo string, s *applicationStreams) services.AttachmentData {
	n := IntRange(s.vehicleCount, 1, min(appAttachmentMaxItems, len(pool)))
	picked := pickDistinct(s.vehiclePick, pool, n)

	vehicles := make([]services.VehicleInput, 0, len(picked))
	for _, c := range picked {
		places := pickDistinct(s.vehicleAssoc, placeIDs, IntRange(s.vehicleAssoc, 1, min(2, len(placeIDs))))
		tableIDsPicked := pickDistinct(s.vehicleAssoc, tableIDs, IntRange(s.vehicleAssoc, 1, min(2, len(tableIDs))))
		vehicles = append(vehicles, services.VehicleInput{
			CarNumber:    c.number,
			CarBrand:     c.mark,
			UnloadPlaces: places,
			TargetTables: tableIDsPicked,
		})
	}

	return services.AttachmentData{
		AttachmentType:        appAttachmentCars,
		AttachmentName:        "fake_cars_template",
		AttachmentDisplayName: "Автомобили (наливка)",
		UniqueAttachmentID:    templateID,
		EntryDateFrom:         &entryFrom,
		EntryDateTo:           &entryTo,
		EntryTimeFrom:         &timeFrom,
		EntryTimeTo:           &timeTo,
		Data:                  services.AttachmentContentData{Vehicles: &vehicles},
	}
}

func buildEmployeeAttachment(templateID int, pool []appEmployeeCandidate, tableIDs []int, entryFrom, entryTo, timeFrom, timeTo string, s *applicationStreams) services.AttachmentData {
	n := IntRange(s.employeeCount, 1, min(appAttachmentMaxItems, len(pool)))
	picked := pickDistinct(s.employeePick, pool, n)

	employees := make([]services.EmployeeInput, 0, len(picked))
	for _, c := range picked {
		tableIDsPicked := pickDistinct(s.employeeAssoc, tableIDs, IntRange(s.employeeAssoc, 1, min(2, len(tableIDs))))
		var middle *string
		if c.middleName != "" {
			m := c.middleName
			middle = &m
		}
		employees = append(employees, services.EmployeeInput{
			LastName:             c.lastName,
			FirstName:            c.firstName,
			MiddleName:           middle,
			CitizenshipID:        c.citizenshipID,
			Position:             c.position,
			PassportSeriesNumber: c.passport,
			PatentNumber:         c.patentNumber,
			OtherPermission:      c.otherPermission,
			TargetTables:         tableIDsPicked,
			// Данные вымышленные, но подача проходит тот же гейт согласия, что и живая:
			// без отметки вложение с людьми не принимается.
			PDConsent: true,
		})
	}

	return services.AttachmentData{
		AttachmentType:        appAttachmentPeople,
		AttachmentName:        "fake_people_template",
		AttachmentDisplayName: "Сотрудники (наливка)",
		UniqueAttachmentID:    templateID,
		EntryDateFrom:         &entryFrom,
		EntryDateTo:           &entryTo,
		EntryTimeFrom:         &timeFrom,
		EntryTimeTo:           &timeTo,
		Data:                  services.AttachmentContentData{Employees: &employees},
	}
}

func buildItemAttachment(templateID int, placeIDs []int, entryFrom, entryTo, timeFrom, timeTo string, s *applicationStreams) services.AttachmentData {
	n := IntRange(s.itemCount, 1, appAttachmentMaxItems)
	items := make([]services.ItemInput, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, services.ItemInput{
			Name:       Pick(s.itemName, appItemNames),
			Count:      IntRange(s.itemQty, 1, 50),
			OrderIndex: i + 1,
		})
	}
	// Места разгрузки для items -- единственный источник на уровне вложения (#706, см.
	// комментарий AttachmentData.UnloadPlaces в application_service.go): в отличие от cars,
	// у items нет per-строчных мест, которые сервис мог бы собрать сам.
	places := pickDistinct(s.itemAssoc, placeIDs, IntRange(s.itemAssoc, 1, min(2, len(placeIDs))))

	return services.AttachmentData{
		AttachmentType:        appAttachmentItems,
		AttachmentName:        "fake_items_template",
		AttachmentDisplayName: "Имущество (наливка)",
		UniqueAttachmentID:    templateID,
		EntryDateFrom:         &entryFrom,
		EntryDateTo:           &entryTo,
		EntryTimeFrom:         &timeFrom,
		EntryTimeTo:           &timeTo,
		UnloadPlaces:          places,
		Data:                  services.AttachmentContentData{Items: &items},
	}
}

// shiftedApplication -- заявка, ждущая сдвига даты: id и дата/время, на которые её нужно
// переставить (та же, что уже легла в окно вложений при создании, см. buildApplication).
type shiftedApplication struct {
	id     int
	sentAt time.Time
}

// shiftApplicationDates переносит sending_datetime только что созданных заявок на дату,
// выбранную для них в buildApplication, и пересчитывает application_number под новую дату.
//
// SubmitCompleteApplication умеет писать только "сейчас": sending_datetime = time.Now(), а
// номер -- COUNT(*) заявок за СЕГОДНЯ плюс один (см. докстринг Step в generator.go про
// отсутствие блокировки и уникального индекса на номере). Без этого прохода партия легла бы
// сплошным слоем на сегодняшний день, а Profile.DaysBack остался бы обещанием без результата.
// Без пересчёта номера вместо этого получилась бы заявка "№ 20260805/003" с датой в прошлом
// году -- расхождение номера и даты, которое заметят первым.
//
// Считает NNN сам, без блокировки: подсчёт существующих (чужих) заявок на дату идёт один
// раз на дату и наращивается по ходу распределения партии -- обычный count+increment,
// потому что этот проход, в отличие от самого создания, строго последовательный и заведомо
// не гонится ни с чем (заявки уже вставлены, конкурентных писателей на этот же id нет).
//
// Трогает СТРОГО заявки apps (свою партию, по id): счётчик существующих исключает записи
// партии через fake_batch_items -- иначе заявки партии, ещё не дошедшие до переноса,
// посчитались бы "чужими" на исходной (сегодняшней) дате и задвоили бы номер.
func shiftApplicationDates(ctx context.Context, env *Env, apps []shiftedApplication) error {
	if len(apps) == 0 {
		return nil
	}
	dateCounts := make(map[string]int, len(apps))
	for _, app := range apps {
		dateKey := app.sentAt.Format("20060102")
		count, seen := dateCounts[dateKey]
		if !seen {
			var existing int64
			err := env.DB.WithContext(ctx).Raw(`
				SELECT COUNT(*) FROM applications a
				WHERE DATE(a.sending_datetime AT TIME ZONE 'UTC') = ?
				AND NOT EXISTS (
					SELECT 1 FROM fake_batch_items fbi
					WHERE fbi.batch_id = ? AND fbi.entity = ? AND fbi.entity_id = a.id
				)`, app.sentAt.Format("2006-01-02"), env.Batch.ID(), models.AuditEntityApplication,
			).Scan(&existing).Error
			if err != nil {
				return fmt.Errorf("подсчёт существующих заявок за %s перед сдвигом дат: %w", dateKey, err)
			}
			count = int(existing)
		}
		count++
		dateCounts[dateKey] = count

		newNumber := fmt.Sprintf("№ %s/%03d", dateKey, count)
		if err := env.DB.WithContext(ctx).Exec(
			`UPDATE applications SET sending_datetime = ?, application_number = ? WHERE id = ?`,
			app.sentAt, newNumber, app.id,
		).Error; err != nil {
			return fmt.Errorf("сдвиг даты заявки %d: %w", app.id, err)
		}

		if err := shiftApplicationChildren(ctx, env, app); err != nil {
			return err
		}
	}
	return nil
}

// shiftApplicationChildren переносит на дату заявки всё, что создано вместе с ней:
// вложения, машины, сотрудников, имущество, состав согласующих и записи истории.
//
// Без этого заявка выглядит противоречиво: номер и дата отправки говорят «месяц назад»,
// а модалка истории показывает «создана сегодня», и у машин в ней сегодняшняя дата
// добавления. Дочерние строки время подачи не хранят -- они берут его из значения по
// умолчанию или из time.Now() в момент вставки, поэтому переносить их приходится следом.
func shiftApplicationChildren(ctx context.Context, env *Env, app shiftedApplication) error {
	day := app.sentAt.Format("2006-01-02")
	statements := []struct {
		what  string
		query string
		args  []interface{}
	}{
		{"вложения", `UPDATE attachments SET created_at = ?, updated_at = ? WHERE application_id = ?`,
			[]interface{}{app.sentAt, app.sentAt, app.id}},
		{"машины", `UPDATE cars SET created_at = ?, updated_at = ?, date_added = ?
			WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`,
			[]interface{}{app.sentAt, app.sentAt, app.sentAt, app.id}},
		{"сотрудников", `UPDATE employees SET created_at = ?, updated_at = ?, date_created = ?
			WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`,
			[]interface{}{app.sentAt, app.sentAt, app.sentAt, app.id}},
		{"имущество", `UPDATE items SET created_at = ?, updated_at = ?, date_created = ?
			WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`,
			[]interface{}{app.sentAt, app.sentAt, day, app.id}},
		{"состав согласующих", `UPDATE application_responsible_users SET created_at = ? WHERE application_id = ?`,
			[]interface{}{app.sentAt, app.id}},
		{"читателей заявки", `UPDATE application_viewers SET created_at = ? WHERE application_id = ?`,
			[]interface{}{app.sentAt, app.id}},
		{"историю заявки", `UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND entity_id = ?`,
			[]interface{}{app.sentAt, models.AuditEntityApplication, app.id}},
		{"историю машин", `UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND entity_id IN (
			SELECT c.id FROM cars c JOIN attachments a ON a.id = c.attachment_id WHERE a.application_id = ?)`,
			[]interface{}{app.sentAt, models.AuditEntityCar, app.id}},
		{"историю сотрудников", `UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND entity_id IN (
			SELECT e.id FROM employees e JOIN attachments a ON a.id = e.attachment_id WHERE a.application_id = ?)`,
			[]interface{}{app.sentAt, models.AuditEntityEmployee, app.id}},
	}
	for _, st := range statements {
		if err := env.DB.WithContext(ctx).Exec(st.query, st.args...).Error; err != nil {
			return fmt.Errorf("сдвиг даты (%s) заявки %d: %w", st.what, app.id, err)
		}
	}
	return nil
}
