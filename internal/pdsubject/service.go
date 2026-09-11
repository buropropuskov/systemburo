// Package pdsubject собирает сведения об одном человеке для интерфейса
// администратора (#2356).
//
// Живёт отдельным пакетом, а не в internal/services, не по вкусу: entityarchive уже
// зависит от services ради AuditRecorder, и сервис внутри services замкнул бы цикл
// импорта. Пакет оркестрирует entityarchive и export, своей работы с базой у него
// почти нет.
package pdsubject

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"systemburo/internal/entityarchive"
	"systemburo/internal/export"
	"systemburo/internal/models"
)

// Сведения о субъекте персональных данных через интерфейс (#2356).
//
// Консольная команда server subject решает ту же задачу, но запросы государственных
// органов приходят регулярно, а лезть в консоль сервера под каждый - плохая практика:
// доступ к консоли шире, чем нужно для ответа на письмо. Поэтому сбор и выгрузка
// доступны администратору с правом, а сама механика переиспользуется целиком.

// PDSubjectService - поиск человека и справка о нём для интерфейса.
type Service struct {
	db *gorm.DB
	// exportPath - каталог, куда команда кладёт файлы. Через интерфейс файл отдаётся
	// потоком и на диск не пишется: лишняя копия персональных данных на сервере -
	// это ещё одно место, откуда они могут утечь.
	exportPath string
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Candidate - один человек из поиска по имени: строки склеены по документу, поэтому
// в списке столько пунктов, сколько разных людей, а не сколько строк в базе.
type Candidate struct {
	FullName string `json:"full_name"`
	// RegistryID / EmployeeID - от чего собирать сведения. Записи реестра может не
	// быть вовсе: человек, встречающийся только в заявках, тоже обязан находиться -
	// именно о нём и приходит запрос государственного органа.
	RegistryID      int  `json:"registry_id"`
	EmployeeID      int  `json:"employee_id"`
	RegistryRows    int  `json:"registry_rows"`
	ApplicationRows int  `json:"application_rows"`
	HasDocument     bool `json:"has_document"`
	// Fuzzy - найдено по похожему написанию, а не точному совпадению имени.
	Fuzzy bool `json:"fuzzy"`
	// Чем однофамильцы отличаются друг от друга. Без этого список трёх людей с одним
	// ФИО выглядит как три одинаковые строки.
	Organization string `json:"organization,omitempty"`
	Position     string `json:"position,omitempty"`
	DocumentTail string `json:"document_tail,omitempty"`
}

// Find ищет человека по имени или по номеру документа.
//
// Документ точнее: в запросе государственного органа он есть чаще, чем верное
// написание фамилии, и находит ровно одного человека - без списка однофамильцев.
func (s *Service) Find(ctx context.Context, fio, document string) ([]Candidate, error) {
	if strings.TrimSpace(document) != "" {
		found, err := entityarchive.FindSubjectCandidatesByDocument(ctx, s.db, document)
		if err != nil {
			return nil, err
		}
		return toCandidates(found), nil
	}
	return s.FindByName(ctx, fio)
}

// FindByName ищет кандидатов по имени. Склейка по имени не делается: решает человек.
func (s *Service) FindByName(ctx context.Context, fio string) ([]Candidate, error) {
	parts := strings.Fields(fio)
	if len(parts) < 2 {
		return nil, fmt.Errorf("укажите хотя бы фамилию и имя")
	}
	middle := ""
	if len(parts) > 2 {
		middle = strings.Join(parts[2:], " ")
	}

	found, err := entityarchive.FindSubjectCandidatesByFIO(ctx, s.db, parts[0], parts[1], middle)
	if err != nil {
		return nil, err
	}
	return toCandidates(found), nil
}

// toCandidates переводит находки в ответ интерфейса.
func toCandidates(found []entityarchive.SubjectCandidate) []Candidate {
	out := make([]Candidate, 0, len(found))
	for _, c := range found {
		out = append(out, Candidate{
			FullName:        c.FullName,
			RegistryID:      c.RegistryID,
			EmployeeID:      c.EmployeeID,
			RegistryRows:    c.RegistryRows,
			ApplicationRows: c.ApplicationRows,
			HasDocument:     c.HasDocument,
			Fuzzy:           c.Fuzzy,
			Organization:    c.Organization,
			Position:        c.Position,
			DocumentTail:    c.DocumentTail,
		})
	}
	return out
}

// Section - раздел справки для показа на экране.
type Section struct {
	Title   string     `json:"title"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// ReportResponse - состав сведений о человеке.
type ReportResponse struct {
	Origin   string    `json:"origin"`
	Basis    string    `json:"basis"`
	Sections []Section `json:"sections"`
	Total    int       `json:"total"`
}

// Report собирает сведения о человеке по записи реестра.
func (s *Service) Report(ctx context.Context, registryID, employeeID int) (*ReportResponse, error) {
	rep, err := s.buildReport(ctx, registryID, employeeID)
	if err != nil {
		return nil, err
	}

	resp := &ReportResponse{Origin: rep.Origin, Basis: entityarchive.SubjectProcessingBasis()}
	for _, s := range rep.Sections {
		// Пустой срез, а не nil: nil уезжает в JSON как null, и экран падает на
		// rows.length у раздела без строк - раздел «Заявки» у человека без заявок
		// ронял всю страницу (поймано ручной проверкой на стенде).
		rows := s.Rows
		if rows == nil {
			rows = [][]string{}
		}
		resp.Sections = append(resp.Sections, Section{
			Title: s.Title, Headers: s.Headers, Rows: rows,
		})
		resp.Total += len(rows)
	}
	return resp, nil
}

// ExportRequest - выгрузка справки файлом. Получатель и реквизиты запроса
// обязательны: без них выдачу нечем обосновать перед проверяющим.
type ExportRequest struct {
	// Указывают одно из двух: запись реестра или строку заявки (у человека без
	// записи реестра второй путь единственный).
	RegistryID int    `json:"registry_id" validate:"omitempty,gt=0"`
	EmployeeID int    `json:"employee_id" validate:"omitempty,gt=0"`
	Format     string `json:"format" validate:"omitempty,oneof=xlsx pdf"`
	Recipient  string `json:"recipient" validate:"required,max=300"`
	RequestRef string `json:"request_ref" validate:"required,max=300"`
	Basis      string `json:"basis" validate:"omitempty,max=1000"`
}

// Export собирает справку, пишет выдачу в журнал и возвращает файл потоком.
//
// Порядок важен: сперва журнал, потом файл. Выдача, не попавшая в журнал, при
// проверке неотличима от утечки, поэтому файла без записи не бывает.
func (s *Service) Export(ctx context.Context, req ExportRequest, actor Actor) ([]byte, string, string, error) {
	target, err := s.resolveTarget(ctx, req.RegistryID, req.EmployeeID)
	if err != nil {
		return nil, "", "", err
	}
	rep, err := entityarchive.BuildSubjectReport(ctx, s.db, target)
	if err != nil {
		return nil, "", "", err
	}

	format := req.Format
	if format == "" {
		format = "xlsx"
	}
	var data []byte
	var mime, ext string
	if format == "pdf" {
		data, err = export.ToPDFMulti(rep.Sections)
		mime, ext = export.MIMEPDF, "pdf"
	} else {
		data, err = export.ToXLSXMulti(rep.Sections)
		mime, ext = export.MIMEXLSX, "xlsx"
	}
	if err != nil {
		return nil, "", "", fmt.Errorf("сборка справки: %w", err)
	}

	name := fmt.Sprintf("Сведения_о_субъекте_%s.%s", rep.MadeAt.Format("20060102-150405"), ext)
	issuedBy := actor.Username
	var actorID *int
	if actor.UserID > 0 {
		id := actor.UserID
		actorID = &id
	}
	if strings.TrimSpace(issuedBy) == "" {
		issuedBy = "интерфейс"
	}
	_, err = entityarchive.RecordDisclosure(ctx, s.db, target, rep, []string{name},
		entityarchive.DisclosureRequest{
			Recipient: req.Recipient, RequestRef: req.RequestRef, Basis: req.Basis,
			IssuedBy: issuedBy, IssuedByUserID: actorID,
		})
	if err != nil {
		return nil, "", "", err
	}
	return data, name, mime, nil
}

// Actor - кто выдаёт сведения через интерфейс. Попадает в журнал выдач: запись
// «выдал интерфейс» ничего не доказывает, проверяющему нужен человек.
type Actor struct {
	UserID   int
	Username string
}

// Disclosures - журнал выдач: весь или по одному человеку.
func (s *Service) Disclosures(ctx context.Context, registryID, employeeID, limit int) ([]models.PDDisclosure, error) {
	key := ""
	if registryID > 0 || employeeID > 0 {
		target, err := s.resolveTarget(ctx, registryID, employeeID)
		if err != nil {
			return nil, err
		}
		key = entityarchive.DisclosureSubjectKey(target)
	}
	return entityarchive.ListDisclosures(ctx, s.db, key, limit)
}

// resolveTarget - цель по записи реестра ИЛИ по строке заявки. Второе нужно тем, у
// кого записи реестра нет вовсе: человек живёт только в заявке, а ответить о нём
// государственному органу всё равно обязаны.
func (s *Service) resolveTarget(ctx context.Context, registryID, employeeID int) (entityarchive.SubjectTarget, error) {
	switch {
	case registryID > 0:
		target, err := entityarchive.SubjectTargetFromRegistry(ctx, s.db, registryID)
		if err != nil {
			return target, err
		}
		if target.Empty() {
			return target, fmt.Errorf(
				"у записи %d нет ни паспорта, ни патента: собрать сведения о человеке не по чему", registryID)
		}
		return target, nil
	case employeeID > 0:
		target, err := entityarchive.SubjectTargetFromEmployee(ctx, s.db, employeeID)
		if err != nil {
			return target, err
		}
		if target.Empty() {
			return target, fmt.Errorf(
				"у строки заявки %d нет ни паспорта, ни патента: собрать сведения не по чему", employeeID)
		}
		return target, nil
	default:
		return entityarchive.SubjectTarget{}, fmt.Errorf("не указано, о ком собирать сведения")
	}
}

// buildReport - общий шаг Report и Export.
func (s *Service) buildReport(ctx context.Context, registryID, employeeID int) (entityarchive.SubjectReport, error) {
	target, err := s.resolveTarget(ctx, registryID, employeeID)
	if err != nil {
		return entityarchive.SubjectReport{}, err
	}
	return entityarchive.BuildSubjectReport(ctx, s.db, target)
}
