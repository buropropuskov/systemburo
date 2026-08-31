package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserService — интерфейс бизнес-логики управления пользователями (admin-only).
type UserService interface {
	// Create создаёт нового пользователя (admin-only). callerUserID - id админа для аудита.
	// Пустой пароль означает «придумай сама и вышли письмом» и допустим только при
	// указанном адресе почты.
	Create(ctx context.Context, callerUserID int, req models.RegisterRequest) error
	// GetAll возвращает список пользователей с организацией, компанией и типом.
	// includeArchived=false отдаёт только активных (is_active=true).
	GetAll(ctx context.Context, includeArchived bool) ([]models.UserInfoResponse, error)
	// GetRecipientCandidates возвращает тех, кого автор заявки может добавить получателем:
	// коллег по организации и компании плюс руководителей. В отличие от GetAll доступен
	// без прав администратора - выбор получателя есть у любого, кто подаёт заявку.
	GetRecipientCandidates(ctx context.Context, username string) ([]models.RecipientCandidate, error)
	// UpdateType обновляет тип пользователя.
	UpdateType(ctx context.Context, callerUserID int, username string, req models.UpdateUserTypeRequest) error
	// UpdatePassword обновляет пароль пользователя. meta (адрес, клиент) идёт в
	// историю входов и может быть nil там, где http-контекста нет.
	UpdatePassword(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest, meta *RequestMeta) error
	// UpdatePasswordKeepingSession - то же, но сохраняет одну сессию: ту, из
	// которой человек сам сменил пароль. keepRefreshToken - её маркер продления
	// из cookie; пустая строка означает «отозвать все».
	UpdatePasswordKeepingSession(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest, meta *RequestMeta, keepRefreshToken string) error
	// ChangeOwnPassword меняет пароль ТЕКУЩЕГО пользователя по подтверждению
	// текущим паролем. Единственный путь смены пароля без права page.admin.users.
	ChangeOwnPassword(ctx context.Context, userID int, req models.ChangeOwnPasswordRequest, meta *RequestMeta, keepRefreshToken string) error
	// UpdateInfo обновляет ФИО, должность, email и телефон пользователя.
	UpdateInfo(ctx context.Context, callerUserID int, username string, req models.UpdateUserInfoRequest) error
	// UpdateOrganization обновляет организацию пользователя.
	UpdateOrganization(ctx context.Context, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error
	// UpdateCompany обновляет компанию пользователя.
	UpdateCompany(ctx context.Context, callerUserID int, username string, req models.UpdateUserCompanyRequest) error
	// Delete архивирует пользователя по username (soft-delete: is_active=false).
	Delete(ctx context.Context, callerUserID int, username string) error
	// Restore восстанавливает архивного пользователя (is_active=true).
	Restore(ctx context.Context, callerUserID int, username string) error
	// GetHistory возвращает историю действий над пользователем (по username).
	GetHistory(ctx context.Context, username string) ([]models.UserHistoryItem, error)
	// SetBanCache подключает кэш блокировок, чтобы архив/восстановление мгновенно
	// сбрасывали его (офбординг без ожидания TTL). Опционально (может не вызываться).
	SetBanCache(banCache *BanCheckService)
	// SetPasswordPolicyProvider подключает источник политики паролей.
	SetPasswordPolicyProvider(p PasswordPolicyProvider)
	// SetMailSender подключает почту и адрес системы для писем работнику с
	// учётными данными. Опционально: без почты пароль задаёт администратор.
	SetMailSender(mail MailSender, baseURL string)

	// GetUserUnloadPlaces возвращает активные места разгрузки, привязанные к охраннику.
	GetUserUnloadPlaces(ctx context.Context, username string) ([]models.UnloadPlace, error)
	// SetUserUnloadPlaces заменяет привязку мест разгрузки для охранника (delete-all-then-recreate).
	SetUserUnloadPlaces(ctx context.Context, username string, req models.SetUserUnloadPlacesRequest) error
	// GetUserTables возвращает активные места прохода, привязанные к охраннику.
	GetUserTables(ctx context.Context, username string) ([]models.SystemTable, error)
	// SetUserTables заменяет привязку мест прохода для охранника (delete-all-then-recreate).
	SetUserTables(ctx context.Context, username string, req models.SetUserTablesRequest) error

	// --- Групповые операции (bulk). Переиспользуют одиночные методы в цикле по
	// username; частичный успех собирается в BulkOpResult. Цель (тип/орг/компания)
	// валидируется один раз до цикла и возвращается ошибкой на весь запрос. ---

	// BulkArchive архивирует набор пользователей (супер-админ уходит в Errors).
	BulkArchive(ctx context.Context, callerUserID int, usernames []string) (*BulkOpResult, error)
	// BulkRestore восстанавливает набор пользователей из архива.
	BulkRestore(ctx context.Context, callerUserID int, usernames []string) (*BulkOpResult, error)
	// BulkUpdateType меняет тип у набора пользователей.
	BulkUpdateType(ctx context.Context, callerUserID int, usernames []string, typeID int) (*BulkOpResult, error)
	// BulkAssignOrganization назначает организацию набору пользователей.
	BulkAssignOrganization(ctx context.Context, callerUserID int, usernames []string, orgID int) (*BulkOpResult, error)
	// BulkAssignCompany назначает компанию набору пользователей.
	BulkAssignCompany(ctx context.Context, callerUserID int, usernames []string, companyID int) (*BulkOpResult, error)
}

// PasswordPolicyProvider отдаёт текущую политику паролей (реализуется SettingsService).
type PasswordPolicyProvider interface {
	GetPasswordPolicy() models.PasswordPolicy
}

type userService struct {
	db                  *gorm.DB
	notificationService NotificationService
	recorder            AuditRecorder
	banCache            *BanCheckService
	policy              PasswordPolicyProvider
	mail                MailSender
	// baseURL - адрес системы для писем. Пустой означает «ссылку не вставлять».
	baseURL string
}

// NewUserService создаёт новый экземпляр сервиса управления пользователями.
// notificationService может быть nil — в этом случае уведомления просто
// не будут создаваться (legacy совместимость в местах, где notification
// не подключён). Триггерные методы проверяют nil перед использованием.
// История аудита создаётся внутри из db (сигнатура конструктора не меняется).
func NewUserService(db *gorm.DB, notificationService NotificationService) UserService {
	return &userService{
		db:                  db,
		notificationService: notificationService,
		recorder:            NewAuditRecorder(db),
	}
}

// SetBanCache подключает кэш блокировок (опционально, после конструирования -
// в main.go banCheckService создаётся позже userService).
func (s *userService) SetBanCache(banCache *BanCheckService) {
	s.banCache = banCache
}

// SetPasswordPolicyProvider подключает источник политики паролей (опционально,
// после конструирования - settingsService в main.go создаётся позже userService).
func (s *userService) SetPasswordPolicyProvider(p PasswordPolicyProvider) {
	s.policy = p
}

// SetMailSender подключает почту (опционально, после конструирования -
// mailService в main.go создаётся позже userService).
func (s *userService) SetMailSender(mail MailSender, baseURL string) {
	s.mail = mail
	s.baseURL = normalizeBaseURL(baseURL)
}

// mailReady сообщает, есть ли куда отправить письмо с учётными данными. Без
// настроенной почты пароль придётся задать администратору вручную.
func (s *userService) mailReady() bool {
	return s.mail != nil && s.mail.Enabled()
}

// passwordPolicy возвращает активную политику, либо безопасный дефолт, если
// провайдер не подключён (валидация НЕ отключается - это критичная проверка).
func (s *userService) passwordPolicy() models.PasswordPolicy {
	if s.policy != nil {
		return s.policy.GetPasswordPolicy()
	}
	return models.DefaultPasswordPolicy()
}

// targetUserID резолвит id пользователя по username для записи в историю.
// Возвращает 0, если не найден (тогда лог пропускается).
func (s *userService) targetUserID(ctx context.Context, username string) int {
	var id int
	s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", username).Scan(&id)
	return id
}

// --- Групповые операции над пользователями ---

// existsByID проверяет наличие строки по id в справочной таблице (константное имя
// таблицы, admin-only) - для валидации цели bulk-назначения один раз до цикла.
func (s *userService) existsByID(ctx context.Context, table string, id int) bool {
	var ok bool
	s.db.WithContext(ctx).Table(table).Select("COUNT(1) > 0").Where("id = ?", id).Row().Scan(&ok)
	return ok
}

// BulkArchive архивирует набор пользователей через Delete. Несуществующие (404) и
// супер-админ (403) честно попадают в Errors (частичный успех).
func (s *userService) BulkArchive(ctx context.Context, callerUserID int, usernames []string) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		if err := s.Delete(ctx, callerUserID, u); err != nil {
			res.addError(s.targetUserID(ctx, u), u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор пользователей через Restore.
func (s *userService) BulkRestore(ctx context.Context, callerUserID int, usernames []string) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		if err := s.Restore(ctx, callerUserID, u); err != nil {
			res.addError(s.targetUserID(ctx, u), u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkUpdateType меняет тип у набора пользователей. Тип валидируется один раз до
// цикла; несуществующий username - в Errors (одиночный UpdateType его не ловит:
// UPDATE по username даёт 0 строк без ошибки).
func (s *userService) BulkUpdateType(ctx context.Context, callerUserID int, usernames []string, typeID int) (*BulkOpResult, error) {
	if !s.existsByID(ctx, "user_types", typeID) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid user type")
	}
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		id := s.targetUserID(ctx, u)
		if id == 0 {
			res.addError(0, u, "Пользователь не найден")
			continue
		}
		if err := s.UpdateType(ctx, callerUserID, u, models.UpdateUserTypeRequest{TypeID: typeID}); err != nil {
			res.addError(id, u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignOrganization назначает организацию набору пользователей. Организация
// валидируется до цикла (иначе FK-нарушение дало бы 500 на каждого).
func (s *userService) BulkAssignOrganization(ctx context.Context, callerUserID int, usernames []string, orgID int) (*BulkOpResult, error) {
	if !s.existsByID(ctx, "organizations", orgID) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Организация не найдена")
	}
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		id := s.targetUserID(ctx, u)
		if id == 0 {
			res.addError(0, u, "Пользователь не найден")
			continue
		}
		if err := s.UpdateOrganization(ctx, callerUserID, u, models.UpdateUserOrganizationRequest{OrganizationID: orgID}); err != nil {
			res.addError(id, u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignCompany назначает компанию набору пользователей.
func (s *userService) BulkAssignCompany(ctx context.Context, callerUserID int, usernames []string, companyID int) (*BulkOpResult, error) {
	if !s.existsByID(ctx, "companies", companyID) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Компания не найдена")
	}
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		id := s.targetUserID(ctx, u)
		if id == 0 {
			res.addError(0, u, "Пользователь не найден")
			continue
		}
		if err := s.UpdateCompany(ctx, callerUserID, u, models.UpdateUserCompanyRequest{CompanyID: companyID}); err != nil {
			res.addError(id, u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// Create создаёт нового пользователя. Доступ - route-middleware page.admin.users.
//
// Пользователь может быть привязан к организации, компании или к обеим сразу.
// Хотя бы одно из двух поле должно быть > 0. Значение 0 означает "не привязан".
func (s *userService) Create(ctx context.Context, callerUserID int, req models.RegisterRequest) error {
	if req.OrganizationID <= 0 && req.CompanyID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Необходимо указать организацию или компанию (хотя бы одно)")
	}

	// Адрес почты проверяется на формат и занятость (#1908) - тем же кодом, что и
	// при правке карточки, иначе мусор заезжал бы через форму создания. Проверка
	// идёт до разбора пароля: от наличия адреса зависит, обязателен ли пароль.
	if req.Email != nil {
		normalized, err := validateUserEmail(ctx, s.db, *req.Email, 0)
		if err != nil {
			return err
		}
		req.Email = &normalized
	}

	hasEmail := req.Email != nil && *req.Email != ""
	// Пароль, не заданный администратором, придумывает система и высылает
	// работнику письмом. Без адреса или без настроенной почты придумывать его
	// некому: пароль, который негде прочитать, запирает человека снаружи.
	if req.Password == "" {
		if !hasEmail {
			return echo.NewHTTPError(http.StatusBadRequest,
				"Без адреса почты пароль задаёт администратор: укажите пароль или заполните адрес")
		}
		if !s.mailReady() {
			return echo.NewHTTPError(http.StatusBadRequest,
				"Почта не настроена, отправить пароль работнику некуда: задайте пароль вручную")
		}
		req.Password = GeneratePassword(s.passwordPolicy())
	}

	if err := ValidatePassword(s.passwordPolicy(), req.Password); err != nil {
		return err
	}

	// Письмо с учётными данными уходит всякий раз, когда есть куда: работник
	// должен узнать логин и пароль от системы, а не по телефону от того, кто
	// завёл ему учётную запись.
	sendLetter := hasEmail && s.mailReady()

	// Отсчёт срока действия пароля начинается с момента заведения учётной записи
	// (#1907): иначе новый работник попадал бы под первую же плановую смену.
	passwordSetAt := time.Now()

	// Работнику поста свой пароль задавать нечем: смену ему закрывает бюро
	// пропусков, и поднятый флаг запер бы его в окне, которого он не пройдёт.
	securityAccount, err := isSecurityUserType(ctx, s.db, req.TypeID)
	if err != nil {
		slog.Error("не удалось определить тип учётной записи", "type_id", req.TypeID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating user")
	}

	user := models.User{
		Username:          req.Username,
		Password:          hashPassword(req.Password),
		PasswordChangedAt: &passwordSetAt,
		OrganizationID:    intPtrOrNil(req.OrganizationID),
		CompanyID:         intPtrOrNil(req.CompanyID),
		TypeID:            req.TypeID,
		LastName:          req.LastName,
		FirstName:         req.FirstName,
		MiddleName:        req.MiddleName,
		Position:          req.Position,
		Email:             req.Email,
		Phone:             req.Phone,
		// Свой пароль работник задаёт при первом входе независимо от того, кто
		// придумал текущий: и придуманный системой, и заданный администратором
		// прошёл через чужие руки, а высланный письмом ещё и лежит открытым
		// текстом в почтовом ящике. Работник поста - исключение: его пароль
		// целиком ведёт бюро пропусков (#2280).
		MustChangePassword: !securityAccount,
	}
	if err := s.createWithWelcomeLetter(ctx, &user, req.Password, sendLetter); err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return echo.NewHTTPError(http.StatusBadRequest, "Username already exists")
		}
		slog.Error("не удалось создать пользователя", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating user")
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, models.UserActionCreated, &callerUserID, map[string]any{
		"username": user.Username,
		"type_id":  user.TypeID,
	})

	// Новый пользователь получает базовую роль "Пользователь" по умолчанию -- так роль
	// выдаёт стартовый набор прав (ТЗ). Best-effort: отсутствие базовой роли не валит создание.
	if user.RoleID == nil {
		var baseRole models.Role
		if err := s.db.WithContext(ctx).Where("code = ? AND is_system = ?", "user", true).First(&baseRole).Error; err == nil {
			if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).
				Update("role_id", baseRole.ID).Error; err != nil {
				slog.Error("не удалось назначить базовую роль", "user_id", user.ID, "error", err)
			}
		}
	}
	return nil
}

// createWithWelcomeLetter заводит учётную запись и, если есть куда, ставит в
// очередь письмо с учётными данными. Одной транзакцией: сбой между записями
// оставил бы работника с паролем, которого он не видел, или письмо с паролем от
// учётной записи, которой нет.
func (s *userService) createWithWelcomeLetter(ctx context.Context, user *models.User, password string, sendLetter bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		// Первый пароль запоминается вместе с учётной записью: запрет на повтор
		// сравнивает новый пароль с прежними, и без этой строки работник смог бы
		// «сменить» выданный ему пароль на него же.
		if err := recordUsedPassword(ctx, tx, user.ID, user.Password); err != nil {
			return err
		}
		if !sendLetter {
			return nil
		}
		return s.mail.Enqueue(ctx, tx, MailMessage{
			To:           *user.Email,
			Subject:      "Учётная запись в системе бюро пропусков",
			Body:         accountCreatedLetterBody(*user, password, s.baseURL),
			TemplateCode: MailTemplateAccountCreated,
			UserID:       &user.ID,
		})
	})
}

// GetAll возвращает пользователей с JOIN на организацию, компанию и тип.
// По умолчанию только активные; includeArchived=true добавляет архивных.
func (s *userService) GetAll(ctx context.Context, includeArchived bool) ([]models.UserInfoResponse, error) {
	result := make([]models.UserInfoResponse, 0)
	q := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.is_active, u.is_banned, u.is_super_admin, u.is_important,
			o.name as organization, u.organization_id,
			c.name as company, u.company_id,
			u.type_id, ut.name as user_type, u.role_id,
			u.last_name, u.first_name, u.middle_name,
			u.position, u.email, u.phone, u.last_seen, u.lockout_level,
			CASE WHEN u.locked_until > NOW() THEN u.locked_until END AS locked_until`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Joins("LEFT JOIN user_types ut ON u.type_id = ut.id")
	if !includeArchived {
		q = q.Where("u.is_active = ?", true)
	}
	if err := q.Order("u.username").Scan(&result).Error; err != nil {
		slog.Error("не удалось получить список пользователей", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching users")
	}

	masks, consentActive := consentMasksWithState(ctx, s.db)
	grants := loadConsentGrants(ctx, s.db)
	for i := range result {
		if at, ok := grants[result[i].ID]; ok {
			result[i].ConsentGranted = true
			granted := at
			result[i].ConsentAt = &granted
		}
		// Кого запрос согласия реально касается - та же мерка, что у гейта.
		result[i].ConsentRequired = consentActive && !result[i].IsSuperAdmin &&
			result[i].IsActive && !result[i].IsBanned
		if _, hidden := masks[result[i].ID]; !hidden {
			continue
		}
		maskUserParts(masks, result[i].ID, &result[i].LastName, &result[i].FirstName, &result[i].MiddleName)
		maskUserContacts(masks, result[i].ID, &result[i].Email, &result[i].Phone)
		result[i].PDHidden = true
	}
	return result, nil
}

// GetRecipientCandidates возвращает кандидатов в получатели заявки для текущего
// пользователя. Пустой список - штатный ответ (человек один в своей организации, и
// руководителей в системе нет), а не ошибка.
func (s *userService) GetRecipientCandidates(ctx context.Context, username string) ([]models.RecipientCandidate, error) {
	var me models.User
	if err := s.db.WithContext(ctx).
		Select("id, organization_id, company_id").
		Where("username = ?", username).
		First(&me).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		slog.Error("не удалось определить пользователя для списка получателей", "error", err, "username", username)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching recipient candidates")
	}
	return loadRecipientCandidates(ctx, s.db, me)
}

// UpdateType обновляет type_id пользователя с проверкой существования типа.
func (s *userService) UpdateType(ctx context.Context, callerUserID int, username string, req models.UpdateUserTypeRequest) error {
	// Проверяем существование типа
	var exists bool
	if err := s.db.WithContext(ctx).
		Table("user_types").
		Select("COUNT(1) > 0").
		Where("id = ?", req.TypeID).
		Row().Scan(&exists); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type")
	}

	var oldType int
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("type_id").Row().Scan(&oldType)

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("type_id", req.TypeID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user type")
	}

	if req.TypeID != oldType {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionTypeChanged, &callerUserID, map[string]any{"old": oldType, "new": req.TypeID})
		}
	}
	return nil
}

// UpdatePassword хеширует и обновляет пароль пользователя.
// После смены пароля все refresh_tokens юзера отзываются - иначе старые
// сессии (возможно скомпрометированные) продолжили бы жить до истечения TTL.
//
// Пароль, заданный администратором, работник меняет при первом входе, и если у
// него указан адрес почты - получает этот пароль письмом. Своя смена (тот же
// метод из ChangeOwnPassword) ни того, ни другого не делает.
func (s *userService) UpdatePassword(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest, meta *RequestMeta) error {
	return s.UpdatePasswordKeepingSession(ctx, callerUserID, username, req, meta, "")
}

// UpdatePasswordKeepingSession меняет пароль, оставляя живой одну сессию.
//
// Человек, сменивший пароль сам, не должен вылетать из системы: он только что
// подтвердил, что это он, и выкидывать его на форму входа - раздражение без
// выигрыша в безопасности. Все остальные сессии гаснут, поэтому угнанная на
// другом устройстве умирает, как и раньше. Так это устроено в системах, где
// смена пароля - обычное действие, а не аварийная процедура.
func (s *userService) UpdatePasswordKeepingSession(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest, meta *RequestMeta, keepRefreshToken string) error {
	if err := ValidatePassword(s.passwordPolicy(), req.Password); err != nil {
		return err
	}

	// Целевую запись читаем до обновления: от неё зависит признак обязательной
	// смены, адрес для письма с новым паролем и перечень прежних паролей - он
	// живёт по id, а сюда приходит логин. Not-found = пользователь не найден;
	// обновление по username тогда просто не тронет строк.
	var user models.User
	found := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error == nil

	if found {
		if err := ensurePasswordNotReused(ctx, s.db, user.ID, req.Password); err != nil {
			return err
		}
	}

	hashed := hashPassword(req.Password)

	// Пароль, заданный администратором, работник обязан сменить при первом входе:
	// его придумал не он, а если у работника есть адрес - пароль ещё и лежит
	// открытым текстом в почтовом ящике. Своя смена под это не подпадает: человек
	// только что задал пароль сам.
	adminSet := found && user.ID != callerUserID
	sendLetter := adminSet && s.mailReady() && user.Email != nil && *user.Email != ""

	// Дата смены двигается вместе с паролем (#1907): от неё считается срок
	// действия при плановой смене. Письмо о новом пароле и отпечаток пароля в
	// перечне прежних пишутся той же транзакцией - иначе сбой между записями
	// оставит работника с паролем, которого он не видел, либо запрет на пароль,
	// который не сохранился.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("users").
			Where("username = ?", username).
			Updates(map[string]any{
				"password":             hashed,
				"password_changed_at":  time.Now(),
				"must_change_password": adminSet,
			}).Error; err != nil {
			return err
		}
		if found {
			if err := recordUsedPassword(ctx, tx, user.ID, hashed); err != nil {
				return err
			}
		}
		if !sendLetter {
			return nil
		}
		return s.mail.Enqueue(ctx, tx, MailMessage{
			To:           *user.Email,
			Subject:      "Новый пароль в системе бюро пропусков",
			Body:         passwordSetByAdminLetterBody(user, req.Password, s.baseURL),
			TemplateCode: MailTemplatePasswordSetByAdmin,
			UserID:       &user.ID,
		})
	}); err != nil {
		slog.Error("не удалось обновить пароль", "username", username, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating password")
	}

	// Revoke all active refresh tokens: чтобы существующие сессии с этой учёткой
	// (возможно скомпрометированные) не дожили до своего TTL. Юзеру придётся
	// перелогиниться на всех устройствах.
	if found {
		revoke := s.db.WithContext(ctx).
			Model(&models.RefreshToken{}).
			Where("user_id = ? AND is_revoked = false", user.ID)
		if keepRefreshToken != "" {
			// Сессию, из которой шла смена, оставляем живой - её владелец только
			// что подтвердил личность текущим паролем.
			revoke = revoke.Where("token_hash <> ?", hashRefreshToken(keepRefreshToken))
		}
		revoke.Update("is_revoked", true)

		// Аудит: факт сброса пароля без значения.
		s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, models.UserActionPasswordReset, &callerUserID, nil)

		// Уведомление о смене пароля. selfChange определяется по совпадению id
		// вызывающего с целевым (гейт page.admin.users допускает и смену
		// админом собственного пароля).
		s.notifyPasswordChanged(ctx, &user, callerUserID)

		// Событие в истории входов. Модель AuthEvent обещает запись при смене
		// пароля с самого начала, но до этого её не делал никто: человек видел
		// в своей ленте входы и отказы, а факт подмены пароля - нет.
		detail := "пароль задан администратором"
		if callerUserID == user.ID {
			detail = "смена своего пароля"
		}
		s.recordPasswordChangeEvent(ctx, &user, meta, true, detail)
	}

	return nil
}

// ChangeOwnPassword меняет пароль текущего пользователя. Требует подтверждения
// текущим паролем: перехваченная сессия иначе даёт смену пароля, то есть полный
// захват учётной записи. Дальше переиспользует UpdatePassword - отзыв сессий,
// аудит и уведомление там уже написаны и должны работать одинаково независимо
// от того, кто менял пароль.
func (s *userService) ChangeOwnPassword(ctx context.Context, userID int, req models.ChangeOwnPasswordRequest, meta *RequestMeta, keepRefreshToken string) error {
	// Пароль работника поста ведёт бюро пропусков (#2280): форму смены ему не
	// показывают, а запрос мимо интерфейса отклоняется здесь.
	securityAccount, err := isSecurityUser(ctx, s.db, userID)
	if err != nil {
		slog.Error("не удалось определить тип учётной записи", "user_id", userID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сменить пароль")
	}
	if securityAccount {
		return echo.NewHTTPError(http.StatusForbidden, "Пароль работника поста меняет бюро пропусков")
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Пользователь не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error loading user")
	}

	currentMatches, err := verifyPassword(user.Password, req.CurrentPassword)
	if err != nil {
		// Хеш в базе не разбирается - дефект данных этой учётки, а не ошибка во
		// введённом пароле. "Текущий пароль указан неверно" тут была бы такой же
		// ложью, как счётчик неудачных попыток при входе по битому хешу (#2017):
		// человек с верным паролем не смог бы сменить его вовсе.
		slog.Error("не удалось проверить текущий пароль: повреждена запись в базе", "username", user.Username, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось проверить текущий пароль. Обратитесь к администратору.")
	}
	if !currentMatches {
		// Неудачная попытка тоже попадает в историю: подбор текущего пароля через
		// эту форму - такой же признак инцидента, как серия неудачных входов.
		s.recordPasswordChangeEvent(ctx, &user, meta, false, "неверный текущий пароль")
		return echo.NewHTTPError(http.StatusBadRequest, "Текущий пароль указан неверно")
	}

	// user.Password уже успешно разобран строкой выше - вторая проверка того же
	// хеша ошибку разбора вернуть не может, но обрабатываем её на случай расхождения.
	sameAsCurrent, err := verifyPassword(user.Password, req.NewPassword)
	if err != nil {
		slog.Error("не удалось сравнить новый пароль с текущим", "username", user.Username, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось проверить новый пароль. Обратитесь к администратору.")
	}
	if sameAsCurrent {
		return echo.NewHTTPError(http.StatusBadRequest, "Новый пароль совпадает с текущим")
	}

	return s.UpdatePasswordKeepingSession(ctx, userID, user.Username,
		models.UpdatePasswordRequest{Password: req.NewPassword}, meta, keepRefreshToken)
}

// recordPasswordChangeEvent пишет запись в auth_events. Best-effort по образцу
// authService.recordAuthEvent: провал журнала не отменяет уже сменённый пароль.
func (s *userService) recordPasswordChangeEvent(ctx context.Context, user *models.User, meta *RequestMeta, success bool, detail string) {
	ip, ua := "", ""
	if meta != nil {
		ip = meta.IPAddress
		ua = meta.UserAgent
	}
	if len(ua) > 255 {
		ua = ua[:255]
	}
	ev := models.AuthEvent{
		UserID:    &user.ID,
		Username:  user.Username,
		EventType: models.AuthEventPasswordChanged,
		Success:   success,
		IPAddress: ip,
		UserAgent: ua,
		Detail:    detail,
	}
	if err := s.db.WithContext(ctx).Create(&ev).Error; err != nil {
		slog.Warn("не удалось записать событие смены пароля", "user_id", user.ID, "error", err)
	}
}

// notifyPasswordChanged создаёт уведомление о смене пароля. Вызывается
// после успешного апдейта; ошибки только логируются (уведомления не
// должны блокировать основной flow).
func (s *userService) notifyPasswordChanged(ctx context.Context, target *models.User, callerUserID int) {
	if s.notificationService == nil {
		return
	}

	// selfChange = пароль сменил сам владелец учётки (id вызывающего совпадает
	// с целевым). Иначе это сделал админ через раздел управления пользователями.
	selfChange := callerUserID != 0 && callerUserID == target.ID

	// Кто именно сменил пароль, человеку не сообщаем: должность меняющего к делу не
	// относится, а важен сам факт - пароль стал другим, и если это был не ты, надо
	// идти разбираться. Прежняя формулировка называла администратора, хотя сменить
	// пароль может и не он.
	message := "Ваш пароль в системе был изменён."
	if selfChange {
		message = "Ваш пароль был успешно изменён."
	}

	dataPayload := map[string]any{
		"changed_at":         time.Now().UTC().Format(time.RFC3339),
		"changed_by_user_id": callerUserID,
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Error("не удалось сериализовать payload уведомления", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := s.notificationService.CreateForUser(
		ctx, target.ID,
		NotificationTypePasswordChanged,
		"Пароль изменён",
		message,
		&dataStr,
	); err != nil {
		slog.Error("не удалось создать уведомление о смене пароля", "user_id", target.ID, "error", err)
	}
}

// UpdateInfo обновляет персональные данные пользователя.
func (s *userService) UpdateInfo(ctx context.Context, callerUserID int, username string, req models.UpdateUserInfoRequest) error {
	// Снимок старых значений до апдейта - чтобы в историю писать дифф "старое -> новое"
	// и только по реально изменившимся полям (фронт шлёт все поля каждый раз).
	var prev struct {
		LastName    string
		FirstName   string
		MiddleName  string
		Position    string
		Email       string
		Phone       string
		IsImportant bool
	}
	s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Select("last_name", "first_name", "middle_name", "position", "email", "phone", "is_important").
		Scan(&prev)

	updates := map[string]interface{}{
		"position": req.Position,
	}
	// ФИО пишем, только если поле пришло. Пустая строка по-прежнему очищает его, а
	// отсутствие поля означает "не трогай": форма редактирования не показывает ФИО
	// работника, скрытое до его согласия на обработку данных, и не должна затирать
	// настоящее значение правкой соседнего поля.
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.MiddleName != nil {
		updates["middle_name"] = *req.MiddleName
	}
	// Контакты скрываются вместе с ФИО, и правило для них то же: нет поля в
	// запросе - не трогаем, пустая строка по-прежнему очищает.
	if req.Email != nil {
		// Адрес проверяется на формат и на занятость (#1908): он стал каналом
		// доставки паролей, и опечатка в нём означает пароль в никуда.
		normalized, err := validateUserEmail(ctx, s.db, *req.Email, s.targetUserID(ctx, username))
		if err != nil {
			return err
		}
		updates["email"] = normalized
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.IsImportant != nil {
		updates["is_important"] = *req.IsImportant
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Updates(updates).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user info")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		// Только изменившиеся поля, как {old, new}. Если ничего не поменялось - не логируем.
		details := map[string]any{}
		diff := func(key string, np *string, old string) {
			if np != nil && *np != old {
				details[key] = map[string]any{"old": old, "new": *np}
			}
		}
		diff("last_name", req.LastName, prev.LastName)
		diff("first_name", req.FirstName, prev.FirstName)
		diff("middle_name", req.MiddleName, prev.MiddleName)
		diff("position", req.Position, prev.Position)
		diff("email", req.Email, prev.Email)
		diff("phone", req.Phone, prev.Phone)
		if req.IsImportant != nil && *req.IsImportant != prev.IsImportant {
			details["is_important"] = map[string]any{"old": prev.IsImportant, "new": *req.IsImportant}
		}
		if len(details) > 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionUpdated, &callerUserID, details)
		}
	}
	return nil
}

// UpdateOrganization обновляет organization_id пользователя.
func (s *userService) UpdateOrganization(ctx context.Context, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error {
	var prev struct{ OrganizationID *int }
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("organization_id").Scan(&prev)
	oldVal := 0
	if prev.OrganizationID != nil {
		oldVal = *prev.OrganizationID
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("organization_id", req.OrganizationID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization")
	}

	if req.OrganizationID != oldVal {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionOrgChanged, &callerUserID, map[string]any{"old": prev.OrganizationID, "new": req.OrganizationID})
		}
	}
	return nil
}

// UpdateCompany обновляет company_id пользователя.
func (s *userService) UpdateCompany(ctx context.Context, callerUserID int, username string, req models.UpdateUserCompanyRequest) error {
	var prev struct{ CompanyID *int }
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("company_id").Scan(&prev)
	oldVal := 0
	if prev.CompanyID != nil {
		oldVal = *prev.CompanyID
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("company_id", req.CompanyID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}

	if req.CompanyID != oldVal {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionCompanyChanged, &callerUserID, map[string]any{"old": prev.CompanyID, "new": req.CompanyID})
		}
	}
	return nil
}

// Delete архивирует пользователя (soft-delete: is_active=false). Строка остаётся,
// поэтому ссылки заявок (sender_user_id и др.) не осиротевают; login/refresh
// блокируются по is_active, активные refresh-токены отзываются.
func (s *userService) Delete(ctx context.Context, callerUserID int, username string) error {
	return s.setActive(ctx, username, false, callerUserID)
}

// Restore восстанавливает архивного пользователя (is_active=true).
func (s *userService) Restore(ctx context.Context, callerUserID int, username string) error {
	return s.setActive(ctx, username, true, callerUserID)
}

func (s *userService) setActive(ctx context.Context, username string, active bool, callerUserID int) error {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "User not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user")
	}
	if user.IsActive == active {
		return nil // no-op
	}
	// Архив = мгновенный офбординг (BanCheck даёт 403), поэтому супер-админа,
	// как и при бане, архивировать нельзя - иначе админ может вырубить владельца.
	if !active && user.IsSuperAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Нельзя архивировать супер-администратора")
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", user.ID).Update("is_active", active).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user")
		}
		if !active {
			// Отзываем активные refresh-токены: существующая сессия гаснет в пределах
			// TTL access-токена (login/refresh уже блокируются по is_active).
			if err := tx.Model(&models.RefreshToken{}).
				Where("user_id = ? AND is_revoked = ?", user.ID, false).
				Updates(map[string]any{"is_revoked": true, "revoked_at": time.Now().UTC()}).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error revoking tokens")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	action := models.UserActionRestored
	if !active {
		action = models.UserActionArchived
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, action, &callerUserID, nil)

	// Сбрасываем кэш блокировок, чтобы архив/восстановление подействовали мгновенно
	// (BanCheck на следующем запросе перечитает is_active, не дожидаясь TTL).
	if s.banCache != nil {
		s.banCache.Invalidate(user.ID)
	}
	return nil
}

// GetHistory возвращает историю действий над пользователем по username (admin-only).
// Переходный период #870: новые записи идут в audit_log, старые лежат в user_histories.
// UNION объединяет обе таблицы в одинаковую форму ответа.
func (s *userService) GetHistory(ctx context.Context, username string) ([]models.UserHistoryItem, error) {
	id := s.targetUserID(ctx, username)
	if id == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	// Read-switch #870 (F.3): до-cutover строки user_histories подняты в audit_log
	// разовым backfill'ом, читаем только audit_log. Старая таблица user_histories дропнута (F.8).
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUser, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.UserHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UserHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}

// resolveUserID резолвит id по username, возвращает 404 если не найден.
func (s *userService) resolveUserID(ctx context.Context, username string) (int, error) {
	id := s.targetUserID(ctx, username)
	if id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	return id, nil
}

// GetUserUnloadPlaces возвращает активные места разгрузки, привязанные к пользователю.
// GET-пикер намеренно фильтрует is_active=true, чтобы архивные места не попадали в список назначения.
func (s *userService) GetUserUnloadPlaces(ctx context.Context, username string) ([]models.UnloadPlace, error) {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return nil, err
	}
	var places []models.UnloadPlace
	if err := s.db.WithContext(ctx).
		Table("unload_places up").
		Select("up.*").
		Joins("JOIN security_user_unload_places sup ON sup.unload_place_id = up.id").
		Where("sup.user_id = ? AND up.is_active = ?", userID, true).
		Order("up.name").
		Scan(&places).Error; err != nil {
		slog.Error("не удалось получить места разгрузки пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user unload places")
	}
	return places, nil
}

// SetUserUnloadPlaces заменяет привязку мест разгрузки (delete-all-then-recreate в транзакции).
func (s *userService) SetUserUnloadPlaces(ctx context.Context, username string, req models.SetUserUnloadPlacesRequest) error {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.SecurityUserUnloadPlace{}).Error; err != nil {
			slog.Error("не удалось удалить старые места разгрузки пользователя", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
		}
		for _, placeID := range req.UnloadPlaceIDs {
			row := models.SecurityUserUnloadPlace{UserID: userID, UnloadPlaceID: placeID}
			if err := tx.Create(&row).Error; err != nil {
				slog.Error("не удалось добавить место разгрузки пользователю", "place_id", placeID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
			}
		}
		return nil
	})
}

// GetUserTables возвращает активные места прохода, привязанные к пользователю.
func (s *userService) GetUserTables(ctx context.Context, username string) ([]models.SystemTable, error) {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return nil, err
	}
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).
		Table("system_tables st").
		Select("st.*").
		Joins("JOIN security_user_tables sut ON sut.table_id = st.id").
		Where("sut.user_id = ? AND st.is_active = ?", userID, true).
		Order("st.name").
		Scan(&tables).Error; err != nil {
		slog.Error("не удалось получить места прохода пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user tables")
	}
	return tables, nil
}

// SetUserTables заменяет привязку мест прохода (delete-all-then-recreate в транзакции).
func (s *userService) SetUserTables(ctx context.Context, username string, req models.SetUserTablesRequest) error {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.SecurityUserTable{}).Error; err != nil {
			slog.Error("не удалось удалить старые места прохода пользователя", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating tables")
		}
		for _, tableID := range req.TableIDs {
			row := models.SecurityUserTable{UserID: userID, TableID: tableID}
			if err := tx.Create(&row).Error; err != nil {
				slog.Error("не удалось добавить место прохода пользователю", "table_id", tableID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating tables")
			}
		}
		return nil
	})
}
