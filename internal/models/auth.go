package models

// --- Requests ---

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=100"`
	// Password пустой означает «пароль придумает система и вышлет письмом».
	// Обязательность зависит от того, указан ли Email, поэтому её проверяет
	// сервис: тегом такое условие не выразить.
	Password       string  `json:"password" validate:"omitempty,min=6,max=255"`
	OrganizationID int     `json:"organization_id"`
	CompanyID      int     `json:"company_id"`
	TypeID         int     `json:"type_id"`
	LastName       *string `json:"last_name"`
	FirstName      *string `json:"first_name"`
	MiddleName     *string `json:"middle_name"`
	Position       *string `json:"position"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken      string `json:"refreshToken"`
	RefreshTokenSnake string `json:"refresh_token"`
}

// GetRefreshToken returns the refresh token from either camelCase or snake_case field.
func (r RefreshTokenRequest) GetRefreshToken() string {
	if r.RefreshToken != "" {
		return r.RefreshToken
	}
	return r.RefreshTokenSnake
}

type LogoutRequest struct {
	RefreshToken      string `json:"refreshToken"`
	RefreshTokenSnake string `json:"refresh_token"`
}

// GetRefreshToken returns the refresh token from either camelCase or snake_case field.
func (r LogoutRequest) GetRefreshToken() string {
	if r.RefreshToken != "" {
		return r.RefreshToken
	}
	return r.RefreshTokenSnake
}

// --- Responses ---

type LoginResponse struct {
	Token          string `json:"token"`
	RefreshToken   string `json:"refreshToken"`
	Organization   string `json:"organization"`
	OrganizationID *int   `json:"organization_id"`
	Company        string `json:"company"`
	CompanyID      *int   `json:"company_id"`
	TypeID         int    `json:"type_id"`
	UserType       string `json:"user_type"`
	IsSuperAdmin   bool   `json:"is_super_admin"`
	IsBanned       bool   `json:"is_banned"`
}

type TokenPairResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

type UserDataResponse struct {
	Username       string  `json:"username"`
	Organization   string  `json:"organization"`
	OrganizationID *int    `json:"organization_id"`
	Company        string  `json:"company"`
	CompanyID      *int    `json:"company_id"`
	LastName       *string `json:"last_name"`
	FirstName      *string `json:"first_name"`
	MiddleName     *string `json:"middle_name"`
	Phone          *string `json:"phone"`
}

type CurrentUserResponse struct {
	ID             int     `json:"id"`
	Username       string  `json:"username"`
	Organization   string  `json:"organization"`
	OrganizationID *int    `json:"organization_id"`
	Company        string  `json:"company"`
	CompanyID      *int    `json:"company_id"`
	TypeID         int     `json:"type_id"`
	UserType       string  `json:"user_type"`
	UserTypeCode   string  `json:"user_type_code"`
	IsSuperAdmin   bool    `json:"is_super_admin"`
	IsBanned       bool    `json:"is_banned"`
	LastName       *string `json:"last_name"`
	FirstName      *string `json:"first_name"`
	MiddleName     *string `json:"middle_name"`
	Position       *string `json:"position"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
}
