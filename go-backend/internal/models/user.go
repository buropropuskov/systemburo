package models

import "time"

type User struct {
	ID             int          `json:"id"`
	Username       string       `gorm:"uniqueIndex;size:100" json:"username"`
	Password       string       `gorm:"size:255" json:"-"`
	OrganizationID int          `json:"organization_id"`
	Organization   Organization `json:"organization,omitempty"`
	CompanyID      int          `json:"company_id"`
	Company        Company      `json:"company,omitempty"`
	TypeID         int          `gorm:"default:1" json:"type_id"`
	UserType       UserType     `gorm:"foreignKey:TypeID" json:"user_type,omitempty"`
	LastName       *string      `gorm:"size:100" json:"last_name"`
	FirstName      *string      `gorm:"size:100" json:"first_name"`
	MiddleName     *string      `gorm:"size:100" json:"middle_name"`
	Position       *string      `gorm:"size:100;column:position" json:"position"`
	Email          *string      `gorm:"size:100" json:"email"`
	Phone          *string      `gorm:"size:20" json:"phone"`
	LastLoginAt    *time.Time   `json:"last_login_at,omitempty"`
}

type Organization struct {
	ID   int    `json:"id"`
	Name string `gorm:"uniqueIndex;size:100" json:"name"`
}

type Company struct {
	ID   int    `json:"id"`
	Name string `gorm:"uniqueIndex;size:100" json:"name"`
}

type UserType struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:50" json:"name"`
	Code string `gorm:"uniqueIndex;size:20" json:"code"`
}

// Junction tables

type OrganizationUser struct {
	ID               int          `json:"id"`
	OrganizationID   int          `gorm:"index" json:"organization_id"`
	Organization     Organization `json:"-"`
	UserID           int          `gorm:"index" json:"user_id"`
	User             User         `json:"-"`
	CreatedAt        *time.Time   `json:"created_at"`
	IsPrimary        bool         `gorm:"default:false" json:"is_primary"`
	RequiredApproval bool         `gorm:"default:false" json:"required_approval"`
}

type CompaniesUser struct {
	ID               int        `json:"id"`
	CompanyID        int        `gorm:"index" json:"company_id"`
	Company          Company    `json:"-"`
	UserID           int        `gorm:"index" json:"user_id"`
	User             User       `json:"-"`
	CreatedAt        *time.Time `json:"created_at"`
	IsPrimary        bool       `gorm:"default:false" json:"is_primary"`
	RequiredApproval bool       `gorm:"default:false" json:"required_approval"`
}
