package services

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Роли участника заявки - машинные ключи. Человекочитаемую подпись рисует фронт:
// иначе переименование бейджа стало бы правкой бэкенда и сменой формы ответа.
//
// Осторожно с парой acceptor/approver - в этом проекте она перепутана исторически.
// Принимающий (взял заявку в работу, applications.responsible_user_id, справочник
// application_approvers) - это acceptor. Согласующий (голосует за заявку, строка в
// application_responsible_users) - approver.
//
// Обязательность голоса - НЕ отдельная роль, а признак required_approval. Карточка
// заявки давно называет всю эту таблицу «ответственными за согласование» и метит
// обязательных отдельной подписью; отдельная роль для необязательного разводила бы
// два интерфейса об одних и тех же людях (человек видел в справочнике организации
// двух согласующих, а в списке участников - «согласующего» и «ответственного»).
const (
	ParticipantRoleSender   = "sender"
	ParticipantRoleAcceptor = "acceptor"
	ParticipantRoleApprover = "approver"
	ParticipantRoleReader   = "reader"
)

// participantRoleRank - старшинство ролей: от того, кто заявку породил, к тому, кто
// её только читает. Задаёт и порядок списка, и выбор primary_role у человека сразу в
// нескольких ролях.
var participantRoleRank = map[string]int{
	ParticipantRoleSender:   0,
	ParticipantRoleAcceptor: 1,
	ParticipantRoleApprover: 2,
	ParticipantRoleReader:   3,
}

// ApplicationParticipant - участник заявки с ролями и контактами.
//
// Один человек - одна запись, даже когда ролей у него несколько (автор заявки часто
// оказывается ещё и согласующим). Повторяющиеся записи фронт всё равно схлопывал бы
// сам: панель рисует по одной строке на человека с одним бейджем, а два одинаковых
// ФИО подряд читаются как ошибка данных. Полный набор лежит в Roles (отсортирован по
// старшинству), бейдж берётся из PrimaryRole - это Roles[0].
type ApplicationParticipant struct {
	UserID     int     `json:"user_id"`
	Username   string  `json:"username"`
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
	// FullName - готовая строка «Фамилия Имя Отчество» (format_full_name). У скрытого
	// работника пуста, у принимающего с заданной маской содержит маску.
	FullName         string  `json:"full_name"`
	Position         *string `json:"position"`
	OrganizationID   *int    `json:"organization_id"`
	OrganizationName *string `json:"organization_name"`
	CompanyID        *int    `json:"company_id"`
	CompanyName      *string `json:"company_name"`
	Email            *string `json:"email"`
	Phone            *string `json:"phone"`
	// Roles - все роли человека в этой заявке, PrimaryRole - старшая из них.
	Roles       []string `json:"roles"`
	PrimaryRole string   `json:"primary_role"`
	// RequiredApproval - голос этого согласующего обязателен для исхода заявки.
	// Карточка заявки метит таких подписью «Обязательно», список участников - тоже.
	RequiredApproval bool `json:"required_approval"`
	// Состояние голоса согласующего - и обязательного, и остальных: голосуют все, у
	// кого есть строка в application_responsible_users, просто необязательный голос
	// на исход не влияет. Прятать его у необязательного значило бы показывать в
	// списке участников не то, что показывает карточка заявки.
	ApprovalStatus   *string    `json:"approval_status"`
	ApprovalComment  *string    `json:"approval_comment"`
	ApprovalDatetime *time.Time `json:"approval_datetime"`
	// PDHidden - работник не дал согласия на обработку персональных данных (#1567):
	// ФИО и контакты скрыты. Без признака интерфейс не отличит «скрыто» от «не заполнено».
	PDHidden bool `json:"pd_hidden"`
}

// participantRow - плоский приёмник строки запроса. Поля перечислены плоско намеренно:
// анонимно встроенная структура в Scan молча не маппится и приходит нулями.
type participantRow struct {
	UserID           int        `gorm:"column:user_id"`
	Role             string     `gorm:"column:role"`
	RequiredApproval bool       `gorm:"column:required_approval"`
	ApprovalStatus   *string    `gorm:"column:approval_status"`
	ApprovalComment  *string    `gorm:"column:approval_comment"`
	ApprovalDatetime *time.Time `gorm:"column:approval_datetime"`
	Username         string     `gorm:"column:username"`
	LastName         *string    `gorm:"column:last_name"`
	FirstName        *string    `gorm:"column:first_name"`
	MiddleName       *string    `gorm:"column:middle_name"`
	FullName         string     `gorm:"column:full_name"`
	Position         *string    `gorm:"column:position"`
	OrganizationID   *int       `gorm:"column:organization_id"`
	OrganizationName *string    `gorm:"column:organization_name"`
	CompanyID        *int       `gorm:"column:company_id"`
	CompanyName      *string    `gorm:"column:company_name"`
	Email            *string    `gorm:"column:email"`
	Phone            *string    `gorm:"column:phone"`
}

// participantsQuery собирает роли из четырёх источников одним UNION ALL и добирает к
// ним карточку человека. Отдельными запросами вышло бы пять походов в базу и пять
// разных способов не забыть про маскировку.
//
// Типы у NULL-колонок проставлены явно: тип столбца UNION выводится слева направо, и
// нетипизированный NULL в первой ветке успевает стать text - дальше ветка с настоящим
// timestamptz уже не сходится с ним и запрос падает.
const participantsQuery = `
WITH participants AS (
	SELECT a.sender_user_id AS user_id, 'sender'::text AS role,
	       false AS required_approval,
	       NULL::text AS approval_status,
	       NULL::text AS approval_comment,
	       NULL::timestamptz AS approval_datetime
	FROM applications a
	WHERE a.id = ?
	UNION ALL
	SELECT a.responsible_user_id, 'acceptor'::text, false, NULL::text, NULL::text, NULL::timestamptz
	FROM applications a
	WHERE a.id = ? AND a.responsible_user_id IS NOT NULL
	UNION ALL
	SELECT aru.user_id, 'approver'::text,
	       COALESCE(aru.required_approval, false),
	       aru.approval_status::text, aru.approval_comment::text, aru.approval_datetime
	FROM application_responsible_users aru
	WHERE aru.application_id = ?
	UNION ALL
	SELECT av.user_id, 'reader'::text, false, NULL::text, NULL::text, NULL::timestamptz
	FROM application_viewers av
	WHERE av.application_id = ?
	UNION ALL
	-- Принимающие: реестр не привязан к заявке, заявку видит любой из них и любой
	-- может взять её в работу. Пока никто не взял, responsible_user_id пуст, и без
	-- этой ветки заявитель видел в получателях только себя.
	SELECT aa.user_id, 'acceptor'::text, false, NULL::text, NULL::text, NULL::timestamptz
	FROM application_approvers aa
	JOIN users au ON au.id = aa.user_id
	WHERE au.is_active AND NOT au.is_banned AND NOT au.is_super_admin
)
SELECT
	p.user_id,
	p.role,
	p.required_approval,
	p.approval_status,
	p.approval_comment,
	p.approval_datetime,
	u.username,
	u.last_name,
	u.first_name,
	u.middle_name,
	format_full_name(u.last_name, u.first_name, u.middle_name) AS full_name,
	u.position,
	u.organization_id,
	o.name AS organization_name,
	u.company_id,
	c.name AS company_name,
	u.email,
	u.phone
FROM participants p
JOIN users u ON u.id = p.user_id
LEFT JOIN organizations o ON o.id = u.organization_id
LEFT JOIN companies c ON c.id = u.company_id
`

// GetApplicationParticipants возвращает всех участников заявки с ролями и контактами.
func (s *applicationService) GetApplicationParticipants(ctx context.Context, applicationID int) ([]ApplicationParticipant, error) {
	var exists int64
	if err := s.db.WithContext(ctx).Table("applications").
		Where("id = ?", applicationID).Count(&exists).Error; err != nil {
		slog.Error("не удалось проверить существование заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if exists == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	var rows []participantRow
	if err := s.db.WithContext(ctx).
		Raw(participantsQuery, applicationID, applicationID, applicationID, applicationID).
		Scan(&rows).Error; err != nil {
		slog.Error("не удалось получить участников заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching application participants")
	}

	result := mergeParticipantRows(rows)
	maskParticipants(ctx, s.db, result)
	sortParticipants(result)
	return result, nil
}

// mergeParticipantRows схлопывает строки одного человека в одну запись с набором ролей.
func mergeParticipantRows(rows []participantRow) []ApplicationParticipant {
	result := make([]ApplicationParticipant, 0, len(rows))
	index := make(map[int]int, len(rows))

	for _, r := range rows {
		pos, seen := index[r.UserID]
		if !seen {
			pos = len(result)
			index[r.UserID] = pos
			result = append(result, ApplicationParticipant{
				UserID:           r.UserID,
				Username:         r.Username,
				LastName:         r.LastName,
				FirstName:        r.FirstName,
				MiddleName:       r.MiddleName,
				FullName:         r.FullName,
				Position:         r.Position,
				OrganizationID:   r.OrganizationID,
				OrganizationName: r.OrganizationName,
				CompanyID:        r.CompanyID,
				CompanyName:      r.CompanyName,
				Email:            r.Email,
				Phone:            r.Phone,
				Roles:            make([]string, 0, 2),
			})
		}
		p := &result[pos]
		// Роль добавляем один раз: принимающий приходит двумя строками - из реестра
		// принимающих и из responsible_user_id взятой в работу заявки, - и без проверки
		// в наборе ролей появлялось бы два одинаковых значения.
		if !slices.Contains(p.Roles, r.Role) {
			p.Roles = append(p.Roles, r.Role)
		}
		if r.Role == ParticipantRoleApprover {
			p.RequiredApproval = r.RequiredApproval
			p.ApprovalStatus = r.ApprovalStatus
			p.ApprovalComment = r.ApprovalComment
			p.ApprovalDatetime = r.ApprovalDatetime
		}
	}

	for i := range result {
		sort.SliceStable(result[i].Roles, func(a, b int) bool {
			return participantRoleRank[result[i].Roles[a]] < participantRoleRank[result[i].Roles[b]]
		})
		result[i].PrimaryRole = result[i].Roles[0]
	}
	return result
}

// maskParticipants прячет персональные данные тех, чьё имя показывать нельзя.
//
// Два независимых источника скрытия:
//   - работник не дал согласия на обработку ПД (#1567) - ФИО и контакты убираем, ставим
//     pd_hidden. Почта и телефон здесь не менее чувствительны, чем фамилия: рабочий адрес
//     вида i.ivanov@ и есть фамилия, и отдавать его, пряча поле «Фамилия», бессмысленно;
//   - принимающему администратор задал отображаемое имя - показываем маску вместо ФИО и
//     тоже убираем контакты: маска ставится ровно затем, чтобы заявитель не знал, кто
//     именно взял заявку, а личная почта эту анонимность снимает. Маска в приоритете -
//     она задана осознанно и персональные данные не раскрывает (см. loadNameMasks).
func maskParticipants(ctx context.Context, db *gorm.DB, participants []ApplicationParticipant) {
	consentMasks := loadConsentMasks(ctx, db)
	approverMasks := loadApproverMasks(ctx, db)
	if len(consentMasks) == 0 && len(approverMasks) == 0 {
		return
	}

	for i := range participants {
		p := &participants[i]
		if isMasked(consentMasks, p.UserID) {
			maskUserParts(consentMasks, p.UserID, &p.LastName, &p.FirstName, &p.MiddleName)
			maskUserContacts(consentMasks, p.UserID, &p.Email, &p.Phone)
			p.FullName = ""
			p.PDHidden = true
		}
		if display, ok := approverMasks[p.UserID]; ok {
			p.LastName, p.FirstName, p.MiddleName = nil, nil, nil
			p.Email, p.Phone = nil, nil
			p.FullName = display
		}
	}
}

// sortParticipants упорядочивает список так, как его читает человек: сверху автор, ниже
// принявший, согласующие, ответственные и читатели.
func sortParticipants(participants []ApplicationParticipant) {
	sort.SliceStable(participants, func(i, j int) bool {
		ri, rj := participantRoleRank[participants[i].PrimaryRole], participantRoleRank[participants[j].PrimaryRole]
		if ri != rj {
			return ri < rj
		}
		ki, kj := participantSortKey(participants[i]), participantSortKey(participants[j])
		if ki != kj {
			return ki < kj
		}
		return participants[i].UserID < participants[j].UserID
	})
}

// participantSortKey - строка, по которой список выглядит упорядоченным для человека.
// Сортируем по видимому имени, а не по настоящему: порядок по скрытой фамилии выдал бы
// её первую букву - скрытый работник встал бы на своё алфавитное место между однофамильцами.
func participantSortKey(p ApplicationParticipant) string {
	if p.FullName == "" {
		return "￿" + p.Username
	}
	return p.FullName
}
