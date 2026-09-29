package handlers_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// idorActor - кто шлёт запрос замка IDOR. Чужак - пользователь другой организации без
// общей компании: совпавший company_id сам даёт доступ к реестру и к элементам поста.
type idorActor string

const (
	idorOwner    idorActor = "owner"    // автор заявки и записей реестра
	idorVoter    idorActor = "voter"    // согласующий с голосом «на рассмотрении»
	idorVoted    idorActor = "voted"    // согласующий, уже отдавший голос
	idorStranger idorActor = "stranger" // другая организация, автор своей заявки
)

// idorWorld - пользователи и общие объекты замка, одни на весь тест.
type idorWorld struct {
	e         *echo.Echo
	db        *gorm.DB
	uploads   string
	orgA      int
	orgB      int
	tokens    map[idorActor]string
	ids       map[idorActor]int
	blankApp  int
	blankAtt  int
	seedCount int
}

// idorFixture - объекты организации A, к которым чужак идёт по id, плюс своя заявка
// чужака для подмены «своя заявка, чужой дочерний объект». Заводится заново на каждый
// запрос: изменяющие методы не должны влиять друг на друга.
type idorFixture struct {
	app, carsAtt, peopleAtt, itemsAtt int
	car, employee                     int
	file, draft                       int
	question, supplement              int
	flag                              int
	uniqueCar, uniqueEmployee         int
	notification                      int
	plate                             string
	strangerApp                       int
	blankApp, blankAtt                int
}

func newIDORWorld(t *testing.T) *idorWorld {
	t.Helper()
	e, db, uploads, cleanup := testutil.SetupTestAppWithUploads(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	orgB := models.Organization{Name: "Чужая организация замка"}
	require.NoError(t, db.Create(&orgB).Error)

	w := &idorWorld{
		e: e, db: db, uploads: uploads, orgA: td.OrgID, orgB: orgB.ID,
		tokens: map[idorActor]string{}, ids: map[idorActor]int{},
	}
	orgs := map[idorActor]int{idorOwner: td.OrgID, idorVoter: td.OrgID, idorVoted: td.OrgID, idorStranger: orgB.ID}
	for actor, org := range orgs {
		name := "idor_" + string(actor)
		w.tokens[actor] = testutil.RegisterAndLogin(t, e, name, "pass123", 1, org, 0)
		w.ids[actor] = getUserID(t, db, name)
	}
	// Дополнять заявку можно только с правом: у чужака оно тоже есть, иначе запрос
	// упрётся в гейт роутера и проверка владения заявкой не будет видна.
	testutil.GrantPermission(t, w.ids[idorOwner], services.KeyActionSupplementApplication)
	testutil.GrantPermission(t, w.ids[idorStranger], services.KeyActionSupplementApplication)

	// Бланк требует шаблона с файлом; запрос только читает, поэтому он один на тест.
	blank := seedDocumentsGateApplication(t, db, td, w.ids[idorOwner])
	w.blankApp, w.blankAtt = blank.appID, blank.attID
	return w
}

// seed заводит объекты организации A. withRound - открытый раунд дополнения: с ним
// нельзя подать новое дополнение, поэтому запрос на подачу берёт заявку без раунда.
func (w *idorWorld) seed(t *testing.T, withRound bool) idorFixture {
	t.Helper()
	w.seedCount++
	n := w.seedCount
	db, owner := w.db, w.ids[idorOwner]

	fx := idorFixture{blankApp: w.blankApp, blankAtt: w.blankAtt}
	fx.app = suppApp(t, db, w.orgA, owner, fmt.Sprintf("IDOR-%d", n), models.ConfirmationPending, models.StatusProcessing)
	fx.strangerApp = suppApp(t, db, w.orgB, w.ids[idorStranger], fmt.Sprintf("IDOR-B-%d", n),
		models.ConfirmationPending, models.StatusProcessing)
	suppResponsible(t, db, fx.app, w.ids[idorVoter], true, "pending")
	suppResponsible(t, db, fx.app, w.ids[idorVoted], true, "approved")

	fx.carsAtt = suppAttachment(t, db, fx.app, "cars", "2099-12-31")
	fx.peopleAtt = suppAttachment(t, db, fx.app, "people", "2099-12-31")
	fx.itemsAtt = suppAttachment(t, db, fx.app, "items", "2099-12-31")

	fx.plate = fmt.Sprintf("А%03dВВ777", n)
	car := models.Car{AttachmentID: fx.carsAtt, CarNumber: &fx.plate}
	require.NoError(t, db.Create(&car).Error)
	fx.car = car.ID
	fx.employee = suppNewEmployee(t, db, fx.peopleAtt, "Замков", nil)

	fx.file = seedApplicationFile(t, db, w.uploads, fx.app, fmt.Sprintf("idor-%d.pdf", n), []byte("%PDF-1.4 замок")).ID
	fx.draft = w.seedDraft(t, n)

	q := models.ApplicationQuestion{ApplicationID: fx.app, AuthorUserID: owner, Subject: "Вопрос замка", Text: "Текст"}
	require.NoError(t, db.Create(&q).Error)
	fx.question = q.ID

	// Флаг ЧС с уже подтверждённым пропуском: есть что снимать и что подтверждать
	// повторно, а голосование он не блокирует.
	flag := models.ApplicationBlacklistFlag{
		ApplicationID: fx.app, ElementType: "cars", ElementID: fx.car,
		ElementNormalized: fx.plate, MatchedBlacklistID: 1, MatchedValue: fx.plate,
	}
	require.NoError(t, db.Create(&flag).Error)
	fx.flag = flag.ID
	require.NoError(t, db.Create(&models.ApplicationBlacklistOverride{
		FlagID: flag.ID, ApplicationID: fx.app, ElementType: "cars", ElementID: fx.car,
		ElementNormalized: fx.plate, MatchedBlacklistID: 1, MatchedValue: fx.plate,
		OverriddenByUserID: w.ids[idorVoter], Comment: "пропуск подтверждён",
	}).Error)

	if withRound {
		fx.supplement = suppNewSupplement(t, db, fx.app, owner, models.SupplementPending)
		suppVoteApproval(t, db, fx.supplement, w.ids[idorVoter], true)
		approved, now := "approved", time.Now().UTC()
		require.NoError(t, db.Create(&models.ApplicationSupplementApproval{
			SupplementID: fx.supplement, UserID: w.ids[idorVoted], RequiredApproval: true,
			ApprovalStatus: &approved, ApprovalDatetime: &now,
		}).Error)
	}

	mark, lastName, firstName := "DAF", "Реестров", "Павел"
	uc := models.UniqueCar{Number: &fx.plate, Mark: &mark, UserID: &owner, OrganizationID: &w.orgA}
	require.NoError(t, db.Create(&uc).Error)
	fx.uniqueCar = uc.ID
	ue := models.UniqueEmployee{LastName: &lastName, FirstName: &firstName, UserID: &owner, OrganizationID: &w.orgA}
	require.NoError(t, db.Create(&ue).Error)
	fx.uniqueEmployee = ue.ID

	title := "Уведомление замка"
	note := models.Notification{UserID: owner, Title: &title}
	require.NoError(t, db.Create(&note).Error)
	fx.notification = note.ID
	return fx
}

// seedDraft - файл, загруженный владельцем, но ещё не приложенный к заявке.
func (w *idorWorld) seedDraft(t *testing.T, n int) int {
	t.Helper()
	dir := filepath.Join(w.uploads, services.ApplicationFilesDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	stored := fmt.Sprintf("idor-draft-%d.pdf", n)
	content := []byte("%PDF-1.4 черновик")
	require.NoError(t, os.WriteFile(filepath.Join(dir, stored), content, 0o600))
	row := models.ApplicationFile{
		FileName: "черновик.pdf", StoredName: stored, MimeType: "application/pdf",
		FileSize: int64(len(content)), UploadedBy: w.ids[idorOwner],
	}
	require.NoError(t, w.db.Create(&row).Error)
	return row.ID
}
