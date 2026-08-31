package services

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Заметка бюро по заявке: принимающий оставляет себе и коллегам объяснение, почему
// заявка не сделана и что по ней осталось. Одна заметка на заявку, общая для всех
// принимающих - переключателя «себе / всем» нет по решению владельца.
//
// Видимость - самое важное здесь, и держится она на трёх запретах:
//
//  1. Поля модели закрыты json:"-" (см. models.Application) - случайная сериализация
//     заявки целиком заметку не вынесет.
//  2. Текст не пишется в audit_log под entity_type='application': GetApplicationHistory
//     отдаёт ленту заявки всем, кто проходит CanAccessApplication, включая ЗАЯВИТЕЛЯ.
//     Заметка в ленте означала бы её показ тому, от кого её прячут. Отдельного типа
//     сущности тоже не заводим: он положил бы копию текста во вторую таблицу, которую
//     читают другие разделы, - лишняя поверхность утечки ради следа, который и так есть
//     в bureau_note_author_id / bureau_note_updated_at.
//  3. В деталь заявки заметка попадает только принимающему (GetApplicationDetails).
//
// Уведомлений по заметке нет: она не событие заявки, а рабочий стикер бюро.

// BureauNoteMaxLen - предел длины заметки. Колонка text, предела на уровне базы нет,
// но принимать безразмерное тело от клиента нельзя: заметка - короткое напоминание,
// а не переписка, для длинных обсуждений есть вопросы к заявке и ветка пересылок.
const BureauNoteMaxLen = 2000

// BureauNoteView - заметка бюро в ответе API.
type BureauNoteView struct {
	Text       string     `json:"text"`
	AuthorID   *int       `json:"author_id"`
	AuthorName string     `json:"author_name"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

// SetBureauNoteRequest - тело PUT /applications/:id/bureau-note. Пустой текст
// (в том числе из одних пробелов) снимает заметку.
type SetBureauNoteRequest struct {
	Note string `json:"note"`
}

// loadBureauNote читает заметку заявки вместе с именем последнего правившего.
// nil - заметки нет (никогда не было или её сняли).
//
// Имя автора берётся без маски принимающего (loadApproverMasks): она существует, чтобы
// ЗАЯВИТЕЛЬ не узнал, кто именно вёл его заявку, а заметку видят только сами принимающие,
// и «Принял(-а) в работу» вместо фамилии коллеги отвечало бы не на тот вопрос. Маска
// несогласившегося на обработку ПД (loadConsentMasks) применяется: она про раскрытие
// персональных данных кому бы то ни было, и коллеги тут не исключение.
func (s *applicationService) loadBureauNote(ctx context.Context, applicationID int) (*BureauNoteView, error) {
	var row struct {
		BureauNote          *string    `gorm:"column:bureau_note"`
		BureauNoteAuthorID  *int       `gorm:"column:bureau_note_author_id"`
		BureauNoteUpdatedAt *time.Time `gorm:"column:bureau_note_updated_at"`
		AuthorName          *string    `gorm:"column:author_name"`
	}

	err := s.db.WithContext(ctx).Table("applications a").
		Select(`a.bureau_note, a.bureau_note_author_id, a.bureau_note_updated_at,
			format_full_name(au.last_name, au.first_name, au.middle_name) as author_name`).
		Joins("LEFT JOIN users au ON a.bureau_note_author_id = au.id").
		Where("a.id = ?", applicationID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		slog.Error("Ошибка чтения заметки бюро", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	if row.BureauNote == nil || strings.TrimSpace(*row.BureauNote) == "" {
		return nil, nil
	}

	name := ""
	if row.AuthorName != nil {
		name = *row.AuthorName
	}
	// Справочник масок тянем, только когда автор известен: без него maskName всё равно
	// вернёт имя как есть, а запрос настроек и списка несогласившихся уже уйдёт.
	if row.BureauNoteAuthorID != nil {
		name = maskName(loadConsentMasks(ctx, s.db), row.BureauNoteAuthorID, name)
	}

	return &BureauNoteView{
		Text:       *row.BureauNote,
		AuthorID:   row.BureauNoteAuthorID,
		AuthorName: name,
		UpdatedAt:  row.BureauNoteUpdatedAt,
	}, nil
}

// SetBureauNote сохраняет или снимает заметку бюро по заявке. Пустой текст снимает её
// вместе с автором и временем - иначе снятая заметка оставляла бы в карточке строку
// «изменил такой-то тогда-то» без самой заметки.
//
// Гейт - роль принимающего, а не право: принимающие заведены отдельным справочником
// (models.ApplicationApprover), права под них нет, и так же гейтится соседний
// take-to-work. Супер-администратор БЕЗ этой роли заметку не сохранит и не увидит -
// см. комментарий к applyBureauNoteVisibility.
func (s *applicationService) SetBureauNote(ctx context.Context, username string, applicationID int, req SetBureauNoteRequest) (*BureauNoteView, error) {
	// Архивная заявка доступна только для чтения - общий запрет системы, заметка не
	// исключение. Отозванную, наоборот, комментировать можно: checkNotWithdrawn закрывает
	// рабочие и согласовательные действия, а объяснение «почему ничего не вышло» на
	// отозванной заявке нужно ровно так же, как на любой другой.
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}

	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !isApprover {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Заметку бюро ведут только принимающие")
	}

	text := strings.TrimSpace(req.Note)
	if len([]rune(text)) > BureauNoteMaxLen {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Заметка слишком длинная")
	}

	updates := map[string]interface{}{
		"bureau_note":            gorm.Expr("NULL"),
		"bureau_note_author_id":  gorm.Expr("NULL"),
		"bureau_note_updated_at": gorm.Expr("NULL"),
	}
	if text != "" {
		updates = map[string]interface{}{
			"bureau_note":            text,
			"bureau_note_author_id":  user.ID,
			"bureau_note_updated_at": time.Now().UTC(),
		}
	}

	res := s.db.WithContext(ctx).Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates)
	if res.Error != nil {
		slog.Error("Ошибка сохранения заметки бюро", "application_id", applicationID, "error", res.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if res.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	return s.loadBureauNote(ctx, applicationID)
}

// applyBureauNoteVisibility кладёт заметку в ответ детали заявки, если смотрящий -
// принимающий, и не кладёт ключ вовсе, если нет. Отсутствие ключа, а не null: у того,
// кому заметка не положена, ответ не должен даже намекать, что она бывает.
//
// Супер-администратор без роли принимающего заметку НЕ видит, хотя CanAccessApplication
// пускает его первым. Три причины. Требование владельца буквальное: заметку видят
// принимающие, остальные нет, и супер-администратор без этой роли - «остальные».
// Фронт считает признак так же строго (ответ /application-approvers/me про себя), поэтому
// послабление на бэке дало бы расхождение: API отдаёт текст, интерфейс его не рисует, и
// заметка живёт только в сетевой панели. И наконец, супер-администратору, которому
// заметка понадобилась по делу, ничего не мешает завести себя принимающим - это его
// собственный раздел, и такой заход оставляет след в audit_log[approver], в отличие от
// тихого сквозного доступа.
func applyBureauNoteVisibility(response map[string]interface{}, note *BureauNoteView, viewerIsApprover bool) {
	if !viewerIsApprover {
		return
	}
	if note == nil {
		response["bureau_note"] = nil
		return
	}
	response["bureau_note"] = note
}
