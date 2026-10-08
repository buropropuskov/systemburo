import { describe, expect, it, vi } from 'vitest';
import { passageFields, mergePassageStatus, passageAllowed, passageExpired, expectedPassageEvent, passageDeadlines } from '../passageProjection';
import { normalizeSnapshotRows } from '../snapshotRows';
import { canRevertMark, lastMarkDirection, markPassage, revertPassage } from '../passageMarks';
import { apiRequest } from '@/api/client';
import { CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS, historyActionText, isPassageHistoryAction, tablePassageHistoryText, correctionStatus } from '../passageHistoryActions';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const stamp = '2035-01-03T12:00:00Z';
const state = { has_event: true, last_event_id: 81, last_event_kind: 'entry', open: true, entry_time_known: true,
  entry_at: '2035-01-01T12:00:00Z', needs_attention: false, in_exit_grace: false, grace_until: null,
  can_correct: true, can_revert_correction: false };
const period = { bounded: true, source: 'individual', entry_date_from: '2035-01-01', entry_date_to: '2035-01-02', entry_time_from: '09:00:00', entry_time_to: null };
const row = { id: 17, passage_state: state, admission: { can_enter: false, can_exit: true, reason: 'expired' }, effective_period: period, server_now: stamp };
describe('passage projection integration #2667', () => {
  it.each(['cars', 'people'])('keeps legacy exit in a %s snapshot without inventing an event or grace', kind => {
    const [frozen] = normalizeSnapshotRows([{ id: 1, territory_status: 2,
      passage_state: { has_event: false, open: false, last_event_kind: '', in_exit_grace: false } }], kind);
    expect(frozen.territory_status).toBe(2);
    expect(frozen.exit_checked).toBe(true);
    expect(frozen.passage_state.has_event).toBe(false);
    expect(frozen.passage_state.in_exit_grace).toBe(false);
    expect(frozen.can_revert).toBe(false);
  });
  it('retains scoped admission and current passage when global status enrichment is stale', () => {
    const item = { ...row, ...passageFields(row) };
    mergePassageStatus(item, { territory_status: 2, can_revert: true, last_mark_table_id: 4, passage_state: { ...state, last_event_id: 70 } });
    expect(item.passage_state.last_event_id).toBe(81);
    expect(item.entry_checked).toBe(true);
    expect(item.can_revert).toBe(false);
    expect(passageAllowed(item, 'entry')).toBe(false);
    expect(passageAllowed(item, 'exit')).toBe(true);
    expect(passageExpired(item)).toBe(true);
  });
  it('never substitutes correction for an observed exit or ordinary revert', () => {
    const item = { ...passageFields({ ...row, passage_state: { ...state, open: false, last_event_kind: 'passage_close', can_revert_correction: true } }), can_revert: true };
    expect(item.exit_checked).toBe(false);
    expect(item.entry_checked).toBe(false);
    expect(canRevertMark(item, 4)).toBe(false);
    expect(lastMarkDirection(item)).toBeNull();
    expect(correctionStatus(item.passage_state)).toBe('Учёт закрыт исправлением');
  });
  it.each(['cars', 'people'])('freezes snapshot facts and strips live grants for %s', kind => {
    const source = { ...row, car_number: 'TEST-17', last_name: 'Synthetic', can_revert: true };
    const [frozen] = normalizeSnapshotRows([source], kind);
    expect(frozen.passage_state).toMatchObject({ open: true, last_event_id: 81, can_correct: false, can_revert_correction: false });
    expect(frozen.effective_period).toEqual(period);
    expect(frozen.server_now).toBe(stamp);
    expect(frozen.can_revert).toBe(false);
    expect(frozen.admission).toEqual({ can_enter: false, can_exit: false, reason: 'expired' });
    expect(passageExpired(frozen)).toBe(true);
    expect(source.passage_state.can_correct).toBe(true);
  });
  it.each(['cars', 'people'])('freezes corrected %s without converting it to observed exit or entry', kind => {
    const [frozen] = normalizeSnapshotRows([{ ...row, territory_status: 2, passage_state: {
      ...state, open: false, last_event_kind: 'passage_close', can_correct: false, can_revert_correction: true,
      in_exit_grace: true, exit_recorded_at: stamp, grace_until: '2035-01-03T12:05:00Z' } }], kind);
    expect(frozen.entry_checked).toBe(false); expect(frozen.exit_checked).toBe(false);
    expect(correctionStatus(frozen.passage_state)).toBe('Учёт закрыт исправлением');
    expect(frozen.passage_state.can_revert_correction).toBe(false);
    expect(frozen.passage_state.grace_until).toBe('2035-01-03T12:05:00Z');
    expect(frozen.admission).toEqual({ can_enter: false, can_exit: false, reason: 'expired' });
    expect(passageExpired(frozen)).toBe(true);
    expect(frozen.server_now).toBe(stamp);
  });
  it('does not invent a clock, period, state or permission for old snapshots', () => {
    const [frozen] = normalizeSnapshotRows([{ id: 1, territory_status: 1 }], 'cars');
    expect(frozen.passage_state).toBeUndefined(); expect(frozen.server_now).toBeUndefined();
    expect(frozen.entry_checked).toBe(true);
  });
  it('schedules strict >48h, registered grace and Moscow half-open date-only end', () => {
    const deadlines = passageDeadlines([{ ...row, passage_state: { ...state, grace_until: '2035-01-03T12:05:00Z' } }]);
    expect(deadlines).toContain(Date.parse(state.entry_at) + 48 * 3600000 + 1);
    expect(deadlines).toContain('2035-01-03T12:05:00Z');
    expect(deadlines).toContain(Date.parse('2035-01-03T00:00:00+03:00'));
  });
  it('sends a truthful zero/nonzero expected ID for mark and ordinary revert', async () => {
    apiRequest.mockReset().mockResolvedValue({ ok: true });
    expect(expectedPassageEvent({ passage_state: { last_event_id: 0 } })).toBe(0);
    expect(expectedPassageEvent({})).toBeUndefined();
    await markPassage({ kind: 'cars', id: 17, direction: 'exit', tableId: 4, expectedLastEventID: 81 });
    await revertPassage({ kind: 'cars', id: 17, direction: 'exit', tableId: 4, reason: 'Synthetic', expectedLastEventID: 82 });
    expect(JSON.parse(apiRequest.mock.calls[0][1].body).expected_last_event_id).toBe(81);
    expect(JSON.parse(apiRequest.mock.calls[1][1].body).expected_last_event_id).toBe(82);
  });
  it.each([CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS])('labels correction history distinctly without changing ordinary vocabulary', actions => {
    expect(historyActionText({ action_type: 'passage_close' }, actions)).toContain('не отметка выхода');
    expect(historyActionText({ action_type: 'passage_close_revert' }, actions)).toBe('Исправление учёта отменено');
    expect(isPassageHistoryAction({ action_type: 'passage_close' })).toBe(true);
    expect(isPassageHistoryAction({ action_type: 'update' })).toBe(false);
    expect(tablePassageHistoryText({ action_type: 'delete' }, 'employee')).toBe('Удаление из таблицы');
  });
});
