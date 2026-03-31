#!/bin/bash
# Test Suite 3: Organizations + Companies
BASE="http://localhost:8090"
P=0; F=0
check() { local n="$1" e="$2" a="$3"; if [ "$a" = "$e" ]; then echo "PASS $n"; P=$((P+1)); else echo "FAIL $n (expected=$e got=$a)"; F=$((F+1)); fi; }

S=$(cat /dev/urandom | tr -dc 'a-z0-9' | head -c 8)
curl -s -X POST $BASE/register -H 'Content-Type: application/json' -d "{\"username\":\"oc_$S\",\"password\":\"Pass123!\",\"type_id\":6,\"organization_id\":4,\"company_id\":2,\"last_name\":\"T\",\"first_name\":\"U\",\"position\":\"T\"}" >/dev/null
BODY=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d "{\"username\":\"oc_$S\",\"password\":\"Pass123!\"}")
T=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null)

# === Organizations ===
R=$(curl -s -w "\n%{http_code}" $BASE/organizations -H "Authorization: Bearer $T")
check "org_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list) and len(d)>0" 2>/dev/null
check "org_is_nonempty_array" "0" "$?"

# Create org
OBODY=$(curl -s -X POST $BASE/organizations -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"TestOrg_$S\"}")
OID=$(echo "$OBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$OID" ] && [ "$OID" != "" ]; check "org_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$OID" ] && [ "$OID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/organizations/$OID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"UpdatedOrg_$S\"}")
    check "org_update" "200" "$(echo "$R"|tail -1)"
fi

R=$(curl -s -w "\n%{http_code}" "$BASE/organizations/with-users" -H "Authorization: Bearer $T")
check "org_with_users" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/organizations/with-users-extended" -H "Authorization: Bearer $T")
check "org_with_users_ext" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/get-organization" -H "Authorization: Bearer $T")
check "org_my_org" "200" "$(echo "$R"|tail -1)"

# Org users
R=$(curl -s -w "\n%{http_code}" "$BASE/organizations/4/users" -H "Authorization: Bearer $T")
check "org_get_users" "200" "$(echo "$R"|tail -1)"

# Org tables
R=$(curl -s -w "\n%{http_code}" "$BASE/organizations/4/tables" -H "Authorization: Bearer $T")
check "org_get_tables" "200" "$(echo "$R"|tail -1)"

# Org unload places
R=$(curl -s -w "\n%{http_code}" "$BASE/organizations/4/unload-places" -H "Authorization: Bearer $T")
check "org_get_unload_places" "200" "$(echo "$R"|tail -1)"

# Delete created org
if [ -n "$OID" ] && [ "$OID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/organizations/$OID" -H "Authorization: Bearer $T")
    check "org_delete" "200" "$(echo "$R"|tail -1)"
fi

# === Companies ===
R=$(curl -s -w "\n%{http_code}" $BASE/companies -H "Authorization: Bearer $T")
check "comp_get_all" "200" "$(echo "$R"|tail -1)"
echo "$R"|sed '$d'|python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d,list) and len(d)>0" 2>/dev/null
check "comp_is_nonempty_array" "0" "$?"

# Create company
CBODY=$(curl -s -X POST $BASE/companies -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"TestComp_$S\"}")
CID=$(echo "$CBODY"|python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)
[ -n "$CID" ] && [ "$CID" != "" ]; check "comp_create" "true" "$([ $? -eq 0 ] && echo true || echo false)"

if [ -n "$CID" ] && [ "$CID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/companies/$CID" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "{\"name\":\"UpdatedComp_$S\"}")
    check "comp_update" "200" "$(echo "$R"|tail -1)"
fi

R=$(curl -s -w "\n%{http_code}" "$BASE/companies/with-users" -H "Authorization: Bearer $T")
check "comp_with_users" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/companies/with-users-extended" -H "Authorization: Bearer $T")
check "comp_with_users_ext" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/companies/2/users" -H "Authorization: Bearer $T")
check "comp_get_users" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/companies/2/tables" -H "Authorization: Bearer $T")
check "comp_get_tables" "200" "$(echo "$R"|tail -1)"

R=$(curl -s -w "\n%{http_code}" "$BASE/companies/2/unload-places" -H "Authorization: Bearer $T")
check "comp_get_unload_places" "200" "$(echo "$R"|tail -1)"

if [ -n "$CID" ] && [ "$CID" != "" ]; then
    R=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/companies/$CID" -H "Authorization: Bearer $T")
    check "comp_delete" "200" "$(echo "$R"|tail -1)"
fi

# Cleanup
curl -s -X DELETE "$BASE/users/oc_$S" -H "Authorization: Bearer $T" >/dev/null

echo ""
echo "=== Orgs+Companies: PASSED=$P FAILED=$F TOTAL=$((P+F)) ==="
