package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"systemburo/internal/apperr"

	"gorm.io/gorm"
)

// SearchService -- сквозной поиск по разделам системы.
type SearchService interface {
	Search(ctx context.Context, userID int, q string, types []SearchEntityType, limit int) (*SearchResponse, error)
}

const (
	// searchMinQueryLen -- порог, ниже которого поиск не запускается. Не косметика:
	// GIN индексирует триграммы, и паттерн ILIKE короче трёх символов индексом не
	// покрывается -- запрос уходит в полный скан сразу по всем разделам.
	searchMinQueryLen  = 3
	searchMaxQueryLen  = 100
	searchDefaultLimit = 5
	searchMaxLimit     = 20

	// searchMaxParallel -- потолок одновременных запросов к базе на один ввод. Пул
	// соединений в приложении явно не настроен, поэтому реальный предел -- это
	// max_connections сервера базы; веер шириной во все разделы при десятке
	// одновременно набирающих пользователей упёрся бы в него.
	searchMaxParallel = 4

	// Бюджеты. Провайдер, не уложившийся в свой, попадает в degraded, остальные
	// разделы отдаются -- поиск отвечает частично, но отвечает.
	searchProviderBudget = 800 * time.Millisecond
	searchTotalBudget    = 2 * time.Second
)

type searchService struct {
	db        *gorm.DB
	resolver  *PermissionResolver
	providers []searchProvider
	cache     *searchCache
}

// NewSearchService создаёт сервис сквозного поиска. Реестр провайдеров проверяется
// здесь, на старте приложения: провайдер без права, доехавший до рантайма, означал бы
// раздел, открытый всем подряд.
func NewSearchService(db *gorm.DB, resolver *PermissionResolver) (SearchService, error) {
	ps := searchProviderOrder()
	if err := validateSearchProviders(ps); err != nil {
		return nil, err
	}
	return &searchService{db: db, resolver: resolver, providers: ps, cache: newSearchCache()}, nil
}

func (s *searchService) Search(ctx context.Context, userID int, q string, types []SearchEntityType, limit int) (*SearchResponse, error) {
	started := time.Now()

	raw := strings.TrimSpace(q)
	if utf8.RuneCountInString(raw) < searchMinQueryLen {
		return nil, apperr.Validation("Запрос должен быть не короче 3 символов")
	}
	if utf8.RuneCountInString(raw) > searchMaxQueryLen {
		return nil, apperr.Validation("Слишком длинный запрос")
	}
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	set, err := s.resolver.Resolve(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("сквозной поиск: не удалось получить права: %w", err)
	}
	// У забаненного Has всегда false, поэтому ни один провайдер не прошёл бы отбор и
	// ответ вышел бы пустым. Отвечаем явным отказом: пустая выдача читается как
	// "ничего не найдено" и прячет блокировку учётной записи.
	if set.IsBanned() {
		return nil, apperr.Forbidden("Учётная запись заблокирована")
	}

	selected := s.selectProviders(set, types)
	if len(selected) == 0 {
		return &SearchResponse{Query: raw, Groups: []SearchGroup{}, TookMS: time.Since(started).Milliseconds()}, nil
	}

	// Кэш проверяется после разбора прав: иначе смена роли не вступала бы в силу до
	// истечения записи, а сам ключ пришлось бы городить поверх набора прав.
	if cached := s.cache.get(userID, raw, limit); cached != nil {
		return cached, nil
	}

	req, err := s.buildRequest(ctx, userID, raw, limit, set)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, searchTotalBudget)
	defer cancel()

	results, degraded := s.fanOut(ctx, selected, req)
	if len(degraded) == len(selected) {
		return nil, apperr.New(http.StatusServiceUnavailable, "Поиск временно недоступен")
	}

	resp := s.assemble(raw, limit, selected, results, degraded)
	resp.TookMS = time.Since(started).Milliseconds()
	s.cache.set(userID, raw, limit, resp)
	return resp, nil
}

// fanOut опрашивает выбранные разделы параллельно.
//
// Сознательно без errgroup: тот отменяет соседей по первой ошибке, а здесь падение
// одного раздела не должно уносить остальные. Ошибка провайдера -- это строка в
// degraded, а не провал запроса.
func (s *searchService) fanOut(ctx context.Context, selected []searchProvider, req searchRequest) (map[SearchEntityType][]SearchItem, []SearchEntityType) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		results  = make(map[SearchEntityType][]SearchItem, len(selected))
		degraded []SearchEntityType
		sem      = make(chan struct{}, searchMaxParallel)
	)

	for _, p := range selected {
		wg.Add(1)
		go func(p searchProvider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pctx, pcancel := context.WithTimeout(ctx, searchProviderBudget)
			defer pcancel()

			items, err := p.Search(pctx, s.db, req)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				slog.Warn("сквозной поиск: раздел не ответил", "type", p.Type(), "error", err)
				degraded = append(degraded, p.Type())
				return
			}
			results[p.Type()] = items
		}(p)
	}
	wg.Wait()

	return results, degraded
}

// assemble собирает ответ в порядке реестра, обрезает лишнюю строку до has_more и
// поднимает наверх раздел с точным совпадением.
func (s *searchService) assemble(raw string, limit int, selected []searchProvider, results map[SearchEntityType][]SearchItem, degraded []SearchEntityType) *SearchResponse {
	groups := make([]SearchGroup, 0, len(selected))
	total := 0

	for _, p := range selected {
		items, ok := results[p.Type()]
		if !ok || len(items) == 0 {
			continue
		}

		// Провайдер берёт limit+1 строку: лишняя не отдаётся, а лишь сообщает, что за
		// пределами превью есть ещё -- точный счётчик пользователь увидит в разделе.
		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}
		rankItems(items, raw)

		groups = append(groups, SearchGroup{
			Type:    p.Type(),
			Title:   p.Title(),
			Count:   len(items),
			HasMore: hasMore,
			Items:   items,
		})
		total += len(items)
	}

	// Раздел с точным совпадением встаёт первым: набравший госномер целиком ждёт
	// машину сверху. Единственное отступление от фиксированного порядка разделов.
	for i, g := range groups {
		if hasExactMatch(g) {
			copy(groups[1:i+1], groups[:i])
			groups[0] = g
			break
		}
	}

	return &SearchResponse{
		Query:    raw,
		Groups:   groups,
		Total:    total,
		Degraded: degraded,
	}
}

// selectProviders отсеивает разделы, на которые у пользователя нет права, и применяет
// пользовательский фильтр types. Проверка идёт через PermissionSet.Has -- он уже
// учитывает бан, super/admin и super-only ключи, собственной логики ролей здесь нет.
func (s *searchService) selectProviders(set PermissionSet, types []SearchEntityType) []searchProvider {
	want := make(map[SearchEntityType]struct{}, len(types))
	for _, t := range types {
		want[t] = struct{}{}
	}

	out := make([]searchProvider, 0, len(s.providers))
	for _, p := range s.providers {
		if len(want) > 0 {
			if _, ok := want[p.Type()]; !ok {
				continue
			}
		}
		if !set.Has(p.PermissionKey()) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// buildRequest читает данные видимости пользователя один раз на запрос: иначе каждый
// раздел лез бы за организацией и компанией в users отдельным запросом.
func (s *searchService) buildRequest(ctx context.Context, userID int, raw string, limit int, set PermissionSet) (searchRequest, error) {
	var owner struct {
		OrganizationID *int `gorm:"column:organization_id"`
		CompanyID      *int `gorm:"column:company_id"`
	}
	if err := s.db.WithContext(ctx).Table("users").
		Select("organization_id, company_id").
		Where("id = ?", userID).
		Scan(&owner).Error; err != nil {
		return searchRequest{}, fmt.Errorf("сквозной поиск: не удалось прочитать данные пользователя: %w", err)
	}

	// Принимающий видит все заявки -- это снимает фильтр видимости целиком, поэтому
	// признак читается здесь, вместе с остальными данными видимости, а не в провайдере.
	var approverCount int64
	if err := s.db.WithContext(ctx).Table("application_approvers").
		Where("user_id = ?", userID).
		Count(&approverCount).Error; err != nil {
		return searchRequest{}, fmt.Errorf("сквозной поиск: не удалось проверить роль принимающего: %w", err)
	}

	return searchRequest{
		Raw:             raw,
		Variants:        buildSearchVariantsFor(raw),
		Limit:           limit,
		UserID:          userID,
		Perms:           set,
		OrgID:           owner.OrganizationID,
		CompanyID:       owner.CompanyID,
		CanSeeAllSystem: searchCanSeeAllSystem(ctx, s.db, userID),
		IsApprover:      approverCount > 0,
	}, nil
}
