package handlers_test

import (
	"context"
	"fmt"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
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

// cascadeExemptions - таблицы, чьи каскадные FK от узлов графа НЕ считаются пропуском
// карты, с причиной для каждой (та же форма, что testutil.CleanupExempt). Пусто: полный
// аудит ревью среза purge (12.08) закрыл все 14 найденных каскадов добавлением узлов в
// organizationNodes, а не исключением (registry.go, комментарий пакета). Новый каскад мимо
// графа обязан либо получить узел, либо явную причину здесь - молчаливого пропуска
// "разберёмся потом" замок не даёт.
var cascadeExemptions = map[string]string{}

// TestOrganizationGraph_NoUnaccountedCascades - замок полноты карты со стороны реальных FK,
// а не только со стороны моделей (тот угол уже стережёт TestOrganizationRoots_* в
// registry_lock_test.go). Обходит information_schema от КАЖДОЙ таблицы графа и падает, если
// у нашедшегося ON DELETE CASCADE потомка нет ни узла в графе, ни записи в cascadeExemptions.
//
// Живёт здесь (DB-backed), а не в entityarchive/registry_lock_test.go: тому тесту схема не
// нужна (schema.Parse офлайн), этому - нужна настоящая база (#706, второй DB-бинарь на ту
// же тест-БД гоняет AutoMigrate параллельно и даёт гонку).
//
// Мотивация - находка живьём: DELETE FROM users каскадно сносил 12 таблиц (уведомления,
// согласия на обработку ПДн, регистрацию принимающего и т.д.), а application_questions/
// application_supplements - ещё 2 (прочтения вопросов, голоса по раунду дополнения) - никто
// из них не имел узла, и снос уносил их молча, мимо Collect/экспорта/сверки покрытия/
// audit_log. Без этого замка рукописная карта расходится с реальными FK снова и тихо.
func TestOrganizationGraph_NoUnaccountedCascades(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	graph, err := entityarchive.GraphTables(entityarchive.TypeOrganization)
	require.NoError(t, err)
	inGraph := make(map[string]bool, len(graph))
	for _, tbl := range graph {
		inGraph[tbl] = true
	}

	type cascadeRow struct {
		Parent string
		Child  string
		Col    string
	}
	var rows []cascadeRow
	q := `
		SELECT ccu.table_name AS parent, tc.table_name AS child, kcu.column_name AS col
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND rc.delete_rule = 'CASCADE'
			AND ccu.table_name IN ?`
	require.NoError(t, db.Raw(q, graph).Scan(&rows).Error)
	require.NotEmpty(t, rows, "обход information_schema не нашёл ни одного каскада от графа - "+
		"замок сам ничего не проверил бы, это тоже повод остановиться")

	var missing []string
	for _, r := range rows {
		if inGraph[r.Child] {
			continue
		}
		if reason, ok := cascadeExemptions[r.Child]; ok {
			t.Logf("каскад %s.%s -> %s разрешён белым списком: %s", r.Parent, r.Col, r.Child, reason)
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (родитель %s, колонка %s)", r.Child, r.Parent, r.Col))
	}
	require.Empty(t, missing, "ON DELETE CASCADE от узла графа сносит таблицу вне графа и вне "+
		"cascadeExemptions - добавь ей узел в organizationNodes() или явную причину в исключениях: %v", missing)
}
