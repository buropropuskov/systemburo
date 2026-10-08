/** Preserve server facts separately from viewer/table grants. */
export function passageFields(row, { snapshot = false } = {}) {
  const state = row?.passage_state;
  if (!state) return {};
  return {
    passage_state: { ...state, ...(snapshot ? { can_correct: false, can_revert_correction: false } : {}) },
    effective_period: row.effective_period ? { ...row.effective_period } : null,
    admission: snapshot ? { can_enter: false, can_exit: false, reason: row.admission?.reason || 'snapshot' } : row.admission ? { ...row.admission } : null,
    server_now: row.server_now ?? null,
    territory_status: state.open ? 1 : state.last_event_kind === 'exit' ? 2 : 0,
    entry_checked: state.open,
    // An administrative correction is not an observed exit.
    exit_checked: !state.open && state.last_event_kind === 'exit',
    ...(snapshot ? { can_revert: false, last_mark_table_id: null } : {}),
  };
}

export function mergePassageStatus(item, status) {
  if (item.passage_state) {
    // Table rows carry their binding/permission-specific admission. A global
    // current-status response must not replace that authoritative scoped read.
    item.can_revert = status.can_revert === true && status.passage_state?.last_event_id === item.passage_state.last_event_id
      && ['entry', 'exit'].includes(item.passage_state.last_event_kind);
    item.last_mark_table_id = status.last_mark_table_id;
    return;
  }
  Object.assign(item, { territory_status: status.territory_status,
    entry_checked: status.territory_status === 1, exit_checked: status.territory_status === 2,
    can_revert: status.can_revert === true, last_mark_table_id: status.last_mark_table_id,
    entry_time: status.entry_time, exit_time: status.last_exit_time });
}

export function passageAllowed(item, direction) {
  if (!item?.passage_state) return true; // Readable legacy payloads have no projection.
  return item.admission?.[direction === 'entry' ? 'can_enter' : 'can_exit'] === true;
}
export function passageExpired(item) { return item?.admission?.reason === 'expired'; }
export function expectedPassageEvent(item) {
  const id = item?.passage_state?.last_event_id;
  return Number.isSafeInteger(id) && id >= 0 ? id : undefined;
}
export function passageDeadlines(rows) {
  return rows.flatMap(row => {
    const state = row.passage_state;
    if (!state) return [];
    return [state.grace_until, ...periodBoundaries(row.effective_period), state.open && state.entry_time_known && state.entry_at
      ? Date.parse(state.entry_at) + 48 * 3600000 + 1 : null].filter(Boolean);
  });
}

function periodBoundaries(period) {
  if (!period?.bounded) return [];
  const instant = (date, clock, end = false) => {
    if (!date) return null;
    const value = Date.parse(date + 'T' + (clock || '00:00:00') + '+03:00');
    return Number.isFinite(value) ? value + (end && !clock ? 86400000 : 0) : null;
  };
  return [instant(period.entry_date_from, period.entry_time_from), instant(period.entry_date_to, period.entry_time_to, true)];
}
