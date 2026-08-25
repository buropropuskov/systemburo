package fakedata

import (
	"context"
	"fmt"

	"systemburo/internal/models"
	"systemburo/internal/services"
)

// postCandidate -- кандидат таблицы поста проходной.
type postCandidate struct {
	Name        string
	DisplayName string
	TableType   string
}

// postCandidates -- таблицы постов, которых стенду не хватает для проверки
// прохода людей и машин. Четыре штуки, по две на тип: этого достаточно, чтобы
// увидеть переключение cars/people, привязку постов к организациям/компаниям и
// фильтр таблицы в навигации, не раздувая стенд лишними постами -- в отличие от
// заявок это не то, что нужно масштабировать профилем. Name -- латинский слаг:
// он идёт в URL таблицы (/table/:tableName на фронте).
var postCandidates = []postCandidate{
	{Name: "kpp-central", DisplayName: "Центральный КПП", TableType: models.TableTypeCars},
	{Name: "kpp-cargo", DisplayName: "Грузовой въезд", TableType: models.TableTypeCars},
	{Name: "checkpoint-main", DisplayName: "Проходная №1", TableType: models.TableTypePeople},
	{Name: "checkpoint-service", DisplayName: "Служебный вход", TableType: models.TableTypePeople},
}

// postsStep наливает таблицы постов (system_tables) обоих типов с полями по
// умолчанию. Как и lookupsStep, не масштабируется профилем и добавляет только
// недостающие имена.
type postsStep struct{}

func (postsStep) Name() string { return "таблицы постов" }

func (postsStep) Plan(Profile) []PlanItem {
	// Count -- размер кандидатского списка (верхняя граница), см. lookupsStep.Plan:
	// сколько реально создастся, зависит от того, что уже есть на стенде, а Plan
	// по контракту пакета базу не читает.
	return []PlanItem{
		{Entity: models.AuditEntitySystemTable, Title: EntityTitle(models.AuditEntitySystemTable), Count: len(postCandidates)},
	}
}

func (postsStep) Run(ctx context.Context, env *Env) error {
	// PermissionService настоящий (не nil): без него Create не сгенерирует права
	// table.<slug>.<verb>, и созданный пост нельзя будет выдать ни одной роли --
	// на стенде, где следующие срезы заводят пользователей и роли, это сразу
	// сделало бы пост недоступным никому кроме супер-админа.
	permSvc := services.NewPermissionService(env.DB)
	svc := services.NewSystemTableService(env.DB, "", 0, permSvc)

	existing, err := existingTableNames(ctx, svc)
	if err != nil {
		return fmt.Errorf("таблицы постов: %w", err)
	}

	for _, cand := range postCandidates {
		if existing[cand.Name] {
			continue
		}
		id, err := svc.Create(ctx, models.CreateSystemTableRequest{
			Name: cand.Name, DisplayName: cand.DisplayName, TableType: cand.TableType,
		})
		if err != nil {
			return fmt.Errorf("таблица поста %q: %w", cand.Name, err)
		}
		if err := env.Batch.Add(ctx, models.AuditEntitySystemTable, id); err != nil {
			return fmt.Errorf("регистрация таблицы поста %d в партии: %w", id, err)
		}
	}

	// Create уже проставляет поля по умолчанию для только что созданных постов.
	// SeedMissingFields здесь безусловно и идемпотентно, как при старте сервера
	// (main.go зовёт её так же) -- недорого, а для постов, заведённых руками до
	// наливки, докладывает недостающие поля тем же путём, что и обычный запуск.
	if err := svc.SeedMissingFields(ctx); err != nil {
		return fmt.Errorf("посев недостающих полей таблиц постов: %w", err)
	}
	return nil
}

// existingTableNames собирает имена всех постов, активных и архивных.
// SystemTableService.GetAll не аддитивен как у остальных справочников этого
// среза: includeArchived=true отдаёт ТОЛЬКО архивные, а не активные+архивные
// (см. докстринг GetAll в system_table_service.go), поэтому набор собирается
// двумя вызовами.
func existingTableNames(ctx context.Context, svc services.SystemTableService) (map[string]bool, error) {
	active, err := svc.GetAll(ctx, false)
	if err != nil {
		return nil, err
	}
	archived, err := svc.GetAll(ctx, true)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(active)+len(archived))
	for _, t := range active {
		names[t.Table.Name] = true
	}
	for _, t := range archived {
		names[t.Table.Name] = true
	}
	return names, nil
}
