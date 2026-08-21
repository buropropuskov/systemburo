package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/models"
)

// resolveValue возвращает строковое значение поля по field_path для конкретной
// строки списка (rowIdx). Для не-list полей rowIdx игнорируется.
//
// field_path следует whitelist из BuiltinTemplateFields() или формату
// "custom.<id>" для кастомных полей.
func resolveValue(bctx *BlankContext, path string, rowIdx int) string {
	switch {
	case strings.HasPrefix(path, "app_items."):
		return resolveApplicationItems(bctx, path)
	case strings.HasPrefix(path, "application."):
		return resolveApplication(bctx, path)
	case strings.HasPrefix(path, "attachment."):
		return resolveAttachment(bctx, path)
	case strings.HasPrefix(path, "car."):
		if rowIdx >= len(bctx.Cars) {
			return ""
		}
		return resolveCar(bctx, &bctx.Cars[rowIdx], path, rowIdx)
	case strings.HasPrefix(path, "employee."):
		if rowIdx >= len(bctx.Employees) {
			return ""
		}
		return resolveEmployee(bctx, &bctx.Employees[rowIdx], path, rowIdx)
	case strings.HasPrefix(path, "item."):
		if rowIdx >= len(bctx.Items) {
			return ""
		}
		return resolveItem(&bctx.Items[rowIdx], path, rowIdx)
	case strings.HasPrefix(path, "custom."):
		id, err := strconv.Atoi(strings.TrimPrefix(path, "custom."))
		if err != nil {
			return ""
		}
		return bctx.CustomValues[id]
	}
	return ""
}

func resolveApplication(bctx *BlankContext, path string) string {
	// Согласовавшие живут в контексте отдельным списком - они не зависят от того,
	// удалось ли загрузить саму заявку.
	switch path {
	case "application.approver_name":
		return joinApprovers(approversForSignature(bctx.Approvers), false)
	case "application.approver_short_name":
		return joinApprovers(approversForSignature(bctx.Approvers), true)
	case "application.approvers":
		return joinApprovers(bctx.Approvers, false)
	case "application.approvers_short":
		return joinApprovers(bctx.Approvers, true)
	}

	app := bctx.Application
	if app == nil {
		return ""
	}
	switch path {
	case "application.application_number":
		return derefStr(app.ApplicationNumber)
	case "application.sending_datetime":
		if app.SendingDatetime == nil {
			return ""
		}
		return app.SendingDatetime.Format("02.01.2006")
	case "application.status":
		return derefStr(app.Status)
	case "application.confirmation":
		return derefStr(app.Confirmation)
	case "application.message":
		return derefStr(app.Message)
	case "application.organization":
		if bctx.Organization != nil {
			return bctx.Organization.Name
		}
	case "application.company":
		if bctx.Company != nil {
			return bctx.Company.Name
		}
	case "application.sender.full_name":
		if bctx.Sender != nil {
			return joinFullName(derefStr(bctx.Sender.LastName), derefStr(bctx.Sender.FirstName), derefStr(bctx.Sender.MiddleName))
		}
	case "application.sender.short_name":
		if bctx.Sender != nil {
			return joinShortName(derefStr(bctx.Sender.LastName), derefStr(bctx.Sender.FirstName), derefStr(bctx.Sender.MiddleName))
		}
	case "application.sender.last_name":
		if bctx.Sender != nil {
			return derefStr(bctx.Sender.LastName)
		}
	case "application.sender.first_name":
		if bctx.Sender != nil {
			return derefStr(bctx.Sender.FirstName)
		}
	case "application.sender.middle_name":
		if bctx.Sender != nil {
			return derefStr(bctx.Sender.MiddleName)
		}
	case "application.sender.phone":
		if bctx.Sender != nil {
			return formatPhone(derefStr(bctx.Sender.Phone))
		}
	case "application.sender.email":
		if bctx.Sender != nil {
			return derefStr(bctx.Sender.Email)
		}
	case "application.sender.position":
		if bctx.Sender != nil {
			return derefStr(bctx.Sender.Position)
		}
	case "application.confirmation_datetime":
		if app.ConfirmationDatetime != nil {
			return app.ConfirmationDatetime.Format("02.01.2006")
		}
	case "application.initiator_name":
		// Инициатора указывают в шапке подачи; у заявок до появления поля берём
		// отправителя - по умолчанию инициатор это он.
		if name := derefStr(app.InitiatorName); name != "" {
			return name
		}
		if bctx.Sender != nil {
			return joinFullName(derefStr(bctx.Sender.LastName), derefStr(bctx.Sender.FirstName), derefStr(bctx.Sender.MiddleName))
		}
	case "application.contact_phone":
		if phone := derefStr(app.ContactPhone); phone != "" {
			return formatPhone(phone)
		}
		if bctx.Sender != nil {
			return formatPhone(derefStr(bctx.Sender.Phone))
		}
	case "application.responsible_comment":
		return derefStr(app.ResponsibleComment)
	}
	return ""
}

// approversForSignature - кого писать под «СОГЛАСОВАНО»: обязательные согласования
// перечисляются ВСЕ (каждое из них - отдельное требование заявки), а необязательные
// представляет первый согласовавший.
func approversForSignature(all []Approver) []Approver {
	required := make([]Approver, 0, len(all))
	for _, a := range all {
		if a.Required {
			required = append(required, a)
		}
	}
	if len(required) > 0 {
		return required
	}
	if len(all) > 0 {
		return all[:1]
	}
	return nil
}

// joinApprovers печатает ФИО согласовавших через запятую: полностью или Фамилия И.О.
func joinApprovers(list []Approver, short bool) string {
	names := make([]string, 0, len(list))
	for _, a := range list {
		var name string
		if short {
			name = joinShortName(a.LastName, a.FirstName, a.MiddleName)
		} else {
			name = joinFullName(a.LastName, a.FirstName, a.MiddleName)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// resolveApplicationItems печатает ТМЦ «Заявок на ввоз» этой заявки в бланке любого
// вложения. Перечни разделяются переносом строки: строки списка бланк рисует только для
// собственного типа вложения, поэтому ввозимый товар идёт столбиком в одной ячейке.
// Заявка без «Заявок на ввоз» оставляет ячейку такой, как её задал шаблон.
func resolveApplicationItems(bctx *BlankContext, path string) string {
	rows := bctx.ApplicationItems
	if len(rows) == 0 {
		return ""
	}
	switch path {
	case "app_items.names":
		names := make([]string, 0, len(rows))
		for _, r := range rows {
			if r.Name != "" {
				names = append(names, r.Name)
			}
		}
		return strings.Join(names, "\n")
	case "app_items.names_with_count":
		lines := make([]string, 0, len(rows))
		for _, r := range rows {
			if r.Name == "" {
				continue
			}
			// Количество не указано - печатаем одно наименование: "Груз - " выглядело бы
			// как потерянное значение.
			if r.Count == nil {
				lines = append(lines, r.Name)
				continue
			}
			lines = append(lines, r.Name+" - "+strconv.Itoa(*r.Count))
		}
		return strings.Join(lines, "\n")
	case "app_items.total_count":
		total, filled := 0, false
		for _, r := range rows {
			if r.Count != nil {
				total += *r.Count
				filled = true
			}
		}
		if !filled {
			return ""
		}
		return strconv.Itoa(total)
	case "app_items.positions_count":
		return strconv.Itoa(len(rows))
	case "app_items.sources":
		seen := make(map[string]struct{}, len(rows))
		names := make([]string, 0, len(rows))
		for _, r := range rows {
			if r.SourceName == "" {
				continue
			}
			if _, dup := seen[r.SourceName]; dup {
				continue
			}
			seen[r.SourceName] = struct{}{}
			names = append(names, r.SourceName)
		}
		return strings.Join(names, ", ")
	}
	return ""
}

// resolveApplicationItemRow печатает строку таблицы ТМЦ заявки теми же путями item.*,
// какими бланк «Заявки на ввоз» печатает собственное имущество: админ размечает вторую
// таблицу привычной группой полей, а данные идут из вложений-ввоза этой заявки.
func resolveApplicationItemRow(rows []ApplicationItemRow, path string, rowIdx int) string {
	if rowIdx < 0 || rowIdx >= len(rows) {
		return ""
	}
	it := rows[rowIdx]
	switch path {
	case "item.row_number":
		return strconv.Itoa(rowIdx + 1)
	case "item.name":
		return it.Name
	case "item.count":
		if it.Count != nil {
			return strconv.Itoa(*it.Count)
		}
	}
	return ""
}

func resolveAttachment(bctx *BlankContext, path string) string {
	a := bctx.Attachment
	if a == nil {
		return ""
	}
	// Одиночные дата и время форматируются так же, как диапазоны ниже (#1454): раньше
	// они отдавались сырыми из БД, и в одном бланке соседствовали "2026-07-15" и
	// "15.07.2026 - 17.07.2026", а время приезжало с секундами.
	switch path {
	case "attachment.entry_date_from":
		return formatDate(derefStr(a.EntryDateFrom))
	case "attachment.entry_date_to":
		return formatDate(derefStr(a.EntryDateTo))
	case "attachment.entry_time_from":
		return formatTime(derefStr(a.EntryTimeFrom))
	case "attachment.entry_time_to":
		return formatTime(derefStr(a.EntryTimeTo))
	case "attachment.entry_date_range":
		from := formatDate(derefStr(a.EntryDateFrom))
		to := formatDate(derefStr(a.EntryDateTo))
		if from != "" && to != "" {
			return from + " - " + to
		}
		if from != "" {
			return from
		}
		return to
	case "attachment.entry_time_range":
		from := formatTime(derefStr(a.EntryTimeFrom))
		to := formatTime(derefStr(a.EntryTimeTo))
		if from != "" && to != "" {
			return from + " - " + to
		}
		if from != "" {
			return from
		}
		return to
	case "attachment.display_name":
		if name := derefStr(a.AttachmentDisplayName); name != "" {
			return name
		}
		return derefStr(a.AttachmentName)
	case "attachment.unload_places":
		return strings.Join(bctx.AttachmentUnloadPlaces, ", ")
	case "attachment.roof_access":
		return yesNo(a.RoofAccess)
	case "attachment.free_parking":
		return yesNo(a.FreeParking)
	}
	return ""
}

// yesNo печатает булев признак вложения словом: пустая ячейка в бланке читалась бы
// как "поле не заполнено", а не как "нет".
func yesNo(v bool) string {
	if v {
		return "Да"
	}
	return "Нет"
}

func resolveCar(bctx *BlankContext, c *models.Car, path string, rowIdx int) string {
	switch path {
	case "car.row_number":
		return strconv.Itoa(rowIdx + 1)
	case "car.car_number":
		return derefStr(c.CarNumber)
	case "car.mark_name":
		if c.MarkName != nil && *c.MarkName != "" {
			return *c.MarkName
		}
		return derefStr(c.CarBrand)
	case "car.unload_place":
		// Строка, собранная формой подачи: при нескольких местах это "Первое и др.".
		// Полный перечень - в car.unload_places ниже (#1454).
		return derefStr(c.UnloadPlace)
	case "car.unload_places":
		return strings.Join(bctx.CarUnloadPlaces[c.ID], ", ")
	case "car.passage_tables":
		return strings.Join(bctx.CarPassageTables[c.ID], ", ")
	case "car.entry_date_from":
		return formatDate(derefStr(c.EntryDateFrom))
	case "car.entry_date_to":
		return formatDate(derefStr(c.EntryDateTo))
	case "car.entry_time_from":
		return formatTime(derefStr(c.EntryTimeFrom))
	case "car.entry_time_to":
		return formatTime(derefStr(c.EntryTimeTo))
	}
	return ""
}

func resolveEmployee(bctx *BlankContext, e *models.Employee, path string, rowIdx int) string {
	switch path {
	case "employee.row_number":
		return strconv.Itoa(rowIdx + 1)
	case "employee.last_name":
		return derefStr(e.LastName)
	case "employee.first_name":
		return derefStr(e.FirstName)
	case "employee.middle_name":
		return derefStr(e.MiddleName)
	case "employee.full_name":
		return joinFullName(derefStr(e.LastName), derefStr(e.FirstName), derefStr(e.MiddleName))
	case "employee.position":
		return derefStr(e.Position)
	case "employee.citizenship":
		if e.CitizenshipID != nil {
			return bctx.Citizenships[*e.CitizenshipID]
		}
	case "employee.passport_series_number":
		return documentValue(bctx, derefStr(e.PassportSeriesNumber))
	case "employee.patent_number":
		return documentValue(bctx, derefStr(e.PatentNumber))
	case "employee.other_permission":
		return documentValue(bctx, derefStr(e.OtherPermission))
	case "employee.target_tables":
		return strings.Join(bctx.EmployeeTargetTables[e.ID], ", ")
	}
	return ""
}

// documentMask - что стоит в ячейке документа вместо значения, когда выгрузка
// документов закрыта правом. Прочерк, а не пустота: пустая ячейка читается как
// «человек не предъявил документ», и охрана на проходной поймёт её именно так.
// Тире длинное: в печатной форме короткий дефис теряется рядом с цифрами соседних
// столбцов и читается как случайный символ, а не как «сведений нет».
const documentMask = "—"

// documentValue отдаёт значение поля из раздела «Документы» карточки либо прочерк,
// если бланк собирается без документов. Прочерк ставится и там, где поле не
// заполнено: иначе пустая ячейка рядом с прочерками сама сообщала бы, что паспорта
// у человека нет, - а это ровно то сведение, которое закрытый режим и прячет.
func documentValue(bctx *BlankContext, value string) string {
	if !bctx.IncludeDocuments {
		return documentMask
	}
	return value
}

func resolveItem(it *models.Item, path string, rowIdx int) string {
	switch path {
	case "item.row_number":
		return strconv.Itoa(rowIdx + 1)
	case "item.name":
		return derefStr(it.Name)
	case "item.count":
		if it.Count != nil {
			return strconv.Itoa(*it.Count)
		}
	}
	return ""
}

func formatDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", s[:min(len(s), 10)])
	if err != nil {
		return s
	}
	return t.Format("02.01.2006")
}

// formatPhone печатает номер так же, как интерфейс: +7 (XXX) XXX XX-XX.
// Эталон - frontend/src/composables/usePhoneFormat.js. Номер, не похожий на
// российский, возвращается без изменений: в бланке исходная строка полезнее пустоты.
func formatPhone(s string) string {
	digits := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	switch {
	case len(digits) == 11 && digits[0] == '8':
		digits[0] = '7'
	case len(digits) == 10:
		digits = append([]rune{'7'}, digits...)
	}
	if len(digits) != 11 || digits[0] != '7' {
		return s
	}
	d := string(digits)
	return "+" + d[0:1] + " (" + d[1:4] + ") " + d[4:7] + " " + d[7:9] + "-" + d[9:11]
}

func formatTime(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, ":", 3)
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return s
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func joinFullName(last, first, middle string) string {
	parts := []string{}
	for _, p := range []string{last, first, middle} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

func joinShortName(last, first, middle string) string {
	res := last
	if first != "" {
		res += " " + first[:lenRune(first, 1)] + "."
	}
	if middle != "" {
		res += " " + middle[:lenRune(middle, 1)] + "."
	}
	return res
}

// lenRune возвращает количество байт первых n символов utf-8 строки.
func lenRune(s string, n int) int {
	count := 0
	for i := range s {
		if count == n {
			return i
		}
		count++
	}
	return len(s)
}

// formatBlankFilename строит имя файла бланка по формату из спеки #183:
// {attachment_display_name}_{application_number}_{org_with_dashes}_{sender_initials}_{date}.xlsx
func formatBlankFilename(bctx *BlankContext) string {
	att := safeStr(bctx.Attachment.AttachmentDisplayName)
	if att == "" {
		att = safeStr(bctx.Attachment.AttachmentName)
	}
	num := ""
	date := ""
	if bctx.Application != nil {
		num = derefStr(bctx.Application.ApplicationNumber)
		if bctx.Application.SendingDatetime != nil {
			date = bctx.Application.SendingDatetime.Format("02-01-2006")
		}
	}
	if date == "" {
		date = time.Now().Format("02-01-2006")
	}
	org := ""
	if bctx.Organization != nil {
		org = strings.ReplaceAll(bctx.Organization.Name, " ", "-")
	} else if bctx.Company != nil {
		org = strings.ReplaceAll(bctx.Company.Name, " ", "-")
	}
	sender := ""
	if bctx.Sender != nil {
		ln := derefStr(bctx.Sender.LastName)
		fn := derefStr(bctx.Sender.FirstName)
		mn := derefStr(bctx.Sender.MiddleName)
		sender = ln
		if fn != "" {
			sender += fn[:lenRune(fn, 1)] + "."
		}
		if mn != "" {
			sender += mn[:lenRune(mn, 1)] + "."
		}
	}
	name := fmt.Sprintf("%s_%s_%s_%s_%s.xlsx", att, num, org, sender, date)
	return sanitizeFilename(name)
}

func safeStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// sanitizeFilename убирает символы которые могут поломать file headers.
var unsafeFilename = regexp.MustCompile(`[/\\:*?"<>|]`)

func sanitizeFilename(s string) string {
	return unsafeFilename.ReplaceAllString(s, "_")
}
