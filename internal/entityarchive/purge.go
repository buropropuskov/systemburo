package entityarchive

// Срез 7: физический снос данных организации из живой системы по проверенному пакету
// (server entity purge). В отличие от retire (обратимое погашение is_active) - необратим:
// строки графа удаляются физически, файлы заявок - с диска.
//
// Инварианты, каждый обязателен и проверен тестами в internal/handlers/entity_purge_test.go:
//
//  1. Снос идёт только по пакету, прошедшему Verify С УКАЗАНИЕМ -type/-id цели - переиспользует
//     тот же гейт, на котором уже держится import (checkIdentity внутри Verify отсекает пакет
//     другой сущности).
//  2. Снос запрещён по незашифрованному пакету: опись открытого пакета сверяется сама с
//     собой, и любой, у кого есть доступ к каталогу, может вырезать из неё запись вместе с
//     файлом - проверка целостности этого не заметит. Отказ явный, без флага-обхода (в
//     отличие от export -plaintext, здесь такого флага нет намеренно).
//  3. Пакет обязан ПОКРЫВАТЬ текущее состояние графа: счётчики строк по каждой таблице описи
//     сверяются с фактическими счётчиками (Collect) и до старта транзакции, и повторно ВНУТРИ
//     неё. Любое расхождение - отказ, а не предупреждение: копия устарела, и снос уничтожил бы
//     то, чего в пакете нет (либо, в другую сторону, описывал бы состояние, которого уже нет).
//  4. Удаление и запись в audit_log идут в ОДНОЙ транзакции: строки без следа в журнале не
//     считаются удалёнными. Details не несёт персональных данных - только путь пакета,
//     отпечаток манифеста, имена таблиц и счётчики.
//  5. audit_log не входит в граф организации (в organizationNodes() узла для него нет) и не
//     имеет внешнего ключа на entity_id (см. комментарий модели) - запись о сносе переживает
//     сам снос той же гарантией, какой model.AuditLog переживает удаление любой сущности.
//  6. Файлы заявок снимаются с диска ПОСЛЕ того, как строки удалены и транзакция
//     зафиксирована - тот же порядок, что у Import, только в обратную сторону (там файл
//     кладётся до строк: осиротевший файл безопаснее строки без файла).
//  7. Порядок удаления - organizationNodes() КАК ЕСТЬ (дети раньше родителей). Import вставляет
//     в ОБРАТНОМ порядке (родители раньше детей) ровно потому, что вставка и удаление зеркальны;
//     удалению разворот не нужен - порядок карты графа для него уже правильный.
//  8. Общие (is_shared) шаблоны отчётов сносимых пользователей отвязываются от владельца
//     (owner_user_id -> NULL) ВНУТРИ той же транзакции, ДО удаления узлов графа - иначе каскад
//     OwnerUserID (OnDelete:CASCADE) унёс бы шаблон, которым пользуются чужие пользователи, не
//     подававшие заявку на снос. Личные (не is_shared) шаблоны отвязку не проходят и уходят
//     штатным каскадом вместе с автором - см. detachSharedReportTemplates.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PurgeOptions - параметры сноса.
type PurgeOptions struct {
	// UploadPath - тот же корень загрузок, что у export/import: под ним лежат файлы заявок,
	// которые снос удалит с диска после успешного удаления строк.
	UploadPath string
	// Decrypt открывает файлы ПАКЕТА - тот же Decryptor, что у Verify/Import. Пакет обязан
	// быть зашифрован (инвариант 2), поэтому nil здесь на реальном пакете даёт закономерный
	// отказ уже на шаге Verify, а не молчаливый разбор конверта как открытого текста.
	Decrypt Decryptor
	// Recorder пишет audit_log о сносе. Обязателен при Apply=true - снос без следа в журнале
	// запрещён тем же приёмом, что и у export/import.
	Recorder AuditRecorder
	// ActorID - кто инициировал снос. Для консольного доступа обычно nil: доступ к консоли
	// сервера уже равнозначен доступу оператора, как и у retire/restore.
	ActorID *int
	// Apply - удалить по-настоящему. Без него команда только проверяет пакет, сверяет
	// покрытие текущего состояния и считает, что удалилось бы - не трогая ни базу, ни диск.
	Apply bool
}

// PurgeTableCount - таблица графа и число строк, реально удалённых (apply) или подлежащих
// удалению (dry-run). Отдельный от graph.TableCount тип: этот же список уходит в
// audit_log.Details, а деталям там нужны snake_case json-теги, как у соседних (exportAuditDetails
// и т.п.), которых у TableCount нет - он используется только для консольной печати.
type PurgeTableCount struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

// PurgeResult - что удалил (или удалил бы при пробном прогоне) снос.
type PurgeResult struct {
	Type    string
	ID      int
	Package string
	// ManifestSHA256 - отпечаток ОТКРЫТОГО содержимого манифеста, тем же приёмом, что у
	// ExportResult.ManifestSHA256: единственный внешний якорь, по которому позже можно
	// сверить, каким именно пакетом был выполнен снос.
	ManifestSHA256 string
	Apply          bool
	Tables         []PurgeTableCount
	Files          int
	// DetachedReportTemplates - число общих шаблонов отчётов (report_templates, is_shared),
	// СНЯТЫХ с владельца перед сносом (owner_user_id -> NULL), а не удалённых вместе с ним -
	// см. detachSharedReportTemplates. Отдельное число, не входит в Tables/TotalRows: строка
	// физически осталась в базе, удалением её считать нельзя.
	DetachedReportTemplates int64
	// Warnings - то, что оператор обязан увидеть, но что не мешает сносу (тот же приём,
	// что у AnonymizeResult.Warnings): общие шаблоны отчётов, чьи авторы входят в снос -
	// перед удалением строк они отвязываются от владельца (DetachedReportTemplates) и
	// остаются доступны всем, кто ими пользуется, но менять/удалять их через API станет
	// некому - loadOwnedTemplate не признаёт своей строку с NULL-владельцем ни для кого.
	Warnings []string
}

// TotalRows - сколько строк удалено (или удалилось бы) во всём графе.
func (r PurgeResult) TotalRows() int64 {
	var n int64
	for _, t := range r.Tables {
		n += t.Rows
	}
	return n
}

// purgeAuditDetails - подробности записи audit_log о сносе. Тип и id цели уже несёт сама
// строка audit_log (entity_type/entity_id), время - её created_at; здесь только то, чего
// в колонках нет. Персональных данных нет: путь пакета - служебное имя каталога
// (<тип>-<id>-<время>), отпечаток - хэш, имена таблиц и счётчики - не значения строк.
type purgeAuditDetails struct {
	Package        string            `json:"package"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	Tables         []PurgeTableCount `json:"tables"`
	Rows           int64             `json:"rows"`
	Files          int               `json:"files"`
	// DetachedReportTemplates - см. PurgeResult.DetachedReportTemplates: отдельное от Rows
	// число, журнал обязан отличать снос от отвязки так же, как и результат команды.
	DetachedReportTemplates int64 `json:"detached_report_templates,omitempty"`
}

// Purge проверяет пакет в dir (Verify с указанием entityType/id - инвариант 1), сверяет его
// покрытие текущего состояния графа (инвариант 3) и физически удаляет граф цели вместе с
// файлами её заявок (инварианты 6-7). Без Apply - только проверка и подсчёт.
func Purge(ctx context.Context, db *gorm.DB, entityType string, id int, dir string, opt PurgeOptions) (PurgeResult, error) {
	if entityType != TypeOrganization {
		return PurgeResult{}, fmt.Errorf("тип %q не поддерживается (v1: только %s)", entityType, TypeOrganization)
	}
	res := PurgeResult{Type: entityType, ID: id, Package: dir, Apply: opt.Apply}

	v, err := Verify(ctx, db, dir, opt.Decrypt, entityType, id)
	if err != nil {
		return res, fmt.Errorf("проверка пакета: %w", err)
	}
	if !v.OK {
		return res, fmt.Errorf("пакет не прошёл проверку (%d ошибок) - снос не начинается: %s",
			len(v.Problems), strings.Join(v.Problems, "; "))
	}
	// Инвариант 2: открытый пакет сверяется сам с собой, подмену такая сверка не увидит.
	// Флага-обхода здесь нет намеренно, в отличие от export -plaintext.
	//
	// v.ManifestEncrypted, а НЕ v.Manifest.Encrypted - последнее разобрано из ТЕЛА
	// манифеста (заявление пакета о самом себе, подделывается вместе с остальным открытым
	// текстом), тогда как ManifestEncrypted - факт того, каким файлом манифест РЕАЛЬНО лежал
	// на диске (см. комментарий поля в verify.go). Verify дополнительно валит OK при
	// расхождении между ними, но гейт здесь не должен зависеть от того, что эта защита не
	// исчезнет при будущей правке - берём факт напрямую.
	if !v.ManifestEncrypted {
		return res, errors.New("пакет записан открытым текстом - опись открытого пакета сверяется сама " +
			"с собой, и любой, у кого есть доступ к каталогу, может вырезать из неё запись вместе с " +
			"файлом незамеченно; снос по такому пакету запрещён без исключений")
	}

	fingerprint, err := manifestFingerprint(dir, opt.Decrypt)
	if err != nil {
		return res, fmt.Errorf("отпечаток манифеста: %w", err)
	}
	res.ManifestSHA256 = fingerprint

	graph, err := Collect(ctx, db, entityType, id)
	if err != nil {
		return res, err
	}
	if diff := comparePackageCoverage(v.Manifest.Tables, graph.Tables); diff != "" {
		return res, coverageMismatchError(diff)
	}

	files, err := applicationFileRows(ctx, db, id)
	if err != nil {
		return res, err
	}
	res.Files = len(files)

	// Считается ДО удаления и попадает в результат независимо от Apply - оператор обязан
	// увидеть предупреждение в пробном прогоне, раньше, чем нажмёт -apply, а не узнать о
	// нём постфактум из уже необратимого сноса.
	warning, err := sharedReportTemplateWarning(ctx, db, id)
	if err != nil {
		return res, err
	}
	if warning != "" {
		res.Warnings = append(res.Warnings, warning)
	}

	if !opt.Apply {
		res.Tables = toPurgeTableCounts(graph.Tables)
		return res, nil
	}
	if opt.Recorder == nil {
		return res, errors.New("не задан журнал аудита (Recorder) - снос без следа в audit_log запрещён")
	}

	var deleted []PurgeTableCount
	var detachedTemplates int64
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Повторная сверка покрытия ВНУТРИ транзакции: закрывает окно между проверкой выше
		// и самим удалением - на практике verify и purge -apply запускает один оператор
		// отдельными командами, между ними может пройти сколько угодно времени. Остаточный
		// риск (данные, изменившиеся между этим SELECT и DELETE ниже уже внутри одной
		// транзакции READ COMMITTED) тот же, с которым здесь живёт retire.go - усиливать
		// изоляцию отдельной задачей, если понадобится.
		//
		// Сверка обязана идти РАНЬШЕ отвязки шаблонов ниже: она сравнивает счётчики строк
		// с пакетом, а пакет снят ДО отвязки - на нём report_templates ещё несёт исходные
		// owner_user_id. Отвязка не меняет число строк report_templates (только колонку),
		// поэтому сама по себе сверку не портит - но идти она обязана после, а не до, иначе
		// один и тот же Purge отказывал бы себе сам при повторном перечитывании инварианта.
		curGraph, err := Collect(ctx, tx, entityType, id)
		if err != nil {
			return err
		}
		if diff := comparePackageCoverage(v.Manifest.Tables, curGraph.Tables); diff != "" {
			return coverageMismatchError(diff)
		}

		// Инвариант 8: отвязка общих шаблонов ДО удаления узлов графа - иначе deleteNode
		// ниже унёс бы их тем же каскадом, что и личные.
		detachedTemplates, err = detachSharedReportTemplates(ctx, tx, id)
		if err != nil {
			return err
		}

		for _, node := range organizationNodes() {
			n, err := deleteNode(ctx, tx, node, id)
			if err != nil {
				return err
			}
			if n > 0 {
				deleted = append(deleted, PurgeTableCount{Table: node.Table, Rows: n})
			}
		}

		entityID := id
		details := purgeAuditDetails{
			Package: dir, ManifestSHA256: fingerprint, Tables: deleted, Rows: sumRows(deleted), Files: len(files),
			DetachedReportTemplates: detachedTemplates,
		}
		return opt.Recorder.Record(ctx, tx, entityType, &entityID, models.OrganizationActionPurged, opt.ActorID, details)
	})
	if txErr != nil {
		return res, fmt.Errorf("снос %s #%d: %w", entityType, id, txErr)
	}
	res.Tables = deleted
	res.DetachedReportTemplates = detachedTemplates

	// Инвариант 6: файлы уходят с диска ТОЛЬКО после того, как транзакция с удалением строк
	// и записью в audit_log зафиксирована. Сбой здесь не откатывает уже сделанное удаление -
	// строки снесены и в журнале, оператор обязан узнать про недоснесённые файлы и убрать их
	// сам, а не считать снос неудавшимся целиком.
	if err := removeApplicationFiles(opt.UploadPath, files); err != nil {
		return res, fmt.Errorf("строки удалены и зафиксированы в audit_log, но не все файлы заявок "+
			"снесены с диска - уберите вручную: %w", err)
	}
	return res, nil
}

// deleteNode физически удаляет строки одного узла графа и возвращает число удалённых строк.
// RowsAffected, а не DELETE ... RETURNING id: часть узлов графа - чистые join-таблицы без
// своего serial id (см. комментарий fixSequence в import.go про "таблицы без serial/identity
// id") - RETURNING id уронил бы удаление именно на них.
func deleteNode(ctx context.Context, tx *gorm.DB, node Node, id int) (int64, error) {
	q := "DELETE FROM " + node.Table + " WHERE " + node.Where
	result := tx.WithContext(ctx).Exec(q, sql.Named("org", id))
	if result.Error != nil {
		return 0, fmt.Errorf("удаление %s: %w", node.Table, result.Error)
	}
	return result.RowsAffected, nil
}

// coverageMismatchError формирует единый текст отказа сверки покрытия - его использует и
// проверка до транзакции, и повторная внутри неё (инвариант один, сообщение должно быть
// одинаковым независимо от того, какая из двух точек его дала).
//
// У активной организации отказ будет срабатывать часто и по мелочам: заявку открыли - в
// графе появилась отметка прочтения, до -apply дело ещё не дошло, а покрытие уже не то.
// Оператору тогда важнее не диагноз (он и так печатается), а что делать - поэтому отказ сам
// называет рабочий порядок, а не оставляет искать обход проверки (обходить её нельзя).
func coverageMismatchError(diff string) error {
	return fmt.Errorf("пакет не покрывает текущее состояние - данные менялись после снятия копии, "+
		"снос уничтожил бы то, чего в пакете нет (или пакет описывает то, чего уже нет): %s. "+
		"У активной организации это ожидаемо - её данные меняются даже при обычном использовании. "+
		"Порядок для необратимого сноса: entity retire -apply (гасит организацию и её пользователей, "+
		"после этого её данные больше не меняются) -> entity export -apply по уже погашенной "+
		"организации -> entity verify по свежему пакету -> entity purge -apply по нему же", diff)
}

// comparePackageCoverage сравнивает счётчики строк в описи пакета с текущим состоянием графа.
// Расхождение в ЛЮБУЮ сторону - и когда сейчас БОЛЬШЕ, чем нёс пакет (данные добавили после
// снятия копии - снос уничтожил бы то, чего в пакете нет), и когда МЕНЬШЕ (пакет описывает
// состояние, которого уже нет) - означает, что копия устарела. Пустая строка - полное совпадение.
func comparePackageCoverage(pkgTables []TableFile, nowTables []TableCount) string {
	pkg := make(map[string]int64, len(pkgTables))
	for _, t := range pkgTables {
		pkg[t.Table] = t.Rows
	}
	now := make(map[string]int64, len(nowTables))
	for _, t := range nowTables {
		now[t.Table] = t.Rows
	}

	all := make(map[string]bool, len(pkg)+len(now))
	for t := range pkg {
		all[t] = true
	}
	for t := range now {
		all[t] = true
	}
	names := make([]string, 0, len(all))
	for t := range all {
		names = append(names, t)
	}
	sort.Strings(names)

	var diffs []string
	for _, t := range names {
		if pkg[t] != now[t] {
			diffs = append(diffs, fmt.Sprintf("%s: в пакете %d, сейчас %d", t, pkg[t], now[t]))
		}
	}
	return strings.Join(diffs, "; ")
}

// manifestFingerprint читает исходное (открытое) содержимое манифеста пакета и хэширует его -
// то же самое содержимое, что видел Verify внутри readManifest, только здесь нужны САМИ байты,
// а не разобранная структура: повторная сериализация уже распарсенного Manifest могла бы
// разойтись с оригиналом по мелочам JSON-форматирования, а отпечаток в audit_log обязан
// описывать РЕАЛЬНЫЙ файл на диске. Двойное присутствие манифеста здесь уже невозможно - к
// этому месту доходят только после успешного Verify (v.OK), а он такой пакет отверг бы раньше.
func manifestFingerprint(dir string, dec Decryptor) (string, error) {
	for _, candidate := range manifestCandidates {
		path := filepath.Join(dir, candidate)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		rc, err := openPackageFile(path, dec)
		if err != nil {
			return "", fmt.Errorf("манифест %s: %w", candidate, err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("манифест %s: чтение: %w", candidate, err)
		}
		sum := sha256.Sum256(body)
		return hex.EncodeToString(sum[:]), nil
	}
	return "", errors.New("манифест не найден")
}

// removeApplicationFiles снимает с диска файлы заявок, унесённые пакетом. Файл, которого уже
// нет (ErrNotExist) - не ошибка: цель ("на диске файла нет") уже достигнута, снос не обязан
// был убрать его сам. Любая другая ошибка (права, занятость) обязана вернуться наверх - не
// глушить, оператор должен узнать, что именно осталось убрать вручную.
func removeApplicationFiles(uploadPath string, files []appFileRow) error {
	var failed []string
	for _, f := range files {
		if f.StoredName == "" {
			continue
		}
		path := filepath.Join(uploadPath, applicationFilesDir, f.StoredName)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failed = append(failed, fmt.Sprintf("%s (заявка %d): %v", f.FileName, f.ID, err))
		}
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "; "))
	}
	return nil
}

// sharedReportTemplateWarning предупреждает про общие шаблоны отчётов (report_templates,
// is_shared), чьи авторы входят в снос: is_shared делает шаблон видимым ВСЕМ, кто им
// пользуется, не только автору, и оператор обязан узнать об этом до -apply. Само по себе
// это сносу больше не мешает (см. detachSharedReportTemplates) - шаблон переживает снос,
// но становится ничьим: loadOwnedTemplate не отдаст его в правку/удаление никому, ни
// прежнему кругу пользователей, ни оператору. Предикат берётся из
// nodeWhere("report_templates") - тот же, что и у самой отвязки, а не переписывается заново.
func sharedReportTemplateWarning(ctx context.Context, exec *gorm.DB, id int) (string, error) {
	where, err := nodeWhere("report_templates")
	if err != nil {
		return "", err
	}
	var count int64
	q := "SELECT COUNT(*) FROM report_templates WHERE " + where + " AND is_shared = true"
	if err := exec.WithContext(ctx).Raw(q, sql.Named("org", id)).Scan(&count).Error; err != nil {
		return "", fmt.Errorf("подсчёт общих шаблонов отчётов: %w", err)
	}
	if count == 0 {
		return "", nil
	}
	return fmt.Sprintf("среди сносимых пользователей есть авторы общих шаблонов отчётов (report_templates, "+
		"is_shared, сейчас %d шт.) - перед удалением строк они будут отвязаны от владельца (owner_user_id "+
		"-> NULL) и останутся доступны всем, кто ими пользуется, но менять или удалять их станет некому",
		count), nil
}

// detachSharedReportTemplates снимает владение с общих шаблонов отчётов сносимых
// пользователей ДО удаления узлов графа (инвариант 8) - иначе каскад OwnerUserID
// (OnDelete:CASCADE) унёс бы вместе с автором и шаблон, которым пользуются чужие
// пользователи, не подававшие заявку на снос своей организации. Личные (не is_shared)
// шаблоны отвязку не проходят - предикат ниже требует is_shared = true, а без него строка
// дойдёт до deleteNode("report_templates") и уйдёт штатным каскадом вместе с автором,
// ровно как и ожидается: личный шаблон умирает вместе с владельцем.
//
// owner_user_id обнуляется, а не переносится на другого пользователя (например, на
// супер-администратора): поле уже допускает NULL - тем же состоянием живут системные
// пресеты (models.ReportTemplate: "Системные пресеты... имеют OwnerUserID=nil"), и
// ListReportTemplates продолжает отдавать шаблон всем через ветку "is_shared = true"
// независимо от владельца - видимость и применение шаблона (то, ради чего его вообще
// расшаривали) не теряются. Перенос на конкретного администратора добавил бы вопрос без
// надёжного ответа: узел "users" ниже удаляет ВСЕХ пользователей организации без
// исключения (organization_id = @org, включая супер-администраторов ЭТОЙ ЖЕ организации),
// так что кандидат в новые владельцы сам может оказаться удалён той же транзакцией -
// пришлось бы отдельно искать супер-администратора вне сносимой организации, ловить
// случай, когда такого нет, и решать вопрос чужого владения ресурсом, который человек не
// создавал. Обратная сторона NULL - строку после этого не сможет поправить или убрать уже
// никто через API (loadOwnedTemplate трактует NULL-владельца как чужого для любого
// вызывающего), но это тот же компромисс, на котором уже стоят системные пресеты, и он
// назван в предупреждении (sharedReportTemplateWarning) до -apply, а не всплывает молча.
func detachSharedReportTemplates(ctx context.Context, tx *gorm.DB, id int) (int64, error) {
	where, err := nodeWhere("report_templates")
	if err != nil {
		return 0, err
	}
	q := "UPDATE report_templates SET owner_user_id = NULL WHERE " + where + " AND is_shared = true"
	result := tx.WithContext(ctx).Exec(q, sql.Named("org", id))
	if result.Error != nil {
		return 0, fmt.Errorf("отвязка общих шаблонов отчётов: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func toPurgeTableCounts(in []TableCount) []PurgeTableCount {
	out := make([]PurgeTableCount, len(in))
	for i, t := range in {
		out[i] = PurgeTableCount{Table: t.Table, Rows: t.Rows}
	}
	return out
}

func sumRows(in []PurgeTableCount) int64 {
	var n int64
	for _, t := range in {
		n += t.Rows
	}
	return n
}
