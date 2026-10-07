package api

import (
	"strconv"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var maxLimit = 100

// SetMaxLimit configures the maximum pagination limit.
func SetMaxLimit(limit int) { maxLimit = limit }

// PaginationParams содержит параметры пагинации, извлечённые из запроса.
type PaginationParams struct {
	Page  int
	Limit int
}

// ParsePagination извлекает page и limit из query-параметров запроса.
// При отсутствии параметров используются значения по умолчанию (page=1, limit=defaultLimit).
func ParsePagination(c echo.Context, defaultLimit int) PaginationParams {
	page := 1
	limit := defaultLimit

	if v := c.QueryParam("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}

	if v := c.QueryParam("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	return PaginationParams{Page: page, Limit: limit}
}

// ApplyPagination применяет offset/limit к GORM-запросу на основании параметров пагинации.
func ApplyPagination(db *gorm.DB, p PaginationParams) *gorm.DB {
	offset, err := models.CheckedOffset(p.Page, p.Limit)
	if err != nil {
		db.AddError(err)
		return db
	}
	return db.Offset(offset).Limit(p.Limit)
}
