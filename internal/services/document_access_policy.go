package services

import (
	"context"
	"net/http"

	"systemburo/internal/apperr"
)

// DocumentAccess отделяет доступ к документным полям от видимости самой записи.
// Вызывающий сначала применяет действующую объектную область.
type DocumentAccess struct {
	actorID int
	checked bool
	set     PermissionSet
}

// ResolveDocumentAccess использует свежие права, включая персональные запреты.
// Отсутствующий пользователь или источник прав приводит к явному отказу.
func ResolveDocumentAccess(ctx context.Context, resolver *PermissionResolver, actorID int) (DocumentAccess, error) {
	if actorID <= 0 {
		return DocumentAccess{}, apperr.Forbidden("Не указан пользователь")
	}
	if resolver == nil {
		return DocumentAccess{}, apperr.New(http.StatusServiceUnavailable, "Не удалось проверить доступ к документам")
	}
	set, err := resolver.ResolveFresh(ctx, actorID)
	if err != nil {
		return DocumentAccess{}, apperr.New(http.StatusServiceUnavailable, "Не удалось проверить доступ к документам", err)
	}
	return DocumentAccess{actorID: actorID, checked: true, set: set}, nil
}

// ReadAllowed сохраняет доступ к собственным поданным данным независимо от
// назначенных документных прав. Состояние пользователя проверяется раньше.
func (p DocumentAccess) ReadAllowed(own bool) bool {
	if !p.checked || p.set.banned || p.set.inactive {
		return false
	}
	return p.set.IsSuperAdmin() || own || p.set.Has(KeyDetailDocuments)
}

func (p DocumentAccess) ExportAllowed(own bool) bool {
	if !p.ReadAllowed(own) {
		return false
	}
	return p.set.IsSuperAdmin() || own || p.set.Has(KeyDetailDocumentsExport)
}

func (p DocumentAccess) owns(ownerID *int) bool {
	return ownerID != nil && *ownerID == p.actorID
}

func clearDocuments(passport, patent, other **string) {
	*passport, *patent, *other = nil, nil, nil
}
