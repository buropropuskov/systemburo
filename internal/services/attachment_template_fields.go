package services

// TemplateField описывает одно поле, доступное для маппинга на ячейку Excel.
// Path - точечный путь, используется при заполнении бланка для resolveValue.
// IsList - принадлежит ли поле "табличной" части (список cars/employees/items).
type TemplateField struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	IsList bool   `json:"is_list,omitempty"`
}

// TemplateFieldGroup - группа полей для UI (Заявка / Авто / Сотрудник / ...).
type TemplateFieldGroup struct {
	Group  string          `json:"group"`
	Label  string          `json:"label"`
	Fields []TemplateField `json:"fields"`
}

// BuiltinTemplateFields - whitelist полей, доступных для маппинга в Excel.
// Backend проверяет что field_path в маппинге принадлежит этому списку
// (см. attachment_blank_service.go::resolveValue).
//
// Custom-поля (attachment_custom_fields) добавляются динамически на основе
// настроек конкретного UniqueAttachment в getTemplateFields() handler-а.
func BuiltinTemplateFields() []TemplateFieldGroup {
	return []TemplateFieldGroup{
		{
			Group: "application",
			Label: "Заявка",
			Fields: []TemplateField{
				{Path: "application.application_number", Label: "Номер заявки"},
				{Path: "application.sending_datetime", Label: "Дата подачи"},
				{Path: "application.status", Label: "Статус"},
				{Path: "application.confirmation", Label: "Состояние согласования"},
				{Path: "application.message", Label: "Сообщение"},
				{Path: "application.organization", Label: "Организация"},
				{Path: "application.company", Label: "Компания"},
				{Path: "application.sender.full_name", Label: "ФИО отправителя"},
				{Path: "application.sender.short_name", Label: "Фамилия И.О. отправителя"},
				{Path: "application.sender.last_name", Label: "Фамилия отправителя"},
				{Path: "application.sender.first_name", Label: "Имя отправителя"},
				{Path: "application.sender.middle_name", Label: "Отчество отправителя"},
				{Path: "application.sender.phone", Label: "Телефон отправителя"},
				{Path: "application.sender.email", Label: "Email отправителя"},
				{Path: "application.sender.position", Label: "Должность отправителя"},
				{Path: "application.confirmation_datetime", Label: "Дата согласования"},
				{Path: "application.approver_name", Label: "Согласовавший (ФИО)"},
				{Path: "application.responsible_comment", Label: "Комментарий ответственного"},
			},
		},
		{
			Group: "attachment",
			Label: "Вложение",
			Fields: []TemplateField{
				{Path: "attachment.entry_date_from", Label: "Дата с"},
				{Path: "attachment.entry_date_to", Label: "Дата по"},
				{Path: "attachment.entry_date_range", Label: "Период действия (дата с - дата по)"},
				{Path: "attachment.entry_time_from", Label: "Время с"},
				{Path: "attachment.entry_time_to", Label: "Время по"},
				{Path: "attachment.entry_time_range", Label: "Время пребывания (с - по)"},
			},
		},
		{
			Group: "car",
			Label: "Автомобиль (список)",
			Fields: []TemplateField{
				{Path: "car.row_number", Label: "Порядковый номер", IsList: true},
				{Path: "car.car_number", Label: "Номер ТС", IsList: true},
				{Path: "car.mark_name", Label: "Марка", IsList: true},
				{Path: "car.unload_place", Label: "Место разгрузки", IsList: true},
				{Path: "car.entry_date_from", Label: "Дата с", IsList: true},
				{Path: "car.entry_date_to", Label: "Дата по", IsList: true},
				{Path: "car.entry_time_from", Label: "Время с", IsList: true},
				{Path: "car.entry_time_to", Label: "Время по", IsList: true},
			},
		},
		{
			Group: "employee",
			Label: "Сотрудник (список)",
			Fields: []TemplateField{
				{Path: "employee.row_number", Label: "Порядковый номер", IsList: true},
				{Path: "employee.last_name", Label: "Фамилия", IsList: true},
				{Path: "employee.first_name", Label: "Имя", IsList: true},
				{Path: "employee.middle_name", Label: "Отчество", IsList: true},
				{Path: "employee.full_name", Label: "ФИО полностью", IsList: true},
				{Path: "employee.position", Label: "Должность", IsList: true},
				{Path: "employee.citizenship", Label: "Гражданство", IsList: true},
				{Path: "employee.passport_series_number", Label: "Серия и номер паспорта", IsList: true},
				{Path: "employee.patent_number", Label: "Номер патента", IsList: true},
				{Path: "employee.other_permission", Label: "Иное разрешение на работы", IsList: true},
			},
		},
		{
			Group: "item",
			Label: "Имущество (список)",
			Fields: []TemplateField{
				{Path: "item.row_number", Label: "Порядковый номер", IsList: true},
				{Path: "item.name", Label: "Наименование", IsList: true},
				{Path: "item.count", Label: "Количество", IsList: true},
			},
		},
	}
}

// IsValidFieldPath проверяет принадлежит ли path встроенному словарю.
// Кастомные поля проверяются отдельно (по uniqueAttachmentID).
func IsValidFieldPath(path string) bool {
	for _, g := range BuiltinTemplateFields() {
		for _, f := range g.Fields {
			if f.Path == path {
				return true
			}
		}
	}
	return false
}
