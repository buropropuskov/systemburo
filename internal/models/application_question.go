package models

import "time"

// ApplicationQuestion - вопрос-топик к заявке (Q&A #973). Адресат - инициатор заявки;
// тред ответов лежит в application_answers. Создание вопроса пишется в историю заявки
// (audit_log, action question_created); ответы в историю не пишутся.
type ApplicationQuestion struct {
	ID            int         `json:"id"`
	ApplicationID int         `gorm:"not null;index:idx_app_question,priority:1" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	AuthorUserID  int         `gorm:"not null;index" json:"author_user_id"`
	Author        User        `gorm:"foreignKey:AuthorUserID;constraint:OnDelete:CASCADE" json:"-"`
	Subject       string      `gorm:"size:150;not null" json:"subject"`
	Text          string      `gorm:"type:text;not null" json:"text"`
	// created_at ставит БД (now()), а не Go: маркер сравнивает его с last_seen_at
	// (тоже now() в той же tx) - при seen-on-post оба равны, свой вопрос не светит маркер.
	CreatedAt time.Time `gorm:"autoCreateTime:false;default:now();index:idx_app_question,priority:2" json:"created_at"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationQuestion) TableName() string { return "application_questions" }

// ApplicationAnswer - ответ в треде вопроса (#973). ApplicationID денормализован от
// родительского вопроса, чтобы маркер "новые вопросы/ответы" в списке заявок считался
// плоским EXISTS без join answers->questions; дрейфа нет (ставится при вставке, гибнет с
// вопросом по question_id CASCADE).
type ApplicationAnswer struct {
	ID            int                 `json:"id"`
	QuestionID    int                 `gorm:"not null;index:idx_answer_question,priority:1" json:"question_id"`
	Question      ApplicationQuestion `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ApplicationID int                 `gorm:"not null;index" json:"application_id"`
	AuthorUserID  int                 `gorm:"not null;index" json:"author_user_id"`
	Author        User                `gorm:"foreignKey:AuthorUserID;constraint:OnDelete:CASCADE" json:"-"`
	Text          string              `gorm:"type:text;not null" json:"text"`
	// created_at ставит БД (now()) - согласовано с last_seen_at для маркера (см. вопрос).
	CreatedAt time.Time `gorm:"autoCreateTime:false;default:now();index:idx_answer_question,priority:2" json:"created_at"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationAnswer) TableName() string { return "application_answers" }

// ApplicationQuestionAttachment - вложения заявки, по которым задан вопрос (#973).
// Join-таблица (как ForwardAttachment): FK-целостность вместо jsonb со стухшими id,
// актуальные имена резолвятся при чтении. Пусто - вопрос по всей заявке.
type ApplicationQuestionAttachment struct {
	ID           int                 `json:"id"`
	QuestionID   int                 `gorm:"uniqueIndex:idx_q_att,priority:1" json:"question_id"`
	Question     ApplicationQuestion `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	AttachmentID int                 `gorm:"uniqueIndex:idx_q_att,priority:2" json:"attachment_id"`
	Attachment   Attachment          `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationQuestionAttachment) TableName() string { return "application_question_attachments" }

// ApplicationQuestionView - per-user отметка последнего просмотра Q&A заявки (#973) для
// маркера "новые вопросы/ответы". Бинарный application_reads не подходит (залипает после
// первого открытия); last_seen_at обновляется при каждом открытии панели вопросов.
type ApplicationQuestionView struct {
	ID            int         `json:"id"`
	ApplicationID int         `gorm:"uniqueIndex:idx_app_user_qview,priority:1" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID        int         `gorm:"uniqueIndex:idx_app_user_qview,priority:2" json:"user_id"`
	User          User        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	LastSeenAt    time.Time   `json:"last_seen_at"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationQuestionView) TableName() string { return "application_question_views" }

// ApplicationQuestionRead - per-user отметка прочтения КОНКРЕТНОГО вопроса-топика (#973).
// В отличие от ApplicationQuestionView (одна метка на заявку - временная граница), read_at
// живёт на топик: непрочитанный топик остаётся новым независимо от других (пользователь
// "остановился на одном, а третье забыл"). Топик новый, если вопрос или его ответ созданы
// позже read_at (или отметки нет). Ставится при взаимодействии с топиком.
type ApplicationQuestionRead struct {
	ID         int                 `json:"id"`
	QuestionID int                 `gorm:"uniqueIndex:idx_q_read,priority:1" json:"question_id"`
	Question   ApplicationQuestion `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID     int                 `gorm:"uniqueIndex:idx_q_read,priority:2" json:"user_id"`
	User       User                `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	ReadAt     time.Time           `json:"read_at"`
}

// TableName задаёт имя таблицы явно.
func (ApplicationQuestionRead) TableName() string { return "application_question_reads" }
