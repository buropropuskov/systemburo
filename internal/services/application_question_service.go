package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// --- DTO Q&A (#973) ---

// CreateQuestionRequest - тело создания вопроса-топика.
type CreateQuestionRequest struct {
	Subject       string `json:"subject" validate:"required,max=150"`
	Text          string `json:"text" validate:"required,max=5000"`
	AttachmentIDs []int  `json:"attachment_ids"`
}

// CreateAnswerRequest - тело ответа в треде.
type CreateAnswerRequest struct {
	Text string `json:"text" validate:"required,max=5000"`
}

// QuestionAttachmentItem - вложение заявки, по которому задан вопрос (имя резолвится при чтении).
type QuestionAttachmentItem struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
}

// AnswerItem - один ответ в треде.
type AnswerItem struct {
	ID           int       `json:"id"`
	QuestionID   int       `json:"question_id"`
	AuthorUserID int       `json:"author_user_id"`
	AuthorName   string    `json:"author_name"`
	Text         string    `json:"text"`
	CreatedAt    time.Time `json:"created_at"`
}

// QuestionWithAnswers - вопрос-топик с вложенным тредом ответов и вложениями.
type QuestionWithAnswers struct {
	ID            int                      `json:"id"`
	ApplicationID int                      `json:"application_id"`
	AuthorUserID  int                      `json:"author_user_id"`
	AuthorName    string                   `json:"author_name"`
	Subject       string                   `json:"subject"`
	Text          string                   `json:"text"`
	Attachments   []QuestionAttachmentItem `json:"attachments"`
	CreatedAt     time.Time                `json:"created_at"`
	Answers       []AnswerItem             `json:"answers"`
	// IsNew - в топике есть непрочитанное для смотрящего (#973): вопрос или его ответ созданы
	// позже read_at топика (или отметки прочтения нет), автор события != смотрящий.
	IsNew bool `json:"is_new"`
}

const questionAuthorName = `format_full_name(u.last_name, u.first_name, u.middle_name)`

// GetApplicationQuestions возвращает вопросы к заявке (#973) с ответами и вложениями.
// forwardViewerID - id для фильтра пер-вложенного пересыла (#680): 0 = супер-админ (видит все).
// readerUserID - РЕАЛЬНЫЙ id смотрящего для флага IsNew (у супера тоже реальный, иначе его
// прочтения по user_id=0 не нашлись бы).
func (s *applicationService) GetApplicationQuestions(ctx context.Context, applicationID, forwardViewerID, readerUserID int) ([]QuestionWithAnswers, error) {
	type qRow struct {
		ID           int
		AuthorUserID int
		AuthorName   string
		Subject      string
		Text         string
		CreatedAt    time.Time
	}
	var qRows []qRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT q.id, q.author_user_id, `+questionAuthorName+` AS author_name, q.subject, q.text, q.created_at
		FROM application_questions q
		JOIN users u ON u.id = q.author_user_id
		WHERE q.application_id = ?
		ORDER BY q.created_at DESC, q.id DESC
	`, applicationID).Scan(&qRows).Error; err != nil {
		slog.Error("Ошибка получения вопросов заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching questions")
	}

	questions := make([]QuestionWithAnswers, 0, len(qRows))
	if len(qRows) == 0 {
		return questions, nil
	}

	// Логин вместо ФИО у авторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)

	ids := make([]int, 0, len(qRows))
	idx := make(map[int]int, len(qRows))
	for i, q := range qRows {
		questions = append(questions, QuestionWithAnswers{
			ID:            q.ID,
			ApplicationID: applicationID,
			AuthorUserID:  q.AuthorUserID,
			AuthorName:    maskName(masks, &q.AuthorUserID, q.AuthorName),
			Subject:       q.Subject,
			Text:          q.Text,
			Attachments:   []QuestionAttachmentItem{},
			CreatedAt:     q.CreatedAt,
			Answers:       []AnswerItem{},
		})
		ids = append(ids, q.ID)
		idx[q.ID] = i
	}

	// Ответы треда (хронологический порядок).
	type aRow struct {
		ID           int
		QuestionID   int
		AuthorUserID int
		AuthorName   string
		Text         string
		CreatedAt    time.Time
	}
	var aRows []aRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT a.id, a.question_id, a.author_user_id, `+questionAuthorName+` AS author_name, a.text, a.created_at
		FROM application_answers a
		JOIN users u ON u.id = a.author_user_id
		WHERE a.question_id IN ?
		ORDER BY a.created_at ASC, a.id ASC
	`, ids).Scan(&aRows).Error; err != nil {
		slog.Error("Ошибка получения ответов заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching answers")
	}
	for _, a := range aRows {
		if i, ok := idx[a.QuestionID]; ok {
			questions[i].Answers = append(questions[i].Answers, AnswerItem{
				ID:           a.ID,
				QuestionID:   a.QuestionID,
				AuthorUserID: a.AuthorUserID,
				AuthorName:   maskName(masks, &a.AuthorUserID, a.AuthorName),
				Text:         a.Text,
				CreatedAt:    a.CreatedAt,
			})
		}
	}

	// Вложения вопросов (актуальные имена).
	type attRow struct {
		QuestionID   int
		AttachmentID int
		DisplayName  string
	}
	var attRows []attRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT qa.question_id, qa.attachment_id, COALESCE(att.attachment_display_name, att.attachment_name, '') AS display_name
		FROM application_question_attachments qa
		JOIN attachments att ON att.id = qa.attachment_id
		WHERE qa.question_id IN ?
		ORDER BY display_name
	`, ids).Scan(&attRows).Error; err != nil {
		slog.Error("Ошибка получения вложений вопросов", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching question attachments")
	}
	// Пер-вложенный пересыл (#680): читателю с ограничением не показываем имена вложений,
	// которые ему персонально не пересылали.
	allowed, restricted, err := s.resolveForwardFilter(ctx, applicationID, forwardViewerID)
	if err != nil {
		return nil, err
	}
	for _, at := range attRows {
		if restricted && !allowed[at.AttachmentID] {
			continue
		}
		if i, ok := idx[at.QuestionID]; ok {
			questions[i].Attachments = append(questions[i].Attachments, QuestionAttachmentItem{
				ID:          at.AttachmentID,
				DisplayName: at.DisplayName,
			})
		}
	}

	// Флаг новизны по per-топик отметке прочтения (#973). read_at топика читается ДО любого
	// обновления - клик пометки прочтения идёт отдельным эндпоинтом.
	type readRow struct {
		QuestionID int
		ReadAt     time.Time
	}
	var readRows []readRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT question_id, read_at FROM application_question_reads
		WHERE user_id = ? AND question_id IN ?
	`, readerUserID, ids).Scan(&readRows).Error; err != nil {
		slog.Error("Ошибка получения отметок прочтения вопросов", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching question reads")
	}
	readAt := make(map[int]time.Time, len(readRows))
	for _, r := range readRows {
		readAt[r.QuestionID] = r.ReadAt
	}
	for i := range questions {
		questions[i].IsNew = topicHasUnseen(&questions[i], readAt[questions[i].ID], readerUserID)
	}

	return questions, nil
}

// topicHasUnseen: в топике есть непрочитанное для readerUserID - вопрос или любой ответ
// созданы позже read (или отметки нет, read = нулевое время), автор события != смотрящий.
func topicHasUnseen(q *QuestionWithAnswers, read time.Time, readerUserID int) bool {
	if q.AuthorUserID != readerUserID && q.CreatedAt.After(read) {
		return true
	}
	for _, a := range q.Answers {
		if a.AuthorUserID != readerUserID && a.CreatedAt.After(read) {
			return true
		}
	}
	return false
}

// CreateApplicationQuestion создаёт вопрос-топик (#973).
func (s *applicationService) CreateApplicationQuestion(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req CreateQuestionRequest) (*QuestionWithAnswers, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	var app struct {
		ID                int
		SenderUserID      int
		ApplicationNumber string
	}
	if err := s.db.WithContext(ctx).Raw("SELECT id, sender_user_id, application_number FROM applications WHERE id = ?", applicationID).Scan(&app).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if app.ID == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	// Задать вопрос может любой с доступом к заявке, включая инициатора (#973):
	// гейт доступа делает handler через CanAccessApplication.

	subject := strings.TrimSpace(req.Subject)
	text := strings.TrimSpace(req.Text)
	if subject == "" || text == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Тема и текст вопроса обязательны")
	}

	// Только вложения самой заявки - чужие ID отбрасываем (как в forward).
	var validAttIDs []int
	if len(req.AttachmentIDs) > 0 {
		if err := s.db.WithContext(ctx).Raw("SELECT id FROM attachments WHERE application_id = ? AND id IN ?", applicationID, req.AttachmentIDs).Scan(&validAttIDs).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}
		// Пер-вложенный пересыл (#680): автор не может сослаться на вложение, которое ему
		// самому не переслали. Супер-админ (viewerID=0) и sender видят все.
		viewerID := user.ID
		if isSuperAdmin {
			viewerID = 0
		}
		allowed, restricted, err := s.resolveForwardFilter(ctx, applicationID, viewerID)
		if err != nil {
			return nil, err
		}
		if restricted {
			filtered := make([]int, 0, len(validAttIDs))
			for _, id := range validAttIDs {
				if allowed[id] {
					filtered = append(filtered, id)
				}
			}
			validAttIDs = filtered
		}
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	question := models.ApplicationQuestion{
		ApplicationID: applicationID,
		AuthorUserID:  user.ID,
		Subject:       subject,
		Text:          text,
	}
	if err := tx.Create(&question).Error; err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания вопроса", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create question")
	}
	// created_at проставила БД (default now()) - читаем для ответа.
	if err := tx.Raw("SELECT created_at FROM application_questions WHERE id = ?", question.ID).Scan(&question.CreatedAt).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	for _, attID := range validAttIDs {
		if err := tx.Exec("INSERT INTO application_question_attachments (question_id, attachment_id) VALUES (?, ?) ON CONFLICT (question_id, attachment_id) DO NOTHING", question.ID, attID).Error; err != nil {
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to link attachment")
		}
	}

	// История: создание вопроса пишется в audit_log (Record=в tx: провал откатит вопрос).
	meta, _ := json.Marshal(map[string]any{"question_id": question.ID, "subject": subject, "attachment_ids": validAttIDs})
	if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &applicationID, "question_created", &user.ID,
		applicationAuditDetails{Comment: &text, Metadata: meta}); err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to record history")
	}

	// Автор просмотрел свой вопрос -> его маркер по этой заявке не загорается.
	if err := tx.Exec(questionViewUpsert, applicationID, user.ID).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to update seen state")
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("Ошибка коммита вопроса", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Уведомление инициатору о новом вопросе (best-effort).
	if s.notificationService != nil && app.SenderUserID != 0 && app.SenderUserID != user.ID {
		appNum := applicationNumberOrFallback(app.ApplicationNumber, applicationID)
		authorName := formatFullName(user.LastName, user.FirstName, user.MiddleName)
		payloadStr := questionNotificationPayload(applicationID, appNum, question.ID)
		if err := s.notificationService.CreateForUser(ctx, app.SenderUserID, NotificationTypeApplicationQuestion,
			"Новый вопрос по заявке",
			fmt.Sprintf("%s задал(-а) вопрос по заявке %s: %s", authorName, appNum, subject),
			&payloadStr); err != nil {
			slog.Warn("не удалось создать уведомление о вопросе", "user_id", app.SenderUserID, "error", err)
		}
	}

	atts := make([]QuestionAttachmentItem, 0, len(validAttIDs))
	if len(validAttIDs) > 0 {
		type attRow struct {
			ID          int
			DisplayName string
		}
		var rows []attRow
		s.db.WithContext(ctx).Raw("SELECT id, COALESCE(attachment_display_name, attachment_name, '') AS display_name FROM attachments WHERE id IN ? ORDER BY display_name", validAttIDs).Scan(&rows)
		for _, r := range rows {
			atts = append(atts, QuestionAttachmentItem{ID: r.ID, DisplayName: r.DisplayName})
		}
	}

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)

	return &QuestionWithAnswers{
		ID:            question.ID,
		ApplicationID: applicationID,
		AuthorUserID:  user.ID,
		AuthorName:    formatFullName(user.LastName, user.FirstName, user.MiddleName),
		Subject:       subject,
		Text:          text,
		Attachments:   atts,
		CreatedAt:     question.CreatedAt,
		Answers:       []AnswerItem{},
	}, nil
}

// CreateApplicationAnswer добавляет ответ в тред (#973). Историю не пишет; уведомляет
// участников обсуждения (автор вопроса + отвечавшие), кроме автора самого ответа.
func (s *applicationService) CreateApplicationAnswer(ctx context.Context, username string, applicationID, questionID int, req CreateAnswerRequest) (*AnswerItem, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	var q struct {
		ID            int
		ApplicationID int
		Subject       string
	}
	if err := s.db.WithContext(ctx).Raw("SELECT id, application_id, subject FROM application_questions WHERE id = ?", questionID).Scan(&q).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if q.ID == 0 || q.ApplicationID != applicationID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Текст ответа обязателен")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	answer := models.ApplicationAnswer{
		QuestionID:    questionID,
		ApplicationID: applicationID,
		AuthorUserID:  user.ID,
		Text:          text,
	}
	if err := tx.Create(&answer).Error; err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания ответа", "question_id", questionID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create answer")
	}
	if err := tx.Raw("SELECT created_at FROM application_answers WHERE id = ?", answer.ID).Scan(&answer.CreatedAt).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if err := tx.Exec(questionViewUpsert, applicationID, user.ID).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to update seen state")
	}
	// Автор ответа участвовал в топике -> он прочитал его (свой ответ и всё прежнее не светят
	// ему новизну; чужой ответ позже снова пометит топик новым, #973).
	if err := tx.Exec(questionReadUpsert, questionID, user.ID).Error; err != nil {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark question read")
	}
	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Уведомления участникам обсуждения (best-effort): автор вопроса + все ответившие, кроме
	// автора текущего ответа.
	if s.notificationService != nil {
		var recipientIDs []int
		s.db.WithContext(ctx).Raw(`
			SELECT DISTINCT uid FROM (
				SELECT author_user_id AS uid FROM application_questions WHERE id = ?
				UNION
				SELECT author_user_id AS uid FROM application_answers WHERE question_id = ?
			) t WHERE uid <> ?
		`, questionID, questionID, user.ID).Scan(&recipientIDs)

		if len(recipientIDs) > 0 {
			var appNumber string
			s.db.WithContext(ctx).Raw("SELECT application_number FROM applications WHERE id = ?", applicationID).Scan(&appNumber)
			appNum := applicationNumberOrFallback(appNumber, applicationID)
			authorName := formatFullName(user.LastName, user.FirstName, user.MiddleName)
			payloadStr := questionNotificationPayload(applicationID, appNum, questionID)
			for _, rid := range recipientIDs {
				if err := s.notificationService.CreateForUser(ctx, rid, NotificationTypeApplicationAnswer,
					"Новый ответ на вопрос",
					fmt.Sprintf("%s ответил(-а) на вопрос «%s» по заявке %s", authorName, q.Subject, appNum),
					&payloadStr); err != nil {
					slog.Warn("не удалось создать уведомление об ответе", "user_id", rid, "error", err)
				}
			}
		}
	}

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)

	return &AnswerItem{
		ID:           answer.ID,
		QuestionID:   questionID,
		AuthorUserID: user.ID,
		AuthorName:   formatFullName(user.LastName, user.FirstName, user.MiddleName),
		Text:         text,
		CreatedAt:    answer.CreatedAt,
	}, nil
}

// MarkQuestionsSeen обновляет last-seen пользователя по Q&A заявки (#973).
func (s *applicationService) MarkQuestionsSeen(ctx context.Context, username string, applicationID int) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	var appCount int64
	if err := s.db.WithContext(ctx).Model(&models.Application{}).Where("id = ?", applicationID).Count(&appCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if appCount == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	if err := s.db.WithContext(ctx).Exec(questionViewUpsert, applicationID, user.ID).Error; err != nil {
		slog.Error("Ошибка отметки просмотра вопросов", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark questions seen")
	}
	return nil
}

// questionViewUpsert - идемпотентный upsert last-seen (application_id, user_id).
const questionViewUpsert = `INSERT INTO application_question_views (application_id, user_id, last_seen_at)
	VALUES (?, ?, now())
	ON CONFLICT (application_id, user_id) DO UPDATE SET last_seen_at = now()`

// questionReadUpsert - идемпотентный upsert per-топик прочтения (question_id, user_id).
const questionReadUpsert = `INSERT INTO application_question_reads (question_id, user_id, read_at)
	VALUES (?, ?, now())
	ON CONFLICT (question_id, user_id) DO UPDATE SET read_at = now()`

// MarkQuestionRead помечает конкретный вопрос-топик прочитанным смотрящим (#973): гасит его
// новизну (и учтённые в нём ответы) для этого пользователя. Идемпотентно.
func (s *applicationService) MarkQuestionRead(ctx context.Context, username string, applicationID, questionID int) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	var q struct {
		ID            int
		ApplicationID int
	}
	if err := s.db.WithContext(ctx).Raw("SELECT id, application_id FROM application_questions WHERE id = ?", questionID).Scan(&q).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if q.ID == 0 || q.ApplicationID != applicationID {
		return echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}
	if err := s.db.WithContext(ctx).Exec(questionReadUpsert, questionID, user.ID).Error; err != nil {
		slog.Error("Ошибка отметки прочтения вопроса", "question_id", questionID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark question read")
	}
	return nil
}

func applicationNumberOrFallback(number string, applicationID int) string {
	if number == "" {
		return fmt.Sprintf("№ %d", applicationID)
	}
	return number
}

func questionNotificationPayload(applicationID int, appNumber string, questionID int) string {
	payload, _ := json.Marshal(map[string]any{
		"application_id":     applicationID,
		"application_number": appNumber,
		"question_id":        questionID,
	})
	return string(payload)
}
