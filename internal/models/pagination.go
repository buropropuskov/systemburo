package models

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
