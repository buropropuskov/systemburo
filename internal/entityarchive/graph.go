package entityarchive

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

// TableCount - строка отчёта: таблица и сколько её строк принадлежит цели.
type TableCount struct {
	Table string
	Rows  int64
}

// Graph - результат сборки: непустые узлы и их счётчики.
type Graph struct {
	Type   string
	ID     int
	Tables []TableCount
}

// Total - сколько строк во всём графе.
func (g Graph) Total() int64 {
	var n int64
	for _, t := range g.Tables {
		n += t.Rows
	}
	return n
}

// Collect собирает граф цели: считает строки каждого узла. Только SELECT count(*),
// база не меняется. Пустые узлы в результат не попадают, чтобы отчёт показывал лишь
// реально задетые таблицы.
//
// Ошибка на любом узле прерывает сборку и возвращается наверх: молча пропустить
// узел с битым предикатом значило бы показать неполный граф как полный - именно на
// таком «зелёном, но неверном» экспорте потерялись бы данные при реимпорте.
func Collect(ctx context.Context, db *gorm.DB, entityType string, id int) (Graph, error) {
	if entityType != TypeOrganization {
		return Graph{}, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}
	g := Graph{Type: entityType, ID: id}
	for _, node := range organizationNodes() {
		var rows int64
		q := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", node.Table, node.Where)
		if err := db.WithContext(ctx).Raw(q, sql.Named("org", id)).Scan(&rows).Error; err != nil {
			return Graph{}, fmt.Errorf("подсчёт строк %s: %w", node.Table, err)
		}
		if rows > 0 {
			g.Tables = append(g.Tables, TableCount{Table: node.Table, Rows: rows})
		}
	}
	return g, nil
}
