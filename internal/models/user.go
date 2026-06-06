package models

import "time"

type User struct {
	ID               int          `json:"id"`
	Username         string       `gorm:"uniqueIndex;size:100" json:"username"`
	Password         string       `gorm:"size:255" json:"-"`
	OrganizationID   *int         `json:"organization_id"`
	Organization     Organization `json:"organization,omitempty"`
	CompanyID        *int         `json:"company_id"`
	Company          Company      `json:"company,omitempty"`
	TypeID           int          `gorm:"default:1" json:"type_id"`
	UserType         UserType     `gorm:"foreignKey:TypeID" json:"user_type,omitempty"`
	RoleID           *int         `json:"role_id"`
	Role             *Role        `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	IsSuperAdmin     bool         `gorm:"default:false;index" json:"is_super_admin"`
	IsActive         bool         `gorm:"default:true;index" json:"is_active"`
	IsBanned         bool         `gorm:"default:false;index" json:"is_banned"`
	BannedAt         *time.Time   `json:"banned_at,omitempty"`
	BannedBy         *int         `json:"banned_by,omitempty"`
	LastName         *string      `gorm:"size:100" json:"last_name"`
	FirstName        *string      `gorm:"size:100" json:"first_name"`
	MiddleName       *string      `gorm:"size:100" json:"middle_name"`
	Position         *string      `gorm:"size:100;column:position" json:"position"`
	Email            *string      `gorm:"size:100" json:"email"`
	Phone            *string      `gorm:"size:20" json:"phone"`
	LastLoginAt      *time.Time   `json:"last_login_at,omitempty"`
	FailedLoginCount int          `gorm:"default:0" json:"-"`
	LockedUntil      *time.Time   `json:"-"`
}

type Organization struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:100" json:"name"`
	// IsActive - архивный флаг (soft-delete). Уникальность name обеспечивается
	// partial unique index (WHERE is_active=true) в migrate.go, а не gorm-тегом,
	// чтобы архивная запись не блокировала создание новой активной с тем же именем.
	IsActive bool `gorm:"default:true;index" json:"is_active"`
}

type Company struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:100" json:"name"`
	// IsActive - архивный флаг (soft-delete). Уникальность name - partial unique
	// index (WHERE is_active=true) в migrate.go, см. Organization.
	IsActive bool `gorm:"default:true;index" json:"is_active"`
}

type UserType struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:50" json:"name"`
	Code string `gorm:"uniqueIndex;size:20" json:"code"`
	// IsSystem помечает встроенные типы, чьи code используются в авторизации
	// (internal/auth/permissions.go). Такие типы нельзя переименовать или удалить.
	IsSystem bool `gorm:"default:false" json:"is_system"`
}

// --- Users management DTOs ---

// UserInfoResponse — ответ с полной информацией о пользователе (JSON поля совпадают с Rust UserInfo).
type UserInfoResponse struct {
	ID             int     `json:"id"`
	Username       string  `json:"username"`
	IsActive       bool    `json:"is_active"`
	IsBanned       bool    `json:"is_banned"`
	IsSuperAdmin   bool    `json:"is_super_admin"`
	Organization   *string `json:"organization"`
	OrganizationID *int    `json:"organization_id"`
	Company        *string `json:"company"`
	CompanyID      *int    `json:"company_id"`
	TypeID         int     `json:"type_id"`
	UserType       string  `json:"user_type"`
	RoleID         *int    `json:"role_id"`
	LastName       *string `json:"last_name"`
	FirstName      *string `json:"first_name"`
	MiddleName     *string `json:"middle_name"`
	Position       *string `json:"position"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
}

// UpdateUserTypeRequest — запрос на обновление типа пользователя.
type UpdateUserTypeRequest struct {
	TypeID int `json:"type_id" validate:"gte=1"`
}

// UpdatePasswordRequest — запрос на обновление пароля пользователя.
type UpdatePasswordRequest struct {
	Password string `json:"password" validate:"required,min=6,max=255"`
}

// UpdateUserInfoRequest — запрос на обновление персональных данных пользователя.
type UpdateUserInfoRequest struct {
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
	Position   *string `json:"position"`
	Email      *string `json:"email"`
	Phone      *string `json:"phone"`
}

// UpdateUserOrganizationRequest — запрос на обновление организации пользователя.
type UpdateUserOrganizationRequest struct {
	OrganizationID int `json:"organization_id" validate:"gte=1"`
}

// UpdateUserCompanyRequest — запрос на обновление компании пользователя.
type UpdateUserCompanyRequest struct {
	CompanyID int `json:"company_id" validate:"gte=1"`
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
