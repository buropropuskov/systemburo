package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Просмотр состава участников заявки - обращение к персональным данным: метод отдаёт
// рабочую почту и телефон каждого. Журнал 152-ФЗ обязан это видеть, иначе через
// карточку заявки контакты собираются мимо учёта.
//
// Запись идёт из middleware в отдельной горутине, поэтому ждём её появления, а не
// читаем сразу.
func waitPDAuditRecord(t *testing.T, db *gorm.DB, path string) models.PDAuditLog {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var log models.PDAuditLog
		err := db.Where("path = ?", path).Order("id DESC").First(&log).Error
		if err == nil {
			return log
		}
		if time.Now().After(deadline) {
			t.Fatalf("запись журнала для %s не появилась за отведённое время: %v", path, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestPDAudit_ParticipantsViewLogged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken, _ := registerParticipant(t, e, db, "pda_sender",
		"Отправителев", "Олег", "Олегович", "pda_sender@example.com", "+7 900 000 10 01", td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	path := fmt.Sprintf("/applications/%d/participants", appID)
	rec := testutil.GET(t, e, path, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	log := waitPDAuditRecord(t, db, "/api"+path)
	assert.Equal(t, "view", log.Action, "чтение состава - просмотр")
	assert.Equal(t, "application_participants", log.Resource, "раздел журнала назван, иначе фильтр его не найдёт")
	assert.Equal(t, "pda_sender", log.Username, "запись привязана к тому, кто смотрел")
	assert.Equal(t, http.StatusOK, log.StatusCode)
	require.NotNil(t, log.UserID, "по одному имени запись не привязать: работника могут переименовать")
}

// Отказ тоже попадает в журнал: попытка посмотреть контакты по чужой заявке - событие
// не менее важное, чем удачный просмотр, и до #1472 такие записи уходили как успешные.
func TestPDAudit_ParticipantsDeniedLogged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken, _ := registerParticipant(t, e, db, "pda_owner",
		"Владелев", "Владимир", "Владимирович", "pda_owner@example.com", "+7 900 000 10 02", td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	outsiderToken := testutil.RegisterAndLogin(t, e, "pda_outsider", participantPassword, 1, td.OrgID, td.CompanyID)

	path := fmt.Sprintf("/applications/%d/participants", appID)
	rec := testutil.GET(t, e, path, testutil.AuthHeader(outsiderToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "посторонний состав участников не видит")

	var log models.PDAuditLog
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := db.Where("path = ? AND username = ?", "/api"+path, "pda_outsider").Order("id DESC").First(&log).Error
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("отказ не попал в журнал: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Equal(t, http.StatusForbidden, log.StatusCode, "в журнале записан именно отказ, а не успешный просмотр")
	assert.Equal(t, "application_participants", log.Resource)
}
