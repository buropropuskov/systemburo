package fakedata

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// createNameRetries -- сколько раз пробуем подобрать организацию/компанию со
// свободным именем, прежде чем сдаться. Имя случайное (OrgNameGenerator), а
// partial unique index organizations/companies смотрит на всю активную таблицу,
// включая записи прошлых партий и записи, заведённые руками -- редкое совпадение
// не должно ронять всю наливку, генератор просто пробует следующее имя.
const createNameRetries = 10

// organizationsStep наливает организации и компании -- профильные по объёму
// справочники, на которые опираются все дальнейшие срезы (ответственные
// пользователи, заявители).
type organizationsStep struct{}

func (organizationsStep) Name() string { return "организации и компании" }

func (organizationsStep) Plan(p Profile) []PlanItem {
	return []PlanItem{
		{Entity: models.AuditEntityOrganization, Title: EntityTitle(models.AuditEntityOrganization), Count: p.Organizations},
		{Entity: models.AuditEntityCompany, Title: EntityTitle(models.AuditEntityCompany), Count: p.Companies},
	}
}

func (organizationsStep) Run(ctx context.Context, env *Env) error {
	orgSvc := services.NewOrganizationService(env.DB)
	compSvc := services.NewCompanyService(env.DB)
	orgNames := NewOrgNameGenerator(env.Seed, "orgnames")
	compNames := NewOrgNameGenerator(env.Seed, "companynames")
	orgTypes := NewStream(env.Seed, "organization-types")
	compTypes := NewStream(env.Seed, "company-types")

	for i := 0; i < env.Profile.Organizations; i++ {
		id, err := createUniqueOrganization(ctx, orgSvc, orgNames, orgTypes, env.ActorUserID)
		if err != nil {
			return fmt.Errorf("организация %d/%d: %w", i+1, env.Profile.Organizations, err)
		}
		// Регистрируем в партии сразу после получения ID, до перехода к следующей
		// организации: сбой между созданием и регистрацией оставил бы на стенде
		// запись, которой нет в перечне партии, и будущее удаление её не увидит.
		if err := env.Batch.Add(ctx, models.AuditEntityOrganization, id); err != nil {
			return fmt.Errorf("регистрация организации %d в партии: %w", id, err)
		}
	}

	for i := 0; i < env.Profile.Companies; i++ {
		id, err := createUniqueCompany(ctx, compSvc, compNames, compTypes, env.ActorUserID)
		if err != nil {
			return fmt.Errorf("компания %d/%d: %w", i+1, env.Profile.Companies, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntityCompany, id); err != nil {
			return fmt.Errorf("регистрация компании %d в партии: %w", id, err)
		}
	}
	return nil
}

// createUniqueOrganization создаёт организацию со случайным именем и типом,
// повторяя попытку с новым именем при конфликте (см. createNameRetries).
func createUniqueOrganization(ctx context.Context, svc services.OrganizationService, gen *OrgNameGenerator, types *Stream, actorID int) (int, error) {
	var lastErr error
	for attempt := 0; attempt < createNameRetries; attempt++ {
		name, err := gen.Next()
		if err != nil {
			return 0, err
		}
		typ := Pick(types, models.OrgTypeValues)
		resp, err := svc.Create(ctx, actorID, services.CreateOrganizationRequest{Name: name, Type: &typ})
		if err == nil {
			return resp.ID, nil
		}
		if !isDuplicateNameConflict(err) {
			return 0, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("не удалось создать организацию за %d попыток, имена заняты в базе: %w", createNameRetries, lastErr)
}

// createUniqueCompany -- зеркало createUniqueOrganization для компаний.
func createUniqueCompany(ctx context.Context, svc services.CompanyService, gen *OrgNameGenerator, types *Stream, actorID int) (int, error) {
	var lastErr error
	for attempt := 0; attempt < createNameRetries; attempt++ {
		name, err := gen.Next()
		if err != nil {
			return 0, err
		}
		typ := Pick(types, models.OrgTypeValues)
		company, err := svc.Create(ctx, actorID, services.CreateCompanyRequest{Name: name, Type: &typ})
		if err == nil {
			return company.ID, nil
		}
		if !isDuplicateNameConflict(err) {
			return 0, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("не удалось создать компанию за %d попыток, имена заняты в базе: %w", createNameRetries, lastErr)
}

// isDuplicateNameConflict распознаёт отказ Create по занятому имени (см.
// applyNameDuplicateFilter в organization_service.go) -- единственный случай,
// когда наливке стоит попробовать другое имя вместо остановки шага с ошибкой.
func isDuplicateNameConflict(err error) bool {
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		return false
	}
	msg, ok := httpErr.Message.(string)
	return ok && (msg == "Организация с таким названием уже существует" || msg == "Компания с таким названием уже существует")
}
