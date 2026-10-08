package services

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"systemburo/internal/models"
	"systemburo/internal/passages"
	"time"
)

const PassageCorrectionAction = "passage_close"
const PassageCorrectionRevertAction = "passage_close_revert"

type PassageState struct {
	HasEvent            bool       `json:"has_event"`
	LastEventID         int64      `json:"last_event_id"`
	LastEventKind       string     `json:"last_event_kind"`
	Open                bool       `json:"open"`
	EntryAt             *time.Time `json:"entry_at"`
	EntryTableID        *int       `json:"entry_table_id"`
	EntryTimeKnown      bool       `json:"entry_time_known"`
	ExitRecordedAt      *time.Time `json:"exit_recorded_at"`
	GraceUntil          *time.Time `json:"grace_until"`
	InExitGrace         bool       `json:"in_exit_grace"`
	NeedsAttention      bool       `json:"needs_attention"`
	CanCorrect          bool       `json:"can_correct"`
	CanRevertCorrection bool       `json:"can_revert_correction"`
}

type PassageAdmission struct {
	CanEnter bool   `json:"can_enter"`
	CanExit  bool   `json:"can_exit"`
	Reason   string `json:"reason"`
}

type PassageResult struct {
	EntityID        int                    `json:"entity_id"`
	EntityKind      ElementKind            `json:"entity_kind"`
	PassageState    PassageState           `json:"passage_state"`
	EffectivePeriod models.EffectivePeriod `json:"effective_period"`
	Admission       PassageAdmission       `json:"admission"`
	ServerNow       time.Time              `json:"server_now"`
}

// PassageLivePredicate shares the same reversal predicate as history readers.
func PassageLivePredicate(alias string) string {
	return passageRevertNotExists(alias)
}

func passageEntityType(kind ElementKind) (string, error) {
	switch kind {
	case ElementCar:
		return models.AuditEntityCar, nil
	case ElementEmployee:
		return models.AuditEntityEmployee, nil
	}
	return "", fmt.Errorf("unsupported passage entity kind")
}

// PassageProjectionSQL is a bounded two-event lateral projection: no N+1 or
// complete per-row history load. caller supplies only static validated aliases.
func PassageProjectionSQL(kind ElementKind, entityAlias, outputAlias string) (string, error) {
	entityType, err := passageEntityType(kind)
	if err != nil {
		return "", err
	}
	for _, alias := range []string{entityAlias, outputAlias} {
		if !safePassageAlias(alias) {
			return "", fmt.Errorf("invalid passage SQL alias")
		}
	}
	return fmt.Sprintf(`LEFT JOIN LATERAL (
	 SELECT latest.id,latest.action,latest.created_at,latest.actor_user_id,
	 (latest.details->>'table_id')::int AS table_id,
	 COALESCE(latest.details->>'closes_unknown_entry','false')='true' AS closes_unknown_entry,
	 previous.id AS previous_id,previous.action AS previous_action,previous.created_at AS previous_at
	 FROM (SELECT p.id,p.action,p.created_at,p.actor_user_id,p.details FROM audit_log p
	 WHERE p.entity_type='%s' AND p.entity_id=%s.id AND p.action IN ('entry','exit','%s') AND %s
	 ORDER BY p.created_at DESC,p.id DESC LIMIT 1) latest
	 LEFT JOIN LATERAL (SELECT p.id,p.action,p.created_at FROM audit_log p
	 WHERE p.entity_type='%s' AND p.entity_id=%s.id AND p.action IN ('entry','exit','%s') AND %s
	 AND (p.created_at,p.id)<(latest.created_at,latest.id)
	 ORDER BY p.created_at DESC,p.id DESC LIMIT 1) previous ON TRUE
	) %s ON TRUE`, entityType, entityAlias, PassageCorrectionAction, PassageLivePredicate("p"), entityType, entityAlias, PassageCorrectionAction, PassageLivePredicate("p"), outputAlias), nil
}

func safePassageAlias(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

type passageProjectionRow struct {
	EntityID           int
	TerritoryStatus    *int
	ID                 int64
	Action             string
	CreatedAt          *time.Time
	ActorUserID        *int
	TableID            *int
	ClosesUnknownEntry bool
	PreviousID         int64
	PreviousAction     string
	PreviousAt         *time.Time
}

func projectPassageRow(row passageProjectionRow, now time.Time) (PassageState, error) {
	toKind := func(action string) passages.Kind {
		if action == PassageCorrectionAction {
			return passages.Correction
		}
		return passages.Kind(action)
	}
	events := []passages.Event{}
	if row.PreviousAt != nil {
		events = append(events, passages.Event{ID: row.PreviousID, Kind: toKind(row.PreviousAction), RecordedAt: *row.PreviousAt})
	}
	if row.CreatedAt != nil {
		events = append(events, passages.Event{ID: row.ID, Kind: toKind(row.Action), RecordedAt: *row.CreatedAt})
	}
	s, err := passages.Resolve(events, now)
	if err != nil {
		return PassageState{}, err
	}
	out := PassageState{HasEvent: s.HasEvent, LastEventID: s.LastEventID, LastEventKind: row.Action, Open: s.Open, EntryAt: s.EntryAt, EntryTimeKnown: s.EntryAt != nil, ExitRecordedAt: s.ExitRecordedAt, GraceUntil: s.GraceUntil, InExitGrace: s.InExitGrace, NeedsAttention: s.NeedsAttention}
	if s.Open {
		out.EntryTableID = row.TableID
	}
	if row.Action == "exit" && row.ClosesUnknownEntry && row.CreatedAt != nil {
		until := row.CreatedAt.Add(passages.ExitGrace)
		out.GraceUntil = &until
		out.InExitGrace = now.Before(until)
	}
	// A legacy cached entry without a live clock is explicitly unknown; do not
	// infer a duration from updated_at, reset time, or territory_entry_time.
	if !s.HasEvent && row.TerritoryStatus != nil && *row.TerritoryStatus == 1 {
		out.Open = true
	}
	return out, nil
}

func LoadPassageStates(ctx context.Context, db *gorm.DB, kind ElementKind, ids []int, now time.Time) (map[int]PassageState, error) {
	out := map[int]PassageState{}
	if len(ids) == 0 {
		return out, nil
	}
	join, err := PassageProjectionSQL(kind, "e", "p")
	if err != nil {
		return nil, err
	}
	var rows []passageProjectionRow
	err = db.WithContext(ctx).Raw("SELECT e.id AS entity_id,e.territory_status,p.* FROM "+string(kind)+" e "+join+" WHERE e.id IN ?", ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		s, err := projectPassageRow(row, now)
		if err != nil {
			return nil, err
		}
		out[row.EntityID] = s
	}
	return out, nil
}

// passageWindow admits one continuous Moscow window, with a half-open end.
// Missing legacy clocks mean midnight/start and next midnight/end, respectively.
func passageWindow(p models.EffectivePeriod, now time.Time) (bool, string) {
	if p.Source == "manual_unbounded" {
		return true, ""
	}
	value := func(s *string) string {
		if s == nil {
			return ""
		}
		return strings.TrimSpace(*s)
	}
	parse := func(date, clock string) (time.Time, error) {
		if clock == "" {
			clock = "00:00"
		}
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if at, err := time.ParseInLocation(layout, date+" "+clock, MoscowLocation()); err == nil {
				return at, nil
			}
		}
		return time.Time{}, fmt.Errorf("invalid passage period")
	}
	start, err := parse(value(p.EntryDateFrom), value(p.EntryTimeFrom))
	if err != nil {
		return false, "invalid_period"
	}
	end, err := parse(value(p.EntryDateTo), value(p.EntryTimeTo))
	if err != nil {
		return false, "invalid_period"
	}
	if value(p.EntryTimeTo) == "" {
		end = end.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		return false, "invalid_period"
	}
	if now.Before(start) {
		return false, "not_started"
	}
	if !now.Before(end) {
		return false, "expired"
	}
	return true, ""
}

// PassageRetentionSQL is only a presence/grace predicate. The caller must retain
// its table binding, visibility, deletion and lifecycle guards around this OR.
// now must be the same server clock used to build the response projection.
func PassageRetentionSQL(entityAlias, projectionAlias string, now time.Time) (string, []any, error) {
	if !safePassageAlias(entityAlias) || !safePassageAlias(projectionAlias) {
		return "", nil, fmt.Errorf("invalid passage SQL alias")
	}
	sql := fmt.Sprintf("(%[2]s.action='entry' OR (%[2]s.id IS NULL AND %[1]s.territory_status=1) OR ((%[2]s.action='%[3]s' OR (%[2]s.action='exit' AND (%[2]s.previous_action='entry' OR %[2]s.closes_unknown_entry))) AND %[2]s.created_at>?))", entityAlias, projectionAlias, PassageCorrectionAction)
	return sql, []any{now.UTC().Add(-passages.ExitGrace)}, nil
}
