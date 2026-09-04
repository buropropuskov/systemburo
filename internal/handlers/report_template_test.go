package handlers_test

import (
	"context"
	"encoding/json"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/reportpresets"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownedByUser считает личные (не системные) шаблоны, принадлежащие пользователю.
func ownedByUser(tpls []models.ReportTemplate, userID int) int {
	n := 0
	for _, t := range tpls {
		if !t.IsSystem && t.OwnerUserID != nil && *t.OwnerUserID == userID {
			n++
		}
	}
	return n
}

// TestReportTemplates_Scoping: системные пресеты видны всем, личные — только
// владельцу, расшаренные — всем.
func TestReportTemplates_Scoping(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	require.NoError(t, database.SeedReportTemplates(db)) // CleanDB чистит report_templates -> пересеваем системные

	svc := services.NewStatisticsService(db, 0)
	ctx := context.Background()
	cfg := json.RawMessage(`{"mode":"list","entity":"cars"}`)

	userA := models.User{Username: "tpl_owner_a", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&userA).Error)
	userB := models.User{Username: "tpl_owner_b", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&userB).Error)

	tplA, err := svc.CreateReportTemplate(ctx, userA.ID, models.SaveReportTemplateRequest{Name: "Личный A", Config: cfg})
	require.NoError(t, err)
	assert.False(t, tplA.IsSystem)
	require.NotNil(t, tplA.OwnerUserID)
	assert.Equal(t, userA.ID, *tplA.OwnerUserID)

	// A видит системные + свой личный.
	listA, err := svc.ListReportTemplates(ctx, userA.ID)
	require.NoError(t, err)
	systemCount := 0
	presets, err := reportpresets.All()
	require.NoError(t, err)
	firstPresetName := presets[0].Title

	hasFirstPreset := false
	for _, tpl := range listA {
		if tpl.IsSystem {
			systemCount++
			if tpl.Name == firstPresetName {
				hasFirstPreset = true
			}
		}
	}
	assert.GreaterOrEqual(t, systemCount, len(presets), "системные пресеты засеяны по общему источнику")
	assert.True(t, hasFirstPreset, "сидирован пресет %q", firstPresetName)
	assert.Equal(t, 1, ownedByUser(listA, userA.ID))

	// B не видит приватный A.
	listB, err := svc.ListReportTemplates(ctx, userB.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, ownedByUser(listB, userA.ID), "B не видит приватный шаблон A")

	// A расшаривает -> B видит.
	shared := true
	_, err = svc.UpdateReportTemplate(ctx, userA.ID, tplA.ID, models.SaveReportTemplateRequest{Name: "Личный A", Config: cfg, IsShared: &shared})
	require.NoError(t, err)
	listB2, err := svc.ListReportTemplates(ctx, userB.ID)
	require.NoError(t, err)
	found := false
	for _, tpl := range listB2 {
		if tpl.ID == tplA.ID {
			found = true
		}
	}
	assert.True(t, found, "B видит расшаренный шаблон A")
}

// TestReportTemplates_Protection: системные и чужие шаблоны защищены от правки и
// удаления; конфиг валидируется.
func TestReportTemplates_Protection(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	require.NoError(t, database.SeedReportTemplates(db)) // CleanDB чистит report_templates -> пересеваем системные

	svc := services.NewStatisticsService(db, 0)
	ctx := context.Background()
	cfg := json.RawMessage(`{"mode":"list","entity":"cars"}`)

	userA := models.User{Username: "tpl_prot_a", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&userA).Error)
	userB := models.User{Username: "tpl_prot_b", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&userB).Error)

	tplA, err := svc.CreateReportTemplate(ctx, userA.ID, models.SaveReportTemplateRequest{Name: "Защита A", Config: cfg})
	require.NoError(t, err)

	// Чужой не может править/удалять.
	_, err = svc.UpdateReportTemplate(ctx, userB.ID, tplA.ID, models.SaveReportTemplateRequest{Name: "Взлом", Config: cfg})
	assert.ErrorIs(t, err, services.ErrTemplateForbidden)
	assert.ErrorIs(t, svc.DeleteReportTemplate(ctx, userB.ID, tplA.ID), services.ErrTemplateForbidden)

	// Системный пресет защищён.
	var sysTpl models.ReportTemplate
	require.NoError(t, db.Where("is_system = ?", true).First(&sysTpl).Error)
	_, err = svc.UpdateReportTemplate(ctx, userA.ID, sysTpl.ID, models.SaveReportTemplateRequest{Name: "Подмена", Config: cfg})
	assert.ErrorIs(t, err, services.ErrTemplateSystem)
	assert.ErrorIs(t, svc.DeleteReportTemplate(ctx, userA.ID, sysTpl.ID), services.ErrTemplateSystem)

	// Несуществующий.
	_, err = svc.UpdateReportTemplate(ctx, userA.ID, 999999, models.SaveReportTemplateRequest{Name: "Нет", Config: cfg})
	assert.ErrorIs(t, err, services.ErrTemplateNotFound)

	// Пустой/невалидный конфиг.
	_, err = svc.CreateReportTemplate(ctx, userA.ID, models.SaveReportTemplateRequest{Name: "Битый", Config: json.RawMessage(`{не json`)})
	assert.ErrorIs(t, err, services.ErrTemplateInvalidConfig)
	_, err = svc.CreateReportTemplate(ctx, userA.ID, models.SaveReportTemplateRequest{Name: "Пустой", Config: nil})
	assert.ErrorIs(t, err, services.ErrTemplateInvalidConfig)
	// JSON-null валиден для json.Valid, но не объект -> тоже отклоняем.
	_, err = svc.CreateReportTemplate(ctx, userA.ID, models.SaveReportTemplateRequest{Name: "Налл", Config: json.RawMessage("null")})
	assert.ErrorIs(t, err, services.ErrTemplateInvalidConfig)

	// Владелец может удалить свой.
	require.NoError(t, svc.DeleteReportTemplate(ctx, userA.ID, tplA.ID))
	_, err = svc.UpdateReportTemplate(ctx, userA.ID, tplA.ID, models.SaveReportTemplateRequest{Name: "После удаления", Config: cfg})
	assert.ErrorIs(t, err, services.ErrTemplateNotFound)
}

// TestSeedReportTemplates_Idempotent: повторный сид не дублирует системные пресеты.
func TestSeedReportTemplates_Idempotent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	presets, err := reportpresets.All()
	require.NoError(t, err)
	want := int64(len(presets))

	require.NoError(t, database.SeedReportTemplates(db))
	var first int64
	require.NoError(t, db.Model(&models.ReportTemplate{}).Where("is_system = ?", true).Count(&first).Error)
	assert.Equal(t, want, first, "системных пресетов столько же, сколько наборов в источнике")

	require.NoError(t, database.SeedReportTemplates(db))
	var second int64
	require.NoError(t, db.Model(&models.ReportTemplate{}).Where("is_system = ?", true).Count(&second).Error)
	assert.Equal(t, want, second, "повторный сид не дублирует")
}

// TestSeedReportTemplates_RenamesLegacy: у заказчика в базе лежат наборы под старыми
// именами (#632). Сид переименовывает их, а не создаёт рядом дубль - иначе список,
// ради сведения которого всё и делалось, снова разъехался бы (#2315).
func TestSeedReportTemplates_RenamesLegacy(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	legacy := models.ReportTemplate{
		Name:        "Сводка за неделю",
		Description: "Поданные заявки по дням за последнюю неделю.",
		Config:      json.RawMessage(`{"mode":"aggregate","metrics":["applications_count"]}`),
		IsSystem:    true,
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, database.SeedReportTemplates(db))

	var stale int64
	require.NoError(t, db.Model(&models.ReportTemplate{}).
		Where("name = ? AND is_system = ?", "Сводка за неделю", true).Count(&stale).Error)
	assert.Equal(t, int64(0), stale, "прежнее имя не осталось в базе")

	var renamed models.ReportTemplate
	require.NoError(t, db.Where("id = ?", legacy.ID).First(&renamed).Error)
	assert.Equal(t, "Сводка по заявкам", renamed.Name, "запись переименована, а не пересоздана")

	presets, err := reportpresets.All()
	require.NoError(t, err)
	var total int64
	require.NoError(t, db.Model(&models.ReportTemplate{}).Where("is_system = ?", true).Count(&total).Error)
	assert.Equal(t, int64(len(presets)), total, "дубля рядом с переименованным не появилось")

	// Описание подтянулось из общего источника, а не осталось прежним.
	var want string
	for _, p := range presets {
		if p.Title == "Сводка по заявкам" {
			want = p.Description
		}
	}
	assert.Equal(t, want, renamed.Description, "описание обновлено по источнику")
}
