package services

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/labstack/echo/v4"
)

// byFactPlateKey - номер «По факту»: конкретную машину он не опознаёт, поэтому таких
// строк во вложении может быть сколько угодно.
const byFactPlateKey = "пофакту"

// compactKey - ключ сравнения без пробелов и регистра: паспорт и госномер набирают
// по-разному («AB 123456» и «ab123456» - одно и то же).
func compactKey(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func spacedKey(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func employeeNameKey(e EmployeeInput) string {
	middle := ""
	if e.MiddleName != nil {
		middle = *e.MiddleName
	}
	parts := []string{spacedKey(e.LastName), spacedKey(e.FirstName), spacedKey(middle)}
	if parts[0] == "" && parts[1] == "" && parts[2] == "" {
		return ""
	}
	return strings.Join(parts, "|")
}

// sameEmployee зеркалит фронтовое правило (frontend/src/utils/applicationDuplicates.js):
// сравниваем по паспорту, а когда он скрыт настройкой полей вложения - по ФИО.
// Две записи без паспорта и без ФИО одинаковыми не считаются: матчить нечем.
func sameEmployee(a, b EmployeeInput) bool {
	passportA := compactKey(a.PassportSeriesNumber)
	passportB := compactKey(b.PassportSeriesNumber)
	if passportA != "" && passportB != "" {
		return passportA == passportB
	}

	nameA := employeeNameKey(a)
	return nameA != "" && nameA == employeeNameKey(b)
}

// sameVehicle: машину опознаёт госномер, марка не участвует - один номер не может
// принадлежать двум машинам.
func sameVehicle(a, b VehicleInput) bool {
	plateA := compactKey(a.CarNumber)
	plateB := compactKey(b.CarNumber)
	if plateA == "" || plateB == "" || plateA == byFactPlateKey || plateB == byFactPlateKey {
		return false
	}
	return plateA == plateB
}

func employeeTitle(e EmployeeInput) string {
	middle := ""
	if e.MiddleName != nil {
		middle = *e.MiddleName
	}
	parts := make([]string, 0, 3)
	for _, part := range []string{e.LastName, e.FirstName, middle} {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if passport := strings.TrimSpace(e.PassportSeriesNumber); passport != "" {
		return passport
	}
	return "сотрудник"
}

func vehicleTitle(v VehicleInput) string {
	parts := make([]string, 0, 2)
	for _, part := range []string{v.CarBrand, v.CarNumber} {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "машина"
	}
	return strings.Join(parts, " ")
}

// validateNoDuplicates отклоняет заявку, где одно лицо или одна машина попали во
// вложение дважды. Фронт гасит такие строки при добавлении, но черновик пользователя
// мог накопить их раньше появления гарда, а запрос может прийти и мимо UI.
// Уникальность считается в пределах вложения: одно лицо в двух вложениях с разными
// датами и местами прохода - легитимный сценарий.
func validateNoDuplicates(req CompleteApplicationRequest) error {
	for _, att := range req.Attachments {
		if att.Data.Vehicles != nil {
			vehicles := *att.Data.Vehicles
			for i := range vehicles {
				for j := 0; j < i; j++ {
					if sameVehicle(vehicles[j], vehicles[i]) {
						return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
							"Во вложении «%s» машина %s добавлена дважды",
							att.AttachmentDisplayName, vehicleTitle(vehicles[i])))
					}
				}
			}
		}

		if att.Data.Employees != nil {
			employees := *att.Data.Employees
			for i := range employees {
				for j := 0; j < i; j++ {
					if sameEmployee(employees[j], employees[i]) {
						return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
							"Во вложении «%s» сотрудник %s добавлен дважды",
							att.AttachmentDisplayName, employeeTitle(employees[i])))
					}
				}
			}
		}
	}

	return nil
}
