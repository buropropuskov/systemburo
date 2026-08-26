package handlers_test

import (
	"regexp"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Схема дополнения заявки (#1685, срез S0). Дополнение - партия сущностей, добавленных во
// вложения уже поданной заявки. Проверяем инварианты, которые держит БД, а не сервис:
// один незакрытый раунд на заявку, уникальность номера раунда, наличие колонок провенанса.

// seedSupplementApp создаёт заявку-носитель для раундов дополнения.
func seedSupplementApp(t *testing.T, db *gorm.DB, orgID int, number string) int {
	t.Helper()
	u := models.User{Username: "supplement_sender_" + number, Password: "x", TypeID: 1, OrganizationID: &orgID}
	require.NoError(t, db.Create(&u).Error)

	num := number
	confirmation := models.ConfirmationApproved
	status := models.StatusInWork
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &confirmation,
		Status:            &status,
		OrganizationID:    orgID,
		SenderUserID:      u.ID,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

func newSupplement(appID, number int, status string) models.ApplicationSupplement {
	return models.ApplicationSupplement{
		ApplicationID:   appID,
		Number:          number,
		Status:          status,
		CreatedByUserID: 1,
	}
}

// TestSupplementSchema_IndexesCreated: AutoMigrate ставит индексы дополнения. Частичный
// уникальный индекс gorm-тегом не выражается (WHERE он не умеет) и живёт отдельной
// функцией - без него ограничение "один открытый раунд" держалось бы только проверкой
// в сервисе, то есть проигрывало бы гонке двух одновременных подач.
func TestSupplementSchema_IndexesCreated(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	for _, idx := range []string{"uidx_app_supplement_open", "idx_app_supplement_number", "idx_app_supplement_approval"} {
		var count int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = ?`, idx).Scan(&count).Error)
		assert.Equal(t, int64(1), count, "индекс %s должен существовать", idx)
	}

	// Список статусов в частичном индексе - литерал (предикат индекса не умеет иначе) и
	// дублирует models.OpenSupplementStatuses. Сверяем оба СОСТАВА, а не вхождение
	// отдельных строк: CREATE INDEX IF NOT EXISTS не переделает уже созданный индекс,
	// поэтому расширение Go-списка без отдельной миграции обязано валить тест здесь, а не
	// проявляться тем, что новый открытый статус молча перестанет быть уникальным.
	var def string
	require.NoError(t, db.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = 'uidx_app_supplement_open'`).Scan(&def).Error)

	literal := regexp.MustCompile(`'([a-z_]+)'`)
	inIndex := make([]string, 0, len(models.OpenSupplementStatuses))
	for _, m := range literal.FindAllStringSubmatch(def, -1) {
		inIndex = append(inIndex, m[1])
	}
	assert.ElementsMatch(t, models.OpenSupplementStatuses, inIndex,
		"состав открытых статусов в индексе и в models.OpenSupplementStatuses должен совпадать, indexdef: %s", def)
}

// TestSupplementSchema_SingleOpenPerApplication: пока раунд ждёт голосов или принятия,
// второй на той же заявке подать нельзя. Иначе состав вложения менялся бы скачками, а
// согласующий получал бы по одной заявке два неразличимых запроса на согласование.
func TestSupplementSchema_SingleOpenPerApplication(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	appID := seedSupplementApp(t, db, td.OrgID, "SUPP-OPEN-1")

	first := newSupplement(appID, 1, models.SupplementPending)
	require.NoError(t, db.Create(&first).Error, "первый раунд должен создаваться")

	second := newSupplement(appID, 2, models.SupplementApproved)
	assert.Error(t, db.Create(&second).Error, "второй незакрытый раунд на той же заявке недопустим")

	// Другая заявка своим раундом не ограничена: индекс частичный по application_id.
	otherAppID := seedSupplementApp(t, db, td.OrgID, "SUPP-OPEN-2")
	otherFirst := newSupplement(otherAppID, 1, models.SupplementPending)
	assert.NoError(t, db.Create(&otherFirst).Error, "раунд на другой заявке не должен блокироваться")
}

// TestSupplementSchema_TerminalDoesNotBlockNext: закрытый раунд следующий не держит -
// иначе после первой же добавки заявку нельзя было бы дополнить никогда.
func TestSupplementSchema_TerminalDoesNotBlockNext(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	appID := seedSupplementApp(t, db, td.OrgID, "SUPP-TERM-1")

	terminal := []string{
		models.SupplementMerged,
		models.SupplementAccepted,
		models.SupplementRejected,
		models.SupplementRefused,
		models.SupplementCancelled,
	}
	for i, status := range terminal {
		s := newSupplement(appID, i+1, status)
		require.NoError(t, db.Create(&s).Error, "закрытый раунд %s должен создаваться", status)
	}

	next := newSupplement(appID, len(terminal)+1, models.SupplementPending)
	assert.NoError(t, db.Create(&next).Error, "после закрытых раундов новый открытый должен проходить")
}

// TestSupplementSchema_NumberUniquePerApplication: номер раунда считается как max+1,
// поэтому две одновременные подачи без уникального индекса получили бы один номер.
func TestSupplementSchema_NumberUniquePerApplication(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	appID := seedSupplementApp(t, db, td.OrgID, "SUPP-NUM-1")

	first := newSupplement(appID, 1, models.SupplementAccepted)
	require.NoError(t, db.Create(&first).Error)

	duplicate := newSupplement(appID, 1, models.SupplementMerged)
	assert.Error(t, db.Create(&duplicate).Error, "номер раунда в рамках заявки должен быть уникален")
}

// TestSupplementSchema_ApprovalUniquePerUser: один голос на согласующего в раунде.
func TestSupplementSchema_ApprovalUniquePerUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	appID := seedSupplementApp(t, db, td.OrgID, "SUPP-VOTE-1")
	supplement := newSupplement(appID, 1, models.SupplementPending)
	require.NoError(t, db.Create(&supplement).Error)

	voter := models.User{Username: "supplement_voter", Password: "x", TypeID: 1, OrganizationID: &td.OrgID}
	require.NoError(t, db.Create(&voter).Error)

	first := models.ApplicationSupplementApproval{SupplementID: supplement.ID, UserID: voter.ID}
	require.NoError(t, db.Create(&first).Error)

	duplicate := models.ApplicationSupplementApproval{SupplementID: supplement.ID, UserID: voter.ID}
	assert.Error(t, db.Create(&duplicate).Error, "второй голос того же согласующего в раунде недопустим")
}

// TestSupplementSchema_ProvenanceColumns: колонки провенанса добавлены и nullable.
// NULL означает "пришло с исходной подачей" - именно поэтому бэкфилл не нужен, а
// существующие строки после выката остаются исходным составом заявки.
func TestSupplementSchema_ProvenanceColumns(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tables := []string{"cars", "employees", "items", "application_blacklist_flags"}
	for _, table := range tables {
		var nullable string
		err := db.Raw(`
			SELECT is_nullable FROM information_schema.columns
			WHERE table_name = ? AND column_name = 'supplement_id'
		`, table).Scan(&nullable).Error
		require.NoError(t, err)
		assert.Equal(t, "YES", nullable, "%s.supplement_id должна существовать и быть nullable", table)
	}
}
