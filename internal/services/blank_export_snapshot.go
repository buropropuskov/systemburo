package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// archiveSnapshotFileName - машиночитаемый слепок заявки рядом с бланками (#1615).
// Бланк показывает только то, что попало в разметку Excel-шаблона; слепок хранит
// все поля заявки, чтобы корпоративная копия пережила потерю арендованного сервера
// вместе с базой, а не то немногое, что администратор когда-то отобразил в бланке.
const archiveSnapshotFileName = "заявка.json"

// archiveSnapshotSchemaVersion - версия состава полей заявка.json. Меняется вместе
// со структурой снапшота, чтобы читающая сторона (корпоративный сервер) могла
// отличить файлы, записанные до и после расширения состава.
const archiveSnapshotSchemaVersion = 1

// archiveSnapshotAttachmentID - зарезервированное значение attachment_id для
// строки реестра слепка заявки (B1, долг A5c). 0 не занят реальными вложениями
// (их id всегда положительны), и слепок под ним пользуется тем же механизмом
// currentDir/frozenDir/relocate, что и обычные бланки: без строки реестра заявка
// без единого бланка не знает своего фактического пути и после смены организации
// заявка.json остаётся сиротой в прежней папке.
const archiveSnapshotAttachmentID = 0

// applicationSnapshot - корень заявка.json. Поля описаны структурами, а не
// map[string]any: порядок полей структуры фиксирован её объявлением, тогда как
// порядок ключей карты решала бы сортировка заново на каждый вызов - тест на
// побайтовую стабильность результата тогда проверял бы не то, что нужно.
type applicationSnapshot struct {
	SchemaVersion int                  `json:"schema_version"`
	Application   snapshotApplication  `json:"application"`
	Approvals     []snapshotApproval   `json:"approvals"`
	Attachments   []snapshotAttachment `json:"attachments"`
}

type snapshotApplication struct {
	ID                   int    `json:"id"`
	Number               string `json:"number,omitempty"`
	Status               string `json:"status,omitempty"`
	Confirmation         string `json:"confirmation,omitempty"`
	Organization         string `json:"organization,omitempty"`
	Company              string `json:"company,omitempty"`
	SenderName           string `json:"sender_name,omitempty"`
	SenderUsername       string `json:"sender_username,omitempty"`
	InitiatorName        string `json:"initiator_name,omitempty"`
	ContactPhone         string `json:"contact_phone,omitempty"`
	Message              string `json:"message,omitempty"`
	ResponsibleName      string `json:"responsible_name,omitempty"`
	ResponsibleComment   string `json:"responsible_comment,omitempty"`
	SendingDatetime      string `json:"sending_datetime,omitempty"`
	ReadingDatetime      string `json:"reading_datetime,omitempty"`
	ConfirmationDatetime string `json:"confirmation_datetime,omitempty"`
	AcceptedAt           string `json:"accepted_at,omitempty"`
	CompletedAt          string `json:"completed_at,omitempty"`
	WithdrawnAt          string `json:"withdrawn_at,omitempty"`
}

// snapshotApproval - согласование заявки (ApplicationResponsibleUser). В отличие от
// подписи бланка, где подписью заявки становится только "approved", слепок хранит
// ВСЕ статусы согласования разом: он аудиторская копия, а не форма для печати.
type snapshotApproval struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Status   string `json:"status,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Datetime string `json:"datetime,omitempty"`
}

type snapshotAttachment struct {
	ID            int                `json:"id"`
	Type          string             `json:"type"`
	TypeName      string             `json:"type_name,omitempty"`
	EntryDateFrom string             `json:"entry_date_from,omitempty"`
	EntryTimeFrom string             `json:"entry_time_from,omitempty"`
	EntryDateTo   string             `json:"entry_date_to,omitempty"`
	EntryTimeTo   string             `json:"entry_time_to,omitempty"`
	RoofAccess    bool               `json:"roof_access,omitempty"`
	FreeParking   bool               `json:"free_parking,omitempty"`
	UnloadPlaces  []string           `json:"unload_places,omitempty"`
	Employees     []snapshotEmployee `json:"employees,omitempty"`
	Cars          []snapshotCar      `json:"cars,omitempty"`
	Items         []snapshotItem     `json:"items,omitempty"`
}

type snapshotEmployee struct {
	ID          int    `json:"id"`
	LastName    string `json:"last_name,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	MiddleName  string `json:"middle_name,omitempty"`
	Position    string `json:"position,omitempty"`
	Citizenship string `json:"citizenship,omitempty"`
	// PassportSeriesNumber/PatentNumber - открытым текстом. В базе поле шифруется
	// (Employee.AfterFind), но бланк по тем же данным (attachment_blank_service)
	// кладёт их читаемыми, и слепок обязан отражать то же самое - иначе он
	// бесполезен как аудиторская копия. Каталог архива - хранилище того же класса
	// защиты, что и база (решение зафиксировано в context.md эпика).
	PassportSeriesNumber string   `json:"passport_series_number,omitempty"`
	PatentNumber         string   `json:"patent_number,omitempty"`
	OtherPermission      string   `json:"other_permission,omitempty"`
	TargetTables         []string `json:"target_tables,omitempty"`
}

type snapshotCar struct {
	ID            int      `json:"id"`
	Number        string   `json:"number,omitempty"`
	Mark          string   `json:"mark,omitempty"`
	UnloadPlace   string   `json:"unload_place,omitempty"`
	EntryDateFrom string   `json:"entry_date_from,omitempty"`
	EntryTimeFrom string   `json:"entry_time_from,omitempty"`
	EntryDateTo   string   `json:"entry_date_to,omitempty"`
	EntryTimeTo   string   `json:"entry_time_to,omitempty"`
	UnloadPlaces  []string `json:"unload_places,omitempty"`
	PassageTables []string `json:"passage_tables,omitempty"`
}

type snapshotItem struct {
	Name  string `json:"name,omitempty"`
	Count *int   `json:"count,omitempty"`
}

// buildApplicationSnapshot собирает заявка.json: поля заявки, людей, машин, ТМЦ,
// согласований и мест разгрузки. Источник данных тот же, что и у бланка
// (attachment_blank_service), но без ограничения одним вложением - слепок обязан
// пережить потерю базы целиком, а не только то, что попало в разметку конкретного
// Excel-бланка.
func buildApplicationSnapshot(ctx context.Context, db *gorm.DB, applicationID int) ([]byte, error) {
	snap := applicationSnapshot{SchemaVersion: archiveSnapshotSchemaVersion}

	app, err := loadSnapshotApplication(ctx, db, applicationID)
	if err != nil {
		return nil, err
	}
	snap.Application = app

	if snap.Approvals, err = loadSnapshotApprovals(ctx, db, applicationID); err != nil {
		return nil, err
	}
	if snap.Attachments, err = loadSnapshotAttachments(ctx, db, applicationID); err != nil {
		return nil, err
	}

	return json.MarshalIndent(snap, "", "  ")
}

// snapshotApplicationRow - плоский приёмник запроса заявки. Плоский намеренно: gorm
// молча не заполняет анонимно встроенные структуры при Scan, и поля пришли бы нулями.
type snapshotApplicationRow struct {
	ID                   int        `gorm:"column:id"`
	Number               string     `gorm:"column:number"`
	Status               string     `gorm:"column:status"`
	Confirmation         string     `gorm:"column:confirmation"`
	Organization         string     `gorm:"column:organization"`
	Company              string     `gorm:"column:company"`
	SenderName           string     `gorm:"column:sender_name"`
	SenderUsername       string     `gorm:"column:sender_username"`
	InitiatorName        string     `gorm:"column:initiator_name"`
	ContactPhone         string     `gorm:"column:contact_phone"`
	Message              string     `gorm:"column:message"`
	ResponsibleName      string     `gorm:"column:responsible_name"`
	ResponsibleComment   string     `gorm:"column:responsible_comment"`
	SendingDatetime      *time.Time `gorm:"column:sending_datetime"`
	ReadingDatetime      *time.Time `gorm:"column:reading_datetime"`
	ConfirmationDatetime *time.Time `gorm:"column:confirmation_datetime"`
	AcceptedAt           *time.Time `gorm:"column:accepted_at"`
	CompletedAt          *time.Time `gorm:"column:completed_at"`
	WithdrawnAt          *time.Time `gorm:"column:withdrawn_at"`
}

func loadSnapshotApplication(ctx context.Context, db *gorm.DB, applicationID int) (snapshotApplication, error) {
	const sql = `
		SELECT a.id AS id,
		       COALESCE(a.application_number, '') AS number,
		       COALESCE(a.status, '') AS status,
		       COALESCE(a.confirmation, '') AS confirmation,
		       COALESCE(o.name, '') AS organization,
		       COALESCE(c.name, '') AS company,
		       COALESCE(format_full_name(u.last_name, u.first_name, u.middle_name), '') AS sender_name,
		       COALESCE(u.username, '') AS sender_username,
		       COALESCE(a.initiator_name, '') AS initiator_name,
		       COALESCE(a.contact_phone, '') AS contact_phone,
		       COALESCE(a.message, '') AS message,
		       COALESCE(format_full_name(ru.last_name, ru.first_name, ru.middle_name), '') AS responsible_name,
		       COALESCE(a.responsible_comment, '') AS responsible_comment,
		       a.sending_datetime, a.reading_datetime, a.confirmation_datetime,
		       a.accepted_at, a.completed_at, a.withdrawn_at
		FROM applications a
		LEFT JOIN organizations o ON o.id = a.organization_id
		LEFT JOIN companies c ON c.id = a.company_id
		LEFT JOIN users u ON u.id = a.sender_user_id
		LEFT JOIN users ru ON ru.id = a.responsible_user_id
		WHERE a.id = ?`

	var row snapshotApplicationRow
	if err := db.WithContext(ctx).Raw(sql, applicationID).Scan(&row).Error; err != nil {
		return snapshotApplication{}, fmt.Errorf("failed to load application for archive snapshot: %w", err)
	}

	return snapshotApplication{
		ID: row.ID, Number: row.Number, Status: row.Status, Confirmation: row.Confirmation,
		Organization: row.Organization, Company: row.Company,
		SenderName: row.SenderName, SenderUsername: row.SenderUsername,
		InitiatorName: row.InitiatorName, ContactPhone: row.ContactPhone, Message: row.Message,
		ResponsibleName: row.ResponsibleName, ResponsibleComment: row.ResponsibleComment,
		SendingDatetime:      formatSnapshotTime(row.SendingDatetime),
		ReadingDatetime:      formatSnapshotTime(row.ReadingDatetime),
		ConfirmationDatetime: formatSnapshotTime(row.ConfirmationDatetime),
		AcceptedAt:           formatSnapshotTime(row.AcceptedAt),
		CompletedAt:          formatSnapshotTime(row.CompletedAt),
		WithdrawnAt:          formatSnapshotTime(row.WithdrawnAt),
	}, nil
}

func loadSnapshotApprovals(ctx context.Context, db *gorm.DB, applicationID int) ([]snapshotApproval, error) {
	var rows []models.ApplicationResponsibleUser
	err := db.WithContext(ctx).Preload("User").
		Where("application_id = ?", applicationID).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load approvals for archive snapshot: %w", err)
	}

	out := make([]snapshotApproval, 0, len(rows))
	for i := range rows {
		u := rows[i].User
		out = append(out, snapshotApproval{
			Name:     snapshotFullName(u.LastName, u.FirstName, u.MiddleName),
			Required: rows[i].RequiredApproval,
			Status:   derefStr(rows[i].ApprovalStatus),
			Comment:  derefStr(rows[i].ApprovalComment),
			Datetime: formatSnapshotTime(rows[i].ApprovalDatetime),
		})
	}
	return out, nil
}

// snapshotAttachmentRow - плоский приёмник запроса вложений заявки.
type snapshotAttachmentRow struct {
	ID            int    `gorm:"column:id"`
	Type          string `gorm:"column:attachment_type"`
	TypeName      string `gorm:"column:type_name"`
	EntryDateFrom string `gorm:"column:entry_date_from"`
	EntryTimeFrom string `gorm:"column:entry_time_from"`
	EntryDateTo   string `gorm:"column:entry_date_to"`
	EntryTimeTo   string `gorm:"column:entry_time_to"`
	RoofAccess    bool   `gorm:"column:roof_access"`
	FreeParking   bool   `gorm:"column:free_parking"`
}

// loadSnapshotAttachments читает все вложения заявки разом и догружает их людей,
// машины, ТМЦ и места разгрузки одним запросом на список - иначе на каждое
// вложение заявки был бы отдельный поход в базу.
func loadSnapshotAttachments(ctx context.Context, db *gorm.DB, applicationID int) ([]snapshotAttachment, error) {
	const sql = `
		SELECT at.id AS id,
		       at.attachment_type AS attachment_type,
		       COALESCE(NULLIF(ua.display_name, ''), NULLIF(ua.title, ''),
		                NULLIF(at.attachment_display_name, ''), NULLIF(at.attachment_name, ''), '') AS type_name,
		       COALESCE(at.entry_date_from, '') AS entry_date_from,
		       COALESCE(at.entry_time_from, '') AS entry_time_from,
		       COALESCE(at.entry_date_to, '') AS entry_date_to,
		       COALESCE(at.entry_time_to, '') AS entry_time_to,
		       at.roof_access AS roof_access,
		       at.free_parking AS free_parking
		FROM attachments at
		LEFT JOIN unique_attachments ua ON ua.id = at.unique_attachment_id
		WHERE at.application_id = ?
		ORDER BY at.id`

	var rows []snapshotAttachmentRow
	if err := db.WithContext(ctx).Raw(sql, applicationID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load attachments for archive snapshot: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	unloadPlaces, err := groupSnapshotNames(ctx, db, `
		SELECT aup.attachment_id AS owner_id, up.name
		FROM attachment_unload_places aup
		JOIN unload_places up ON aup.unload_place_id = up.id
		WHERE aup.attachment_id IN ?
		ORDER BY aup.order_index NULLS LAST, up.name
	`, ids)
	if err != nil {
		return nil, err
	}

	employees, err := loadSnapshotEmployees(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	cars, err := loadSnapshotCars(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	items, err := loadSnapshotItems(ctx, db, ids)
	if err != nil {
		return nil, err
	}

	out := make([]snapshotAttachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, snapshotAttachment{
			ID: r.ID, Type: r.Type, TypeName: r.TypeName,
			EntryDateFrom: r.EntryDateFrom, EntryTimeFrom: r.EntryTimeFrom,
			EntryDateTo: r.EntryDateTo, EntryTimeTo: r.EntryTimeTo,
			RoofAccess: r.RoofAccess, FreeParking: r.FreeParking,
			UnloadPlaces: unloadPlaces[r.ID],
			Employees:    employees[r.ID],
			Cars:         cars[r.ID],
			Items:        items[r.ID],
		})
	}
	return out, nil
}

// loadSnapshotEmployees читает людей всех переданных вложений через модель, а не
// сырым SQL - это включает Employee.AfterFind и отдаёт паспорт/патент уже
// расшифрованными, как их видит бланк.
func loadSnapshotEmployees(ctx context.Context, db *gorm.DB, attachmentIDs []int) (map[int][]snapshotEmployee, error) {
	out := make(map[int][]snapshotEmployee)

	var rows []models.Employee
	if err := db.WithContext(ctx).Where("attachment_id IN ?", attachmentIDs).Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load employees for archive snapshot: %w", err)
	}
	if len(rows) == 0 {
		return out, nil
	}

	citizenIDs := make([]int, 0, len(rows))
	for _, e := range rows {
		if e.CitizenshipID != nil {
			citizenIDs = append(citizenIDs, *e.CitizenshipID)
		}
	}
	citizenships := make(map[int]string, len(citizenIDs))
	if len(citizenIDs) > 0 {
		var cz []models.Citizenship
		if err := db.WithContext(ctx).Where("id IN ?", citizenIDs).Find(&cz).Error; err != nil {
			return nil, fmt.Errorf("failed to load citizenships for archive snapshot: %w", err)
		}
		for _, c := range cz {
			citizenships[c.ID] = c.Name
		}
	}

	empIDs := make([]int, len(rows))
	for i, e := range rows {
		empIDs[i] = e.ID
	}
	targetTables, err := groupSnapshotNames(ctx, db, `
		SELECT ett.employee_id AS owner_id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name
		FROM employee_target_tables ett
		JOIN system_tables st ON ett.table_id = st.id
		WHERE ett.employee_id IN ?
		ORDER BY ett.order_index NULLS LAST, name
	`, empIDs)
	if err != nil {
		return nil, err
	}

	for _, e := range rows {
		if e.AttachmentID == nil {
			continue
		}
		citizenship := ""
		if e.CitizenshipID != nil {
			citizenship = citizenships[*e.CitizenshipID]
		}
		out[*e.AttachmentID] = append(out[*e.AttachmentID], snapshotEmployee{
			ID: e.ID, LastName: derefStr(e.LastName), FirstName: derefStr(e.FirstName),
			MiddleName: derefStr(e.MiddleName), Position: derefStr(e.Position),
			Citizenship:          citizenship,
			PassportSeriesNumber: derefStr(e.PassportSeriesNumber),
			PatentNumber:         derefStr(e.PatentNumber),
			OtherPermission:      derefStr(e.OtherPermission),
			TargetTables:         targetTables[e.ID],
		})
	}
	return out, nil
}

func loadSnapshotCars(ctx context.Context, db *gorm.DB, attachmentIDs []int) (map[int][]snapshotCar, error) {
	out := make(map[int][]snapshotCar)

	var rows []models.Car
	if err := db.WithContext(ctx).Where("attachment_id IN ?", attachmentIDs).Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load cars for archive snapshot: %w", err)
	}
	if len(rows) == 0 {
		return out, nil
	}

	carIDs := make([]int, len(rows))
	for i, c := range rows {
		carIDs[i] = c.ID
	}
	unloadPlaces, err := groupSnapshotNames(ctx, db, `
		SELECT cup.car_id AS owner_id, up.name
		FROM car_unload_places cup
		JOIN unload_places up ON cup.unload_place_id = up.id
		WHERE cup.car_id IN ?
		ORDER BY cup.order_index NULLS LAST, up.name
	`, carIDs)
	if err != nil {
		return nil, err
	}
	passageTables, err := groupSnapshotNames(ctx, db, `
		SELECT ctt.car_id AS owner_id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name
		FROM car_target_tables ctt
		JOIN system_tables st ON ctt.table_id = st.id
		WHERE ctt.car_id IN ?
		ORDER BY ctt.order_index NULLS LAST, name
	`, carIDs)
	if err != nil {
		return nil, err
	}

	for _, c := range rows {
		out[c.AttachmentID] = append(out[c.AttachmentID], snapshotCar{
			ID: c.ID, Number: derefStr(c.CarNumber), Mark: snapshotCarMark(c),
			UnloadPlace:   derefStr(c.UnloadPlace),
			EntryDateFrom: derefStr(c.EntryDateFrom), EntryTimeFrom: derefStr(c.EntryTimeFrom),
			EntryDateTo: derefStr(c.EntryDateTo), EntryTimeTo: derefStr(c.EntryTimeTo),
			UnloadPlaces:  unloadPlaces[c.ID],
			PassageTables: passageTables[c.ID],
		})
	}
	return out, nil
}

// snapshotCarMark выбирает актуальное название марки: снимок присвоения (MarkName)
// важнее устаревшего свободного текста CarBrand - тот же приоритет, что и у
// остального интерфейса при отображении машины.
func snapshotCarMark(c models.Car) string {
	if c.MarkName != nil && strings.TrimSpace(*c.MarkName) != "" {
		return *c.MarkName
	}
	return derefStr(c.CarBrand)
}

func loadSnapshotItems(ctx context.Context, db *gorm.DB, attachmentIDs []int) (map[int][]snapshotItem, error) {
	out := make(map[int][]snapshotItem)

	var rows []models.Item
	if err := db.WithContext(ctx).Where("attachment_id IN ?", attachmentIDs).Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load items for archive snapshot: %w", err)
	}
	for _, it := range rows {
		out[it.AttachmentID] = append(out[it.AttachmentID], snapshotItem{Name: derefStr(it.Name), Count: it.Count})
	}
	return out, nil
}

// groupSnapshotNames раскладывает результат запроса вида (owner_id, name) по
// владельцам - то же, что делает groupNamesByOwner для бланка, но с ошибкой наружу.
//
// Отдельный хелпер именно ради ошибки. Общий groupNamesByOwner при сбое запроса
// логирует и отдаёт то, что успел собрать: у бланка это строка, пропавшая из
// таблицы, которую человек увидит глазами при печати. У слепка тот же пропуск
// молчалив - пустые места разгрузки или посты запишутся как полный файл, а после
// заморозки останутся такими навсегда. Поэтому сбой запроса здесь отменяет запись
// слепка целиком: лучше отсутствующий файл, который видно в ответе и который
// перезапишется на следующем прогоне, чем правдоподобно неполный.
func groupSnapshotNames(ctx context.Context, db *gorm.DB, query string, ids []int) (map[int][]string, error) {
	out := make(map[int][]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	var rows []struct {
		OwnerID int    `gorm:"column:owner_id"`
		Name    string `gorm:"column:name"`
	}
	if err := db.WithContext(ctx).Raw(query, ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load related names for archive snapshot: %w", err)
	}
	for _, r := range rows {
		out[r.OwnerID] = append(out[r.OwnerID], r.Name)
	}
	return out, nil
}

// snapshotFullName склеивает ФИО из указателей на части имени.
func snapshotFullName(last, first, middle *string) string {
	parts := make([]string, 0, 3)
	for _, p := range []*string{last, first, middle} {
		if p != nil && strings.TrimSpace(*p) != "" {
			parts = append(parts, strings.TrimSpace(*p))
		}
	}
	return strings.Join(parts, " ")
}

// formatSnapshotTime приводит момент к RFC3339 в UTC. Формат детерминирован для
// одного и того же значения независимо от того, в какой локации его вернул
// драйвер БД - без приведения к общей зоне побайтовая стабильность слепка между
// прогонами была бы негарантирована.
func formatSnapshotTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// writeApplicationSnapshot кладёт заявка.json в папку заявки тем же писателем и по
// тем же гарантиям атомарности, что и бланки (ArchiveWriter.WriteFile). Возвращает,
// был ли файл записан на этом прогоне, и хэш/размер актуального содержимого - с B1
// слепок ведёт строку реестра, как и обычные бланки, и ей нужно чем заполниться.
//
// Заморозка запрещает ПЕРЕЗАПИСЬ уже лежащего слепка, но не первую его запись, и
// проверяется наличием файла на диске, а не признаком «заявка заморожена» из
// реестра. Гейт по признаку оставлял бы без слепка навсегда: заявки, замороженные
// до появления этого кода; заявки, у которых запись сорвалась ровно на том прогоне,
// где замёрзли бланки; и починить это было бы нечем - ручное «пересоздать»
// упиралось бы в тот же признак. Строка реестра (B1) добавлена только ради
// фактического пути для переезда, решение «писать или нет» по-прежнему смотрит на
// диск, а не на неё.
func writeApplicationSnapshot(ctx context.Context, db *gorm.DB, writer *ArchiveWriter, applicationID int, levels []string, frozen bool) (written bool, hash string, size int64, err error) {
	// Имя слепка тоже несёт признак шифрования: по нему чтение понимает, надо ли
	// расшифровывать. Без суффикса зашифрованный файл отдавался бы как есть, и
	// принимающая сторона получила бы нечитаемый мусор вместо описания заявки.
	snapshotName := writer.Crypto().FileName(archiveSnapshotFileName)

	exists, err := writer.Exists(levels, snapshotName)
	if err != nil {
		return false, "", 0, err
	}
	if frozen && exists {
		return false, "", 0, nil
	}

	data, err := buildApplicationSnapshot(ctx, db, applicationID)
	if err != nil {
		return false, "", 0, err
	}
	sum := sha256.Sum256(data)
	hash, size = hex.EncodeToString(sum[:]), int64(len(data))

	if exists {
		changed, err := snapshotContentChanged(writer, levels, data)
		if err != nil {
			return false, hash, size, err
		}
		if !changed {
			return false, hash, size, nil
		}
	}

	if err := writer.WriteFile(levels, snapshotName, data); err != nil {
		return false, hash, size, err
	}
	return true, hash, size, nil
}

// snapshotContentChanged сравнивает хэш нового слепка с уже лежащим на диске.
// Совпадение останавливает запись до неё самой: как и у бланков, это экономит I/O
// и не двигает mtime, от которого зависит инкрементальная синхронизация на рабочий
// компьютер.
func snapshotContentChanged(writer *ArchiveWriter, levels []string, data []byte) (bool, error) {
	full, err := writer.Resolve(append(append([]string{}, levels...), writer.Crypto().FileName(archiveSnapshotFileName))...)
	if err != nil {
		return false, err
	}

	// Сравнивать надо расшифрованное содержимое: шифрование берёт новый ключ потока
	// на каждую запись, поэтому байты на диске отличаются даже у неизменного слепка,
	// и сравнение шифротекстов переписывало бы файл на каждом прогоне - двигая mtime
	// и заставляя синхронизацию на рабочий компьютер перекачивать его снова.
	rc, err := writer.Crypto().Open(full)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("failed to read existing archive snapshot: %w", err)
	}
	defer rc.Close()

	existing, err := io.ReadAll(rc)
	if err != nil {
		return false, fmt.Errorf("failed to read existing archive snapshot: %w", err)
	}

	newSum := sha256.Sum256(data)
	oldSum := sha256.Sum256(existing)
	return !bytes.Equal(newSum[:], oldSum[:]), nil
}
