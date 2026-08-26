package database

import (
	"reflect"
	"strings"
	"testing"
)

// TestAllModels_NoLegacyHistoryTables закрепляет итог эпика #870: история и аудит
// пишутся в общий audit_log через services.AuditRecorder, а НЕ отдельными per-entity
// *History-таблицами. Тест падает, если в AllModels появится новая модель с суффиксом
// "History" - это сигнал вести историю через recorder.Record/Log с своим entity_type
// (models.AuditEntity*), а не плодить очередную *_history таблицу/сервис/endpoint.
//
// Исключения - две намеренно сохранённые модели (НЕ для записи истории через них, лежат
// под будущие фичи): UserBanHistory (полноценный аудит блокировок - issue #936) и
// ApplicationStatusHistory (история статусов заявки). Если добавляешь сюда третье
// исключение - почти наверняка ты делаешь не то: заведи запись в audit_log.
func TestAllModels_NoLegacyHistoryTables(t *testing.T) {
	allowed := map[string]bool{
		"UserBanHistory":           true,
		"ApplicationStatusHistory": true,
	}
	for _, m := range AllModels() {
		name := reflect.TypeOf(m).Elem().Name()
		if strings.HasSuffix(name, "History") && !allowed[name] {
			t.Errorf("модель %q в AllModels: новую историю вести через audit_log "+
				"(services.AuditRecorder.Record/Log + entity_type-константа models.AuditEntity*), "+
				"а не отдельной *History-таблицей (#870)", name)
		}
	}
}
