package services

import (
	"context"
	"fmt"
	"strings"

	"systemburo/internal/normalize"

	"gorm.io/gorm"
)

// applyRegistryScope сужает выборку реестра (unique_cars/unique_employees) до строк,
// которые пользователь вправе видеть. Зеркало ветки "all" из buildCarsQuery
// (unique_car_service.go) и buildEmployeesQuery (unique_employee_service.go): свои
// записи плюс записи своей организации и своей компании; полный срез системы -- только
// тем, кому его открывает searchCanSeeAllSystem.
//
// Скоуп применяется внутри провайдера, а не в хендлере: гейт эндпоинта и отсев по
// PermissionKey отвечают на "показывать ли раздел", но не на "какие строки раздела".
// Разведение этих двух вопросов и есть защита от повторения #1524/#1528.
//
// Ветки organization/company навешиваются только при непустом значении у пользователя.
// В реестрах на их месте подставляется 0, что безопасно (колонки nullable, id с
// единицы), но лишнее условие в OR ничего не находит и только удлиняет запрос.
func applyRegistryScope(q *gorm.DB, alias string, req searchRequest) *gorm.DB {
	if req.CanSeeAllSystem {
		return q
	}

	cond := fmt.Sprintf("%s.user_id = ?", alias)
	args := []interface{}{req.UserID}
	if req.OrgID != nil && *req.OrgID != 0 {
		cond += fmt.Sprintf(" OR %s.organization_id = ?", alias)
		args = append(args, *req.OrgID)
	}
	if req.CompanyID != nil && *req.CompanyID != 0 {
		cond += fmt.Sprintf(" OR %s.company_id = ?", alias)
		args = append(args, *req.CompanyID)
	}
	return q.Where(cond, args...)
}

// searchCanSeeAllSystem -- вправе ли пользователь видеть системный срез реестров.
//
// Намеренно обёртка вокруг userCanSeeAllSystem, а не собственный предикат: поиск не
// должен быть шире листинга ни для кого. В системе есть рассинхрон -- реестры гейтят
// системный срез флагами is_super_admin/is_admin напрямую из users, минуя резолвер,
// тогда как грант section.registry.all_system существует в каталоге и раздаётся ролям,
// из-за чего его носитель видит вкладку и получает 403.
//
// Чинить рассинхрон здесь нельзя: переход на резолвер расширил бы доступ носителям
// гранта, то есть поменял матрицу доступа. Хуже того, починка только в поиске родила бы
// второй перекос -- запись нашлась бы в поиске и не открылась в реестре. Когда
// рассинхрон будут устранять, менять придётся ровно эту функцию.
func searchCanSeeAllSystem(ctx context.Context, db *gorm.DB, userID int) bool {
	return userCanSeeAllSystem(ctx, db, userID)
}

// buildSearchVariantsFor -- варианты запроса для сквозного поиска.
//
// Отличается от buildSearchVariants (application_helpers.go) тем, что не добавляет
// заведомо бесполезные варианты: каждый лишний вариант умножается на число колонок
// раздела и превращается в отдельное ILIKE-условие.
//
//   - раскладка добавляется всегда: она ловит реальную ошибку ввода в обе стороны
//     (набрал "Hjujktd" вместо "Роголев" и наоборот для номеров и марок);
//   - normalize.Plate добавляется только для запросов с цифрами. Он приводит строку к
//     верхнему регистру и удаляет пробелы -- для ФИО это даёт либо дубль (ILIKE
//     регистронезависим), либо склейку "ИВАНПЕТРОВ", которая не встречается в данных.
//     Смысл он имеет для госномеров, а те всегда с цифрами;
//   - дедупликация идёт по нижнему регистру, тогда как buildSearchVariants сравнивает
//     точные строки и оставляет пару "Роголев"/"РОГОЛЕВ" целиком.
//
// buildSearchVariants не трогаем: у него другие потребители (Центр заявок, реестры,
// доступные вложения), и смена их семантики в объём сквозного поиска не входит.
func buildSearchVariantsFor(raw string) []string {
	variants := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		variants = append(variants, v)
	}

	add(raw)
	add(normalize.SwitchLayout(raw))
	if strings.ContainsAny(raw, "0123456789") {
		add(normalize.Plate(raw))
	}
	return variants
}
