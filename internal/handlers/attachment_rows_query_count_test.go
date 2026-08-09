package handlers_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Сборка строк вложения не должна ходить в базу по разу на строку (#1050). Раньше на
// каждую машину уходило два подзапроса (места разгрузки и таблицы «Проезда»), на каждого
// сотрудника - один: заявка на двадцать машин давала сорок обращений вместо двух.
//
// Тест считает SQL, а не время: замер времени на тестовой базе шумный и в CI врёт.

// sqlCounter - gorm-логгер, считающий запросы по подстроке в тексте SQL.
type sqlCounter struct {
	gormlogger.Interface
	mu      sync.Mutex
	counts  map[string]int
	markers []string
}

func newSQLCounter(markers ...string) *sqlCounter {
	return &sqlCounter{
		Interface: gormlogger.Discard,
		counts:    make(map[string]int),
		markers:   markers,
	}
}

func (c *sqlCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.markers {
		if strings.Contains(sql, m) {
			c.counts[m]++
		}
	}
}

func (c *sqlCounter) get(marker string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[marker]
}

// seedAttachmentWithRows создаёт вложение с n машинами и n сотрудниками, у каждой строки
// своё место разгрузки и своя таблица - чтобы N+1 был виден по числу запросов.
func seedAttachmentWithRows(t *testing.T, db *gorm.DB, rows int) (attachmentID int) {
	t.Helper()
	var appID int
	require.NoError(t, db.Raw(`
		INSERT INTO applications (application_number, confirmation, status, sending_datetime, organization_id)
		VALUES (?, 'Согласование', 'В обработке', NOW(), (SELECT id FROM organizations ORDER BY id LIMIT 1))
		RETURNING id`, fmt.Sprintf("N1-%d", time.Now().UnixNano()%1000000)).Scan(&appID).Error)

	require.NoError(t, db.Raw(`
		INSERT INTO attachments (application_id, attachment_type, attachment_name, entry_date_from, entry_date_to)
		VALUES (?, 'cars', 'n1_tmpl', '2026-01-01', '2099-12-31') RETURNING id`, appID).Scan(&attachmentID).Error)

	for i := 0; i < rows; i++ {
		var carID, empID, placeID int
		suffix := fmt.Sprintf("%d_%d", time.Now().UnixNano()%1000000, i)
		require.NoError(t, db.Raw(`INSERT INTO cars (attachment_id, car_number, car_brand, status)
			VALUES (?, ?, 'Марка', 1) RETURNING id`, attachmentID, "N"+suffix).Scan(&carID).Error)
		require.NoError(t, db.Raw(`INSERT INTO employees (attachment_id, last_name, first_name, status)
			VALUES (?, ?, 'Имя', 1) RETURNING id`, attachmentID, "Фам"+suffix).Scan(&empID).Error)
		require.NoError(t, db.Raw(`INSERT INTO unload_places (name) VALUES (?) RETURNING id`,
			"Место "+suffix).Scan(&placeID).Error)
		require.NoError(t, db.Exec(`INSERT INTO car_unload_places (car_id, unload_place_id, order_index)
			VALUES (?, ?, ?)`, carID, placeID, i).Error)

		dn := "Проезд " + suffix
		tbl := models.SystemTable{Name: "n1_tbl_" + suffix, DisplayName: &dn, TableType: "cars", IsActive: true}
		require.NoError(t, db.Create(&tbl).Error)
		require.NoError(t, db.Exec(`INSERT INTO car_target_tables (car_id, table_id, order_index)
			VALUES (?, ?, ?)`, carID, tbl.ID, i).Error)
		require.NoError(t, db.Exec(`INSERT INTO employee_target_tables (employee_id, table_id, order_index)
			VALUES (?, ?, ?)`, empID, tbl.ID, i).Error)
	}
	return attachmentID
}

func TestAttachmentRows_NoQueryPerRow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	const rows = 5
	attachmentID := seedAttachmentWithRows(t, db, rows)

	counter := newSQLCounter("car_unload_places", "car_target_tables", "employee_target_tables")
	counted := db.Session(&gorm.Session{Logger: counter})
	svc := newWorkflowService(counted)

	cars, err := svc.GetAttachmentCars(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	require.Len(t, cars, rows)

	emps, err := svc.GetAttachmentEmployees(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	require.Len(t, emps, rows)

	// По одному запросу на связь, независимо от числа строк. До правки было по одному
	// на КАЖДУЮ строку, то есть 5, 5 и 5 при rows=5.
	assert.Equal(t, 1, counter.get("car_unload_places"),
		"места разгрузки берутся одним запросом на всю выборку")
	assert.Equal(t, 1, counter.get("car_target_tables"),
		"таблицы «Проезда» берутся одним запросом на всю выборку")
	assert.Equal(t, 1, counter.get("employee_target_tables"),
		"места прохода берутся одним запросом на всю выборку")
}

// Данные после перехода на пакетную выборку остались те же: у каждой машины своё место
// и своя таблица, ничего не перемешалось между строками.
func TestAttachmentRows_BatchKeepsRowsIntact(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	const rows = 3
	attachmentID := seedAttachmentWithRows(t, db, rows)
	svc := newWorkflowService(db)

	cars, err := svc.GetAttachmentCars(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	require.Len(t, cars, rows)

	seenPlaces := map[string]bool{}
	for _, c := range cars {
		require.Len(t, c.UnloadPlaces, 1, "у каждой машины ровно своё место разгрузки")
		require.Len(t, c.TargetTables, 1, "у каждой машины ровно своя таблица «Проезда»")
		assert.False(t, seenPlaces[c.UnloadPlaces[0].Name], "места не должны повторяться между машинами")
		seenPlaces[c.UnloadPlaces[0].Name] = true
	}

	emps, err := svc.GetAttachmentEmployees(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	for _, e := range emps {
		require.Len(t, e.TargetTables, 1, "у каждого сотрудника ровно своё место прохода")
	}
}

// Пустая выборка не должна ходить в базу за связями вовсе и обязана отдавать пустые
// срезы, а не nil: фронт различает «привязок нет» и «поле не пришло».
func TestAttachmentRows_EmptyAttachmentSkipsLinkQueries(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	attachmentID := seedAttachmentWithRows(t, db, 0)

	counter := newSQLCounter("car_unload_places", "car_target_tables", "employee_target_tables")
	svc := newWorkflowService(db.Session(&gorm.Session{Logger: counter}))

	cars, err := svc.GetAttachmentCars(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	assert.Empty(t, cars)
	emps, err := svc.GetAttachmentEmployees(context.Background(), attachmentID, services.SupplementScopeAll)
	require.NoError(t, err)
	assert.Empty(t, emps)

	assert.Equal(t, 0, counter.get("car_unload_places"))
	assert.Equal(t, 0, counter.get("car_target_tables"))
	assert.Equal(t, 0, counter.get("employee_target_tables"))
}
