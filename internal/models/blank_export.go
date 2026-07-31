package models

import "time"

// BlankExport - строка реестра файлового архива (#1615): одновременно очередь на
// выгрузку бланка и указатель «какой файл где лежит». Одна строка на пару
// (application_id, attachment_id) - у заявки несколько вложений, у каждого свой бланк.
//
// Без FK на applications/attachments - философия audit_log и daily_pass_reports:
// каскад снёс бы строку при удалении вложения и оставил файл-сироту, про которого
// система забыла. Осиротевшие строки помечаются статусом, а не удаляются молча.
//
// RelDir/FileName хранят ФАКТИЧЕСКОЕ положение файла. Желаемое считается шаблоном на
// каждом прогоне, и расхождение между ними и есть команда переименовать. Обратный
// порядок (сначала БД, потом диск) оставил бы каталог-сироту при падении между шагами.
type BlankExport struct {
	ID int `gorm:"primaryKey" json:"id"`

	ApplicationID int `gorm:"not null;uniqueIndex:idx_blank_exports_pair,priority:1" json:"application_id"`
	AttachmentID  int `gorm:"not null;uniqueIndex:idx_blank_exports_pair,priority:2" json:"attachment_id"`
	// UniqueAttachmentID и TemplateID - тип вложения и Excel-бланк, по которым файл
	// сгенерирован. Нужны, чтобы после правки шаблона пересобрать только его файлы.
	UniqueAttachmentID *int `gorm:"index" json:"unique_attachment_id"`
	TemplateID         *int `gorm:"index" json:"template_id"`

	// BucketDate - дата каталога: день подачи заявки в рабочей таймзоне. Хранится
	// отдельно от sending_datetime, потому что раскладка по годам и месяцам считается
	// именно по ней, а сравнивать date с timestamp в UTC на каждой выборке дорого.
	BucketDate time.Time `gorm:"type:date;not null;index" json:"bucket_date"`

	RelDir      string `gorm:"size:1024;not null;default:''" json:"rel_dir"`
	FileName    string `gorm:"size:512;not null;default:''" json:"file_name"`
	SizeBytes   int64  `gorm:"not null;default:0" json:"size_bytes"`
	ContentHash string `gorm:"size:64;not null;default:''" json:"content_hash"`

	Status   string `gorm:"size:16;not null;default:'pending';index:idx_blank_exports_queue,priority:1" json:"status"`
	Attempts int    `gorm:"not null;default:0" json:"attempts"`
	// LastError - текст последней ошибки для списка «Ошибки» в интерфейсе. Пустая
	// строка у успешных: NULL здесь ничего не добавляет, а условий в выборках прибавляет.
	LastError string `gorm:"type:text;not null;default:''" json:"last_error"`
	// NextAttemptAt - когда воркер имеет право взять строку снова. NULL у тех, кого
	// ретраить не нужно (ok, skipped, no_template).
	NextAttemptAt *time.Time `gorm:"index:idx_blank_exports_queue,priority:2" json:"next_attempt_at"`

	// QueueReason - что поставило строку в очередь (подача, правка, бэкфилл, сверка).
	// Нужен при разборе «почему файл переписался», когда заявку никто не трогал.
	QueueReason string     `gorm:"size:32;not null;default:''" json:"queue_reason"`
	QueuedAt    time.Time  `gorm:"not null" json:"queued_at"`
	GeneratedAt *time.Time `json:"generated_at"`

	// FrozenAt - момент, после которого файл считается окончательным и больше не
	// перезаписывается (заморозка через archive.freeze_after_days от завершения
	// заявки). Без заморозки корпоративная копия расходилась бы с оригиналом, а
	// удаление с арендованной машины было бы небезопасным: переносить и удалять
	// можно только то, что уже не изменится.
	FrozenAt *time.Time `gorm:"index" json:"frozen_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName задаёт имя таблицы явно, без gorm-плюрализации.
func (BlankExport) TableName() string { return "blank_exports" }

// Статусы строки реестра. Разделение «не получилось» на четыре состояния сделано
// ради воркера и интерфейса: ретраить имеет смысл только BlankExportFailed и
// BlankExportBlocked, остальные ждут действия человека или вообще ничего не ждут.
const (
	// BlankExportPending - в очереди на выгрузку.
	BlankExportPending = "pending"
	// BlankExportOK - файл записан, хэш и размер актуальны.
	BlankExportOK = "ok"
	// BlankExportFailed - транзиентная ошибка, ретрай с нарастающей паузой.
	BlankExportFailed = "failed"
	// BlankExportSkipped - выгрузка выключена для этого типа вложения или глобально.
	BlankExportSkipped = "skipped"
	// BlankExportNoTemplate - у типа вложения нет активного Excel-бланка. Отдельно от
	// skipped: это не решение администратора, а видимый пробел в архиве, и его надо
	// показывать счётчиком, иначе неполнота обнаружится в момент, когда заявка
	// понадобится.
	BlankExportNoTemplate = "no_template"
	// BlankExportBlocked - выгрузка остановлена нехваткой места или квотой.
	BlankExportBlocked = "blocked"
	// BlankExportOrphan - вложение исчезло, а файл на диске остался. Докладывается,
	// но не удаляется автоматически.
	BlankExportOrphan = "orphan"
)

// AllBlankExportStatuses - перечень известных статусов. Нужен там, где статус приходит
// снаружи (фильтр списка ошибок): опечатку надо поймать, а не отдать пустую выборку.
var AllBlankExportStatuses = []string{
	BlankExportPending, BlankExportOK, BlankExportFailed, BlankExportSkipped,
	BlankExportNoTemplate, BlankExportBlocked, BlankExportOrphan,
}

// ArchiveSettings - настройки файлового архива. Живут в system_settings под ключами
// archive.*, но мимо knownKeys: универсальный PUT /settings/:key доступен только
// супер-администратору и не умеет проверять шаблон пути (#1615).
//
// Корня архива здесь нет намеренно: путь задаётся переменной окружения. Смена корня
// из интерфейса означала бы мгновенное «архив пропал» и запись куда угодно на диске.
type ArchiveSettings struct {
	// Enabled - глобальный рубильник выгрузки. Выключен по умолчанию: включение
	// начинает писать на диск персональные данные, это осознанное действие
	// администратора, а не следствие обновления системы.
	Enabled bool `json:"enabled"`

	DirTemplate  string `json:"dir_template"`
	FileTemplate string `json:"file_template"`

	// QuotaBytes - потолок объёма архива, 0 = без ограничения.
	QuotaBytes int64 `json:"quota_bytes"`
	// MinFreeBytes - сколько места на разделе обязано остаться свободным. Ниже порога
	// выгрузка встаёт в blocked, но подача заявок не затрагивается.
	MinFreeBytes int64 `json:"min_free_bytes"`
	// WarnPercent - доля заполнения, после которой администраторы получают уведомление.
	WarnPercent int `json:"warn_percent"`
	// RecheckDays - глубина окна ночной сверки реестра с диском.
	RecheckDays int `json:"recheck_days"`
	// FreezeAfterDays - через сколько дней после завершения заявки её файлы
	// становятся окончательными. 0 - замораживать сразу при завершении.
	FreezeAfterDays int `json:"freeze_after_days"`
	// ZipMaxBytes - потолок одной выгрузки ZIP за период.
	ZipMaxBytes int64 `json:"zip_max_bytes"`
}

// UpdateArchiveSettingsRequest - частичное обновление настроек архива. Все поля
// указатели: форма правит одну настройку за раз, а отсутствующий ключ обязан
// означать «не трогать», иначе сохранение шаблона сбрасывало бы квоту и пороги.
type UpdateArchiveSettingsRequest struct {
	Enabled         *bool   `json:"enabled"`
	DirTemplate     *string `json:"dir_template"`
	FileTemplate    *string `json:"file_template"`
	QuotaBytes      *int64  `json:"quota_bytes"`
	MinFreeBytes    *int64  `json:"min_free_bytes"`
	WarnPercent     *int    `json:"warn_percent"`
	RecheckDays     *int    `json:"recheck_days"`
	FreezeAfterDays *int    `json:"freeze_after_days"`
	ZipMaxBytes     *int64  `json:"zip_max_bytes"`
}

// ArchivePreviewRequest - запрос живого превью шаблонов пути. Шаблоны приходят как
// есть из редактора, до сохранения: конструктор показывает результат на каждый ввод.
type ArchivePreviewRequest struct {
	DirTemplate  string `json:"dir_template"`
	FileTemplate string `json:"file_template"`
	// ApplicationID - на какой заявке показывать. 0 означает «взять последнюю
	// поданную», а если заявок нет вовсе - собрать пример из значений-образцов.
	ApplicationID int `json:"application_id"`
}

// ArchivePreviewResponse - результат превью: разложенный путь и претензии к шаблонам.
// Претензии отдаются отдельно от ошибки, чтобы конструктор подсвечивал конкретный
// плейсхолдер и продолжал показывать превью по остальным.
type ArchivePreviewResponse struct {
	Levels   []string `json:"levels"`
	FileName string   `json:"file_name"`
	RelPath  string   `json:"rel_path"`
	// Synthetic - превью построено на значениях-образцах, а не на реальной заявке.
	Synthetic bool `json:"synthetic"`
	// ApplicationNumber - номер заявки, по которой построено превью (пусто у образца).
	ApplicationNumber string                 `json:"application_number"`
	DirProblems       []ArchiveTemplateIssue `json:"dir_problems"`
	FileProblems      []ArchiveTemplateIssue `json:"file_problems"`
}

// ArchiveTemplateIssue - претензия к одному плейсхолдеру шаблона.
type ArchiveTemplateIssue struct {
	Token  string `json:"token"`
	Reason string `json:"reason"`
}
