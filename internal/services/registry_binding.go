package services

import "systemburo/internal/apperr"

// registryBinding - организация и компания, к которым привязана запись реестра
// машин или сотрудников. Привязка и есть доступ: canEditCar/canEditEmployee
// пускают к записи всех из её организации и компании.
type registryBinding struct {
	OrganizationID *int
	CompanyID      *int
}

// checkRegistryBinding не даёт привязать запись реестра к чужой организации или
// компании. Правило то же, что у подачи заявки (#1437): своя из профиля - всегда,
// чужая - только с правом KeyApplicationOrganizationOverride или администратору.
// Без проверки запись подбрасывалась в реестр любой организации, а своя уводилась
// туда правкой.
//
// current - привязка записи до правки (при создании пустая). Значение, которое
// уже стоит в записи, пропускается: администратор правит машину контрагента и
// шлёт её привязку как есть, а сотрудник той же организации не может её сменить
// без права, но и не обязан отвязывать. nil (отвязать) разрешён всегда.
func checkRegistryBinding(want, own, current registryBinding, allowForeign bool) error {
	if allowForeign {
		return nil
	}
	if !bindingAllowed(want.OrganizationID, own.OrganizationID, current.OrganizationID) {
		return apperr.Forbidden("Привязать запись к чужой организации нельзя")
	}
	if !bindingAllowed(want.CompanyID, own.CompanyID, current.CompanyID) {
		return apperr.Forbidden("Привязать запись к чужой компании нельзя")
	}
	return nil
}

func bindingAllowed(want, own, current *int) bool {
	return want == nil || sameID(want, own) || sameID(want, current)
}

func sameID(a, b *int) bool {
	return a != nil && b != nil && *a == *b
}
