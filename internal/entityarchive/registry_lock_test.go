package entityarchive

import (
	"sync"
	"testing"

	"systemburo/internal/database"

	"gorm.io/gorm/schema"
)

// TestOrganizationRoots_CoverAllOrgScopedModels - замок полноты графа организации.
//
// Любая модель из database.AllModels() с полем OrganizationID обязана присутствовать в
// directOrgRoots (и, значит, иметь узел в organizationNodes). Иначе новую таблицу,
// привязанную к организации, тихо забудут внести в выгрузку: экспорт пройдёт зелёным, а
// при обратном импорте недостача всплывёт потерей данных. Тест ловит это в момент
// добавления таблицы, а не на реальной выгрузке.
//
// schema.Parse разбирает модель без соединения с базой (нужен только для точного имени
// таблицы, учитывающего TableName-оверрайды), поэтому тест быстрый и не требует стенда.
func TestOrganizationRoots_CoverAllOrgScopedModels(t *testing.T) {
	cache := &sync.Map{}
	namer := schema.NamingStrategy{}
	for _, m := range database.AllModels() {
		s, err := schema.Parse(m, cache, namer)
		if err != nil {
			t.Fatalf("не разобралась схема модели %T: %v", m, err)
		}
		if _, ok := s.FieldsByName["OrganizationID"]; !ok {
			continue
		}
		if !directOrgRoots[s.Table] {
			t.Errorf("модель %s (таблица %s) имеет OrganizationID, но её нет в directOrgRoots: "+
				"добавь узел в organizationNodes и таблицу в directOrgRoots, иначе выгрузка "+
				"организации будет неполной", s.Name, s.Table)
		}
	}
}

// TestOrganizationRoots_AllPresentInNodes держит directOrgRoots и organizationNodes в
// согласии: корень, объявленный прямым, обязан реально встречаться узлом графа.
func TestOrganizationRoots_AllPresentInNodes(t *testing.T) {
	inNodes := make(map[string]bool)
	for _, n := range organizationNodes() {
		inNodes[n.Table] = true
	}
	for table := range directOrgRoots {
		if !inNodes[table] {
			t.Errorf("таблица %s числится в directOrgRoots, но узла в organizationNodes для неё нет", table)
		}
	}
}
