package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Steps() перечисляет девять готовых на сегодня срезов наполнения (#1682): организации/
// компании, справочники без профиля, таблицы постов, реестры сотрудников и машин, чёрные
// списки, пользователи, заявки, стадии обработки заявок, проходы через посты.
func TestSteps_ReturnsRegisteredSteps(t *testing.T) {
	steps := fakedata.Steps()
	require.Len(t, steps, 9)
	for _, s := range steps {
		require.NotEmpty(t, s.Name(), "у каждого шага должно быть непустое имя для вывода и ошибок")
	}
}

// Plan честно показывает организации и компании по объёму выбранного профиля.
func TestPlan_OrganizationsAndCompaniesScaleWithProfile(t *testing.T) {
	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			items := fakedata.Plan(profile)
			counts := planCounts(items)

			require.Equal(t, profile.Organizations, counts[models.AuditEntityOrganization])
			require.Equal(t, profile.Companies, counts[models.AuditEntityCompany])
		})
	}
}

// Справочники без профиля (места разгрузки, марки, гражданства, форматы номеров),
// таблицы постов и шаблоны вложений не масштабируются профилем -- один и тот же
// кандидатский список показывается независимо от small/medium/large (см.
// lookupsStep.Plan, postsStep.Plan в internal/fakedata/lookups.go, posts.go, и
// applicationsStep.Plan в applications.go).
func TestPlan_DictionariesAndPostsAreFixedRegardlessOfProfile(t *testing.T) {
	want := map[string]int{
		models.AuditEntityUnloadPlace:        10,
		models.AuditEntityMark:               20,
		models.AuditEntityCitizenship:        10,
		models.AuditEntityLicensePlateFormat: 3,
		models.AuditEntitySystemTable:        4,
		models.AuditEntityUniqueAttachment:   3,
	}

	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			counts := planCounts(fakedata.Plan(profile))
			for entity, count := range want {
				require.Equal(t, count, counts[entity], "entity=%s", entity)
			}
		})
	}
}

// PlanTotal суммирует ровно то, что создастся: строки создаваемых записей, без строк
// отметок на уже существующих.
func TestPlanTotal_SumsCreatedItemsOnly(t *testing.T) {
	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	items := fakedata.Plan(profile)
	var want int
	for _, item := range items {
		if item.Mark {
			continue
		}
		want += item.Count
	}
	require.Equal(t, want, fakedata.PlanTotal(items))
	require.Positive(t, fakedata.PlanTotal(items))
}

// Отметки прохода не должны попадать в «всего создастся»: они ставятся на уже созданных
// машинах и сотрудниках. Пока они входили в итог, предпоказ обещал 1465 записей, а отчёт
// о наливке показывал 963 -- разница объяснялась только чтением исходников.
func TestPlan_PassageMarksAreSeparatedFromCreatedRecords(t *testing.T) {
	profile, err := fakedata.ProfileByName("medium")
	require.NoError(t, err)

	items := fakedata.Plan(profile)
	marks := fakedata.PlanMarks(items)
	require.Len(t, marks, 2, "отметки проезда машин и прохода сотрудников")

	var markTotal int
	for _, item := range marks {
		require.True(t, item.Mark)
		markTotal += item.Count
	}
	require.Positive(t, markTotal)

	records := fakedata.PlanRecords(items)
	require.Len(t, records, len(items)-len(marks))
	require.Equal(t, len(items), len(records)+len(marks), "строка плана либо создаётся, либо отмечается")
	require.Equal(t, fakedata.PlanTotal(items)+markTotal, planItemsSum(items),
		"итог создаваемого плюс отметки -- это вся таблица плана")
}

func planItemsSum(items []fakedata.PlanItem) int {
	var sum int
	for _, item := range items {
		sum += item.Count
	}
	return sum
}

func planCounts(items []fakedata.PlanItem) map[string]int {
	counts := make(map[string]int, len(items))
	for _, item := range items {
		counts[item.Entity] = item.Count
	}
	return counts
}
