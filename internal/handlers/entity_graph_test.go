package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

// Сборщик графа данных сущности (server entity show). Тесты живут здесь, а не рядом с
// пакетом entityarchive: тестовая база одна на прогон, а AutoMigrate идёт один раз на
// бинарь. Второй бинарь, работающий с той же базой, мигрирует её одновременно с этим -
// `go test -p 4 ./...` (так гоняет CI) падает на `tuple concurrently updated`.

// TestCollect_RunsEveryNodePredicate прогоняет весь граф против настоящей схемы: Collect
// считает строки КАЖДОГО узла, поэтому битое имя таблицы или столбца в предикате всплыло
// бы здесь ошибкой, а не тихо неполным экспортом. Заодно проверяет, что корневой узел
// считает саму организацию, а чужой идентификатор даёт пустой граф.
func TestCollect_RunsEveryNodePredicate(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "ACME-эксперимент"}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("создать организацию: %v", err)
	}
	// Пользователь и заявка организации: обход обязан собрать не только корень, но и
	// связанные строки - иначе «Collect не упал» ещё не значит «Collect собрал».
	user := models.User{Username: "acme-user", OrganizationID: &org.ID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("создать пользователя: %v", err)
	}
	app := models.Application{OrganizationID: org.ID, SenderUserID: user.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("создать заявку: %v", err)
	}

	graph, err := entityarchive.Collect(context.Background(), db, entityarchive.TypeOrganization, org.ID)
	if err != nil {
		t.Fatalf("Collect (все предикаты должны выполниться): %v", err)
	}

	rows := make(map[string]int64, len(graph.Tables))
	for _, tc := range graph.Tables {
		rows[tc.Table] = tc.Rows
	}
	if rows["organizations"] != 1 {
		t.Fatalf("узел organizations посчитал %d строк, ожидалась 1 (сама организация)", rows["organizations"])
	}
	if rows["applications"] != 1 {
		t.Fatalf("узел applications посчитал %d, ожидалась 1 (заявка организации)", rows["applications"])
	}
	if rows["users"] != 1 {
		t.Fatalf("узел users посчитал %d, ожидался 1 (пользователь организации)", rows["users"])
	}

	// Чужой идентификатор: граф пуст, ни один узел не должен зацепить лишнего.
	empty, err := entityarchive.Collect(context.Background(), db, entityarchive.TypeOrganization, org.ID+999999)
	if err != nil {
		t.Fatalf("Collect по несуществующей организации: %v", err)
	}
	if empty.Total() != 0 {
		t.Fatalf("граф несуществующей организации содержит %d строк, ожидался 0", empty.Total())
	}
}

// TestCollect_RejectsUnknownType фиксирует, что v1 знает только организацию: другой тип -
// явная ошибка, а не пустой (ложно-успешный) результат.
func TestCollect_RejectsUnknownType(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	if _, err := entityarchive.Collect(context.Background(), db, "car", 1); err == nil {
		t.Fatal("Collect с неподдерживаемым типом должен возвращать ошибку, а не пустой граф")
	}
}
