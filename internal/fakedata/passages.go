package fakedata

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"gorm.io/gorm"
)

// passagesStep отмечает въезды и выезды машин и сотрудников партии через посты
// проходной (#1682, том 8): часть машин/людей уже поданных и принятых в работу
// заявок (applicationsStep -> stagesStep) отмечается фактически прошедшей КПП --
// иначе стенд показывает заявки "в работе", но ни одной живой отметки прохода, и
// разделы "Проезд"/"Проходная" (текущий статус на территории) и суточные отчёты
// охранника (DailyPassReportService) остаются пустыми.
//
// Отмечает ТОЛЬКО тех, кто реально виден на посту: машина/сотрудник со status=1 и
// хотя бы одной привязкой к таблице поста (car_target_tables/employee_target_tables) --
// та же связка, что видит охрана в tableCarsBase/GetActiveEmployeesForTable.
// activateApplicationItems переводит status в 1 ровно у принятых в работу заявок
// (#1085); заявки, возвращённые из работы, отклонённые или отозванные, здесь не
// встречаются вовсе -- activateApplicationItems(false) уже вернул их status в 0, и
// на посту (как и у настоящей охраны) таких не видно.
//
// Автор отметки -- администратор партии, а не "принимающий" или тип пользователя
// "охрана": право отмечать проход (table.<пост>.entry/exit, RequireTablePassVerb)
// раздаётся ролями/группами или флагом is_admin/is_super_admin (см.
// permission_resolver.go), а наливка пользователей (usersStep) не выдаёт "охране"
// никакого автоматического права -- код типа пользователя с полномочиями не связан
// после эпика отвязки от user_type. Единственные учётные записи партии, у которых
// это право гарантированно есть -- администраторы (is_admin) и супер-админ
// (Env.ActorUserID), поэтому исторически честный автор -- один из них.
//
// Идёт последним в Steps(): нужны и заявки applicationsStep, и переходы stagesStep
// (accepted_at, активные строки на посту) -- обе стадии обязаны отработать раньше.
type passagesStep struct{}

func (passagesStep) Name() string { return "проходы через посты" }

// Plan -- ОЦЕНКА по числу машин/сотрудников профиля (Profile.Cars/Profile.Employees),
// а не потолок: отмечаются не записи реестра, а машины и люди во вложениях заявок, и
// одна машина реестра попадает в несколько заявок. На стенде 11.08.2026 профиль small
// (20 машин, 30 сотрудников) дал 25 и 32 отметки -- больше собственных чисел профиля.
// Сколько выйдет на самом деле, известно только после того, как applicationsStep/
// stagesStep разыграют состав и стадии, а Plan по контракту пакета базу не читает.
func (passagesStep) Plan(p Profile) []PlanItem {
	if p.Applications <= 0 {
		return nil
	}
	return []PlanItem{
		{Entity: models.AuditEntityCar, Title: EntityTitle(models.AuditEntityCar), Count: p.Cars, Mark: true},
		{Entity: models.AuditEntityEmployee, Title: EntityTitle(models.AuditEntityEmployee), Count: p.Employees, Mark: true},
	}
}

func (passagesStep) Run(ctx context.Context, env *Env) error {
	if env.Profile.Applications <= 0 {
		// applicationsStep ничего не подал -- проходам нечего прогонять, то же
		// короткое замыкание, что у stagesStep на пустой партии заявок. Проверяем ДО
		// поиска автора отметки: на профиле без заявок и без пользователей (Users=0)
		// в партии может не найтись ни одного администратора, а падать здесь не за что --
		// делать всё равно нечего.
		return nil
	}

	actors, err := loadPassageActors(ctx, env)
	if err != nil {
		return err
	}

	cars, err := loadPassageCarCandidates(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return err
	}
	employees, err := loadPassageEmployeeCandidates(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return err
	}
	if len(cars) == 0 && len(employees) == 0 {
		// В отличие от Profile.Applications<=0 выше -- это партия, где заявки БЫЛИ, но
		// ни одна не дошла до "принята в работу" (status=1 у её машин/сотрудников).
		// Предпосылок для отметки прохода нет -- честная ошибка вместо тихого пропуска:
		// молчание здесь замаскировало бы то, что стадии обработки отработали не так,
		// как ожидалось.
		return fmt.Errorf("в партии нет ни одной машины/сотрудника заявки со статусом %q -- отмечать "+
			"проход некому и негде (шаг стадий должен был перевести хотя бы одну заявку партии в работу)",
			models.StatusInWork)
	}

	recorder := services.NewAuditRecorder(env.DB)
	carSvc := services.NewCarService(env.DB, recorder)
	empSvc := services.NewEmployeeService(env.DB, recorder)

	streams := newPassageStreams(env.Seed)
	now := time.Now().UTC()

	markedCars, err := runCarPassages(ctx, carSvc, env.DB, cars, actors, streams, now)
	if err != nil {
		return fmt.Errorf("проходы машин: %w", err)
	}
	markedEmployees, err := runEmployeePassages(ctx, empSvc, env.DB, employees, actors, streams, now)
	if err != nil {
		return fmt.Errorf("проходы сотрудников: %w", err)
	}
	// Не Batch.Add: машина и сотрудник уже принадлежат заявке партии и уйдут вместе с
	// ней, а отметка -- действие над ними, удалять по ней нечего. Счёт нужен отчёту:
	// фактическое число всегда меньше плана (до поста доходят только принятые в работу
	// и с непросроченным пропуском), и разницу человек должен видеть числом.
	env.Batch.Mark(models.AuditEntityCar, markedCars)
	env.Batch.Mark(models.AuditEntityEmployee, markedEmployees)
	return nil
}

// --- автор отметки ---

// loadPassageActors -- администраторы партии, которым по коду разрешено отмечать
// проход (см. докстринг passagesStep). Пустой список означает "в партии никто не
// подходит" -- тогда резервом идёт администратор стенда (Env.ActorUserID), от чьего
// имени наливка пишет остальную историю: он либо супер-админ (allowAll), либо
// администратор (adminAll), и оба покрывают динамическое право table.*.entry/exit
// (оно не входит в superOnlyKeys, см. permission_catalog.go).
func loadPassageActors(ctx context.Context, env *Env) ([]int, error) {
	admins, err := loadBatchAdminIDs(ctx, env.DB, env.Batch.ID())
	if err != nil {
		return nil, err
	}
	if len(admins) > 0 {
		return admins, nil
	}
	if env.ActorUserID > 0 {
		return []int{env.ActorUserID}, nil
	}
	return nil, fmt.Errorf("отмечать проход некому: в партии нет ни одного администратора " +
		"(шаг пользователей должен был назначить хотя бы одного, см. adminUserCount в users.go) и не " +
		"определён администратор стенда (Env.ActorUserID)")
}

// loadBatchAdminIDs читает активных администраторов, НАЗНАЧЕННЫХ usersStep этой
// партии (по перечню партии, entity=AuditEntityUser) -- зеркало loadBatchApprovers в
// stages.go. Архивные/заблокированные исключены: их вход не должен считаться
// действующим автором.
func loadBatchAdminIDs(ctx context.Context, db *gorm.DB, batchID int) ([]int, error) {
	var ids []int
	err := db.WithContext(ctx).Raw(`
		SELECT u.id
		FROM users u
		JOIN fake_batch_items fbi ON fbi.entity_id = u.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND u.is_admin = true AND u.is_active = true AND u.is_banned = false
		ORDER BY u.id`, batchID, models.AuditEntityUser).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("администраторы партии для отметки проходов: %w", err)
	}
	return ids, nil
}

// --- источники данных: кандидаты на отметку прохода ---

// passageCandidate -- то немногое о машине/сотруднике, что нужно отметке прохода:
// id для сервисного вызова, ОДНА таблица поста (details.table_id отметки -- пост
// физически один за раз, даже если запись видна в нескольких таблицах проходной) и
// момент принятия заявки в работу -- отправная точка окна историчности прохода.
type passageCandidate struct {
	ID         int       `gorm:"column:id"`
	TableID    int       `gorm:"column:table_id"`
	AcceptedAt time.Time `gorm:"column:accepted_at"`
	// EntryDateTo -- последний день, когда пропуск действует. Проход обязан попасть в
	// окно допуска: заявки разложены по прошлому на срок до года, а пропуск выдают на
	// неделю-другую, поэтому у большинства принятых заявок окно давно закрыто, и
	// сегодняшний въезд по такому пропуску охрана бы не пропустила.
	EntryDateTo time.Time `gorm:"column:entry_date_to"`
}

// passageWindowEnd -- до какого момента можно поставить проход: конец последнего дня
// действия пропуска, но не позже «сейчас».
func passageWindowEnd(c passageCandidate, now time.Time) time.Time {
	endOfDay := time.Date(c.EntryDateTo.Year(), c.EntryDateTo.Month(), c.EntryDateTo.Day(), 23, 59, 59, 0, c.EntryDateTo.Location())
	if endOfDay.Before(now) {
		return endOfDay
	}
	return now
}

// passableCandidates отбрасывает тех, чей пропуск закончился раньше, чем заявку взяли
// в работу: пройти они не могли в принципе, и отметка была бы выдумкой. Так бывает у
// старых заявок -- пропуск выдают на недели, а в работу заявку могли взять позже.
func passableCandidates(all []passageCandidate, now time.Time) []passageCandidate {
	out := make([]passageCandidate, 0, len(all))
	for _, c := range all {
		if passageWindowEnd(c, now).After(stageBase(c.AcceptedAt, now)) {
			out = append(out, c)
		}
	}
	return out
}

// passageWindowOpen -- действует ли пропуск ещё сегодня. Остаться на территории может
// только тот, у кого он не закончился: иначе на посту висел бы человек с просроченным
// пропуском, которого там быть не может.
func passageWindowOpen(c passageCandidate, now time.Time) bool {
	return !c.EntryDateTo.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
}

// loadPassageCarCandidates читает машины СТРОГО этой партии (через fake_batch_items
// на заявке, как loadBatchApplications в stages.go), которые реально видны на посту:
// status=1 (activateApplicationItems переводит в этот статус только принятые в
// работу и не возвращённые обратно) и хотя бы одна привязка car_target_tables.
// DISTINCT ON (c.id) с ORDER BY (c.id, ctt.id) детерминированно берёт САМУЮ первую
// привязку машины к посту -- тот же пост, куда её включили при принятии в работу.
func loadPassageCarCandidates(ctx context.Context, db *gorm.DB, batchID int) ([]passageCandidate, error) {
	var rows []passageCandidate
	err := db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (c.id) c.id AS id, ctt.table_id AS table_id, app.accepted_at AS accepted_at,
			att.entry_date_to::timestamp AS entry_date_to
		FROM cars c
		JOIN attachments att ON att.id = c.attachment_id
		JOIN applications app ON app.id = att.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = app.id
		JOIN car_target_tables ctt ON ctt.car_id = c.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND c.status = 1 AND app.accepted_at IS NOT NULL
		ORDER BY c.id, ctt.id`, batchID, models.AuditEntityApplication).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("машины партии для проходов: %w", err)
	}
	return rows, nil
}

// loadPassageEmployeeCandidates -- зеркало loadPassageCarCandidates для сотрудников
// (employees/employee_target_tables).
func loadPassageEmployeeCandidates(ctx context.Context, db *gorm.DB, batchID int) ([]passageCandidate, error) {
	var rows []passageCandidate
	err := db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (e.id) e.id AS id, ett.table_id AS table_id, app.accepted_at AS accepted_at,
			att.entry_date_to::timestamp AS entry_date_to
		FROM employees e
		JOIN attachments att ON att.id = e.attachment_id
		JOIN applications app ON app.id = att.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = app.id
		JOIN employee_target_tables ett ON ett.employee_id = e.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND e.status = 1 AND app.accepted_at IS NOT NULL
		ORDER BY e.id, ett.id`, batchID, models.AuditEntityApplication).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("сотрудники партии для проходов: %w", err)
	}
	return rows, nil
}

// --- распределение остался/выехал и потоки случайности ---

// passageStayExitCounts делит total кандидатов на "остался на территории" (только
// въезд) и "выехал" (въезд + выезд) count'ами, а не броском монеты на каждого --
// тот же приём и та же причина, что stageBucketSizes в stages.go: на маленьком
// профиле вероятностный бросок может не дать ни одного из двух состояний, и тест
// станет плавающим. Остаются на территории МЕНЬШИНСТВО (userShareCount(total,3),
// но не более total-1): за долгую историю прохода больше тех, кто уже выехал, чем
// тех, кто всё ещё внутри в момент "сейчас". При total<2 разделить оба состояния
// невозможно физически -- единственный кандидат уходит в "выехал".
func passageStayExitCounts(total int) (stay, exit int) {
	if total <= 0 {
		return 0, 0
	}
	stay = userShareCount(total, 3)
	if stay >= total {
		stay = total - 1
	}
	return stay, total - stay
}

// passageStreams -- независимые потоки случайности прохода, см. stageStreams в
// stages.go: каждый домен (машины/сотрудники) получает СВОЙ набор потоков, а не общий
// на оба -- иначе правка числа кандидатов-машин сдвинула бы RNG-последовательность,
// которую видят сотрудники, и повтор с тем же -seed перестал бы быть чистым для одного
// домена при правке другого (тот же принцип, что развёл userStreams/stageStreams).
type passageStreams struct {
	carActor    *Stream // какой администратор партии отмечает проход машины
	carEntryGap *Stream // момент въезда внутри окна [принятие в работу, сейчас]
	carExitGap  *Stream // момент выезда внутри окна [въезд, сейчас]
	empActor    *Stream // то же для сотрудников
	empEntryGap *Stream
	empExitGap  *Stream
}

func newPassageStreams(seed int64) *passageStreams {
	return &passageStreams{
		carActor:    NewStream(seed, "passage-car-actor"),
		carEntryGap: NewStream(seed, "passage-car-entry-gap"),
		carExitGap:  NewStream(seed, "passage-car-exit-gap"),
		empActor:    NewStream(seed, "passage-employee-actor"),
		empEntryGap: NewStream(seed, "passage-employee-entry-gap"),
		empExitGap:  NewStream(seed, "passage-employee-exit-gap"),
	}
}

// passageMoment -- следующий исторический момент строго после prev (когда окно это
// позволяет) и не позже hi.
//
// Дискретизация -- МИЛЛИСЕКУНДЫ, а не минуты, как у nextStageMoment (stages.go).
// Заявка могла быть принята в работу мгновения назад (узкое окно [accepted_at,
// сейчас]), а въезд и выезд -- два ПОСЛЕДОВАТЕЛЬНЫХ прохода внутри этого же окна:
// минутный шаг схлопнул бы оба в один и тот же now при окне короче минуты (обычное
// дело для свежепринятой заявки) -- ровно та ловушка прошлого среза ("выезд позже
// въезда" перестаёт быть правдой, если сторожить только "не раньше"). При delta>=1мс
// результат строго больше prev -- это и даёт "выезд СТРОГО позже въезда" в
// runCarPassages/runEmployeePassages, а не только "не раньше".
func passageMoment(s *Stream, prev, hi time.Time) time.Time {
	if !prev.Before(hi) {
		return hi
	}
	msLeft := int(hi.Sub(prev).Milliseconds())
	if msLeft < 1 {
		return hi
	}
	delta := time.Duration(IntRange(s, 1, msLeft)) * time.Millisecond
	next := prev.Add(delta)
	if next.After(hi) {
		return hi
	}
	return next
}

// --- сдвиг дат (сырой SQL -- то же исключение из "сервисный слой, а не SQL", что
// shift*-хелперы в stages.go): UpdateCarTerritoryStatus/UpdateEmployeeTerritoryStatus
// пишут NOW() реального времени, и без переноса проход заявки месячной давности
// выглядел бы совершённым только что ---

// shiftEntityAuditLog переносит запись истории конкретного действия (entry/exit) на
// исторический момент прохода. entity_id+action уникальны в рамках одной отметки --
// у каждой машины/сотрудника этого прогона максимум один "entry" и один "exit".
func shiftEntityAuditLog(ctx context.Context, db *gorm.DB, entityType string, entityID int, action string, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE audit_log SET created_at = ? WHERE entity_type = ? AND entity_id = ? AND action = ?`,
		at, entityType, entityID, action,
	).Error; err != nil {
		return fmt.Errorf("сдвиг истории (%s) %s %d: %w", action, entityType, entityID, err)
	}
	return nil
}

func shiftCarEntry(ctx context.Context, db *gorm.DB, carID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE cars SET territory_entry_time = ?, updated_at = ? WHERE id = ?`, at, at, carID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг времени въезда машины %d: %w", carID, err)
	}
	return nil
}

func shiftCarExit(ctx context.Context, db *gorm.DB, carID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE cars SET updated_at = ? WHERE id = ?`, at, carID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг времени выезда машины %d: %w", carID, err)
	}
	return nil
}

func shiftEmployeeEntry(ctx context.Context, db *gorm.DB, employeeID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE employees SET territory_entry_time = ?, updated_at = ? WHERE id = ?`, at, at, employeeID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг времени входа сотрудника %d: %w", employeeID, err)
	}
	return nil
}

func shiftEmployeeExit(ctx context.Context, db *gorm.DB, employeeID int, at time.Time) error {
	if err := db.WithContext(ctx).Exec(
		`UPDATE employees SET updated_at = ? WHERE id = ?`, at, employeeID,
	).Error; err != nil {
		return fmt.Errorf("сдвиг времени выхода сотрудника %d: %w", employeeID, err)
	}
	return nil
}

// --- сценарии: сервисный вызов + перенос дат сразу следом ---

// runCarPassages отмечает въезд каждой машины кандидата и выезд -- доле из них
// (passageStayExitCounts). Порядок кандидатов из SQL детерминирован (ORDER BY c.id),
// поэтому одно и то же -seed даёт один и тот же состав "остался"/"выехал".
func runCarPassages(ctx context.Context, svc services.CarService, db *gorm.DB, cars []passageCandidate, actors []int, s *passageStreams, now time.Time) (int, error) {
	cars = passableCandidates(cars, now)
	stay, _ := passageStayExitCounts(len(cars))
	for i, c := range cars {
		actorID := Pick(s.carActor, actors)
		base := stageBase(c.AcceptedAt, now)
		windowEnd := passageWindowEnd(c, now)
		entryAt := passageMoment(s.carEntryGap, base, windowEnd)

		req := services.UpdateCarTerritoryStatusRequest{
			UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{
				TerritoryStatus: 1, UserID: &actorID, TableID: &c.TableID,
			},
		}
		if err := svc.UpdateCarTerritoryStatus(ctx, c.ID, req); err != nil {
			return 0, fmt.Errorf("въезд машины %d: %w", c.ID, err)
		}
		if err := shiftCarEntry(ctx, db, c.ID, entryAt); err != nil {
			return 0, err
		}
		if err := shiftEntityAuditLog(ctx, db, models.AuditEntityCar, c.ID, "entry", entryAt); err != nil {
			return 0, err
		}

		if i < stay && passageWindowOpen(c, now) {
			continue // остаётся на территории -- выезда не будет
		}

		exitAt := passageMoment(s.carExitGap, entryAt, windowEnd)
		exitReq := services.UpdateCarTerritoryStatusRequest{
			UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{
				TerritoryStatus: 2, UserID: &actorID, TableID: &c.TableID,
			},
		}
		if err := svc.UpdateCarTerritoryStatus(ctx, c.ID, exitReq); err != nil {
			return 0, fmt.Errorf("выезд машины %d: %w", c.ID, err)
		}
		if err := shiftCarExit(ctx, db, c.ID, exitAt); err != nil {
			return 0, err
		}
		if err := shiftEntityAuditLog(ctx, db, models.AuditEntityCar, c.ID, "exit", exitAt); err != nil {
			return 0, err
		}
	}
	return len(cars), nil
}

// runEmployeePassages -- зеркало runCarPassages для сотрудников.
func runEmployeePassages(ctx context.Context, svc services.EmployeeService, db *gorm.DB, employees []passageCandidate, actors []int, s *passageStreams, now time.Time) (int, error) {
	employees = passableCandidates(employees, now)
	stay, _ := passageStayExitCounts(len(employees))
	for i, e := range employees {
		actorID := Pick(s.empActor, actors)
		base := stageBase(e.AcceptedAt, now)
		windowEnd := passageWindowEnd(e, now)
		entryAt := passageMoment(s.empEntryGap, base, windowEnd)

		req := services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &actorID, TableID: &e.TableID}
		if err := svc.UpdateEmployeeTerritoryStatus(ctx, e.ID, req); err != nil {
			return 0, fmt.Errorf("вход сотрудника %d: %w", e.ID, err)
		}
		if err := shiftEmployeeEntry(ctx, db, e.ID, entryAt); err != nil {
			return 0, err
		}
		if err := shiftEntityAuditLog(ctx, db, models.AuditEntityEmployee, e.ID, "entry", entryAt); err != nil {
			return 0, err
		}

		if i < stay && passageWindowOpen(e, now) {
			continue // остаётся на территории -- выхода не будет
		}

		exitAt := passageMoment(s.empExitGap, entryAt, windowEnd)
		exitReq := services.UpdateTerritoryStatusRequest{TerritoryStatus: 2, UserID: &actorID, TableID: &e.TableID}
		if err := svc.UpdateEmployeeTerritoryStatus(ctx, e.ID, exitReq); err != nil {
			return 0, fmt.Errorf("выход сотрудника %d: %w", e.ID, err)
		}
		if err := shiftEmployeeExit(ctx, db, e.ID, exitAt); err != nil {
			return 0, err
		}
		if err := shiftEntityAuditLog(ctx, db, models.AuditEntityEmployee, e.ID, "exit", exitAt); err != nil {
			return 0, err
		}
	}
	return len(employees), nil
}

// PassageStayExitCountsForTest открывает распределение остался/выехал проверке: сама
// функция приватная, а поведение на маленьком числе кандидатов стоит сторожить (см.
// StageBucketSizesForTest в stages.go).
func PassageStayExitCountsForTest(total int) (stay, exit int) {
	return passageStayExitCounts(total)
}

// PassageMomentForTest открывает passageMoment проверке -- гарантия "строго после prev"
// это ровно то место, где прошлый срез уже ошибся один раз (см. докстринг
// passageMoment), стоит сторожить отдельно от internal/handlers, без базы.
func PassageMomentForTest(s *Stream, prev, hi time.Time) time.Time {
	return passageMoment(s, prev, hi)
}
