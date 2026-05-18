package services

import (
	"errors"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// TrashDBRef - адаптер для TrashHandler.DBRef: один SQL-запрос на определение
// table_type (cars|people) по systemTableID. Вынесен в services чтобы не
// плодить GORM в handlers.
type TrashDBRef struct {
	db *gorm.DB
}

// NewTrashDBRef создаёт адаптер.
func NewTrashDBRef(db *gorm.DB) *TrashDBRef {
	return &TrashDBRef{db: db}
}

// GetTableType возвращает "cars"/"people" по ID таблицы.
func (r *TrashDBRef) GetTableType(tableID int) (string, error) {
	var st models.SystemTable
	if err := r.db.Select("id, table_type").First(&st, tableID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", echo.NewHTTPError(http.StatusNotFound, "Таблица не найдена")
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения таблицы")
	}
	return st.TableType, nil
}
