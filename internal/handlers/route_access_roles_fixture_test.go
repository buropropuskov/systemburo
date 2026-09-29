package handlers_test

import (
	"fmt"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// roleActor - роль пользователя внутри одной заявки. Все шестеро из одной организации:
// чужую организацию проверяет замок IDOR, здесь отказ обязан идти от роли, а не от
// организации.
type roleActor string

const (
	roleAuthor      roleActor = "заявитель"
	roleVoter       roleActor = "согласующий"   // required_approval, голос обязателен
	roleResponsible roleActor = "ответственный" // строка согласующих без обязательного голоса
	roleReader      roleActor = "читатель"      // application_viewers, только просмотр
	roleAccepter    roleActor = "принимающий"   // application_approvers, видит все заявки
	roleStranger    roleActor = "посторонний"   // своя организация, к заявке не причастен
)

var roleActors = []roleActor{roleAuthor, roleVoter, roleResponsible, roleReader, roleAccepter, roleStranger}

// roleNewcomer - получатель пересылки: коллега из той же организации, в заявке не участвует.
const roleNewcomer = "новичок"

type roleWorld struct {
	e         *echo.Echo
	db        *gorm.DB
	org       int
	tokens    map[roleActor]string
	ids       map[roleActor]int
	newcomer  int
	seedCount int
}

// roleSeed - состояние заявки, в котором законный запрос действия проходит.
type roleSeed struct {
	status, confirmation string
	// vote - голос согласующего и ответственного в основном круге.
	vote string
	// round - статус раунда дополнения; пусто - раунда нет.
	round string
	// roundVote - голос согласующего и ответственного в раунде.
	roundVote string
	// flag - предупреждение ЧС по машине; overridden - пропуск по нему уже подтверждён.
	flag, overridden bool
}

// roleApp - заявка матрицы и её дочерние объекты.
type roleApp struct {
	app, carsAtt, itemsAtt, car, question, flag, supplement int
}

func newRoleWorld(t *testing.T) *roleWorld {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	w := &roleWorld{e: e, db: db, org: td.OrgID, tokens: map[roleActor]string{}, ids: map[roleActor]int{}}
	for i, actor := range roleActors {
		name := fmt.Sprintf("roles_%d", i)
		w.tokens[actor] = testutil.RegisterAndLogin(t, e, name, "pass123", 1, td.OrgID, td.CompanyID)
		w.ids[actor] = getUserID(t, db, name)
		// Право дополнять выдано всем: иначе запрос упрётся в гейт роутера, и отказ
		// по авторству не будет виден.
		testutil.GrantPermission(t, w.ids[actor], services.KeyActionSupplementApplication)
	}
	testutil.RegisterUser(t, e, "roles_newcomer", "pass123", 1, td.OrgID, td.CompanyID)
	w.newcomer = getUserID(t, db, "roles_newcomer")
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: w.ids[roleAccepter]}).Error)
	return w
}

// seed заводит свежую заявку: изменяющие запросы не должны влиять друг на друга.
func (w *roleWorld) seed(t *testing.T, s roleSeed) roleApp {
	t.Helper()
	w.seedCount++
	db := w.db
	f := roleApp{}
	f.app = suppApp(t, db, w.org, w.ids[roleAuthor], fmt.Sprintf("ROLES-%d", w.seedCount), s.confirmation, s.status)
	suppResponsible(t, db, f.app, w.ids[roleVoter], true, s.vote)
	suppResponsible(t, db, f.app, w.ids[roleResponsible], false, s.vote)
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: f.app, UserID: w.ids[roleReader]}).Error)

	f.carsAtt = suppAttachment(t, db, f.app, "cars", "2099-12-31")
	f.itemsAtt = suppAttachment(t, db, f.app, "items", "2099-12-31")
	plate := fmt.Sprintf("Р%03dЛЬ777", w.seedCount%1000)
	car := models.Car{AttachmentID: f.carsAtt, CarNumber: &plate}
	require.NoError(t, db.Create(&car).Error)
	f.car = car.ID

	q := models.ApplicationQuestion{ApplicationID: f.app, AuthorUserID: w.ids[roleAuthor], Subject: "Вопрос", Text: "Текст"}
	require.NoError(t, db.Create(&q).Error)
	f.question = q.ID

	if s.flag {
		flag := models.ApplicationBlacklistFlag{
			ApplicationID: f.app, ElementType: "cars", ElementID: f.car,
			ElementNormalized: plate, MatchedBlacklistID: 1, MatchedValue: plate,
		}
		require.NoError(t, db.Create(&flag).Error)
		f.flag = flag.ID
		if s.overridden {
			require.NoError(t, db.Create(&models.ApplicationBlacklistOverride{
				FlagID: flag.ID, ApplicationID: f.app, ElementType: "cars", ElementID: f.car,
				ElementNormalized: plate, MatchedBlacklistID: 1, MatchedValue: plate,
				OverriddenByUserID: w.ids[roleVoter], Comment: "пропуск подтверждён",
			}).Error)
		}
	}

	if s.round != "" {
		f.supplement = suppNewSupplement(t, db, f.app, w.ids[roleAuthor], s.round)
		for _, actor := range []roleActor{roleVoter, roleResponsible} {
			vote := s.roundVote
			require.NoError(t, db.Create(&models.ApplicationSupplementApproval{
				SupplementID: f.supplement, UserID: w.ids[actor],
				RequiredApproval: actor == roleVoter, ApprovalStatus: &vote,
			}).Error)
		}
	}
	return f
}

// trace - всё, что изменяющий запрос мог сдвинуть в заявке, плюс записи журнала от
// имени того, кто его прислал. Отказ обязан оставить строку прежней.
func (w *roleWorld) trace(t *testing.T, f roleApp, actorID int) string {
	t.Helper()
	var row struct {
		Audit                            int64
		Status, Confirmation, BureauNote string
		Circle, Round                    string
		Viewers, Overrides, ActiveCars   int64
		Period                           string
	}
	require.NoError(t, w.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM audit_log WHERE actor_user_id = ?) AS audit,
			COALESCE(a.status, '') AS status,
			COALESCE(a.confirmation, '') AS confirmation,
			COALESCE(a.bureau_note, '') AS bureau_note,
			(SELECT COALESCE(string_agg(user_id || ':' || required_approval || ':' || COALESCE(approval_status, ''), ',' ORDER BY user_id), '')
				FROM application_responsible_users WHERE application_id = a.id) AS circle,
			(SELECT COALESCE(string_agg(s.status || '/' || COALESCE(v.votes, ''), ',' ORDER BY s.id), '')
				FROM application_supplements s
				LEFT JOIN LATERAL (
					SELECT string_agg(user_id || ':' || COALESCE(approval_status, ''), ';' ORDER BY user_id) AS votes
					FROM application_supplement_approvals WHERE supplement_id = s.id
				) v ON true
				WHERE s.application_id = a.id) AS round,
			(SELECT COUNT(*) FROM application_viewers WHERE application_id = a.id) AS viewers,
			(SELECT COUNT(*) FROM application_blacklist_overrides WHERE application_id = a.id) AS overrides,
			(SELECT COUNT(*) FROM cars WHERE id = ? AND date_removed IS NULL) AS active_cars,
			(SELECT COALESCE(string_agg(entry_date_from || '..' || entry_date_to, ',' ORDER BY id), '')
				FROM attachments WHERE application_id = a.id) AS period
		FROM applications a WHERE a.id = ?`, actorID, f.car, f.app).Scan(&row).Error)
	return strings.Join([]string{
		fmt.Sprintf("журнал=%d", row.Audit), "статус=" + row.Status, "подтверждение=" + row.Confirmation,
		"заметка=" + row.BureauNote, "круг=" + row.Circle, "раунд=" + row.Round,
		fmt.Sprintf("читатели=%d пропуски=%d машин=%d", row.Viewers, row.Overrides, row.ActiveCars),
		"срок=" + row.Period,
	}, " ")
}
