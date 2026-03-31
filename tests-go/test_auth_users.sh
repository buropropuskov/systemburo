#!/bin/bash
# Test Suite 1: Auth + Users Management
BASE="http://localhost:8090"
P=0; F=0
check() { local n="$1" e="$2" a="$3"; if [ "$a" = "$e" ]; then echo "PASS $n"; P=$((P+1)); else echo "FAIL $n (expected=$e got=$a)"; F=$((F+1)); fi; }

S=$(cat /dev/urandom | tr -dc 'a-z0-9' | head -c 8)

# --- Register ---
R=$(curl -s -w "\n%{http_code}" -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"au_user_$S\",\"password\":\"Pass123!\",\"type_id\":1,\"organization_id\":4,\"company_id\":2,\"last_name\":\"T\",\"first_name\":\"U\",\"position\":\"T\"}")
check "register_user" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"au_admin_$S\",\"password\":\"Pass123!\",\"type_id\":6,\"organization_id\":4,\"company_id\":2,\"last_name\":\"A\",\"first_name\":\"U\",\"position\":\"A\"}")
check "register_admin" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"au_user_$S\",\"password\":\"Pass123!\",\"type_id\":1,\"organization_id\":4,\"company_id\":2,\"last_name\":\"T\",\"first_name\":\"U\",\"position\":\"T\"}")
check "register_duplicate" "400" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/register -H 'Content-Type: application/json' -d '{"username":"incomplete"}')
S1=$(echo "$R"|tail -1)
[ "$S1" = "400" ] || [ "$S1" = "422" ] || [ "$S1" = "500" ]; check "register_missing_fields" "4xx/5xx" "$([ $? -eq 0 ] && echo '4xx/5xx' || echo $S1)"

# --- Login ---
BODY=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"au_user_$S\",\"password\":\"Pass123!\"}")
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
REFRESH=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))" 2>/dev/null)
[ -n "$TOKEN" ] && [ ${#TOKEN} -gt 20 ]; check "login_success_has_token" "true" "$([ $? -eq 0 ] && echo true || echo false)"
[ -n "$REFRESH" ] && [ ${#REFRESH} -gt 20 ]; check "login_success_has_refresh" "true" "$([ $? -eq 0 ] && echo true || echo false)"

# Check login response fields
echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'organization' in d and 'company' in d and 'type_id' in d and 'user_type' in d" 2>/dev/null
check "login_response_fields" "0" "$?"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"au_user_$S\",\"password\":\"Wrong!\"}")
check "login_wrong_pass" "401" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/login -H 'Content-Type: application/json' -d '{"username":"no_exist_xyz","password":"P!"}')
check "login_nonexistent" "401" "$(echo "$R"|tail -1)"

# --- Refresh Token ---
RBODY=$(curl -s -X POST $BASE/refresh-token -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$REFRESH\"}")
NT=$(echo "$RBODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
NR=$(echo "$RBODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))" 2>/dev/null)
[ -n "$NT" ] && [ ${#NT} -gt 20 ]; check "refresh_returns_new_token" "true" "$([ $? -eq 0 ] && echo true || echo false)"
TOKEN="$NT"

# Old refresh should be invalid (one-time use)
R=$(curl -s -w "\n%{http_code}" -X POST $BASE/refresh-token -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$REFRESH\"}")
check "refresh_old_token_rejected" "401" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" -X POST $BASE/refresh-token -H 'Content-Type: application/json' -d '{"refreshToken":"invalid.token.here"}')
check "refresh_invalid_token" "401" "$(echo "$R"|tail -1)"

# --- User Data ---
R=$(curl -s -w "\n%{http_code}" $BASE/user-data -H "Authorization: Bearer $TOKEN")
check "user_data_success" "200" "$(echo "$R"|tail -1)"
echo "$R" | sed '$d' | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['username']=='au_user_$S'" 2>/dev/null
check "user_data_correct_username" "0" "$?"

# --- Current User ---
R=$(curl -s -w "\n%{http_code}" $BASE/users/me -H "Authorization: Bearer $TOKEN")
check "current_user_success" "200" "$(echo "$R"|tail -1)"
echo "$R" | sed '$d' | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'user_type_code' in d and 'id' in d" 2>/dev/null
check "current_user_has_fields" "0" "$?"

# --- Logout ---
R=$(curl -s -w "\n%{http_code}" -X POST $BASE/logout -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$NR\"}")
check "logout_success" "200" "$(echo "$R"|tail -1)"

# --- Protected without token ---
R=$(curl -s -w "\n%{http_code}" $BASE/user-data)
check "no_token_401" "401" "$(echo "$R"|tail -1)"

# --- Admin login ---
ABODY=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"au_admin_$S\",\"password\":\"Pass123!\"}")
AT=$(echo "$ABODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)

# --- Users Management (admin) ---
R=$(curl -s -w "\n%{http_code}" $BASE/users/all -H "Authorization: Bearer $AT")
check "admin_get_all_users" "200" "$(echo "$R"|tail -1)"
echo "$R" | sed '$d' | python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list) and len(d)>0" 2>/dev/null
check "users_all_is_array" "0" "$?"

# Update user type
R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/users/au_user_$S/type" -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"type_id":2}')
check "update_user_type" "200" "$(echo "$R"|tail -1)"

# Update user info
R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/users/au_user_$S/info" -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"last_name":"Updated","first_name":"Name"}')
check "update_user_info" "200" "$(echo "$R"|tail -1)"

# Update user password
R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/users/au_user_$S/password" -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"password":"NewPass123!"}')
check "update_user_password" "200" "$(echo "$R"|tail -1)"

# Non-admin trying admin endpoint
UBODY=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"au_user_$S\",\"password\":\"NewPass123!\"}")
UT=$(echo "$UBODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
R=$(curl -s -w "\n%{http_code}" $BASE/users/all -H "Authorization: Bearer $UT")
S1=$(echo "$R"|tail -1)
[ "$S1" = "403" ] || [ "$S1" = "401" ]; check "non_admin_users_all_forbidden" "403/401" "$([ $? -eq 0 ] && echo '403/401' || echo $S1)"

# Delete user
R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/users/au_user_$S" -H "Authorization: Bearer $AT")
check "delete_user" "200" "$(echo "$R"|tail -1)"

# Cleanup admin
curl -s -X DELETE "$BASE/users/au_admin_$S" -H "Authorization: Bearer $AT" >/dev/null

echo ""
echo "=== Auth+Users: PASSED=$P FAILED=$F TOTAL=$((P+F)) ==="
