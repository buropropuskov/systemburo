package models

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/normalize"
)

type User struct {
	ID             int          `json:"id"`
	Username       string       `gorm:"uniqueIndex;size:100" json:"username"`
	Password       string       `gorm:"size:255" json:"-"`
	OrganizationID *int         `json:"organization_id"`
	Organization   Organization `json:"organization,omitempty"`
	CompanyID      *int         `json:"company_id"`
	Company        Company      `json:"company,omitempty"`
	TypeID         int          `gorm:"default:1" json:"type_id"`
	UserType       UserType     `gorm:"foreignKey:TypeID" json:"user_type,omitempty"`
	RoleID         *int         `json:"role_id"`
	Role           *Role        `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	IsSuperAdmin   bool         `gorm:"default:false;index" json:"is_super_admin"`
	// IsAdmin -- администратор (тумблер в карточке). Базово получает все права,
	// кроме super-only ключей и личных deny-override (см. PermissionResolver).
	// Отвязано от user_type: админство задаётся флагом, а не кодом типа.
	IsAdmin     bool       `gorm:"default:false;index" json:"is_admin"`
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
	IsBanned    bool       `gorm:"default:false;index" json:"is_banned"`
	IsImportant bool       `gorm:"default:false" json:"is_important"`
	BannedAt    *time.Time `json:"banned_at,omitempty"`
	BannedBy    *int       `json:"banned_by,omitempty"`
	// BanReason -- текущая причина блокировки (показывается заблокированному в ЛК).
	// Обнуляется при разблокировке; хронология ведётся в UserBanHistory.
	BanReason  *string `gorm:"type:text" json:"ban_reason,omitempty"`
	LastName   *string `gorm:"size:100" json:"last_name"`
	FirstName  *string `gorm:"size:100" json:"first_name"`
	MiddleName *string `gorm:"size:100" json:"middle_name"`
	Position   *string `gorm:"size:100;column:position" json:"position"`
	// Почта и телефон шифруются (#2351): это каналы связи с человеком, и дамп базы
	// не должен отдавать их списком. Тип text, а не размерный: шифротекст длиннее
	// исходного значения.
	//
	// Рядом лежат свёртки. По ним работает то, ради чего поля читали в базе:
	// проверка занятости адреса и точный поиск человека по адресу или номеру.
	// Поиск по ЧАСТИ строки после шифрования невозможен - это цена решения.
	Email       *string    `gorm:"type:text" json:"email"`
	Phone       *string    `gorm:"type:text" json:"phone"`
	EmailHMAC   *string    `gorm:"size:64;index" json:"-"`
	PhoneHMAC   *string    `gorm:"size:64;index" json:"-"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	// LastSeen - момент последней активности (любой authenticated-запрос),
	// обновляется middleware с троттлингом. В отличие от LastLoginAt отражает
	// присутствие "онлайн" между логинами; по нему считается users_online (#632).
	LastSeen         *time.Time `gorm:"index" json:"last_seen,omitempty"`
	FailedLoginCount int        `gorm:"default:0" json:"-"`
	LockedUntil      *time.Time `json:"-"`
	// LockoutLevel - ступень лестницы кулдаунов (0 = блокировок в текущей цепочке
	// не было). Каждая блокировка поднимает ступень, успешный вход и сброс
	// администратором опускают её в ноль; сутки без неудачных попыток - тоже.
	LockoutLevel int `gorm:"default:0" json:"-"`
	// LastFailedLoginAt - момент последней неудачной попытки. Держит окно
	// накопления счётчика и точку отсчёта для затухания ступени.
	LastFailedLoginAt *time.Time `json:"-"`
	// PasswordChangedAt - когда пароль менялся в последний раз (#1907). От неё
	// считается срок действия при плановой смене: график индивидуальный, у каждого
	// свой отсчёт. Существующим учётным записям проставлена дата внедрения, а не
	// дата создания - иначе в день включения ротации истекли бы разом все старые.
	PasswordChangedAt *time.Time `gorm:"index" json:"-"`
	// MustChangePassword - при следующем входе система потребует задать свой пароль.
	// Плановая проверка сроков только его и делает: пароль остаётся прежним, а
	// работать человек начинает, задав новый. Ставится и при выдаче придуманного
	// системой пароля - перехваченное письмо тогда даёт доступ лишь до того, как
	// войдёт владелец.
	MustChangePassword bool `gorm:"default:false;index" json:"-"`
	// PasswordRotatedAt - когда пароль в последний раз меняла сама система: при
	// обновлении паролей всем работникам или сбросе из карточки. Отличается от
	// PasswordChangedAt, которую двигает и обычная смена самим человеком.
	PasswordRotatedAt *time.Time `json:"-"`
	// OnboardingCompletedVersion - версия онбординг-тура, которую прошёл юзер.
	// null = не проходил. Хранится per-user (а не per-browser), чтобы тур не
	// сбрасывался при смене устройства; при подъёме версии шагов тур показывается заново.
	OnboardingCompletedVersion *int `json:"onboarding_completed_version,omitempty"`
	// Theme - выбранная тема оформления (#1415), одно из ThemeIDs. null = юзер не
	// выбирал, показываем DefaultTheme. Хранится per-user (а не только в
	// localStorage), чтобы оформление ехало за человеком между устройствами.
	Theme *string `gorm:"size:32" json:"theme,omitempty"`
}

type Organization struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:100" json:"name"`
	// Type - тип справочника (issue #1046): одно из OrgTypeValues либо NULL
	// («не указан» у записей до появления поля). Валидируется в сервисе, не gorm-тегом.
	Type *string `gorm:"size:32" json:"type"`
	// IsActive - архивный флаг (soft-delete). Уникальность name обеспечивается
	// partial unique index (WHERE is_active=true) в migrate.go, а не gorm-тегом,
	// чтобы архивная запись не блокировала создание новой активной с тем же именем.
	IsActive bool `gorm:"default:true;index" json:"is_active"`
	// NameNormalized - ключ дедупликации наименования (#1437): normalize.OrgName(Name).
	// По нему ищется существующая запись при подаче заявки, поэтому три написания
	// одного юрлица не могут создать три строки. Наружу не отдаётся - служебное поле.
	NameNormalized string `gorm:"size:150;index" json:"-"`
	// ModerationStatus - ModerationApproved у обычной записи, ModerationPending у
	// пришедшей из формы заявки и ещё не разобранной принимающим (#1437). Колонка
	// добавляется с DEFAULT, поэтому записи, жившие до неё, читаются как проверенные.
	ModerationStatus string `gorm:"size:16;not null;default:'approved';index" json:"moderation_status"`
	// CreatedByUserID - кто завёл запись из заявки; NULL у записей справочника.
	CreatedByUserID *int `json:"created_by_user_id,omitempty"`
}

// BeforeSave шифрует контакты и держит свёртки в согласии со значениями (#2351).
// Свёртка считается от НОРМАЛИЗОВАННОГО значения: у почты это нижний регистр (два
// адреса, различающиеся только им, ведут в один ящик), у телефона - одни цифры
// (+7, 8 и скобки пишут как придётся, а человек один и тот же).
func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.Email != nil {
		u.EmailHMAC = crypto.HMACOptional(normalizedEmailPtr(u.Email))
		enc, err := crypto.EncryptOptional(u.Email)
		if err != nil {
			return err
		}
		u.Email = enc
	}
	if u.Phone != nil {
		u.PhoneHMAC = crypto.HMACOptional(normalizedPhonePtr(u.Phone))
		enc, err := crypto.EncryptOptional(u.Phone)
		if err != nil {
			return err
		}
		u.Phone = enc
	}
	return nil
}

// AfterFind возвращает контакты в читаемый вид.
func (u *User) AfterFind(tx *gorm.DB) error {
	u.Email = crypto.DecryptOptional(u.Email)
	u.Phone = crypto.DecryptOptional(u.Phone)
	return nil
}

// NormalizeEmailForHMAC и NormalizePhoneForHMAC - единственный источник правил
// нормализации. Расходись они между записью и поиском, человек перестал бы
// находиться по собственному адресу.
func NormalizeEmailForHMAC(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func NormalizePhoneForHMAC(v string) string {
	var digits strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	s := digits.String()
	// Восьмёрка и семёрка в начале означают одну и ту же страну: 8 900 ... и
	// +7 900 ... это один номер, записанный по привычке по-разному.
	if len(s) == 11 && s[0] == '8' {
		s = "7" + s[1:]
	}
	return s
}

func normalizedEmailPtr(v *string) *string {
	if v == nil {
		return nil
	}
	n := NormalizeEmailForHMAC(*v)
	return &n
}

func normalizedPhonePtr(v *string) *string {
	if v == nil {
		return nil
	}
	n := NormalizePhoneForHMAC(*v)
	return &n
}

// BeforeSave держит ключ дедупликации в согласии с наименованием. Ловит Create и
// Save структурой; map-обновления (organizationService.Update) пишут name_normalized
// явно - хук туда не достаёт, поскольку в UPDATE попадают только ключи map.
func (o *Organization) BeforeSave(*gorm.DB) error {
	if o.Name != "" {
		o.NameNormalized = normalize.OrgName(o.Name)
	}
	return nil
}

type Company struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:100" json:"name"`
	// Type - тип справочника (issue #1046), см. Organization.Type.
	Type *string `gorm:"size:32" json:"type"`
	// IsActive - архивный флаг (soft-delete). Уникальность name - partial unique
	// index (WHERE is_active=true) в migrate.go, см. Organization.
	IsActive bool `gorm:"default:true;index" json:"is_active"`
	// NameNormalized - ключ дедупликации наименования, см. Organization.NameNormalized.
	NameNormalized string `gorm:"size:150;index" json:"-"`
	// ModerationStatus - статус разбора записи, см. Organization.ModerationStatus.
	ModerationStatus string `gorm:"size:16;not null;default:'approved';index" json:"moderation_status"`
	// CreatedByUserID - кто завёл запись из заявки; NULL у записей справочника.
	CreatedByUserID *int `json:"created_by_user_id,omitempty"`
}

// BeforeSave держит ключ дедупликации в согласии с наименованием, см.
// Organization.BeforeSave.
func (c *Company) BeforeSave(*gorm.DB) error {
	if c.Name != "" {
		c.NameNormalized = normalize.OrgName(c.Name)
	}
	return nil
}

type UserType struct {
	ID   int    `json:"id"`
	Name string `gorm:"size:50" json:"name"`
	Code string `gorm:"uniqueIndex;size:20" json:"code"`
	// IsSystem помечает встроенные типы, чьи code используются в авторизации
	// (internal/auth/permissions.go). Такие типы нельзя переименовать или удалить.
	IsSystem bool `gorm:"default:false" json:"is_system"`
}

// --- Users management DTOs ---

// UserInfoResponse — ответ с полной информацией о пользователе (JSON поля совпадают с Rust UserInfo).
type UserInfoResponse struct {
	ID             int     `json:"id"`
	Username       string  `json:"username"`
	IsActive       bool    `json:"is_active"`
	IsBanned       bool    `json:"is_banned"`
	IsSuperAdmin   bool    `json:"is_super_admin"`
	IsAdmin        bool    `json:"is_admin"`
	IsImportant    bool    `json:"is_important"`
	Organization   *string `json:"organization"`
	OrganizationID *int    `json:"organization_id"`
	Company        *string `json:"company"`
	CompanyID      *int    `json:"company_id"`
	TypeID         int     `json:"type_id"`
	UserType       string  `json:"user_type"`
	RoleID         *int    `json:"role_id"`
	LastName       *string `json:"last_name"`
	FirstName      *string `json:"first_name"`
	MiddleName     *string `json:"middle_name"`
	// PDHidden -- персональные данные не пусты, а скрыты: работник не дал согласия
	// на их обработку (#1567). Скрыты и ФИО, и рабочие контакты. Форма
	// редактирования обязана отличать это от незаполненных полей, иначе сохранение
	// соседнего поля затрёт настоящие данные.
	PDHidden bool `json:"pd_hidden"`
	// ConsentGranted -- у работника есть действующее согласие на обработку
	// персональных данных, ConsentAt -- когда он его дал.
	ConsentGranted bool    `json:"consent_granted"`
	ConsentAt      *string `json:"consent_at"`
	// ConsentRequired -- согласие сейчас спрашивают, и этого работника это касается.
	// Без признака «согласия нет» неотличимо от «его и не спрашивают»: пока запрос
	// выключен, согласия нет вообще ни у кого.
	ConsentRequired bool    `json:"consent_required"`
	Position        *string `json:"position"`
	Email           *string `json:"email"`
	Phone           *string `json:"phone"`
	// LastSeen - последняя активность (см. User.LastSeen). Без omitempty: «никогда
	// не заходил» должно доезжать до клиента явным null, а не отсутствием ключа -
	// таблица пользователей рисует по нему прочерк, а не «только что».
	LastSeen *time.Time `json:"last_seen"`
	// LockedUntil - момент окончания блокировки входа. null означает «не заблокирован»:
	// истёкшие локи запрос отсекает сам, чтобы админке не пришлось сравнивать время.
	LockedUntil *time.Time `json:"locked_until"`
	// LockoutLevel - ступень лестницы кулдаунов. Показывает, каким будет следующий
	// кулдаун, если человек продолжит ошибаться.
	LockoutLevel int `json:"lockout_level"`
}

// RecipientCandidate — пользователь, которого автор может добавить получателем заявки.
// Узкий срез полей: форме нужно показать человека и отличить однофамильцев по должности,
// остальное (контакты, роль, признаки администратора) к выбору получателя отношения не имеет.
type RecipientCandidate struct {
	ID         int     `json:"id"`
	Username   string  `json:"username"`
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
	Position   *string `json:"position"`
	// Organization и Company названы как в UserInfoResponse, а не organization_name
	// соседних ответов: окно пересылки получает оба ответа в ОДИН проп allUsers
	// (носителю page.admin.users - /users/all, остальным - кандидатов), и второе имя
	// того же поля потребовало бы развилки в каждом месте, где окно его читает.
	Organization *string `json:"organization"`
	Company      *string `json:"company"`
	// PDHidden -- ФИО скрыто: работник не дал согласия на обработку ПД (#1567).
	// Организацию и компанию согласие не закрывает: это данные работодателя, а не
	// работника, и в администраторском списке они видны у скрытого работника тоже.
	PDHidden bool `json:"pd_hidden"`
}

// UpdateUserTypeRequest — запрос на обновление типа пользователя.
type UpdateUserTypeRequest struct {
	TypeID int `json:"type_id" validate:"gte=1"`
}

// UpdatePasswordRequest — запрос на обновление пароля пользователя.
type UpdatePasswordRequest struct {
	Password string `json:"password" validate:"required,min=6,max=255"`
}

// ChangeOwnPasswordRequest — запрос на смену СВОЕГО пароля. В отличие от
// UpdatePasswordRequest (админ задаёт пароль другому) требует подтверждения
// текущим паролем: без него угнанная сессия превращается в захват учётной записи.
type ChangeOwnPasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=255"`
	NewPassword     string `json:"new_password" validate:"required,min=6,max=255"`
}

// UpdateUserInfoRequest — запрос на обновление персональных данных пользователя.
type UpdateUserInfoRequest struct {
	LastName    *string `json:"last_name"`
	FirstName   *string `json:"first_name"`
	MiddleName  *string `json:"middle_name"`
	Position    *string `json:"position"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	IsImportant *bool   `json:"is_important"`
}

// UpdateUserOrganizationRequest — запрос на обновление организации пользователя.
type UpdateUserOrganizationRequest struct {
	OrganizationID int `json:"organization_id" validate:"gte=1"`
}

// UpdateUserCompanyRequest — запрос на обновление компании пользователя.
type UpdateUserCompanyRequest struct {
	CompanyID int `json:"company_id" validate:"gte=1"`
}

// SetUserUnloadPlacesRequest — запрос на замену привязки мест разгрузки охранника (#706).
// Пустой slice снимает все привязки.
type SetUserUnloadPlacesRequest struct {
	UnloadPlaceIDs []int `json:"unload_place_ids"`
}

// SetUserTablesRequest — запрос на замену привязки мест прохода охранника (#706).
// Пустой slice снимает все привязки.
type SetUserTablesRequest struct {
	TableIDs []int `json:"table_ids"`
}

// Junction tables

type OrganizationUser struct {
	ID               int          `json:"id"`
	OrganizationID   int          `gorm:"index" json:"organization_id"`
	Organization     Organization `json:"-"`
	UserID           int          `gorm:"index" json:"user_id"`
	User             User         `json:"-"`
	CreatedAt        *time.Time   `json:"created_at"`
	IsPrimary        bool         `gorm:"default:false" json:"is_primary"`
	RequiredApproval bool         `gorm:"default:false" json:"required_approval"`
}

type CompaniesUser struct {
	ID               int        `json:"id"`
	CompanyID        int        `gorm:"index" json:"company_id"`
	Company          Company    `json:"-"`
	UserID           int        `gorm:"index" json:"user_id"`
	User             User       `json:"-"`
	CreatedAt        *time.Time `json:"created_at"`
	IsPrimary        bool       `gorm:"default:false" json:"is_primary"`
	RequiredApproval bool       `gorm:"default:false" json:"required_approval"`
}
