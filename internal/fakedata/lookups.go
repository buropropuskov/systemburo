package fakedata

import (
	"context"
	"fmt"

	"systemburo/internal/models"
	"systemburo/internal/services"
)

// lookupsStep досыпает четыре небольших справочника фиксированными наборами
// правдоподобных значений: места разгрузки, марки машин, гражданства, форматы
// номеров. В отличие от organizationsStep эти справочники не масштабируются
// профилем -- для проверки селекторов, фильтров и формы сотрудника/машины
// достаточно одного устойчивого набора, а не тысяч записей.
//
// Шаг идемпотентен по построению: перед созданием каждой сущности читает через
// сервис уже существующие имена (активные и архивные) и пропускает совпадения.
// Часть кандидатов (места разгрузки, форматы РФ/Беларусь/Казахстан) совпадает по
// имени с cmd/seed/demo.go -- если тот сид уже отработал (SEED_DEMO=true), эти
// записи пропускаются как уже существующие, а не дублируются.
type lookupsStep struct{}

func (lookupsStep) Name() string {
	return "места разгрузки, марки, гражданства, форматы номеров"
}

func (lookupsStep) Plan(Profile) []PlanItem {
	// Count -- размер кандидатского списка, то есть верхняя граница: сколько
	// создастся, если на стенде нет вообще ничего. Реальное число может быть
	// меньше -- уже существующие имена шаг пропускает, а Plan(p Profile) по
	// контракту пакета (см. generator.go) не имеет доступа к базе, чтобы
	// посчитать точный остаток.
	return []PlanItem{
		{Entity: models.AuditEntityUnloadPlace, Title: EntityTitle(models.AuditEntityUnloadPlace), Count: len(unloadPlaceCandidates)},
		{Entity: models.AuditEntityMark, Title: EntityTitle(models.AuditEntityMark), Count: len(markCandidates)},
		{Entity: models.AuditEntityCitizenship, Title: EntityTitle(models.AuditEntityCitizenship), Count: len(citizenshipCandidates)},
		{Entity: models.AuditEntityLicensePlateFormat, Title: EntityTitle(models.AuditEntityLicensePlateFormat), Count: len(plateFormatCandidates)},
	}
}

func (lookupsStep) Run(ctx context.Context, env *Env) error {
	if err := runUnloadPlaces(ctx, env); err != nil {
		return err
	}
	if err := runMarks(ctx, env); err != nil {
		return err
	}
	if err := runCitizenships(ctx, env); err != nil {
		return err
	}
	return runPlateFormats(ctx, env)
}

// unloadPlaceCandidates -- места разгрузки, которых стенду не хватает для
// проверки селекторов и привязок к организациям/компаниям.
var unloadPlaceCandidates = []string{
	"Склад №1", "Склад №2", "Склад №3",
	"Рампа А", "Рампа Б", "Рампа В",
	"Северный въезд", "Южный въезд",
	"Контейнерная площадка", "Погрузочная площадка №1",
}

func runUnloadPlaces(ctx context.Context, env *Env) error {
	svc := services.NewUnloadPlaceService(env.DB)
	all, err := svc.GetAll(ctx, true)
	if err != nil {
		return fmt.Errorf("места разгрузки: %w", err)
	}
	existing := make(map[string]bool, len(all))
	for _, p := range all {
		existing[p.Name] = true
	}
	for _, name := range unloadPlaceCandidates {
		if existing[name] {
			continue
		}
		id, err := svc.Create(ctx, env.ActorUserID, services.CreateUnloadPlaceRequest{Name: name})
		if err != nil {
			return fmt.Errorf("место разгрузки %q: %w", name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityUnloadPlace, id); err != nil {
			return fmt.Errorf("регистрация места разгрузки %d в партии: %w", id, err)
		}
	}
	return nil
}

// markCandidates -- марки машин: типичные грузовые и легковые марки, которые
// встречаются на въезде (отечественная коммерческая техника вперемешку с
// легковыми иномарками), достаточно для проверки формы машины и фильтров.
var markCandidates = []string{
	"КамАЗ", "ГАЗель", "ГАЗ", "УАЗ", "ПАЗ", "ЛиАЗ", "ВАЗ (Lada)",
	"Toyota", "Hyundai", "Kia", "Volkswagen", "Ford", "Renault",
	"Mercedes-Benz", "Volvo", "Scania", "MAN", "Iveco", "Isuzu", "Mitsubishi Fuso",
}

func runMarks(ctx context.Context, env *Env) error {
	svc := services.NewMarkService(env.DB)
	all, err := svc.GetAll(ctx, true)
	if err != nil {
		return fmt.Errorf("марки машин: %w", err)
	}
	existing := make(map[string]bool, len(all))
	for _, m := range all {
		existing[m.Name] = true
	}
	for _, name := range markCandidates {
		if existing[name] {
			continue
		}
		mark, err := svc.Create(ctx, models.CreateMarkRequest{Name: name}, env.ActorUserID)
		if err != nil {
			return fmt.Errorf("марка %q: %w", name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityMark, mark.ID); err != nil {
			return fmt.Errorf("регистрация марки %d в партии: %w", mark.ID, err)
		}
	}
	return nil
}

// citizenshipCandidate -- кандидат гражданства с признаком патента.
type citizenshipCandidate struct {
	Name           string
	PatentRequired bool
}

// citizenshipCandidates -- гражданства для проверки патентного признака в форме
// сотрудника. Страны ЕАЭС (Беларусь, Казахстан, Армения, Киргизия) патент на
// работу не оформляют по договору о трудовой миграции внутри союза, у остальных
// -- оформляют (PatentRequired=true), это и есть признак, который должен быть на
// стенде разным у разных стран, а не одинаковым у всех.
var citizenshipCandidates = []citizenshipCandidate{
	{Name: "Россия", PatentRequired: false},
	{Name: "Беларусь", PatentRequired: false},
	{Name: "Казахстан", PatentRequired: false},
	{Name: "Армения", PatentRequired: false},
	{Name: "Киргизия", PatentRequired: false},
	{Name: "Узбекистан", PatentRequired: true},
	{Name: "Таджикистан", PatentRequired: true},
	{Name: "Молдова", PatentRequired: true},
	{Name: "Украина", PatentRequired: true},
	{Name: "Азербайджан", PatentRequired: true},
}

func runCitizenships(ctx context.Context, env *Env) error {
	svc := services.NewCitizenshipService(env.DB)
	all, err := svc.GetAll(ctx, true)
	if err != nil {
		return fmt.Errorf("гражданства: %w", err)
	}
	existing := make(map[string]bool, len(all))
	hasDefault := false
	for _, c := range all {
		existing[c.Name] = true
		if c.IsDefault {
			hasDefault = true
		}
	}
	for _, cand := range citizenshipCandidates {
		if existing[cand.Name] {
			continue
		}
		// "Россия" становится гражданством по умолчанию, только если на стенде
		// ещё нет ни одного дефолтного -- иначе Create молча снял бы дефолт с
		// того, что назначил администратор или предыдущая партия.
		isDefault := !hasDefault && cand.Name == "Россия"
		patentRequired := cand.PatentRequired
		id, err := svc.Create(ctx, env.ActorUserID, models.CreateCitizenshipRequest{
			Name: cand.Name, IsDefault: &isDefault, PatentRequired: &patentRequired,
		})
		if err != nil {
			return fmt.Errorf("гражданство %q: %w", cand.Name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityCitizenship, id); err != nil {
			return fmt.Errorf("регистрация гражданства %d в партии: %w", id, err)
		}
		if isDefault {
			hasDefault = true
		}
	}
	return nil
}

// plateFormatCandidate -- кандидат формата номера со своими ячейками.
type plateFormatCandidate struct {
	Name        string
	CountryCode string
	Icon        string
	Cells       []models.CreateFormatCellRequest
}

// plateFormatCandidates -- форматы номеров для трёх стран, чьи машины реально
// встречаются на въезде (та же тройка, что cmd/seed/demo.go заводит без ячеек).
// Ячейки описывают структуру знака правдоподобно для валидатора формы номера,
// но не претендуют на точность действующего стандарта каждой страны -- это
// вымышленные данные проверочного стенда, а не юридический справочник.
var plateFormatCandidates = []plateFormatCandidate{
	{
		Name: "Россия", CountryCode: "RU", Icon: "🇷🇺",
		Cells: []models.CreateFormatCellRequest{
			{CellOrder: 1, CellType: "letter", MinLength: intPtr(1), MaxLength: intPtr(1), AllowedLetters: strPtr(PlateLetters), AlphabetType: strPtr("cyrillic"), Language: strPtr("ru")},
			{CellOrder: 2, CellType: "digit", MinLength: intPtr(3), MaxLength: intPtr(3)},
			{CellOrder: 3, CellType: "letter", MinLength: intPtr(2), MaxLength: intPtr(2), AllowedLetters: strPtr(PlateLetters), AlphabetType: strPtr("cyrillic"), Language: strPtr("ru")},
			{CellOrder: 4, CellType: "digit", MinLength: intPtr(2), MaxLength: intPtr(3)},
		},
	},
	{
		Name: "Беларусь", CountryCode: "BY", Icon: "🇧🇾",
		Cells: []models.CreateFormatCellRequest{
			{CellOrder: 1, CellType: "digit", MinLength: intPtr(4), MaxLength: intPtr(4)},
			{CellOrder: 2, CellType: "letter", MinLength: intPtr(2), MaxLength: intPtr(2), AllowedLetters: strPtr("ABEKMHOPCTXY"), AlphabetType: strPtr("latin"), Language: strPtr("by")},
			{CellOrder: 3, CellType: "digit", MinLength: intPtr(1), MaxLength: intPtr(1)},
		},
	},
	{
		Name: "Казахстан", CountryCode: "KZ", Icon: "🇰🇿",
		Cells: []models.CreateFormatCellRequest{
			{CellOrder: 1, CellType: "digit", MinLength: intPtr(3), MaxLength: intPtr(3)},
			{CellOrder: 2, CellType: "letter", MinLength: intPtr(3), MaxLength: intPtr(3), AllowedLetters: strPtr(PlateLetters), AlphabetType: strPtr("cyrillic"), Language: strPtr("kz")},
			{CellOrder: 3, CellType: "digit", MinLength: intPtr(2), MaxLength: intPtr(2)},
		},
	},
}

func runPlateFormats(ctx context.Context, env *Env) error {
	svc := services.NewLicensePlateFormatService(env.DB)
	all, err := svc.GetAll(ctx, true)
	if err != nil {
		return fmt.Errorf("форматы номеров: %w", err)
	}
	existing := make(map[string]bool, len(all))
	hasDefault := false
	for _, f := range all {
		existing[f.Format.Name] = true
		if f.Format.IsDefault {
			hasDefault = true
		}
	}
	for _, cand := range plateFormatCandidates {
		if existing[cand.Name] {
			continue
		}
		isDefault := !hasDefault && cand.Name == "Россия"
		id, err := svc.Create(ctx, env.ActorUserID, models.CreateLicensePlateFormatRequest{
			Name: cand.Name, CountryCode: strPtr(cand.CountryCode), Icon: strPtr(cand.Icon),
			IsDefault: &isDefault, Cells: cand.Cells,
		})
		if err != nil {
			return fmt.Errorf("формат номеров %q: %w", cand.Name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityLicensePlateFormat, id); err != nil {
			return fmt.Errorf("регистрация формата номеров %d в партии: %w", id, err)
		}
		if isDefault {
			hasDefault = true
		}
	}
	return nil
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
