package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Защита состава вложения от строк ещё не принятого дополнения (#1685, срез S1). Логики
// подачи дополнения в коде пока нет, поэтому раунд и его строки создаются прямыми Create
// по моделям - тесты стерегут именно читателей состава, а не путь его появления.
//
// Секциями на одном поднятом приложении: отдельные SetupTestApp с CleanDB перебивают
// границу go test -timeout у пакета handlers (та же причина, что в attachment_blank_test).

const suppTestPassword = "supppass_long_enough_for_login"

// suppNewSupplement заводит раунд дополнения заявки в заданном статусе.
func suppNewSupplement(t *testing.T, db *gorm.DB, appID, authorID int, status string) int {
	t.Helper()
	sup := models.ApplicationSupplement{
		ApplicationID:   appID,
		Number:          1,
		Status:          status,
		CreatedByUserID: authorID,
	}
	require.NoError(t, db.Create(&sup).Error)
	return sup.ID
}

// suppNewEmployee создаёт сотрудника вложения; supplementID nil - исходный состав подачи.
func suppNewEmployee(t *testing.T, db *gorm.DB, attID int, lastName string, supplementID *int) int {
	t.Helper()
	ln := lastName
	status := 0
	emp := models.Employee{
		AttachmentID: &attID,
		SupplementID: supplementID,
		LastName:     &ln,
		Status:       &status,
	}
	require.NoError(t, db.Create(&emp).Error)
	return emp.ID
}

// suppEmployeeLastNames вытаскивает фамилии выдачи - по ним и сверяем состав.
func suppEmployeeLastNames(rows []services.EmployeeWithTables) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.LastName)
	}
	return out
}

func TestSupplementGuards(t *testing.T) {
	h := setupSecurityHTTP(t)
	t.Run("охрана видит только допущенный состав", func(t *testing.T) { suppSecurityDetailSection(t, h) })
	t.Run("бланк собирается из допущенного состава", func(t *testing.T) { suppBlankSection(t, h) })
	t.Run("активация обходит непринятое дополнение", func(t *testing.T) { suppActivationSection(t, h) })
	t.Run("завершение по сроку снимает открытое дополнение", func(t *testing.T) { suppExpirySection(t, h) })
	t.Run("непринятое дополнение не открывает вложение охране", func(t *testing.T) { suppVisibilityGateSection(t, h) })
	t.Run("отзыв заявки снимает открытое дополнение", func(t *testing.T) { suppWithdrawSection(t, h) })
}

// Гейт видимости "Доступные мне" пускает people-вложение к охраннику по пересечению постов
// его сотрудников с назначенными охраннику. Пост новой строке назначается сразу при подаче
// дополнения, поэтому без фильтра по статусу раунда непринятый сотрудник открывал бы охране
// вложение целиком - вместе с заголовком, организацией и сроками заявки, к посту которой ни
// один ДОПУЩЕННЫЙ человек не относится. Фильтр в детали от этого не спасает: она отдаёт
// пустой состав, но сам факт заявки уже раскрыт.
func suppVisibilityGateSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	table := w.newPeopleTable(t, "supp-gate-post")
	w.assignTable(t, table)

	appID := w.newApp(t, models.ConfirmationApproved)
	attID := w.newAttachment(t, appID, "people")

	// К посту охранника привязан ТОЛЬКО сотрудник непринятого раунда.
	pendingSup := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)
	pendingID := suppNewEmployee(t, w.db, attID, "Скрытый", &pendingSup)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: pendingID, TableID: table}).Error)

	rec := testutil.GET(t, h.e, "/applications/available-attachments", testutil.AuthHeader(h.guardToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, row := range testutil.ParseResponse[[]services.AvailableAttachment](t, rec) {
		require.NotEqual(t, attID, row.AttachmentID,
			"вложение, которое открывается только непринятым дополнением, охране не показывается")
	}

	// И прямой заход по идентификатору тоже закрыт - гейт доступа тот же предикат.
	rec = testutil.GET(t, h.e, fmt.Sprintf("/applications/available-attachments/%d", attID), testutil.AuthHeader(h.guardToken))
	require.Equal(t, http.StatusForbidden, rec.Code,
		"деталь вложения по прямой ссылке тоже закрыта: %s", rec.Body.String())

	// Как только раунд принят, вложение открывается штатно - фильтр держится на статусе,
	// а не на самом факте принадлежности строки дополнению.
	require.NoError(t, w.db.Model(&models.ApplicationSupplement{}).Where("id = ?", pendingSup).
		Update("status", models.SupplementAccepted).Error)
	require.ElementsMatch(t, []string{"Скрытый"}, suppEmployeeLastNames(secGetDetail(t, h, attID, h.guardToken).Employees),
		"принятое дополнение открывает вложение охране")
}

// Отзыв заявки отправителем - терминальный переход без обратного пути. Открытому раунду
// после него идти некуда: принимать его некому, а у согласующих он остался бы вечной задачей.
func suppWithdrawSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	appID := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusInWork)
	supID := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)

	rec := testutil.POST(t, h.e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, "отзыв заявки автором: %s", rec.Body.String())

	var sup models.ApplicationSupplement
	require.NoError(t, w.db.First(&sup, supID).Error)
	require.Equal(t, models.SupplementCancelled, sup.Status, "отзыв заявки снимает открытый раунд")
}

// Деталь "Доступные мне": сотрудники непринятого дополнения охране не видны, исходный
// состав и принятое дополнение - видны. Через эту же выборку идут серия и номер паспорта
// и номер патента, поэтому лишняя строка здесь - утечка персональных данных, а не просто
// преждевременный показ. Карточка самой заявки (контраст в конце) отдаёт всё.
func suppSecurityDetailSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	table := w.newPeopleTable(t, "supp-post")
	w.assignTable(t, table)

	appID := w.newApp(t, models.ConfirmationApproved)
	attID := w.newAttachment(t, appID, "people")

	// Исходный сотрудник с привязкой к посту - через неё охранник и получает вложение.
	originalID := suppNewEmployee(t, w.db, attID, "Первичный", nil)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: originalID, TableID: table}).Error)

	pendingSup := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)
	pendingID := suppNewEmployee(t, w.db, attID, "Непринятый", &pendingSup)
	// Пост дополнению уже назначен: фильтр обязан держаться на статусе раунда, а не на
	// том, что у новой строки якобы нет привязок.
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: pendingID, TableID: table}).Error)

	acceptedSup := models.ApplicationSupplement{
		ApplicationID: appID, Number: 2, Status: models.SupplementAccepted, CreatedByUserID: w.senderID,
	}
	require.NoError(t, w.db.Create(&acceptedSup).Error)
	suppNewEmployee(t, w.db, attID, "Принятый", &acceptedSup.ID)

	detail := secGetDetail(t, h, attID, h.guardToken)
	require.ElementsMatch(t, []string{"Первичный", "Принятый"}, suppEmployeeLastNames(detail.Employees),
		"охране уходит исходный состав и принятое дополнение, непринятое - нет")

	// Контраст: карточка заявки у автора показывает и непринятое - он только что его
	// добавил и должен видеть, что добавка ушла на согласование.
	rec := testutil.GET(t, h.e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	card := testutil.ParseResponse[[]services.EmployeeWithTables](t, rec)
	require.ElementsMatch(t, []string{"Первичный", "Непринятый", "Принятый"}, suppEmployeeLastNames(card),
		"автор заявки видит весь состав вложения, включая непринятое дополнение")
}

// Бланк пропуска - документ допуска: его печатают и несут на пост. Строка непринятого
// дополнения в нём означала бы проход мимо согласования.
func suppBlankSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	name := "people_supp_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, w.db.Create(&ua).Error)
	// blankSeedTemplate кладёт номер заявки в A1 и списочное employee.last_name с B5.
	blankSeedTemplate(t, w.db, ua.ID)
	t.Cleanup(func() {
		w.db.Where("unique_attachment_id = ?", ua.ID).Delete(&models.AttachmentTemplate{})
	})
	// Второй перечень того же бланка - ТМЦ «Заявок на ввоз» этой заявки: он собирается
	// отдельным запросом, поэтому проверяется отдельной ячейкой.
	var tplID int
	require.NoError(t, w.db.Table("attachment_templates").
		Where("unique_attachment_id = ?", ua.ID).Select("id").Scan(&tplID).Error)
	require.NoError(t, w.db.Create(&models.AttachmentTemplateMapping{
		TemplateID: tplID, CellRef: "C1", FieldPath: "app_items.names",
	}).Error)

	appID := w.newApp(t, models.ConfirmationApproved)
	att := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
	require.NoError(t, w.db.Create(&att).Error)

	suppNewEmployee(t, w.db, att.ID, "Бланковый", nil)
	pendingSup := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)
	suppNewEmployee(t, w.db, att.ID, "Недопущенный", &pendingSup)

	itemsAtt := models.Attachment{ApplicationID: &appID, AttachmentType: "items"}
	require.NoError(t, w.db.Create(&itemsAtt).Error)
	admittedItem, pendingItem := "Ящик", "Коробка"
	require.NoError(t, w.db.Create(&models.Item{AttachmentID: itemsAtt.ID, Name: &admittedItem}).Error)
	require.NoError(t, w.db.Create(&models.Item{
		AttachmentID: itemsAtt.ID, Name: &pendingItem, SupplementID: &pendingSup,
	}).Error)

	data := generateBlankBytes(t, services.NewAttachmentBlankService(w.db), appID, att.ID)
	out, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()

	sheet := out.GetSheetName(0)
	first, err := out.GetCellValue(sheet, "B5")
	require.NoError(t, err)
	require.Equal(t, "Бланковый", first, "исходный состав в бланк попадает")
	second, err := out.GetCellValue(sheet, "B6")
	require.NoError(t, err)
	require.Empty(t, second, "сотрудник непринятого дополнения в бланк допуска попасть не должен")

	items, err := out.GetCellValue(sheet, "C1")
	require.NoError(t, err)
	require.Equal(t, admittedItem, items, "перечень ТМЦ заявки в бланке тоже без непринятого дополнения")
}

// Принятие заявки в работу активирует строки (status->1, после чего они видны на КПП).
// Раундов оно не различает, поэтому без фильтра оживило бы и непринятое дополнение -
// в обход согласования.
func suppActivationSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	approverToken := testutil.RegisterAndLogin(t, h.e, "suppapprover", suppTestPassword, 1, w.orgID, 0)
	makeApprover(t, w.db, "suppapprover")
	approverID := secUserIDByUsername(t, w.db, "suppapprover")

	appID := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusProcessing)
	attID := w.newAttachment(t, appID, "people")
	originalID := suppNewEmployee(t, w.db, attID, "Активируемый", nil)
	pendingSup := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)
	pendingID := suppNewEmployee(t, w.db, attID, "Ожидающий", &pendingSup)

	body := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID)
	rec := testutil.POST(t, h.e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "принять в работу: %s", rec.Body.String())

	var original, pending models.Employee
	require.NoError(t, w.db.First(&original, originalID).Error)
	require.NoError(t, w.db.First(&pending, pendingID).Error)
	require.NotNil(t, original.Status)
	require.Equal(t, 1, *original.Status, "исходный состав активируется принятием в работу")
	require.NotNil(t, pending.Status)
	require.Equal(t, 0, *pending.Status, "строка непринятого дополнения активироваться не должна")
}

// Заявка ушла в "Завершено" по истечении срока вложений: открытому раунду идти некуда,
// принимать его будет некому. Без снятия pending остался бы на закрытой заявке навсегда.
func suppExpirySection(t *testing.T, h secHTTPWorld) {
	w := h.w
	appID := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusInWork)

	status, yesterday := 1, time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	att := models.Attachment{
		ApplicationID: &appID, AttachmentType: "people",
		EntryDateTo: &yesterday, Status: &status,
	}
	require.NoError(t, w.db.Create(&att).Error)

	supID := suppNewSupplement(t, w.db, appID, w.senderID, models.SupplementPending)

	require.NoError(t, w.svc.CheckExpiredAttachments(context.Background()))

	var app models.Application
	require.NoError(t, w.db.First(&app, appID).Error)
	require.NotNil(t, app.Status)
	require.Equal(t, models.StatusCompleted, *app.Status, "заявка с истёкшими вложениями завершается")

	var sup models.ApplicationSupplement
	require.NoError(t, w.db.First(&sup, supID).Error)
	require.Equal(t, models.SupplementCancelled, sup.Status, "открытое дополнение закрытой заявки снимается")

	var logged int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityApplication, appID, models.AuditActionSupplementCancelled).
		Count(&logged).Error)
	require.EqualValues(t, 1, logged, "снятие раунда попадает в историю заявки")
}
