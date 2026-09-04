package services

import (
	"errors"
	"strings"
	"testing"

	"systemburo/internal/models"
)

// TestListEngine_CoversCatalogEntities гарантирует, что для КАЖДОЙ list-сущности
// каталога (B1) движок умеет собрать план со всеми её столбцами. Иначе UI
// предложит выгрузку, которая упадёт на исполнении.
func TestListEngine_CoversCatalogEntities(t *testing.T) {
	for _, entity := range reportListEntityOrder {
		req := models.ReportRequest{Mode: "list", Entity: entity}
		plan, err := buildListPlan(req, false)
		if err != nil {
			t.Errorf("сущность %q: движок не собрал план: %v", entity, err)
			continue
		}
		catCols := reportListEntityRegistry[entity].columns
		if len(plan.columns) != len(catCols) {
			t.Errorf("сущность %q: столбцов в плане %d, в каталоге %d", entity, len(plan.columns), len(catCols))
		}
		for i, c := range catCols {
			if plan.columns[i].Key != c.key {
				t.Errorf("сущность %q: столбец #%d ключ %q != каталог %q", entity, i, plan.columns[i].Key, c.key)
			}
			// Тип форматирования (date/time/datetime) пробрасывается фронту как есть.
			if plan.columns[i].Type != c.format {
				t.Errorf("сущность %q: столбец %q тип %q != каталог %q", entity, c.key, plan.columns[i].Type, c.format)
			}
			// каждый столбец каталога должен попасть в SELECT под своим алиасом
			if !strings.Contains(plan.selectStr, " AS "+c.key) {
				t.Errorf("сущность %q: столбец %q отсутствует в select %q", entity, c.key, plan.selectStr)
			}
		}
	}
}

// TestListEngine_CatalogFiltersResolvable сверяет, что каждый фильтр, заявленный
// каталогом для list-сущности, движок умеет применить (date_range или filterExpr).
func TestListEngine_CatalogFiltersResolvable(t *testing.T) {
	for _, entity := range reportListEntityOrder {
		exec, ok := listExecRegistry[entity]
		if !ok {
			t.Errorf("сущность каталога %q без схемы исполнения", entity)
			continue
		}
		for _, f := range reportListEntityRegistry[entity].filters {
			if f == "date_range" {
				if exec.tsColumn == "" {
					t.Errorf("сущность %q: фильтр date_range без tsColumn", entity)
				}
				continue
			}
			if _, ok := exec.filterExpr[f]; !ok {
				t.Errorf("сущность %q: фильтр %q каталога без выражения исполнения", entity, f)
			}
		}
	}
}

// TestListEngine_ExecMatchesCatalog — у каждой схемы исполнения есть сущность в
// каталоге и наоборот (защита от рассинхрона реестров).
func TestListEngine_ExecMatchesCatalog(t *testing.T) {
	for _, entity := range reportListEntityOrder {
		if _, ok := listExecRegistry[entity]; !ok {
			t.Errorf("сущность каталога %q без схемы исполнения", entity)
		}
	}
	for entity := range listExecRegistry {
		if _, ok := reportListEntityRegistry[entity]; !ok {
			t.Errorf("схема исполнения %q без сущности в каталоге", entity)
		}
	}
}

func TestListPlan_WorkApplicationsBaseFilter(t *testing.T) {
	req := models.ReportRequest{Mode: "list", Entity: "work_applications"}
	plan, err := buildListPlan(req, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	// baseWhere ограничивает выборку типом «Заявка на работы» через аргумент-плейсхолдер.
	var hasBase bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "ua.display_name = ?") {
			hasBase = true
			if len(w.args) != 1 || w.args[0] != workAttachmentDisplayName {
				t.Errorf("ожидался аргумент %q, got %v", workAttachmentDisplayName, w.args)
			}
		}
	}
	if !hasBase {
		t.Errorf("baseWhere по типу вложения не попал в where: %+v", plan.wheres)
	}
	// телефон ответственного добавлен в SELECT (по требованию шаблона).
	if !strings.Contains(plan.selectStr, "ru.phone") {
		t.Errorf("ожидался телефон ответственного в select %q", plan.selectStr)
	}
	// «наименование работ» подбирается по подписи через плейсхолдер (не конкатенацией).
	if !strings.Contains(plan.selectStr, "acf.label ILIKE ?") {
		t.Errorf("ожидался ILIKE ? для подписи поля работ, got %q", plan.selectStr)
	}
	if len(plan.selectArgs) != 1 || plan.selectArgs[0] != workNameFieldPattern {
		t.Errorf("ожидался selectArg %q, got %v", workNameFieldPattern, plan.selectArgs)
	}
}

func TestListPlan_FiltersAndDateRange(t *testing.T) {
	req := models.ReportRequest{
		Mode:   "list",
		Entity: "work_applications",
		Filters: []models.ReportFilterValue{
			{Key: "status", Values: []string{models.StatusInWork}},
			{Key: "date_range", From: "2026-06-01", To: "2026-06-17"},
			{Key: "organization", Values: []string{"  "}}, // только пустое -> пропустить
		},
	}
	plan, err := buildListPlan(req, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	var hasStatus, hasFrom, hasTo, hasOrg bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "app.status IN") {
			hasStatus = true
		}
		if strings.Contains(w.expr, "app.sending_datetime >= ?") {
			hasFrom = true
		}
		if strings.Contains(w.expr, "app.sending_datetime <= ?") {
			hasTo = true
		}
		if strings.Contains(w.expr, "org.name IN") {
			hasOrg = true
		}
	}
	if !hasStatus {
		t.Errorf("фильтр status не попал в where: %+v", plan.wheres)
	}
	if !hasFrom || !hasTo {
		t.Errorf("date_range не разложился на from/to: %+v", plan.wheres)
	}
	if hasOrg {
		t.Errorf("пустой фильтр organization не должен давать WHERE: %+v", plan.wheres)
	}
}

func TestListPlan_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		req  models.ReportRequest
	}{
		{"unknown entity", models.ReportRequest{Mode: "list", Entity: "nope"}},
		{"unknown filter", models.ReportRequest{Mode: "list", Entity: "cars",
			Filters: []models.ReportFilterValue{{Key: "status", Values: []string{models.StatusInWork}}}}},
		{"date_range without ts", models.ReportRequest{Mode: "list", Entity: "cars",
			Filters: []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildListPlan(tc.req, false)
			if err == nil {
				t.Fatalf("ожидалась ошибка валидации")
			}
			if !errors.Is(err, ErrInvalidReportRequest) {
				t.Errorf("ожидался ErrInvalidReportRequest, got %v", err)
			}
		})
	}
}

func TestListPlan_LimitClamped(t *testing.T) {
	req := models.ReportRequest{Mode: "list", Entity: "people", Limit: maxReportLimit + 500}
	plan, err := buildListPlan(req, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if plan.limit != maxReportLimit {
		t.Errorf("limit не зажат: got %d, want %d", plan.limit, maxReportLimit)
	}
}

// Пока персональные данные скрыты до согласия, колонка принимающего не должна
// отдавать ни ФИО, ни телефон: строку собирает база, и подменить её после выборки
// нечем - идентификатора работника в выдаче отчёта нет.
func TestBuildListPlan_MaskedResponsibleColumn(t *testing.T) {
	req := models.ReportRequest{Mode: "list", Entity: "work_applications"}

	open, err := buildListPlan(req, false)
	if err != nil {
		t.Fatalf("план без маскировки не собрался: %v", err)
	}
	if !strings.Contains(open.selectStr, "ru.phone") {
		t.Error("без маскировки телефон принимающего в колонке остаётся")
	}

	masked, err := buildListPlan(req, true)
	if err != nil {
		t.Fatalf("план с маскировкой не собрался: %v", err)
	}
	if !strings.Contains(masked.selectStr, "pd_consents") {
		t.Error("выдача колонки должна сверяться с наличием согласия")
	}
	if !strings.Contains(masked.selectStr, "ru.username") {
		t.Error("вместо ФИО и телефона ожидается логин")
	}
	// Мерка обязана совпасть с gatedUsersWhere: кого запрос согласия не касается,
	// того и в отчёте не обезличиваем, иначе супер-администратор и архивные
	// работники в отчёте выглядят иначе, чем во всей остальной системе.
	for _, guard := range []string{"ru.is_super_admin", "NOT ru.is_active", "ru.is_banned"} {
		if !strings.Contains(masked.selectStr, guard) {
			t.Errorf("в условии маскировки нет оговорки %q", guard)
		}
	}
	// Телефон остаётся только в ветке «согласие есть»; у остальных - логин.
	if !strings.Contains(masked.selectStr, "ELSE '@'") {
		t.Error("у работника без согласия ожидается логин вместо ФИО и телефона")
	}
}

// Выбор столбцов (#2313): выгрузка отдавала жёстко зашитый набор, и лишнее удаляли
// уже в Excel. Порядок держим каталожный - иначе колонки переставлялись бы от
// запроса к запросу, а вслед за ними и столбцы выгрузки.
func TestListPlan_ColumnsSubset(t *testing.T) {
	full, err := buildListPlan(models.ReportRequest{Mode: "list", Entity: "cars"}, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	req := models.ReportRequest{Mode: "list", Entity: "cars", Columns: []string{"mark", "car_number"}}
	plan, err := buildListPlan(req, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(plan.columns) != 2 {
		t.Fatalf("ожидались две колонки, got %d (%+v)", len(plan.columns), plan.columns)
	}

	// Порядок - как в каталоге, а не как в запросе (там марка шла первой).
	var wantOrder []string
	for _, c := range reportListEntityRegistry["cars"].columns {
		if c.key == "mark" || c.key == "car_number" {
			wantOrder = append(wantOrder, c.key)
		}
	}
	for i, c := range plan.columns {
		if c.Key != wantOrder[i] {
			t.Errorf("колонка %d: ожидался %q, got %q", i, wantOrder[i], c.Key)
		}
	}

	// Невыбранные столбцы не попадают ни в SELECT, ни в описание результата.
	for _, c := range full.columns {
		if c.Key == "mark" || c.Key == "car_number" {
			continue
		}
		if strings.Contains(plan.selectStr, " AS "+c.Key) {
			t.Errorf("невыбранный столбец %q попал в select %q", c.Key, plan.selectStr)
		}
	}
}

func TestListPlan_UnknownColumnRejected(t *testing.T) {
	req := models.ReportRequest{Mode: "list", Entity: "cars", Columns: []string{"car_number", "нет-такого"}}
	if _, err := buildListPlan(req, false); err == nil {
		t.Fatal("ожидалась ошибка на неизвестный столбец")
	}
}

func TestListPlan_EmptyColumnsKeepsAll(t *testing.T) {
	full, err := buildListPlan(models.ReportRequest{Mode: "list", Entity: "cars"}, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	empty, err := buildListPlan(models.ReportRequest{Mode: "list", Entity: "cars", Columns: []string{}}, false)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(empty.columns) != len(full.columns) {
		t.Errorf("пустой выбор должен давать все столбцы: %d против %d", len(empty.columns), len(full.columns))
	}
}
