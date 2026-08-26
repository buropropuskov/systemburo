package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

// Тесты опциональных фильтров вкладки "Доступные мне" (#706, срез BE-S6). DB-backed, пакет
// handlers_test (единственный DB-бинарь, см. security_visibility_test.go). Переиспользуют secWorld
// и хелперы из security_visibility_test.go; ниже - только доп. фабрики для org/company/sender и
// именованных вложений. Главный инвариант: фильтры сужают выдачу ПОВЕРХ гейта мест, не вместо него.

func secPtrStr(s string) *string { return &s }

func (w secWorld) newOrg(t *testing.T, name string) int {
	t.Helper()
	o := models.Organization{Name: name, IsActive: true}
	require.NoError(t, w.db.Create(&o).Error)
	return o.ID
}

func (w secWorld) newCompany(t *testing.T, name string) int {
	t.Helper()
	c := models.Company{Name: name, IsActive: true}
	require.NoError(t, w.db.Create(&c).Error)
	return c.ID
}

func (w secWorld) newSenderNamed(t *testing.T, username, last, first, middle string) int {
	t.Helper()
	userTypeID := secUserTypeIDByCode(t, w.db, "user")
	u := models.User{
		Username: username, Password: "x", TypeID: userTypeID,
		LastName: &last, FirstName: &first, MiddleName: &middle,
	}
	require.NoError(t, w.db.Create(&u).Error)
	return u.ID
}

// newAppWith создаёт согласованную заявку в активном допуске (status='В работе') с заданными
// организацией/компанией/отправителем - видимую охране во вкладке "Доступные мне".
func (w secWorld) newAppWith(t *testing.T, orgID int, companyID *int, senderID int) int {
	t.Helper()
	now := time.Now()
	conf := models.ConfirmationApproved
	st := models.StatusInWork
	app := models.Application{
		OrganizationID:  orgID,
		CompanyID:       companyID,
		SenderUserID:    senderID,
		Confirmation:    &conf,
		Status:          &st,
		SendingDatetime: &now,
	}
	require.NoError(t, w.db.Create(&app).Error)
	return app.ID
}

func (w secWorld) newAppOrg(t *testing.T, orgID int) int {
	t.Helper()
	return w.newAppWith(t, orgID, nil, w.senderID)
}

func (w secWorld) newAttachmentNamed(t *testing.T, appID int, atype, name string) int {
	t.Helper()
	att := models.Attachment{ApplicationID: &appID, AttachmentType: atype, AttachmentName: &name}
	require.NoError(t, w.db.Create(&att).Error)
	return att.ID
}

func (w secWorld) setAttachmentDisplayName(t *testing.T, attID int, name string) {
	t.Helper()
	require.NoError(t, w.db.Model(&models.Attachment{}).Where("id = ?", attID).
		Update("attachment_display_name", name).Error)
}

func (w secWorld) setAppNumber(t *testing.T, appID int, number string) {
	t.Helper()
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("application_number", number).Error)
}

func (w secWorld) setAttachmentEntryTime(t *testing.T, attID int, from, to string) {
	t.Helper()
	require.NoError(t, w.db.Model(&models.Attachment{}).Where("id = ?", attID).
		Updates(map[string]interface{}{"entry_time_from": from, "entry_time_to": to}).Error)
}

func (w secWorld) listFiltered(t *testing.T, ctx context.Context, f services.AvailableAttachmentFilters) ([]services.AvailableAttachment, int64) {
	t.Helper()
	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, f, 1, 50)
	require.NoError(t, err)
	return rows, total
}

func TestAvailableAttachments_FilterByType(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)

	app := w.newApp(t, models.ConfirmationApproved)
	carsAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, carsAtt, place)
	itemsAtt := w.newAttachment(t, app, "items")
	w.attachPlace(t, itemsAtt, place)

	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{AttachmentType: secPtrStr("cars")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, carsAtt))
	require.False(t, secContainsAttachment(rows, itemsAtt))

	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{AttachmentType: secPtrStr("items")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, itemsAtt))

	// Невалидный тип игнорируется (не сужает) - возвращаются оба вложения.
	_, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{AttachmentType: secPtrStr("bogus")})
	require.EqualValues(t, 2, total, "невалидный attachment_type не должен сужать выдачу")
}

func TestAvailableAttachments_FilterByOrganization(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	org2 := w.newOrg(t, "Орг 2")

	app1 := w.newApp(t, models.ConfirmationApproved) // org = w.orgID
	att1 := w.newAttachment(t, app1, "cars")
	w.attachPlace(t, att1, place)
	app2 := w.newAppOrg(t, org2)
	att2 := w.newAttachment(t, app2, "cars")
	w.attachPlace(t, att2, place)

	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{OrganizationID: secPtrInt(w.orgID)})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, att1))
	require.False(t, secContainsAttachment(rows, att2), "фильтр по организации скрывает чужую")
}

func TestAvailableAttachments_FilterByCompany(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	comp1 := w.newCompany(t, "Компания 1")
	comp2 := w.newCompany(t, "Компания 2")

	app1 := w.newAppWith(t, w.orgID, secPtrInt(comp1), w.senderID)
	att1 := w.newAttachment(t, app1, "cars")
	w.attachPlace(t, att1, place)
	app2 := w.newAppWith(t, w.orgID, secPtrInt(comp2), w.senderID)
	att2 := w.newAttachment(t, app2, "cars")
	w.attachPlace(t, att2, place)

	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{CompanyID: secPtrInt(comp1)})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, att1))
	require.False(t, secContainsAttachment(rows, att2), "фильтр по компании скрывает чужую")
}

func TestAvailableAttachments_FilterBySearch(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	ivanov := w.newSenderNamed(t, "ivanov", "Иванов", "Иван", "Иванович")

	appA := w.newAppWith(t, w.orgID, nil, ivanov)
	w.setAppNumber(t, appA, "APP-777")
	attA := w.newAttachmentNamed(t, appA, "cars", "Volvo Truck") // латиница для проверки ILIKE-регистра
	w.attachPlace(t, attA, place)

	appB := w.newAppWith(t, w.orgID, nil, w.senderID) // sender1 без ФИО
	w.setAppNumber(t, appB, "APP-100")
	attB := w.newAttachmentNamed(t, appB, "cars", "Lada")
	w.setAttachmentDisplayName(t, attB, "Рефрижератор")
	w.attachPlace(t, attB, place)

	// По номеру заявки.
	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("777")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, attA))
	require.False(t, secContainsAttachment(rows, attB))

	// По имени вложения, регистронезависимо (ILIKE).
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("volvo")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, attA))

	// По отображаемому имени вложения (attachment_display_name).
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("Рефрижератор")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, attB))
	require.False(t, secContainsAttachment(rows, attA))

	// По ФИО отправителя.
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("Иванов")})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, attA))
	require.False(t, secContainsAttachment(rows, attB), "поиск по ФИО не цепляет отправителя без имени")

	// По месту разгрузки (attachment_unload_places) - мощный поиск, старый ILIKE этого не умел.
	// Оба вложения привязаны к "Склад А", поэтому находятся оба.
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("Склад")})
	require.EqualValues(t, 2, total)
	require.True(t, secContainsAttachment(rows, attA))
	require.True(t, secContainsAttachment(rows, attB))

	// Несуществующая подстрока - пусто.
	_, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: secPtrStr("несуществует")})
	require.EqualValues(t, 0, total)
}

func TestAvailableAttachments_EmptyFilterMatchesBaseline(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	app := w.newApp(t, models.ConfirmationApproved)
	for i := 0; i < 3; i++ {
		att := w.newAttachment(t, app, "cars")
		w.attachPlace(t, att, place)
	}

	// nil-поля фильтра = поведение до BE-S6: все совпавшие по месту вложения.
	_, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{})
	require.EqualValues(t, 3, total, "пустой фильтр возвращает всю доступную выдачу")
}

func TestAvailableAttachments_FiltersApplyForSuperAdmin(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// Супер-админ видит все согласованные вложения без гейта мест - места никому не назначаем.
	org2 := w.newOrg(t, "Орг 2")
	app1 := w.newApp(t, models.ConfirmationApproved) // org = w.orgID
	att1 := w.newAttachment(t, app1, "cars")
	app2 := w.newAppOrg(t, org2)
	att2 := w.newAttachment(t, app2, "items")

	// Без фильтра супер-админ видит оба.
	_, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)

	// Фильтр по организации сужает и на пути супер-админа.
	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{OrganizationID: secPtrInt(org2)}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, att2))
	require.False(t, secContainsAttachment(rows, att1))

	// Фильтр по типу так же действует для супер-админа.
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{AttachmentType: secPtrStr("cars")}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, att1))
}

func TestAvailableAttachments_FilterDoesNotBypassPlaceGate(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Мой склад", true)
	otherPlace := w.newUnloadPlace(t, "Чужой склад", true)
	w.assignUnloadPlace(t, myPlace)
	org2 := w.newOrg(t, "Орг 2")

	app1 := w.newApp(t, models.ConfirmationApproved) // org = w.orgID
	mineCars := w.newAttachment(t, app1, "cars")
	w.attachPlace(t, mineCars, myPlace)

	app2 := w.newAppOrg(t, org2)
	foreignCars := w.newAttachment(t, app2, "cars")
	w.attachPlace(t, foreignCars, otherPlace)

	// Фильтр по типу совпадает с обоими, но чужое место остаётся скрытым.
	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{AttachmentType: secPtrStr("cars")})
	require.EqualValues(t, 1, total, "фильтр не обходит гейт мест")
	require.True(t, secContainsAttachment(rows, mineCars))
	require.False(t, secContainsAttachment(rows, foreignCars), "чужое место скрыто несмотря на совпадение типа")

	// Фильтр по организации чужого вложения не раскрывает его - оно вне мест охранника.
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{OrganizationID: secPtrInt(org2)})
	require.EqualValues(t, 0, total, "фильтр по чужой организации не раскрывает вложение вне мест охранника")
	require.False(t, secContainsAttachment(rows, foreignCars))
}

// TestAvailableAttachments_FilterByNight закрепляет границы ночного окна [22:00, 06:00): 22:00
// включается, 06:00 нет; смешанный формат HH:MM/HH:MM:SS считается одинаково; окно через полночь
// и NULL-окно. Граничные времена - главный риск среза (string-сравнение HH:MM в SQL).
func TestAvailableAttachments_FilterByNight(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	app := w.newApp(t, models.ConfirmationApproved)

	cases := []struct {
		name      string
		from, to  string
		wantNight bool
	}{
		{"дневное 09-18 (HH:MM:SS)", "09:00:00", "18:00:00", false},
		{"конец ровно 22:00 - граница включена", "19:00", "22:00", true},
		{"вечер до 22:00 - не ночь", "21:00", "21:59", false},
		{"начало ровно 22:00", "22:00", "23:00", true},
		{"раннее утро до 06:00", "00:00", "05:00", true},
		{"начало ровно 06:00 - граница исключена", "06:00", "08:00", false},
		{"начало 05:59 < 06:00", "05:59", "06:30", true},
		{"через полночь 23-02 (wrap)", "23:00", "02:00", true},
		{"смешанный формат, конец >= 22:00", "08:00:00", "23:00", true},
	}

	type nightCase struct {
		att  int
		name string
		want bool
	}
	ncs := make([]nightCase, 0, len(cases))
	for _, c := range cases {
		att := w.newAttachment(t, app, "cars")
		w.attachPlace(t, att, place)
		w.setAttachmentEntryTime(t, att, c.from, c.to)
		ncs = append(ncs, nightCase{att, c.name, c.wantNight})
	}

	// Вложение без указанного окна (NULL-времена) - не ночь.
	noTimeAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, noTimeAtt, place)

	night := true
	rows, _ := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Night: &night})
	for _, nc := range ncs {
		require.Equalf(t, nc.want, secContainsAttachment(rows, nc.att), "ночной фильтр: %s", nc.name)
	}
	require.False(t, secContainsAttachment(rows, noTimeAtt), "вложение без окна въезда не ночь")

	// Фильтр аддитивен: без него видны все вложения (включая дневные и без окна).
	_, totalAll := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{})
	require.EqualValues(t, len(cases)+1, totalAll, "без ночного фильтра видны все вложения")
}

// TestAvailableAttachments_NightIgnoredDuringSearch - при активном поиске ночной предикат не
// навешивается (поиск отдаёт и ночные, и дневные), как и фильтр "Завершённые".
func TestAvailableAttachments_NightIgnoredDuringSearch(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	app := w.newApp(t, models.ConfirmationApproved)

	dayAtt := w.newAttachmentNamed(t, app, "cars", "Ромашка дневная")
	w.attachPlace(t, dayAtt, place)
	w.setAttachmentEntryTime(t, dayAtt, "09:00", "18:00")

	nightAtt := w.newAttachmentNamed(t, app, "cars", "Ромашка ночная")
	w.attachPlace(t, nightAtt, place)
	w.setAttachmentEntryTime(t, nightAtt, "23:00", "23:30")

	// Без поиска night=true оставляет только ночное.
	night := true
	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Night: &night})
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, nightAtt))
	require.False(t, secContainsAttachment(rows, dayAtt))

	// Поиск отменяет ночной предикат: видны и дневное, и ночное (даже при night=true).
	query := "Ромашка"
	rows, total = w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Search: &query, Night: &night})
	require.EqualValues(t, 2, total, "поиск показывает и ночные, и дневные")
	require.True(t, secContainsAttachment(rows, dayAtt))
	require.True(t, secContainsAttachment(rows, nightAtt))
}

// TestAvailableAttachments_NightDoesNotBypassPlaceGate - ночной фильтр аддитивен поверх гейта мест:
// ночное вложение на ЧУЖОМ месте не раскрывается (по образцу TestAvailableAttachments_FilterDoesNotBypassPlaceGate).
func TestAvailableAttachments_NightDoesNotBypassPlaceGate(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Мой склад", true)
	otherPlace := w.newUnloadPlace(t, "Чужой склад", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	mineNight := w.newAttachment(t, app, "cars")
	w.attachPlace(t, mineNight, myPlace)
	w.setAttachmentEntryTime(t, mineNight, "23:00", "23:30")

	foreignNight := w.newAttachment(t, app, "cars")
	w.attachPlace(t, foreignNight, otherPlace)
	w.setAttachmentEntryTime(t, foreignNight, "23:00", "23:30")

	night := true
	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Night: &night})
	require.EqualValues(t, 1, total, "ночной фильтр не обходит гейт мест")
	require.True(t, secContainsAttachment(rows, mineNight))
	require.False(t, secContainsAttachment(rows, foreignNight), "ночное вложение на чужом месте скрыто")
}

// TestAvailableAttachments_NightAndCompletedCombined - оба тоггла активны (видны в UI рядом):
// SQL даёт status='Завершено' AND ночное окно -> только ночные завершённые.
func TestAvailableAttachments_NightAndCompletedCombined(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)

	doneApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusCompleted)
	activeApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusInWork)

	doneNight := w.newAttachment(t, doneApp, "cars") // завершённая + ночь -> проходит
	w.attachPlace(t, doneNight, place)
	w.setAttachmentEntryTime(t, doneNight, "23:00", "23:30")

	doneDay := w.newAttachment(t, doneApp, "cars") // завершённая, но день -> отсекается night
	w.attachPlace(t, doneDay, place)
	w.setAttachmentEntryTime(t, doneDay, "09:00", "18:00")

	activeNight := w.newAttachment(t, activeApp, "cars") // ночь, но не завершённая -> отсекается completed
	w.attachPlace(t, activeNight, place)
	w.setAttachmentEntryTime(t, activeNight, "23:00", "23:30")

	night, completed := true, true
	rows, total := w.listFiltered(t, ctx, services.AvailableAttachmentFilters{Night: &night, Completed: &completed})
	require.EqualValues(t, 1, total, "night+completed: только ночные завершённые")
	require.True(t, secContainsAttachment(rows, doneNight))
	require.False(t, secContainsAttachment(rows, doneDay))
	require.False(t, secContainsAttachment(rows, activeNight))
}
