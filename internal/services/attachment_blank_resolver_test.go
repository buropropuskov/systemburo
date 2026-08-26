package services

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Одиночные дата и время в бланке идут в том же виде, что и диапазоны (#1454).
// Раньше они отдавались сырыми: в бланке соседствовали "2026-07-15" и
// "15.07.2026 - 17.07.2026", а время приезжало с секундами.
func TestResolveValue_DateTimeFormat(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	bctx := &BlankContext{
		Attachment: &models.Attachment{
			EntryDateFrom: strPtr("2026-07-15"),
			EntryDateTo:   strPtr("2026-07-17"),
			EntryTimeFrom: strPtr("09:00:00"),
			EntryTimeTo:   strPtr("18:30:00"),
		},
		Cars: []models.Car{{
			EntryDateFrom: strPtr("2026-07-15"),
			EntryDateTo:   strPtr("2026-07-17"),
			EntryTimeFrom: strPtr("09:00:00"),
			EntryTimeTo:   strPtr("18:30:00"),
		}},
	}

	cases := []struct {
		path string
		want string
	}{
		{"attachment.entry_date_from", "15.07.2026"},
		{"attachment.entry_date_to", "17.07.2026"},
		{"attachment.entry_time_from", "09:00"},
		{"attachment.entry_time_to", "18:30"},
		{"attachment.entry_date_range", "15.07.2026 - 17.07.2026"},
		{"attachment.entry_time_range", "09:00 - 18:30"},
		{"car.entry_date_from", "15.07.2026"},
		{"car.entry_date_to", "17.07.2026"},
		{"car.entry_time_from", "09:00"},
		{"car.entry_time_to", "18:30"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			require.Equal(t, tc.want, resolveValue(bctx, tc.path, 0))
		})
	}
}

// Дата, пришедшая из БД с временной частью, тоже приводится к дд.мм.гггг,
// а нераспознанное значение остаётся как есть - в бланке лучше исходная строка,
// чем пустая ячейка.
func TestResolveValue_DateFallback(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	withDate := func(v string) *BlankContext {
		return &BlankContext{Attachment: &models.Attachment{EntryDateFrom: strPtr(v)}}
	}
	require.Equal(t, "15.07.2026", resolveValue(withDate("2026-07-15T00:00:00Z"), "attachment.entry_date_from", 0))
	require.Equal(t, "не дата", resolveValue(withDate("не дата"), "attachment.entry_date_from", 0))
	require.Empty(t, resolveValue(withDate(""), "attachment.entry_date_from", 0))
}

// Поля вложения, которые заявитель заполняет в форме, но привязать к бланку было
// нельзя (#1454): название бланка, места разгрузки, крыша и парковка.
func TestResolveValue_AttachmentFields(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	bctx := &BlankContext{
		Attachment: &models.Attachment{
			AttachmentName:        strPtr("avtozayavka_2"),
			AttachmentDisplayName: strPtr("Автозаявка №2"),
			RoofAccess:            true,
			FreeParking:           false,
		},
		AttachmentUnloadPlaces: []string{"Ворота Черепашки", "Склад 4"},
	}

	require.Equal(t, "Автозаявка №2", resolveValue(bctx, "attachment.display_name", 0))
	require.Equal(t, "Ворота Черепашки, Склад 4", resolveValue(bctx, "attachment.unload_places", 0))
	require.Equal(t, "Да", resolveValue(bctx, "attachment.roof_access", 0))
	require.Equal(t, "Нет", resolveValue(bctx, "attachment.free_parking", 0))

	// Без отображаемого имени показываем техническое - пустая ячейка хуже.
	bctx.Attachment.AttachmentDisplayName = nil
	require.Equal(t, "avtozayavka_2", resolveValue(bctx, "attachment.display_name", 0))
}

// Новые пути обязаны быть в словаре: иначе UpdateMappings отклонит привязку,
// которую редактор дал создать.
func TestBuiltinTemplateFields_CoversNewAttachmentPaths(t *testing.T) {
	for _, path := range []string{
		"attachment.display_name",
		"attachment.unload_places",
		"attachment.roof_access",
		"attachment.free_parking",
	} {
		require.True(t, IsValidFieldPath(path), "путь %s должен быть в словаре", path)
	}
}

// Привязки элементов (#1454): полный список мест разгрузки машины вместо
// собранной формой строки "Первое и др.", посты «Проезд» и места прохода.
func TestResolveValue_ItemBindings(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	bctx := &BlankContext{
		Cars: []models.Car{{
			ID:          7,
			CarNumber:   strPtr("О 593 УЕ 325"),
			UnloadPlace: strPtr("Ворота Черепашки и др."),
		}},
		Employees: []models.Employee{{ID: 11, LastName: strPtr("Иванов")}},
		CarUnloadPlaces: map[int][]string{
			7: {"Ворота Черепашки", "Склад 4"},
		},
		CarPassageTables: map[int][]string{
			7: {"ПОСТ №72 (АВТО)"},
		},
		EmployeeTargetTables: map[int][]string{
			11: {"ПОСТ №1", "ПОСТ №3"},
		},
	}

	require.Equal(t, "Ворота Черепашки и др.", resolveValue(bctx, "car.unload_place", 0))
	require.Equal(t, "Ворота Черепашки, Склад 4", resolveValue(bctx, "car.unload_places", 0))
	require.Equal(t, "ПОСТ №72 (АВТО)", resolveValue(bctx, "car.passage_tables", 0))
	require.Equal(t, "ПОСТ №1, ПОСТ №3", resolveValue(bctx, "employee.target_tables", 0))

	// Элемент без привязок отдаёт пустую строку, а не панику по отсутствующему ключу.
	bctx.Cars = append(bctx.Cars, models.Car{ID: 8})
	require.Empty(t, resolveValue(bctx, "car.unload_places", 1))
}

func TestBuiltinTemplateFields_CoversItemBindings(t *testing.T) {
	for _, path := range []string{"car.unload_places", "car.passage_tables", "employee.target_tables"} {
		require.True(t, IsValidFieldPath(path), "путь %s должен быть в словаре", path)
	}
}

// Телефон в бланке печатается как в интерфейсе (#1454), эталон формата -
// frontend/src/composables/usePhoneFormat.js.
func TestFormatPhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"89100530055", "+7 (910) 053 00-55"},
		{"79100530055", "+7 (910) 053 00-55"},
		{"9100530055", "+7 (910) 053 00-55"},
		{"+7 (910) 053 00-55", "+7 (910) 053 00-55"},
		{"", ""},
		{"123", "123"},                     // не похоже на номер - отдаём как есть
		{"+380671234567", "+380671234567"}, // не российский - не трогаем
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, formatPhone(tc.in), "вход %q", tc.in)
	}
}

// Инициатор и контактный телефон берутся из заявки, а у старых заявок (поля пустые) -
// из профиля отправителя.
func TestResolveValue_InitiatorFields(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	sender := &models.User{
		LastName: strPtr("Петров"), FirstName: strPtr("Игорь"), MiddleName: strPtr("Львович"),
		Phone: strPtr("89990001122"),
	}

	withApp := func(app *models.Application) *BlankContext {
		return &BlankContext{Application: app, Sender: sender}
	}

	filled := withApp(&models.Application{
		InitiatorName: strPtr("Сидорова Анна"),
		ContactPhone:  strPtr("89100530055"),
	})
	require.Equal(t, "Сидорова Анна", resolveValue(filled, "application.initiator_name", 0))
	require.Equal(t, "+7 (910) 053 00-55", resolveValue(filled, "application.contact_phone", 0))

	legacy := withApp(&models.Application{})
	require.Equal(t, "Петров Игорь Львович", resolveValue(legacy, "application.initiator_name", 0))
	require.Equal(t, "+7 (999) 000 11-22", resolveValue(legacy, "application.contact_phone", 0))
	require.Equal(t, "+7 (999) 000 11-22", resolveValue(legacy, "application.sender.phone", 0))
}

func TestBuiltinTemplateFields_CoversInitiator(t *testing.T) {
	require.True(t, IsValidFieldPath("application.initiator_name"))
	require.True(t, IsValidFieldPath("application.contact_phone"))
}

// ТМЦ «Заявок на ввоз» в бланке соседнего вложения: списочная секция бланка занята его
// собственным типом, поэтому перечень идёт одной ячейкой построчно.
func TestResolveValue_ApplicationItems(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	bctx := &BlankContext{
		ApplicationItems: []ApplicationItemRow{
			{Name: "Кабель ВВГнг 3х2.5", Count: intPtr(200), SourceName: "Заявка на ввоз"},
			{Name: "Щит распределительный", Count: intPtr(2), SourceName: "Заявка на ввоз"},
			{Name: "Лестница-стремянка", SourceName: "Заявка на ввоз №2"},
		},
	}

	require.Equal(t,
		"Кабель ВВГнг 3х2.5\nЩит распределительный\nЛестница-стремянка",
		resolveValue(bctx, "app_items.names", 0))
	require.Equal(t,
		"Кабель ВВГнг 3х2.5 - 200\nЩит распределительный - 2\nЛестница-стремянка",
		resolveValue(bctx, "app_items.names_with_count", 0),
		"позиция без количества печатается одним наименованием")
	require.Equal(t, "202", resolveValue(bctx, "app_items.total_count", 0))
	require.Equal(t, "3", resolveValue(bctx, "app_items.positions_count", 0))
	require.Equal(t, "Заявка на ввоз, Заявка на ввоз №2", resolveValue(bctx, "app_items.sources", 0),
		"вложения-источники перечисляются без повторов")

	// rowIdx списочной строки на эти поля не влияет: они не списочные.
	require.Equal(t, resolveValue(bctx, "app_items.names", 0), resolveValue(bctx, "app_items.names", 5))
}

// Заявка без «Заявок на ввоз» оставляет ячейку такой, как её задал шаблон: генератор
// пустые значения не пишет.
func TestResolveValue_ApplicationItemsEmpty(t *testing.T) {
	empty := &BlankContext{}
	for _, path := range []string{
		"app_items.names", "app_items.names_with_count",
		"app_items.total_count", "app_items.positions_count", "app_items.sources",
	} {
		require.Empty(t, resolveValue(empty, path, 0), "путь %s на пустом перечне", path)
	}

	// Позиции есть, а количество не заполнено ни у одной - сумма пустая, а не "0".
	noCounts := &BlankContext{ApplicationItems: []ApplicationItemRow{{Name: "Груз"}}}
	require.Empty(t, resolveValue(noCounts, "app_items.total_count", 0))
	require.Equal(t, "1", resolveValue(noCounts, "app_items.positions_count", 0))
}

func TestBuiltinTemplateFields_CoversApplicationItems(t *testing.T) {
	for _, path := range []string{
		"app_items.names", "app_items.names_with_count",
		"app_items.total_count", "app_items.positions_count", "app_items.sources",
	} {
		require.True(t, IsValidFieldPath(path), "путь %s должен быть в словаре", path)
	}
	// Поля обычные: списочная секция бланка принадлежит его собственному типу.
	for _, g := range BuiltinTemplateFields() {
		if g.Group != "app_items" {
			continue
		}
		for _, f := range g.Fields {
			require.False(t, f.IsList, "поле %s не должно быть списочным", f.Path)
		}
	}
}

// Подпись «СОГЛАСОВАНО» в бланке: обязательные согласования перечисляются все,
// необязательные представляет первый согласовавший.
func TestResolveValue_Approvers(t *testing.T) {
	full := []Approver{
		{LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович"},
		{LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Required: true},
		{LastName: "Сидорова", FirstName: "Анна", MiddleName: "Сергеевна", Required: true},
	}
	bctx := &BlankContext{Approvers: full}

	require.Equal(t, "Петров Пётр Петрович, Сидорова Анна Сергеевна",
		resolveValue(bctx, "application.approver_name", 0),
		"под подписью идут все обязательные согласования")
	require.Equal(t, "Петров П. П., Сидорова А. С.",
		resolveValue(bctx, "application.approver_short_name", 0))
	require.Equal(t, "Иванов Иван Иванович, Петров Пётр Петрович, Сидорова Анна Сергеевна",
		resolveValue(bctx, "application.approvers", 0),
		"отдельным полем доступны все согласовавшие")
	require.Equal(t, "Иванов И. И., Петров П. П., Сидорова А. С.",
		resolveValue(bctx, "application.approvers_short", 0))

	// Обязательных нет - подписывает первый согласовавший, остальные только в «всех».
	onlyOptional := &BlankContext{Approvers: []Approver{
		{LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович"},
		{LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович"},
	}}
	require.Equal(t, "Иванов Иван Иванович", resolveValue(onlyOptional, "application.approver_name", 0))
	require.Equal(t, "Иванов И. И.", resolveValue(onlyOptional, "application.approver_short_name", 0))
	require.Equal(t, "Иванов Иван Иванович, Петров Пётр Петрович",
		resolveValue(onlyOptional, "application.approvers", 0))

	// Никто не согласовал - ячейки бланка остаются такими, как их задал шаблон.
	empty := &BlankContext{}
	for _, path := range []string{
		"application.approver_name", "application.approver_short_name",
		"application.approvers", "application.approvers_short",
	} {
		require.Empty(t, resolveValue(empty, path, 0), "путь %s без согласовавших", path)
	}
}

func TestBuiltinTemplateFields_CoversApprovers(t *testing.T) {
	for _, path := range []string{
		"application.approver_name", "application.approver_short_name",
		"application.approvers", "application.approvers_short",
	} {
		require.True(t, IsValidFieldPath(path), "путь %s должен быть в словаре", path)
	}
}

// Транспорт «Автозаявок» в бланке ввоза: под него отведена одна ячейка «Марка и гос.
// номер Т/С», поэтому несколько машин идут в ней по строкам.
func TestResolveValue_ApplicationCars(t *testing.T) {
	bctx := &BlankContext{
		ApplicationCars: []ApplicationCarRow{
			{Number: "О 593 УЕ 325", Mark: "ГАЗель", SourceName: "Автозаявка"},
			{Number: "Х 101 ХХ 777", Mark: "MAN", SourceName: "Автозаявка"},
			{Number: "К 050 УА 902", SourceName: "Автозаявка №2"},
		},
	}

	require.Equal(t, "ГАЗель О 593 УЕ 325\nMAN Х 101 ХХ 777\nК 050 УА 902",
		resolveValue(bctx, "app_cars.marks_numbers", 0),
		"машина без марки печатается одним номером")
	require.Equal(t, "О 593 УЕ 325\nХ 101 ХХ 777\nК 050 УА 902",
		resolveValue(bctx, "app_cars.numbers", 0))
	require.Equal(t, "ГАЗель\nMAN", resolveValue(bctx, "app_cars.marks", 0),
		"пустые марки в перечень не попадают")
	require.Equal(t, "3", resolveValue(bctx, "app_cars.count", 0))
	require.Equal(t, "Автозаявка, Автозаявка №2", resolveValue(bctx, "app_cars.sources", 0))

	// Заявка без автозаявок оставляет ячейку такой, как её задал шаблон.
	empty := &BlankContext{}
	for _, path := range []string{
		"app_cars.marks_numbers", "app_cars.numbers", "app_cars.marks",
		"app_cars.count", "app_cars.sources",
	} {
		require.Empty(t, resolveValue(empty, path, 0), "путь %s без машин", path)
	}
}

func TestBuiltinTemplateFields_CoversApplicationCars(t *testing.T) {
	for _, path := range []string{
		"app_cars.marks_numbers", "app_cars.numbers", "app_cars.marks",
		"app_cars.count", "app_cars.sources",
	} {
		require.True(t, IsValidFieldPath(path), "путь %s должен быть в словаре", path)
	}
	for _, g := range BuiltinTemplateFields() {
		if g.Group != "app_cars" {
			continue
		}
		for _, f := range g.Fields {
			require.False(t, f.IsList, "поле %s не должно быть списочным", f.Path)
		}
	}
}
