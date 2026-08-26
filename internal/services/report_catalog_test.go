package services

import (
	"testing"

	"systemburo/internal/models"
)

// TestBuildReportCatalog_Structure проверяет, что каталог собирается из реестров
// в фиксированном порядке и с полным набором элементов.
func TestBuildReportCatalog_Structure(t *testing.T) {
	cat := buildReportCatalog(dynamicReportOptions{})

	if len(cat.Metrics) != len(reportMetricOrder) {
		t.Fatalf("metrics: got %d, want %d", len(cat.Metrics), len(reportMetricOrder))
	}
	for i, key := range reportMetricOrder {
		if cat.Metrics[i].Key != key {
			t.Errorf("metric[%d]: got %q, want %q", i, cat.Metrics[i].Key, key)
		}
		if cat.Metrics[i].Label == "" {
			t.Errorf("metric %q: пустая подпись", key)
		}
	}

	if len(cat.Dimensions) != len(reportDimensionOrder) {
		t.Fatalf("dimensions: got %d, want %d", len(cat.Dimensions), len(reportDimensionOrder))
	}
	if len(cat.Filters) != len(reportFilterOrder) {
		t.Fatalf("filters: got %d, want %d", len(cat.Filters), len(reportFilterOrder))
	}
	if len(cat.ListEntities) != len(reportListEntityOrder) {
		t.Fatalf("list entities: got %d, want %d", len(cat.ListEntities), len(reportListEntityOrder))
	}
	if len(cat.Granularities) == 0 {
		t.Error("granularities пустые")
	}
}

// TestReportRegistries_Consistency ловит висячие ссылки между реестрами:
// каждый разрез метрики, фильтр сущности и т.п. должен существовать.
func TestReportRegistries_Consistency(t *testing.T) {
	for key, m := range reportMetricRegistry {
		if m.baseTable == "" || m.aggExpr == "" {
			t.Errorf("metric %q: пустой baseTable/aggExpr", key)
		}
		for _, dim := range m.dimensions {
			if _, ok := reportDimensionRegistry[dim]; !ok {
				t.Errorf("metric %q ссылается на неизвестный разрез %q", key, dim)
			}
		}
	}

	for key, e := range reportListEntityRegistry {
		if len(e.columns) == 0 {
			t.Errorf("list entity %q без столбцов", key)
		}
		for _, f := range e.filters {
			if _, ok := reportFilterRegistry[f]; !ok {
				t.Errorf("list entity %q ссылается на неизвестный фильтр %q", key, f)
			}
		}
	}

	for _, key := range reportMetricOrder {
		if _, ok := reportMetricRegistry[key]; !ok {
			t.Errorf("metric order содержит отсутствующий ключ %q", key)
		}
	}
	for _, key := range reportFilterOrder {
		if _, ok := reportFilterRegistry[key]; !ok {
			t.Errorf("filter order содержит отсутствующий ключ %q", key)
		}
	}
	for _, key := range reportDimensionOrder {
		if _, ok := reportDimensionRegistry[key]; !ok {
			t.Errorf("dimension order содержит отсутствующий ключ %q", key)
		}
	}
	for _, key := range reportListEntityOrder {
		if _, ok := reportListEntityRegistry[key]; !ok {
			t.Errorf("list entity order содержит отсутствующий ключ %q", key)
		}
	}
}

// TestMetricApplicableFilters проверяет, что каталог отдаёт per-metric фильтры,
// каждый из них исполним движком (date_range или есть в aggMetricRegistry.filters),
// date_range присутствует у всех метрик, а порядок совпадает с reportFilterOrder.
func TestMetricApplicableFilters(t *testing.T) {
	cat := buildReportCatalog(dynamicReportOptions{})
	for _, m := range cat.Metrics {
		if len(m.Filters) == 0 {
			t.Errorf("метрика %q без применимых фильтров", m.Key)
			continue
		}
		var hasDateRange bool
		schema := aggMetricRegistry[m.Key]
		prev := -1
		for _, f := range m.Filters {
			if f == "date_range" {
				hasDateRange = true
			} else {
				if _, ok := schema.filters[f]; !ok {
					t.Errorf("метрика %q публикует фильтр %q, не исполнимый движком", m.Key, f)
				}
				if _, ok := reportFilterRegistry[f]; !ok {
					t.Errorf("метрика %q публикует фильтр %q без записи в каталоге фильтров", m.Key, f)
				}
			}
			// порядок — подпоследовательность reportFilterOrder.
			idx := indexOf(reportFilterOrder, f)
			if idx < 0 {
				t.Errorf("фильтр %q вне reportFilterOrder", f)
			} else if idx <= prev {
				t.Errorf("метрика %q: фильтры не в порядке reportFilterOrder (%v)", m.Key, m.Filters)
			} else {
				prev = idx
			}
		}
		if !hasDateRange {
			t.Errorf("метрика %q: date_range должен быть применим (период по tsColumn)", m.Key)
		}
	}
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return -1
}

// TestBuildReportCatalog_DynamicOptions проверяет подстановку значений
// динамических справочников в соответствующие dict-фильтры.
func TestBuildReportCatalog_DynamicOptions(t *testing.T) {
	dyn := dynamicReportOptions{
		organizations: []models.ReportOption{{Value: "ООО Рога", Label: "ООО Рога"}},
		unloadPlaces:  []models.ReportOption{{Value: "Дебаркадер №1", Label: "Дебаркадер №1"}},
	}
	cat := buildReportCatalog(dyn)

	byKey := make(map[string]models.ReportFilterInfo, len(cat.Filters))
	for _, f := range cat.Filters {
		byKey[f.Key] = f
	}

	if got := byKey["organization"].Options; len(got) != 1 || got[0].Value != "ООО Рога" {
		t.Errorf("organization options: got %+v", got)
	}
	if got := byKey["unload_place"].Options; len(got) != 1 || got[0].Value != "Дебаркадер №1" {
		t.Errorf("unload_place options: got %+v", got)
	}
	// company не передавали -> опций нет
	if got := byKey["company"].Options; len(got) != 0 {
		t.Errorf("company options: ожидалось пусто, got %+v", got)
	}
	// status — статичный enum, не зависит от dyn
	if got := byKey["status"].Options; len(got) == 0 {
		t.Error("status options: ожидался статичный набор статусов")
	}
	if byKey["date_range"].Type != models.ReportFieldDate {
		t.Errorf("date_range: тип %q, ожидался date", byKey["date_range"].Type)
	}
}
