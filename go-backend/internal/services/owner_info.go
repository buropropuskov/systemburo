package services

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OwnerInfo содержит информацию о владельце (организация/компания) для фильтрации.
type OwnerInfo struct {
	HasOrganization bool `json:"has_organization"`
	HasCompany      bool `json:"has_company"`
	OrganizationID  *int `json:"organization_id"`
	CompanyID       *int `json:"company_id"`
	UserID          int  `json:"user_id"`
}

// getOwnerInfo получает информацию о владельце по username.
func getOwnerInfo(db *gorm.DB, ctx context.Context, username string) (*OwnerInfo, error) {
	var result struct {
		UserID          int  `gorm:"column:user_id"`
		OrganizationID  *int `gorm:"column:organization_id"`
		CompanyID       *int `gorm:"column:company_id"`
		HasOrganization bool `gorm:"column:has_organization"`
		HasCompany      bool `gorm:"column:has_company"`
	}

	err := db.WithContext(ctx).
		Table("users u").
		Select(`u.id as user_id, u.organization_id, u.company_id,
			CASE WHEN o.id IS NOT NULL THEN true ELSE false END as has_organization,
			CASE WHEN c.id IS NOT NULL THEN true ELSE false END as has_company`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Where("u.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user info")
	}

	return &OwnerInfo{
		HasOrganization: result.HasOrganization,
		HasCompany:      result.HasCompany,
		OrganizationID:  result.OrganizationID,
		CompanyID:       result.CompanyID,
		UserID:          result.UserID,
	}, nil
}
