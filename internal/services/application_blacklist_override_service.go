package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OverrideBlacklistFlagRequest - запрос "всё равно пропустить" по конкретному флагу (#481).
type OverrideBlacklistFlagRequest struct {
	FlagID  int    `json:"flag_id" validate:"gte=1"`
	Comment string `json:"comment" validate:"required,min=1,max=1000"`
}

// OverrideBlacklistFlag фиксирует решение ответственного "всё равно пропустить" по
// помеченному элементу заявки (#481, срез 4): пишет аудит-запись (кто/когда/коммент +
// снимок совпавшего значения) и тем самым снимает блокировку согласования по этому флагу.
// Ответственный по заявке либо принимающий. Идемпотентно: повторный override того же флага не
// плодит дубль (uniqueIndex на flag_id) и возвращает успех.
func (s *applicationService) OverrideBlacklistFlag(ctx context.Context, username string, applicationID int, req OverrideBlacklistFlagRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Комментарий к пропуску обязателен")
	}

	// Подтвердить пропуск может ответственный по заявке либо принимающий: решение
	// о возможном обходе ЧС принимает тот, кто первым дошёл до заявки, а отменять
	// подтверждение оба могли и раньше - проверка та же.
	allowed, err := s.canManageBlacklistOverride(ctx, applicationID, user.ID)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для подтверждения пропуска")
	}

	// Флаг должен существовать и принадлежать этой заявке.
	var flag models.ApplicationBlacklistFlag
	err = s.db.WithContext(ctx).Where("id = ? AND application_id = ?", req.FlagID, applicationID).First(&flag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Предупреждение не найдено для этой заявки")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения предупреждения")
	}

	var existing int64
	if err := s.db.WithContext(ctx).Model(&models.ApplicationBlacklistOverride{}).
		Where("flag_id = ?", flag.ID).Count(&existing).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки подтверждения пропуска")
	}
	if existing > 0 {
		return nil // уже подтверждён - идемпотентно
	}

	now := time.Now().UTC()
	override := models.ApplicationBlacklistOverride{
		FlagID:             flag.ID,
		ApplicationID:      flag.ApplicationID,
		ElementType:        flag.ElementType,
		ElementID:          flag.ElementID,
		ElementNormalized:  flag.ElementNormalized,
		MatchedBlacklistID: flag.MatchedBlacklistID,
		MatchedValue:       flag.MatchedValue,
		OverriddenByUserID: user.ID,
		Comment:            comment,
		CreatedAt:          now,
	}
	// Создание override + запись решения в обе истории (заявки и элемента) атомарно.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&override).Error; err != nil {
			return err
		}
		return s.logBlacklistOverrideAction(ctx, tx, flag, user.ID, "blacklist_override", comment)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil // гонка двух параллельных override одного флага - не ошибка
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения подтверждения пропуска")
	}
	// Подтверждение пропуска меняет доступность согласования - участники детали
	// увидят это live (#840 V4).
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)
	return nil
}

// logBlacklistOverrideAction фиксирует подтверждение/отмену пропуска И в истории заявки, И в
// истории самого элемента (машины/человека) - чтобы решение было видно из обеих карточек (#481,
// срез C-followup). actionType: 'blacklist_override' / 'blacklist_override_revoke'. В комментарий
// кладём, КАКОЙ элемент и на что похоже (+ причину для подтверждения), иначе в истории не видно,
// о какой машине/человеке речь.
func (s *applicationService) logBlacklistOverrideAction(ctx context.Context, tx *gorm.DB, flag models.ApplicationBlacklistFlag, userID int, actionType, reason string) error {
	label := s.blacklistElementLabel(tx, flag)
	desc := label
	if flag.MatchedValue != "" {
		desc = strings.TrimSpace(label + " - похоже на " + flag.MatchedValue)
	}
	comment := desc
	if reason = strings.TrimSpace(reason); reason != "" {
		comment = desc + " (причина: " + reason + ")"
	}

	meta, _ := json.Marshal(map[string]any{
		"flag_id":              flag.ID,
		"matched_blacklist_id": flag.MatchedBlacklistID,
		"matched_value":        flag.MatchedValue,
		"element":              label,
	})
	// created_at проставляет recorder временем вставки (#870, срез 1.14): в рамках одной
	// транзакции расхождение с моментом override - микросекунды, история заявки и элемента
	// разные endpoint-ы. Как и у car/employee веток ниже.
	if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &flag.ApplicationID, actionType, &userID,
		applicationAuditDetails{Comment: &comment, Metadata: meta}); err != nil {
		return err
	}

	switch flag.ElementType {
	case models.BlacklistElementCar:
		// recorder проставляет created_at временем вставки, а не `at`: в рамках одной
		// транзакции расхождение микросекунды, история заявки и машины - разные endpoint-ы.
		return s.recorder.Record(ctx, tx, models.AuditEntityCar, &flag.ElementID, actionType, &userID, carAuditDetails{Comment: &comment})
	case models.BlacklistElementEmployee:
		// created_at проставляет recorder временем вставки, как у car-ветки выше.
		return s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &flag.ElementID, actionType, &userID, carAuditDetails{Comment: &comment})
	}
	return nil
}

// blacklistElementLabel - человекочитаемое имя помеченного элемента для истории: "номер марка"
// для машины, ФИО для человека. Берём из текущей строки cars/employees (она жива на момент
// override/отмены - это элемент заявки).
func (s *applicationService) blacklistElementLabel(tx *gorm.DB, flag models.ApplicationBlacklistFlag) string {
	switch flag.ElementType {
	case models.BlacklistElementCar:
		var r struct {
			CarNumber string
			CarBrand  string
		}
		tx.Raw("SELECT car_number, car_brand FROM cars WHERE id = ?", flag.ElementID).Scan(&r)
		return joinNonEmpty(" ", r.CarNumber, r.CarBrand)
	case models.BlacklistElementEmployee:
		var r struct {
			LastName   string
			FirstName  string
			MiddleName string
		}
		tx.Raw("SELECT last_name, first_name, middle_name FROM employees WHERE id = ?", flag.ElementID).Scan(&r)
		return joinNonEmpty(" ", r.LastName, r.FirstName, r.MiddleName)
	}
	return ""
}

// joinNonEmpty склеивает непустые (после TrimSpace) части через sep.
func joinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// DeleteBlacklistOverride снимает ранее подтверждённый пропуск по флагу (#481, срез C):
// удаляет аудит-запись override, чем снова блокирует согласование заявки по этому элементу
// (hasUnoverriddenBlacklistFlags опять вернёт true). Право отмены шире, чем создание: не
// только ответственный по заявке, но и принимающий (глобальный согласующий) - ошибочное
// подтверждение должен мочь снять и тот, кто его не ставил. Override-строку удаляем (иначе
// гейт не разблокируется обратно), поэтому факт отмены пишем отдельно в историю заявки.
func (s *applicationService) DeleteBlacklistOverride(ctx context.Context, username string, applicationID, flagID int) error {
	if flagID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный идентификатор предупреждения")
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	allowed, err := s.canManageBlacklistOverride(ctx, applicationID, user.ID)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для отмены подтверждения пропуска")
	}

	// Флаг должен существовать и принадлежать этой заявке - защита от подмены чужого flag_id.
	var flag models.ApplicationBlacklistFlag
	if err := s.db.WithContext(ctx).Where("id = ? AND application_id = ?", flagID, applicationID).First(&flag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Предупреждение не найдено для этой заявки")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения предупреждения")
	}

	var override models.ApplicationBlacklistOverride
	if err := s.db.WithContext(ctx).Where("flag_id = ? AND application_id = ?", flag.ID, applicationID).First(&override).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Подтверждение пропуска не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения подтверждения пропуска")
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.ApplicationBlacklistOverride{}, override.ID).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка отмены подтверждения пропуска")
		}
		// Отмена без причины - комментарий пустой, история элемента подставит matched_value.
		if err := s.logBlacklistOverrideAction(ctx, tx, flag, user.ID, "blacklist_override_revoke", ""); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории")
		}
		return nil
	}); err != nil {
		return err
	}
	// Снятие подтверждения снова блокирует согласование - участники увидят live (#840 V4).
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)
	return nil
}

// canManageBlacklistOverride - право отменять подтверждение пропуска (#481, срез C):
// ответственный по этой заявке ИЛИ принимающий (глобальный согласующий). Шире, чем право
// создавать override (только ответственный).
func (s *applicationService) canManageBlacklistOverride(ctx context.Context, applicationID, userID int) (bool, error) {
	var responsibleID int
	if err := s.db.WithContext(ctx).Raw(
		"SELECT id FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		applicationID, userID,
	).Scan(&responsibleID).Error; err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки прав на заявку")
	}
	if responsibleID != 0 {
		return true, nil
	}
	return s.isApprover(ctx, userID)
}

// hasUnoverriddenBlacklistFlags - есть ли у заявки помеченные элементы без override (#481).
// Гейт согласования основного круга: пока true, голос "approved" запрещён. Принимает
// *gorm.DB, чтобы вызываться и внутри транзакции согласования, и отдельно.
//
// Считает по ВСЕЙ заявке, включая флаги дополнений (#1685), и это намеренно: дополнение
// заявки, ещё не принятой в работу, вливается в текущий круг (статус раунда merged), то
// есть основной круг согласует в том числе его строки. Сузить проверку до
// supplement_id IS NULL значило бы пропускать похожие на ЧС строки влитой добавки мимо гейта.
func hasUnoverriddenBlacklistFlags(ctx context.Context, db *gorm.DB, applicationID int) (bool, error) {
	return hasUnoverriddenFlags(ctx, db, applicationID, nil)
}

// hasUnoverriddenSupplementBlacklistFlags - тот же гейт для круга раунда дополнения (#1685):
// считает только флаги ЭТОГО раунда. Старый неперекрытый флаг исходного состава согласованию
// раунда не мешает - тот состав давно прошёл свой круг, и голосующий по добавке за него не
// отвечает.
func hasUnoverriddenSupplementBlacklistFlags(ctx context.Context, db *gorm.DB, applicationID, supplementID int) (bool, error) {
	return hasUnoverriddenFlags(ctx, db, applicationID, &supplementID)
}

// hasUnoverriddenFlags - общий счёт неперекрытых предупреждений. supplementID nil - вся
// заявка целиком, иначе только флаги указанного раунда.
func hasUnoverriddenFlags(ctx context.Context, db *gorm.DB, applicationID int, supplementID *int) (bool, error) {
	query := db.WithContext(ctx).
		Table("application_blacklist_flags f").
		Where("f.application_id = ?", applicationID).
		Where("NOT EXISTS (SELECT 1 FROM application_blacklist_overrides o WHERE o.flag_id = f.id)").
		// Пометка - снимок на момент подачи. Если запись чёрного списка потом убрали,
		// держать заявку из-за снятого запрета незачем: предупреждать больше не о чем.
		Where(blacklistFlagStillActive)
	if supplementID != nil {
		query = query.Where("f.supplement_id = ?", *supplementID)
	}
	var cnt int64
	if err := query.Count(&cnt).Error; err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки предупреждений ЧС")
	}
	return cnt > 0, nil
}

// blacklistFlagStillActive - условие «запрет, на который ссылается пометка, ещё действует».
// Пометка хранит снимок совпавшей записи, поэтому без этой проверки снятая из чёрного
// списка машина продолжала бы держать заявку на согласовании.
const blacklistFlagStillActive = `(
	    (f.element_type = 'car' AND EXISTS (
	       SELECT 1 FROM vehicle_blacklists vb WHERE vb.id = f.matched_blacklist_id AND vb.is_active))
	 OR (f.element_type = 'employee' AND EXISTS (
	       SELECT 1 FROM person_blacklists pb WHERE pb.id = f.matched_blacklist_id AND pb.is_active))
	  )`
