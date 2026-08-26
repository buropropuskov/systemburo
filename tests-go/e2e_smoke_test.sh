#!/bin/bash
# E2E smoke tests for Go backend — mirrors Rust test scenarios
# Usage: bash go-backend/tests/e2e_smoke_test.sh [BASE_URL]

BASE="${1:-http://localhost:8090}"
PASS=0
FAIL=0
ERRORS=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

assert_status() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"
    local body="$4"
    if [ "$actual" = "$expected" ]; then
        echo -e "  ${GREEN}PASS${NC} $test_name (HTTP $actual)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}FAIL${NC} $test_name — expected $expected, got $actual"
        [ -n "$body" ] && echo "       Body: $(echo "$body" | head -c 200)"
        FAIL=$((FAIL + 1))
        ERRORS="$ERRORS\n  FAIL: $test_name (expected $expected, got $actual)"
    fi
}

assert_json_field() {
    local test_name="$1"
    local body="$2"
    local field="$3"
    local val
    val=$(echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$field','__MISSING__'))" 2>/dev/null)
    if [ "$val" != "__MISSING__" ] && [ -n "$val" ]; then
        echo -e "  ${GREEN}PASS${NC} $test_name (field '$field' present)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}FAIL${NC} $test_name — field '$field' missing in response"
        FAIL=$((FAIL + 1))
        ERRORS="$ERRORS\n  FAIL: $test_name (field '$field' missing)"
    fi
}

assert_array() {
    local test_name="$1"
    local body="$2"
    local is_arr
    is_arr=$(echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if isinstance(d, list) else 'no')" 2>/dev/null)
    if [ "$is_arr" = "yes" ]; then
        local count
        count=$(echo "$body" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null)
        echo -e "  ${GREEN}PASS${NC} $test_name (array, $count items)"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}FAIL${NC} $test_name — expected array"
        FAIL=$((FAIL + 1))
        ERRORS="$ERRORS\n  FAIL: $test_name (not array)"
    fi
}

# Helper: HTTP request
req() {
    local method="$1" path="$2" token="$3" data="$4"
    local args=("-s" "-w" "\n%{http_code}" "-X" "$method")
    [ -n "$token" ] && args+=("-H" "Authorization: Bearer $token")
    [ -n "$data" ] && args+=("-H" "Content-Type: application/json" "-d" "$data")
    curl "${args[@]}" "${BASE}${path}"
}

parse_response() {
    local resp="$1"
    BODY=$(echo "$resp" | sed '$d')
    STATUS=$(echo "$resp" | tail -1)
}

SUFFIX=$(date +%s)

echo -e "\n${YELLOW}=== E2E Smoke Tests: $BASE ===${NC}\n"

# ============================
echo -e "${YELLOW}--- Auth ---${NC}"
# ============================

# Register success
parse_response "$(req POST /register '' "{\"username\":\"gotest_${SUFFIX}\",\"password\":\"TestPass123!\",\"type_id\":1,\"organization_id\":4,\"company_id\":2,\"last_name\":\"Test\",\"first_name\":\"User\",\"position\":\"Tester\"}")"
assert_status "register_success" "200" "$STATUS" "$BODY"

# Register duplicate
parse_response "$(req POST /register '' "{\"username\":\"gotest_${SUFFIX}\",\"password\":\"TestPass123!\",\"type_id\":1,\"organization_id\":4,\"company_id\":2,\"last_name\":\"Test\",\"first_name\":\"User\",\"position\":\"Tester\"}")"
assert_status "register_duplicate" "400" "$STATUS" "$BODY"

# Login success
parse_response "$(req POST /login '' "{\"username\":\"gotest_${SUFFIX}\",\"password\":\"TestPass123!\"}")"
assert_status "login_success" "200" "$STATUS" "$BODY"
assert_json_field "login_has_token" "$BODY" "token"
assert_json_field "login_has_refreshToken" "$BODY" "refreshToken"
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
REFRESH=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))" 2>/dev/null)

# Login wrong password
parse_response "$(req POST /login '' '{"username":"gotest_'"$SUFFIX"'","password":"WrongPass!"}')"
assert_status "login_wrong_password" "401" "$STATUS" "$BODY"

# Login nonexistent user
parse_response "$(req POST /login '' '{"username":"noexist_xyz","password":"AnyPass!"}')"
assert_status "login_nonexistent" "401" "$STATUS" "$BODY"

# Refresh token
parse_response "$(req POST /refresh-token '' "{\"refreshToken\":\"$REFRESH\"}")"
assert_status "refresh_token" "200" "$STATUS" "$BODY"
assert_json_field "refresh_has_token" "$BODY" "token"
NEW_TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
NEW_REFRESH=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))" 2>/dev/null)
TOKEN="$NEW_TOKEN"

# User data
parse_response "$(req GET /user-data "$TOKEN")"
assert_status "user_data" "200" "$STATUS" "$BODY"

# Current user
parse_response "$(req GET /users/me "$TOKEN")"
assert_status "current_user" "200" "$STATUS" "$BODY"

# Logout
parse_response "$(req POST /logout "$TOKEN" "{\"refreshToken\":\"$NEW_REFRESH\"}")"
assert_status "logout" "200" "$STATUS" "$BODY"

# Protected without token
parse_response "$(req GET /user-data)"
assert_status "protected_no_token" "401" "$STATUS" "$BODY"

# Re-login for further tests
parse_response "$(req POST /login '' "{\"username\":\"gotest_${SUFFIX}\",\"password\":\"TestPass123!\"}")"
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)

# Register admin (type_id=6 = buropropuskov)
parse_response "$(req POST /register '' "{\"username\":\"goadmin_${SUFFIX}\",\"password\":\"AdminPass123!\",\"type_id\":6,\"organization_id\":4,\"company_id\":2,\"last_name\":\"Admin\",\"first_name\":\"User\",\"position\":\"Admin\"}")"
assert_status "register_admin" "200" "$STATUS" "$BODY"
parse_response "$(req POST /login '' "{\"username\":\"goadmin_${SUFFIX}\",\"password\":\"AdminPass123!\"}")"
ADMIN_TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)

# ============================
echo -e "\n${YELLOW}--- Public Endpoints ---${NC}"
# ============================

parse_response "$(req GET /user-types)"
assert_status "user_types_public" "200" "$STATUS" "$BODY"
assert_array "user_types_is_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Citizenships ---${NC}"
# ============================

parse_response "$(req GET /citizenships "$ADMIN_TOKEN")"
assert_status "citizenships_get_all" "200" "$STATUS" "$BODY"
assert_array "citizenships_array" "$BODY"

parse_response "$(req POST /citizenships "$ADMIN_TOKEN" '{"name":"TestCountry","code":"TC","is_default":false}')"
assert_status "citizenships_create" "200" "$STATUS" "$BODY"
CIT_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

if [ -n "$CIT_ID" ] && [ "$CIT_ID" != "" ]; then
    parse_response "$(req PUT /citizenships/$CIT_ID "$ADMIN_TOKEN" '{"name":"TestCountryUpdated","code":"TU","is_default":false}')"
    assert_status "citizenships_update" "200" "$STATUS" "$BODY"

    parse_response "$(req DELETE /citizenships/$CIT_ID "$ADMIN_TOKEN")"
    assert_status "citizenships_delete" "200" "$STATUS" "$BODY"
fi

# ============================
echo -e "\n${YELLOW}--- License Plate Formats ---${NC}"
# ============================

parse_response "$(req GET /license-plate-formats "$ADMIN_TOKEN")"
assert_status "lpf_get_all" "200" "$STATUS" "$BODY"
assert_array "lpf_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Attachments ---${NC}"
# ============================

parse_response "$(req GET /attachments "$ADMIN_TOKEN")"
assert_status "attachments_active" "200" "$STATUS" "$BODY"
assert_array "attachments_array" "$BODY"

parse_response "$(req GET /attachments/all "$ADMIN_TOKEN")"
assert_status "attachments_all" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- User Types Management ---${NC}"
# ============================

parse_response "$(req GET /user-types-management "$ADMIN_TOKEN")"
assert_status "utm_get_all" "200" "$STATUS" "$BODY"
assert_array "utm_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Organizations ---${NC}"
# ============================

parse_response "$(req GET /organizations "$ADMIN_TOKEN")"
assert_status "orgs_get_all" "200" "$STATUS" "$BODY"
assert_array "orgs_array" "$BODY"

parse_response "$(req GET /organizations/with-users "$ADMIN_TOKEN")"
assert_status "orgs_with_users" "200" "$STATUS" "$BODY"

parse_response "$(req GET /get-organization "$TOKEN")"
assert_status "my_organization" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Companies ---${NC}"
# ============================

parse_response "$(req GET /companies "$ADMIN_TOKEN")"
assert_status "companies_get_all" "200" "$STATUS" "$BODY"
assert_array "companies_array" "$BODY"

parse_response "$(req GET /companies/with-users "$ADMIN_TOKEN")"
assert_status "companies_with_users" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Users Management ---${NC}"
# ============================

parse_response "$(req GET /users/all "$ADMIN_TOKEN")"
assert_status "users_get_all" "200" "$STATUS" "$BODY"
assert_array "users_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Unload Places ---${NC}"
# ============================

parse_response "$(req GET /unload-places "$ADMIN_TOKEN")"
assert_status "unload_places_all" "200" "$STATUS" "$BODY"
assert_array "unload_places_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- System Tables ---${NC}"
# ============================

parse_response "$(req GET /system-tables "$ADMIN_TOKEN")"
assert_status "system_tables_all" "200" "$STATUS" "$BODY"
assert_array "system_tables_array" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Cars ---${NC}"
# ============================

# Адресный путь «Проезд» (#1036): таблица с id=1 может отсутствовать - эндпоинт всё
# равно отвечает 200 с пустым массивом, поэтому дымовая проверка не зависит от данных.
parse_response "$(req GET /cars/active-for-table/1 "$ADMIN_TOKEN")"
assert_status "cars_active" "200" "$STATUS" "$BODY"

parse_response "$(req GET /cars/history/all "$ADMIN_TOKEN")"
assert_status "cars_history_all" "200" "$STATUS" "$BODY"

parse_response "$(req GET /cars/history/current-status "$ADMIN_TOKEN")"
assert_status "cars_current_status" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Unique Cars ---${NC}"
# ============================

parse_response "$(req GET '/unique-cars?filter_type=all' "$ADMIN_TOKEN")"
assert_status "unique_cars_all" "200" "$STATUS" "$BODY"

parse_response "$(req GET /unique-cars/ownership-info "$TOKEN")"
assert_status "unique_cars_ownership" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Unique Employees ---${NC}"
# ============================

parse_response "$(req GET '/unique-employees?filter_type=all' "$ADMIN_TOKEN")"
assert_status "unique_employees_all" "200" "$STATUS" "$BODY"

parse_response "$(req GET /unique-employees/ownership-info "$TOKEN")"
assert_status "unique_employees_ownership" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Feedback ---${NC}"
# ============================

parse_response "$(req POST /feedback "$TOKEN" '{"message":"Test feedback from Go"}')"
assert_status "feedback_create" "200" "$STATUS" "$BODY"

parse_response "$(req GET /feedback/my "$TOKEN")"
assert_status "feedback_my" "200" "$STATUS" "$BODY"
assert_array "feedback_my_array" "$BODY"

parse_response "$(req GET /feedback/all "$ADMIN_TOKEN")"
assert_status "feedback_all" "200" "$STATUS" "$BODY"

parse_response "$(req GET /feedback/stats "$ADMIN_TOKEN")"
assert_status "feedback_stats" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Applications ---${NC}"
# ============================

parse_response "$(req GET /applications "$ADMIN_TOKEN")"
assert_status "applications_all" "200" "$STATUS" "$BODY"

parse_response "$(req GET /applications/user "$TOKEN")"
assert_status "applications_user" "200" "$STATUS" "$BODY"

# ============================
echo -e "\n${YELLOW}--- Swagger ---${NC}"
# ============================

SWAGGER_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${BASE}/swagger/index.html")
assert_status "swagger_ui" "200" "$SWAGGER_STATUS" ""

SWAGGER_JSON_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${BASE}/swagger/doc.json")
assert_status "swagger_json" "200" "$SWAGGER_JSON_STATUS" ""

# ============================
# Cleanup test user
# ============================
parse_response "$(req DELETE /users/gotest_${SUFFIX} "$ADMIN_TOKEN")"
parse_response "$(req DELETE /users/goadmin_${SUFFIX} "$ADMIN_TOKEN")"

# ============================
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${GREEN}PASSED: $PASS${NC}  ${RED}FAILED: $FAIL${NC}  TOTAL: $((PASS + FAIL))"
if [ $FAIL -gt 0 ]; then
    echo -e "\n${RED}Failed tests:${NC}$ERRORS"
fi
echo -e "${YELLOW}========================================${NC}\n"

exit $FAIL
