package main

import (
	"fmt"
	"log"
	"time"

	"systemburo/internal/normalize"

	"gorm.io/gorm"
)

// seedDemoData создаёт полный демо-набор для ручного QA и e2e: справочники,
// заявки в разных статусах (включая архивные), вложения, машины, сотрудники,
// история, уведомления и объявления. Идемпотентно: проверки WHERE NOT EXISTS
// и DEMO/* номера заявок гарантируют отсутствие дубликатов при повторе.
//
// См. issue #84 - покрывает acceptance criteria для разблокировки UI-сценариев.
func seedDemoData(db *gorm.DB, orgID, compID, userID int) {
	orgIDs, compIDs, placeNames, err := seedExtendedDictionaries(db, orgID, compID)
	if err != nil {
		log.Printf("demo seed: dictionaries failed: %v", err)
		return
	}
	uaIDs := seedUniqueAttachments(db)
	if uaIDs.cars == 0 {
		log.Printf("demo seed: no cars unique attachment, aborting")
		return
	}
	seedNewsAndAnnouncements(db, userID)
	seedNotifications(db, userID)
	seedExtendedApplications(db, orgIDs, compIDs, userID, uaIDs, placeNames)
	fmt.Println("Demo data seeded (extended).")
}

// uniqueAttachmentIDs хранит шаблоны вложений по типу.
type uniqueAttachmentIDs struct {
	cars   int
	people int
	items  int
}

func seedExtendedDictionaries(db *gorm.DB, defaultOrgID, defaultCompID int) ([]int, []int, []string, error) {
	// Организации: оставляем дефолтную + добавляем демо-3 если их нет.
	orgNames := []string{"ООО Демо-Партнёр", "АО Демо-Логистика", "ИП Иванов И.И."}
	orgIDs := []int{defaultOrgID}
	for _, name := range orgNames {
		var id int
		db.Raw(`SELECT id FROM organizations WHERE name = ? LIMIT 1`, name).Scan(&id)
		if id == 0 {
			// name_normalized пишем сразу: INSERT в обход gorm-модели хук BeforeSave не
			// зовёт, а без ключа запись не участвует в дедупликации наименований (#1437)
			// до следующего запуска сервера с бэкфиллом.
			if err := db.Raw(`INSERT INTO organizations (name, name_normalized) VALUES (?, ?) RETURNING id`,
				name, normalize.OrgName(name)).Scan(&id).Error; err != nil {
				// Наименование, схлопнувшееся по ключу с существующим, отобьёт уникальный
				// индекс - молча пропускать такую организацию нельзя, дальше на неё
				// ссылаются демо-заявки.
				log.Printf("demo seed: организация %q не создана: %v", name, err)
			}
		}
		if id != 0 {
			orgIDs = append(orgIDs, id)
		}
	}

	compNames := []string{"ООО Демо-Сервис", "АО Демо-Транс", "ИП Петров П.П."}
	compIDs := []int{defaultCompID}
	for _, name := range compNames {
		var id int
		db.Raw(`SELECT id FROM companies WHERE name = ? LIMIT 1`, name).Scan(&id)
		if id == 0 {
			if err := db.Raw(`INSERT INTO companies (name, name_normalized) VALUES (?, ?) RETURNING id`,
				name, normalize.OrgName(name)).Scan(&id).Error; err != nil {
				log.Printf("demo seed: компания %q не создана: %v", name, err)
			}
		}
		if id != 0 {
			compIDs = append(compIDs, id)
		}
	}

	// Места разгрузки: 5 шт для тестирования селекторов.
	placeNames := []string{"Склад №1", "Склад №2", "Рампа А", "Рампа Б", "Северный въезд"}
	for _, name := range placeNames {
		if err := db.Exec(`
			INSERT INTO unload_places (name)
			SELECT ? WHERE NOT EXISTS (SELECT 1 FROM unload_places WHERE name = ?)
		`, name, name).Error; err != nil {
			return nil, nil, nil, fmt.Errorf("unload_places %q: %w", name, err)
		}
	}

	// License plate formats: 3 региональных формата.
	// Колонки модели: name, country_code, icon, is_active, is_default.
	lpfSpecs := []struct {
		name        string
		countryCode string
	}{
		{"Россия", "RU"},
		{"Беларусь", "BY"},
		{"Казахстан", "KZ"},
	}
	for _, s := range lpfSpecs {
		db.Exec(`
			INSERT INTO license_plate_formats (name, country_code, is_active)
			SELECT ?, ?, true WHERE NOT EXISTS (SELECT 1 FROM license_plate_formats WHERE name = ?)
		`, s.name, s.countryCode, s.name)
	}

	return orgIDs, compIDs, placeNames, nil
}

// seedUniqueAttachments создаёт три шаблона: cars/people/items. Возвращает их ID.
func seedUniqueAttachments(db *gorm.DB) uniqueAttachmentIDs {
	templates := []struct {
		attachmentType string
		name           string
		displayName    string
		title          string
	}{
		{"cars", "cars_demo", "Автомобили (демо)", "Автомобили"},
		{"people", "people_demo", "Сотрудники (демо)", "Сотрудники"},
		{"items", "items_demo", "Имущество (демо)", "Имущество"},
	}
	for _, t := range templates {
		db.Exec(`
			INSERT INTO unique_attachments (attachment_type, name, display_name, title, is_active)
			SELECT ?, ?, ?, ?, true
			WHERE NOT EXISTS (SELECT 1 FROM unique_attachments WHERE name = ?)
		`, t.attachmentType, t.name, t.displayName, t.title, t.name)
	}
	var ids uniqueAttachmentIDs
	db.Raw(`SELECT id FROM unique_attachments WHERE name = 'cars_demo' LIMIT 1`).Scan(&ids.cars)
	db.Raw(`SELECT id FROM unique_attachments WHERE name = 'people_demo' LIMIT 1`).Scan(&ids.people)
	db.Raw(`SELECT id FROM unique_attachments WHERE name = 'items_demo' LIMIT 1`).Scan(&ids.items)
	return ids
}

func seedNewsAndAnnouncements(db *gorm.DB, userID int) {
	// Новости: 2 шт.
	newsSpecs := []struct {
		title       string
		description string
		fullText    string
	}{
		{"Демо-новость", "Это пример новости для /news", "Полный текст демо-новости. Создано cmd/seed-demo."},
		{"Запуск нового склада", "Открыт склад №3 в северной части территории", "Подробности про новый склад. График работы 24/7."},
	}
	for _, n := range newsSpecs {
		db.Exec(`
			INSERT INTO news (title, description, full_text, created_by, is_active)
			SELECT ?, ?, ?, ?, true
			WHERE NOT EXISTS (SELECT 1 FROM news WHERE title = ?)
		`, n.title, n.description, n.fullText, userID, n.title)
	}

	// Объявления: одно активное + одно архивное.
	now := time.Now()
	annSpecs := []struct {
		title       string
		description string
		fullText    string
		isImportant bool
		isActive    bool
	}{
		{"Важное демо-объявление", "Это активное объявление для проверки UI", "Полный текст объявления. Видно в /news и в шапке.", true, true},
		{"Архивное объявление", "Объявление, которое уже скрыто", "Это объявление осталось в истории как пример архивного.", false, false},
	}
	for _, a := range annSpecs {
		var exists int64
		db.Raw(`SELECT COUNT(*) FROM announcements WHERE title = ?`, a.title).Scan(&exists)
		if exists > 0 {
			continue
		}
		if a.isActive {
			db.Exec(`
				INSERT INTO announcements (title, description, full_text, is_important, is_active, created_by, activated_at, activated_by)
				VALUES (?, ?, ?, ?, true, ?, ?, ?)
			`, a.title, a.description, a.fullText, a.isImportant, userID, now, userID)
		} else {
			db.Exec(`
				INSERT INTO announcements (title, description, full_text, is_important, is_active, created_by)
				VALUES (?, ?, ?, ?, false, ?)
			`, a.title, a.description, a.fullText, a.isImportant, userID)
		}
	}
}

func seedNotifications(db *gorm.DB, userID int) {
	notifSpecs := []struct {
		notifType string
		title     string
		message   string
		isRead    bool
	}{
		{"info", "Добро пожаловать", "Это демо-уведомление, создано cmd/seed-demo.", false},
		{"warning", "Запланированные работы", "В субботу с 02:00 до 04:00 возможны кратковременные перебои.", false},
		{"success", "Заявка согласована", "Ваша заявка DEMO/003 успешно прошла согласование.", true},
	}
	for _, n := range notifSpecs {
		db.Exec(`
			INSERT INTO notifications (user_id, type, title, message, is_read, created_at)
			SELECT ?, ?, ?, ?, ?, NOW()
			WHERE NOT EXISTS (
				SELECT 1 FROM notifications WHERE user_id = ? AND title = ?
			)
		`, userID, n.notifType, n.title, n.message, n.isRead, userID, n.title)
	}
}

// appSpec описывает желаемое состояние демо-заявки. Поля - всё что важно
// для разнообразия UI-сценариев: статусы, периоды, наполнение.
type appSpec struct {
	number         string
	status         string
	confirmation   string // "Согласовано" / "Не согласовано" / "Согласование"
	dataApproval   string // "true" если ApproverComplete
	daysOffset     int    // entry_date_from относительно today (- прошлое, + будущее)
	durationDays   int    // длительность активности
	withCars       bool
	withPeople     bool
	withItems      bool
	message        string
	completedAfter bool // simulate confirmation_datetime в прошлом для архива
}

func seedExtendedApplications(db *gorm.DB, orgIDs, compIDs []int, userID int, uaIDs uniqueAttachmentIDs, placeNames []string) {
	specs := []appSpec{
		{number: "DEMO/001", status: "В работе", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: 0, durationDays: 30, withCars: true, withPeople: true,
			message: "Заявка на завоз материалов", completedAfter: false},
		{number: "DEMO/002", status: "Непрочитано", confirmation: "Согласование", dataApproval: "false",
			daysOffset: 1, durationDays: 7, withCars: true,
			message: "Новая заявка от подрядчика"},
		{number: "DEMO/003", status: "В обработке", confirmation: "Согласование", dataApproval: "false",
			daysOffset: 0, durationDays: 14, withCars: true, withItems: true,
			message: "Доставка оборудования"},
		{number: "DEMO/004", status: "Согласование", confirmation: "Согласование", dataApproval: "false",
			daysOffset: 2, durationDays: 10, withPeople: true,
			message: "Запрос на пропуск сотрудников"},
		{number: "DEMO/005", status: "Не согласовано", confirmation: "Не согласовано", dataApproval: "false",
			daysOffset: -3, durationDays: 5, withCars: true,
			message: "Заявка отклонена из-за неполных данных"},
		{number: "DEMO/006", status: "В работе", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: -1, durationDays: 14, withCars: true, withPeople: true, withItems: true,
			message: "Комплексная заявка: машины + сотрудники + имущество"},
		{number: "DEMO/007", status: "Завершено", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: -7, durationDays: 7, withCars: true,
			message: "Недавно завершённая заявка", completedAfter: true},
		{number: "DEMO/008", status: "Завершено", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: -60, durationDays: 7, withCars: true,
			message: "Архивная заявка (60 дней назад)", completedAfter: true},
		{number: "DEMO/009", status: "Завершено", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: -45, durationDays: 14, withPeople: true,
			message: "Архивная заявка с сотрудниками", completedAfter: true},
		{number: "DEMO/010", status: "Отказано", confirmation: "Не согласовано", dataApproval: "false",
			daysOffset: -10, durationDays: 5, withItems: true,
			message: "Заявка отказана администратором"},
		{number: "DEMO/011", status: "Согласование", confirmation: "Согласование", dataApproval: "false",
			daysOffset: 7, durationDays: 30, withCars: true,
			message: "Будущая заявка (через неделю)"},
		{number: "DEMO/012", status: "В работе", confirmation: "Согласовано", dataApproval: "true",
			daysOffset: 0, durationDays: 60, withCars: true, withPeople: true,
			message: "Долгая заявка - 2 месяца"},
	}

	for i, spec := range specs {
		orgID := orgIDs[i%len(orgIDs)]
		compID := compIDs[i%len(compIDs)]
		seedOneApp(db, spec, orgID, compID, userID, uaIDs, placeNames)
	}
}

func seedOneApp(db *gorm.DB, spec appSpec, orgID, compID, userID int, uaIDs uniqueAttachmentIDs, placeNames []string) {
	var existingID int
	db.Raw(`SELECT id FROM applications WHERE application_number = ? LIMIT 1`, spec.number).Scan(&existingID)
	if existingID != 0 {
		return
	}

	now := time.Now()
	from := now.AddDate(0, 0, spec.daysOffset)
	to := from.AddDate(0, 0, spec.durationDays)
	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	var confirmationDatetime *time.Time
	if spec.completedAfter {
		t := from.AddDate(0, 0, spec.durationDays)
		confirmationDatetime = &t
	}

	var appID int
	db.Raw(`
		INSERT INTO applications (
			application_number, confirmation, sending_datetime, organization_id, company_id,
			sender_user_id, message, status, data_approval, confirmation_datetime
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, spec.number, spec.confirmation, now, orgID, compID, userID, spec.message, spec.status, spec.dataApproval, confirmationDatetime).Scan(&appID)
	if appID == 0 {
		log.Printf("demo seed: failed to create application %s", spec.number)
		return
	}

	if spec.withCars && uaIDs.cars != 0 {
		seedAttachmentCars(db, appID, uaIDs.cars, fromStr, toStr, userID, placeNames)
	}
	if spec.withPeople && uaIDs.people != 0 {
		seedAttachmentPeople(db, appID, uaIDs.people, fromStr, toStr)
	}
	if spec.withItems && uaIDs.items != 0 {
		seedAttachmentItems(db, appID, uaIDs.items, fromStr, toStr)
	}

	db.Exec(`
		INSERT INTO application_responsible_users (application_id, user_id, is_primary, required_approval)
		VALUES (?, ?, true, false)
		ON CONFLICT DO NOTHING
	`, appID, userID)
}

// seedAttachmentCars создаёт вложение cars с 2 машинами и их историей в audit_log.
func seedAttachmentCars(db *gorm.DB, appID, uaCarsID int, fromStr, toStr string, userID int, placeNames []string) {
	var attachmentID int
	db.Raw(`
		INSERT INTO attachments (
			application_id, attachment_type, attachment_name, attachment_display_name,
			entry_date_from, entry_date_to, entry_time_from, entry_time_to,
			unique_attachment_id, status, created_at, updated_at
		)
		VALUES (?, 'cars', 'cars_demo', 'Автомобили (демо)', ?, ?, '09:00:00', '18:00:00', ?, 1, NOW(), NOW())
		RETURNING id
	`, appID, fromStr, toStr, uaCarsID).Scan(&attachmentID)
	if attachmentID == 0 {
		return
	}

	cars := []struct {
		number string
		brand  string
	}{
		{fmt.Sprintf("А%03d%s777", appID%1000, "БВ"), "Toyota Camry"},
		{fmt.Sprintf("Х%03d%s199", appID%1000, "ЕТ"), "Kamaz 65117"},
	}
	place := placeNames[0]
	if len(placeNames) > 1 {
		place = placeNames[appID%len(placeNames)]
	}

	var firstCarID int
	for i, c := range cars {
		var carID int
		db.Raw(`
			INSERT INTO cars (
				attachment_id, car_number, car_brand, unload_place,
				entry_date_from, entry_time_from, entry_date_to, entry_time_to,
				status, date_added
			)
			VALUES (?, ?, ?, ?, ?, '09:00:00', ?, '18:00:00', 1, NOW())
			RETURNING id
		`, attachmentID, c.number, c.brand, place, fromStr, toStr).Scan(&carID)
		if i == 0 {
			firstCarID = carID
		}
	}

	if firstCarID == 0 {
		return
	}
	now := time.Now()
	history := []struct {
		action  string
		comment string
		offset  time.Duration
	}{
		{"create", "Создана запись об автомобиле", -48 * time.Hour},
		{"update", "Обновлены данные водителя", -36 * time.Hour},
		{"entry", "Прибытие на территорию", -24 * time.Hour},
		{"exit", "Убытие с территории", -1 * time.Hour},
	}
	for _, h := range history {
		// Демо-история машины пишется в общий audit_log (#870): cars_history дропнута,
		// читатели истории на audit_log-only. Форма details - {comment} как у recorder.
		db.Exec(`
			INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, details, created_at)
			VALUES ('car', ?, ?, ?, jsonb_build_object('comment', ?::text), ?)
		`, firstCarID, h.action, userID, h.comment, now.Add(h.offset))
	}
}

func seedAttachmentPeople(db *gorm.DB, appID, uaPeopleID int, fromStr, toStr string) {
	var attachmentID int
	db.Raw(`
		INSERT INTO attachments (
			application_id, attachment_type, attachment_name, attachment_display_name,
			entry_date_from, entry_date_to, entry_time_from, entry_time_to,
			unique_attachment_id, status, created_at, updated_at
		)
		VALUES (?, 'people', 'people_demo', 'Сотрудники (демо)', ?, ?, '09:00:00', '18:00:00', ?, 1, NOW(), NOW())
		RETURNING id
	`, appID, fromStr, toStr, uaPeopleID).Scan(&attachmentID)
	if attachmentID == 0 {
		return
	}
	employees := []struct {
		last  string
		first string
		mid   string
		pos   string
	}{
		{"Иванов", "Иван", "Иванович", "Водитель"},
		{"Петров", "Пётр", "Петрович", "Грузчик"},
		{"Сидоров", "Алексей", "Сергеевич", "Сопровождающий"},
	}
	for _, e := range employees {
		db.Exec(`
			INSERT INTO employees (
				attachment_id, last_name, first_name, middle_name, position,
				status, created_at, updated_at, date_created
			)
			VALUES (?, ?, ?, ?, ?, 1, NOW(), NOW(), NOW())
		`, attachmentID, e.last, e.first, e.mid, e.pos)
	}
}

func seedAttachmentItems(db *gorm.DB, appID, uaItemsID int, fromStr, toStr string) {
	var attachmentID int
	db.Raw(`
		INSERT INTO attachments (
			application_id, attachment_type, attachment_name, attachment_display_name,
			entry_date_from, entry_date_to, entry_time_from, entry_time_to,
			unique_attachment_id, status, created_at, updated_at
		)
		VALUES (?, 'items', 'items_demo', 'Имущество (демо)', ?, ?, '09:00:00', '18:00:00', ?, 1, NOW(), NOW())
		RETURNING id
	`, appID, fromStr, toStr, uaItemsID).Scan(&attachmentID)
	if attachmentID == 0 {
		return
	}
	items := []struct {
		name string
		qty  int
	}{
		{"Ноутбуки Dell Latitude", 5},
		{"Серверная стойка 42U", 1},
		{"Коробки с документами", 10},
	}
	for _, it := range items {
		db.Exec(`
			INSERT INTO items (
				attachment_id, name, count, date_created, created_at, updated_at
			)
			VALUES (?, ?, ?, NOW(), NOW(), NOW())
		`, attachmentID, it.name, it.qty)
	}
}
