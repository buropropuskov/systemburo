package fakedata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// blacklistCreateRetries -- сколько раз пробуем создать запись заново со свежими
// случайными данными при конфликте уникальности (см. registryCreateRetries в
// registries.go). VehicleBlacklistService/PersonBlacklistService держат partial unique
// index на активную запись (номер+марка / ФИО) -- повторный прогон с тем же -seed на уже
// наполненной базе может подобрать комбинацию, которую предыдущий прогон уже занял.
const blacklistCreateRetries = 5

// vehiclePlateLetterPos -- индекс буквы серии, которую меняет похожая пара номеров
// (letter1 + 3 цифры + letter2 + letter3, см. PlateGenerator.candidate в plate.go). Код
// региона (то, что идёт дальше) не трогаем: он должен остаться существующим кодом из
// PlateRegionCodes, а не случайным числом.
const vehiclePlateLetterPos = 5

// blacklistsStep наливает записи чёрных списков машин и людей (#1682, том 4) через
// VehicleBlacklistService/PersonBlacklistService. Часть записей -- независимые (как в
// реальном списке), часть -- намеренно похожи на реальные записи реестров (unique_cars/
// unique_employees, созданные registriesStep): отличаются на одну букву в номере или
// фамилии. Смысл похожих пар -- дать стенду данные, на которых виден результат работы
// detectBlacklistSimilarity/FindSimilar (#481, application_service.go): без них
// предупреждение о возможном обходе ЧС никогда не сработало бы на вымышленных данных, а
// проверить его было бы не на чем.
type blacklistsStep struct{}

func (blacklistsStep) Name() string { return "чёрные списки машин и людей" }

func (blacklistsStep) Plan(p Profile) []PlanItem {
	return []PlanItem{
		{Entity: models.AuditEntityVehicleBlacklist, Title: EntityTitle(models.AuditEntityVehicleBlacklist), Count: p.Blacklists},
		{Entity: models.AuditEntityPersonBlacklist, Title: EntityTitle(models.AuditEntityPersonBlacklist), Count: p.Blacklists},
	}
}

func (blacklistsStep) Run(ctx context.Context, env *Env) error {
	if env.Profile.Blacklists <= 0 {
		return nil
	}

	// Записи чёрного списка держат автора (created_by_user_id) напрямую числом, но похожие
	// пары строятся от реестров, а те читаются по username владельца (см. registries.go) --
	// без администратора наливать нечем и не от кого, шаг обязан упасть, а не пропустить
	// её молча (см. то же рассуждение в registriesStep.Run).
	username, err := actorUsername(ctx, env.DB, env.ActorUserID)
	if err != nil {
		return err
	}
	if username == "" {
		return fmt.Errorf("на стенде нет ни одного администратора: записи чёрного списка держат " +
			"автора (created_by_user_id), а похожие пары читают реестры сотрудников и машин по " +
			"владельцу. Заведите учётную запись администратора (make staging-seed) и повторите")
	}

	marks, err := loadBlacklistMarks(ctx, env.DB)
	if err != nil {
		return err
	}

	if err := runVehicleBlacklist(ctx, env, username, marks); err != nil {
		return err
	}
	return runPersonBlacklist(ctx, env, username)
}

// similarPairCount -- сколько из total записей строятся похожими на запись реестра, а не
// независимыми. Половина, округлённая вверх (при total=1 -- показательная похожая пара
// важнее одной случайной записи): стенду нужно и то, на чём сработает детектор близости, и
// обычные независимые записи, как в настоящем чёрном списке.
func similarPairCount(total int) int {
	return (total + 1) / 2
}

// isBlacklistConflict распознаёт отказ Create по уже занятой активной записи (409, см.
// isUniqueViolation в vehicle_blacklist_service.go/person_blacklist_service.go) --
// единственный случай, когда наливке стоит попробовать другие случайные данные вместо
// остановки шага с ошибкой.
func isBlacklistConflict(err error) bool {
	var httpErr *echo.HTTPError
	return errors.As(err, &httpErr) && httpErr.Code == http.StatusConflict
}

// markRef -- марка с идентификатором. registryRefs (registries.go) хранит для машин
// реестра только имена -- этому шагу дополнительно нужен MarkID для
// CreateVehicleBlacklistRequest.
type markRef struct {
	id   int
	name string
}

// loadBlacklistMarks читает активные марки машин через MarkService (тот же список, что
// видит пользователь в форме).
func loadBlacklistMarks(ctx context.Context, db *gorm.DB) ([]markRef, error) {
	marks, err := services.NewMarkService(db).GetAll(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("марки машин для чёрного списка: %w", err)
	}
	if len(marks) == 0 {
		return nil, fmt.Errorf("в базе нет ни одной активной марки машин -- чёрный список машин наполнять нечем")
	}
	refs := make([]markRef, 0, len(marks))
	for _, m := range marks {
		refs = append(refs, markRef{id: m.ID, name: m.Name})
	}
	return refs, nil
}

// markIDByName ищет марку по имени без учёта регистра. Второе значение -- нашлась ли.
func markIDByName(marks []markRef, name string) (int, bool) {
	for _, m := range marks {
		if strings.EqualFold(m.name, name) {
			return m.id, true
		}
	}
	return 0, false
}

// mutateLastRune заменяет последнюю руну строки на другую из alphabet -- гарантированно
// отличную от исходной. Один символ разницы -- ровно то расстояние, которое нужно похожей
// паре ЧС: запись остаётся заметно похожей на запись реестра (проходит порог
// blacklistSimilarityThreshold, см. vehicle_blacklist_service.go), но не совпадает с ней
// точно -- иначе на неё сработал бы не FindSimilar, а точный Check.
func mutateLastRune(s *Stream, str string, alphabet []rune) string {
	runes := []rune(str)
	if len(runes) == 0 {
		return str
	}
	idx := len(runes) - 1
	for {
		repl := Pick(s, alphabet)
		if repl != runes[idx] {
			runes[idx] = repl
			return string(runes)
		}
	}
}

// mutatePlateLetter -- зеркало mutateLastRune для номеров: меняет не последний символ
// строки (это код региона), а букву на позиции vehiclePlateLetterPos.
func mutatePlateLetter(s *Stream, plate string) string {
	runes := []rune(plate)
	if len(runes) <= vehiclePlateLetterPos {
		return plate
	}
	for {
		repl := Pick(s, plateLetterRunes)
		if repl != runes[vehiclePlateLetterPos] {
			runes[vehiclePlateLetterPos] = repl
			return string(runes)
		}
	}
}

// --- машины ---

// vehicleBlacklistReasons -- правдоподобные причины блокировки машины. Список
// разнообразный намеренно: одна строка на все записи выдала бы вымышленные данные с
// первого взгляда на список.
var vehicleBlacklistReasons = []string{
	"Неоднократные нарушения пропускного режима на территории",
	"Попытка проезда по поддельному пропуску",
	"Систематическое превышение времени стоянки без согласования",
	"Отказ водителя от досмотра на КПП",
	"Использование пропуска, оформленного на другое юридическое лицо",
	"Повреждение шлагбаума при проезде",
	"Перевозка груза без сопроводительных документов",
	"Внесена по требованию службы безопасности заказчика",
	"Проезд на территорию в состоянии алкогольного опьянения",
	"Неоднократные жалобы охраны на агрессивное поведение водителя",
	"Съезд с разрешённого маршрута движения по территории",
	"Попытка въезда по номеру, не совпадающему с поданной заявкой",
}

// carCandidate -- то немногое из записи реестра машин, что нужно похожей паре: номер и
// имя марки.
type carCandidate struct {
	number string
	mark   string
}

// vehicleCandidates отбирает из реестра машины с заполненными номером и маркой, у которых
// номер достаточно длинный для mutatePlateLetter. registriesStep всегда пишет оба поля, но
// фильтр защищает от паники на будущих изменениях формата, а не от реального сегодняшнего
// случая.
func vehicleCandidates(cars []services.UniqueCarWithRelations) []carCandidate {
	out := make([]carCandidate, 0, len(cars))
	for _, c := range cars {
		if c.Number == nil || c.Mark == nil {
			continue
		}
		number := strings.TrimSpace(*c.Number)
		mark := strings.TrimSpace(*c.Mark)
		if mark == "" || len([]rune(number)) <= vehiclePlateLetterPos {
			continue
		}
		out = append(out, carCandidate{number: number, mark: mark})
	}
	return out
}

// vehicleBlacklistStreams -- независимые потоки случайности для наливки чёрного списка
// машин, см. employeeStreams/carStreams в registries.go.
type vehicleBlacklistStreams struct {
	similarPick *Stream         // какую машину реестра взять для похожей пары
	letterPick  *Stream         // на какую букву заменить серию в похожей паре
	independent *PlateGenerator // номера независимых (не похожих) записей
	markPick    *Stream         // марка независимых записей и запасной вариант похожей пары
	reason      *Stream
}

func newVehicleBlacklistStreams(seed int64) *vehicleBlacklistStreams {
	return &vehicleBlacklistStreams{
		similarPick: NewStream(seed, "blacklist-vehicle-similar-pick"),
		letterPick:  NewStream(seed, "blacklist-vehicle-letter"),
		independent: NewPlateGeneratorWithDomain(seed, "blacklist-vehicle-independent-plates"),
		markPick:    NewStream(seed, "blacklist-vehicle-mark"),
		reason:      NewStream(seed, "blacklist-vehicle-reason"),
	}
}

func runVehicleBlacklist(ctx context.Context, env *Env, username string, marks []markRef) error {
	cars, err := services.NewUniqueCarService(env.DB).GetAll(ctx, username, "")
	if err != nil {
		return fmt.Errorf("реестр машин для чёрного списка: %w", err)
	}
	candidates := vehicleCandidates(cars)
	if len(candidates) == 0 {
		return fmt.Errorf("реестр машин пуст -- нечем строить похожие пары для чёрного списка " +
			"(шаг реестров должен был выполниться раньше и оставить хотя бы одну машину)")
	}

	svc := services.NewVehicleBlacklistService(env.DB, services.NewAuditRecorder(env.DB))
	streams := newVehicleBlacklistStreams(env.Seed)
	similarCount := similarPairCount(env.Profile.Blacklists)

	for i := 0; i < env.Profile.Blacklists; i++ {
		entry, err := createFakeVehicleBlacklist(ctx, svc, i < similarCount, candidates, marks, streams, env.ActorUserID)
		if err != nil {
			return fmt.Errorf("запись чёрного списка машин %d/%d: %w", i+1, env.Profile.Blacklists, err)
		}
		// Регистрация в партии сразу после создания -- сбой между Create и Add оставил бы
		// запись без строки в партии, и будущее удаление партии её не увидит (см. тот же
		// приём в registries.go).
		if err := env.Batch.Add(ctx, models.AuditEntityVehicleBlacklist, entry.ID); err != nil {
			return fmt.Errorf("регистрация записи чёрного списка машин %d в партии: %w", entry.ID, err)
		}
	}
	return nil
}

// createFakeVehicleBlacklist создаёт запись, повторяя попытку со свежими случайными
// данными при конфликте уникальности (см. blacklistCreateRetries).
func createFakeVehicleBlacklist(ctx context.Context, svc services.VehicleBlacklistService, similar bool, candidates []carCandidate, marks []markRef, s *vehicleBlacklistStreams, actorID int) (*models.VehicleBlacklist, error) {
	var lastErr error
	for attempt := 0; attempt < blacklistCreateRetries; attempt++ {
		req, err := buildVehicleBlacklistRequest(similar, candidates, marks, s)
		if err != nil {
			return nil, err
		}
		entry, err := svc.Create(ctx, req, actorID)
		if err == nil {
			return entry, nil
		}
		if !isBlacklistConflict(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("не удалось создать запись за %d попыток, номер и марка конфликтуют: %w", blacklistCreateRetries, lastErr)
}

func buildVehicleBlacklistRequest(similar bool, candidates []carCandidate, marks []markRef, s *vehicleBlacklistStreams) (models.CreateVehicleBlacklistRequest, error) {
	reason := Pick(s.reason, vehicleBlacklistReasons)

	if similar {
		car := Pick(s.similarPick, candidates)
		number := mutatePlateLetter(s.letterPick, car.number)
		markID, ok := markIDByName(marks, car.mark)
		if !ok {
			// Марка машины реестра не нашлась среди активных марок (архивирована между
			// шагами внутри одного прогона не бывает, но без запасного варианта запись
			// осталась бы вовсе без markID). Пара при этом теряет соответствие по марке:
			// похожим остаётся только номер, а он и есть то, по чему срабатывает детектор.
			markID = Pick(s.markPick, marks).id
		}
		return models.CreateVehicleBlacklistRequest{CarNumber: number, MarkID: markID, Reason: reason}, nil
	}

	number, err := s.independent.Next()
	if err != nil {
		return models.CreateVehicleBlacklistRequest{}, fmt.Errorf("номер для независимой записи чёрного списка: %w", err)
	}
	markID := Pick(s.markPick, marks).id
	return models.CreateVehicleBlacklistRequest{CarNumber: number, MarkID: markID, Reason: reason}, nil
}

// --- люди ---

// personBlacklistReasons -- правдоподобные причины блокировки человека, см.
// vehicleBlacklistReasons.
var personBlacklistReasons = []string{
	"Нарушение пропускного режима: проход по чужому удостоверению",
	"Утрата личного пропуска при невыясненных обстоятельствах",
	"Попытка проноса запрещённых предметов на территорию",
	"Конфликтная ситуация с сотрудником охраны",
	"Работа без средств индивидуальной защиты на охраняемом объекте",
	"Передача пропуска третьему лицу",
	"Систематическое нарушение согласованного графика допуска",
	"Внесён по представлению службы безопасности организации",
	"Появление на территории в состоянии алкогольного опьянения",
	"Нарушение режима коммерческой тайны",
	"Отказ от прохождения досмотра на КПП",
	"Курение в местах, не предназначенных для этого",
}

// surnameMutationLetters -- согласные, которыми меняем последнюю букву фамилии в похожей
// паре ЧС. Ограничены согласными: фамилии словаря surnames (names.go) оканчиваются на
// согласную (-ов/-ев/-ин), и замена на гласную выглядела бы неправдоподобно ("Иванов" ->
// "Иваноа").
var surnameMutationLetters = []rune("бвгджзклмнпрстфхцчшщ")

// personCandidate -- то немногое из записи реестра сотрудников, что нужно похожей паре:
// ФИО.
type personCandidate struct {
	lastName, firstName, middleName string
}

// personCandidates отбирает из реестра сотрудников с заполненными фамилией и именем.
func personCandidates(employees []services.UniqueEmployeeWithRelations) []personCandidate {
	out := make([]personCandidate, 0, len(employees))
	for _, e := range employees {
		if e.LastName == nil || e.FirstName == nil {
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
		out = append(out, personCandidate{lastName: last, firstName: first, middleName: middle})
	}
	return out
}

// personBlacklistStreams -- независимые потоки случайности для наливки чёрного списка
// людей, см. vehicleBlacklistStreams.
type personBlacklistStreams struct {
	similarPick *Stream // кого из реестра взять для похожей пары
	letterPick  *Stream // на какую букву заменить конец фамилии в похожей паре
	independent *Stream // ФИО независимых (не похожих) записей, см. RandomFullName
	reason      *Stream
}

func newPersonBlacklistStreams(seed int64) *personBlacklistStreams {
	return &personBlacklistStreams{
		similarPick: NewStream(seed, "blacklist-person-similar-pick"),
		letterPick:  NewStream(seed, "blacklist-person-letter"),
		independent: NewStream(seed, "blacklist-person-independent-names"),
		reason:      NewStream(seed, "blacklist-person-reason"),
	}
}

func runPersonBlacklist(ctx context.Context, env *Env, username string) error {
	employees, err := services.NewUniqueEmployeeService(env.DB).GetAll(ctx, username, "")
	if err != nil {
		return fmt.Errorf("реестр сотрудников для чёрного списка: %w", err)
	}
	candidates := personCandidates(employees)
	if len(candidates) == 0 {
		return fmt.Errorf("реестр сотрудников пуст -- нечем строить похожие пары для чёрного списка " +
			"(шаг реестров должен был выполниться раньше и оставить хотя бы одного сотрудника)")
	}

	svc := services.NewPersonBlacklistService(env.DB, services.NewAuditRecorder(env.DB))
	streams := newPersonBlacklistStreams(env.Seed)
	similarCount := similarPairCount(env.Profile.Blacklists)

	for i := 0; i < env.Profile.Blacklists; i++ {
		entry, err := createFakePersonBlacklist(ctx, svc, i < similarCount, candidates, streams, env.ActorUserID)
		if err != nil {
			return fmt.Errorf("запись чёрного списка людей %d/%d: %w", i+1, env.Profile.Blacklists, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityPersonBlacklist, entry.ID); err != nil {
			return fmt.Errorf("регистрация записи чёрного списка людей %d в партии: %w", entry.ID, err)
		}
	}
	return nil
}

// createFakePersonBlacklist создаёт запись, повторяя попытку со свежими случайными данными
// при конфликте уникальности (см. blacklistCreateRetries).
func createFakePersonBlacklist(ctx context.Context, svc services.PersonBlacklistService, similar bool, candidates []personCandidate, s *personBlacklistStreams, actorID int) (*models.PersonBlacklist, error) {
	var lastErr error
	for attempt := 0; attempt < blacklistCreateRetries; attempt++ {
		req := buildPersonBlacklistRequest(similar, candidates, s)
		entry, err := svc.Create(ctx, req, actorID)
		if err == nil {
			return entry, nil
		}
		if !isBlacklistConflict(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("не удалось создать запись за %d попыток, ФИО конфликтует: %w", blacklistCreateRetries, lastErr)
}

func buildPersonBlacklistRequest(similar bool, candidates []personCandidate, s *personBlacklistStreams) models.CreatePersonBlacklistRequest {
	reason := Pick(s.reason, personBlacklistReasons)

	if similar {
		p := Pick(s.similarPick, candidates)
		last := mutateLastRune(s.letterPick, p.lastName, surnameMutationLetters)
		return models.CreatePersonBlacklistRequest{LastName: last, FirstName: p.firstName, MiddleName: p.middleName, Reason: reason}
	}

	name := RandomFullName(s.independent)
	return models.CreatePersonBlacklistRequest{LastName: name.LastName, FirstName: name.FirstName, MiddleName: name.MiddleName, Reason: reason}
}
