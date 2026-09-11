package handlers_test

// Сборщик данных одного человека (server subject show). Тесты живут здесь, а не рядом с
// пакетом entityarchive, по той же причине, что и у графа организации: тестовая база
// одна на прогон, а второй бинарь мигрировал бы её одновременно.

import (
	"context"
	"fmt"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// subjectFixture заводит человека в реестре и участником заявки с тем же паспортом,
// плюс однофамильца с другим паспортом. Возвращает паспорт цели.
func subjectFixture(t *testing.T, db *gorm.DB) (passport string, attachmentID int) {
	t.Helper()

	org := models.Organization{Name: "Субъект-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "subject-owner", OrganizationID: &org.ID}
	require.NoError(t, db.Create(&user).Error)
	app := models.Application{OrganizationID: org.ID, SenderUserID: user.ID}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&att).Error)

	passport = "4510 987654"
	last, first := "Субъектов", "Пётр"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first,
		PassportSeriesNumber: &passport,
		OrganizationID:       &org.ID,
	}).Error)
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &passport,
	}).Error)

	// Однофамилец с другим документом: склейка по имени приписала бы его проходы цели.
	other := "4510 111222"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &other,
	}).Error)

	return passport, att.ID
}

// TestCollectSubject_GathersRowsByDocument - сбор идёт по свёртке документа и собирает
// человека во всех источниках сразу: и запись реестра, и строку заявки.
func TestCollectSubject_GathersRowsByDocument(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	passport, _ := subjectFixture(t, db)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	graph, err := entityarchive.CollectSubject(context.Background(), db, target)
	require.NoError(t, err)

	rows := make(map[string]int64, len(graph.Tables))
	for _, tc := range graph.Tables {
		rows[tc.Table] = tc.Rows
	}
	assert.Equal(t, int64(1), rows["unique_employees"], "запись реестра обязана попасть в сбор")
	assert.Equal(t, int64(1), rows["employees"], "участие в заявке тоже: однофамилец с другим паспортом не в счёт")
}

// TestCollectSubject_IgnoresNamesakes - однофамилец с другим документом в сбор не
// попадает. Это главный инвариант: приписать человеку чужие проходы в ответе
// государственному органу - ошибка, которую потом никто не заметит.
func TestCollectSubject_IgnoresNamesakes(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	passport, _ := subjectFixture(t, db)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	graph, err := entityarchive.CollectSubject(context.Background(), db, target)
	require.NoError(t, err)

	for _, tc := range graph.Tables {
		if tc.Table == "employees" {
			assert.Equal(t, int64(1), tc.Rows, "в сбор попал однофамилец: склейка идёт по документу, а не по имени")
		}
	}

	// По имени однофамилец обязан быть виден - решает человек, а не система. Строки
	// при этом склеены по документу: у цели запись реестра и строка заявки с одним
	// паспортом дают ОДИН пункт, у однофамильца с другим паспортом - свой.
	candidates, err := entityarchive.FindSubjectCandidatesByFIO(context.Background(), db, "Субъектов", "Пётр", "")
	require.NoError(t, err)
	require.Len(t, candidates, 2, "два человека с одним именем и разными паспортами")

	var subject, namesake entityarchive.SubjectCandidate
	for _, c := range candidates {
		if c.RegistryRows > 0 {
			subject = c
			continue
		}
		namesake = c
	}
	assert.Equal(t, 1, subject.RegistryRows, "запись реестра цели")
	assert.Equal(t, 1, subject.ApplicationRows, "и её же строка в заявке - тот же паспорт")
	assert.Equal(t, 1, namesake.ApplicationRows, "однофамилец живёт только в заявке")
	assert.Zero(t, namesake.RegistryID, "записи реестра у него нет: собирать придётся от строки заявки")
	assert.NotZero(t, namesake.EmployeeID)
}

// TestSubjectTarget_EmptyDocumentMatchesNothing - цель без документов ничего не
// собирает. Без явной проверки пустая свёртка совпала бы со всеми строками, где
// документа нет вовсе, и ответ органу вобрал бы чужих людей.
func TestSubjectTarget_EmptyDocumentMatchesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	subjectFixture(t, db)

	_, err := entityarchive.CollectSubject(context.Background(), db, entityarchive.SubjectTarget{})
	require.Error(t, err, "цель без документа обязана отказывать, а не собирать всё подряд")

	// Патент цели не задан, паспорт задан: строки с пустым патентом не должны
	// притянуться по второму условию.
	target := entityarchive.SubjectTargetFromDocuments("4510 987654", "")
	graph, err := entityarchive.CollectSubject(context.Background(), db, target)
	require.NoError(t, err)
	for _, tc := range graph.Tables {
		assert.LessOrEqual(t, tc.Rows, int64(2), "узел %s собрал лишнее: пустая свёртка патента совпала с пустыми значениями", tc.Table)
	}
}

// TestSubjectTargetFromRegistry_ReadsHashesWithoutDecrypting - цель по записи реестра
// берёт готовые свёртки. Так оператор ищет человека, найденного в интерфейсе, не вводя
// номер паспорта руками и не расшифровывая его.
func TestSubjectTargetFromRegistry_ReadsHashesWithoutDecrypting(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	passport, _ := subjectFixture(t, db)

	var id int
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("last_name = ?", "Субъектов").
		Select("id").Row().Scan(&id))

	target, err := entityarchive.SubjectTargetFromRegistry(context.Background(), db, id)
	require.NoError(t, err)
	require.False(t, target.Empty())
	assert.Equal(t, crypto.ComputeHMAC(passport, crypto.GetGlobalKey()), target.PassportHMAC)
}

// TestSubjectGraph_NoUnaccountedCascades - замок полноты карты со стороны реальных
// внешних ключей. Обходит information_schema от каждого узла графа субъекта и падает,
// если у найденного ON DELETE CASCADE потомка нет узла. Тот же замок, что у графа
// организации, и по той же причине: рукописная карта расходится со схемой тихо.
func TestSubjectGraph_NoUnaccountedCascades(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	graph, err := entityarchive.GraphTables(entityarchive.TypeSubject)
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
	require.Empty(t, missing, "ON DELETE CASCADE от узла графа субъекта ведёт в таблицу вне графа - "+
		"добавь ей узел в subjectNodes(): %v", missing)
}

// TestFindSubjectCandidates_MiddleNameOptional - отчество необязательно: в запросе
// государственного органа его часто нет, а человек в системе заведён с ним. Поиск,
// требующий полного совпадения тройки, не нашёл бы никого - поймано живым прогоном
// команды на базе, тест держит поведение.
func TestFindSubjectCandidates_MiddleNameOptional(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Отчество-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	last, first, middle := "Отчествов", "Пётр", "Петрович"
	passport := "4510 555666"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, MiddleName: &middle,
		PassportSeriesNumber: &passport, OrganizationID: &org.ID,
	}).Error)

	ctx := context.Background()

	found, err := entityarchive.FindSubjectCandidatesByFIO(ctx, db, "Отчествов", "Пётр", "")
	require.NoError(t, err)
	assert.Len(t, found, 1, "без отчества человек обязан находиться")

	// «Пётр» и «Петр» - один человек: нормализация снимает «ё», как в normalize.Name.
	found, err = entityarchive.FindSubjectCandidatesByFIO(ctx, db, "отчествов", "петр", "петрович")
	require.NoError(t, err)
	assert.Len(t, found, 1, "регистр и «ё» не должны мешать")

	found, err = entityarchive.FindSubjectCandidatesByFIO(ctx, db, "Отчествов", "Пётр", "Сергеевич")
	require.NoError(t, err)
	assert.Empty(t, found, "чужое отчество - другой человек")
}

// TestFindSubjectCandidates_TypoInSurname - опечатка в фамилии не должна означать
// «человека в системе нет». Запрос государственного органа приходит с чужих слов, и
// «Мякотних» вместо «Мякотных» там обычное дело. Найденное по похожему написанию
// помечается: домысливать за оператора молча нельзя, иначе в ответ уедет посторонний.
func TestFindSubjectCandidates_TypoInSurname(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Опечатка-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	last, first := "Мякотных", "Сергей"
	passport := "4510 121212"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, PassportSeriesNumber: &passport,
		OrganizationID: &org.ID,
	}).Error)

	ctx := context.Background()

	exact, err := entityarchive.FindSubjectCandidatesByFIO(ctx, db, "Мякотных", "Сергей", "")
	require.NoError(t, err)
	require.Len(t, exact, 1)
	assert.False(t, exact[0].Fuzzy, "точное совпадение не помечается похожим")

	typo, err := entityarchive.FindSubjectCandidatesByFIO(ctx, db, "Мякотних", "Сергей", "")
	require.NoError(t, err)
	require.Len(t, typo, 1, "опечатка в одной букве не должна прятать человека")
	assert.True(t, typo[0].Fuzzy, "найденное по похожему написанию обязано быть помечено")

	far, err := entityarchive.FindSubjectCandidatesByFIO(ctx, db, "Кузнецов", "Сергей", "")
	require.NoError(t, err)
	assert.Empty(t, far, "другая фамилия - не опечатка")
}

// TestFindSubjectCandidatesByDocument - поиск по номеру документа. В запросе
// государственного органа он есть чаще, чем верное написание фамилии, и однозначен:
// находит того, о ком спрашивают, без списка однофамильцев.
func TestFindSubjectCandidatesByDocument(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	passport, _ := subjectFixture(t, db)

	found, err := entityarchive.FindSubjectCandidatesByDocument(context.Background(), db, passport)
	require.NoError(t, err)
	require.Len(t, found, 1, "по документу находится ровно один человек, однофамилец не в счёт")
	assert.Contains(t, found[0].FullName, "Субъектов")
	assert.Equal(t, 1, found[0].RegistryRows)
	assert.Equal(t, 1, found[0].ApplicationRows)

	none, err := entityarchive.FindSubjectCandidatesByDocument(context.Background(), db, "0000 000000")
	require.NoError(t, err)
	assert.Empty(t, none, "чужой документ не должен находить никого")

	_, err = entityarchive.FindSubjectCandidatesByDocument(context.Background(), db, "  ")
	require.Error(t, err, "пустой документ - это не поиск по всем подряд")
}
