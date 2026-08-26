package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Разбор записей справочника, заведённых из заявки (#1437).
//
// Подача с незнакомым наименованием создаёт организацию или компанию со статусом
// «на проверке» (см. application_org_resolve.go), чтобы у заявки был живой
// organization_id. Принимающий разбирает такую запись тремя действиями:
//
//	approve - наименование верное, запись остаётся в справочнике;
//	rename  - наименование поправили (опечатка, сокращение); запись считается разобранной;
//	merge   - это дубль существующей записи: ссылки переезжают на неё, черновик удаляется.
//
// Организации и компании ведём одним кодом: расхождение между зеркальными сервисами
// тихое, а разбор для обеих сущностей одинаков.

// directoryRefColumn - место, которым что-то ссылается на запись справочника.
// PairColumn заполнен у таблиц-связок: там строка источника, дублирующая уже
// существующую у цели пару, при слиянии удаляется, а не переезжает - иначе у цели
// появятся две одинаковые привязки (уникального индекса на паре нет).
type directoryRefColumn struct {
	Table      string
	Column     string
	PairColumn string
}

// directoryModeration описывает разбираемый справочник.
type directoryModeration struct {
	table       string
	label       string
	auditEntity string
	// Действия аудита: подтверждение, переименование и слияние.
	actionApproved string
	actionRenamed  string
	actionMerged   string
	notFoundMsg    string
	emptyNameMsg   string
	refs           []directoryRefColumn
}

// organizationRefColumns - все места, ссылающиеся на организацию. Список сверяется с
// information_schema тестом TestDirectoryModeration: новая таблица с organization_id,
// не попавшая сюда, оставит после слияния осиротевшие строки.
var organizationRefColumns = []directoryRefColumn{
	{Table: "applications", Column: "organization_id"},
	{Table: "attachments", Column: "organization_id"},
	{Table: "unique_cars", Column: "organization_id"},
	{Table: "unique_employees", Column: "organization_id"},
	{Table: "users", Column: "organization_id"},
	{Table: "organization_users", Column: "organization_id", PairColumn: "user_id"},
	{Table: "organization_tables", Column: "organization_id", PairColumn: "table_id"},
	{Table: "organization_unload_places", Column: "organization_id", PairColumn: "unload_place_id"},
}

// companyRefColumns - зеркало organizationRefColumns для компаний.
var companyRefColumns = []directoryRefColumn{
	{Table: "applications", Column: "company_id"},
	{Table: "attachments", Column: "company_id"},
	{Table: "unique_cars", Column: "company_id"},
	{Table: "unique_employees", Column: "company_id"},
	{Table: "users", Column: "company_id"},
	{Table: "companies_users", Column: "company_id", PairColumn: "user_id"},
	{Table: "companies_tables", Column: "company_id", PairColumn: "table_id"},
	{Table: "companies_unload_places", Column: "company_id", PairColumn: "unload_place_id"},
}

// OrganizationRefTables и CompanyRefTables отдают перечень таблиц, ссылающихся на
// запись справочника. Экспортированы ради теста, который сверяет список с
// information_schema: новая таблица со ссылкой, не попавшая сюда, оставит после
// слияния осиротевшие строки, а FK не даст удалить черновик.
func OrganizationRefTables() []string { return refTables(organizationRefColumns) }

// CompanyRefTables - зеркало OrganizationRefTables для компаний.
func CompanyRefTables() []string { return refTables(companyRefColumns) }

func refTables(refs []directoryRefColumn) []string {
	tables := make([]string, 0, len(refs))
	for _, ref := range refs {
		tables = append(tables, ref.Table)
	}
	return tables
}

var organizationModeration = directoryModeration{
	table:          "organizations",
	label:          "Организация",
	auditEntity:    models.AuditEntityOrganization,
	actionApproved: models.OrganizationActionApproved,
	actionRenamed:  models.OrganizationActionRenamed,
	actionMerged:   models.OrganizationActionMerged,
	notFoundMsg:    "Организация не найдена или находится в архиве",
	emptyNameMsg:   "Укажите наименование организации",
	refs:           organizationRefColumns,
}

var companyModeration = directoryModeration{
	table:          "companies",
	label:          "Компания",
	auditEntity:    models.AuditEntityCompany,
	actionApproved: models.CompanyActionApproved,
	actionRenamed:  models.CompanyActionRenamed,
	actionMerged:   models.CompanyActionMerged,
	notFoundMsg:    "Компания не найдена или находится в архиве",
	emptyNameMsg:   "Укажите наименование компании",
	refs:           companyRefColumns,
}

// DirectoryRenameRequest - тело запроса на исправление наименования при разборе.
type DirectoryRenameRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// DirectoryMergeRequest - тело запроса на привязку черновика к существующей записи.
type DirectoryMergeRequest struct {
	TargetID int `json:"target_id" validate:"required,gte=1"`
}

// DirectoryEntry - запись справочника в ответе разбора.
//
// CreatedByUserID наружу не отдаём (json:"-"): это внутренняя сторона разбора - кому
// сообщить о его исходе. Заполняется только чтением черновика (loadPendingEntry), у
// целевых и конфликтующих записей остаётся nil.
type DirectoryEntry struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	ModerationStatus string `json:"moderation_status"`
	CreatedByUserID  *int   `json:"-"`
}

// DirectoryMergeResult - итог слияния: сколько ссылок переехало на целевую запись и
// сколько дублирующих привязок удалено. Числа идут во фронт и в тесты: «слияние прошло»
// без них означало бы лишь отсутствие ошибки.
type DirectoryMergeResult struct {
	Target     DirectoryEntry `json:"target"`
	Reassigned map[string]int `json:"reassigned"`
	DroppedDup map[string]int `json:"dropped_duplicates"`
}

// Исходы разбора: запись подтверждена, переименована либо столкнулась с уже
// существующим наименованием.
const (
	DirectoryModerationApproved = "approved"
	DirectoryModerationRenamed  = "renamed"
	DirectoryModerationConflict = "conflict"
)

// DirectoryModerationResult - результат подтверждения или переименования.
//
// Столкновение с существующим наименованием отдаётся не ошибкой, а исходом conflict с
// самой записью: для принимающего это не сбой, а развилка - привязать черновик к
// найденной записи (merge). Ошибкой фронт получил бы только текст: envelope при
// success=false несёт message и теряет данные, по которым предлагается привязка.
type DirectoryModerationResult struct {
	Status   string          `json:"status"`
	Entry    *DirectoryEntry `json:"entry,omitempty"`
	Existing *DirectoryEntry `json:"existing,omitempty"`
	Message  string          `json:"message,omitempty"`
}

// loadPendingEntry возвращает активную запись «на проверке». Разбирать уже проверенную
// запись нечего: подтверждение второй раз ничего не меняет, а переименование и слияние
// проверенной - это администрирование справочника, у него свои эндпоинты и своё право.
func loadPendingEntry(ctx context.Context, db *gorm.DB, def directoryModeration, id int) (DirectoryEntry, error) {
	var entry DirectoryEntry
	err := db.WithContext(ctx).
		Raw("SELECT id, name, moderation_status, created_by_user_id FROM "+def.table+" WHERE id = ? AND is_active = true", id).
		Scan(&entry).Error
	if err != nil {
		slog.Error("не удалось прочитать запись справочника", "table", def.table, "id", id, "error", err)
		return DirectoryEntry{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения справочника")
	}
	if entry.ID == 0 {
		return DirectoryEntry{}, echo.NewHTTPError(http.StatusNotFound, def.notFoundMsg)
	}
	if entry.ModerationStatus != models.ModerationPending {
		return DirectoryEntry{}, echo.NewHTTPError(http.StatusBadRequest, def.label+" уже проверена")
	}
	return entry, nil
}

// findKeyConflict ищет активную запись с тем же ключом дедупликации. Пустой ключ
// (наименование из одних кавычек) сверяем точной строкой, как applyNameDuplicateFilter.
//
// Ищутся записи в ЛЮБОМ статусе разбора, потому что ключ занимает и черновик: partial
// unique index по name_normalized (срез 9) не различает статусы, и переименование в
// наименование чужого черновика отбилось бы на уровне базы. Проверенная запись при этом
// предпочтительнее - к ней принимающий может привязать черновик, к другому черновику нет
// (см. conflictOutcome).
func findKeyConflict(ctx context.Context, db *gorm.DB, def directoryModeration, name string, excludeID int) (DirectoryEntry, error) {
	condition, arg := "name_normalized = ?", normalize.OrgName(name)
	if arg == "" {
		condition, arg = "name = ?", name
	}
	var existing DirectoryEntry
	query := fmt.Sprintf(
		"SELECT id, name, moderation_status FROM %s WHERE is_active = true AND id <> ? AND %s ORDER BY (moderation_status = ?) DESC, id ASC LIMIT 1",
		def.table, condition,
	)
	if err := db.WithContext(ctx).Raw(query, excludeID, arg, models.ModerationApproved).Scan(&existing).Error; err != nil {
		slog.Error("не удалось проверить конфликт наименований", "table", def.table, "error", err)
		return DirectoryEntry{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки справочника")
	}
	return existing, nil
}

// conflictOutcome переводит найденный конфликт в ответ разбора.
//
// Столкновение с ПРОВЕРЕННОЙ записью - развилка: принимающий привязывает черновик к ней
// (исход conflict несёт саму запись, по ней фронт предлагает слияние). Столкновение с
// другим ЧЕРНОВИКОМ склеить нечем - цель слияния обязана быть проверенной, иначе выйдет
// тупик «дубль есть, привязать некуда». Такой случай отдаётся ошибкой с объяснением
// порядка: сначала разбирается тот черновик, потом этот получает конфликт уже с ним.
func conflictOutcome(def directoryModeration, conflict DirectoryEntry) (DirectoryModerationResult, error) {
	if conflict.ModerationStatus != models.ModerationApproved {
		return DirectoryModerationResult{}, echo.NewHTTPError(http.StatusBadRequest,
			def.label+" с таким наименованием уже ждёт разбора - сначала разберите её")
	}
	return DirectoryModerationResult{
		Status:   DirectoryModerationConflict,
		Existing: &conflict,
		Message:  def.label + " с таким наименованием уже есть в справочнике",
	}, nil
}

// applicationRefColumn находит колонку, которой applications ссылается на
// разбираемый справочник (organization_id либо company_id), по общему списку
// refs (#1615, B1) - тот же источник, которым уже пользуется слияние, так что
// enqueue не может разъехаться с реальными связями при будущей правке refs.
func applicationRefColumn(def directoryModeration) string {
	for _, ref := range def.refs {
		if ref.Table == "applications" {
			return ref.Column
		}
	}
	return ""
}

// enqueueDirectoryArchiveExport ставит в очередь на пересборку файлового архива
// заявки, ссылающиеся на запись справочника id (#1615, B1): наименование
// организации/компании печатается в бланке и в слепке заявки, и разбор
// (подтверждение, переименование, слияние) может его поменять.
func enqueueDirectoryArchiveExport(ctx context.Context, db *gorm.DB, enqueuer BlankExportEnqueuer, def directoryModeration, id int) {
	if enqueuer == nil {
		return
	}
	column := applicationRefColumn(def)
	if column == "" {
		return
	}
	var appIDs []int
	if err := db.WithContext(ctx).Model(&models.Application{}).
		Where(column+" = ?", id).Pluck("id", &appIDs).Error; err != nil {
		slog.Warn("не удалось собрать заявки для пересборки архива после разбора справочника",
			"table", def.table, "id", id, "error", err)
		return
	}
	enqueuer.EnqueueApplications(appIDs, BlankExportReasonUpdate)
}

// approveDirectoryEntry подтверждает запись «на проверке».
func approveDirectoryEntry(ctx context.Context, db *gorm.DB, rec AuditRecorder, enqueuer BlankExportEnqueuer, def directoryModeration, id, actorID int) (DirectoryModerationResult, error) {
	entry, err := loadPendingEntry(ctx, db, def, id)
	if err != nil {
		return DirectoryModerationResult{}, err
	}

	// Запись с тем же ключом могла появиться, пока черновик ждал разбора: подтверждать его
	// нельзя, иначе в справочнике останутся два одинаковых наименования.
	conflict, err := findKeyConflict(ctx, db, def, entry.Name, id)
	if err != nil {
		return DirectoryModerationResult{}, err
	}
	if conflict.ID != 0 {
		return conflictOutcome(def, conflict)
	}

	// Подтверждение принимает наименование как верное, но не как попало набранное:
	// оформление приводим к канону здесь же (#1437). Иначе «ооо "братишк» оставалось бы в
	// справочнике навсегда - принимающему логично нажать «Добавить», а не «Исправить»,
	// ведь само наименование правильное. Ключ дедупликации канон не меняет, поэтому
	// name_normalized и проверка конфликта выше остаются в силе.
	display := normalize.OrgNameDisplay(entry.Name)

	actor := actorID
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"moderation_status": models.ModerationApproved}
		if display != entry.Name {
			updates["name"] = display
		}
		if err := tx.Table(def.table).Where("id = ? AND moderation_status = ?", id, models.ModerationPending).
			Updates(updates).Error; err != nil {
			return err
		}
		return rec.Record(ctx, tx, def.auditEntity, &id, def.actionApproved, &actor, map[string]any{"name": display})
	})
	if err != nil {
		// Подтверждение переписывает name, когда канонизирует легаси-черновик, а значит
		// может упереться в уникальный индекс так же, как переименование: отвечаем
		// конфликтом, а не пятисоткой.
		if isUniqueViolation(err) {
			conflict, conflictErr := findKeyConflict(ctx, db, def, entry.Name, id)
			if conflictErr != nil {
				return DirectoryModerationResult{}, conflictErr
			}
			if conflict.ID != 0 {
				slog.Info("наименование заняли во время подтверждения", "table", def.table, "id", id, "conflict", conflict.ID)
				return conflictOutcome(def, conflict)
			}
		}
		slog.Error("не удалось подтвердить запись справочника", "table", def.table, "id", id, "error", err)
		return DirectoryModerationResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка подтверждения записи")
	}
	slog.Info("запись справочника подтверждена", "table", def.table, "id", id, "actor", actorID)

	// Наименование в бланке/слепке заявок обязано следовать канону (#1615, B1) -
	// но только если оно реально сменилось: подтверждение легаси-черновика без
	// правки текста не повод трогать архив.
	if display != entry.Name {
		enqueueDirectoryArchiveExport(ctx, db, enqueuer, def, id)
	}

	entry.Name = display
	entry.ModerationStatus = models.ModerationApproved
	return DirectoryModerationResult{Status: DirectoryModerationApproved, Entry: &entry}, nil
}

// rename исправляет наименование записи «на проверке» и тем самым разбирает её:
// принимающий, поправивший опечатку, уже подтвердил запись, второе действие не нужно.
// notifier - уведомления инициатору наименования, может быть nil (см. notifyDirectoryResolved).
func renameDirectoryEntry(ctx context.Context, db *gorm.DB, rec AuditRecorder, notifier NotificationService, enqueuer BlankExportEnqueuer, def directoryModeration, id int, rawName string, actorID int) (DirectoryModerationResult, error) {
	entry, err := loadPendingEntry(ctx, db, def, id)
	if err != nil {
		return DirectoryModerationResult{}, err
	}

	// Исправленное принимающим наименование тоже канонизируем: он правит опечатку, а
	// оформление (ОПФ, заглавная, кавычки) держит система (#1437).
	name := normalize.OrgNameDisplay(rawName)
	if name == "" || normalize.OrgName(name) == "" || normalize.OrgNameMeaningless(name) {
		// Наименование без букв и цифр («"""», «---») ключа либо не даёт, либо даёт
		// бессмысленный: дедупликация по нему не работает, а в справочнике оно остаётся
		// мусором. Правило то же, что в подаче и в админском справочнике (#1437).
		return DirectoryModerationResult{}, echo.NewHTTPError(http.StatusBadRequest, def.emptyNameMsg)
	}

	conflict, err := findKeyConflict(ctx, db, def, name, id)
	if err != nil {
		return DirectoryModerationResult{}, err
	}
	if conflict.ID != 0 {
		return conflictOutcome(def, conflict)
	}

	normalized := normalize.OrgName(name)
	actor := actorID
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Условие по статусу в UPDATE - защита от второго разбора того же черновика,
		// прошедшего проверку конфликта одновременно с этим.
		// name_normalized пишем явно: BeforeSave до map-обновления не достаёт.
		if err := tx.Table(def.table).Where("id = ? AND moderation_status = ?", id, models.ModerationPending).Updates(map[string]any{
			"name":              name,
			"name_normalized":   normalized,
			"moderation_status": models.ModerationApproved,
		}).Error; err != nil {
			return err
		}
		// Контракт details тот же, что у переименования из админки (organization_service):
		// name - новое значение, from.name - старое. Модалка истории рисует «было -> стало»
		// именно по ним, свои ключи оставили бы запись без содержательной строки.
		return rec.Record(ctx, tx, def.auditEntity, &id, def.actionRenamed, &actor, map[string]any{
			"name":   name,
			"from":   map[string]any{"name": entry.Name},
			"source": "moderation",
		})
	})
	if err != nil {
		// Ключ мог занять кто-то между проверкой конфликта и записью: наименование
		// пришло параллельной подачей или другим разбором, и UPDATE отбил уникальный
		// индекс. Транзакция откатилась, поэтому перечитываем справочник и отвечаем как
		// при обычном конфликте, а не пятисоткой.
		if isUniqueViolation(err) {
			conflict, conflictErr := findKeyConflict(ctx, db, def, name, id)
			if conflictErr != nil {
				return DirectoryModerationResult{}, conflictErr
			}
			if conflict.ID != 0 {
				slog.Info("наименование заняли во время разбора", "table", def.table, "id", id, "conflict", conflict.ID)
				return conflictOutcome(def, conflict)
			}
		}
		slog.Error("не удалось переименовать запись справочника", "table", def.table, "id", id, "error", err)
		return DirectoryModerationResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка переименования записи")
	}
	slog.Info("запись справочника переименована при разборе", "table", def.table, "id", id, "actor", actorID)

	notifyDirectoryResolved(ctx, notifier, def.table, entry.CreatedByUserID, actorID,
		def.label+" уточнена",
		fmt.Sprintf("Указанное вами наименование «%s» исправлено на «%s».", entry.Name, name))
	enqueueDirectoryArchiveExport(ctx, db, enqueuer, def, id)

	renamed := DirectoryEntry{ID: id, Name: name, ModerationStatus: models.ModerationApproved}
	return DirectoryModerationResult{Status: DirectoryModerationRenamed, Entry: &renamed}, nil
}

// merge переносит ссылки черновика на существующую запись и удаляет сам черновик.
// Удаляем, а не архивируем: архив - это запись, которой пользовались, а черновик после
// слияния не значит ничего и только засоряет справочник дублем.
// notifier - уведомления инициатору наименования, может быть nil.
func mergeDirectoryEntry(ctx context.Context, db *gorm.DB, rec AuditRecorder, notifier NotificationService, enqueuer BlankExportEnqueuer, def directoryModeration, sourceID, targetID, actorID int) (DirectoryMergeResult, error) {
	if sourceID == targetID {
		return DirectoryMergeResult{}, echo.NewHTTPError(http.StatusBadRequest, "Нельзя привязать запись к самой себе")
	}
	source, err := loadPendingEntry(ctx, db, def, sourceID)
	if err != nil {
		return DirectoryMergeResult{}, err
	}

	var target DirectoryEntry
	if err := db.WithContext(ctx).
		Raw("SELECT id, name, moderation_status FROM "+def.table+" WHERE id = ? AND is_active = true", targetID).
		Scan(&target).Error; err != nil {
		slog.Error("не удалось прочитать целевую запись справочника", "table", def.table, "id", targetID, "error", err)
		return DirectoryMergeResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения справочника")
	}
	if target.ID == 0 {
		return DirectoryMergeResult{}, echo.NewHTTPError(http.StatusNotFound, def.notFoundMsg)
	}
	if target.ModerationStatus != models.ModerationApproved {
		// Привязка к другому черновику оставила бы обе записи неразобранными.
		return DirectoryMergeResult{}, echo.NewHTTPError(http.StatusBadRequest, "Привязывать можно только к проверенной записи справочника")
	}

	// Заявки черновика собираем ДО слияния (#1615, B1): после переноса ссылок
	// organization_id/company_id этих заявок уже указывает на targetID, и искать
	// их по sourceID было бы поздно.
	var affectedAppIDs []int
	if enqueuer != nil {
		if column := applicationRefColumn(def); column != "" {
			if err := db.WithContext(ctx).Model(&models.Application{}).
				Where(column+" = ?", sourceID).Pluck("id", &affectedAppIDs).Error; err != nil {
				slog.Warn("не удалось собрать заявки для пересборки архива перед слиянием справочника",
					"table", def.table, "source", sourceID, "error", err)
			}
		}
	}

	reassigned := map[string]int{}
	dropped := map[string]int{}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, ref := range def.refs {
			if ref.PairColumn != "" {
				// Строки связки, для которых у цели уже есть такая же пара, переезжать
				// не должны: уникального индекса на паре нет, и цель получила бы дубль.
				dup := tx.Exec(fmt.Sprintf(
					`DELETE FROM %[1]s s WHERE s.%[2]s = ? AND EXISTS (
						SELECT 1 FROM %[1]s t WHERE t.%[2]s = ? AND t.%[3]s = s.%[3]s)`,
					ref.Table, ref.Column, ref.PairColumn,
				), sourceID, targetID)
				if dup.Error != nil {
					return fmt.Errorf("удаление дублей %s: %w", ref.Table, dup.Error)
				}
				if dup.RowsAffected > 0 {
					dropped[ref.Table] = int(dup.RowsAffected)
				}
			}
			moved := tx.Exec(fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", ref.Table, ref.Column, ref.Column), targetID, sourceID)
			if moved.Error != nil {
				return fmt.Errorf("перепривязка %s: %w", ref.Table, moved.Error)
			}
			if moved.RowsAffected > 0 {
				reassigned[ref.Table] = int(moved.RowsAffected)
			}
		}

		if err := tx.Exec("DELETE FROM "+def.table+" WHERE id = ?", sourceID).Error; err != nil {
			return fmt.Errorf("удаление записи %s: %w", def.table, err)
		}

		actor := actorID
		// Запись аудита вешаем на целевую запись: черновика после слияния нет, и
		// история слияния должна читаться там, где теперь живут ссылки.
		return rec.Record(ctx, tx, def.auditEntity, &targetID, def.actionMerged, &actor, map[string]any{
			"merged_id":   sourceID,
			"merged_name": source.Name,
			"reassigned":  reassigned,
		})
	})
	if err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			return DirectoryMergeResult{}, httpErr
		}
		slog.Error("не удалось привязать запись справочника", "table", def.table, "source", sourceID, "target", targetID, "error", err)
		return DirectoryMergeResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка привязки записи")
	}
	slog.Info("запись справочника привязана к существующей", "table", def.table, "source", sourceID, "target", targetID, "actor", actorID)

	notifyDirectoryResolved(ctx, notifier, def.table, source.CreatedByUserID, actorID,
		def.label+" привязана к справочнику",
		fmt.Sprintf("Указанное вами наименование «%s» привязано к записи справочника «%s».", source.Name, target.Name))
	if enqueuer != nil {
		enqueuer.EnqueueApplications(affectedAppIDs, BlankExportReasonUpdate)
	}

	return DirectoryMergeResult{Target: target, Reassigned: reassigned, DroppedDup: dropped}, nil
}
