package fakedata

import "systemburo/internal/models"

// Названия видов записей для вывода команды. Раньше их держал у себя каждый
// потребитель: предварительный показ печатал «Заявки», отчёт об удалении -- «Заявки»,
// а отчёт о наливке -- код сущности («application», «unique_employee»), потому что
// брал ключи прямо из перечня партии. Человек, читающий вывод, видел один и тот же
// вид записей то по-русски, то латиницей.
//
// Ключ -- значение models.AuditEntity*, то же, которым вид зарегистрирован в партии.
var entityTitles = map[string]string{
	models.AuditEntityApplication:        "Заявки",
	models.AuditEntityApprover:           "Принимающие",
	models.AuditEntityUniqueCar:          "Машины (реестр)",
	models.AuditEntityUniqueEmployee:     "Сотрудники (реестр)",
	models.AuditEntityVehicleBlacklist:   "Чёрный список машин",
	models.AuditEntityPersonBlacklist:    "Чёрный список людей",
	models.AuditEntityUser:               "Пользователи",
	models.AuditEntitySystemTable:        "Таблицы постов",
	models.AuditEntityUniqueAttachment:   "Шаблоны вложений",
	models.AuditEntityLicensePlateFormat: "Форматы номеров",
	models.AuditEntityCitizenship:        "Гражданства",
	models.AuditEntityMark:               "Марки машин",
	models.AuditEntityUnloadPlace:        "Места разгрузки",
	models.AuditEntityCompany:            "Компании",
	models.AuditEntityOrganization:       "Организации",
	models.AuditEntityCar:                "Отметки проезда машин",
	models.AuditEntityEmployee:           "Отметки прохода сотрудников",
}

// EntityTitle -- название вида записей в именительном падеже множественного числа.
// Неизвестный вид отдаётся как есть: пропажа строки в выводе хуже кода сущности в ней.
func EntityTitle(entity string) string {
	if title, ok := entityTitles[entity]; ok {
		return title
	}
	return entity
}
