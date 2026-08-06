package fakedata

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// Голоса согласующих в словаре UserApprovalRequest.Status (approval_tally.go держит те
// же значения приватными в internal/services -- недоступны отсюда, поэтому свои копии).
const (
	stageVoteApproved = "approved"
	stageVoteRejected = "rejected"
)

// stageBlacklistOverrideReason -- комментарий "всё равно пропустить" по предупреждениям о
// сходстве с чёрным списком (#481). blacklistsStep намеренно строит часть ЧС похожей на
// реальные записи реестров, но не идентичной -- SubmitCompleteApplication ловит это
// нечёткое сходство и помечает элемент заявки флагом независимо от того, что
// appVehicleCandidates/appEmployeeCandidates уже исключили ТОЧНЫЕ совпадения с ЧС из пула.
// Без снятия флага ApproveApplicationByUser('approved') отказывает 409 -- согласовать
// такую заявку тем же путём, что и вручную, нельзя было бы вовсе.
const stageBlacklistOverrideReason = "Наливка стенда: сходство с ЧС проверено, совпадение случайное"

// stagesStep прогоняет уже поданные заявки партии (applicationsStep) по стадиям
// обработки Центра заявок (#1682, том 7): часть остаётся непрочитанной, часть
// согласована/отклонена согласующими, большинство принято в работу принимающим, малая
// доля возвращена из работы обратно в обработку, малая доля отозвана автором. Без
// этого шага стенд показывает только "Непрочитано" -- ни один другой фильтр Центра
// (В обработке/В работе/Отказано/Отозвана, Согласовано/Не согласовано) не на чем
// проверить.
//
// Каждая стадия идёт через тот же сервисный метод, которым действует живой пользователь
// (TakeApplicationToWork/ApproveApplicationByUser/WithdrawApplication/
// RevokeApplicationFromWork/GetApplicationByID) -- прямая запись в applications.status
// оставила бы заявку без истории, без approval_datetime у согласующих и без гейтов,
// которые эти методы проверяют (согласование перед принятием, отзыв только автором и т.п.).
//
// Идёт последним в Steps(): нужны реальные заявки applicationsStep, их согласующие
// (application_responsible_users, назначенные при подаче) и принимающие usersStep
// (application_approvers) -- все уже должны быть в базе.
type stagesStep struct{}

func (stagesStep) Name() string { return "стадии обработки заявок" }

// Plan -- стадии не заводят новых сущностей, только переводят уже поданные заявки
// партии через существующие переходы, поэтому в предпоказе отдельной строки нет.
func (stagesStep) Plan(p Profile) []PlanItem { return nil }

func (stagesStep) Run(ctx context.Context, env *Env) error {
	apps, err := loadBatchApplications(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		// Profile.Applications<=0 -- applicationsStep ничего не подал, стадиям нечего
		// прогонять (то же короткое замыкание, что у самого applicationsStep).
		return nil
	}

	approvers, err := loadBatchApprovers(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return err
	}
	if len(approvers) == 0 {
		return fmt.Errorf("в партии нет ни одного принимающего -- стадиям некому читать, согласовывать " +
			"и принимать заявки в работу (шаг пользователей должен был назначить хотя бы одного, см. " +
			"approverUserCount в users.go)")
	}

	recorder := services.NewAuditRecorder(env.DB)
	appSvc := services.NewApplicationService(
		env.DB,
		services.NewPermissionService(env.DB),
		services.NewNotificationService(env.DB),
		services.NewVehicleBlacklistService(env.DB, recorder),
		services.NewPersonBlacklistService(env.DB, recorder),
		recorder,
		// Без опций: как и в applicationsStep, наливка не публикатор real-time сигналов
		// и не продюсер обновления таблиц/доступного охране -- у неё нет живых
		// подключённых клиентов. AvailableRefreshPublisher/TablesRefreshPublisher на nil-
		// получателе не паникуют (см. их собственные док-комментарии), поэтому опустить
		// их безопасно даже там, где стадии реально доводят заявку до "Согласовано".
	)

	streams := newStageStreams(env.Seed)
	now := time.Now().UTC()

	unreadN, approvedN, rejectedN, revokedN, withdrawnN := stageBucketSizes(len(apps))

	idx := 0
	idx += unreadN // непрочитанные -- пропускаем целиком, действий нет

	for _, app := range apps[idx : idx+approvedN] {
		reader := Pick(streams.reader, approvers)
		if err := runApprovedOnlyStage(ctx, appSvc, env.DB, reader, app, streams, now); err != nil {
			return fmt.Errorf("стадия «согласована» (заявка %d): %w", app.ID, err)
		}
	}
	idx += approvedN

	for _, app := range apps[idx : idx+rejectedN] {
		reader := Pick(streams.reader, approvers)
		if err := runRejectedStage(ctx, appSvc, env.DB, reader, app, streams, now); err != nil {
			return fmt.Errorf("стадия «отклонена» (заявка %d): %w", app.ID, err)
		}
	}
	idx += rejectedN

	for _, app := range apps[idx : idx+revokedN] {
		approver := Pick(streams.reader, approvers)
		if err := runRevokedStage(ctx, appSvc, env.DB, approver, app, streams, now); err != nil {
			return fmt.Errorf("стадия «возвращена из работы» (заявка %d): %w", app.ID, err)
		}
	}
	idx += revokedN

	for _, app := range apps[idx : idx+withdrawnN] {
		if err := runWithdrawnStage(ctx, appSvc, env.DB, app, streams, now); err != nil {
			return fmt.Errorf("стадия «отозвана» (заявка %d): %w", app.ID, err)
		}
	}
	idx += withdrawnN

	// Большинство -- обычный "счастливый путь": согласована и принята в работу.
	for _, app := range apps[idx:] {
		approver := Pick(streams.reader, approvers)
		if err := runAcceptedStage(ctx, appSvc, env.DB, approver, app, streams, now); err != nil {
			return fmt.Errorf("стадия «в работе» (заявка %d): %w", app.ID, err)
		}
	}

	return nil
}

// --- распределение заявок партии по стадиям ---

// stageBucketSizes делит заявки count'ами, а не броском монеты на каждую -- тот же
// приём и та же причина, что bannedUserCount/archivedUserCount в users.go: на
// маленьком профиле вероятностный бросок может не дать ни одной заявки в каком-то
// состоянии, и стадия не проверится вовсе (эта ловушка уже случилась на шаге
// пользователей). "Принята в работу" -- заведомое большинство (остаток после пяти
// меньшинств), остальное -- явные, но некрупные доли.
func stageBucketSizes(total int) (unread, approvedOnly, rejected, revoked, withdrawn int) {
	if total <= 0 {
		return
	}
	unread = userShareCount(total, 10)
	approvedOnly = userShareCount(total, 10)
	rejected = userShareCount(total, 15)
	revoked = userShareCount(total, 10)
	withdrawn = userShareCount(total, 15)

	minority := unread + approvedOnly + rejected + revoked + withdrawn
	if minority > total {
		// Заявок меньше, чем стадий-меньшинств: раздаём по одной, пока хватает, вместо
		// пропорционального ужатия. Пропорция при total<5 давала ноль сразу всем пяти
		// (целочисленное деление), и вся партия молча уходила в "принята в работу" --
		// стадий на стенде не оставалось вовсе, а команда об этом не сообщала.
		buckets := []*int{&unread, &approvedOnly, &rejected, &revoked, &withdrawn}
		left := total
		for _, b := range buckets {
			if left <= 0 {
				*b = 0
				continue
			}
			*b = 1
			left--
		}
	}
	return
}

// --- источники данных: заявки и люди партии ---

// batchApplication -- то немногое о поданной заявке, что нужно стадиям: id для
// сервисных вызовов, логин отправителя (WithdrawApplication принимает его, не id) и
// дата подачи -- отправная точка окна для исторических меток стадии.
type batchApplication struct {
	ID             int       `gorm:"column:id"`
	SenderUsername string    `gorm:"column:sender_username"`
	SentAt         time.Time `gorm:"column:sent_at"`
}

// loadBatchApplications читает заявки СТРОГО этой партии (через fake_batch_items) --
// стадии не должны задеть чужую заявку, поданную вручную или другой партией.
func loadBatchApplications(ctx context.Context, db *gorm.DB, batchID int) ([]batchApplication, error) {
	var rows []batchApplication
	err := db.WithContext(ctx).Raw(`
		SELECT a.id AS id, u.username AS sender_username, a.sending_datetime AS sent_at
		FROM applications a
		JOIN fake_batch_items fbi ON fbi.entity_id = a.id
		JOIN users u ON u.id = a.sender_user_id
		WHERE fbi.batch_id = ? AND fbi.entity = ?
		ORDER BY a.id`, batchID, models.AuditEntityApplication).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("заявки партии для стадий обработки: %w", err)
	}
	return rows, nil
}

// stageApprover -- принимающий, которым стадии читают/согласовывают/принимают заявки.
type stageApprover struct {
	UserID   int    `gorm:"column:user_id"`
	Username string `gorm:"column:username"`
}

// loadBatchApprovers читает принимающих, НАЗНАЧЕННЫХ usersStep этой партии (по перечню
// партии, entity=AuditEntityApprover -- та же сущность, которой ApproverService.Create
// пишет историю, см. соответствующий комментарий в users.go). Архивные/заблокированные
// исключены: под ними TakeApplicationToWork не проходит.
func loadBatchApprovers(ctx context.Context, db *gorm.DB, batchID int) ([]stageApprover, error) {
	var rows []stageApprover
	err := db.WithContext(ctx).Raw(`
		SELECT u.id AS user_id, u.username AS username
		FROM users u
		JOIN fake_batch_items fbi ON fbi.entity_id = u.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND u.is_active = true AND u.is_banned = false
		ORDER BY u.id`, batchID, models.AuditEntityApprover).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("принимающие партии для стадий обработки: %w", err)
	}
	return rows, nil
}

// responsibleVoter -- согласующий конкретной заявки, как он лежит в
// application_responsible_users (см. approverRow в users.go -- тот же приём чтения
// сырым SQL для состава, которого нет в GET-сервисах, отдающих его для интерфейса).
type responsibleVoter struct {
	UserID           int    `gorm:"column:user_id"`
	Username         string `gorm:"column:username"`
	RequiredApproval bool   `gorm:"column:required_approval"`
}

func loadResponsibleVoters(ctx context.Context, db *gorm.DB, applicationID int) ([]responsibleVoter, error) {
	var rows []responsibleVoter
	err := db.WithContext(ctx).Raw(`
		SELECT aru.user_id AS user_id, u.username AS username, aru.required_approval AS required_approval
		FROM application_responsible_users aru
		JOIN users u ON u.id = aru.user_id
		WHERE aru.application_id = ?
		ORDER BY aru.id`, applicationID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("согласующие заявки %d: %w", applicationID, err)
	}
	return rows, nil
}

// --- потоки случайности и исторические метки времени стадий ---

// stageStreams -- независимые потоки случайности стадий (см. applicationStreams в
// applications.go): каждый домен переходов получает свой поток, чтобы правка одного не
// сдвигала остальные при повторе с тем же -seed.
type stageStreams struct {
	reader      *Stream // какой принимающий читает/решает/принимает эту заявку
	readGap     *Stream // сколько минут после подачи заявку впервые открыли
	decideGap   *Stream // сколько минут после прочтения согласующие вынесли решение
	acceptGap   *Stream // сколько минут после решения заявку приняли в работу
	revokeGap   *Stream // сколько минут после принятия её вывели из работы
	withdrawGap *Stream // сколько минут после подачи заявитель отозвал заявку
}

func newStageStreams(seed int64) *stageStreams {
	return &stageStreams{
		reader:      NewStream(seed, "stage-reader"),
		readGap:     NewStream(seed, "stage-read-gap"),
		decideGap:   NewStream(seed, "stage-decide-gap"),
		acceptGap:   NewStream(seed, "stage-accept-gap"),
		revokeGap:   NewStream(seed, "stage-revoke-gap"),
		withdrawGap: NewStream(seed, "stage-withdraw-gap"),
	}
}

// stageBase -- точка отсчёта окна стадий заявки: не раньше подачи, но и не позже
// "сейчас". buildApplication (applications.go) при daysBack=0 иногда подбирает час
// подачи ПОЗЖЕ фактического момента запуска наливки (диапазон appWorkHourStart..End
// шире, чем прошло реального времени с начала прогона) -- без этой поправки окно
// [sentAt, now] стало бы отрицательным и IntRange запаниковал бы.
func stageBase(sentAt, now time.Time) time.Time {
	if sentAt.After(now) {
		return now
	}
	return sentAt
}

// nextStageMoment -- следующий момент стадии: строго не раньше prev и никогда не
// позже now. Момент берётся случайной долей ОСТАВШЕГОСЯ окна [prev, now], а не
// фиксированным шагом -- если заявка подана только что (окно узкое), метка
// схлопывается почти в now, а не проваливается за него.
func nextStageMoment(s *Stream, prev, now time.Time) time.Time {
	if !prev.Before(now) {
		return now
	}
	minutesLeft := int(now.Sub(prev).Minutes())
	if minutesLeft < 1 {
		return now
	}
	delta := time.Duration(IntRange(s, 1, minutesLeft)) * time.Minute
	next := prev.Add(delta)
	if next.After(now) {
		return now
	}
	return next
}

// --- сдвиг дат стадий (сырой SQL -- то же исключение из "сервисный слой, а не SQL",
// что shiftApplicationDates/shiftApplicationChildren в applications.go): сервисные
// методы стадий пишут NOW() реального времени, и без переноса переход по заявке
// месячной давности выглядел бы совершённым сегодня) ---

func shiftReadingDatetime(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE applications SET reading_datetime = ? WHERE id = ?`, at, appID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты прочтения заявки %d: %w", appID, err)
	}
	return nil
}

// shiftConfirmationDecision сдвигает момент решения согласующих. status_updated_at
// правится тут же: это единственное реальное изменение confirmation в этой стадии, и
// bumpStatusUpdated внутри ApproveApplicationByUser сработал ровно в этот момент.
func shiftConfirmationDecision(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE applications SET confirmation_datetime = ?, status_updated_at = ? WHERE id = ?`, at, at, appID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты решения согласующих заявки %d: %w", appID, err)
	}
	return nil
}

// shiftAcceptedAt сдвигает момент первого принятия в работу. status_updated_at здесь
// же перезаписывает то, что оставила предыдущая стадия (решение согласующих) -- accept
// бампает его безусловно, поэтому итоговое значение обязано быть моментом accept, а не
// более ранним решением.
func shiftAcceptedAt(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE applications SET accepted_at = ?, status_updated_at = ? WHERE id = ?`, at, at, appID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты принятия в работу заявки %d: %w", appID, err)
	}
	return nil
}

func shiftStatusUpdatedAt(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE applications SET status_updated_at = ? WHERE id = ?`, at, appID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты смены статуса заявки %d: %w", appID, err)
	}
	return nil
}

func shiftWithdrawnAt(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE applications SET withdrawn_at = ?, status_updated_at = ? WHERE id = ?`, at, at, appID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты отзыва заявки %d: %w", appID, err)
	}
	return nil
}

func shiftResponsibleVotes(ctx context.Context, db *gorm.DB, appID int, voterIDs []int, at time.Time) error {
	if len(voterIDs) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Exec(
		`UPDATE application_responsible_users SET approval_datetime = ? WHERE application_id = ? AND user_id IN ?`,
		at, appID, voterIDs,
	).Error; err != nil {
		return fmt.Errorf("сдвиг даты голосов заявки %d: %w", appID, err)
	}
	return nil
}

// shiftApplicationAuditLog переносит записи истории заявки конкретного действия --
// один вызов может задеть несколько строк (несколько "approve" при нескольких
// обязательных согласующих раунда), и это правильно: все голоса раунда датируются
// одним моментом решения.
func shiftApplicationAuditLog(ctx context.Context, db *gorm.DB, appID int, action string, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND entity_id = ? AND action = ?`,
		at, models.AuditEntityApplication, appID, action,
	).Error; err != nil {
		return fmt.Errorf("сдвиг истории (%s) заявки %d: %w", action, appID, err)
	}
	return nil
}

// shiftAttachmentItemsAuditLog переносит историю машин/сотрудников заявки для
// КОНКРЕТНОГО действия (action) -- переиспользуется и принятием в работу
// ("добавлен в таблицу проходной", activateApplicationItems активирует их этим же
// вызовом TakeApplicationToWork), и снятием предупреждения о сходстве с ЧС
// ("blacklist_override", пишется и на саму заявку, и на помеченный элемент, см.
// logBlacklistOverrideAction). Без переноса машина/сотрудник заявки месячной давности
// получили бы отметку в истории "только что" (то же рассуждение, что "историю машин"/
// "историю сотрудников" в shiftApplicationChildren, applications.go).
func shiftAttachmentItemsAuditLog(ctx context.Context, db *gorm.DB, appID int, action string, at time.Time) error {
	statements := []struct {
		what       string
		entityType string
		query      string
	}{
		{"машин", models.AuditEntityCar, `UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND action = ? AND entity_id IN (
			SELECT c.id FROM cars c JOIN attachments att ON att.id = c.attachment_id WHERE att.application_id = ?)`},
		{"сотрудников", models.AuditEntityEmployee, `UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND action = ? AND entity_id IN (
			SELECT e.id FROM employees e JOIN attachments att ON att.id = e.attachment_id WHERE att.application_id = ?)`},
	}
	for _, st := range statements {
		if err := db.WithContext(ctx).Exec(st.query, at, st.entityType, action, appID).Error; err != nil {
			return fmt.Errorf("сдвиг истории (%s, %s) заявки %d: %w", action, st.what, appID, err)
		}
	}
	return nil
}

// shiftAttachmentItemsTouched переносит на момент перехода отметку об изменении машин и
// сотрудников заявки.
//
// Активация и снятие с работы (activateApplicationItems) пишут в updated_at время
// прогона, и без этого переноса реестр показывал бы «обновлено сегодня» у машин из
// заявки месячной давности. Поле отдаётся по API, то есть видно снаружи.
func shiftAttachmentItemsTouched(ctx context.Context, db *gorm.DB, appID int, at time.Time) error {
	statements := []struct {
		what  string
		query string
	}{
		{"машин", `UPDATE cars SET updated_at = ? WHERE attachment_id IN (
			SELECT id FROM attachments WHERE application_id = ?)`},
		{"сотрудников", `UPDATE employees SET updated_at = ? WHERE attachment_id IN (
			SELECT id FROM attachments WHERE application_id = ?)`},
	}
	for _, st := range statements {
		if err := db.WithContext(ctx).Exec(st.query, at, appID).Error; err != nil {
			return fmt.Errorf("сдвиг отметки изменения (%s) заявки %d: %w", st.what, appID, err)
		}
	}
	return nil
}

// --- переходы стадий: сервисный вызов + перенос дат сразу следом ---

// stageRead открывает заявку от лица принимающего -- предпосылка для решения/принятия:
// без прочтения "Согласовано"/"В работе" на заявке, которую никто не открывал,
// выглядело бы так, будто бюро решает не глядя. GetApplicationByID -- тот же путь,
// которым сотрудник бюро открывает деталь заявки в интерфейсе; он же при первом
// прочтении не отправителем переводит "Непрочитано" -> "В обработке" и проставляет
// reading_datetime (см. application_service.go). Действует по построению ровно один
// раз на свежую заявку партии -- буквенно-числовые стадии друг друга не пересекают, и
// повторного вызова с уже снятым статусом здесь не бывает.
func stageRead(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, reader stageApprover, app batchApplication, at time.Time) error {
	if _, err := appSvc.GetApplicationByID(ctx, reader.Username, app.ID); err != nil {
		return fmt.Errorf("прочтение заявки %d принимающим %s: %w", app.ID, reader.Username, err)
	}
	if err := shiftReadingDatetime(ctx, db, app.ID, at); err != nil {
		return err
	}
	return shiftApplicationAuditLog(ctx, db, app.ID, "read", at)
}

// approveFully согласовывает заявку голосами ВСЕХ обязательных согласующих -- ровно
// условие, при котором tallyApprovals (approval_tally.go) переводит confirmation в
// "Согласовано". Необязательных согласующих не трогаем: их голос кворум не решает, и
// оставшийся "pending" -- реалистичная картина (не каждый согласующий успевает
// проголосовать до решения обязательных).
func approveFully(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, appID int, at time.Time) error {
	voters, err := loadResponsibleVoters(ctx, db, appID)
	if err != nil {
		return err
	}
	var requiredIDs []int
	var overseer string
	for _, v := range voters {
		if !v.RequiredApproval {
			continue
		}
		if overseer == "" {
			overseer = v.Username
		}
		requiredIDs = append(requiredIDs, v.UserID)
	}
	if len(requiredIDs) == 0 {
		return fmt.Errorf("у заявки %d нет ни одного обязательного согласующего -- согласовать целиком "+
			"нечем (шаг пользователей должен был назначить хотя бы одного каждой организации/компании, "+
			"см. ensureOrganizationApprovers/ensureCompanyApprovers в users.go)", appID)
	}
	// Снимаем блокировку по сходству с ЧС ДО голосов -- иначе первый же 'approved' на
	// помеченной заявке отказал бы 409 (см. stageBlacklistOverrideReason).
	if err := overridePendingBlacklistFlags(ctx, appSvc, db, appID, overseer, at); err != nil {
		return err
	}
	for _, v := range voters {
		if !v.RequiredApproval {
			continue
		}
		req := services.UserApprovalRequest{UserID: v.UserID, Status: stageVoteApproved}
		if err := appSvc.ApproveApplicationByUser(ctx, v.Username, appID, req); err != nil {
			return fmt.Errorf("согласование пользователем %s: %w", v.Username, err)
		}
	}
	if err := shiftResponsibleVotes(ctx, db, appID, requiredIDs, at); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, "approve", at); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, "confirmation_change", at); err != nil {
		return err
	}
	return shiftConfirmationDecision(ctx, db, appID, at)
}

// rejectOneRequired отклоняет заявку голосом ОДНОГО обязательного согласующего --
// tallyApprovals хоронит круг сразу на первом обязательном отказе, не дожидаясь
// остальных (approval_tally.go), поэтому одного голоса достаточно для честного
// "Не согласовано" и реалистичнее массового отказа всеми разом.
func rejectOneRequired(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, appID int, at time.Time) error {
	voters, err := loadResponsibleVoters(ctx, db, appID)
	if err != nil {
		return err
	}
	var target *responsibleVoter
	for i := range voters {
		if voters[i].RequiredApproval {
			target = &voters[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("у заявки %d нет ни одного обязательного согласующего -- отклонить нечем "+
			"(шаг пользователей должен был назначить хотя бы одного, см. ensureOrganizationApprovers в "+
			"users.go)", appID)
	}
	req := services.UserApprovalRequest{UserID: target.UserID, Status: stageVoteRejected}
	if err := appSvc.ApproveApplicationByUser(ctx, target.Username, appID, req); err != nil {
		return fmt.Errorf("отклонение пользователем %s: %w", target.Username, err)
	}
	if err := shiftResponsibleVotes(ctx, db, appID, []int{target.UserID}, at); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, "reject", at); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, "confirmation_change", at); err != nil {
		return err
	}
	return shiftConfirmationDecision(ctx, db, appID, at)
}

// acceptToWork принимает согласованную заявку в работу от имени принимающего --
// TakeApplicationToWork требует confirmation="Согласовано" (или полное отсутствие
// согласующих), поэтому approveFully обязан отработать раньше этого вызова.
func acceptToWork(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, approver stageApprover, appID int, at time.Time) error {
	req := services.TakeToWorkRequest{UserID: approver.UserID, Action: "accept"}
	if err := appSvc.TakeApplicationToWork(ctx, approver.Username, appID, req); err != nil {
		return fmt.Errorf("принятие в работу принимающим %s: %w", approver.Username, err)
	}
	if err := shiftAcceptedAt(ctx, db, appID, at); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, models.AuditActionTakeToWork, at); err != nil {
		return err
	}
	if err := shiftAttachmentItemsAuditLog(ctx, db, appID, models.AuditActionAddedToTable, at); err != nil {
		return err
	}
	return shiftAttachmentItemsTouched(ctx, db, appID, at)
}

// overridePendingBlacklistFlags снимает блокировку согласования по всем непокрытым
// предупреждениям о сходстве с ЧС заявки (см. stageBlacklistOverrideReason) -- от лица
// одного из её согласующих: OverrideBlacklistFlag требует, чтобы вызывающий сам был
// ответственным по заявке (как и голосование). Нет предупреждений -- нет и вызовов,
// согласование идёт как обычно.
func overridePendingBlacklistFlags(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, appID int, actorUsername string, at time.Time) error {
	var flagIDs []int
	err := db.WithContext(ctx).Raw(`
		SELECT f.id FROM application_blacklist_flags f
		WHERE f.application_id = ?
		AND NOT EXISTS (SELECT 1 FROM application_blacklist_overrides o WHERE o.flag_id = f.id)`, appID).
		Scan(&flagIDs).Error
	if err != nil {
		return fmt.Errorf("непокрытые предупреждения о сходстве с ЧС заявки %d: %w", appID, err)
	}
	if len(flagIDs) == 0 {
		return nil
	}
	for _, flagID := range flagIDs {
		req := services.OverrideBlacklistFlagRequest{FlagID: flagID, Comment: stageBlacklistOverrideReason}
		if err := appSvc.OverrideBlacklistFlag(ctx, actorUsername, appID, req); err != nil {
			return fmt.Errorf("подтверждение пропуска по предупреждению %d пользователем %s: %w", flagID, actorUsername, err)
		}
	}
	if err := shiftApplicationAuditLog(ctx, db, appID, "blacklist_override", at); err != nil {
		return err
	}
	return shiftAttachmentItemsAuditLog(ctx, db, appID, "blacklist_override", at)
}

// --- сценарии стадий ---

// runApprovedOnlyStage -- заявка прочитана и полностью согласована, но принимающий её
// ещё не тронул: confirmation="Согласовано", status остаётся "В обработке".
func runApprovedOnlyStage(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, reader stageApprover, app batchApplication, s *stageStreams, now time.Time) error {
	base := stageBase(app.SentAt, now)
	tRead := nextStageMoment(s.readGap, base, now)
	tDecide := nextStageMoment(s.decideGap, tRead, now)

	if err := stageRead(ctx, appSvc, db, reader, app, tRead); err != nil {
		return err
	}
	return approveFully(ctx, appSvc, db, app.ID, tDecide)
}

// runRejectedStage -- заявка прочитана и отклонена одним обязательным согласующим:
// confirmation="Не согласовано", status остаётся "В обработке".
func runRejectedStage(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, reader stageApprover, app batchApplication, s *stageStreams, now time.Time) error {
	base := stageBase(app.SentAt, now)
	tRead := nextStageMoment(s.readGap, base, now)
	tDecide := nextStageMoment(s.decideGap, tRead, now)

	if err := stageRead(ctx, appSvc, db, reader, app, tRead); err != nil {
		return err
	}
	return rejectOneRequired(ctx, appSvc, db, app.ID, tDecide)
}

// runAcceptedStage -- "счастливый путь" большинства партии: прочитана, согласована,
// принята в работу. status="В работе", confirmation="Согласовано".
func runAcceptedStage(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, approver stageApprover, app batchApplication, s *stageStreams, now time.Time) error {
	base := stageBase(app.SentAt, now)
	tRead := nextStageMoment(s.readGap, base, now)
	tDecide := nextStageMoment(s.decideGap, tRead, now)
	tAccept := nextStageMoment(s.acceptGap, tDecide, now)

	if err := stageRead(ctx, appSvc, db, approver, app, tRead); err != nil {
		return err
	}
	if err := approveFully(ctx, appSvc, db, app.ID, tDecide); err != nil {
		return err
	}
	return acceptToWork(ctx, appSvc, db, approver, app.ID, tAccept)
}

// runRevokedStage -- заявку успели прочитать, согласовать и принять в работу, но
// принимающий вывел её обратно в обработку: status возвращается в "В обработке"
// (RevokeApplicationFromWork), confirmation остаётся "Согласовано" -- отзыв из работы
// его не трогает. Тот же принимающий, что принимал: реалистичнее, чем случайный другой
// человек передумывает за него.
func runRevokedStage(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, approver stageApprover, app batchApplication, s *stageStreams, now time.Time) error {
	base := stageBase(app.SentAt, now)
	tRead := nextStageMoment(s.readGap, base, now)
	tDecide := nextStageMoment(s.decideGap, tRead, now)
	tAccept := nextStageMoment(s.acceptGap, tDecide, now)
	tRevoke := nextStageMoment(s.revokeGap, tAccept, now)

	if err := stageRead(ctx, appSvc, db, approver, app, tRead); err != nil {
		return err
	}
	if err := approveFully(ctx, appSvc, db, app.ID, tDecide); err != nil {
		return err
	}
	if err := acceptToWork(ctx, appSvc, db, approver, app.ID, tAccept); err != nil {
		return err
	}

	revokeReq := services.RevokeFromWorkRequest{UserID: approver.UserID}
	if err := appSvc.RevokeApplicationFromWork(ctx, approver.Username, app.ID, revokeReq); err != nil {
		return fmt.Errorf("возврат заявки %d из работы принимающим %s: %w", app.ID, approver.Username, err)
	}
	if err := shiftStatusUpdatedAt(ctx, db, app.ID, tRevoke); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, app.ID, "revoke_from_work", tRevoke); err != nil {
		return err
	}
	return shiftAttachmentItemsTouched(ctx, db, app.ID, tRevoke)
}

// runWithdrawnStage -- заявитель отзывает собственную заявку (#951): status="Отозвана".
// Без предварительного прочтения -- отзыв доступен и до того, как заявку кто-либо
// открыл (WithdrawApplication проверяет только принадлежность автору и что заявка ещё
// не в терминальном статусе), и часть реальных отзывов происходит именно так.
func runWithdrawnStage(ctx context.Context, appSvc services.ApplicationService, db *gorm.DB, app batchApplication, s *stageStreams, now time.Time) error {
	base := stageBase(app.SentAt, now)
	tWithdraw := nextStageMoment(s.withdrawGap, base, now)

	if err := appSvc.WithdrawApplication(ctx, app.SenderUsername, app.ID); err != nil {
		return fmt.Errorf("отзыв заявки %d автором %s: %w", app.ID, app.SenderUsername, err)
	}
	if err := shiftWithdrawnAt(ctx, db, app.ID, tWithdraw); err != nil {
		return err
	}
	if err := shiftApplicationAuditLog(ctx, db, app.ID, models.AuditActionWithdraw, tWithdraw); err != nil {
		return err
	}
	return shiftAttachmentItemsTouched(ctx, db, app.ID, tWithdraw)
}

// StageBucketSizesForTest открывает распределение стадий проверке: сама функция
// приватная, а поведение на маленькой партии стоит сторожить -- пропорциональное ужатие
// однажды уже обнуляло все меньшинства разом.
func StageBucketSizesForTest(total int) (unread, approvedOnly, rejected, revoked, withdrawn int) {
	return stageBucketSizes(total)
}
