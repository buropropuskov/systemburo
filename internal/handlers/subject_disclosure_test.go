package handlers_test

// Журнал выдач сведений третьим лицам (#2356).
//
// Передача сведений по запросу органа - это раскрытие персональных данных. При
// проверке оператор обязан показать, что раскрытие было законным, и журнал - его
// единственное доказательство. Поэтому проверяется не «запись создалась», а то, что
// выдачу нельзя провести мимо журнала.

import (
	"bytes"
	"context"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func disclosureFixture(t *testing.T, db *gorm.DB) (entityarchive.SubjectTarget, entityarchive.SubjectReport) {
	t.Helper()

	org := models.Organization{Name: "Журнал-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	last, first := "Выдачин", "Роман"
	passport := "4510 161803"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, PassportSeriesNumber: &passport,
		OrganizationID: &org.ID,
	}).Error)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	rep, err := entityarchive.BuildSubjectReport(context.Background(), db, target)
	require.NoError(t, err)
	return target, rep
}

// TestDisclosure_RequiresRecipientAndRequest - выдача без получателя или реквизитов
// запроса не регистрируется. Запись без них бессмысленна: показать проверяющему
// «кому-то когда-то выдали» - то же самое, что не показать ничего.
func TestDisclosure_RequiresRecipientAndRequest(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target, rep := disclosureFixture(t, db)

	ctx := context.Background()
	cases := map[string]entityarchive.DisclosureRequest{
		"без получателя": {RequestRef: "исх. 1", IssuedBy: "консоль сервера"},
		"без запроса":    {Recipient: "УМВД", IssuedBy: "консоль сервера"},
		"без выдавшего":  {Recipient: "УМВД", RequestRef: "исх. 1"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := entityarchive.RecordDisclosure(ctx, db, target, rep, nil, req)
			require.Error(t, err)
		})
	}
}

// TestDisclosure_RecordsScopeAndSubject - в записи остаётся то, что показывают
// проверяющему: кому, по какому запросу, о ком и в каком объёме.
func TestDisclosure_RecordsScopeAndSubject(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target, rep := disclosureFixture(t, db)
	// Проверка смотрит на свёртку документа в записи, а она пишется только при
	// заданном ключе шифрования (#2463) - тестовое окружение работает без него.
	withEncryptionKey(t)

	entry, err := entityarchive.RecordDisclosure(context.Background(), db, target, rep,
		[]string{"/var/entity-export/subject-20260908-120000.xlsx"},
		entityarchive.DisclosureRequest{
			Recipient:  "УМВД по г. Москве",
			RequestRef: "исх. 12/345 от 08.09.2026",
			Basis:      "запрос о нахождении на объекте",
			IssuedBy:   "консоль сервера",
		})
	require.NoError(t, err)

	assert.Equal(t, "УМВД по г. Москве", entry.Recipient)
	assert.Equal(t, "исх. 12/345 от 08.09.2026", entry.RequestRef)
	assert.Contains(t, entry.SubjectName, "Выдачин", "по свёртке документа проверяющий человека не опознает")
	assert.Contains(t, entry.Scope, "Сведения:", "объём выданного - часть доказательства")
	assert.Equal(t, "subject-20260908-120000.xlsx", entry.Files,
		"путь целиком не пишем: он привязан к машине, а журнал переживает переезд")
	assert.NotContains(t, entry.SubjectHMAC, "4510",
		"сам документ в журнал попасть не должен: без ключа шифрования ComputeHMAC "+
			"работает passthrough, и в журнал уходил открытый паспорт")
	assert.Len(t, entry.SubjectHMAC, 64, "ключ человека - sha256 в hex")
}

// TestDisclosure_ListsBySubject - журнал читается и целиком, и по одному человеку:
// на запрос «что вы выдавали про меня» отвечает вторая выборка.
func TestDisclosure_ListsBySubject(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target, rep := disclosureFixture(t, db)
	ctx := context.Background()

	req := entityarchive.DisclosureRequest{
		Recipient: "УМВД", RequestRef: "исх. 1", IssuedBy: "консоль сервера",
	}
	_, err := entityarchive.RecordDisclosure(ctx, db, target, rep, nil, req)
	require.NoError(t, err)

	all, err := entityarchive.ListDisclosures(ctx, db, "", 50)
	require.NoError(t, err)
	require.Len(t, all, 1)

	// Ключ шифрования задаётся на время проверки: без него ключ субъекта пустой
	// (#2463), выборка «по одному человеку» выродилась бы в выборку всего журнала,
	// и тест сравнивал бы сам с собой.
	withEncryptionKey(t)
	keyed, err := entityarchive.RecordDisclosure(ctx, db, target, rep, nil, req)
	require.NoError(t, err)
	require.NotEmpty(t, keyed.SubjectHMAC, "при заданном ключе выдача опознаётся")

	mine, err := entityarchive.ListDisclosures(ctx, db, entityarchive.DisclosureSubjectKey(target), 50)
	require.NoError(t, err)
	require.Len(t, mine, 1)

	other, err := entityarchive.ListDisclosures(ctx, db, "свёртка-другого-человека", 50)
	require.NoError(t, err)
	assert.Empty(t, other, "журнал по одному человеку не должен показывать чужие выдачи")
}

// withEncryptionKey включает шифрование на время одной проверки и возвращает всё как
// было. Тестовое окружение работает без ключа (testutil ставит nil), а поведение
// журнала выдач от него зависит - проверять надо оба режима.
func withEncryptionKey(t *testing.T) {
	t.Helper()
	crypto.SetGlobalKey(bytes.Repeat([]byte{7}, 32))
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })
}

// Без ключа шифрования свёртка документа в журнал не пишется (#2463). Причина не в
// аккуратности: при выключенном шифровании в цель приходит сам номер документа, а
// десять цифр перебираются за минуты - запись, которой доказывают законность
// раскрытия, стала бы последним местом, где человек опознаётся. Факт выдачи при этом
// фиксируется по-прежнему: он важнее склейки.
func TestDisclosure_NoSubjectKeyWithoutEncryption(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target, rep := disclosureFixture(t, db)

	entry, err := entityarchive.RecordDisclosure(context.Background(), db, target, rep, nil,
		entityarchive.DisclosureRequest{Recipient: "УМВД", RequestRef: "исх. 2", IssuedBy: "консоль сервера"})
	require.NoError(t, err)

	assert.Empty(t, entry.SubjectHMAC, "без ключа опознавать человека в журнале нечем")
	assert.NotEmpty(t, entry.Recipient, "сама выдача записана")
	assert.Empty(t, entityarchive.DisclosureSubjectKey(target))
}

// С ключом свёртка пишется и не содержит самого документа: она нужна, чтобы собрать
// все выдачи по человеку спустя годы, когда его записей в системе уже нет.
func TestDisclosure_SubjectKeyHidesDocument(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target, rep := disclosureFixture(t, db)
	withEncryptionKey(t)

	entry, err := entityarchive.RecordDisclosure(context.Background(), db, target, rep, nil,
		entityarchive.DisclosureRequest{Recipient: "УМВД", RequestRef: "исх. 3", IssuedBy: "консоль сервера"})
	require.NoError(t, err)

	require.NotEmpty(t, entry.SubjectHMAC)
	assert.Len(t, entry.SubjectHMAC, 64, "ключ - шестнадцатеричная свёртка sha256")
	assert.NotContains(t, entry.SubjectHMAC, target.PassportHMAC)
}
