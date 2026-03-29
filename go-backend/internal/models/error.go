package models

// HTTPError — структура ошибки для Swagger документации.
type HTTPError struct {
	Message string `json:"message" example:"Error description"`
}
