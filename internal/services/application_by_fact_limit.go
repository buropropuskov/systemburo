package services

import (
	"fmt"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Ограничения на заявку с машиной «По факту» (#2320).
//
// Номер «По факту» не опознаёт конкретную машину: по такому пропуску на территорию
// заезжает кто угодно. Пока ограничений не было, одна организация могла держать
// сколько угодно таких заявок и на любой срок - то есть постоянный безымянный
// пропуск, чего пропускной режим не предполагает.
//
// Три правила, все проверяются при подаче:
//
//  1. в заявке не больше одной машины «По факту» - иначе ограничение на число
//     заявок обходится строками внутри одной;
//  2. срок такой заявки - один календарный день (до 23:59 этого дня);
//  3. у организации не может быть двух незавершённых заявок «По факту» с живым
//     сроком одновременно.
//
// Ограничение общее: обхода для администратора или сотрудника бюро нет. Нужна
// вторая машина - закрывают первую заявку или подают обычную, с номером.

// byFactDayHint - что писать заявителю про срок. Держим рядом с проверкой: текст
// уходит прямо в форму подачи, и он должен объяснять правило, а не только запрещать.
const byFactDayHint = "Заявка на машину «По факту» действует один день: дата начала и дата окончания должны совпадать."

// validateByFactVehicles проверяет правила, для которых не нужна база: число машин
// «По факту» в заявке и срок каждого такого вложения.
func validateByFactVehicles(req CompleteApplicationRequest) error {
	count := 0
	for _, att := range req.Attachments {
		if att.Data.Vehicles == nil {
			continue
		}
		for _, v := range *att.Data.Vehicles {
			if !isByFactPlate(v.CarNumber) {
				continue
			}
			count++
			if count > 1 {
				return echo.NewHTTPError(http.StatusBadRequest,
					"В заявке может быть только одна машина «По факту». "+
						"Для остальных укажите номера или подайте отдельную заявку позже.")
			}
			if err := validateByFactPeriod(att); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateByFactPeriod требует у вложения с машиной «По факту» срок в один день.
func validateByFactPeriod(att AttachmentData) error {
	from := trimPtr(att.EntryDateFrom)
	to := trimPtr(att.EntryDateTo)

	// Пустой срок опаснее длинного: вложение без даты окончания живёт бессрочно.
	if from == "" || to == "" {
		return echo.NewHTTPError(http.StatusBadRequest,
			"Укажите даты начала и окончания. "+byFactDayHint)
	}
	if from != to {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Указан срок с %s по %s. %s", from, to, byFactDayHint))
	}
	return nil
}

// ensureNoActiveByFactApplication не даёт организации завести вторую незавершённую
// заявку «По факту», пока у первой не истёк срок.
//
// Смотрим на строки cars, а не на текст заявки: машина «По факту» опознаётся по
// номеру, и именно он попадает в таблицу проходной. «Живой срок» считается тем же
// выражением, что и везде (moscow_sql.go) - по московским часам и с учётом крайнего
// времени пребывания.
func (s *applicationService) ensureNoActiveByFactApplication(tx *gorm.DB, organizationID *int, hasByFact bool) error {
	if !hasByFact || organizationID == nil {
		return nil
	}

	var existing struct {
		Number string
		DateTo string
	}
	err := tx.Raw(`
		SELECT COALESCE(app.application_number, '') AS number,
		       COALESCE(NULLIF(TRIM(a.entry_date_to), ''), '') AS date_to
		FROM cars c
		JOIN attachments a ON a.id = c.attachment_id
		JOIN applications app ON app.id = a.application_id
		WHERE app.organization_id = ?
		  AND COALESCE(app.status, '') NOT IN ?
		  AND LOWER(REPLACE(TRIM(c.car_number), ' ', '')) = ?
		  AND `+passValidNowSQL("a")+`
		LIMIT 1
	`, *organizationID, models.ArchivableStatuses, byFactCompactPlate()).
		Scan(&existing).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось проверить действующие заявки «По факту»")
	}
	if existing.Number == "" && existing.DateTo == "" {
		return nil
	}

	return echo.NewHTTPError(http.StatusBadRequest, byFactBusyMessage(existing.Number, existing.DateTo))
}

// byFactBusyMessage - отказ с указанием, какая именно заявка занимает место:
// без номера заявитель не поймёт, что закрывать.
func byFactBusyMessage(number, dateTo string) string {
	msg := "У организации уже есть действующая заявка на машину «По факту»"
	if number != "" {
		msg += " " + number
	}
	if dateTo != "" {
		msg += fmt.Sprintf(" (действует по %s)", dateTo)
	}
	return msg + ". Одновременно допускается только одна такая заявка."
}

// hasByFactVehicle сообщает, есть ли в заявке машина «По факту».
func hasByFactVehicle(req CompleteApplicationRequest) bool {
	for _, att := range req.Attachments {
		if att.Data.Vehicles == nil {
			continue
		}
		for _, v := range *att.Data.Vehicles {
			if isByFactPlate(v.CarNumber) {
				return true
			}
		}
	}
	return false
}

func trimPtr(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

// ensureSupplementByFactAllowed применяет те же правила к дополнению уже поданной
// заявки: иначе ограничение обходится в два шага - подать заявку с номером, а
// машину «По факту» дописать дополнением.
//
// Считаем вместе со строками, которые в заявке уже есть: правило «одна машина на
// заявку» не различает, пришла она при подаче или дополнением.
func (s *applicationService) ensureSupplementByFactAllowed(tx *gorm.DB, applicationID int, vehicles []VehicleInput) error {
	adding := 0
	for _, v := range vehicles {
		if isByFactPlate(v.CarNumber) {
			adding++
		}
	}
	if adding == 0 {
		return nil
	}

	var existing int64
	if err := tx.Raw(`
		SELECT COUNT(*)
		FROM cars c
		JOIN attachments a ON a.id = c.attachment_id
		WHERE a.application_id = ?
		  AND LOWER(REPLACE(TRIM(c.car_number), ' ', '')) = ?
	`, applicationID, byFactCompactPlate()).Scan(&existing).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось проверить машины «По факту» в заявке")
	}

	if adding+int(existing) > 1 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"В заявке может быть только одна машина «По факту». "+
				"Дополнить заявку второй нельзя - подайте отдельную заявку, когда срок текущей закончится.")
	}
	return nil
}

// byFactCompactPlate - ключ сравнения номера «по факту» в SQL: без пробелов и в
// нижнем регистре, как приводит его LOWER(REPLACE(...)) в запросах.
func byFactCompactPlate() string {
	return strings.ToLower(strings.ReplaceAll(vehicleByFactPlate, " ", ""))
}
