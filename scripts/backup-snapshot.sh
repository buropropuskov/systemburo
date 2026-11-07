# Shared by backup.sh and backup-verify.sh. Source from Bash.
BACKUP_COUNTS_SQL='SELECT (SELECT count(*) FROM public.users), (SELECT count(*) FROM public.applications), (SELECT count(*) FROM public.system_tables);'
BACKUP_COUNTS_PATTERN='^[0-9]+\|[0-9]+\|[0-9]+$'

backup_dump_snapshot() (
  local raw="$1" input output pid snapshot counts digest
  coproc BACKUP_SNAPSHOT {
    "${COMPOSE[@]}" exec -T db psql -X -qAt -F '|' -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME"
  }
  pid=$BACKUP_SNAPSHOT_PID
  exec {input}>&"${BACKUP_SNAPSHOT[1]}" {output}<&"${BACKUP_SNAPSHOT[0]}"
  trap 'exec {input}>&- {output}<&-; wait "$pid" || true' EXIT
  printf '%s\n' 'BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;' 'SELECT pg_export_snapshot();' "$BACKUP_COUNTS_SQL" >&"$input"
  IFS= read -r snapshot <&"$output"
  IFS= read -r counts <&"$output"
  [[ "$snapshot" =~ ^[[:xdigit:]]+-[[:xdigit:]]+-[[:digit:]]+$ ]]
  [[ "$counts" =~ $BACKUP_COUNTS_PATTERN ]]
  "${COMPOSE[@]}" exec -T db pg_dump -U "$DB_USER" -Fc --snapshot="$snapshot" --dbname="$DB_NAME" > "$raw"
  printf '%s\n' 'ROLLBACK;' '\q' >&"$input"
  wait "$pid"
  digest="$(sha256sum < "$raw")"
  printf '%s\n' v1 "${digest%% *}" "$counts"
)

backup_manifest_name() {
  printf 'verify-%s.counts\n' "${1##*/}"
}
