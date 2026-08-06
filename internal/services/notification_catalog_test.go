package services

import "testing"

func TestNotificationCatalogFieldsFilled(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, meta := range NotificationCatalog() {
		if _, dup := seen[meta.Code]; dup {
			t.Errorf("дубликат кода в каталоге уведомлений: %s", meta.Code)
		}
		seen[meta.Code] = struct{}{}
		if meta.Code == "" || meta.Label == "" || meta.Description == "" || meta.Category == "" {
			t.Errorf("узел каталога уведомлений с пустым полем: %+v", meta)
		}
	}
	if len(seen) == 0 {
		t.Fatal("каталог уведомлений пуст")
	}
}

func TestNotificationCatalogMandatoryOnlySecurity(t *testing.T) {
	t.Parallel()
	for _, meta := range NotificationCatalog() {
		isSecurity := meta.Category == NotificationCategorySecurity
		if meta.Mandatory != isSecurity {
			t.Errorf("%s: Mandatory=%v при категории %s (обязательными должны быть ровно security-типы)",
				meta.Code, meta.Mandatory, meta.Category)
		}
	}
}

func TestNotificationCatalogStableOrder(t *testing.T) {
	t.Parallel()
	first := NotificationCatalog()
	second := NotificationCatalog()
	if len(first) != len(second) {
		t.Fatalf("длина каталога различается между вызовами: %d != %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Code != second[i].Code {
			t.Fatalf("порядок каталога нестабилен на позиции %d: %s != %s", i, first[i].Code, second[i].Code)
		}
	}
	// В пределах категории коды идут по возрастанию, категории - в порядке
	// notificationCategoryOrder.
	rank := make(map[NotificationCategory]int, len(notificationCategoryOrder))
	for i, cat := range notificationCategoryOrder {
		rank[cat] = i
	}
	for i := 1; i < len(first); i++ {
		prev, cur := first[i-1], first[i]
		if rank[prev.Category] > rank[cur.Category] {
			t.Errorf("категория уехала не по порядку: %s (%s) после %s (%s)", cur.Code, cur.Category, prev.Code, prev.Category)
		}
		if prev.Category == cur.Category && prev.Code >= cur.Code {
			t.Errorf("коды внутри категории %s не по возрастанию: %s перед %s", cur.Category, prev.Code, cur.Code)
		}
	}
}

func TestNotificationTypeMeta(t *testing.T) {
	t.Parallel()
	meta, ok := NotificationTypeMeta(NotificationTypePasswordChanged)
	if !ok {
		t.Fatal("password_changed должен быть в каталоге")
	}
	if meta.Category != NotificationCategorySecurity || !meta.Mandatory {
		t.Errorf("password_changed: неожиданные метаданные %+v", meta)
	}
	if _, ok := NotificationTypeMeta("nonexistent_type"); ok {
		t.Error("несуществующий код не должен резолвиться каталогом")
	}
}

// notificationTypesTriggeredInServices -- коды, которые реально передаются в
// CreateForUser по сервисам (internal/services/*.go). Список сверяется руками при
// каждой правке триггера. Добавил новый тип уведомления в сервисах - добавь его
// сюда И в notificationCatalog, иначе этот тест упадёт.
var notificationTypesTriggeredInServices = []string{
	NotificationTypeApplicationCreated,
	NotificationTypeApplicationApprovalRequired,
	NotificationTypeApplicationForwarded,
	NotificationTypeApplicationStatusChanged,
	NotificationTypeApplicationQuestion,
	NotificationTypeApplicationAnswer,
	NotificationTypeApplicationSupplementReady,
	NotificationTypeApplicationSupplementDecided,
	NotificationTypeApprovalReminder,
	NotificationTypePasswordChanged,
	NotificationTypeArchiveQuotaWarning,
	NotificationTypeDirectoryPending,
	NotificationTypeDirectoryResolved,
}

func TestNotificationCatalogCoversTriggeredTypes(t *testing.T) {
	t.Parallel()
	for _, code := range notificationTypesTriggeredInServices {
		if _, ok := NotificationTypeMeta(code); !ok {
			t.Errorf("тип уведомления %q передаётся в CreateForUser, но отсутствует в каталоге", code)
		}
	}
}

func TestNotificationCategoriesOrder(t *testing.T) {
	t.Parallel()
	cats := NotificationCategories()
	want := []NotificationCategory{
		NotificationCategoryApplication,
		NotificationCategorySecurity,
		NotificationCategoryPassage,
		NotificationCategoryContent,
		NotificationCategorySystem,
	}
	if len(cats) != len(want) {
		t.Fatalf("ожидалось %d категорий, получено %d", len(want), len(cats))
	}
	for i, cat := range want {
		if cats[i] != cat {
			t.Errorf("категория на позиции %d: %s, ожидалось %s", i, cats[i], cat)
		}
	}
}
