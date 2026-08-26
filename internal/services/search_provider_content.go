package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// contentSearchProvider ищет по новостям, объявлениям и документам -- всему, что живёт в
// разделе «Обзор и новости».
//
// В выдачу идут только опубликованные материалы: действующие новости и объявления,
// видимые документы. Черновики и снятые с публикации доступны управлению разделом, а не
// подсказкам поиска, и показывать их тому, кто их не откроет, значило бы дразнить
// ссылкой на 403.
//
// Три таблицы объединены одним запросом: у них общее право, общая форма строки
// (заголовок с описанием) и небольшой объём.
type contentSearchProvider struct{}

func (contentSearchProvider) Type() SearchEntityType { return SearchTypeContent }
func (contentSearchProvider) Title() string          { return "Новости и документы" }
func (contentSearchProvider) PermissionKey() string  { return KeyPageNews }

func (contentSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	// full_text новостей и объявлений -- размеченный текст целиком. Ищем по нему, но в
	// подзаголовок кладём короткое описание: показывать кусок разметки в подсказке
	// незачем.
	// По разметке целиком ищем только точное вхождение: нечёткое сравнение читает
	// значение целиком и на длинной новости стоит дороже всего запроса.
	textCols := []string{"title", "description", "full_text"}
	textFuzzy := []string{"title", "description"}
	newsCond, newsArgs := searchConditionFuzzyIn(textCols, textFuzzy, req.Raw)
	annCond, annArgs := searchConditionFuzzyIn(textCols, textFuzzy, req.Raw)
	docCond, docArgs := searchCondition([]string{"title", "description", "file_name"}, req.Raw)

	sql := fmt.Sprintf(`SELECT id, title, subtitle, kind FROM (
		(SELECT id, title, CONCAT_WS(' · ', 'Новость', NULLIF(description, '')) AS subtitle, 'news' AS kind
		 FROM news WHERE is_active AND (%s) ORDER BY id DESC LIMIT ?)
		UNION ALL
		(SELECT id, title, CONCAT_WS(' · ', 'Объявление', NULLIF(description, '')) AS subtitle, 'announcement' AS kind
		 FROM announcements WHERE is_active AND (%s) ORDER BY id DESC LIMIT ?)
		UNION ALL
		(SELECT id, title, CONCAT_WS(' · ', 'Документ', NULLIF(description, '')) AS subtitle, 'document' AS kind
		 FROM documents WHERE is_visible AND (%s) ORDER BY id DESC LIMIT ?)
	) c
	ORDER BY CASE
		WHEN LOWER(TRIM(title)) = LOWER(TRIM(?)) THEN 0
		WHEN LOWER(title) LIKE LOWER(?) || '%%' THEN 1
		ELSE 2 END, title
	LIMIT ?`, newsCond, annCond, docCond)

	args := make([]interface{}, 0, len(newsArgs)+len(annArgs)+len(docArgs)+6)
	args = append(args, newsArgs...)
	args = append(args, req.Limit+1)
	args = append(args, annArgs...)
	args = append(args, req.Limit+1)
	args = append(args, docArgs...)
	args = append(args, req.Limit+1)
	args = append(args, req.Raw, req.Raw, req.Limit+1)

	return scanKindedRows(ctx, db, SearchTypeContent, sql, args, "поиск по новостям и документам")
}

// feedbackSearchProvider ищет по обращениям обратной связи.
//
// Отдельным разделом, а не вместе с новостями: обращения закрыты своим правом
// (page.admin.feedback), тем же, что и их список. Своё обращение пользователь видит в
// личном разделе, и заводить для этого второй канал незачем -- поиск нужен тому, кто
// разбирает обращения.
type feedbackSearchProvider struct{}

func (feedbackSearchProvider) Type() SearchEntityType { return SearchTypeFeedback }
func (feedbackSearchProvider) Title() string          { return "Обратная связь" }
func (feedbackSearchProvider) PermissionKey() string  { return KeyPageAdminFeedback }

func (feedbackSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	// Текст обращения -- свободный и длинный, нечётко его не сравниваем: по нему ищут
	// точную фразу, а фамилию автора рядом ищут с опечаткой.
	cols := []string{"f.message", "u.last_name", "u.first_name", "u.middle_name", "u.username"}
	fuzzyCols := []string{"u.last_name", "u.first_name", "u.middle_name", "u.username"}
	cond, args := searchConditionFuzzyIn(cols, fuzzyCols, req.Raw)

	rows := make([]searchRow, 0, req.Limit+1)
	// Запрос строится на соединении транзакции: порог нечёткого сравнения выставлен
	// внутри неё, и построенный снаружи запрос его бы не увидел.
	if err := withTrigramThreshold(ctx, db, func(tx *gorm.DB) error {
		return tx.
			Table("feedback f").
			Joins("LEFT JOIN users u ON f.user_id = u.id").
			Select(`f.id AS id,
				LEFT(f.message, 120) AS title,
				CONCAT_WS(' · ',
					NULLIF(TRIM(CONCAT_WS(' ', u.last_name, u.first_name)), ''),
					NULLIF(f.status, '')) AS subtitle`).
			Where(cond, args...).
			Order("f.id DESC").
			Limit(req.Limit + 1).
			Scan(&rows).Error
	}); err != nil {
		return nil, fmt.Errorf("поиск по обращениям: %w", err)
	}

	return rowsToItems(SearchTypeFeedback, "feedback", rows), nil
}

// scanKindedRows выполняет запрос, где каждая строка несёт собственный код сущности, и
// переводит результат в элементы выдачи. Общий для разделов, собранных из нескольких
// таблиц: там одна группа ведёт на разные страницы.
func scanKindedRows(ctx context.Context, db *gorm.DB, t SearchEntityType, sql string, args []interface{}, errContext string) ([]SearchItem, error) {
	var rows []struct {
		ID       int    `gorm:"column:id"`
		Title    string `gorm:"column:title"`
		Subtitle string `gorm:"column:subtitle"`
		Kind     string `gorm:"column:kind"`
	}
	// В транзакции с выставленным порогом: у всех этих разделов условие содержит
	// нечёткое сравнение, а оно без порога взяло бы значение по умолчанию.
	err := withTrigramThreshold(ctx, db, func(tx *gorm.DB) error {
		return tx.Raw(sql, args...).Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errContext, err)
	}

	items := make([]SearchItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SearchItem{
			ID:       r.ID,
			Type:     t,
			Title:    r.Title,
			Subtitle: r.Subtitle,
			Target:   SearchTarget{Entity: r.Kind, ID: r.ID},
		})
	}
	return items, nil
}
