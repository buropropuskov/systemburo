package handlers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

// TestBlacklist_BatchIsFasterThanPerRow измеряет то, ради чего пакетный поиск заводился:
// подача большого списка упиралась в число обращений к базе, а не в вычисления. Чёрный
// список короткий, поэтому один запрос на 500 значений обязан идти заметно быстрее, чем
// 500 запросов по одному.
//
// Порог намеренно мягкий (втрое): тест меряет время на живой базе и не должен мигать от
// нагрузки соседних прогонов. Реальная разница на порядок больше.
func TestBlacklist_BatchIsFasterThanPerRow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	mark := seedMark(t, db, "BL_Bench")
	ctx := context.Background()
	svc := newVehicleBlacklistService(db)

	for i := 0; i < 4; i++ {
		_, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
			CarNumber: fmt.Sprintf("А%03dВС799", 100+i), MarkID: mark.ID, Reason: "нагрузка",
		}, userID)
		require.NoError(t, err)
	}

	const строк = 500
	номера := make([]string, 0, строк)
	for i := 0; i < строк; i++ {
		номера = append(номера, fmt.Sprintf("Х%03dУК%03d", i%1000, i%900))
	}

	// Прогрев: первый запрос платит за план и соединение, в замер это попадать не должно.
	_, err := svc.FindSimilarBatch(ctx, номера[:10])
	require.NoError(t, err)

	стартПострочно := time.Now()
	for _, n := range номера {
		_, err := svc.FindSimilar(ctx, n)
		require.NoError(t, err)
	}
	построчно := time.Since(стартПострочно)

	стартПакет := time.Now()
	_, err = svc.FindSimilarBatch(ctx, номера)
	require.NoError(t, err)
	пакетом := time.Since(стартПакет)

	t.Logf("построчно %d значений: %s; пакетом: %s; быстрее в %.1f раза",
		строк, построчно, пакетом, float64(построчно)/float64(пакетом))
	require.Less(t, пакетом*3, построчно,
		"пакетный поиск обязан быть заметно быстрее построчного: %s против %s", пакетом, построчно)
}
