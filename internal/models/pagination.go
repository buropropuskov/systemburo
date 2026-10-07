package models

import "systemburo/internal/apperr"

// CheckedOffset computes the offset after the caller applies its paging policy.
func CheckedOffset(page, perPage int) (int, error) {
	if page < 1 || perPage < 1 {
		return 0, apperr.Validation("Invalid pagination parameters")
	}
	maxInt := int(^uint(0) >> 1)
	if page-1 > maxInt/perPage {
		return 0, apperr.Validation("Pagination offset is too large")
	}
	return (page - 1) * perPage, nil
}

// PaginationParams holds pagination query parameters.
type PaginationParams struct {
	Page    int `query:"page"`
	PerPage int `query:"per_page"`
}

// Normalize sets defaults for invalid values.
func (p *PaginationParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 || p.PerPage > 100 {
		p.PerPage = 20
	}
}

// PaginationMeta is returned alongside paginated data.
type PaginationMeta struct {
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
}
