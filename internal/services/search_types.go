package services

// Контракт сквозного поиска (GET /api/search): типы результата и внутренний запрос,
// общий для всех провайдеров разделов.

// SearchEntityType -- машинный код группы результатов. Значение уходит в API
// (group.type, item.type) и в маппинг deep-link на фронте, поэтому смена значения --
// ломающее изменение контракта, а не переименование.
type SearchEntityType string

const (
	SearchTypeEmployees    SearchEntityType = "employees"
	SearchTypeCars         SearchEntityType = "cars"
	SearchTypeApplications SearchEntityType = "applications"
	SearchTypeDirectories  SearchEntityType = "directories"
	SearchTypeUsers        SearchEntityType = "users"
	SearchTypeBlacklist    SearchEntityType = "blacklist"
	SearchTypeContent      SearchEntityType = "content"
	SearchTypeFeedback     SearchEntityType = "feedback"
)

// SearchTarget -- адрес перехода из результата. Отдаём сущность и её id, а не готовый
// путь: маршруты живут во фронтовом router.js и меняются без ведома бэка, маппинг
// entity -> route держит фронт в одном месте.
type SearchTarget struct {
	Entity string `json:"entity"`
	ID     int    `json:"id"`
}

// SearchItem -- одна найденная строка. Полей паспорта и патента здесь нет и быть не
// может: они зашифрованы в БД, а их выдача в поиске означала бы канал к персональным
// данным шире, чем у листинга сущности (152-ФЗ).
type SearchItem struct {
	ID           int              `json:"id"`
	Type         SearchEntityType `json:"type"`
	Title        string           `json:"title"`
	Subtitle     string           `json:"subtitle,omitempty"`
	MatchedField string           `json:"matched_field,omitempty"`
	Score        float64          `json:"score"`
	Target       SearchTarget     `json:"target"`
}

// SearchGroup -- результаты одного раздела. Count никогда не больше числа отданных
// Items: точного счётчика нет намеренно, COUNT(*) удвоил бы число запросов и обесценил
// раннюю остановку по LIMIT. Точное число пользователь видит на целевой странице.
type SearchGroup struct {
	Type    SearchEntityType `json:"type"`
	Title   string           `json:"title"`
	Count   int              `json:"count"`
	HasMore bool             `json:"has_more"`
	Items   []SearchItem     `json:"items"`
}

// SearchResponse -- тело data ответа GET /api/search.
type SearchResponse struct {
	Query    string             `json:"query"`
	Groups   []SearchGroup      `json:"groups"`
	Total    int                `json:"total"`
	TookMS   int64              `json:"took_ms"`
	Degraded []SearchEntityType `json:"degraded,omitempty"`
}

// searchRequest -- нормализованный вход одного поиска, общий для всех провайдеров.
// Скоуп-данные (OrgID/CompanyID/CanSeeAllSystem) читаются ОДИН раз в searchService.Search
// и передаются готовыми: иначе каждый провайдер лез бы за ними в users отдельным запросом.
type searchRequest struct {
	Raw      string   // сырой ввод: нужен точному сравнению в ORDER BY и ранжированию
	Variants []string // варианты запроса (раскладка, госномер), см. buildSearchVariantsFor
	Limit    int
	UserID   int
	Perms    PermissionSet

	OrgID           *int
	CompanyID       *int
	CanSeeAllSystem bool
	// IsApprover снимает фильтр видимости заявок целиком -- принимающий видит все.
	// Читается один раз на запрос, как и остальные данные видимости.
	IsApprover bool
}

// searchRow -- приёмник строк провайдера. Структура плоская намеренно: gorm .Scan в
// структуру со встроенной анонимной структурой молча не заполняет её поля, и запрос
// возвращает нули при зелёной сборке.
type searchRow struct {
	ID       int    `gorm:"column:id"`
	Title    string `gorm:"column:title"`
	Subtitle string `gorm:"column:subtitle"`
}

// Значения SearchItem.MatchedField: подсказка фронту, какую из двух строк подсвечивать.
// Точную колонку не отдаём -- вычислять её в SQL значило бы гонять CASE по всем полям
// раздела ради подсветки.
const (
	matchedFieldTitle    = "title"
	matchedFieldSubtitle = "subtitle"
)
