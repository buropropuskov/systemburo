#!/usr/bin/env bash
# Verify startup and restart compatibility using disposable local resources.
set -Eeuo pipefail

if [[ $# -ne 1 || ! "$1" =~ ^[[:alnum:]][[:alnum:]._/:@-]*$ ]]; then
  echo "Usage: verify_postgres_runtime.sh IMAGE" >&2
  exit 1
fi
image="$1"
docker image inspect "$image" >/dev/null

suffix="$(od -An -N12 -tx1 /dev/urandom | tr -d ' \n')"
owner="$(od -An -N12 -tx1 /dev/urandom | tr -d ' \n')"
[[ "$suffix" =~ ^[0-9a-f]{24}$ && "$owner" =~ ^[0-9a-f]{24}$ ]]
container="buro-pg-smoke-${suffix}"
volume="${container}-data"
container_id=""
volume_owned=false

cleanup() {
  local result=$?
  trap - EXIT
  if [[ -n "$container_id" ]]; then
    docker rm --force "$container_id" >/dev/null || result=1
  fi
  if [[ "$volume_owned" == true ]]; then
    local current_owner
    current_owner="$(docker volume inspect --format '{{index .Labels "buro.runtime-smoke-owner"}}' "$volume")" || current_owner=""
    if [[ "$current_owner" == "$owner" ]]; then
      docker volume rm "$volume" >/dev/null || result=1
    else
      echo "Refusing to remove a volume whose ownership changed" >&2
      result=1
    fi
  fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

# Name collisions are failures, never permission to reuse an existing resource.
if docker container inspect "$container" >/dev/null 2>&1 || docker volume inspect "$volume" >/dev/null 2>&1; then
  echo "Disposable runtime resource name already exists" >&2
  exit 1
fi
docker volume create --label "buro.runtime-smoke-owner=$owner" "$volume" >/dev/null
[[ "$(docker volume inspect --format '{{index .Labels "buro.runtime-smoke-owner"}}' "$volume")" == "$owner" ]]
volume_owned=true
container_id="$(docker create --pull=never --name "$container" --network none \
  --label "buro.runtime-smoke-owner=$owner" \
  -e POSTGRES_USER=smoke -e POSTGRES_PASSWORD=testpass -e POSTGRES_DB=smoke \
  --mount "type=volume,source=$volume,target=/var/lib/postgresql/data" "$image")"

ready() {
  local readiness_deadline=$((SECONDS + 60))
  while (( SECONDS < readiness_deadline )); do
    # The temporary init server listens only on Unix sockets; require final TCP readiness.
    if timeout 3s docker exec "$container_id" pg_isready -h 127.0.0.1 -p 5432 -U smoke -d smoke -t 1 >/dev/null 2>&1; then
      return 0
    fi
    if [[ "$(timeout 3s docker inspect --format '{{.State.Running}}' "$container_id")" != true ]]; then
      echo "PostgreSQL stopped before becoming ready" >&2
      return 1
    fi
    sleep 1
  done
  echo "PostgreSQL TCP readiness timed out" >&2
  return 1
}

sql() {
  timeout 10s docker exec -e PGPASSWORD=testpass "$container_id" \
    psql -h 127.0.0.1 -p 5432 -U smoke -d smoke -At -v ON_ERROR_STOP=1 -c "$1"
}

check_process() {
  [[ "$(docker exec "$container_id" awk '$1 == "Uid:" {print $2}' /proc/1/status)" == 70 ]]
  [[ "$(docker exec "$container_id" cat /proc/1/comm)" == postgres ]]
}

stop_cleanly() {
  timeout 15s docker stop --time 10 "$container_id" >/dev/null
  [[ "$(docker inspect --format '{{.State.ExitCode}}' "$container_id")" == 0 ]]
}

docker start "$container_id" >/dev/null
ready
check_process
version="$(sql 'SHOW server_version;')"
[[ "$version" == 16.* ]]
echo "PostgreSQL runtime: $version; PID 1 UID 70"
sql 'CREATE TABLE runtime_smoke_marker (value integer NOT NULL); INSERT INTO runtime_smoke_marker VALUES (42);' >/dev/null
[[ "$(sql 'SELECT value FROM runtime_smoke_marker;')" == 42 ]]
stop_cleanly
docker start "$container_id" >/dev/null
ready
check_process
[[ "$(sql 'SELECT value FROM runtime_smoke_marker;')" == 42 ]]
stop_cleanly
echo "PostgreSQL startup, restart and persistence checks passed"
