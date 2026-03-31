package models

import "time"

type LicensePlateFormat struct {
	ID          int       `json:"id"`
	Name        string    `gorm:"size:100" json:"name"`
	CountryCode *string   `gorm:"size:10" json:"country_code"`
	Icon        *string   `gorm:"size:50" json:"icon"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	IsDefault   bool      `gorm:"default:false" json:"is_default"`
}

type LicensePlateFormatCell struct {
	ID             int                `json:"id"`
	FormatID       int                `gorm:"index" json:"format_id"`
	Format         LicensePlateFormat `gorm:"foreignKey:FormatID;constraint:OnDelete:CASCADE" json:"-"`
	CellOrder      int                `json:"cell_order"`
	CellType       string             `gorm:"size:20" json:"cell_type"`
	MinLength      *int               `json:"min_length"`
	MaxLength      *int               `json:"max_length"`
	AllowedLetters *string            `gorm:"size:100" json:"allowed_letters"`
	AlphabetType   *string            `gorm:"size:20" json:"alphabet_type"`
	Language       *string            `gorm:"size:10" json:"language"`
	PaddingChar    *string            `gorm:"size:5" json:"padding_char"`
	PaddingSide    *string            `gorm:"size:10" json:"padding_side"`
	CreatedAt      time.Time          `json:"created_at"`
}

// LicensePlateFormatWithCells — формат номера с вложенными ячейками.
// JSON-поля совпадают с Rust-ответом: {"format": {...}, "cells": [...]}.
type LicensePlateFormatWithCells struct {
	Format LicensePlateFormat       `json:"format"`
	Cells  []LicensePlateFormatCell `json:"cells"`
}

// CreateLicensePlateFormatRequest — запрос на создание формата номерного знака.
type CreateLicensePlateFormatRequest struct {
	Name        string                    `json:"name" validate:"required,min=1,max=100"`
	CountryCode *string                   `json:"country_code"`
	Icon        *string                   `json:"icon"`
	IsDefault   *bool                     `json:"is_default"`
	Cells       []CreateFormatCellRequest `json:"cells"`
}

// CreateFormatCellRequest — ячейка формата в запросе на создание.
type CreateFormatCellRequest struct {
	CellOrder      int     `json:"cell_order"`
	CellType       string  `json:"cell_type"`
	MinLength      *int    `json:"min_length"`
	MaxLength      *int    `json:"max_length"`
	AllowedLetters *string `json:"allowed_letters"`
	AlphabetType   *string `json:"alphabet_type"`
	Language       *string `json:"language"`
	PaddingChar    *string `json:"padding_char"`
	PaddingSide    *string `json:"padding_side"`
}

// UpdateLicensePlateFormatRequest — запрос на обновление формата номерного знака.
type UpdateLicensePlateFormatRequest struct {
	Name        string                    `json:"name" validate:"required,min=1,max=100"`
	CountryCode *string                   `json:"country_code"`
	Icon        *string                   `json:"icon"`
	IsDefault   *bool                     `json:"is_default"`
	Cells       []UpdateFormatCellRequest `json:"cells"`
}

// UpdateFormatCellRequest — ячейка формата в запросе на обновление.
type UpdateFormatCellRequest struct {
	ID             *int    `json:"id"`
	CellOrder      int     `json:"cell_order"`
	CellType       string  `json:"cell_type"`
	MinLength      *int    `json:"min_length"`
	MaxLength      *int    `json:"max_length"`
	AllowedLetters *string `json:"allowed_letters"`
	AlphabetType   *string `json:"alphabet_type"`
	Language       *string `json:"language"`
	PaddingChar    *string `json:"padding_char"`
	PaddingSide    *string `json:"padding_side"`
}

// CreateFormatResponse — ответ после создания формата.
type CreateFormatResponse struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}
