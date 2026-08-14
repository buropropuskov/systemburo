package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RemoveApplicationElementsRequest - убрать людей или машины из уже поданной заявки.
type RemoveApplicationElementsRequest struct {
	ElementType string `json:"element_type" validate:"required,oneof=people cars"`
	ElementIDs  []int  `json:"element_ids" validate:"required,min=1"`
	Reason      string `json:"reason" validate:"required,min=1,max=1000"`
}

// RemoveApplicationElements убирает человека или машину из поданной заявки.
//
// Зачем это нужно: элемент, похожий на запись чёрного списка, держит всю заявку -
// пока по нему не принято решение, заявку нельзя взять в работу. Раньше решение было
// одно, «всё равно пропустить», и если пропускать нельзя, заявка вставала целиком.
// Убрать можно любой элемент, а не только помеченный: заявитель ошибается в составе
// не реже, чем попадает на чёрный список.
//
// Удаление мягкое, тем же механизмом, что у таблиц постов: status = 0 и date_deleted,
// запись остаётся в корзине и в истории. Физическое удаление порвало бы связь с
// отметками прохода и превратило бы заявку в документ с пропущенной строкой.
func (s *applicationService) RemoveApplicationElements(ctx context.Context, username string, applicationID int, req RemoveApplicationElementsRequest) (int, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Причина удаления обязательна")
	}

	// Права и статус заявки - те же, что при доназначении постов принимающим.
	actx, err := s.checkAssignmentAllowed(ctx, username, applicationID)
	if err != nil {
		return 0, err
	}

	// Колонка даты удаления у таблиц разная: у машин date_removed, у работников
	// date_deleted. Различие историческое, но в SQL его приходится учитывать.
	elementTable, deletedColumn, entityType := "cars", "date_removed", models.AuditEntityCar
	if req.ElementType == "people" {
		elementTable, deletedColumn, entityType = "employees", "date_deleted", models.AuditEntityEmployee
	}

	elementIDs, err := s.elementsOfApplication(ctx, applicationID, elementTable, req.ElementIDs)
	if err != nil {
		return 0, err
	}

	removed := 0
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, elementID := range elementIDs {
			label, err := s.elementLabel(ctx, tx, elementTable, elementID)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			res := tx.Table(elementTable).
				Where(fmt.Sprintf("id = ? AND %s IS NULL AND is_purged = false", deletedColumn), elementID).
				Updates(map[string]interface{}{"status": 0, deletedColumn: now, "updated_at": now})
			if res.Error != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления элемента заявки")
			}
			// Уже убранный элемент повторный вызов не считает и в историю не пишет:
			// две вкладки принимающего дают два запроса на одну строку.
			if res.RowsAffected == 0 {
				continue
			}
			removed++

			// Род зависит от того, что убрали: «Машина ... убрана», «Иванов ... убран».
			removedWord := "убран(а)"
			if req.ElementType == "cars" {
				removedWord = "убрана"
			}
			comment := fmt.Sprintf("%s %s из заявки принимающим. Причина: %s", label, removedWord, reason)
			if err := s.recorder.Record(ctx, tx, entityType, &elementID, "delete", &actx.userID,
				carAuditDetails{Comment: &comment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи в историю элемента")
			}

			appComment := fmt.Sprintf("%s. Причина: %s", label, reason)
			if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &applicationID,
				"element_removed", &actx.userID, carAuditDetails{Comment: &appComment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи в историю заявки")
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Таблицы постов собираются по активным строкам, поэтому после смены status
	// пост должен обновиться сразу, без перезагрузки страницы охранником.
	if s.tablesProducer != nil {
		for _, elementID := range elementIDs {
			if req.ElementType == "people" {
				s.tablesProducer.NotifyEmployeeChanged(ctx, elementID)
			} else {
				s.tablesProducer.NotifyCarsChanged(ctx, elementID)
			}
		}
	}

	return removed, nil
}

// elementLabel - как элемент называется в истории: ФИО человека либо номер машины.
func (s *applicationService) elementLabel(ctx context.Context, tx *gorm.DB, elementTable string, elementID int) (string, error) {
	if elementTable == "employees" {
		var row struct {
			LastName   *string
			FirstName  *string
			MiddleName *string
		}
		if err := tx.Raw("SELECT last_name, first_name, middle_name FROM employees WHERE id = ?", elementID).
			Scan(&row).Error; err != nil {
			return "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения сотрудника")
		}
		name := formatFullName(row.LastName, row.FirstName, row.MiddleName)
		if name == "" {
			name = fmt.Sprintf("Сотрудник #%d", elementID)
		}
		return name, nil
	}

	var number *string
	if err := tx.Raw("SELECT car_number FROM cars WHERE id = ?", elementID).Scan(&number).Error; err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения машины")
	}
	if number == nil || strings.TrimSpace(*number) == "" {
		return fmt.Sprintf("Машина #%d", elementID), nil
	}
	return fmt.Sprintf("Машина %s", strings.TrimSpace(*number)), nil
}
