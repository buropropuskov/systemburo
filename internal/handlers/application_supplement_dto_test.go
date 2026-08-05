package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Признак дополнения в ответах API (#1685, срез S5). Без него интерфейс не отличает строку,
// добавленную дополнением, от исходного состава подачи, и не видит, что по заявке идёт
// повторный круг: статус и согласование самой заявки дополнение намеренно не двигает.
//
// Раунды и их строки заводятся прямыми Create по моделям - так же, как в
// application_supplement_guards_test: проверяются читатели состава, а не путь его появления.
//
// Секциями на одном поднятом приложении: отдельные SetupTestApp с CleanDB перебивают границу
// go test -timeout у пакета handlers.

// suppDetail - интересующий срез ответа GET /applications/:id/details (сервис отдаёт map,
// экспортируемого типа у ответа нет).
type suppDetail struct {
	OpenSupplement   *services.OpenSupplementInfo `json:"open_supplement"`
	SupplementsCount int                          `json:"supplements_count"`
}

// suppRound заводит раунд дополнения заданного номера и статуса.
func suppRound(t *testing.T, db *gorm.DB, appID, authorID, number int, status string, comment *string) int {
	t.Helper()
	sup := models.ApplicationSupplement{
		ApplicationID:   appID,
		Number:          number,
		Status:          status,
		Comment:         comment,
		CreatedByUserID: authorID,
	}
	require.NoError(t, db.Create(&sup).Error)
	return sup.ID
}

// suppNewCar создаёт машину вложения; supplementID nil - исходный состав подачи.
func suppNewCar(t *testing.T, db *gorm.DB, attID int, number string, supplementID *int) {
	t.Helper()
	num, status := number, 0
	require.NoError(t, db.Create(&models.Car{
		AttachmentID: attID,
		SupplementID: supplementID,
		CarNumber:    &num,
		Status:       &status,
	}).Error)
}

// suppNewItem создаёт позицию ТМЦ вложения; supplementID nil - исходный состав подачи.
func suppNewItem(t *testing.T, db *gorm.DB, attID int, name string, supplementID *int) {
	t.Helper()
	n, count := name, 1
	require.NoError(t, db.Create(&models.Item{
		AttachmentID: attID,
		SupplementID: supplementID,
		Name:         &n,
		Count:        &count,
	}).Error)
}

// suppGetDetail читает деталь заявки под указанным токеном.
func suppGetDetail(t *testing.T, h secHTTPWorld, appID int, token string) suppDetail {
	t.Helper()
	rec := testutil.GET(t, h.e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[suppDetail](t, rec)
}

// suppFindApp достаёт заявку из листинга по идентификатору.
func suppFindApp(t *testing.T, rows []services.ApplicationWithDetails, appID int) services.ApplicationWithDetails {
	t.Helper()
	for _, row := range rows {
		if row.ID == appID {
			return row
		}
	}
	t.Fatalf("заявка %d не найдена в листинге", appID)
	return services.ApplicationWithDetails{}
}

func TestSupplementDTO(t *testing.T) {
	h := setupSecurityHTTP(t)
	t.Run("состав вложения помечен раундом дополнения", func(t *testing.T) { suppMarkSection(t, h) })
	t.Run("деталь заявки отдаёт открытый раунд и число раундов", func(t *testing.T) { suppOpenRoundSection(t, h) })
	t.Run("деталь заявки без открытого раунда отдаёт null", func(t *testing.T) { suppClosedRoundSection(t, h) })
	t.Run("листинг заявок помечает открытый раунд", func(t *testing.T) { suppListSection(t, h) })
	t.Run("охране непринятое не уходит, принятое приходит с номером", func(t *testing.T) { suppSecurityMarkSection(t, h) })
}

// Три ручки состава вложения на одном наборе строк: исходная подача, непринятый раунд,
// принятый раунд. Проверяются все четыре поля признака - номер и статус приезжают JOIN-ом
// того же запроса, поэтому промах в нём виден именно здесь.
func suppMarkSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	appID := w.newApp(t, models.ConfirmationApproved)
	carsAtt := w.newAttachment(t, appID, "cars")
	peopleAtt := w.newAttachment(t, appID, "people")
	itemsAtt := w.newAttachment(t, appID, "items")

	pending := suppRound(t, w.db, appID, w.senderID, 1, models.SupplementPending, nil)
	accepted := suppRound(t, w.db, appID, w.senderID, 2, models.SupplementAccepted, nil)

	suppNewCar(t, w.db, carsAtt, "A100AA777", nil)
	suppNewCar(t, w.db, carsAtt, "B200BB777", &pending)
	suppNewCar(t, w.db, carsAtt, "C300CC777", &accepted)

	suppNewEmployee(t, w.db, peopleAtt, "Исходный", nil)
	suppNewEmployee(t, w.db, peopleAtt, "Непринятый", &pending)
	suppNewEmployee(t, w.db, peopleAtt, "Принятый", &accepted)

	suppNewItem(t, w.db, itemsAtt, "Исходная позиция", nil)
	suppNewItem(t, w.db, itemsAtt, "Непринятая позиция", &pending)
	suppNewItem(t, w.db, itemsAtt, "Принятая позиция", &accepted)

	// assertMark сверяет признак строки с ожидаемым раундом. wantID nil - исходный состав.
	assertMark := func(label string, mark services.SupplementMark, wantID *int, wantNumber int, wantStatus string, wantPending bool) {
		t.Helper()
		if wantID == nil {
			assert.Nil(t, mark.SupplementID, "%s: исходный состав подачи без раунда", label)
			assert.Nil(t, mark.SupplementNumber, "%s: номера раунда у исходного состава нет", label)
			assert.Nil(t, mark.SupplementStatus, "%s: статуса раунда у исходного состава нет", label)
			assert.False(t, mark.IsPending, "%s: исходный состав допущен", label)
			return
		}
		require.NotNil(t, mark.SupplementID, "%s: раунд у строки должен быть", label)
		assert.Equal(t, *wantID, *mark.SupplementID, "%s: идентификатор раунда", label)
		require.NotNil(t, mark.SupplementNumber, "%s: номер раунда должен приехать JOIN-ом", label)
		assert.Equal(t, wantNumber, *mark.SupplementNumber, "%s: номер раунда", label)
		require.NotNil(t, mark.SupplementStatus, "%s: статус раунда должен приехать JOIN-ом", label)
		assert.Equal(t, wantStatus, *mark.SupplementStatus, "%s: статус раунда", label)
		assert.Equal(t, wantPending, mark.IsPending, "%s: признак «ещё не допущено»", label)
	}

	rec := testutil.GET(t, h.e, fmt.Sprintf("/attachments/%d/cars", carsAtt), testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cars := testutil.ParseResponse[[]services.CarWithPlaces](t, rec)
	require.Len(t, cars, 3, "автору видны все три машины")
	byNumber := make(map[string]services.SupplementMark, len(cars))
	for _, c := range cars {
		byNumber[c.CarNumber] = c.SupplementMark
	}
	assertMark("машина исходной подачи", byNumber["A100AA777"], nil, 0, "", false)
	assertMark("машина непринятого раунда", byNumber["B200BB777"], &pending, 1, models.SupplementPending, true)
	assertMark("машина принятого раунда", byNumber["C300CC777"], &accepted, 2, models.SupplementAccepted, false)

	rec = testutil.GET(t, h.e, fmt.Sprintf("/attachments/%d/employees", peopleAtt), testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	employees := testutil.ParseResponse[[]services.EmployeeWithTables](t, rec)
	require.Len(t, employees, 3, "автору видны все три строки состава")
	byLastName := make(map[string]services.SupplementMark, len(employees))
	for _, e := range employees {
		byLastName[e.LastName] = e.SupplementMark
	}
	assertMark("сотрудник исходной подачи", byLastName["Исходный"], nil, 0, "", false)
	assertMark("сотрудник непринятого раунда", byLastName["Непринятый"], &pending, 1, models.SupplementPending, true)
	assertMark("сотрудник принятого раунда", byLastName["Принятый"], &accepted, 2, models.SupplementAccepted, false)

	// Форма JSON, а не только распаковка в Go-тип: признак встроен анонимной структурой, и
	// разбор ответа обратно в тот же тип не отличил бы плоские ключи от вложенного объекта.
	// Фронту нужны именно плоские - на них он и завязывается.
	raw := testutil.ParseResponse[[]map[string]any](t, rec)
	require.NotEmpty(t, raw)
	for _, key := range []string{"supplement_id", "supplement_number", "supplement_status", "is_pending"} {
		assert.Contains(t, raw[0], key, "признак дополнения лежит в самой строке состава")
	}
	assert.NotContains(t, raw[0], "SupplementMark", "встроенная структура не должна всплывать объектом")

	rec = testutil.GET(t, h.e, fmt.Sprintf("/attachments/%d/items", itemsAtt), testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	items := testutil.ParseResponse[[]services.ItemInfo](t, rec)
	require.Len(t, items, 3, "автору видны все три позиции ТМЦ")
	byName := make(map[string]services.SupplementMark, len(items))
	for _, i := range items {
		byName[i.Name] = i.SupplementMark
	}
	assertMark("позиция исходной подачи", byName["Исходная позиция"], nil, 0, "", false)
	assertMark("позиция непринятого раунда", byName["Непринятая позиция"], &pending, 1, models.SupplementPending, true)
	assertMark("позиция принятого раунда", byName["Принятая позиция"], &accepted, 2, models.SupplementAccepted, false)
}

// Открытый раунд в детали заявки: счётчики считают реально добавленное по типам, закрытый
// раунд в open_supplement не попадает, но в общее число раундов входит.
func suppOpenRoundSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	appID := w.newApp(t, models.ConfirmationApproved)
	carsAtt := w.newAttachment(t, appID, "cars")
	peopleAtt := w.newAttachment(t, appID, "people")
	itemsAtt := w.newAttachment(t, appID, "items")

	// Закрытый раунд заявки: в open_supplement ему места нет, в supplements_count - есть.
	closed := suppRound(t, w.db, appID, w.senderID, 1, models.SupplementAccepted, nil)
	suppNewCar(t, w.db, carsAtt, "D400DD777", &closed)

	comment := "Добавили водителя и погрузчик"
	open := suppRound(t, w.db, appID, w.senderID, 2, models.SupplementPending, &comment)
	suppNewCar(t, w.db, carsAtt, "E500EE777", &open)
	suppNewEmployee(t, w.db, peopleAtt, "Добавленный", &open)
	suppNewEmployee(t, w.db, peopleAtt, "Ещё один", &open)

	// Строки вне раунда в счётчики попадать не должны - ни исходный состав, ни чужой раунд.
	suppNewItem(t, w.db, itemsAtt, "Исходная позиция", nil)
	suppNewCar(t, w.db, carsAtt, "F600FF777", nil)

	detail := suppGetDetail(t, h, appID, h.userToken)
	require.NotNil(t, detail.OpenSupplement, "по заявке идёт раунд - деталь обязана его отдать")
	assert.Equal(t, open, detail.OpenSupplement.ID)
	assert.Equal(t, 2, detail.OpenSupplement.Number, "номер раунда для подписи «Дополнение №2»")
	assert.Equal(t, models.SupplementPending, detail.OpenSupplement.Status)
	require.NotNil(t, detail.OpenSupplement.Comment)
	assert.Equal(t, comment, *detail.OpenSupplement.Comment)
	assert.Equal(t, w.senderID, detail.OpenSupplement.CreatedByUserID)
	assert.NotEmpty(t, detail.OpenSupplement.CreatedByName, "автор раунда подписан")
	assert.False(t, detail.OpenSupplement.CreatedAt.IsZero(), "время подачи раунда заполнено")
	assert.Equal(t, services.SupplementCounts{Vehicles: 1, Employees: 2, Items: 0},
		detail.OpenSupplement.Counts, "счётчики считают строки этого раунда, а не всё вложение")
	assert.Equal(t, 2, detail.SupplementsCount, "закрытый раунд в общее число входит")
}

// Раунда нет либо он закрылся - деталь отдаёт null, а не последний известный раунд.
func suppClosedRoundSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	appID := w.newApp(t, models.ConfirmationApproved)

	detail := suppGetDetail(t, h, appID, h.userToken)
	assert.Nil(t, detail.OpenSupplement, "у заявки без дополнений открытого раунда нет")
	assert.Equal(t, 0, detail.SupplementsCount)

	supID := suppRound(t, w.db, appID, w.senderID, 1, models.SupplementPending, nil)
	detail = suppGetDetail(t, h, appID, h.userToken)
	require.NotNil(t, detail.OpenSupplement, "поданный раунд открыт")

	// Принятие закрывает раунд: карточка возвращается к «повторного круга нет».
	require.NoError(t, w.db.Model(&models.ApplicationSupplement{}).Where("id = ?", supID).
		Update("status", models.SupplementAccepted).Error)
	detail = suppGetDetail(t, h, appID, h.userToken)
	assert.Nil(t, detail.OpenSupplement, "принятый раунд открытым не считается")
	assert.Equal(t, 1, detail.SupplementsCount, "но из числа раундов не исчезает")

	// Снятый раунд тоже закрыт - иначе метка висела бы на отозванной добавке.
	require.NoError(t, w.db.Model(&models.ApplicationSupplement{}).Where("id = ?", supID).
		Update("status", models.SupplementCancelled).Error)
	detail = suppGetDetail(t, h, appID, h.userToken)
	assert.Nil(t, detail.OpenSupplement, "снятый раунд открытым не считается")
}

// has_open_supplement в листингах: и в кабинете автора, и в Центре. Метка нужна там, где
// статус заявки повторный круг никак не выдаёт.
func suppListSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	quiet := w.newApp(t, models.ConfirmationApproved)
	noisy := w.newApp(t, models.ConfirmationApproved)
	supID := suppRound(t, w.db, noisy, w.senderID, 1, models.SupplementPending, nil)

	rec := testutil.GET(t, h.e, "/applications/user", testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseResponse[[]services.ApplicationWithDetails](t, rec)
	assert.False(t, suppFindApp(t, rows, quiet).HasOpenSupplement, "заявка без дополнений метки не несёт")
	assert.True(t, suppFindApp(t, rows, noisy).HasOpenSupplement, "по заявке идёт раунд - кабинет это показывает")

	// Центр заявок: тот же список столбцов, другой фильтр видимости. Чужие заявки в Центре
	// видит принимающий - без записи в application_approvers список админа пуст.
	makeApprover(t, w.db, "testadmin")
	rec = testutil.GET(t, h.e, "/applications", testutil.AuthHeader(h.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseResponse[[]services.ApplicationWithDetails](t, rec)
	assert.True(t, suppFindApp(t, rows, noisy).HasOpenSupplement, "Центр показывает идущий раунд")

	// Решение по раунду снимает метку - иначе «идёт повторный круг» висело бы вечно.
	require.NoError(t, w.db.Model(&models.ApplicationSupplement{}).Where("id = ?", supID).
		Update("status", models.SupplementRefused).Error)
	rec = testutil.GET(t, h.e, "/applications/user", testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseResponse[[]services.ApplicationWithDetails](t, rec)
	assert.False(t, suppFindApp(t, rows, noisy).HasOpenSupplement, "закрытый раунд метку снимает")
}

// Признак дополнения едет и охране, но только на допущенных строках: срез у неё сужен, и
// новое поле его не расширяет. Проверка стоит здесь, потому что DTO у обоих читателей общий -
// добавленные колонки попали в security-путь заодно.
func suppSecurityMarkSection(t *testing.T, h secHTTPWorld) {
	w := h.w
	table := w.newPeopleTable(t, "supp-dto-post")
	w.assignTable(t, table)

	appID := w.newApp(t, models.ConfirmationApproved)
	attID := w.newAttachment(t, appID, "people")

	originalID := suppNewEmployee(t, w.db, attID, "Допущенный", nil)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: originalID, TableID: table}).Error)

	pending := suppRound(t, w.db, appID, w.senderID, 1, models.SupplementPending, nil)
	pendingID := suppNewEmployee(t, w.db, attID, "Ожидающий", &pending)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: pendingID, TableID: table}).Error)

	accepted := suppRound(t, w.db, appID, w.senderID, 2, models.SupplementAccepted, nil)
	acceptedID := suppNewEmployee(t, w.db, attID, "Принятый", &accepted)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: acceptedID, TableID: table}).Error)

	detail := secGetDetail(t, h, attID, h.guardToken)
	require.ElementsMatch(t, []string{"Допущенный", "Принятый"}, suppEmployeeLastNames(detail.Employees),
		"новые поля выдачу охране не расширили")
	for _, emp := range detail.Employees {
		assert.False(t, emp.IsPending, "охране непринятых строк не достаётся: %s", emp.LastName)
		if emp.LastName == "Принятый" {
			require.NotNil(t, emp.SupplementNumber)
			assert.Equal(t, 2, *emp.SupplementNumber, "принятое дополнение подписано номером раунда")
		}
	}
}
