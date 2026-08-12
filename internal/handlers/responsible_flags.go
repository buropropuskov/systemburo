package handlers

import (
	"log/slog"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// callerOwnDirectoryIDs - собственные организация/компания вызывающего (users.organization_id,
// users.company_id), нужны только для сравнения со спрошенной записью справочника.
type callerOwnDirectoryIDs struct {
	OrganizationID *int `gorm:"column:organization_id"`
	CompanyID      *int `gorm:"column:company_id"`
}

// canSeeRequiredApproval решает, виден ли вызывающему признак обязательного согласующего
// (#2013) для конкретной записи организации/компании targetID - внутренняя карта того,
// кто в ней проводит решения. Открыт двум кругам:
//  1. тем, у кого есть право на раздел справочников - админ видит любую запись;
//  2. заявителю, который спрашивает про СВОЮ организацию/компанию - ровно это читает
//     форма подачи заявки (CreateApplication.vue), чтобы показать дефолтных согласующих
//     и корректно проставить их при отправке. Чужая запись для не-админа остаётся
//     закрытой - маршрут открыт всем вошедшим, и без этого любой мог узнать карту
//     согласования произвольной организации, зная только её id.
//
// own выбирает нужное поле (OrganizationID для /organizations, CompanyID для /companies).
func canSeeRequiredApproval(c echo.Context, db *gorm.DB, resolver *services.PermissionResolver, targetID int, own func(callerOwnDirectoryIDs) *int) bool {
	ctx := c.Request().Context()
	userID, _ := c.Get("user_id").(int)

	allowed, err := resolver.HasPermission(ctx, userID, services.KeyPageAdminDirectories)
	if err != nil {
		// Не даём сбою проверки права раскрыть служебное поле - при ошибке считаем,
		// что права нет, но саму ошибку не глотаем молча.
		slog.Error("Не удалось проверить право на служебные поля справочника", "error", err)
	} else if allowed {
		return true
	}

	var caller callerOwnDirectoryIDs
	if err := db.WithContext(ctx).Table("users").Select("organization_id, company_id").
		Where("id = ?", userID).Scan(&caller).Error; err != nil {
		slog.Error("Не удалось определить организацию/компанию вызывающего", "error", err)
		return false
	}
	ownID := own(caller)
	return ownID != nil && *ownID == targetID
}
