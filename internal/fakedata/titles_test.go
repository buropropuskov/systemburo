package fakedata

// Замок на названия видов записей: отчёт о наливке печатал коды сущностей латиницей
// («application», «unique_employee»), потому что брал ключи прямо из перечня партии.
// Теперь и он, и предварительный показ, и отчёт об удалении зовут EntityTitle -- новый
// вид записей без названия в titles.go снова вернул бы код в вывод.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntityTitle_CoversEveryPurgedEntity(t *testing.T) {
	for _, step := range purgeOrder {
		title := EntityTitle(step.entity)
		require.NotEqual(t, step.entity, title, "вид %q удаляется, но названия для вывода у него нет", step.entity)
	}
}

func TestEntityTitle_CoversEveryPlannedEntity(t *testing.T) {
	for _, name := range ProfileNames() {
		profile, err := ProfileByName(name)
		require.NoError(t, err)
		for _, item := range Plan(profile) {
			require.NotEqual(t, item.Entity, EntityTitle(item.Entity),
				"вид %q попадает в предварительный показ, но названия для вывода у него нет", item.Entity)
			require.Equal(t, EntityTitle(item.Entity), item.Title,
				"предварительный показ и отчёт должны звать один и тот же список названий")
		}
	}
}

// Неизвестный вид отдаётся как есть -- пропажа строки в выводе хуже кода сущности в ней.
func TestEntityTitle_UnknownEntityPassesThrough(t *testing.T) {
	require.Equal(t, "нет_такого_вида", EntityTitle("нет_такого_вида"))
}
