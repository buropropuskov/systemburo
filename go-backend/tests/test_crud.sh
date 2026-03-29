#!/bin/bash
# Test Suite 2: CRUD domains (citizenships, license-formats, attachments, user-types-management)
BASE="http://localhost:8090"
P=0; F=0
check() { local n="$1" e="$2" a="$3"; if [ "$a" = "$e" ]; then echo "PASS $n"; P=$((P+1)); else echo "FAIL $n (expected=$e got=$a)"; F=$((F+1)); fi; }

S=$(cat /dev/urandom | tr -dc 'a-z0-9' | head -c 8)
curl -s -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"crud_$S\",\"password\":\"Pass123!\",\"type_id\":6,\"organization_id\":4,\"company_id\":2,\"last_name\":\"T\",\"first_name\":\"U\",\"position\":\"T\"}" >/dev/null
BODY=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"crud_$S\",\"password\":\"Pass123!\"}")
T=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)

# === Citizenships ===
R=$(curl -s -w "\n%{http_code}" $BASE/citizenships -H "Authorization: Bearer $T")
check "cit_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; assert isinstance(json.load(sys.stdin),list)" 2>/dev/null
check "cit_is_array" "0" "$?"

# Create
CBODY=$(curl -s -X POST $BASE/citizenships -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"name":"TestCitizenship","code":"ZZ","is_default":false}')
CID=$(echo "$CBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$CID" ] && [ "$CID" != "" ]; check "cit_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

# Update
R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/citizenships/$CID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"name":"UpdatedCit","code":"YY","is_default":false}')
check "cit_update" "200" "$(echo "$R"|tail -1)"

# Delete
R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/citizenships/$CID" -H "Authorization: Bearer $T")
check "cit_delete" "200" "$(echo "$R"|tail -1)"

# Clear defaults
R=$(curl -s -w "\n%{http_code}" -X POST "$BASE/citizenships/clear-default" -H "Authorization: Bearer $T")
check "cit_clear_defaults" "200" "$(echo "$R"|tail -1)"

# === License Plate Formats ===
R=$(curl -s -w "\n%{http_code}" $BASE/license-plate-formats -H "Authorization: Bearer $T")
check "lpf_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; assert isinstance(json.load(sys.stdin),list)" 2>/dev/null
check "lpf_is_array" "0" "$?"

# Create format
LBODY=$(curl -s -X POST "$BASE/license-plate-formats" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"name":"TestFormat","cells":[{"cell_order":1,"cell_type":"letters","min_length":1,"max_length":3}]}')
LID=$(echo "$LBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$LID" ] && [ "$LID" != "" ]; check "lpf_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

# Update format
if [ -n "$LID" ] && [ "$LID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/license-plate-formats/$LID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"name":"UpdatedFormat","cells":[{"cell_order":1,"cell_type":"numbers","min_length":1,"max_length":2}]}')
    check "lpf_update" "200" "$(echo "$R"|tail -1)"
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/license-plate-formats/$LID" -H "Authorization: Bearer $T")
    check "lpf_delete" "200" "$(echo "$R"|tail -1)"
fi

# === Attachments ===
R=$(curl -s -w "\n%{http_code}" $BASE/attachments -H "Authorization: Bearer $T")
check "att_active" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; assert isinstance(json.load(sys.stdin),list)" 2>/dev/null
check "att_active_array" "0" "$?"

R=$(curl -s -w "\n%{http_code}" $BASE/attachments/all -H "Authorization: Bearer $T")
check "att_all" "200" "$(echo "$R"|tail -1)"

# Create attachment
ABODY=$(curl -s -X POST $BASE/attachments -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"attachment_type\":\"cars\",\"name\":\"test_att_$S\",\"display_name\":\"Test Attachment\",\"title\":\"test\"}")
AID=$(echo "$ABODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$AID" ] && [ "$AID" != "" ]; check "att_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$AID" ] && [ "$AID" != "" ]; then
    # Get by ID
    R=$(curl -s -w "\n%{http_code}" "$BASE/attachments/$AID" -H "Authorization: Bearer $T")
    check "att_get_by_id" "200" "$(echo "$R"|tail -1)"

    # Update
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/attachments/$AID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"name":"Updated","type":"test","is_active":true}')
    check "att_update" "200" "$(echo "$R"|tail -1)"

    # Delete (soft)
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/attachments/$AID" -H "Authorization: Bearer $T")
    check "att_delete" "200" "$(echo "$R"|tail -1)"

    # Restore
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/attachments/$AID/restore" -H "Authorization: Bearer $T")
    check "att_restore" "200" "$(echo "$R"|tail -1)"
fi

# === User Types Management ===
R=$(curl -s -w "\n%{http_code}" $BASE/user-types-management -H "Authorization: Bearer $T")
check "utm_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; assert isinstance(json.load(sys.stdin),list)" 2>/dev/null
check "utm_is_array" "0" "$?"

# Create user type
UBODY=$(curl -s -X POST "$BASE/user-types-management" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"TestType_$S\",\"code\":\"tt_$S\"}")
UTID=$(echo "$UBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$UTID" ] && [ "$UTID" != "" ]; check "utm_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$UTID" ] && [ "$UTID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/user-types-management/$UTID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"Updated_$S\",\"code\":\"ut_$S\"}")
    check "utm_update" "200" "$(echo "$R"|tail -1)"
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/user-types-management/$UTID" -H "Authorization: Bearer $T")
    check "utm_delete" "200" "$(echo "$R"|tail -1)"
fi

# Cleanup
curl -s -X DELETE "$BASE/users/crud_$S" -H "Authorization: Bearer $T" >/dev/null

echo ""
echo "=== CRUD Domains: PASSED=$P FAILED=$F TOTAL=$((P+F)) ==="
