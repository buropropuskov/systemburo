package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
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

// nameMaskUnavailable не позволяет спутать сбой проверки с отсутствием масок.
func nameMaskUnavailable(err error) error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка отображения данных временно недоступна").SetInternal(err)
}

// pdConsentMaskingActive повторяет условие гейта: настройка И видимый текст.
// Настройки читаются напрямую, а ошибка источника возвращается вызывающему коду.
func pdConsentMaskingActive(ctx context.Context, db *gorm.DB) (bool, error) {
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
		return false, nameMaskUnavailable(err)
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
	return required && hasVisibleText(text), nil
}

// loadConsentMasks возвращает карту user_id -> логин для работников, не давших
// согласия на обработку персональных данных. nil означает «маскировать некого» -
// вызывающая сторона тогда работает как раньше, без накладных расходов.
//
// В выборку идут только те, кого запрос согласия реально касается (gatedUsersWhere):
// супер-администратор проходит гейт всегда, а архивных и заблокированных отбивают
// раньше. Считай мы иначе, скрытыми оказались бы имена тех, у кого система согласия
// и не спрашивает.
func loadConsentMasks(ctx context.Context, db *gorm.DB) (map[int]string, error) {
	masks, _, err := consentMasksWithState(ctx, db)
	return masks, err
}

// consentMasksWithState - то же, что loadConsentMasks, плюс признак «запрос
// согласия сейчас работает». Пустая карта масок сама по себе не отвечает на этот
// вопрос: масок нет и когда запрос выключен, и когда все уже согласились.
func consentMasksWithState(ctx context.Context, db *gorm.DB) (map[int]string, bool, error) {
	active, err := pdConsentMaskingActive(ctx, db)
	if err != nil {
		return nil, false, err
	}
	if !active {
		return nil, false, nil
	}
	type row struct {
		ID       int
		Username string
	}
	var rows []row
	err = db.WithContext(ctx).
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
	if err != nil {
		return nil, true, nameMaskUnavailable(err)
	}
	masks := make(map[int]string, len(rows))
	for _, r := range rows {
		// С собачкой, как логины показываются во всём интерфейсе: иначе подставленный
		// вместо фамилии логин читается как фамилия.
		masks[r.ID] = "@" + r.Username
	}
	return masks, true, nil
}

// loadNameMasks собирает обе маски отображаемого имени: заданную администратором
// для принимающего и вынужденную для не давшего согласия. Согласие имеет
// приоритет: произвольная подпись принимающего тоже может содержать его ФИО.
func loadNameMasks(ctx context.Context, db *gorm.DB) (map[int]string, error) {
	consent, err := loadConsentMasks(ctx, db)
	if err != nil {
		return nil, err
	}
	return mergeDisplayNameMasks(ctx, db, consent)
}

// Reuse an already loaded consent snapshot when its legal-state flag is also
// needed by a caller. This avoids a second settings/consents read for /users/all.
func mergeDisplayNameMasks(ctx context.Context, db *gorm.DB, consent map[int]string) (map[int]string, error) {
	approver, err := loadApproverMasks(ctx, db)
	if err != nil {
		return nil, err
	}
	if len(approver) == 0 {
		return consent, nil
	}
	if len(consent) == 0 {
		return approver, nil
	}
	merged := make(map[int]string, len(consent)+len(approver))
	for id, name := range approver {
		merged[id] = name
	}
	for id, name := range consent {
		merged[id] = name
	}
	return merged, nil
}

// maskUserParts скрывает фамилию, имя и отчество работника, если его имя
// маскируется. Интерфейс, получив пустое ФИО, показывает логин сам - подменять
// фамилию логином не нужно и вредно: логин попал бы в поле «Фамилия».
func maskUserParts(masks map[int]string, userID int, last, first, middle **string) {
	if !isMasked(masks, userID) {
		return
	}
	*last = nil
	*first = nil
	*middle = nil
}

// maskUserContacts скрывает рабочие контакты работника. Почта и телефон - такие же
// персональные данные, как фамилия, и до согласия их не показывают наравне с ней.
func maskUserContacts(masks map[int]string, userID int, email, phone **string) {
	if !isMasked(masks, userID) {
		return
	}
	*email = nil
	*phone = nil
}

// isMasked сообщает, скрыты ли персональные данные этого работника.
func isMasked(masks map[int]string, userID int) bool {
	if len(masks) == 0 {
		return false
	}
	_, ok := masks[userID]
	return ok
}

// loadConsentGrants возвращает user_id -> когда работник дал действующее согласие
// на обработку персональных данных (RFC3339). Нужна администраторскому списку
// работников: без неё «дал ли человек согласие» видно только косвенно, по скрытому ФИО.
func loadConsentGrants(ctx context.Context, db *gorm.DB) map[int]string {
	type row struct {
		UserID    int       `gorm:"column:user_id"`
		GrantedAt time.Time `gorm:"column:granted_at"`
	}
	var rows []row
	err := db.WithContext(ctx).
		Table("pd_consents").
		Select("user_id, MAX(granted_at) AS granted_at").
		Where("consent_type = ? AND granted = true AND revoked_at IS NULL", ConsentTypePDProcessing).
		Group("user_id").
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil
	}
	grants := make(map[int]string, len(rows))
	for _, r := range rows {
		grants[r.UserID] = r.GrantedAt.UTC().Format(time.RFC3339)
	}
	return grants
}

// ownerDisplayName собирает «за кем закреплена запись» для реестров сотрудников и
// машин: ФИО владельца, а у не давшего согласия на обработку своих данных - его логин
// с собачкой (та же маска, что применяется к именам во всём интерфейсе).
//
// Логин остаётся и запасным вариантом: у части учётных записей ФИО не заполнено вовсе,
// и пустая строка в карточке читалась бы как «ничей».
func ownerDisplayName(masks map[int]string, userID *int, username, last, first, middle *string) *string {
	if userID == nil {
		return nil
	}
	if mask, ok := masks[*userID]; ok {
		v := mask
		return &v
	}
	parts := make([]string, 0, 3)
	for _, p := range []*string{last, first, middle} {
		if p != nil && strings.TrimSpace(*p) != "" {
			parts = append(parts, strings.TrimSpace(*p))
		}
	}
	if len(parts) == 0 {
		if username == nil || strings.TrimSpace(*username) == "" {
			return nil
		}
		v := "@" + strings.TrimSpace(*username)
		return &v
	}
	v := strings.Join(parts, " ")
	return &v
}
