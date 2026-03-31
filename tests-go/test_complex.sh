#!/bin/bash
# Test Suite 4: Complex domains (unique-cars, unique-employees, feedback, unload-places, system-tables)
BASE="http://localhost:8090"
P=0; F=0
check() { local n="$1" e="$2" a="$3"; if [ "$a" = "$e" ]; then echo "PASS $n"; P=$((P+1)); else echo "FAIL $n (expected=$e got=$a)"; F=$((F+1)); fi; }

S=$(cat /dev/urandom | tr -dc 'a-z0-9' | head -c 8)
# Register admin + regular user
curl -s -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"cx_admin_$S\",\"password\":\"Pass123!\",\"type_id\":6,\"organization_id\":4,\"company_id\":2,\"last_name\":\"A\",\"first_name\":\"U\",\"position\":\"A\"}" >/dev/null
curl -s -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"cx_user_$S\",\"password\":\"Pass123!\",\"type_id\":1,\"organization_id\":4,\"company_id\":2,\"last_name\":\"U\",\"first_name\":\"U\",\"position\":\"U\"}" >/dev/null

AT=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"cx_admin_$S\",\"password\":\"Pass123!\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)
UT=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"cx_user_$S\",\"password\":\"Pass123!\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)

# === Unique Cars ===
R=$(curl -s -w "\n%{http_code}" "$BASE/unique-cars?filter_type=all" -H "Authorization: Bearer $AT")
check "ucar_get_all" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/unique-cars?filter_type=user" -H "Authorization: Bearer $UT")
check "ucar_get_user" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/unique-cars/ownership-info" -H "Authorization: Bearer $UT")
check "ucar_ownership" "200" "$(echo "$R"|tail -1)"

CBODY=$(curl -s -X POST $BASE/unique-cars -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d "{\"number\":\"A${S}123\",\"mark\":\"Toyota\"}")
UCID=$(echo "$CBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$UCID" ] && [ "$UCID" != "" ]; check "ucar_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$UCID" ] && [ "$UCID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/unique-cars/$UCID" -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d "{\"number\":\"B${S}456\",\"mark\":\"Honda\"}")
    check "ucar_update" "200" "$(echo "$R"|tail -1)"
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/unique-cars/$UCID" -H "Authorization: Bearer $UT")
    check "ucar_delete" "200" "$(echo "$R"|tail -1)"
fi

# === Unique Employees ===
R=$(curl -s -w "\n%{http_code}" "$BASE/unique-employees?filter_type=all" -H "Authorization: Bearer $AT")
check "uemp_get_all" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/unique-employees/ownership-info" -H "Authorization: Bearer $UT")
check "uemp_ownership" "200" "$(echo "$R"|tail -1)"

EBODY=$(curl -s -X POST $BASE/unique-employees -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d "{\"last_name\":\"Тест\",\"first_name\":\"Сотрудник\",\"passport_series_number\":\"${S}1234\"}")
UEID=$(echo "$EBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$UEID" ] && [ "$UEID" != "" ]; check "uemp_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$UEID" ] && [ "$UEID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/unique-employees/$UEID" -H "Authorization: Bearer $UT")
    check "uemp_delete" "200" "$(echo "$R"|tail -1)"
fi

# === Feedback ===
FBODY=$(curl -s -X POST $BASE/feedback -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{"message":"Test feedback message"}')
FID=$(echo "$FBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$FID" ] && [ "$FID" != "" ]; check "fb_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

R=$(curl -s -w "\n%{http_code}" "$BASE/feedback/my" -H "Authorization: Bearer $UT")
check "fb_my" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list) and len(d)>0" 2>/dev/null
check "fb_my_has_items" "0" "$?"

R=$(curl -s -w "\n%{http_code}" "$BASE/feedback/all" -H "Authorization: Bearer $AT")
check "fb_all_admin" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/feedback/stats" -H "Authorization: Bearer $AT")
check "fb_stats" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert 'total' in d and 'unread' in d" 2>/dev/null
check "fb_stats_fields" "0" "$?"

if [ -n "$FID" ] && [ "$FID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/feedback/$FID/status" -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"status":"Решено"}')
    check "fb_update_status" "200" "$(echo "$R"|tail -1)"
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/feedback/$FID/read" -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"is_read":true}')
    check "fb_mark_read" "200" "$(echo "$R"|tail -1)"
fi

# === Unload Places ===
R=$(curl -s -w "\n%{http_code}" $BASE/unload-places -H "Authorization: Bearer $AT")
check "up_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list)" 2>/dev/null
check "up_is_array" "0" "$?"

# Get existing by ID
FIRST_UP=$(echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['id'] if d else '')" 2>/dev/null)
if [ -n "$FIRST_UP" ]; then
    R=$(curl -s -w "\n%{http_code}" "$BASE/unload-places/$FIRST_UP" -H "Authorization: Bearer $AT")
    check "up_get_by_id" "200" "$(echo "$R"|tail -1)"
    echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert 'current_status' in d and 'time_slots' in d" 2>/dev/null
    check "up_has_details" "0" "$?"

    R=$(curl -s -w "\n%{http_code}" "$BASE/unload-places/$FIRST_UP/time-slots" -H "Authorization: Bearer $AT")
    check "up_time_slots" "200" "$(echo "$R"|tail -1)"
fi

# === System Tables ===
R=$(curl -s -w "\n%{http_code}" $BASE/system-tables -H "Authorization: Bearer $AT")
check "st_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list)" 2>/dev/null
check "st_is_array" "0" "$?"

FIRST_ST=$(echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['table']['id'] if d else '')" 2>/dev/null)
if [ -n "$FIRST_ST" ]; then
    R=$(curl -s -w "\n%{http_code}" "$BASE/system-tables/$FIRST_ST" -H "Authorization: Bearer $AT")
    check "st_get_by_id" "200" "$(echo "$R"|tail -1)"
    echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert 'current_status' in d and 'fields' in d" 2>/dev/null
    check "st_has_details" "0" "$?"

    R=$(curl -s -w "\n%{http_code}" "$BASE/system-tables/$FIRST_ST/time-slots" -H "Authorization: Bearer $AT")
    check "st_time_slots" "200" "$(echo "$R"|tail -1)"
fi

# Cleanup
curl -s -X DELETE "$BASE/users/cx_admin_$S" -H "Authorization: Bearer $AT" >/dev/null
curl -s -X DELETE "$BASE/users/cx_user_$S" -H "Authorization: Bearer $AT" >/dev/null

echo ""
echo "=== Complex Domains: PASSED=$P FAILED=$F TOTAL=$((P+F)) ==="
