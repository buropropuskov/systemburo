package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// applicationSearchProvider ищет по заявкам.
//
// Видимость зеркалит applyApplicationAccessFilter (application_helpers.go): своя заявка,
// заявка, где пользователь ответственный или наблюдатель; принимающий видит все.
//
// Набор колонок намеренно уже, чем у поиска в Центре заявок (applyApplicationFilters).
// Тот делает шесть EXISTS-подзапросов и вдобавок фильтр архива с приведением даты, из-за
// которого условие перестаёт опираться на индекс и вложения сканируются целиком. Центру
// это по карману -- он открывается по кнопке; подсказки же летят на каждый введённый
// символ. Здесь остаются номер заявки, текст письма, организация с компанией и два
// EXISTS -- по машинам и сотрудникам, ради которых поиск по заявкам и нужен ("найти
// заявку по госномеру или фамилии").
//
// ФИО в выдачу не идут вовсе, включая подзаголовок: у принимающих имена маскируются
// (approver_mask.go), и показывать их здесь значило бы обходить маскировку.
type applicationSearchProvider struct{}

func (applicationSearchProvider) Type() SearchEntityType { return SearchTypeApplications }
func (applicationSearchProvider) Title() string          { return "Заявки" }

// Раздел гейтится личным кабинетом, а не Центром заявок. Центр -- рабочее место
// принимающего, и права page.center у базовой роли нет; свои заявки рядовой пользователь
// видит именно в кабинете. Гейтить Центром значило бы отрезать от поиска собственные
// заявки тому, кто их и так видит, -- поиск обязан совпадать с тем, что человеку
// доступно, а не с тем, через какой раздел он туда попадает. Кто какие заявки увидит,
// решает applyApplicationAccessFilter внутри Search.
func (applicationSearchProvider) PermissionKey() string { return KeyPagePersonal }

func (applicationSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	// Тело письма ищется только точным вхождением: нечёткое сравнение просматривает
	// его целиком, а письма доходят до 70 килобайт (см. searchConditionFuzzyIn).
	cols := []string{"a.application_number", "a.message", "o.name", "c.name"}
	fuzzyCols := []string{"a.application_number", "o.name", "c.name"}
	cond, args := searchConditionFuzzyIn(cols, fuzzyCols, req.Raw)

	// car_brand -- устаревшая колонка марки, но в данных заполнена именно она: снимок
	// mark_name появился позже и есть у единиц записей. Ищем по обеим, иначе заявка не
	// находится по марке своей машины.
	carCols := []string{"cr.car_number", "cr.mark_name", "cr.car_brand"}
	carCond, carArgs := searchCondition(carCols, req.Raw)
	cond += fmt.Sprintf(` OR EXISTS(
		SELECT 1 FROM attachments att
		JOIN cars cr ON cr.attachment_id = att.id
		WHERE att.application_id = a.id AND (%s))`, carCond)
	args = append(args, carArgs...)

	empCols := []string{"e.last_name", "e.first_name", "e.middle_name"}
	empCond, empArgs := searchCondition(empCols, req.Raw)
	cond += fmt.Sprintf(` OR EXISTS(
		SELECT 1 FROM attachments att2
		JOIN employees e ON e.attachment_id = att2.id
		WHERE att2.application_id = a.id AND (%s))`, empCond)
	args = append(args, empArgs...)

	rows := make([]searchRow, 0, req.Limit+1)
	if err := withTrigramThreshold(ctx, db, func(tx *gorm.DB) error {
		q := tx.
			Table("applications a").
			Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
			Joins("LEFT JOIN companies c ON a.company_id = c.id").
			Select(`a.id AS id,
				CONCAT('Заявка ', COALESCE(a.application_number, CAST(a.id AS TEXT))) AS title,
				CONCAT_WS(' · ', COALESCE(o.name, c.name), NULLIF(a.status, '')) AS subtitle,
				`+matchRankExpr("a.application_number"), req.Raw, req.Raw).
			Where(cond, args...)

		q = applyApplicationAccessFilter(q, req.UserID, req.IsApprover)

		return q.
			Order("match_rank, a.id DESC").
			Limit(req.Limit + 1).
			Scan(&rows).Error
	}); err != nil {
		return nil, fmt.Errorf("поиск по заявкам: %w", err)
	}

	return rowsToItems(SearchTypeApplications, "application", rows), nil
}
