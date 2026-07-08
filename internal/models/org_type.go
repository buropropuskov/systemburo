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
)

// OrgTypeValues - допустимые значения типа в порядке отображения.
var OrgTypeValues = []string{
	OrgTypeTenant,
	OrgTypeContractor,
	OrgTypeDepartment,
	OrgTypeOrganization,
}

// IsValidOrgType сообщает, входит ли v в допустимые значения типа.
func IsValidOrgType(v string) bool {
	switch v {
	case OrgTypeTenant, OrgTypeContractor, OrgTypeDepartment, OrgTypeOrganization:
		return true
	default:
		return false
	}
}
