package services

import "context"

// impersonatorCtxKey - ключ инициатора сеанса «войти как пользователь» (#1912) в
// context.Context запроса. Приватный тип: чужой пакет не подменит значение
// случайным совпадением строкового ключа.
type impersonatorCtxKey struct{}

// WithImpersonator кладёт в контекст идентификатор администратора, открывшего
// сеанс от чужого имени. Вызывается один раз, разбором маркера доступа.
func WithImpersonator(ctx context.Context, actorUserID int) context.Context {
	return context.WithValue(ctx, impersonatorCtxKey{}, actorUserID)
}

// ImpersonatorFromContext возвращает инициатора сеанса и признак того, что запрос
// вообще идёт в режиме «войти как пользователь». Обычный вход даёт false.
func ImpersonatorFromContext(ctx context.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value(impersonatorCtxKey{}).(int)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}
