package services

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Режимы групповой привязки набора (места разгрузки, таблицы, ответственные).
const (
	// BulkModeReplace затирает текущий набор выбранным (поведение одиночных методов).
	BulkModeReplace = "replace"
	// BulkModeAdd объединяет выбранное с текущим (union), не отвязывая существующее.
	BulkModeAdd = "add"
)

// BulkItemError — ошибка по одной сущности в групповой операции (частичный успех).
type BulkItemError struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

// BulkOpResult — результат групповой операции над справочником: успешные
// применены, неуспешные собраны в Errors. По образцу BatchCreateCarsResponse:
// операция не падает целиком, статус 207 при наличии ошибок.
type BulkOpResult struct {
	SuccessCount int             `json:"success_count"`
	ErrorCount   int             `json:"error_count"`
	Errors       []BulkItemError `json:"errors"`
}

// newBulkResult создаёт результат с непустым (не nil) срезом ошибок - чтобы в
// JSON всегда было [] вместо null.
func newBulkResult() *BulkOpResult {
	return &BulkOpResult{Errors: []BulkItemError{}}
}

// addError фиксирует провал по одной сущности.
func (r *BulkOpResult) addError(id int, name, msg string) {
	r.Errors = append(r.Errors, BulkItemError{ID: id, Name: name, Error: msg})
}

// finalize проставляет ErrorCount по накопленным ошибкам.
func (r *BulkOpResult) finalize() *BulkOpResult {
	r.ErrorCount = len(r.Errors)
	return r
}

// HTTPStatus — 200 при полном успехе, 207 (MultiStatus) при частичном/полном провале.
func (r *BulkOpResult) HTTPStatus() int {
	if r.ErrorCount > 0 {
		return http.StatusMultiStatus
	}
	return http.StatusOK
}

// --- DTO запросов групповых операций (общие для организаций и компаний) ---

// BulkIDsRequest — тело для архива/восстановления: только список ID.
type BulkIDsRequest struct {
	IDs []int `json:"ids"`
}

// BulkTypeRequest — групповая смена типа. Type=nil снимает тип.
type BulkTypeRequest struct {
	IDs  []int   `json:"ids"`
	Type *string `json:"type"`
}

// BulkUnloadPlacesRequest — групповое назначение мест разгрузки с режимом.
type BulkUnloadPlacesRequest struct {
	IDs            []int  `json:"ids"`
	UnloadPlaceIDs []int  `json:"unload_place_ids"`
	Mode           string `json:"mode"`
}

// BulkTablesRequest — групповое назначение целевых таблиц с режимом.
type BulkTablesRequest struct {
	IDs      []int  `json:"ids"`
	TableIDs []int  `json:"table_ids"`
	Mode     string `json:"mode"`
}

// BulkUserAssignment — назначаемый ответственный с индивидуальным флагом
// обязательного согласования (задаётся на каждого отдельно, как в детали).
type BulkUserAssignment struct {
	Username         string `json:"username"`
	RequiredApproval bool   `json:"required_approval"`
}

// BulkUsersRequest — групповое назначение ответственных с режимом. primary в
// группе не назначается (per-entity деталь); обязательное согласование
// индивидуально на каждого назначаемого.
type BulkUsersRequest struct {
	IDs   []int                `json:"ids"`
	Users []BulkUserAssignment `json:"users"`
	Mode  string               `json:"mode"`
}

// --- DTO групповых операций над пользователями (ключ - username, не int id) ---

// BulkUsernamesRequest — тело для группового архива/восстановления пользователей.
type BulkUsernamesRequest struct {
	Usernames []string `json:"usernames"`
}

// BulkUserTypeRequest — групповая смена типа пользователей.
type BulkUserTypeRequest struct {
	Usernames []string `json:"usernames"`
	TypeID    int      `json:"type_id"`
}

// BulkUserOrganizationRequest — групповое назначение организации пользователям.
type BulkUserOrganizationRequest struct {
	Usernames      []string `json:"usernames"`
	OrganizationID int      `json:"organization_id"`
}

// BulkUserCompanyRequest — групповое назначение компании пользователям.
type BulkUserCompanyRequest struct {
	Usernames []string `json:"usernames"`
	CompanyID int      `json:"company_id"`
}

// BulkUserBanRequest — групповая блокировка пользователей. Reason опционален
// (причина показывается заблокированному в ЛК), пустой -> без причины.
type BulkUserBanRequest struct {
	Usernames []string `json:"usernames"`
	Reason    string   `json:"reason"`
}

// --- Хелперы ---

// isValidBulkMode проверяет режим привязки.
func isValidBulkMode(mode string) bool {
	return mode == BulkModeReplace || mode == BulkModeAdd
}

// uniqueInts убирает дубликаты из набора id с сохранением порядка. Дубли в
// ids раздули бы SuccessCount (одна сущность посчиталась бы дважды).
func uniqueInts(a []int) []int {
	return unionInts(a, nil)
}

// uniqueStrings убирает дубликаты из набора username с сохранением порядка (дубли
// раздули бы SuccessCount - один пользователь посчитался бы дважды).
func uniqueStrings(a []string) []string {
	seen := make(map[string]struct{}, len(a))
	out := make([]string, 0, len(a))
	for _, x := range a {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// unionInts объединяет два набора int с сохранением порядка (сначала a, затем
// новые из b), без дубликатов. Для режима add: текущие связи + выбранные.
func unionInts(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, src := range [][]int{a, b} {
		for _, x := range src {
			if _, ok := seen[x]; ok {
				continue
			}
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}

// bulkErrMsg достаёт человекочитаемое сообщение из ошибки одиночного метода
// (echo.HTTPError), чтобы положить его в BulkItemError.Error.
func bulkErrMsg(err error) string {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		if msg, ok := he.Message.(string); ok {
			return msg
		}
	}
	return "Ошибка обработки"
}
