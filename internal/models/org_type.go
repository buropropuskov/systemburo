package models

// Тип справочников Organization и Company (issue #1046). Поле nullable:
// пустое (NULL) трактуется как «не указан» - так помечены записи, созданные до
// появления типа. Значения общие для обеих сущностей, храним в одном месте,
// чтобы валидатор и список не разъехались между сервисами.
const (
	OrgTypeTenant       = "Арендатор"
	OrgTypeContractor   = "Подрядчик"
	OrgTypeDepartment   = "Отдел"
	OrgTypeOrganization = "Организация"
	OrgTypeCompany      = "Компания"
)

// OrgTypeValues - допустимые значения типа в порядке отображения.
// Единый список для организаций и компаний (обе сущности принимают все значения).
var OrgTypeValues = []string{
	OrgTypeTenant,
	OrgTypeContractor,
	OrgTypeDepartment,
	OrgTypeOrganization,
	OrgTypeCompany,
}

// IsValidOrgType сообщает, входит ли v в допустимые значения типа.
func IsValidOrgType(v string) bool {
	switch v {
	case OrgTypeTenant, OrgTypeContractor, OrgTypeDepartment, OrgTypeOrganization, OrgTypeCompany:
		return true
	default:
		return false
	}
}
