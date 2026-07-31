package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// searchProvider -- один раздел сквозного поиска.
//
// Контракт безопасности: провайдер ОБЯЗАН сам сузить свои строки до тех, что
// пользователь вправе видеть. PermissionKey отвечает на вопрос "показывать ли раздел"
// и НЕ заменяет фильтр строк -- право на страницу это не право на все её данные.
// Подмена одного другим -- причина закрытых дыр #1524/#1528/#1530/#1531, и сквозной
// поиск даёт для неё самую широкую поверхность в системе.
//
// Скоуп обязан зеркалить фильтр листинга той же сущности целиком, а не его часть;
// в комментарии провайдера указывается, какую именно функцию он зеркалит.
type searchProvider interface {
	// Type -- машинный код группы, уходит в API.
	Type() SearchEntityType
	// Title -- заголовок группы для интерфейса.
	Title() string
	// PermissionKey -- право, без которого провайдер не запускается. Пустая строка
	// запрещена, см. validateSearchProviders.
	PermissionKey() string
	// Search выполняет один запрос и возвращает не более req.Limit+1 строк: лишняя --
	// признак has_more, наружу она не отдаётся.
	Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error)
}

// searchProviderOrder -- фиксированный порядок групп в выдаче.
//
// Единого сквозного score между разделами нет намеренно: similarity короткого поля
// ("Роголев Иван Петрович") и длинного (текст заявки со словом "Роголев") измеряют
// разное, короткое всегда выигрывает, и общий рейтинг систематически задвигал бы
// заявки вниз. Порядок здесь = порядок групп в ответе, кроме подъёма группы с точным
// совпадением наверх (см. search_rank.go).
func searchProviderOrder() []searchProvider {
	return []searchProvider{
		uniqueEmployeeSearchProvider{},
		uniqueCarSearchProvider{},
		applicationSearchProvider{},
		userSearchProvider{},
		blacklistSearchProvider{},
		directorySearchProvider{},
		contentSearchProvider{},
		feedbackSearchProvider{},
	}
}

// IsKnownSearchType сообщает, есть ли такой раздел в реестре. Нужен обработчику, чтобы
// отличить опечатку в параметре types от раздела, на который просто нет прав: первое --
// ошибка запроса, второе -- штатно отсутствующая группа.
func IsKnownSearchType(t SearchEntityType) bool {
	for _, p := range searchProviderOrder() {
		if p.Type() == t {
			return true
		}
	}
	return false
}

// rowsToItems переводит строки провайдера в элементы выдачи. Score и MatchedField
// проставляются позже, в rankItems: они зависят от запроса целиком, а не от строки.
func rowsToItems(t SearchEntityType, entity string, rows []searchRow) []SearchItem {
	items := make([]SearchItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SearchItem{
			ID:       r.ID,
			Type:     t,
			Title:    r.Title,
			Subtitle: r.Subtitle,
			Target:   SearchTarget{Entity: entity, ID: r.ID},
		})
	}
	return items
}

// validateSearchProviders -- проверка реестра на старте, до первого запроса.
// Провайдер без права или с чужим ключом не должен доехать до рантайма: там его
// отсутствие означало бы раздел, открытый всем подряд.
func validateSearchProviders(ps []searchProvider) error {
	seen := make(map[SearchEntityType]struct{}, len(ps))
	for _, p := range ps {
		key := p.PermissionKey()
		if key == "" {
			return fmt.Errorf("search provider %q без permission-ключа", p.Type())
		}
		if !IsValidKey(key) {
			return fmt.Errorf("search provider %q: неизвестный ключ %q", p.Type(), key)
		}
		if _, dup := seen[p.Type()]; dup {
			return fmt.Errorf("search provider: дубль типа %q", p.Type())
		}
		seen[p.Type()] = struct{}{}
	}
	return nil
}
