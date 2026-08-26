package services

import (
	"strconv"
	"sync"
	"time"
)

// Короткий кэш ответов поиска. Ловит повторный набор, Backspace и "закрыл-открыл
// палитру": подсказки летят на каждый ввод, и без кэша один и тот же запрос уходит в
// базу по нескольку раз за секунду.
const (
	searchCacheTTL = 10 * time.Second
	// searchCacheMaxEntries -- потолок, после которого кэш сбрасывается целиком.
	// Вытеснение по одному не нужно: записи живут 10 секунд, за это время до потолка
	// доходит только всплеск, а полный сброс дешевле поддержки LRU-списка.
	searchCacheMaxEntries = 512
)

type searchCacheEntry struct {
	resp      *SearchResponse
	expiresAt time.Time
}

type searchCache struct {
	mu      sync.RWMutex
	entries map[string]searchCacheEntry
}

func newSearchCache() *searchCache {
	return &searchCache{entries: make(map[string]searchCacheEntry, 64)}
}

// key включает userID обязательно: выдача уже сужена правами и видимостью
// пользователя, и общий на всех ключ раздавал бы чужие записи первому попавшемуся.
func (c *searchCache) key(userID int, query string, limit int) string {
	return strconv.Itoa(userID) + "\x00" + strconv.Itoa(limit) + "\x00" + query
}

func (c *searchCache) get(userID int, query string, limit int) *SearchResponse {
	c.mu.RLock()
	e, ok := c.entries[c.key(userID, query, limit)]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil
	}
	return e.resp
}

func (c *searchCache) set(userID int, query string, limit int, resp *SearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= searchCacheMaxEntries {
		c.entries = make(map[string]searchCacheEntry, 64)
	}
	c.entries[c.key(userID, query, limit)] = searchCacheEntry{
		resp:      resp,
		expiresAt: time.Now().Add(searchCacheTTL),
	}
}
