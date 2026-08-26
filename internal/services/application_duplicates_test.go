package services

import (
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func strPtr(v string) *string { return &v }

func peopleAttachment(name string, employees ...EmployeeInput) AttachmentData {
	list := employees
	return AttachmentData{
		AttachmentType:        "people",
		AttachmentDisplayName: name,
		Data:                  AttachmentContentData{Employees: &list},
	}
}

func carsAttachment(name string, vehicles ...VehicleInput) AttachmentData {
	list := vehicles
	return AttachmentData{
		AttachmentType:        "cars",
		AttachmentDisplayName: name,
		Data:                  AttachmentContentData{Vehicles: &list},
	}
}

func TestValidateNoDuplicates(t *testing.T) {
	t.Parallel()

	ivanov := EmployeeInput{LastName: "Иванов", FirstName: "Иван", MiddleName: strPtr("Иванович"), PassportSeriesNumber: "4510 111111"}

	tests := []struct {
		name        string
		attachments []AttachmentData
		wantErr     bool
		wantIn      string
	}{
		{
			name:        "разные сотрудники проходят",
			attachments: []AttachmentData{peopleAttachment("Люди", ivanov, EmployeeInput{LastName: "Петров", FirstName: "Пётр", PassportSeriesNumber: "4510 222222"})},
		},
		{
			name:        "тот же паспорт с другим написанием - дубль",
			attachments: []AttachmentData{peopleAttachment("Люди", ivanov, EmployeeInput{LastName: "Иванов", FirstName: "Иван", PassportSeriesNumber: "4510111111"})},
			wantErr:     true,
			wantIn:      "Иванов Иван",
		},
		{
			name: "без паспорта дубль ловится по ФИО без учёта регистра",
			attachments: []AttachmentData{peopleAttachment("Люди",
				EmployeeInput{LastName: "Сидоров", FirstName: "Сидр", MiddleName: strPtr("Сидорович")},
				EmployeeInput{LastName: " сидоров ", FirstName: "СИДР", MiddleName: strPtr("Сидорович")})},
			wantErr: true,
			wantIn:  "добавлен дважды",
		},
		{
			name: "тёзки с разными паспортами проходят",
			attachments: []AttachmentData{peopleAttachment("Люди", ivanov,
				EmployeeInput{LastName: "Иванов", FirstName: "Иван", MiddleName: strPtr("Иванович"), PassportSeriesNumber: "4510 999999"})},
		},
		{
			name: "пустые записи не считаются одинаковыми",
			attachments: []AttachmentData{peopleAttachment("Люди",
				EmployeeInput{Position: "Слесарь"},
				EmployeeInput{Position: "Слесарь"})},
		},
		{
			name: "одно лицо в двух вложениях - не дубль",
			attachments: []AttachmentData{
				peopleAttachment("Люди 1", ivanov),
				peopleAttachment("Люди 2", ivanov),
			},
		},
		{
			name:        "разные машины проходят",
			attachments: []AttachmentData{carsAttachment("Машины", VehicleInput{CarNumber: "A777AA 777", CarBrand: "BMW"}, VehicleInput{CarNumber: "B111BB 77", CarBrand: "Toyota"})},
		},
		{
			name:        "тот же номер с другой маркой - дубль",
			attachments: []AttachmentData{carsAttachment("Машины", VehicleInput{CarNumber: "A777AA 777", CarBrand: "BMW"}, VehicleInput{CarNumber: "a777aa777", CarBrand: "Toyota"})},
			wantErr:     true,
			wantIn:      "добавлена дважды",
		},
		{
			name:        "несколько машин «По факту» допустимы",
			attachments: []AttachmentData{carsAttachment("Машины", VehicleInput{CarNumber: "По факту", CarBrand: "По факту"}, VehicleInput{CarNumber: "По факту", CarBrand: "BMW"})},
		},
		{
			name: "имя вложения попадает в ошибку",
			attachments: []AttachmentData{carsAttachment("Разгрузка склада",
				VehicleInput{CarNumber: "A777AA 777", CarBrand: "BMW"},
				VehicleInput{CarNumber: "A777AA 777", CarBrand: "BMW"})},
			wantErr: true,
			wantIn:  "Разгрузка склада",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateNoDuplicates(CompleteApplicationRequest{Attachments: tt.attachments})

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ожидалась успешная валидация, получена ошибка: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("ожидалась ошибка про дубль, валидация прошла")
			}
			httpErr, ok := err.(*echo.HTTPError)
			if !ok {
				t.Fatalf("ожидался *echo.HTTPError, получен %T", err)
			}
			if httpErr.Code != http.StatusBadRequest {
				t.Errorf("код ответа = %d, ожидался %d", httpErr.Code, http.StatusBadRequest)
			}
			message, _ := httpErr.Message.(string)
			if !strings.Contains(message, tt.wantIn) {
				t.Errorf("сообщение %q не содержит %q", message, tt.wantIn)
			}
		})
	}
}

func TestValidateNoDuplicatesEmptyRequest(t *testing.T) {
	t.Parallel()

	if err := validateNoDuplicates(CompleteApplicationRequest{}); err != nil {
		t.Fatalf("заявка без вложений не должна падать: %v", err)
	}

	empty := AttachmentData{AttachmentType: "items", AttachmentDisplayName: "ТМЦ"}
	if err := validateNoDuplicates(CompleteApplicationRequest{Attachments: []AttachmentData{empty}}); err != nil {
		t.Fatalf("вложение без людей и машин не должно падать: %v", err)
	}
}
