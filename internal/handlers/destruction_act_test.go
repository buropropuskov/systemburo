package handlers_test

// Акт уничтожения персональных данных за период (#2357).
//
// Акт - документ, которым оператор подтверждает проверяющему, что данные уничтожены.
// Поэтому тесты стерегут не «файл записался», а то, из-за чего акт может оказаться
// ложью: границы периода включительны с обеих сторон, свод сходится с перечнем, даты
// в принятом виде 01.01.0000, и персональных данных в бумаге нет.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// destroyAt обезличивает заявку и сдвигает запись журнала на заданную дату: период акта
// иначе не проверить - все записи ложились бы сегодняшним днём.
func destroyAt(t *testing.T, db *gorm.DB, number string, at time.Time, basis string) {
	t.Helper()
	appID, _ := applicationWithPeople(t, db, number)
	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Basis: basis, Apply: true})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE destruction_log SET created_at = ? WHERE entity_id = ?`,
		at, appID).Error)
}

func TestDestructionAct_PeriodBoundsAreInclusive(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	destroyAt(t, db, "№ 20260228/900", from.AddDate(0, 0, -1), entityarchive.BasisOperator)
	destroyAt(t, db, "№ 20260301/901", from, entityarchive.BasisRetention)
	// Конец дня последней даты периода: без включающей верхней границы уничтожение
	// 31 марта не попало бы в акт «по 31 марта», и акт занизил бы объём.
	destroyAt(t, db, "№ 20260331/902", to.Add(23*time.Hour+59*time.Minute), entityarchive.BasisRetention)
	destroyAt(t, db, "№ 20260401/903", to.AddDate(0, 0, 1), entityarchive.BasisOperator)

	act, err := entityarchive.BuildDestructionAct(context.Background(), db, from, to)
	require.NoError(t, err)
	require.Len(t, act.Records, 2, "в акт входят обе граничные даты и ничего вокруг них")

	numbers := []string{act.Records[0].ApplicationNumber, act.Records[1].ApplicationNumber}
	assert.ElementsMatch(t, []string{"№ 20260301/901", "№ 20260331/902"}, numbers)
}

func TestDestructionAct_SummaryMatchesList(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	destroyAt(t, db, "№ 20260502/910", from.AddDate(0, 0, 1), entityarchive.BasisRetention)
	destroyAt(t, db, "№ 20260503/911", from.AddDate(0, 0, 2), entityarchive.BasisRetention)
	destroyAt(t, db, "№ 20260504/912", from.AddDate(0, 0, 3), entityarchive.BasisSubjectRequest)

	act, err := entityarchive.BuildDestructionAct(context.Background(), db, from, to)
	require.NoError(t, err)
	require.Len(t, act.Records, 3)
	require.Len(t, act.Sections, 2, "перечень и свод по основаниям")

	list, summary := act.Sections[0], act.Sections[1]
	require.Len(t, list.Rows, 3)
	assert.Equal(t, "Заявка № 20260502/910", list.Rows[0][1])
	assert.Regexp(t, `^\d{2}\.\d{2}\.\d{4}$`, list.Rows[0][0], "даты в виде 01.01.0000")
	assert.Equal(t, "истёк срок хранения", list.Rows[0][3])

	// Итог свода обязан сойтись с перечнем: разойдись они, акт подтверждал бы объём,
	// которого не было.
	last := summary.Rows[len(summary.Rows)-1]
	assert.Equal(t, "Итого", last[0])
	assert.Equal(t, "3", last[1])
	sumRows := 0
	for _, row := range list.Rows {
		n, err := strconv.Atoi(row[5])
		require.NoError(t, err)
		sumRows += n
	}
	assert.Equal(t, strconv.Itoa(sumRows), last[2], "строки свода и перечня обязаны совпасть")

	// Основания разнесены по своим строкам, а не свалены в одну.
	bases := []string{summary.Rows[0][0], summary.Rows[1][0]}
	assert.ElementsMatch(t,
		[]string{"истёк срок хранения", "требование субъекта персональных данных"}, bases)
}

func TestDestructionAct_WritesFilesWithoutPersonalData(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	destroyAt(t, db, "№ 20260705/920", from.AddDate(0, 0, 4), entityarchive.BasisRetention)

	passport, _ := anonymizeSubjectFixture(t, db)
	_, err := entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.SubjectTargetFromDocuments(passport, ""),
		entityarchive.DestructionOptions{Basis: entityarchive.BasisSubjectRequest, Apply: true})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE destruction_log SET created_at = ? WHERE entity_type = ?`,
		from.AddDate(0, 0, 5), "unique_employee").Error)

	act, err := entityarchive.BuildDestructionAct(context.Background(), db, from, to)
	require.NoError(t, err)
	require.Len(t, act.Records, 2)

	root := t.TempDir()
	written, err := entityarchive.WriteDestructionAct(root, act)
	require.NoError(t, err)
	require.Len(t, written, 2, "xlsx для работы и pdf для официального ответа")
	assert.Equal(t, filepath.Join(root, "destruction-act-20260701-20260731.xlsx"), written[0])
	assert.Equal(t, filepath.Join(root, "destruction-act-20260701-20260731.pdf"), written[1])
	for _, f := range written {
		info, err := os.Stat(f)
		require.NoError(t, err)
		assert.Positive(t, info.Size())
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"акт лежит рядом с выгрузками и читается только владельцем процесса")
	}

	// Человек в акте назван безлично: документ подтверждает уничтожение сведений о
	// нём и сам этих сведений нести не должен - ни имени, ни документа, ни отпечатка.
	var subjectRow []string
	for _, row := range act.Sections[0].Rows {
		if strings.HasPrefix(row[1], "Сведения") {
			subjectRow = row
		}
	}
	require.NotNil(t, subjectRow, "запись о человеке обязана попасть в акт")
	joined := strings.Join(subjectRow, " | ")
	for _, secret := range []string{"Стираев", "Артём", passport, "505050",
		entityarchive.DisclosureSubjectKey(entityarchive.SubjectTargetFromDocuments(passport, ""))} {
		assert.NotContains(t, joined, secret, "персональные данные в акте: %q", secret)
	}
}

func TestDestructionAct_RefusesInvertedPeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, err := entityarchive.BuildDestructionAct(context.Background(), db,
		time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	require.Error(t, err, "перевёрнутый период - опечатка оператора, а не пустой акт")
}

func TestDestructionAct_ParsesDatesInSystemFormat(t *testing.T) {
	v, err := entityarchive.ParseActDate("01.03.2026")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), v)

	_, err = entityarchive.ParseActDate("2026-03-01")
	require.Error(t, err, "вид дат в системе - 01.01.0000, и акт не исключение")
}
