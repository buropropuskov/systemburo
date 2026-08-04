package fakedata

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Step -- один вид наполнения: справочники, пользователи, заявки, проходы. Шаги
// перечислены в Steps в порядке зависимостей и выполняются последовательно.
//
// Последовательно не из осторожности: номер заявки считается как число заявок за сутки
// плюс один, внутри транзакции и без блокировки, а уникального индекса на номере нет.
// Параллельные шаги выдали бы заявкам одинаковые номера, и дубль прошёл бы молча.
type Step interface {
	// Name -- имя шага для вывода и сообщений об ошибках.
	Name() string
	// Plan -- что шаг создаст при таком профиле. Вызывается без записи в базу.
	Plan(p Profile) []PlanItem
	// Run наливает данные и регистрирует созданное в партии.
	Run(ctx context.Context, env *Env) error
}

// PlanItem -- строка предварительного показа: что и сколько создастся.
type PlanItem struct {
	// Entity -- вид сущности из констант models.AuditEntity*.
	Entity string
	// Title -- название вида для человека, в именительном падеже множественного числа.
	Title string
	Count int
}

// Env -- окружение шага: куда писать и что регистрировать.
type Env struct {
	DB      *gorm.DB
	Batch   *Batch
	Profile Profile
	// Seed -- источник случайности партии. Шаги берут из него свои генераторы, чтобы
	// повтор с тем же значением давал ту же партию.
	Seed int64
	// UserPassword -- пароль создаваемых пользователей, печатается в итоговой сводке.
	UserPassword string
}

// Steps перечисляет шаги наполнения в порядке зависимостей: справочники раньше
// пользователей, пользователи раньше заявок, заявки раньше проходов.
//
// Сами шаги добавляются следующими срезами #1682. Здесь пусто намеренно: каркас
// партий, отметки стенда и предварительного показа проверяется отдельно от наполнения.
func Steps() []Step {
	return []Step{}
}

// Plan собирает предварительный показ по всем шагам.
func Plan(p Profile) []PlanItem {
	var items []PlanItem
	for _, step := range Steps() {
		items = append(items, step.Plan(p)...)
	}
	return items
}

// PlanTotal -- сколько записей создастся всего.
func PlanTotal(items []PlanItem) int {
	total := 0
	for _, item := range items {
		total += item.Count
	}
	return total
}

// Run прогоняет шаги наполнения и закрывает партию сводкой.
func Run(ctx context.Context, env *Env) error {
	for _, step := range Steps() {
		if err := step.Run(ctx, env); err != nil {
			// Партия остаётся с уже зарегистрированными записями и не откатывается:
			// иначе созданное до сбоя останется на стенде без перечня, по которому его
			// потом удалять.
			return fmt.Errorf("шаг %q: %w", step.Name(), err)
		}
	}
	if err := env.Batch.Close(ctx); err != nil {
		return err
	}
	return nil
}
