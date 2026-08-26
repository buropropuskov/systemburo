package handlers_test

// Согласие на обработку ПД у налитых работников (#1682 + #1567): стенд с включённым
// запросом согласия встречал каждого созданного наливкой работника окном согласия и до
// самой системы не пускал -- данные в списках есть, а посмотреть их под работником
// нельзя. Проверяем то, что видит человек: гейт согласия налитого работника не держит,
// и согласие записано действующей редакцией, а не «первой попавшейся».
// testutil.SetupTestApp поднимает базу -- по правилу проекта такие тесты живут только в
// internal/handlers.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeConsentEnv включает запрос согласия на стенде и отдаёт редакцию с отпечатком --
// ровно так же, как их читает команда наливки (fakeConsentStamp в cmd/server/fake.go).
func fakeConsentEnv(t *testing.T, db *gorm.DB) (services.SettingsService, *services.PDConsentGateService, fakedata.ConsentStamp) {
	t.Helper()
	ctx := context.Background()
	settings := services.NewSettingsService(db, &config.Config{PaginationMaxLimit: 100})
	require.NoError(t, settings.SetPDConsentText(ctx, "<p>Согласие на обработку персональных данных</p>", true))
	require.NoError(t, settings.SetPDConsentRequired(ctx, true))
	// Ещё один подъём редакции: согласие, записанное «первой» версией, на таком стенде
	// гейт не устроит -- именно это и должно быть поймано, если наливка перестанет
	// брать редакцию из настроек.
	_, err := settings.BumpPDConsentVersion(ctx)
	require.NoError(t, err)

	gate := services.NewPDConsentGateService(services.NewConsentService(db), settings, time.Minute)
	req, err := gate.Requirement(ctx)
	require.NoError(t, err)
	require.True(t, req.Enabled, "запрос согласия должен быть включён -- иначе проверять нечего")
	require.Greater(t, req.Version, 1)
	return settings, gate, fakedata.ConsentStamp{Version: req.Version, Hash: req.Hash}
}

func TestFakeUsers_GrantsPDConsentToEveryCreatedUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	_, gate, stamp := fakeConsentEnv(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-consent"), 515, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{
		DB: db, Batch: batch, Profile: profile, Seed: 515, Consent: stamp,
	}))

	var userIDs []int
	require.NoError(t, db.Raw(`
		SELECT entity_id FROM fake_batch_items WHERE batch_id = ? AND entity = ? ORDER BY entity_id`,
		batch.ID(), models.AuditEntityUser).Scan(&userIDs).Error)
	require.Len(t, userIDs, profile.Users)

	for _, id := range userIDs {
		needs, err := gate.NeedsConsent(ctx, id)
		require.NoError(t, err)
		require.False(t, needs, "налитый работник %d обязан пройти гейт согласия, а не упереться в окно", id)
	}

	var consents []models.PDConsent
	require.NoError(t, db.Where("user_id IN ?", userIDs).Find(&consents).Error)
	require.Len(t, consents, len(userIDs), "согласие должно быть у каждого созданного работника, включая заблокированных и архивных")
	for _, c := range consents {
		require.Equal(t, services.ConsentTypePDProcessing, c.ConsentType)
		require.True(t, c.Granted)
		require.Nil(t, c.RevokedAt)
		require.Equal(t, stamp.Version, c.DocumentVersion, "редакция берётся из настроек стенда")
		require.Equal(t, stamp.Hash, c.DocumentHash)
	}
}

// Согласие уходит вместе с работником: иначе после удаления партии в pd_consents
// оставались бы записи о людях, которых больше нет.
func TestFakeUsers_PurgeRemovesConsents(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	_, _, stamp := fakeConsentEnv(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-consent-purge")
	batch, err := fakedata.OpenBatch(ctx, db, label, 516, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{
		DB: db, Batch: batch, Profile: profile, Seed: 516, Consent: stamp,
	}))

	var before int64
	require.NoError(t, db.Model(&models.PDConsent{}).Count(&before).Error)
	require.GreaterOrEqual(t, before, int64(profile.Users))

	_, err = fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)

	var after int64
	require.NoError(t, db.Model(&models.PDConsent{}).Count(&after).Error)
	require.Equal(t, before-int64(profile.Users), after,
		"после удаления партии согласий должно остаться ровно столько, сколько было до неё")
}
