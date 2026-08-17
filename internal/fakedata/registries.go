package fakedata

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// registryCreateRetries -- сколько раз пробуем создать запись реестра заново со
// свежими случайными данными при конфликте уникальности (см. createNameRetries в
// organizations.go). Паспорт и госномер рисуются из большого, но конечного
// пространства (docs.go, plate.go) -- коллизия внутри одного прогона почти
// исключена, но UniqueEmployeeService/UniqueCarService проверяют уникальность в
// скоупе user_id, а он у всех записей реестра общий (владелец -- администратор
// стенда), и этот скоуп не сбрасывается между партиями. Повторный прогон -- то
// самое место, где редкое совпадение может случиться, и падать из-за него шаг не
// должен.
const registryCreateRetries = 5

// registriesStep наливает реестры личного кабинета -- уникальных сотрудников и
// машин (#1682, том 3). В отличие от organizationsStep/lookupsStep эти сущности
// ссылаются на организации, компании, гражданства, марки и форматы номеров, поэтому
// шаг подключается в Steps() после них и читает их id из базы, а не выдумывает свои.
type registriesStep struct{}

func (registriesStep) Name() string { return "реестры сотрудников и машин" }

func (registriesStep) Plan(p Profile) []PlanItem {
	return []PlanItem{
		{Entity: models.AuditEntityUniqueEmployee, Title: EntityTitle(models.AuditEntityUniqueEmployee), Count: p.Employees},
		{Entity: models.AuditEntityUniqueCar, Title: EntityTitle(models.AuditEntityUniqueCar), Count: p.Cars},
	}
}

func (registriesStep) Run(ctx context.Context, env *Env) error {
	// UniqueEmployeeService/UniqueCarService.Create принимают username вызывающего
	// (в отличие от OrganizationService/CompanyService, которые берут числовой id
	// напрямую) -- внутри они сами резолвят владельца через getEmployeeOwnerInfo/
	// getCarOwnerInfo по этому username, и полученный id идёт в user_id записи.
	// В отличие от organizationsStep/lookupsStep, здесь это не просто автор истории:
	// unique_employees/unique_cars.user_id держит настоящий внешний ключ на users
	// (fk_unique_employees_user), и ghost-владелец (id=0 как у actor_user_id в
	// остальных шагах) вставку не пройдёт, а не просто оставит дыру в истории.
	// Без администратора шаг обязан упасть, а не пропустить наливку: люди и машины --
	// то, ради чего команду запускают, и партия без них означала бы «готово» при пустом
	// реестре. Молчаливый пропуск заметили бы только на стенде, открыв личный кабинет.
	username, err := actorUsername(ctx, env.DB, env.ActorUserID)
	if err != nil {
		return err
	}
	if username == "" {
		return fmt.Errorf("на стенде нет ни одного администратора: реестры сотрудников и машин " +
			"держат владельца внешним ключом, и записывать их не на кого. Заведите учётную запись " +
			"администратора (make staging-seed) и повторите")
	}

	refs, err := loadRegistryRefs(ctx, env.DB)
	if err != nil {
		return err
	}

	if err := runUniqueEmployees(ctx, env, refs, username); err != nil {
		return err
	}
	return runUniqueCars(ctx, env, refs, username)
}

// actorUsername ищет username администратора стенда по его id. Пустая строка без
// ошибки означает «администратора нет», ошибка -- сбой запроса.
//
// Различать обязательно: сведённые в одно значение, они дали бы при обрыве соединения
// сообщение «на стенде нет администратора», и чинить пошли бы не то.
func actorUsername(ctx context.Context, db *gorm.DB, actorID int) (string, error) {
	if actorID == 0 {
		return "", nil
	}
	var username string
	if err := db.WithContext(ctx).Raw(`SELECT username FROM users WHERE id = ?`, actorID).Scan(&username).Error; err != nil {
		return "", fmt.Errorf("не удалось определить учётную запись администратора стенда (id %d): %w", actorID, err)
	}
	return username, nil
}

// registryRefs -- реальные id справочников, на которые опираются реестры. Читаются
// один раз перед обеими наливками (employees и cars используют одни и те же
// организации/компании).
type registryRefs struct {
	organizationIDs []int
	companyIDs      []int
	citizenships    []models.Citizenship
	markNames       []string
	formatIDs       []int
	// ruFormatID -- id формата номеров России, если он есть в справочнике.
	// PlateGenerator (plate.go) собирает номера только по ГОСТ Р 50577, поэтому
	// машине с таким номером логично ставить именно российский формат -- иначе у
	// машины будет номер российского вида с форматом, чьи ячейки его не разбирают
	// (например, у Беларуси/Казахстана другая структура, см. lookupsStep). 0, если
	// формата с CountryCode="RU" в справочнике не нашлось.
	ruFormatID int
}

// loadRegistryRefs читает справочники ЧЕРЕЗ сервисы (не прямым SQL), как остальные
// шаги наливки: организации и компании берутся только проверенные (ModerationApproved) --
// запись "на проверке" не то, на что должна ссылаться свежесозданная запись реестра.
// Гражданства/марки/форматы берутся только активные -- тот же набор, что видит в
// выпадающем списке пользователь, заполняющий форму руками.
func loadRegistryRefs(ctx context.Context, db *gorm.DB) (registryRefs, error) {
	var refs registryRefs

	orgs, err := services.NewOrganizationService(db).GetAll(ctx)
	if err != nil {
		return refs, fmt.Errorf("организации для реестров: %w", err)
	}
	for _, o := range orgs {
		if o.ModerationStatus == models.ModerationApproved {
			refs.organizationIDs = append(refs.organizationIDs, o.ID)
		}
	}
	if len(refs.organizationIDs) == 0 {
		return refs, fmt.Errorf("в базе нет ни одной проверенной организации -- реестры сотрудников и машин наполнять нечем")
	}

	companies, err := services.NewCompanyService(db).GetAll(ctx)
	if err != nil {
		return refs, fmt.Errorf("компании для реестров: %w", err)
	}
	for _, c := range companies {
		if c.ModerationStatus == models.ModerationApproved {
			refs.companyIDs = append(refs.companyIDs, c.ID)
		}
	}
	// Пустой список компаний не останавливает шаг: в отличие от организации, компания
	// у записи реестра не обязательна (NewUniqueEmployeeRequest/NewUniqueCarRequest
	// принимают её опционально) -- см. buildEmployeeRequest/buildCarRequest.

	citizenships, err := services.NewCitizenshipService(db).GetAll(ctx, false)
	if err != nil {
		return refs, fmt.Errorf("гражданства для реестров: %w", err)
	}
	refs.citizenships = citizenships
	if len(refs.citizenships) == 0 {
		return refs, fmt.Errorf("в базе нет ни одного активного гражданства -- реестр сотрудников наполнять нечем")
	}

	marks, err := services.NewMarkService(db).GetAll(ctx, false)
	if err != nil {
		return refs, fmt.Errorf("марки машин для реестров: %w", err)
	}
	for _, m := range marks {
		refs.markNames = append(refs.markNames, m.Name)
	}
	if len(refs.markNames) == 0 {
		return refs, fmt.Errorf("в базе нет ни одной активной марки машин -- реестр машин наполнять нечем")
	}

	formats, err := services.NewLicensePlateFormatService(db).GetAll(ctx, false)
	if err != nil {
		return refs, fmt.Errorf("форматы номеров для реестров: %w", err)
	}
	for _, f := range formats {
		refs.formatIDs = append(refs.formatIDs, f.Format.ID)
		if refs.ruFormatID == 0 && f.Format.CountryCode != nil && *f.Format.CountryCode == "RU" {
			refs.ruFormatID = f.Format.ID
		}
	}
	if len(refs.formatIDs) == 0 {
		return refs, fmt.Errorf("в базе нет ни одного активного формата номеров -- реестр машин наполнять нечем")
	}

	return refs, nil
}

// --- сотрудники ---

// otherPermissionOptions -- варианты поля "иное разрешение", зеркало
// availablePermissions в EmployeeForm.vue. Тот же список, что видит пользователь в
// форме -- иначе вымышленное значение оказалось бы текстом, которого форма никогда
// не предложит, и данные сразу выдали бы себя как сгенерированные.
var otherPermissionOptions = []string{
	"Иностранцы с видом на жительство (ВНЖ) или разрешением на временное проживание (РВП)",
	"Беженцы или получившие временное убежище в России",
	"Участники Госпрограммы переселения соотечественников в РФ и члены их семей",
	"Люди с временным удостоверением личности лица без гражданства, выданным в России",
	"Студенты, которые работают в образовательных организациях или хозяйственных обществах и партнёрствах, созданных этими организациями",
	"Студенты, обучающиеся очно в образовательных организациях",
	"Работники посольств и консульств",
	"Аккредитованные журналисты",
	"Специалисты аккредитованных ИТ‑компаний",
	"Специалисты иностранных компаний, которых пригласили для монтажных работ или гарантийно‑сервисного обслуживания оборудования",
	"Сотрудники представительств иностранных организаций",
	"Медики, педагоги, учёные, которые работают на территории международного медицинского кластера",
	"Педагоги и учёные, которых пригласили на работу в образовательные или научные организации",
	"Педагоги и учёные, прибывшие с деловой или гуманитарной целью в образовательные или научные организации, кроме духовных",
	"Творческие работники, учёные и педагоги, прибывшие с гостевым или деловым визитом — до 30 календарных дней",
	"Творческие работники, учёные и педагоги, прибывшие по приглашению госучреждений культуры и искусства для участия в мероприятиях — до 30 календарных дней",
}

// employeeStreams -- независимые потоки случайности для наливки сотрудников. Домены
// разведены по тому же принципу, что и в rand.go: правка одного не сдвигает
// остальные при повторе с тем же -seed.
type employeeStreams struct {
	names        *Stream
	positions    *Stream
	citizenships *Stream
	passport     *Stream
	permKind     *Stream
	permission   *Stream
	org          *Stream
	companyDraw  *Stream
	companyPick  *Stream
}

func newEmployeeStreams(seed int64) *employeeStreams {
	return &employeeStreams{
		names:        NewStream(seed, "employee-names"),
		positions:    NewStream(seed, "employee-positions"),
		citizenships: NewStream(seed, "employee-citizenship"),
		passport:     NewStream(seed, "employee-passport"),
		permKind:     NewStream(seed, "employee-permission-kind"),
		permission:   NewStream(seed, "employee-permission-text"),
		org:          NewStream(seed, "employee-org"),
		companyDraw:  NewStream(seed, "employee-company-draw"),
		companyPick:  NewStream(seed, "employee-company-pick"),
	}
}

func runUniqueEmployees(ctx context.Context, env *Env, refs registryRefs, username string) error {
	svc := services.NewUniqueEmployeeService(env.DB)
	streams := newEmployeeStreams(env.Seed)

	for i := 0; i < env.Profile.Employees; i++ {
		id, err := createFakeEmployee(ctx, svc, refs, username, streams)
		if err != nil {
			return fmt.Errorf("сотрудник %d/%d: %w", i+1, env.Profile.Employees, err)
		}
		// Регистрация в партии сразу после создания -- сбой между Create и Add
		// оставил бы сотрудника без записи в партии, и будущее удаление партии его
		// не увидит (см. тот же приём в organizations.go).
		if err := env.Batch.Add(ctx, models.AuditEntityUniqueEmployee, id); err != nil {
			return fmt.Errorf("регистрация сотрудника %d в партии: %w", id, err)
		}
	}
	return nil
}

// createFakeEmployee создаёт сотрудника, повторяя попытку со свежими случайными
// данными при конфликте уникальности паспорта (см. registryCreateRetries).
func createFakeEmployee(ctx context.Context, svc services.UniqueEmployeeService, refs registryRefs, username string, s *employeeStreams) (int, error) {
	var lastErr error
	for attempt := 0; attempt < registryCreateRetries; attempt++ {
		req := buildEmployeeRequest(refs, s)
		resp, err := svc.Create(ctx, username, req)
		if err == nil {
			return resp.ID, nil
		}
		if !isDuplicateEmployeeConflict(err) {
			return 0, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("не удалось создать сотрудника за %d попыток, паспортные данные конфликтуют: %w", registryCreateRetries, lastErr)
}

func buildEmployeeRequest(refs registryRefs, s *employeeStreams) services.NewUniqueEmployeeRequest {
	name := RandomFullName(s.names)
	position := RandomPosition(s.positions)
	citizenship := Pick(s.citizenships, refs.citizenships)
	passport := Passport(s.passport)
	orgID := Pick(s.org, refs.organizationIDs)

	req := services.NewUniqueEmployeeRequest{
		LastName:             &name.LastName,
		FirstName:            &name.FirstName,
		MiddleName:           &name.MiddleName,
		CitizenshipID:        &citizenship.ID,
		Position:             &position,
		PassportSeriesNumber: &passport,
		OrganizationID:       &orgID,
		// Согласие субъекта обязательно для новой записи реестра, а данные здесь
		// вымышленные - отметку ставим сразу, иначе наполнение стенда не проходит гейт.
		PDConsent: true,
	}

	// Компания -- не у каждого сотрудника: в отличие от организации, поле
	// необязательное и в реальном личном кабинете тоже заполняется не всегда
	// (см. loadRegistryRefs -- пустой список компаний это переживает).
	if len(refs.companyIDs) > 0 && Chance(s.companyDraw, 0.5) {
		compID := Pick(s.companyPick, refs.companyIDs)
		req.CompanyID = &compID
	}

	// Патент/иное разрешение имеют смысл только при гражданстве, требующем патент
	// (effectivePatentRequired в EmployeeForm.vue) -- при остальных гражданствах поле
	// в форме не показывается, и на стенде оно тоже остаётся пустым. Внутри
	// патентной группы -- либо номер патента, либо один из вариантов "иного
	// разрешения" (форма собирает их как взаимоисключающий выбор, см.
	// EmployeeForm.vue: patentNumber/selectedPermission).
	if citizenship.PatentRequired {
		if Chance(s.permKind, 0.5) {
			patent := Patent(s.passport)
			req.PatentNumber = &patent
		} else {
			permission := Pick(s.permission, otherPermissionOptions)
			req.OtherPermission = &permission
		}
	}

	return req
}

// isDuplicateEmployeeConflict распознаёт отказ Create по занятым паспортным данным
// (см. uniqueEmployeeService.Create в unique_employee_service.go) -- единственный
// случай, когда наливке стоит попробовать другие случайные данные вместо остановки
// шага с ошибкой.
func isDuplicateEmployeeConflict(err error) bool {
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		return false
	}
	msg, ok := httpErr.Message.(string)
	if !ok {
		return false
	}
	switch msg {
	case "Сотрудник с такими паспортными данными уже привязан к вашему аккаунту",
		"Сотрудник с такими паспортными данными уже существует в этой организации",
		"Сотрудник с такими паспортными данными уже существует в этой компании":
		return true
	default:
		return false
	}
}

// --- машины ---

// carStreams -- независимые потоки случайности для наливки машин, см. employeeStreams.
type carStreams struct {
	plates         *PlateGenerator
	marks          *Stream
	org            *Stream
	companyDraw    *Stream
	companyPick    *Stream
	formatFallback *Stream
}

func newCarStreams(seed int64) *carStreams {
	return &carStreams{
		plates:         NewPlateGenerator(seed),
		marks:          NewStream(seed, "car-marks"),
		org:            NewStream(seed, "car-org"),
		companyDraw:    NewStream(seed, "car-company-draw"),
		companyPick:    NewStream(seed, "car-company-pick"),
		formatFallback: NewStream(seed, "car-format-fallback"),
	}
}

func runUniqueCars(ctx context.Context, env *Env, refs registryRefs, username string) error {
	svc := services.NewUniqueCarService(env.DB)
	streams := newCarStreams(env.Seed)

	for i := 0; i < env.Profile.Cars; i++ {
		id, err := createFakeCar(ctx, svc, refs, username, streams)
		if err != nil {
			return fmt.Errorf("машина %d/%d: %w", i+1, env.Profile.Cars, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityUniqueCar, id); err != nil {
			return fmt.Errorf("регистрация машины %d в партии: %w", id, err)
		}
	}
	return nil
}

// createFakeCar создаёт машину, повторяя попытку со свежим номером при конфликте
// уникальности (см. registryCreateRetries). PlateGenerator.Next сам не повторяет
// номер внутри текущего прогона (см. plate.go), но не знает о номерах, записанных
// предыдущими партиями -- редкую коллизию с ними и ловит этот retry.
func createFakeCar(ctx context.Context, svc services.UniqueCarService, refs registryRefs, username string, s *carStreams) (int, error) {
	var lastErr error
	for attempt := 0; attempt < registryCreateRetries; attempt++ {
		req, err := buildCarRequest(refs, s)
		if err != nil {
			return 0, err
		}
		resp, err := svc.Create(ctx, username, req)
		if err == nil {
			return resp.ID, nil
		}
		if !isDuplicateCarConflict(err) {
			return 0, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("не удалось создать машину за %d попыток, номер конфликтует: %w", registryCreateRetries, lastErr)
}

func buildCarRequest(refs registryRefs, s *carStreams) (services.NewUniqueCarRequest, error) {
	plate, err := s.plates.Next()
	if err != nil {
		return services.NewUniqueCarRequest{}, fmt.Errorf("номер машины: %w", err)
	}
	mark := Pick(s.marks, refs.markNames)
	orgID := Pick(s.org, refs.organizationIDs)

	// Формат номеров -- российский, если он есть (см. registryRefs.ruFormatID):
	// PlateGenerator собирает только номера ГОСТ Р 50577, и любой другой формат
	// (Беларусь/Казахстан из lookupsStep) описывает другую структуру знака. Формата
	// с CountryCode="RU" не нашлось -- берём наугад из того, что есть, лишь бы id
	// был реальным: без плейсхолдера формата в справочнике машину всё равно нельзя
	// оставить без формата, реестр требует id, а не "0".
	formatID := refs.ruFormatID
	if formatID == 0 {
		formatID = Pick(s.formatFallback, refs.formatIDs)
	}

	req := services.NewUniqueCarRequest{
		Number:         plate,
		Mark:           mark,
		OrganizationID: &orgID,
		FormatID:       &formatID,
	}
	if len(refs.companyIDs) > 0 && Chance(s.companyDraw, 0.5) {
		compID := Pick(s.companyPick, refs.companyIDs)
		req.CompanyID = &compID
	}
	return req, nil
}

// isDuplicateCarConflict -- зеркало isDuplicateEmployeeConflict для машин (см.
// uniqueCarService.Create в unique_car_service.go).
func isDuplicateCarConflict(err error) bool {
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		return false
	}
	msg, ok := httpErr.Message.(string)
	if !ok {
		return false
	}
	switch msg {
	case "Автомобиль уже привязан к вашему аккаунту",
		"Автомобиль с этим номером и маркой уже существует в этой организации",
		"Автомобиль с этим номером и маркой уже существует в этой компании":
		return true
	default:
		return false
	}
}
