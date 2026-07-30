package services

import (
	"context"

	"gorm.io/gorm"
)

// Скрытие персональных данных работника, не давшего согласия на их обработку
// (#1567). Пока согласия нет, другим работникам вместо фамилии, имени и отчества
// показывается логин учётной записи.
//
// Логин, а не «Данные скрыты»: из трёх одинаковых строк «Данные скрыты» нельзя
// выбрать нужного согласующего, и работа встала бы. Интерфейс уже умеет этот случай -
// когда ФИО в учётной записи не заполнено, он и так печатает логин.
//
// Признак - «никогда не давал согласия», а НЕ «не подтвердил текущую редакцию».
// Второе связало бы видимость имён с повышением редакции: администратор правит
// запятую в тексте, и вся система разом теряет имена до тех пор, пока каждый
// работник не перезайдёт.
//
// Маскировка работает, только когда запрос согласия реально работает - тем же
// условием, что и гейт (тумблер включён И текст непустой). На установке, где
// согласия не спрашивают вовсе, ни у кого его нет, и без этого условия имена
// сменились бы логинами у всех сразу.

// pdConsentMaskingActive сообщает, работает ли сейчас запрос согласия. Условие
// повторяет PDConsentGateService.Requirement: признак из настроек И видимый текст,
// проверяемый той же hasVisibleText. Читаем настройки напрямую, потому что маску
// применяют сервисы, у которых на руках только соединение с базой.
func pdConsentMaskingActive(ctx context.Context, db *gorm.DB) bool {
	type row struct {
		Key   string
		Value string
	}
	var rows []row
	err := db.WithContext(ctx).
		Table("system_settings").
		Select("key, value").
		Where("key IN ?", []string{pdConsentRequiredKey, pdConsentTextKey}).
		Scan(&rows).Error
	if err != nil {
		// Тихо не маскируем: недоступная настройка не повод обезличить всю систему.
		return false
	}
	var required bool
	var text string
	for _, r := range rows {
		switch r.Key {
		case pdConsentRequiredKey:
			required = r.Value == "true"
		case pdConsentTextKey:
			text = r.Value
		}
	}
	return required && hasVisibleText(text)
}

// loadConsentMasks возвращает карту user_id -> логин для работников, не давших
// согласия на обработку персональных данных. nil означает «маскировать некого» -
// вызывающая сторона тогда работает как раньше, без накладных расходов.
//
// В выборку идут только те, кого запрос согласия реально касается (gatedUsersWhere):
// супер-администратор проходит гейт всегда, а архивных и заблокированных отбивают
// раньше. Считай мы иначе, скрытыми оказались бы имена тех, у кого система согласия
// и не спрашивает.
func loadConsentMasks(ctx context.Context, db *gorm.DB) map[int]string {
	if !pdConsentMaskingActive(ctx, db) {
		return nil
	}
	type row struct {
		ID       int
		Username string
	}
	var rows []row
	err := db.WithContext(ctx).
		Table("users").
		Select("users.id, users.username").
		Where(gatedUsersWhere).
		Where(`NOT EXISTS (
			SELECT 1 FROM pd_consents c
			WHERE c.user_id = users.id
			  AND c.consent_type = ?
			  AND c.granted = true
			  AND c.revoked_at IS NULL
		)`, ConsentTypePDProcessing).
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil
	}
	masks := make(map[int]string, len(rows))
	for _, r := range rows {
		masks[r.ID] = r.Username
	}
	return masks
}

// loadNameMasks собирает обе маски отображаемого имени: заданную администратором
// для принимающего и вынужденную для не давшего согласия. Маска принимающего в
// приоритете - она задана осознанно и персональные данные тоже не раскрывает.
func loadNameMasks(ctx context.Context, db *gorm.DB) map[int]string {
	consent := loadConsentMasks(ctx, db)
	approver := loadApproverMasks(ctx, db)
	if len(approver) == 0 {
		return consent
	}
	if len(consent) == 0 {
		return approver
	}
	merged := make(map[int]string, len(consent)+len(approver))
	for id, name := range consent {
		merged[id] = name
	}
	for id, name := range approver {
		merged[id] = name
	}
	return merged
}

// maskUserParts скрывает фамилию, имя и отчество работника, если его имя
// маскируется. Интерфейс, получив пустое ФИО, показывает логин сам - подменять
// фамилию логином не нужно и вредно: логин попал бы в поле «Фамилия».
func maskUserParts(masks map[int]string, userID int, last, first, middle **string) {
	if len(masks) == 0 {
		return
	}
	if _, ok := masks[userID]; !ok {
		return
	}
	*last = nil
	*first = nil
	*middle = nil
}
