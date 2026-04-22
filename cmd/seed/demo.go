package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// seedDemoData создаёт минимальный демо-набор для проверки UI-сценариев из issue #64.
// Идемпотентно: повторный запуск не ломает данные — используем ON CONFLICT / проверку существования.
//
// Создаёт:
// - Unload places (2 шт) и license plate formats (1 шт)
// - UniqueAttachment-шаблоны: cars, people, items
// - Активное объявление (для /news карточки справа и TheHeader "Важное объявление")
// - Обычную новость (для /news ленты слева)
// - Уведомление для пользователя (для UserNotificationsInline в кабинете)
// - Заявку "В работе" с вложением cars, двумя машинами и 3 записями истории (для VehicleDetailsModal)
func seedDemoData(db *gorm.DB, orgID, compID, userID int) {
	if err := seedDictionaries(db); err != nil {
		log.Printf("demo seed: dictionaries failed: %v", err)
		return
	}
	uaCarsID := seedUniqueAttachments(db)
	if uaCarsID == 0 {
		log.Printf("demo seed: no cars unique attachment, aborting")
		return
	}
	seedNewsAndAnnouncement(db, userID)
	seedNotification(db, userID)
	seedCarsApplication(db, orgID, compID, userID, uaCarsID)
	fmt.Println("Demo data seeded.")
}

func seedDictionaries(db *gorm.DB) error {
	if err := db.Exec(`
		INSERT INTO unload_places (name) VALUES ('Склад №1'), ('Склад №2'), ('Рампа А')
		ON CONFLICT (name) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("unload_places: %w", err)
	}

	// License plate formats — проверим схему таблицы и вставим если пусто.
	// В БД таблица может называться license_plate_formats.
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM license_plate_formats").Scan(&count).Error; err != nil {
		// Таблица не существует — пропускаем.
		return nil
	}
	if count == 0 {
		db.Exec(`INSERT INTO license_plate_formats (name, pattern) VALUES ('Россия', '^[АВЕКМНОРСТУХ]\d{3}[АВЕКМНОРСТУХ]{2}\d{2,3}$') ON CONFLICT DO NOTHING`)
	}
	return nil
}

// seedUniqueAttachments создаёт шаблоны вложений: cars/people/items.
// Возвращает ID cars-шаблона (нужен для привязки к заявке).
func seedUniqueAttachments(db *gorm.DB) int {
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
			VALUES (?, ?, ?, ?, true)
			ON CONFLICT (name) DO UPDATE SET display_name = EXCLUDED.display_name
		`, t.attachmentType, t.name, t.displayName, t.title)
	}
	var uaCarsID int
	db.Raw(`SELECT id FROM unique_attachments WHERE name = 'cars_demo' LIMIT 1`).Scan(&uaCarsID)
	return uaCarsID
}

func seedNewsAndAnnouncement(db *gorm.DB, userID int) {
	db.Exec(`
		INSERT INTO news (title, description, full_text, created_by, is_active)
		SELECT 'Демо-новость', 'Это пример новости для /news', 'Полный текст демо-новости. Создано cmd/seed-demo.', ?, true
		WHERE NOT EXISTS (SELECT 1 FROM news WHERE title = 'Демо-новость')
	`, userID)

	// Активное объявление: одно, с is_important.
	var annCount int64
	db.Raw("SELECT COUNT(*) FROM announcements WHERE is_active = true").Scan(&annCount)
	if annCount == 0 {
		now := time.Now()
		db.Exec(`
			INSERT INTO announcements (title, description, full_text, is_important, is_active, created_by, activated_at, activated_by)
			VALUES ('Важное демо-объявление', 'Это активное объявление для проверки UI', 'Полный текст объявления. Видно в /news и в шапке.', true, true, ?, ?, ?)
		`, userID, now, userID)
	}
}

func seedNotification(db *gorm.DB, userID int) {
	db.Exec(`
		INSERT INTO notifications (user_id, type, title, message, is_read, created_at)
		SELECT ?, 'info', 'Добро пожаловать', 'Это демо-уведомление, создано cmd/seed-demo.', false, NOW()
		WHERE NOT EXISTS (
			SELECT 1 FROM notifications WHERE user_id = ? AND title = 'Добро пожаловать'
		)
	`, userID, userID)
}

// seedCarsApplication создаёт заявку "В работе" с вложением cars, двумя машинами и 3 записями истории.
// Активна сегодня (для фильтра active_today) и имеет cars_history (для VehicleDetailsModal).
func seedCarsApplication(db *gorm.DB, orgID, compID, userID, uaCarsID int) {
	// Проверяем что демо-заявки ещё нет.
	var appID int
	db.Raw(`SELECT id FROM applications WHERE application_number = 'DEMO/001' LIMIT 1`).Scan(&appID)
	if appID != 0 {
		return
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	oneMonthLater := now.AddDate(0, 1, 0).Format("2006-01-02")

	db.Raw(`
		INSERT INTO applications (
			application_number, confirmation, sending_datetime, organization_id, company_id,
			sender_user_id, message, status, data_approval
		)
		VALUES ('DEMO/001', 'Согласовано', ?, ?, ?, ?, 'Демо-заявка из cmd/seed-demo', 'В работе', 'true')
		RETURNING id
	`, now, orgID, compID, userID).Scan(&appID)
	if appID == 0 {
		log.Printf("demo seed: failed to create application")
		return
	}

	// Вложение cars.
	var attachmentID int
	db.Raw(`
		INSERT INTO attachments (
			application_id, attachment_type, attachment_name, attachment_display_name,
			entry_date_from, entry_date_to, entry_time_from, entry_time_to,
			unique_attachment_id, status, created_at, updated_at
		)
		VALUES (?, 'cars', 'cars_demo', 'Автомобили (демо)', ?, ?, '09:00:00', '18:00:00', ?, 1, NOW(), NOW())
		RETURNING id
	`, appID, today, oneMonthLater, uaCarsID).Scan(&attachmentID)
	if attachmentID == 0 {
		log.Printf("demo seed: failed to create attachment")
		return
	}

	// Машины.
	type demoCar struct {
		number string
		brand  string
	}
	cars := []demoCar{
		{"А123БВ777", "Toyota Camry"},
		{"Х456ЕТ199", "Kamaz 65117"},
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
			VALUES (?, ?, ?, 'Склад №1', ?, '09:00:00', ?, '18:00:00', 1, NOW())
			RETURNING id
		`, attachmentID, c.number, c.brand, today, oneMonthLater).Scan(&carID)
		if i == 0 {
			firstCarID = carID
		}
	}

	// cars_history — 3 записи для первой машины.
	if firstCarID != 0 {
		historyEntries := []struct {
			actionType string
			comment    string
			offset     time.Duration
		}{
			{"create", "Создана запись об автомобиле", -48 * time.Hour},
			{"entry", "Прибытие на территорию", -24 * time.Hour},
			{"exit", "Убытие с территории", -1 * time.Hour},
		}
		for _, h := range historyEntries {
			db.Exec(`
				INSERT INTO cars_history (car_id, user_id, action_type, comment, created_at)
				VALUES (?, ?, ?, ?, ?)
			`, firstCarID, userID, h.actionType, h.comment, now.Add(h.offset))
		}
	}

	// Привязка ответственного пользователя к заявке.
	db.Exec(`
		INSERT INTO application_responsible_users (application_id, user_id, is_primary, required_approval)
		VALUES (?, ?, true, false)
		ON CONFLICT DO NOTHING
	`, appID, userID)

	fmt.Printf("Demo application created: id=%d (DEMO/001), car_history entries: 3\n", appID)
}
