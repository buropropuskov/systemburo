package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ManualAttachService -- привязка ручного вложения-сироты (#1049 режим-2) к заявке.
// Только super/admin (гейт в роутере). Два пути: усыновить сироту целиком (проставить
// application_id, обнулить org/company/is_manual) ЛИБО перевесить её сущности на уже
// существующее вложение заявки и удалить опустевшую сироту.
type ManualAttachService interface {
	// AttachToApplication привязывает ручное вложение-сироту к заявке. orphanAttID -
	// вложение с is_manual=true и application_id NULL. Ровно одно из req.ApplicationID /
	// req.TargetAttachmentID определяет путь.
	AttachToApplication(ctx context.Context, orphanAttID int, req AttachToApplicationRequest, userID int) (*AttachToApplicationResponse, error)
}

// AttachToApplicationRequest -- тело POST /attachments/:id/attach-to-application.
// Взаимоисключающие поля: ApplicationID (усыновить сироту в эту заявку) ИЛИ
// TargetAttachmentID (перевесить сущности сироты на это вложение заявки). Ровно одно.
type AttachToApplicationRequest struct {
	ApplicationID      *int `json:"application_id"`
	TargetAttachmentID *int `json:"target_attachment_id"`
}

// AttachToApplicationResponse -- результат привязки.
type AttachToApplicationResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	ApplicationID int    `json:"application_id"`
	AttachmentID  int    `json:"attachment_id"`
}

type manualAttachService struct {
	db                *gorm.DB
	recorder          AuditRecorder
	tablesProducer    *TablesRefreshPublisher
	availableProducer *AvailableRefreshPublisher
}

// NewManualAttachService создаёт сервис привязки ручных вложений к заявкам. Продюсеры
// real-time опциональны (nil безопасен - привязка отработает без live-обновления).
func NewManualAttachService(db *gorm.DB, recorder AuditRecorder, tablesProducer *TablesRefreshPublisher, availableProducer *AvailableRefreshPublisher) ManualAttachService {
	return &manualAttachService{
		db:                db,
		recorder:          recorder,
		tablesProducer:    tablesProducer,
		availableProducer: availableProducer,
	}
}

// AttachToApplication выполняет привязку. Валидация цепочки: сущность ⊂ вложение (по датам,
// проверяется при перевешивании машин), целевая заявка активна и согласована. Всё одной
// транзакцией; частичной привязки быть не должно.
func (s *manualAttachService) AttachToApplication(ctx context.Context, orphanAttID int, req AttachToApplicationRequest, userID int) (*AttachToApplicationResponse, error) {
	if (req.ApplicationID == nil) == (req.TargetAttachmentID == nil) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите либо application_id (усыновить), либо target_attachment_id (перевесить), но не оба")
	}

	var orphan models.Attachment
	if err := s.db.WithContext(ctx).First(&orphan, orphanAttID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
		}
		return nil, fmt.Errorf("failed to load attachment: %w", err)
	}
	if !orphan.IsManual || orphan.ApplicationID != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Вложение не является ручным или уже привязано к заявке")
	}

	// Сущности сироты нужны для аудита, real-time и (для машин) валидации времени.
	carIDs, empIDs, err := s.loadEntityIDs(ctx, orphan)
	if err != nil {
		return nil, err
	}

	var appID int
	if req.ApplicationID != nil {
		appID, err = s.adoptOrphan(ctx, &orphan, *req.ApplicationID, carIDs, empIDs, userID)
	} else {
		appID, err = s.reattachToExisting(ctx, &orphan, *req.TargetAttachmentID, carIDs, empIDs, userID)
	}
	if err != nil {
		return nil, err
	}

	// Метка сущностей сменилась (стали заявочными) и появилось новое доступное вложение -
	// обновляем таблицы проходной и вкладку "Доступные мне" охраны. Best-effort, вне транзакции.
	s.notifyChanged(ctx, orphan.AttachmentType, carIDs, empIDs)

	slog.Info("ручное вложение привязано к заявке", "attachment_id", orphanAttID, "application_id", appID, "user_id", userID)
	return &AttachToApplicationResponse{
		Success:       true,
		Message:       "Attachment linked to application",
		ApplicationID: appID,
		AttachmentID:  orphanAttID,
	}, nil
}

// adoptOrphan усыновляет сироту целиком: вложение само становится частью заявки. Сущности
// остаются в нём (application_id проставляется вложению), org/company/is_manual обнуляются -
// далее org берётся из заявки (scoped COALESCE). Окно вложения не меняется, поэтому
// «сущность ⊂ вложение» уже выполнено by construction (машины созданы с датами вложения).
func (s *manualAttachService) adoptOrphan(ctx context.Context, orphan *models.Attachment, applicationID int, carIDs, empIDs []int, userID int) (int, error) {
	app, err := s.loadActiveApprovedApp(ctx, applicationID)
	if err != nil {
		return 0, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Блокируем строку сироты и перепроверяем инвариант под lock: два параллельных
		// вызова на один orphan (двойной клик, две admin-сессии) сериализуются - второй
		// увидит уже привязанное вложение и получит 409, а не тихо перезапишет первый.
		if err := lockManualOrphan(tx, orphan.ID); err != nil {
			return err
		}
		if err := tx.Model(&models.Attachment{}).Where("id = ?", orphan.ID).Updates(map[string]interface{}{
			"application_id":  applicationID,
			"organization_id": nil,
			"company_id":      nil,
			"is_manual":       false,
		}).Error; err != nil {
			slog.Error("не удалось усыновить ручное вложение", "attachment_id", orphan.ID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error linking attachment")
		}
		return s.recordAttach(ctx, tx, orphan.AttachmentType, carIDs, empIDs, app.ApplicationNumber, userID)
	})
	if err != nil {
		return 0, err
	}
	return applicationID, nil
}

// reattachToExisting перевешивает сущности сироты на уже существующее вложение заявки и
// удаляет опустевшую сироту. Для машин проверяет «сущность ⊂ вложение» по датам (car.dates
// должны попадать в окно целевого вложения, иначе 422). У сотрудников своего времени нет.
// Места разгрузки машин переносятся на целевое вложение, иначе после удаления сироты охрана
// потеряет их из видимости (attachment_unload_places - источник мест вложения для "Доступные мне").
func (s *manualAttachService) reattachToExisting(ctx context.Context, orphan *models.Attachment, targetAttID int, carIDs, empIDs []int, userID int) (int, error) {
	if targetAttID == orphan.ID {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Нельзя перевесить вложение само на себя")
	}

	var target models.Attachment
	if err := s.db.WithContext(ctx).First(&target, targetAttID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не найдено")
		}
		return 0, fmt.Errorf("failed to load target attachment: %w", err)
	}
	if target.ApplicationID == nil {
		return 0, echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не принадлежит заявке")
	}
	if target.AttachmentType != orphan.AttachmentType {
		return 0, echo.NewHTTPError(http.StatusUnprocessableEntity, "Тип целевого вложения не совпадает с ручным")
	}

	if orphan.AttachmentType != "cars" && orphan.AttachmentType != "people" {
		// is_manual ставится только на cars/people (S3/S4). Явный guard: без него будущий
		// ручной items-тип прошёл бы switch мимо всех веток (сущности не перевешены), а
		// удаление сироты снесло бы items FK-каскадом. Ловим до любых мутаций.
		return 0, echo.NewHTTPError(http.StatusUnprocessableEntity, "Неподдерживаемый тип вложения")
	}

	app, err := s.loadActiveApprovedApp(ctx, *target.ApplicationID)
	if err != nil {
		return 0, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Блокируем сироту и валидируем/мутируем под lock - параллельный вызов на тот же
		// orphan сериализуется (второй увидит удалённую/привязанную сироту -> 409), а даты
		// машин проверяются атомарно с перевешиванием.
		if err := lockManualOrphan(tx, orphan.ID); err != nil {
			return err
		}

		switch orphan.AttachmentType {
		case "cars":
			// Валидация времени (сущность ⊂ вложение) - только для машин: у сотрудника
			// своего времени нет, окно берётся с вложения.
			var cars []models.Car
			if err := tx.Where("attachment_id = ?", orphan.ID).Find(&cars).Error; err != nil {
				return fmt.Errorf("failed to load orphan cars: %w", err)
			}
			for _, car := range cars {
				if !dateRangeWithin(car.EntryDateFrom, car.EntryDateTo, target.EntryDateFrom, target.EntryDateTo) {
					num := ""
					if car.CarNumber != nil {
						num = *car.CarNumber
					}
					return echo.NewHTTPError(http.StatusUnprocessableEntity, fmt.Sprintf("Период действия машины %s выходит за окно вложения заявки", strings.TrimSpace(num)))
				}
			}
			if err := tx.Model(&models.Car{}).Where("attachment_id = ?", orphan.ID).Update("attachment_id", targetAttID).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error reattaching cars")
			}
			// Места сироты -> целевое вложение (идемпотентно, дубликаты не создаём).
			if err := tx.Exec(`
				INSERT INTO attachment_unload_places (attachment_id, unload_place_id, order_index, created_at)
				SELECT ?, unload_place_id, order_index, NOW()
				FROM attachment_unload_places WHERE attachment_id = ?
				ON CONFLICT (attachment_id, unload_place_id) DO NOTHING`, targetAttID, orphan.ID).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error transferring attachment places")
			}
		case "people":
			if err := tx.Model(&models.Employee{}).Where("attachment_id = ?", orphan.ID).Update("attachment_id", targetAttID).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error reattaching employees")
			}
		}

		if err := s.recordAttach(ctx, tx, orphan.AttachmentType, carIDs, empIDs, app.ApplicationNumber, userID); err != nil {
			return err
		}

		// Сирота опустела - удаляем. Её attachment_unload_places уходят FK-каскадом; сущности
		// уже перевешены на target, под каскад не попадают.
		if err := tx.Delete(&models.Attachment{}, orphan.ID).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting orphan attachment")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return *target.ApplicationID, nil
}

// loadActiveApprovedApp загружает заявку и требует, чтобы она была активной и согласованной
// (confirmation='Согласовано', status='В работе'). Иначе привязка спрятала бы запись из таблиц
// проходной (scoped-запросы показывают заявочные только при этом условии) - блокируем 422.
func (s *manualAttachService) loadActiveApprovedApp(ctx context.Context, applicationID int) (*models.Application, error) {
	var app models.Application
	if err := s.db.WithContext(ctx).First(&app, applicationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "Заявка не найдена")
		}
		return nil, fmt.Errorf("failed to load application: %w", err)
	}
	confirmed := app.Confirmation != nil && *app.Confirmation == models.ConfirmationApproved
	inWork := app.Status != nil && *app.Status == models.StatusInWork
	if !confirmed || !inWork {
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "Привязка возможна только к активной согласованной заявке")
	}
	return &app, nil
}

// loadEntityIDs возвращает id сущностей сироты по её типу (cars -> carIDs, people -> empIDs).
func (s *manualAttachService) loadEntityIDs(ctx context.Context, orphan models.Attachment) (carIDs, empIDs []int, err error) {
	switch orphan.AttachmentType {
	case "cars":
		if err = s.db.WithContext(ctx).Model(&models.Car{}).Where("attachment_id = ?", orphan.ID).Pluck("id", &carIDs).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to load orphan car ids: %w", err)
		}
	case "people":
		if err = s.db.WithContext(ctx).Model(&models.Employee{}).Where("attachment_id = ?", orphan.ID).Pluck("id", &empIDs).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to load orphan employee ids: %w", err)
		}
	}
	return carIDs, empIDs, nil
}

// recordAttach пишет запись истории "Привязано к заявке N" на каждую перевешенную сущность.
// Действие "update" (существующий словарь FE getActionText), суть - в комментарии.
func (s *manualAttachService) recordAttach(ctx context.Context, tx *gorm.DB, attType string, carIDs, empIDs []int, appNumber *string, userID int) error {
	num := ""
	if appNumber != nil {
		num = *appNumber
	}
	comment := strings.TrimSpace(fmt.Sprintf("Привязано к заявке %s", num))

	if attType == "cars" {
		for i := range carIDs {
			id := carIDs[i]
			if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "update", &userID, carAuditDetails{Comment: &comment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error recording car history")
			}
		}
	}
	if attType == "people" {
		for i := range empIDs {
			id := empIDs[i]
			if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "update", &userID, carAuditDetails{Comment: &comment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error recording employee history")
			}
		}
	}
	return nil
}

// notifyChanged шлёт real-time обновления (таблицы проходной + "Доступные мне") по сменившимся
// сущностям. Best-effort: nil-продюсеры допустимы (тесты/окружение без событий).
func (s *manualAttachService) notifyChanged(ctx context.Context, attType string, carIDs, empIDs []int) {
	if s.tablesProducer != nil {
		switch attType {
		case "cars":
			s.tablesProducer.NotifyCarsChangedBatch(ctx, carIDs)
		case "people":
			s.tablesProducer.NotifyEmployeesChangedBatch(ctx, empIDs)
		}
	}
	if s.availableProducer != nil {
		s.availableProducer.NotifyAvailableChanged(ctx)
	}
}

// lockManualOrphan блокирует строку вложения-сироты (SELECT ... FOR UPDATE) внутри транзакции
// и подтверждает инвариант «is_manual И application_id IS NULL». Сериализует конкурентные
// привязки одного orphan: второй вызов дожидается lock и видит уже привязанную/удалённую
// сироту -> 409 вместо тихого last-write-wins.
func lockManualOrphan(tx *gorm.DB, orphanID int) error {
	var locked models.Attachment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, orphanID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusConflict, "Вложение уже изменено другим запросом")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error locking attachment")
	}
	if !locked.IsManual || locked.ApplicationID != nil {
		return echo.NewHTTPError(http.StatusConflict, "Вложение уже привязано к заявке")
	}
	return nil
}

// dateRangeWithin проверяет, что диапазон дат сущности [entityFrom, entityTo] вложен в диапазон
// вложения [containerFrom, containerTo]. Даты - строки формата YYYY-MM-DD, где лексикографический
// порядок совпадает с хронологическим (тот же инвариант, что у scoped-запросов с CURRENT_DATE
// BETWEEN). Открытый край НЕ нарушает вложенность: пустая граница вложения = «без ограничения»,
// пустая граница сущности = «наследует окно вложения» (scoped-показ гейтит машину по датам
// ВЛОЖЕНИЯ, не сущности) - обе трактовки корректно дают «вложено».
func dateRangeWithin(entityFrom, entityTo, containerFrom, containerTo *string) bool {
	ef, et := strVal(entityFrom), strVal(entityTo)
	cf, ct := strVal(containerFrom), strVal(containerTo)
	if ef != "" && cf != "" && ef < cf {
		return false
	}
	if et != "" && ct != "" && et > ct {
		return false
	}
	return true
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}
