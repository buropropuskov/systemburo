package fakedata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DefaultUserPassword -- пароль создаваемых пользователей "из коробки", используется,
// если Env.UserPassword не передан (пустая строка). Подобран так, чтобы пройти почти
// любую разумную политику паролей (длина, оба регистра, цифра, спецсимвол) -- реальную
// политику стенда наливка всё равно проверяет перед созданием (см. usersStep.Run), но
// дефолт не должен спотыкаться об неё в обычной конфигурации.
const DefaultUserPassword = "FakeStand-2026!"

// userCreateRetries -- сколько раз пробуем создать пользователя заново со свежими
// случайными данными при конфликте логина (см. registryCreateRetries в registries.go).
// Логин собирается из типа и случайного числа -- коллизия внутри одного прогона
// маловероятна, но повторный прогон с тем же -seed на уже наполненной базе может
// задеть занятый логин.
const userCreateRetries = 5

// userUsernameMaxNumber -- верхняя граница случайного суффикса логина. Диапазон с
// большим запасом относительно объёма профиля "large" (500 пользователей) -- редкие
// коллизии всё равно ловит retry.
const userUsernameMaxNumber = 99999

// userTypeWeights -- вес типа пользователя при случайном выборе (ключ -- user_types.code,
// см. Seed в migrate.go). Большинство наливаемых -- обычные заявители (user/renter/
// contractor), меньшая доля -- охрана, руководство и сотрудники бюро: их роль в системе
// не "подать заявку", но они всё равно учётные записи, и стенду нужно разнообразие типов,
// а не сплошной "user". buropropuskov -- собственный персонал бюро, поэтому доля минимальна.
var userTypeWeights = map[string]int{
	"user":          35,
	"renter":        20,
	"contractor":    20,
	"security":      10,
	"manager":       10,
	"buropropuskov": 5,
}

// userTypeDefaultWeight -- вес типа, которого нет в userTypeWeights (кастомный тип,
// заведённый администратором стенда уже после этого списка). Наравне со штатной
// "малой" долей -- чтобы не выпадать из распределения совсем.
const userTypeDefaultWeight = 5

// userBanReasons -- правдоподобные причины блокировки пользователя, см.
// vehicleBlacklistReasons/personBlacklistReasons в blacklists.go.
var userBanReasons = []string{
	"Неоднократные нарушения пропускного режима",
	"Подозрение на компрометацию учётной записи",
	"Систематическая подача недостоверных данных в заявках",
	"По заявлению организации: сотрудник уволен",
	"Нарушение регламента взаимодействия с бюро пропусков",
}

// usersStep наливает пользователей-заявителей и расставляет их роли в системе (#1682,
// том 5): типы, права администратора, принимающих (application_approvers) и согласующих
// организаций/компаний (organization_users/companies_users). Организации и компании уже
// должны существовать в базе (organizationsStep) -- пользователь обязан быть привязан
// хотя бы к одной из них (см. userService.Create).
type usersStep struct{}

func (usersStep) Name() string { return "пользователи и согласующие" }

func (usersStep) Plan(p Profile) []PlanItem {
	return []PlanItem{
		{Entity: models.AuditEntityUser, Title: EntityTitle(models.AuditEntityUser), Count: p.Users},
		{Entity: models.AuditEntityApprover, Title: EntityTitle(models.AuditEntityApprover), Count: approverUserCount(p.Users)},
	}
}

func (usersStep) Run(ctx context.Context, env *Env) error {
	if env.Profile.Users <= 0 {
		return nil
	}

	// Пользователи создаются от имени администратора стенда (аудит), а принимающих
	// ApproverService.Create назначает по РЕАЛЬНОМУ логину автора -- без администратора
	// назначать принимающих не от кого. Падаем сразу, а не тихо пропускаем расстановку
	// ролей (см. то же рассуждение в registries.go/blacklists.go).
	actorName, err := actorUsername(ctx, env.DB, env.ActorUserID)
	if err != nil {
		return err
	}
	if actorName == "" {
		return fmt.Errorf("на стенде нет ни одного администратора: пользователи создаются от его имени " +
			"(аудит), а принимающих ApproverService назначает по реальному логину автора. Заведите " +
			"учётную запись администратора (make staging-seed) и повторите")
	}

	refs, err := loadUserAffiliationRefs(ctx, env.DB)
	if err != nil {
		return err
	}
	types, err := loadUserTypes(ctx, env.DB)
	if err != nil {
		return err
	}

	password := strings.TrimSpace(env.UserPassword)
	if password == "" {
		password = DefaultUserPassword
	}
	policy := env.PasswordPolicy
	if policy.MinLength <= 0 {
		policy = models.DefaultPasswordPolicy()
	}
	if err := services.ValidatePassword(policy, password); err != nil {
		return fmt.Errorf("пароль наливки (-user-pass) не соответствует политике паролей стенда: %w", err)
	}

	recorder := services.NewAuditRecorder(env.DB)
	resolver := services.NewPermissionResolver(env.DB)
	userSvc := services.NewUserService(env.DB, nil)
	userSvc.SetPasswordPolicyProvider(staticPasswordPolicy(policy))
	banSvc := services.NewUserBanService(env.DB, resolver, nil, recorder)
	permSvc := services.NewPermissionGroupService(env.DB, resolver)
	approverSvc := services.NewApproverService(env.DB)
	consentSvc := services.NewConsentService(env.DB)

	streams := newUserStreams(env.Seed)

	// --- создание пользователей ---

	created := make([]createdFakeUser, 0, env.Profile.Users)
	for i := 0; i < env.Profile.Users; i++ {
		cu, err := createFakeUser(ctx, env.DB, userSvc, env.ActorUserID, refs, types, password, streams)
		if err != nil {
			return fmt.Errorf("пользователь %d/%d: %w", i+1, env.Profile.Users, err)
		}
		// Регистрация в партии сразу после создания -- сбой между Create и Add оставил бы
		// пользователя без записи в партии (см. тот же приём в registries.go).
		if err := env.Batch.Add(ctx, models.AuditEntityUser, cu.id); err != nil {
			return fmt.Errorf("регистрация пользователя %d в партии: %w", cu.id, err)
		}
		if err := grantFakeUserConsent(ctx, consentSvc, env.Consent, cu.id); err != nil {
			return fmt.Errorf("согласие на обработку ПД пользователю %d: %w", cu.id, err)
		}
		created = append(created, cu)
	}

	// --- состояния: часть заблокирована, часть заархивирована ---
	//
	// Считаем count'ами, а не вероятностью на каждого: на маленьком профиле бросок монетки
	// может не дать ни одного заблокированного, и стенду будет не на чем проверить это
	// состояние. Первые bannedCount/archivedCount из created -- любая часть партии подходит
	// одинаково, порядок создания случаен сам по себе (имя/тип/оргпривязка -- из независимых
	// потоков).
	bannedCount := bannedUserCount(env.Profile.Users)
	archivedCount := archivedUserCount(env.Profile.Users)
	if bannedCount+archivedCount > len(created) {
		// Ручное переопределение -users заметно ниже профиля -- ужимаем архивных, чтобы
		// осталось на кого делить состояния и назначать роли дальше по шагу.
		archivedCount = len(created) - bannedCount
		if archivedCount < 0 {
			archivedCount = 0
			bannedCount = len(created)
		}
	}

	for _, cu := range created[:bannedCount] {
		reason := Pick(streams.banReason, userBanReasons)
		if err := banSvc.Ban(ctx, cu.id, env.ActorUserID, reason); err != nil {
			return fmt.Errorf("блокировка пользователя %d: %w", cu.id, err)
		}
	}
	for _, cu := range created[bannedCount : bannedCount+archivedCount] {
		if err := userSvc.Delete(ctx, env.ActorUserID, cu.username); err != nil {
			return fmt.Errorf("архивация пользователя %d: %w", cu.id, err)
		}
	}

	// active -- пользователи в обычном состоянии: кандидаты на права администратора,
	// принимающих и согласующих. Заблокированный/архивный не должен войти в систему,
	// поэтому им эти роли не назначаются -- реальный процесс упёрся бы в человека,
	// который не может залогиниться.
	active := created[bannedCount+archivedCount:]

	// --- права администратора: реальный механизм привилегий после эпика отвязки от
	// user_type (см. project_auth_decouple_usertype в памяти проекта) -- базовую роль
	// "Пользователь" Create уже назначил всем автоматически, других системных ролей на
	// свежем стенде нет (RoleService заводит их только по действию администратора), поэтому
	// "роль, дающая права" здесь -- это флаг is_admin, а не Role. ---

	adminN := min(adminUserCount(env.Profile.Users), len(active))
	for _, cu := range active[:adminN] {
		if err := permSvc.SetUserAdmin(ctx, cu.id, true); err != nil {
			return fmt.Errorf("права администратора пользователю %d: %w", cu.id, err)
		}
	}

	// --- принимающие (application_approvers): "принял" заявку и становится
	// responsible_user, см. project_approver_responsible_mask в памяти проекта ---
	//
	// Кандидаты -- активные БЕЗ учёта уже назначенных администраторов (active[adminN:]),
	// а не весь active заново: иначе на профилях, где adminUserCount и approverUserCount
	// совпадают (одна и та же формула userShareCount с делителем 10), это были бы одни и
	// те же люди -- стенду нужнее разнообразие ролей, а не сплошное "админ = принимающий".

	approverPool := active[adminN:]
	approverN := min(approverUserCount(env.Profile.Users), len(approverPool))
	for _, cu := range approverPool[:approverN] {
		if err := approverSvc.Create(ctx, cu.id, actorName); err != nil {
			return fmt.Errorf("назначение принимающего (пользователь %d): %w", cu.id, err)
		}
		// AuditEntityApprover -- та же сущность, которой approverService.Create само пишет
		// историю (по user_id, не по id строки application_approvers, см. approver_service.go).
		if err := env.Batch.Add(ctx, models.AuditEntityApprover, cu.id); err != nil {
			return fmt.Errorf("регистрация принимающего %d в партии: %w", cu.id, err)
		}
	}

	// --- согласующие: у каждой организации должен быть хотя бы один required_approval,
	// иначе будущим заявкам некому назначить согласование ---

	if err := ensureOrganizationApprovers(ctx, env.DB, refs.organizationIDs, active, env.ActorUserID, streams); err != nil {
		return err
	}
	if err := ensureCompanyApprovers(ctx, env.DB, refs.companyIDs, active, env.ActorUserID, streams); err != nil {
		return err
	}

	return nil
}

// staticPasswordPolicy -- адаптер services.PasswordPolicyProvider поверх уже прочитанной
// политики (Env.PasswordPolicy). usersStep не тянет зависимость на internal/config и на
// живой SettingsService -- значение политики читает вызывающий код заранее
// (settingsService.GetPasswordPolicy в cmd/server/fake.go).
type staticPasswordPolicy models.PasswordPolicy

func (p staticPasswordPolicy) GetPasswordPolicy() models.PasswordPolicy {
	return models.PasswordPolicy(p)
}

// createdFakeUser -- то немногое о только что созданном пользователе, что нужно
// дальнейшим шагам расстановки ролей: id для сервисов, принимающих числовой
// идентификатор, username -- для тех, что принимают логин (Delete, ApproverService.Create).
type createdFakeUser struct {
	id       int
	username string
}

// userShareCount -- сколько из total пользователей партии заводятся в нестандартном
// состоянии/роли (заблокирован, архивный, администратор). Не менее одного при total>0 --
// иначе на маленьком профиле стенду не на чем проверить это состояние (см. постановку
// #1682, том 5).
func userShareCount(total, divisor int) int {
	if total <= 0 {
		return 0
	}
	n := total / divisor
	if n < 1 {
		n = 1
	}
	return n
}

func bannedUserCount(total int) int   { return userShareCount(total, 12) }
func archivedUserCount(total int) int { return userShareCount(total, 10) }
func adminUserCount(total int) int    { return userShareCount(total, 10) }

// approverUserCount -- сколько пользователей заводим принимающими. "Несколько", не
// пропорционально бесконечно -- потолок независимо от объёма профиля.
func approverUserCount(total int) int {
	n := userShareCount(total, 10)
	const approverCap = 20
	if n > approverCap {
		n = approverCap
	}
	return n
}

// userStreams -- независимые потоки случайности для наливки пользователей, см.
// employeeStreams в registries.go: домены разведены, чтобы правка одного не сдвигала
// остальные при повторе с тем же -seed.
type userStreams struct {
	names        *Stream
	positions    *Stream
	phone        *Stream
	typeDraw     *Stream
	usernameNum  *Stream
	orgDraw      *Stream
	orgPick      *Stream
	companyDraw  *Stream
	companyPick  *Stream
	banReason    *Stream
	approverPick *Stream
}

func newUserStreams(seed int64) *userStreams {
	return &userStreams{
		names:        NewStream(seed, "user-names"),
		positions:    NewStream(seed, "user-positions"),
		phone:        NewStream(seed, "user-phone"),
		typeDraw:     NewStream(seed, "user-type"),
		usernameNum:  NewStream(seed, "user-username"),
		orgDraw:      NewStream(seed, "user-org-draw"),
		orgPick:      NewStream(seed, "user-org-pick"),
		companyDraw:  NewStream(seed, "user-company-draw"),
		companyPick:  NewStream(seed, "user-company-pick"),
		banReason:    NewStream(seed, "user-ban-reason"),
		approverPick: NewStream(seed, "user-approver-pick"),
	}
}

// userAffiliationRefs -- id организаций/компаний, к которым можно привязать пользователя.
type userAffiliationRefs struct {
	organizationIDs []int
	companyIDs      []int
}

// loadUserAffiliationRefs читает организации и компании ЧЕРЕЗ сервисы (не прямым SQL),
// как остальные шаги наливки -- только проверенные (ModerationApproved), см.
// loadRegistryRefs в registries.go. Компании отдельной функцией, а не переиспользованием
// loadRegistryRefs: та дополнительно требует непустые гражданства/марки/форматы,
// пользователям это не нужно, и такая связка сделала бы шаг пользователей зависимым от
// справочников, которые ему не нужны.
func loadUserAffiliationRefs(ctx context.Context, db *gorm.DB) (userAffiliationRefs, error) {
	var refs userAffiliationRefs

	orgs, err := services.NewOrganizationService(db).GetAll(ctx)
	if err != nil {
		return refs, fmt.Errorf("организации для пользователей: %w", err)
	}
	for _, o := range orgs {
		if o.ModerationStatus == models.ModerationApproved {
			refs.organizationIDs = append(refs.organizationIDs, o.ID)
		}
	}
	if len(refs.organizationIDs) == 0 {
		return refs, fmt.Errorf("в базе нет ни одной проверенной организации -- пользователей привязывать " +
			"не к чему (шаг организаций должен был выполниться раньше и оставить хотя бы одну)")
	}

	companies, err := services.NewCompanyService(db).GetAll(ctx)
	if err != nil {
		return refs, fmt.Errorf("компании для пользователей: %w", err)
	}
	for _, c := range companies {
		if c.ModerationStatus == models.ModerationApproved {
			refs.companyIDs = append(refs.companyIDs, c.ID)
		}
	}
	// Пустой список компаний не останавливает шаг: организации достаточно (см.
	// drawAffiliation) -- как и у сотрудников/машин реестра, компания необязательна.

	return refs, nil
}

// loadUserTypes читает типы пользователей через UserTypeService. Пустым список не
// бывает на смигрированной базе (Seed в migrate.go всегда заводит шесть системных типов),
// но проверка остаётся -- падать честно лучше, чем Pick на пустом словаре.
func loadUserTypes(ctx context.Context, db *gorm.DB) ([]services.UserTypeWithCount, error) {
	types, err := services.NewUserTypeService(db).GetAllWithCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("типы пользователей: %w", err)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("в базе нет ни одного типа пользователей -- это не ожидается на " +
			"смигрированной базе (миграция всегда сеет user_types)")
	}
	return types, nil
}

// pickUserType выбирает тип пользователя взвешенным случайным броском (userTypeWeights).
func pickUserType(s *Stream, types []services.UserTypeWithCount) services.UserTypeWithCount {
	total := 0
	weights := make([]int, len(types))
	for i, t := range types {
		w, ok := userTypeWeights[t.Code]
		if !ok {
			w = userTypeDefaultWeight
		}
		weights[i] = w
		total += w
	}
	roll := IntRange(s, 0, total-1)
	for i, w := range weights {
		if roll < w {
			return types[i]
		}
		roll -= w
	}
	return types[len(types)-1] // недостижимо при total>0, страховка от ошибки округления
}

// drawAffiliation выбирает организацию и/или компанию для нового пользователя. Хотя бы
// одно возвращается ненулевым всегда -- userService.Create требует orgID>0 или
// companyID>0, а непустой пул organizationIDs гарантирован loadUserAffiliationRefs.
func drawAffiliation(refs userAffiliationRefs, s *userStreams) (orgID, companyID int) {
	if Chance(s.orgDraw, 0.8) {
		orgID = Pick(s.orgPick, refs.organizationIDs)
	}
	if len(refs.companyIDs) > 0 && (orgID == 0 || Chance(s.companyDraw, 0.3)) {
		companyID = Pick(s.companyPick, refs.companyIDs)
	}
	if orgID == 0 && companyID == 0 {
		orgID = Pick(s.orgPick, refs.organizationIDs)
	}
	return
}

// randomUsername собирает логин из кода типа и случайного числа -- читаемо и легко
// набрать тому, кто будет входить под этим пользователем на стенде.
func randomUsername(s *Stream, typeCode string) string {
	return fmt.Sprintf("%s%d", typeCode, IntRange(s, 1, userUsernameMaxNumber))
}

// buildUserRequest собирает запрос на создание пользователя. Возвращает и сам логин --
// он нужен вызывающему коду отдельно от req (например, для Delete/архивации по username).
func buildUserRequest(refs userAffiliationRefs, types []services.UserTypeWithCount, password string, s *userStreams) (models.RegisterRequest, string) {
	name := RandomFullName(s.names)
	position := RandomPosition(s.positions)
	phone := Phone(s.phone)
	typ := pickUserType(s.typeDraw, types)
	username := randomUsername(s.usernameNum, typ.Code)
	email := username + "@example.test"
	orgID, companyID := drawAffiliation(refs, s)

	req := models.RegisterRequest{
		Username:       username,
		Password:       password,
		OrganizationID: orgID,
		CompanyID:      companyID,
		TypeID:         typ.ID,
		LastName:       &name.LastName,
		FirstName:      &name.FirstName,
		MiddleName:     &name.MiddleName,
		Position:       &position,
		Email:          &email,
		Phone:          &phone,
	}
	return req, username
}

// createFakeUser создаёт пользователя, повторяя попытку со свежими случайными данными при
// конфликте логина (см. userCreateRetries). userService.Create не возвращает id созданной
// записи -- resolveFakeUserID достаёт его отдельным чтением по username сразу после успеха.
func createFakeUser(ctx context.Context, db *gorm.DB, svc services.UserService, callerUserID int, refs userAffiliationRefs, types []services.UserTypeWithCount, password string, s *userStreams) (createdFakeUser, error) {
	var lastErr error
	for attempt := 0; attempt < userCreateRetries; attempt++ {
		req, username := buildUserRequest(refs, types, password, s)
		if err := svc.Create(ctx, callerUserID, req); err != nil {
			if !isDuplicateUsernameConflict(err) {
				return createdFakeUser{}, err
			}
			lastErr = err
			continue
		}
		id, err := resolveFakeUserID(ctx, db, username)
		if err != nil {
			return createdFakeUser{}, err
		}
		if err := clearFakeUserPasswordChange(ctx, db, id); err != nil {
			return createdFakeUser{}, err
		}
		return createdFakeUser{id: id, username: username}, nil
	}
	return createdFakeUser{}, fmt.Errorf("не удалось создать пользователя за %d попыток, логин конфликтует: %w", userCreateRetries, lastErr)
}

// clearFakeUserPasswordChange снимает с налитого работника требование задать свой
// пароль при первом входе.
//
// Заведение учётной записи поднимает этот признак всем: живому работнику пароль
// придумывает система, и менять его при первом входе он обязан. Налитому паролем
// служит общий `-user-pass`, придумывать ему нечего, а признак закрывает весь
// защищённый API до смены пароля -- зайти под налитым работником и посмотреть
// стенд его глазами стало бы нельзя. Та же беда, что и с согласием на обработку
// данных ниже, и лечится так же.
func clearFakeUserPasswordChange(ctx context.Context, db *gorm.DB, userID int) error {
	if err := db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Update("must_change_password", false).Error; err != nil {
		return fmt.Errorf("снятие требования сменить пароль у пользователя %d: %w", userID, err)
	}
	return nil
}

// consentUserAgent -- чем записано согласие созданного работника. Живой человек
// расписывается из браузера, и там в pd_consents ложится его строка агента; здесь
// подписи не было вовсе, и врать про браузер нельзя -- в истории 152-ФЗ должно быть
// видно, что согласие проставила наливка стенда.
const consentUserAgent = "server fake (наливка стенда)"

// consentIP -- адрес, с которого записано согласие. Наливка идёт на самом сервере,
// поэтому петля, а не выдуманный внешний адрес.
const consentIP = "127.0.0.1"

// grantFakeUserConsent расписывает созданного работника за согласие на обработку
// персональных данных (#1567).
//
// Без этого стенд с включённым запросом согласия (legal.pd_consent_required) встречает
// каждого налитого работника окном согласия и до самой системы не пускает: гейт
// сравнивает принятую редакцию с требуемой, а принятой у него не было ни одной. Данные
// в списках при этом есть -- проверить их под работником нельзя, и выглядит это не как
// настройка стенда, а как сломанный вход.
//
// Редакция и отпечаток берутся из настроек стенда (Env.Consent): согласие «первой
// редакции» на стенде, где текст уже правили, гейт не устроит.
func grantFakeUserConsent(ctx context.Context, svc services.ConsentService, stamp ConsentStamp, userID int) error {
	_, err := svc.Grant(ctx, userID, models.GrantConsentRequest{ConsentType: services.ConsentTypePDProcessing},
		consentIP, consentUserAgent, stamp.Version, stamp.Hash)
	return err
}

// resolveFakeUserID читает id только что созданного пользователя по username, см.
// actorUsername в registries.go -- тот же приём чтения напрямую, но в обратную сторону.
func resolveFakeUserID(ctx context.Context, db *gorm.DB, username string) (int, error) {
	var id int
	if err := db.WithContext(ctx).Table("users").Select("id").Where("username = ?", username).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("не удалось определить id только что созданного пользователя %q: %w", username, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("пользователь %q создан, но не находится в базе", username)
	}
	return id, nil
}

// isDuplicateUsernameConflict распознаёт отказ Create по занятому логину (см.
// userService.Create в user_service.go: "unique"/"duplicate" в ошибке БД превращается в
// echo 400 "Username already exists") -- единственный случай, когда наливке стоит
// попробовать другой логин вместо остановки шага с ошибкой.
func isDuplicateUsernameConflict(err error) bool {
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		return false
	}
	msg, ok := httpErr.Message.(string)
	return ok && msg == "Username already exists"
}

// boolPtr -- маленький хелпер для полей запросов OrganizationUserRequest/CompanyUserRequest
// (*bool): взять адрес литерала в Go без временной переменной нельзя.
func boolPtr(b bool) *bool { return &b }

// approverRow -- строка состава ответственных так, как она лежит в базе.
//
// Читается сырым запросом, а не через GetOrganizationUsers/GetUsers: те отдают состав
// для интерфейса и отбрасывают архивных пользователей. Запись же (UpdateOrganizationUsers)
// заменяет список целиком, поэтому состав, собранный без архивных, физически удалил бы
// их строки при повторном прогоне -- вместе с историей того, кто был согласующим.
type approverRow struct {
	Username         string
	IsPrimary        bool
	RequiredApproval bool
	// IsActive -- жив ли пользователь строки. Архивный согласующий заявку согласовать не
	// может, поэтому «согласующий уже есть» считается только по активным, а в запись
	// состава идут все строки, включая архивные.
	IsActive bool
}

func loadOrganizationApproverRows(ctx context.Context, db *gorm.DB, orgID int) ([]approverRow, error) {
	var rows []approverRow
	err := db.WithContext(ctx).Raw(`
		SELECT u.username, ou.is_primary, ou.required_approval, u.is_active
		FROM organization_users ou
		JOIN users u ON u.id = ou.user_id
		WHERE ou.organization_id = ?
		ORDER BY ou.id`, orgID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("состав ответственных организации %d: %w", orgID, err)
	}
	return rows, nil
}

func loadCompanyApproverRows(ctx context.Context, db *gorm.DB, companyID int) ([]approverRow, error) {
	var rows []approverRow
	err := db.WithContext(ctx).Raw(`
		SELECT u.username, cu.is_primary, cu.required_approval, u.is_active
		FROM companies_users cu
		JOIN users u ON u.id = cu.user_id
		WHERE cu.company_id = ?
		ORDER BY cu.id`, companyID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("состав ответственных компании %d: %w", companyID, err)
	}
	return rows, nil
}

func hasRequiredApprovalRow(rows []approverRow) bool {
	for _, r := range rows {
		if r.RequiredApproval && r.IsActive {
			return true
		}
	}
	return false
}

// hasPrimaryRow смотрит на все строки, включая архивные: сервис отказывает, если в
// присланном составе двое главных ответственных, и архивность его не интересует.
func hasPrimaryRow(rows []approverRow) bool {
	for _, r := range rows {
		if r.IsPrimary {
			return true
		}
	}
	return false
}

// --- согласующие организаций ---

// ensureOrganizationApprovers гарантирует каждой проверенной организации хотя бы одного
// согласующего (organization_users.required_approval) -- от этих строк зависит, кто
// увидит будущую заявку на согласование (см. buildResponsibleUsers в application_service.go).
// Читает-мержит-пишет: организацию, где required_approval уже есть, не трогает -- повторный
// прогон не должен перетасовывать состав, заведённый вручную или предыдущей партией.
func ensureOrganizationApprovers(ctx context.Context, db *gorm.DB, orgIDs []int, pool []createdFakeUser, actorID int, s *userStreams) error {
	if len(pool) == 0 {
		// Активных кандидатов нет (крайний случай -- очень маленькое ручное
		// переопределение -users, см. Run): организациям нечем назначать согласующих.
		return nil
	}
	svc := services.NewOrganizationService(db)
	for _, orgID := range orgIDs {
		existing, err := loadOrganizationApproverRows(ctx, db, orgID)
		if err != nil {
			return err
		}
		if hasRequiredApprovalRow(existing) {
			continue
		}
		candidate := Pick(s.approverPick, pool)
		req := mergeOrgApproverRequest(existing, candidate.username, !hasPrimaryRow(existing))
		if err := svc.UpdateOrganizationUsers(ctx, actorID, orgID, req); err != nil {
			return fmt.Errorf("назначение согласующего организации %d: %w", orgID, err)
		}
	}
	return nil
}

// mergeOrgApproverRequest сохраняет существующий состав ответственных и добавляет нового
// согласующего -- UpdateOrganizationUsers заменяет весь список, а не дополняет его.
func mergeOrgApproverRequest(existing []approverRow, newUsername string, makePrimary bool) services.UpdateOrganizationUsersRequest {
	users := make([]services.OrganizationUserRequest, 0, len(existing)+1)
	for _, u := range existing {
		users = append(users, services.OrganizationUserRequest{
			Username:         u.Username,
			IsPrimary:        boolPtr(u.IsPrimary),
			RequiredApproval: boolPtr(u.RequiredApproval),
		})
	}
	users = append(users, services.OrganizationUserRequest{
		Username:         newUsername,
		IsPrimary:        boolPtr(makePrimary),
		RequiredApproval: boolPtr(true),
	})
	return services.UpdateOrganizationUsersRequest{Users: users}
}

// --- согласующие компаний (зеркало ensureOrganizationApprovers) ---

func ensureCompanyApprovers(ctx context.Context, db *gorm.DB, companyIDs []int, pool []createdFakeUser, actorID int, s *userStreams) error {
	if len(pool) == 0 {
		return nil
	}
	svc := services.NewCompanyService(db)
	for _, companyID := range companyIDs {
		existing, err := loadCompanyApproverRows(ctx, db, companyID)
		if err != nil {
			return err
		}
		if hasRequiredApprovalRow(existing) {
			continue
		}
		candidate := Pick(s.approverPick, pool)
		req := mergeCompanyApproverRequest(existing, candidate.username, !hasPrimaryRow(existing))
		if err := svc.UpdateUsers(ctx, actorID, companyID, req); err != nil {
			return fmt.Errorf("назначение согласующего компании %d: %w", companyID, err)
		}
	}
	return nil
}

func mergeCompanyApproverRequest(existing []approverRow, newUsername string, makePrimary bool) services.UpdateCompanyUsersRequest {
	users := make([]services.CompanyUserRequest, 0, len(existing)+1)
	for _, u := range existing {
		users = append(users, services.CompanyUserRequest{
			Username:         u.Username,
			IsPrimary:        boolPtr(u.IsPrimary),
			RequiredApproval: boolPtr(u.RequiredApproval),
		})
	}
	users = append(users, services.CompanyUserRequest{
		Username:         newUsername,
		IsPrimary:        boolPtr(makePrimary),
		RequiredApproval: boolPtr(true),
	})
	return services.UpdateCompanyUsersRequest{Users: users}
}
