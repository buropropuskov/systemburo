package middleware

import (
	"strings"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// impersonationDeniedPaths - маршруты, закрытые в режиме «войти как пользователь»
// (#1912). Сверка идёт с шаблоном маршрута (c.Path), а не с адресом запроса:
// шаблон не зависит от подставленных значений и виден грепом рядом с router.go.
//
// Граница проведена по судьбе учётной записи: смотреть чужими глазами можно,
// распоряжаться учётной записью - нет. Иначе режим сам становится способом
// закрепиться: сменил пароль работнику, выдал себе признак администратора,
// удалил следы.
var impersonationDeniedPaths = []string{
	// Пароли. Свой пароль в режиме - это пароль того, от чьего имени работают;
	// смена отняла бы у владельца доступ к собственной учётной записи. Рассылка
	// новых паролей (поштучно и всем сразу, #1910) - то же самое, только оптом.
	"/api/users/me/password",
	"/api/users/:username/password",
	"/api/users/:username/rotate-password",
	"/api/settings/password-rotation/run",
	// Настройки системы целиком: под ними живёт расписание плановой смены паролей,
	// а менять общесистемную настройку из чужой учётной записи незачем в принципе.
	"/api/settings/:key",
	// Судьба учётной записи: удаление (архивирование), восстановление, пакетные
	// операции над теми же действиями.
	"/api/users/:username",
	"/api/users/:username/restore",
	"/api/users/bulk/archive",
	"/api/users/bulk/restore",
	// Тип пользователя - историческое основание админства: смена типа рядом со
	// сменой признаков, а не рядом с правкой ФИО.
	"/api/users/:username/type",
	// Признаки и роли.
	"/api/users/:id/role",
	"/api/users/:id/admin",
	"/api/users/:user_id/permission-groups/:group_id",
	// Блокировки: и выдача, и снятие. Снятие блокировки входа - тоже распоряжение
	// доступом, пусть и в пользу владельца.
	"/api/users/:id/ban",
	"/api/users/:id/unban",
	"/api/users/bulk/ban",
	"/api/users/bulk/unban",
	"/api/users/:username/reset-lockout",
	// Цепочка режимов: из чужой учётной записи нельзя войти в третью. Иначе
	// проверка «нельзя войти от имени более полномочного» обходится в два шага.
	"/api/users/:id/impersonate",
	// Согласие на обработку персональных данных (152-ФЗ). Согласие даёт человек за
	// себя - подтверждение чужими руками не согласие вовсе, а подделка; отзыв
	// чужими руками отнимает у работника доступ. Отзыв за работника у
	// администратора остаётся своим отдельным маршрутом под правом.
	"/api/consents/accept",
	"/api/consents",
	"/api/consents/:type",
	"/api/users/:username/consent",
}

// impersonationDeniedPrefixes - поддеревья, закрытые целиком: правка прав, групп и
// ролей. Перечислять их поимённо значило бы забыть про следующий добавленный
// маршрут, а любой из них меняет модель доступа.
var impersonationDeniedPrefixes = []string{
	"/api/permissions",
	"/api/permission-groups",
	"/api/roles",
}

// DenyUnderImpersonation закрывает опасные действия, пока запрос идёт от чужого
// имени. Чтение (GET/HEAD/OPTIONS) не трогаем: ради него режим и заводится -
// администратор должен увидеть систему глазами работника.
//
// Гейт стоит на группе, а не на отдельных маршрутах: список закрытого лежит в
// одном месте и читается как правило, а не собирается по строчке из router.go.
func DenyUnderImpersonation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, ok := services.ImpersonatorFromContext(c.Request().Context()); !ok {
				return next(c)
			}
			if !deniedUnderImpersonation(c.Request().Method, c.Path()) {
				return next(c)
			}
			return apperr.Forbidden(
				"Действие недоступно, пока вы работаете от имени другого пользователя. " +
					"Вернитесь в свою учётную запись.")
		}
	}
}

func deniedUnderImpersonation(method, routePath string) bool {
	if isSafeMethod(method) {
		return false
	}
	for _, p := range impersonationDeniedPaths {
		if routePath == p {
			return true
		}
	}
	for _, p := range impersonationDeniedPrefixes {
		if routePath == p || strings.HasPrefix(routePath, p+"/") {
			return true
		}
	}
	return false
}
