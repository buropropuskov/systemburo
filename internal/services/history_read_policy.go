package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// HistoryReadPolicy разделяет изменения полей и события проходов в карточке.
// Журналы таблиц сохраняют отдельную проверку права на журнал поста.
type HistoryReadPolicy struct {
	Changes  bool
	Passages bool
}

func ResolveHistoryReadPolicy(ctx context.Context, resolver *PermissionResolver, userID int) (HistoryReadPolicy, error) {
	if resolver == nil {
		return HistoryReadPolicy{}, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
	}
	set, err := resolver.ResolveFresh(ctx, userID)
	if err != nil {
		return HistoryReadPolicy{}, echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
	}
	p := HistoryReadPolicy{Changes: set.Has(KeyDetailFullHistory), Passages: set.Has(KeyDetailEntryExitHistory)}
	if !p.Changes && !p.Passages {
		return p, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	return p, nil
}

func (p HistoryReadPolicy) Allows(action string) bool {
	switch action {
	case "entry", "exit", models.AuditActionEntryRevert, models.AuditActionExitRevert, PassageCorrectionAction, PassageCorrectionRevertAction:
		return p.Passages
	default:
		return p.Changes
	}
}

func (p HistoryReadPolicy) Cars(items []CarHistoryItemResponse) []CarHistoryItemResponse {
	out := make([]CarHistoryItemResponse, 0, len(items))
	for _, item := range items {
		if p.Allows(item.ActionType) {
			out = append(out, item)
		}
	}
	return out
}

func (p HistoryReadPolicy) Employees(items []EmployeeHistoryItem) []EmployeeHistoryItem {
	out := make([]EmployeeHistoryItem, 0, len(items))
	for _, item := range items {
		if p.Allows(item.ActionType) {
			out = append(out, item)
		}
	}
	return out
}
