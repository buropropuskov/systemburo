package models

import "time"

type Citizenship struct {
	ID             int       `json:"id"`
	Name           string    `gorm:"size:100" json:"name"`
	Icon           *string   `gorm:"size:50" json:"icon"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	IsDefault      bool      `gorm:"default:false" json:"is_default"`
	PatentRequired bool      `gorm:"default:false" json:"patent_required"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateCitizenshipRequest -- запрос на создание гражданства.
type CreateCitizenshipRequest struct {
	Name           string  `json:"name" validate:"required,min=1,max=100"`
	Icon           *string `json:"icon"`
	IsDefault      *bool   `json:"is_default"`
	PatentRequired *bool   `json:"patent_required"`
}

// UpdateCitizenshipRequest -- запрос на обновление гражданства.
// is_active намеренно отсутствует: архивацией/восстановлением управляют
// DELETE (архив) и POST /restore, чтобы действие логировалось в историю и
// проходило проверку дефолта, а не менялось молча через PUT.
type UpdateCitizenshipRequest struct {
	Name           string  `json:"name" validate:"required,min=1,max=100"`
	Icon           *string `json:"icon"`
	IsDefault      *bool   `json:"is_default"`
	PatentRequired *bool   `json:"patent_required"`
}
