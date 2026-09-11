package entityarchive

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
)

// Повторное применение уничтожений после восстановления из копии (#2357).
//
// Копия, снятая до уничтожения, возвращает всё как было - вместе с записями, которые
// оператор обязан был уничтожить. Закон допускает держать копии ограниченный срок при
// условии, что после восстановления удаления применяются заново; этот проход и есть
// исполнение условия.
//
// Он встроен в scripts/restore.sh отдельным шагом, а не оставлен командой, о которой
// надо помнить. Восстанавливают в спешке, и такой шаг пропускают первым - а пропустив,
// получают систему, где уничтоженные данные снова живут, и никто об этом не знает.
//
// Опора прохода - устойчивость идентификаторов: pg_restore возвращает строки с теми же
// id, что были, поэтому заявка и организация опознаются по идентификатору. У человека
// своего идентификатора нет, и он опознаётся по якорям обезличенных строк
// (subjectTargetFromAnchors).
//
// Записи журнала проход НЕ удаляет: следующая копия может оказаться ещё старше, и то же
// самое придётся снимать ещё раз. Растёт только счётчик повторов.

// Исходы повторного применения по одной записи журнала.
const (
	// ReplayGone - данные не вернулись (или уже сняты), делать нечего.
	ReplayGone = "gone"
	// ReplayApplied - данные вернулись из копии и сняты заново.
	ReplayApplied = "applied"
	// ReplayManual - вернулись, но автоматически повторить нельзя. Молча пропустить
	// такую запись нельзя: оператор решит, что проход отработал полностью.
	ReplayManual = "manual"
	// ReplayFailed - попытка снять не удалась.
	ReplayFailed = "failed"
)

// ReplayOutcome - что случилось с одной записью журнала.
type ReplayOutcome struct {
	Record models.DestructionRecord
	Status string
	// Detail - человеческое объяснение: что нашли и что сделали (или почему не смогли).
	Detail string
	Rows   int
	Files  int
}

// ReplayResult - итог прохода по журналу.
type ReplayResult struct {
	Checked  int
	Gone     int
	Applied  int
	Manual   int
	Failed   int
	Outcomes []ReplayOutcome
}

// ReplayDestructions проходит журнал уничтожения и снимает то, что вернулось из копии.
//
// Без opt.Apply - только показ: что вернулось и что было бы снято. Основание берётся из
// самой записи, а не задаётся заново: повторяется то же решение над теми же данными.
func ReplayDestructions(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, paths FilePaths, apply bool) (ReplayResult, error) {
	var records []models.DestructionRecord
	if err := db.WithContext(ctx).Order("id").Find(&records).Error; err != nil {
		return ReplayResult{}, fmt.Errorf("чтение журнала уничтожения: %w", err)
	}

	res := ReplayResult{Checked: len(records)}
	for _, rec := range records {
		out := replayOne(ctx, db, recorder, paths, rec, apply)
		switch out.Status {
		case ReplayGone:
			res.Gone++
		case ReplayApplied:
			res.Applied++
		case ReplayManual:
			res.Manual++
		case ReplayFailed:
			res.Failed++
		}
		// В отчёт идут только записи, потребовавшие внимания: на установке, где
		// уничтожений накопились тысячи, перечень «ничего не вернулось» никто не
		// прочитает, и в нём потеряется единственная строка, которая важна.
		if out.Status != ReplayGone {
			res.Outcomes = append(res.Outcomes, out)
		}
	}
	return res, nil
}

// replayOne разбирает одну запись журнала. Ошибку наружу не отдаёт: одна сбойная запись
// не должна останавливать проход - остальные снять всё равно надо, а о сбое сказано
// вслух отдельным исходом (тот же приём, что у обезличивания по сроку).
func replayOne(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, paths FilePaths,
	rec models.DestructionRecord, apply bool) ReplayOutcome {
	out := ReplayOutcome{Record: rec, Status: ReplayGone}

	opt := DestructionOptions{
		Files:    paths,
		ActorID:  rec.ActorID,
		Basis:    rec.Basis,
		ReplayOf: &rec.ID,
		Apply:    apply,
	}

	switch {
	case rec.EntityType == models.AuditEntityApplication && rec.Action == DestructionAnonymized:
		return replayApplicationAnonymize(ctx, db, recorder, rec, opt, out)

	case rec.EntityType == models.AuditEntityApplication && rec.Action == DestructionPurged:
		return replayApplicationPurge(ctx, db, recorder, rec, opt, out)

	case rec.EntityType == models.AuditEntityUniqueEmployee && rec.Action == DestructionAnonymized:
		return replaySubjectAnonymize(ctx, db, recorder, rec, opt, out)

	case rec.EntityType == models.AuditEntityOrganization && rec.Action == DestructionAnonymized:
		return replayOrganizationAnonymize(ctx, db, recorder, rec, opt, out)

	case rec.EntityType == models.AuditEntityOrganization && rec.Action == DestructionPurged:
		// Снос организации идёт только по проверенному пакету выгрузки - повторить его
		// автоматически нечем. Оператор обязан узнать об этом строкой в отчёте, а не
		// обнаружить вернувшуюся организацию через полгода.
		if rec.EntityID == nil {
			out.Status = ReplayManual
			out.Detail = "снос организации без идентификатора - проверьте вручную"
			return out
		}
		exists, err := orgExists(ctx, db, *rec.EntityID)
		if err != nil {
			return replayFailed(out, err)
		}
		if !exists {
			return out
		}
		out.Status = ReplayManual
		out.Detail = fmt.Sprintf("организация #%d вернулась из копии, снесите её заново по пакету: "+
			"entity retire -apply -> entity export -apply -> entity verify -> entity purge -apply", *rec.EntityID)
		return out

	default:
		// Запись сделана версией, которая знала вид уничтожения, неизвестный этой.
		// Промолчать нельзя: проход отчитался бы о полном применении, ничего не сделав.
		out.Status = ReplayManual
		out.Detail = fmt.Sprintf("вид записи неизвестен (%s/%s) - разберитесь вручную",
			rec.EntityType, rec.Action)
		return out
	}
}

func replayFailed(out ReplayOutcome, err error) ReplayOutcome {
	out.Status = ReplayFailed
	out.Detail = err.Error()
	return out
}

func replayApplicationAnonymize(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder,
	rec models.DestructionRecord, opt DestructionOptions, out ReplayOutcome) ReplayOutcome {
	if rec.EntityID == nil {
		out.Status = ReplayManual
		out.Detail = "обезличивание заявки без идентификатора - проверьте вручную"
		return out
	}
	id := *rec.EntityID

	// Не «заявка существует», а «у заявки снова живые имена»: восстановленная копия
	// могла быть снята уже ПОСЛЕ обезличивания, и тогда снимать нечего.
	live, err := applicationHasLivePeople(ctx, db, id)
	if err != nil {
		return replayFailed(out, err)
	}
	if !live {
		return out
	}

	out.Status = ReplayApplied
	out.Detail = fmt.Sprintf("заявка #%d вернулась с живыми участниками", id)
	if !opt.Apply {
		return out
	}
	applied, err := AnonymizeApplication(ctx, db, recorder, id, opt)
	if err != nil {
		return replayFailed(out, err)
	}
	out.Rows = applied.Total()
	out.Files = applied.Files.Total()
	return out
}

func replayApplicationPurge(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder,
	rec models.DestructionRecord, opt DestructionOptions, out ReplayOutcome) ReplayOutcome {
	if rec.EntityID == nil {
		out.Status = ReplayManual
		out.Detail = "уничтожение заявки без идентификатора - проверьте вручную"
		return out
	}
	id := *rec.EntityID

	exists, err := applicationExists(ctx, db, id)
	if err != nil {
		return replayFailed(out, err)
	}
	if !exists {
		return out
	}

	out.Status = ReplayApplied
	out.Detail = fmt.Sprintf("заявка #%d вернулась из копии целиком", id)
	if !opt.Apply {
		return out
	}
	applied, err := PurgeApplication(ctx, db, recorder, id, opt)
	if err != nil {
		return replayFailed(out, err)
	}
	out.Rows = int(applied.TotalRows())
	out.Files = applied.Files.Total()
	return out
}

func replaySubjectAnonymize(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder,
	rec models.DestructionRecord, opt DestructionOptions, out ReplayOutcome) ReplayOutcome {
	anchors := parseSubjectAnchors(rec.SubjectAnchors)
	if len(anchors) == 0 {
		out.Status = ReplayManual
		out.Detail = "обезличивание человека без якорей - найти его в восстановленной базе нечем"
		return out
	}

	target, err := subjectTargetFromAnchors(ctx, db, anchors)
	if err != nil {
		return replayFailed(out, err)
	}
	// Пустая цель - штатный исход, а не сбой: якорь вернулся уже обезличенным (копия
	// свежее уничтожения) либо не вернулся вовсе. Снимать в обоих случаях нечего.
	if target.Empty() {
		return out
	}

	out.Status = ReplayApplied
	out.Detail = "человек вернулся из копии, найден по якорю обезличенной строки"
	if !opt.Apply {
		return out
	}
	applied, err := AnonymizeSubject(ctx, db, recorder, target, opt)
	if err != nil {
		return replayFailed(out, err)
	}
	out.Rows = applied.Total()
	return out
}

func replayOrganizationAnonymize(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder,
	rec models.DestructionRecord, opt DestructionOptions, out ReplayOutcome) ReplayOutcome {
	if rec.EntityID == nil {
		out.Status = ReplayManual
		out.Detail = "обезличивание организации без идентификатора - проверьте вручную"
		return out
	}
	id := *rec.EntityID

	live, err := organizationHasLivePD(ctx, db, id)
	if err != nil {
		return replayFailed(out, err)
	}
	if !live {
		return out
	}

	out.Status = ReplayApplied
	out.Detail = fmt.Sprintf("организация #%d вернулась с живыми персональными данными", id)
	if !opt.Apply {
		return out
	}
	applied, err := Anonymize(ctx, db, recorder, TypeOrganization, id, opt)
	if err != nil {
		return replayFailed(out, err)
	}
	out.Rows = applied.Total()
	return out
}

// applicationHasLivePeople - у заявки снова есть участники с именем или документом.
func applicationHasLivePeople(ctx context.Context, db *gorm.DB, id int) (bool, error) {
	var live bool
	q := "SELECT " + fmt.Sprintf(applicationLivePeople, "@app")
	if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&live).Error; err != nil {
		return false, fmt.Errorf("проверка участников заявки %d: %w", id, err)
	}
	return live, nil
}

// organizationHasLivePD - у организации снова есть персональные данные: имена её
// пользователей, сотрудников её заявок или записей реестра. Проверяются те же три
// таблицы, что затирает обезличивание, - иначе проход либо промолчал бы о вернувшихся
// данных, либо гонял бы обезличивание по уже пустой организации при каждом прогоне.
func organizationHasLivePD(ctx context.Context, db *gorm.DB, id int) (bool, error) {
	q := `SELECT (
		EXISTS (SELECT 1 FROM users WHERE id IN (` + orgUsers + `) AND last_name IS NOT NULL)
		OR EXISTS (SELECT 1 FROM employees WHERE id IN (` + orgEmps + `) AND last_name IS NOT NULL)
		OR EXISTS (SELECT 1 FROM unique_employees WHERE organization_id = @org AND last_name IS NOT NULL)
	)`
	var live bool
	if err := db.WithContext(ctx).Raw(q, sql.Named("org", id)).Scan(&live).Error; err != nil {
		return false, fmt.Errorf("проверка персональных данных организации %d: %w", id, err)
	}
	return live, nil
}

// subjectTargetFromAnchors восстанавливает цель обезличивания по якорям из журнала.
//
// Ключ поиска берётся не из журнала, а из самой вернувшейся строки: она приезжает из
// копии вместе со своей свёрткой документа. Журналу поэтому не нужно хранить ни
// документ, ни свёртку, ни отпечаток свёртки - серия и номер паспорта это десять
// цифр, и любой их отпечаток перебирается за минуты, а журнал переживает и данные, и
// сами копии.
//
// Свёртки достаточно одного якоря: дальше AnonymizeSubject ищет по ней человека во
// всех трёх таблицах и снимет в том числе строки, которых в момент уничтожения уже не
// существовало, а из копии они вернулись. Якоря перебираются по порядку, потому что
// часть из них могла вернуться уже обезличенной или не вернуться вовсе.
func subjectTargetFromAnchors(ctx context.Context, db *gorm.DB, anchors []subjectAnchor) (SubjectTarget, error) {
	for _, a := range anchors {
		if !subjectAnchorTables[a.Table] {
			// Таблица из записи, сделанной другой версией. Пропускаем молча: это не
			// повод уронить проход, остальные якоря ещё не проверены.
			continue
		}
		var row struct {
			PassportSeriesNumberHMAC *string
			PatentNumberHMAC         *string
		}
		q := fmt.Sprintf(`SELECT passport_series_number_hmac, patent_number_hmac
			FROM %s WHERE id = ?`, a.Table)
		if err := db.WithContext(ctx).Raw(q, a.ID).Scan(&row).Error; err != nil {
			return SubjectTarget{}, fmt.Errorf("чтение якоря %s:%d: %w", a.Table, a.ID, err)
		}

		t := SubjectTarget{Origin: fmt.Sprintf("по якорю %s:%d из журнала уничтожения", a.Table, a.ID)}
		if row.PassportSeriesNumberHMAC != nil {
			t.PassportHMAC = *row.PassportSeriesNumberHMAC
		}
		if row.PatentNumberHMAC != nil {
			t.PatentHMAC = *row.PatentNumberHMAC
		}
		if !t.Empty() {
			return t, nil
		}
	}
	return SubjectTarget{}, nil
}

// subjectAnchorTables - таблицы, в которых якорь может лежать. Имя подставляется в
// запрос, поэтому берётся только из этого перечня, а не из записи журнала как есть.
var subjectAnchorTables = map[string]bool{
	"unique_employees":      true,
	"employees":             true,
	"application_employees": true,
}
