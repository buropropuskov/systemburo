package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Дополнение заявки (#1685): заявитель добавляет людей, машины или ТМЦ во вложения уже
// поданной заявки, не переподавая её.
//
// Ключевое здесь - что дополнение НЕ трогает applications.confirmation и applications.status.
// От этой пары производен допуск строки на КПП (предикат confirmation='Согласовано' AND
// status IN ('В работе','Завершено') стоит в трёх запросах видимости), поэтому откат заявки
// на повторное согласование снял бы с проходной всех, кого уже пустили. Повторный круг
// живёт отдельной сущностью - раундом дополнения.

// CreateSupplementRequest - тело запроса на дополнение поданной заявки.
type CreateSupplementRequest struct {
	// Comment - зачем понадобилась добавка; виден согласующим в карточке раунда.
	Comment   *string              `json:"comment"`
	Additions []SupplementAddition `json:"additions"`
}

// SupplementAddition - строки, добавляемые в одно вложение заявки. Содержимое описано
// теми же DTO, что и подача (VehicleInput/EmployeeInput/ItemInput): формы фронта эмитят
// эту форму, и повторять её вторым набором типов значило бы разъехаться при первой правке.
type SupplementAddition struct {
	AttachmentID int             `json:"attachment_id"`
	Vehicles     []VehicleInput  `json:"vehicles"`
	Employees    []EmployeeInput `json:"employees"`
	Items        []ItemInput     `json:"items"`
}

// SupplementCounts - сколько строк каждого вида принёс раунд.
type SupplementCounts struct {
	Vehicles  int `json:"vehicles"`
	Employees int `json:"employees"`
	Items     int `json:"items"`
}

// CreateSupplementResponse - созданный раунд. Status отвечает на главный вопрос клиента:
// merged - добавка влилась в текущий круг согласования, pending - ушла на отдельный.
type CreateSupplementResponse struct {
	SupplementID int              `json:"supplement_id"`
	Number       int              `json:"number"`
	Status       string           `json:"status"`
	Counts       SupplementCounts `json:"counts"`
}

// SupplementApprovalInfo - голос согласующего по раунду (снимок ответственных заявки на
// момент подачи дополнения).
type SupplementApprovalInfo struct {
	SupplementID     int        `json:"-"`
	UserID           int        `json:"user_id"`
	Username         string     `json:"username"`
	FullName         string     `json:"full_name"`
	RequiredApproval bool       `json:"required_approval"`
	ApprovalStatus   *string    `json:"approval_status"`
	ApprovalComment  *string    `json:"approval_comment"`
	ApprovalDatetime *time.Time `json:"approval_datetime"`
}

// SupplementInfo - раунд дополнения для карточки заявки.
type SupplementInfo struct {
	ID                   int                      `json:"id"`
	ApplicationID        int                      `json:"application_id"`
	Number               int                      `json:"number"`
	Status               string                   `json:"status"`
	Comment              *string                  `json:"comment"`
	CreatedByUserID      int                      `json:"created_by_user_id"`
	CreatedByName        string                   `json:"created_by_name"`
	CreatedAt            time.Time                `json:"created_at"`
	ConfirmationDatetime *time.Time               `json:"confirmation_datetime"`
	DecidedByUserID      *int                     `json:"decided_by_user_id"`
	DecisionComment      *string                  `json:"decision_comment"`
	DecidedAt            *time.Time               `json:"decided_at"`
	Counts               SupplementCounts         `json:"counts"`
	Approvals            []SupplementApprovalInfo `json:"approvals"`
}

// supplementAuditActionCreated - действие в ленте истории заявки (entity_type=application,
// entity_id=id заявки), чтобы раунд был виден там же, где остальные события заявки.
const supplementAuditActionCreated = "supplement_created"

// supplementAllowedStatuses - статусы заявки, в которых её ещё можно дополнить. Закрытые
// (Завершено, Отказано, Отозвана) не дополняются: добавлять строки в закрытую заявку
// значило бы воскрешать её мимо штатного пути.
var supplementAllowedStatuses = []string{models.StatusUnread, models.StatusProcessing, models.StatusInWork}

// supplementAttachment - вложение-цель дополнения в том виде, в каком его проверяют гарды.
type supplementAttachment struct {
	ID                 int
	AttachmentType     string
	UniqueAttachmentID *int
	Status             *int
	ApplicationID      *int
	// Expired - срок действия вложения истёк: принимать в мёртвое окно новые строки
	// нельзя, они не доживут до проходной.
	Expired bool
}

// supplementTarget - вложение и строки, которые в него добавляют.
type supplementTarget struct {
	attachment supplementAttachment
	addition   SupplementAddition
}

// content собирает содержимое в форму, которую понимают общие с подачей валидаторы.
func (t supplementTarget) content() AttachmentContentData {
	data := AttachmentContentData{}
	if len(t.addition.Vehicles) > 0 {
		vehicles := t.addition.Vehicles
		data.Vehicles = &vehicles
	}
	if len(t.addition.Employees) > 0 {
		employees := t.addition.Employees
		data.Employees = &employees
	}
	if len(t.addition.Items) > 0 {
		items := t.addition.Items
		data.Items = &items
	}
	return data
}

// CreateSupplement создаёт раунд дополнения заявки и заводит его строки.
func (s *applicationService) CreateSupplement(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req CreateSupplementRequest) (*CreateSupplementResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if len(req.Additions) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Дополнение без добавленных строк")
	}

	// Отдельных checkNotArchived/checkNotWithdrawn тут нет намеренно: статус заявки гейтит
	// один allow-list supplementAllowedStatuses под тем же локом, что и остальные проверки,
	// а архивные и отозванные в него не входят. Две параллельные модели «куда дополнять
	// нельзя» разъехались бы при первом же новом статусе.

	var app struct {
		Status            *string
		SenderUserID      int
		ApplicationNumber *string
	}
	res := s.db.WithContext(ctx).
		Raw("SELECT status, sender_user_id, application_number FROM applications WHERE id = ?", applicationID).Scan(&app)
	if res.Error != nil {
		slog.Error("дополнение: не удалось прочитать заявку", "application_id", applicationID, "error", res.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
	}
	if res.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	// Владение проверяем ДО валидации содержимого: иначе посторонний по тексту ошибок
	// («вложение не принадлежит заявке» против «поле обязательно») выяснял бы состав чужой заявки.
	if err := ensureSupplementAllowed(app.SenderUserID, app.Status, user.ID, isSuperAdmin); err != nil {
		return nil, err
	}

	targets, counts, err := s.resolveSupplementTargets(ctx, applicationID, req.Additions)
	if err != nil {
		return nil, err
	}

	for _, t := range targets {
		uniqueID := 0
		if t.attachment.UniqueAttachmentID != nil {
			uniqueID = *t.attachment.UniqueAttachmentID
		}
		if err := s.validateAttachmentRequiredFields(ctx, uniqueID, t.attachment.AttachmentType, t.content()); err != nil {
			return nil, err
		}
		if err := s.validateBlacklistEntries(ctx, t.addition.Vehicles, t.addition.Employees); err != nil {
			return nil, err
		}
		if err := validateNoSupplementDuplicates(t); err != nil {
			return nil, err
		}
		if err := s.validateNotAlreadyInAttachment(ctx, t); err != nil {
			return nil, err
		}
	}

	comment := trimmedSupplementComment(req.Comment)
	now := time.Now().UTC()

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Перечитываем заявку под блокировкой строки: между гардом выше и вставкой её могли
	// отозвать, закрыть или принять в работу - а от статуса зависит сама ветка поведения.
	var locked struct {
		Status       *string
		SenderUserID int
	}
	if err := tx.Raw("SELECT status, sender_user_id FROM applications WHERE id = ? FOR UPDATE", applicationID).Scan(&locked).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось заблокировать заявку", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
	}
	if err := ensureSupplementAllowed(locked.SenderUserID, locked.Status, user.ID, isSuperAdmin); err != nil {
		tx.Rollback()
		return nil, err
	}

	var openCount int64
	if err := tx.Model(&models.ApplicationSupplement{}).
		Where("application_id = ? AND status IN ?", applicationID, models.OpenSupplementStatuses).
		Count(&openCount).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось проверить открытые раунды", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to check open supplements")
	}
	if openCount > 0 {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusConflict, "У заявки уже есть дополнение на рассмотрении - дождитесь решения по нему")
	}

	var maxNumber int
	if err := tx.Raw("SELECT COALESCE(MAX(number), 0) FROM application_supplements WHERE application_id = ?", applicationID).
		Scan(&maxNumber).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось получить номер раунда", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to allocate supplement number")
	}

	// Граница веток проходит там, где есть что терять: в работе сущности заявки уже
	// активированы и стоят на КПП, до этого их статус 0 и терять нечего.
	inWork := locked.Status != nil && *locked.Status == models.StatusInWork
	supplementStatus := models.SupplementMerged
	if inWork {
		supplementStatus = models.SupplementPending
	}

	supplement := models.ApplicationSupplement{
		ApplicationID:   applicationID,
		Number:          maxNumber + 1,
		Status:          supplementStatus,
		Comment:         comment,
		CreatedByUserID: user.ID,
		CreatedAt:       now,
	}
	if err := tx.Create(&supplement).Error; err != nil {
		tx.Rollback()
		// Гонка двух подач: частичный уникальный индекс не даст второму открытому раунду
		// лечь, а проверка выше её не ловит - отвечаем тем же 409, что и явный гард.
		if isUniqueViolation(err) {
			return nil, echo.NewHTTPError(http.StatusConflict, "У заявки уже есть дополнение на рассмотрении - дождитесь решения по нему")
		}
		slog.Error("дополнение: не удалось создать раунд", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create supplement")
	}

	var pendingVehicleFlags []pendingVehicleFlag
	var pendingEmployeeFlags []pendingEmployeeFlag
	for _, t := range targets {
		vFlags, eFlags, err := s.insertSupplementEntities(ctx, tx, t, supplement.ID, user.ID, now)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		pendingVehicleFlags = append(pendingVehicleFlags, vFlags...)
		pendingEmployeeFlags = append(pendingEmployeeFlags, eFlags...)
	}

	if inWork {
		if err := s.snapshotSupplementApprovers(tx, applicationID, supplement.ID, now); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if err := s.mergeSupplementIntoApprovalRound(ctx, tx, applicationID, user.ID); err != nil {
		tx.Rollback()
		return nil, err
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"supplement_id": supplement.ID,
		"number":        supplement.Number,
		"status":        supplement.Status,
		"vehicles":      counts.Vehicles,
		"employees":     counts.Employees,
		"items":         counts.Items,
	})
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, supplementAuditActionCreated, &user.ID,
		applicationAuditDetails{Comment: comment, Metadata: meta})

	if err := tx.Commit().Error; err != nil {
		slog.Error("дополнение: не удалось зафиксировать транзакцию", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Мягкое предупреждение о возможном обходе ЧС по похожему номеру/ФИО (#481). Флаги
	// помечаем раундом: иначе добавка выглядела бы как предупреждение по исходной подаче.
	s.detectBlacklistSimilarity(ctx, applicationID, &supplement.ID, pendingVehicleFlags, pendingEmployeeFlags)

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	s.notifySupplementApprovers(ctx, applicationID, supplement, app.ApplicationNumber, user.ID, inWork)

	return &CreateSupplementResponse{
		SupplementID: supplement.ID,
		Number:       supplement.Number,
		Status:       supplement.Status,
		Counts:       counts,
	}, nil
}

// ensureSupplementAllowed - кто и когда вправе дополнять: автор заявки либо супер-админ,
// и только пока заявка не закрыта. Зеркало гарда отзыва своей заявки (WithdrawApplication).
func ensureSupplementAllowed(senderUserID int, status *string, userID int, isSuperAdmin bool) error {
	if !isSuperAdmin && senderUserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Дополнить можно только собственную заявку")
	}
	current := ""
	if status != nil {
		current = *status
	}
	if !slices.Contains(supplementAllowedStatuses, current) {
		return echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Заявку в статусе «%s» дополнить нельзя", current))
	}
	return nil
}

// trimmedSupplementComment приводит пустой после trim комментарий к NULL - иначе карточка
// раунда рисовала бы пустую строку комментария как заполненную.
func trimmedSupplementComment(comment *string) *string {
	if comment == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*comment)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// resolveSupplementTargets проверяет вложения-цели и считает объём добавки. Возвращает
// цели в порядке запроса - порядок строк в ответе и в истории должен совпадать с поданным.
func (s *applicationService) resolveSupplementTargets(ctx context.Context, applicationID int, additions []SupplementAddition) ([]supplementTarget, SupplementCounts, error) {
	targets := make([]supplementTarget, 0, len(additions))
	counts := SupplementCounts{}
	seen := make(map[int]struct{}, len(additions))

	for _, addition := range additions {
		if _, dup := seen[addition.AttachmentID]; dup {
			return nil, counts, echo.NewHTTPError(http.StatusBadRequest, "Вложение указано в дополнении дважды")
		}
		seen[addition.AttachmentID] = struct{}{}

		var att supplementAttachment
		res := s.db.WithContext(ctx).Raw(`
			SELECT
				id,
				attachment_type,
				unique_attachment_id,
				status,
				application_id,
				COALESCE(NULLIF(entry_date_to::text, '')::date < `+moscowTodaySQL+`, false) AS expired
			FROM attachments
			WHERE id = ?
		`, addition.AttachmentID).Scan(&att)
		if res.Error != nil {
			slog.Error("дополнение: не удалось прочитать вложение", "attachment_id", addition.AttachmentID, "error", res.Error)
			return nil, counts, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load attachment")
		}
		// Чужое и несуществующее вложение отвечают одинаково: по различию ответов
		// перебором id вычисляется состав чужих заявок.
		if res.RowsAffected == 0 || att.ApplicationID == nil || *att.ApplicationID != applicationID {
			return nil, counts, echo.NewHTTPError(http.StatusBadRequest, "Вложение не принадлежит этой заявке")
		}
		if att.Status == nil || *att.Status != 1 {
			return nil, counts, echo.NewHTTPError(http.StatusBadRequest, "Вложение неактивно - дополнить его нельзя")
		}
		if att.Expired {
			return nil, counts, echo.NewHTTPError(http.StatusBadRequest, "Срок действия вложения истёк - дополнить его нельзя")
		}
		if err := ensureSupplementContentMatchesType(att.AttachmentType, addition); err != nil {
			return nil, counts, err
		}

		counts.Vehicles += len(addition.Vehicles)
		counts.Employees += len(addition.Employees)
		counts.Items += len(addition.Items)
		targets = append(targets, supplementTarget{attachment: att, addition: addition})
	}

	if counts.Vehicles+counts.Employees+counts.Items == 0 {
		return nil, counts, echo.NewHTTPError(http.StatusBadRequest, "Дополнение без добавленных строк")
	}
	return targets, counts, nil
}

// ensureSupplementContentMatchesType не даёт положить сотрудников в cars-вложение и наоборот:
// тип вложения определяет, какие таблицы КПП и какой бланк его обслуживают.
func ensureSupplementContentMatchesType(attachmentType string, addition SupplementAddition) error {
	mismatch := func() error {
		return echo.NewHTTPError(http.StatusBadRequest, "Содержимое дополнения не совпадает с типом вложения")
	}
	switch attachmentType {
	case "cars":
		if len(addition.Employees) > 0 || len(addition.Items) > 0 {
			return mismatch()
		}
		if len(addition.Vehicles) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Дополнение вложения без добавленных строк")
		}
	case "people":
		if len(addition.Vehicles) > 0 || len(addition.Items) > 0 {
			return mismatch()
		}
		if len(addition.Employees) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Дополнение вложения без добавленных строк")
		}
	case "items":
		if len(addition.Vehicles) > 0 || len(addition.Employees) > 0 {
			return mismatch()
		}
		if len(addition.Items) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Дополнение вложения без добавленных строк")
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Неизвестный тип вложения")
	}
	return nil
}

// validateNoSupplementDuplicates отклоняет дубли внутри одной партии добавки - тем же
// правилом, что и подача (validateNoDuplicates). Дубли против УЖЕ лежащих во вложении
// строк здесь не ловятся: у них другой источник правды (состав вложения) и своё поведение.
func validateNoSupplementDuplicates(t supplementTarget) error {
	vehicles := t.addition.Vehicles
	for i := range vehicles {
		for j := 0; j < i; j++ {
			if sameVehicle(vehicles[j], vehicles[i]) {
				return echo.NewHTTPError(http.StatusBadRequest,
					fmt.Sprintf("Машина %s добавлена в дополнение дважды", vehicleTitle(vehicles[i])))
			}
		}
	}
	employees := t.addition.Employees
	for i := range employees {
		for j := 0; j < i; j++ {
			if sameEmployee(employees[j], employees[i]) {
				return echo.NewHTTPError(http.StatusBadRequest,
					fmt.Sprintf("Сотрудник %s добавлен в дополнение дважды", employeeTitle(employees[i])))
			}
		}
	}
	return nil
}

// validateNotAlreadyInAttachment отклоняет добавку строки, которая в этом вложении уже
// есть. Дело не в аккуратности списка: таблица поста схлопывает сотрудников по паспорту
// (ROW_NUMBER PARTITION BY passport_series_number_hmac), и при равных сроках побеждает
// строка с большим идентификатором, то есть добавленная. Старая скрывается целиком, а
// вместе с ней - её набор постов: человек молча оказывается на других проходных, чем был.
//
// Менять посты уже допущенному человеку надо не второй строкой, а назначением постов
// (AssignElementTables) - оно для того и заведено. Машины дублировать тоже незачем: номер
// во вложении один, вторая строка лишь плодит путаницу на посту.
//
// Строки в корзине и окончательно удалённые не в счёт: их на посту нет, и повторная
// подача такого человека - обычное, а не ошибочное действие.
func (s *applicationService) validateNotAlreadyInAttachment(ctx context.Context, t supplementTarget) error {
	attID := t.attachment.ID

	for _, v := range t.addition.Vehicles {
		var exists int64
		if err := s.db.WithContext(ctx).Raw(`
			SELECT COUNT(*) FROM cars
			WHERE attachment_id = ? AND date_removed IS NULL AND is_purged = false
			  AND REPLACE(LOWER(car_number), ' ', '') = REPLACE(LOWER(?), ' ', '')
		`, attID, v.CarNumber).Scan(&exists).Error; err != nil {
			slog.Error("дополнение: не удалось проверить машину во вложении", "attachment_id", attID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error checking attachment content")
		}
		if exists > 0 {
			return echo.NewHTTPError(http.StatusConflict,
				fmt.Sprintf("Машина %s уже есть в этом вложении", vehicleTitle(v)))
		}
	}

	for _, e := range t.addition.Employees {
		hmac := crypto.HMACOptional(&e.PassportSeriesNumber)
		if hmac == nil || strings.TrimSpace(e.PassportSeriesNumber) == "" {
			continue
		}
		var exists int64
		if err := s.db.WithContext(ctx).Raw(`
			SELECT COUNT(*) FROM employees
			WHERE attachment_id = ? AND date_deleted IS NULL AND is_purged = false
			  AND passport_series_number_hmac = ?
		`, attID, *hmac).Scan(&exists).Error; err != nil {
			slog.Error("дополнение: не удалось проверить сотрудника во вложении", "attachment_id", attID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error checking attachment content")
		}
		if exists > 0 {
			return echo.NewHTTPError(http.StatusConflict,
				fmt.Sprintf("Сотрудник %s уже есть в этом вложении", employeeTitle(e)))
		}
	}
	return nil
}

// insertSupplementEntities заводит строки дополнения в существующем вложении. Машины и
// сотрудники создаются неактивными (status = 0) - на КПП они попадут только после принятия
// раунда; даты наследуются от вложения, как при подаче.
func (s *applicationService) insertSupplementEntities(ctx context.Context, tx *gorm.DB, t supplementTarget, supplementID, actorID int, now time.Time) ([]pendingVehicleFlag, []pendingEmployeeFlag, error) {
	attID := t.attachment.ID
	var vehicleFlags []pendingVehicleFlag
	var employeeFlags []pendingEmployeeFlag

	// Дедуп-union мест добавленных машин: attachment_unload_places - источник видимости
	// вложения для охраны, и новое место должно в него попасть (#706).
	carPlacesSet := make(map[int]struct{})
	for _, v := range t.addition.Vehicles {
		var carID int
		// Даты и время берём прямо из вложения тем же INSERT..SELECT: копия колонки в
		// колонку исключает расхождение форматов с путём подачи.
		err := tx.Raw(`
			INSERT INTO cars (attachment_id, supplement_id, car_number, car_brand, unload_place, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status, pd_consent_at, pd_consent_by_user_id)
			SELECT a.id, ?, ?, ?, ?, a.entry_date_from, a.entry_time_from, a.entry_date_to, a.entry_time_to, 0, ?, ?
			FROM attachments a WHERE a.id = ?
			RETURNING id
		`, supplementID, v.CarNumber, v.CarBrand, v.UnloadPlace, consentAt(v.PDConsent, now), consentBy(v.PDConsent, actorID), attID).Scan(&carID).Error
		if err != nil || carID == 0 {
			slog.Error("дополнение: не удалось создать машину", "attachment_id", attID, "error", err)
			return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating car")
		}

		vehicleFlags = append(vehicleFlags, pendingVehicleFlag{carID: carID, carNumber: v.CarNumber})

		carComment := fmt.Sprintf("Автомобиль %s %s добавлен дополнением", v.CarNumber, v.CarBrand)
		s.recorder.Log(ctx, tx, models.AuditEntityCar, &carID, "create", &actorID, carAuditDetails{Comment: &carComment})

		for _, placeID := range v.UnloadPlaces {
			if err := tx.Exec("INSERT INTO car_unload_places (car_id, unload_place_id, order_index) VALUES (?, ?, 1)", carID, placeID).Error; err != nil {
				slog.Error("дополнение: не удалось привязать место разгрузки", "car_id", carID, "unload_place_id", placeID, "error", err)
				return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error linking unload place")
			}
			carPlacesSet[placeID] = struct{}{}
		}
		// Историю попадания в таблицу пишет активация раунда, не подача - здесь машина
		// ещё неактивна и охране не видна (#1085).
		for _, tableID := range v.TargetTables {
			if err := tx.Exec("INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, tableID).Error; err != nil {
				slog.Error("дополнение: не удалось привязать машину к таблице", "car_id", carID, "table_id", tableID, "error", err)
				return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error linking car to table")
			}
		}
	}
	for placeID := range carPlacesSet {
		if err := tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID).Error; err != nil {
			slog.Error("дополнение: не удалось дополнить места вложения", "attachment_id", attID, "unload_place_id", placeID, "error", err)
			return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error linking unload place")
		}
	}

	for _, e := range t.addition.Employees {
		statusZero := 0
		lastName, firstName := e.LastName, e.FirstName
		citizenshipID, position := e.CitizenshipID, e.Position
		attachmentID, roundID := attID, supplementID
		employee := models.Employee{
			AttachmentID:         &attachmentID,
			SupplementID:         &roundID,
			LastName:             &lastName,
			FirstName:            &firstName,
			MiddleName:           e.MiddleName,
			CitizenshipID:        &citizenshipID,
			Position:             &position,
			PassportSeriesNumber: nilIfBlank(e.PassportSeriesNumber),
			PatentNumber:         nilIfBlankPtr(e.PatentNumber),
			OtherPermission:      e.OtherPermission,
			Status:               &statusZero,
			PDConsentAt:          consentAt(e.PDConsent, now),
			PDConsentByUserID:    consentBy(e.PDConsent, actorID),
		}
		if err := tx.Create(&employee).Error; err != nil {
			slog.Error("дополнение: не удалось создать сотрудника", "attachment_id", attID, "error", err)
			return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
		}
		empID := employee.ID

		middle := ""
		if e.MiddleName != nil {
			middle = *e.MiddleName
		}
		employeeFlags = append(employeeFlags, pendingEmployeeFlag{
			empID: empID, lastName: lastName, firstName: firstName, middleName: middle,
		})
		empComment := fmt.Sprintf("Сотрудник %s добавлен дополнением",
			strings.TrimSpace(strings.Join([]string{lastName, firstName, middle}, " ")))
		s.recorder.Log(ctx, tx, models.AuditEntityEmployee, &empID, "create", &actorID, carAuditDetails{Comment: &empComment})

		for _, tableID := range e.TargetTables {
			if err := tx.Exec("INSERT INTO employee_target_tables (employee_id, table_id, order_index) VALUES (?, ?, 1)", empID, tableID).Error; err != nil {
				slog.Error("дополнение: не удалось привязать сотрудника к таблице", "employee_id", empID, "table_id", tableID, "error", err)
				return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error linking employee to table")
			}
		}
	}

	for _, item := range t.addition.Items {
		if err := tx.Exec(`
			INSERT INTO items (attachment_id, supplement_id, name, count, date_created)
			VALUES (?, ?, ?, ?, ?)
		`, attID, supplementID, item.Name, item.Count, now.Format("2006-01-02")).Error; err != nil {
			slog.Error("дополнение: не удалось создать ТМЦ", "attachment_id", attID, "error", err)
			return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating item")
		}
	}

	return vehicleFlags, employeeFlags, nil
}

// mergeSupplementIntoApprovalRound - ветка «заявка ещё не в работе»: сущности заявки не
// активированы, на КПП терять нечего, поэтому добавка вливается в текущий круг. Голоса
// ответственных сбрасываются (они согласовывали другой состав) и confirmation пересчитывается
// штатным путём - тем же, что и после отзыва согласования.
func (s *applicationService) mergeSupplementIntoApprovalRound(ctx context.Context, tx *gorm.DB, applicationID, actorID int) error {
	var oldConfirmation *string
	if err := tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation).Error; err != nil {
		slog.Error("дополнение: не удалось прочитать согласование", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read confirmation")
	}

	if err := tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = 'pending', approval_comment = NULL, approval_datetime = NULL
		WHERE application_id = ?
	`, applicationID).Error; err != nil {
		slog.Error("дополнение: не удалось сбросить голоса", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to reset approvals")
	}

	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		return err
	}

	var newConfirmation *string
	if err := tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation).Error; err != nil {
		slog.Error("дополнение: не удалось перечитать согласование", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read confirmation")
	}

	if !sameStringPtr(oldConfirmation, newConfirmation) {
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "confirmation_change", &actorID,
			applicationAuditDetails{OldValue: oldConfirmation, NewValue: newConfirmation})
		if err := s.bumpStatusUpdated(tx, applicationID, &actorID); err != nil {
			return err
		}
	}
	return nil
}

// sameStringPtr сравнивает два необязательных значения с учётом NULL.
func sameStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// snapshotSupplementApprovers - ветка «заявка в работе»: голоса основного круга не трогаем
// (иначе updateConfirmationBasedOnApprovals уронил бы confirmation и снял с КПП уже
// допущенных), а состав голосующих по раунду фиксируем снимком ответственных на этот момент.
func (s *applicationService) snapshotSupplementApprovers(tx *gorm.DB, applicationID, supplementID int, now time.Time) error {
	err := tx.Exec(`
		INSERT INTO application_supplement_approvals (supplement_id, user_id, required_approval, approval_status, created_at)
		SELECT ?, aru.user_id, COALESCE(aru.required_approval, false), 'pending', ?
		FROM application_responsible_users aru
		WHERE aru.application_id = ?
		ON CONFLICT DO NOTHING
	`, supplementID, now, applicationID).Error
	if err != nil {
		slog.Error("дополнение: не удалось снять снимок согласующих", "application_id", applicationID, "supplement_id", supplementID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to snapshot supplement approvers")
	}
	return nil
}

// notifySupplementApprovers зовёт тех, от кого ждут голоса: по раунду - его снимок,
// при вливании в текущий круг - обязательные ответственные заявки. Автору не шлём.
// Best-effort после commit: сбой уведомления не отменяет уже созданный раунд.
func (s *applicationService) notifySupplementApprovers(ctx context.Context, applicationID int, supplement models.ApplicationSupplement, applicationNumber *string, authorID int, inWork bool) {
	if s.notificationService == nil {
		return
	}

	query := "SELECT user_id FROM application_responsible_users WHERE application_id = ? AND required_approval = true"
	arg := applicationID
	if inWork {
		query = "SELECT user_id FROM application_supplement_approvals WHERE supplement_id = ? AND required_approval = true"
		arg = supplement.ID
	}
	var recipients []int
	if err := s.db.WithContext(ctx).Raw(query, arg).Scan(&recipients).Error; err != nil {
		slog.Warn("дополнение: не удалось собрать получателей уведомления", "application_id", applicationID, "err", err)
		return
	}

	number := ""
	if applicationNumber != nil {
		number = *applicationNumber
	}
	// Без данных окно подробностей не даёт перейти к заявке (#1748).
	payloadBytes, _ := json.Marshal(map[string]any{
		"application_id":     applicationID,
		"application_number": number,
		"supplement_number":  supplement.Number,
	})
	payloadStr := string(payloadBytes)

	message := fmt.Sprintf("В заявку %s добавлены новые строки - требуется согласование.", number)
	if inWork {
		message = fmt.Sprintf("По заявке %s подано дополнение №%d - требуется согласование.", number, supplement.Number)
	}

	for _, userID := range recipients {
		if userID == authorID {
			continue
		}
		if err := s.notificationService.CreateForUser(ctx, userID,
			NotificationTypeApplicationApprovalRequired, "Требуется согласование", message, &payloadStr); err != nil {
			slog.Warn("дополнение: уведомление не создано", "user_id", userID, "application_id", applicationID, "err", err)
		}
	}
}

// GetApplicationSupplements возвращает раунды дополнения заявки, новые сверху.
func (s *applicationService) GetApplicationSupplements(ctx context.Context, applicationID int) ([]SupplementInfo, error) {
	type supplementRow struct {
		ID                   int
		ApplicationID        int
		Number               int
		Status               string
		Comment              *string
		CreatedByUserID      int
		CreatedByName        string
		CreatedAt            time.Time
		ConfirmationDatetime *time.Time
		DecidedByUserID      *int
		DecisionComment      *string
		DecidedAt            *time.Time
		Vehicles             int
		Employees            int
		Items                int
	}

	var rows []supplementRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			s.id, s.application_id, s.number, s.status, s.comment,
			s.created_by_user_id,
			COALESCE(format_full_name(u.last_name, u.first_name, u.middle_name), u.username, '') AS created_by_name,
			s.created_at, s.confirmation_datetime,
			s.decided_by_user_id, s.decision_comment, s.decided_at,
			(SELECT COUNT(*) FROM cars c WHERE c.supplement_id = s.id) AS vehicles,
			(SELECT COUNT(*) FROM employees e WHERE e.supplement_id = s.id) AS employees,
			(SELECT COUNT(*) FROM items i WHERE i.supplement_id = s.id) AS items
		FROM application_supplements s
		LEFT JOIN users u ON u.id = s.created_by_user_id
		WHERE s.application_id = ?
		ORDER BY s.number DESC
	`, applicationID).Scan(&rows).Error
	if err != nil {
		slog.Error("дополнение: не удалось получить раунды", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching supplements")
	}

	result := make([]SupplementInfo, 0, len(rows))
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
		result = append(result, SupplementInfo{
			ID:                   r.ID,
			ApplicationID:        r.ApplicationID,
			Number:               r.Number,
			Status:               r.Status,
			Comment:              r.Comment,
			CreatedByUserID:      r.CreatedByUserID,
			CreatedByName:        r.CreatedByName,
			CreatedAt:            r.CreatedAt,
			ConfirmationDatetime: r.ConfirmationDatetime,
			DecidedByUserID:      r.DecidedByUserID,
			DecisionComment:      r.DecisionComment,
			DecidedAt:            r.DecidedAt,
			Counts:               SupplementCounts{Vehicles: r.Vehicles, Employees: r.Employees, Items: r.Items},
			Approvals:            []SupplementApprovalInfo{},
		})
	}
	if len(ids) == 0 {
		return result, nil
	}

	var approvals []SupplementApprovalInfo
	err = s.db.WithContext(ctx).Raw(`
		SELECT
			sa.supplement_id, sa.user_id, u.username,
			COALESCE(format_full_name(u.last_name, u.first_name, u.middle_name), '') AS full_name,
			COALESCE(sa.required_approval, false) AS required_approval,
			sa.approval_status, sa.approval_comment, sa.approval_datetime
		FROM application_supplement_approvals sa
		JOIN users u ON u.id = sa.user_id
		WHERE sa.supplement_id IN ?
		ORDER BY sa.required_approval DESC, u.last_name, u.first_name
	`, ids).Scan(&approvals).Error
	if err != nil {
		slog.Error("дополнение: не удалось получить голоса", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching supplement approvals")
	}

	// Маска принимающего и логин вместо ФИО у не давших согласия на обработку ПД -
	// тот же слой, что в детали заявки.
	masks := loadNameMasks(ctx, s.db)
	byID := make(map[int]int, len(result))
	for i, r := range result {
		byID[r.ID] = i
	}
	for _, a := range approvals {
		idx, ok := byID[a.SupplementID]
		if !ok {
			continue
		}
		userID := a.UserID
		a.FullName = maskName(masks, &userID, a.FullName)
		result[idx].Approvals = append(result[idx].Approvals, a)
	}
	for i := range result {
		userID := result[i].CreatedByUserID
		result[i].CreatedByName = maskName(masks, &userID, result[i].CreatedByName)
	}
	return result, nil
}
