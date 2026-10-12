import { describe, expect, it } from 'vitest';
import { canQuickRevertMark, canRevertMark, QUICK_REVERT_WINDOW_MS } from '../passageMarks';
import { historicalPeriodExpired, passageDeadlines } from '../passageProjection';
const stamp = '2030-01-01T12:00:00Z';
const start = Date.parse(stamp);
const row = (kind = 'entry') => ({ can_revert: true, last_mark_table_id: 7, server_now: stamp,
  passage_state: { last_event_kind: kind, last_event_id: 9, entry_at: stamp, exit_recorded_at: stamp } });
describe('quick cancellation uses server time without changing its grant', () => {
  it.each(['entry', 'exit'])('%s disappears at fifteen minutes, while administrative cancellation remains allowed', kind => {
    const item = row(kind);
    expect(canQuickRevertMark(item, 7, start + QUICK_REVERT_WINDOW_MS - 1)).toBe(true);
    expect(canQuickRevertMark(item, 7, start + QUICK_REVERT_WINDOW_MS)).toBe(false);
    expect(canQuickRevertMark(item, 7, start + QUICK_REVERT_WINDOW_MS + 1)).toBe(false);
    expect(canRevertMark(item, 7)).toBe(true);
    expect(passageDeadlines([item])).toContain(start + QUICK_REVERT_WINDOW_MS);
  });
  it('does not infer a timestamp or grant from the local clock', () => {
    expect(canQuickRevertMark({ can_revert: true }, 7, start)).toBe(false);
    expect(canQuickRevertMark(row(), 8, start)).toBe(false);
    expect(canQuickRevertMark({ ...row(), can_revert: false }, 7, start)).toBe(false);
    expect(canQuickRevertMark(row(), 7, start - 1)).toBe(false);
    expect(canQuickRevertMark({ ...row(), server_now: null }, 7, null)).toBe(false);
    expect(canQuickRevertMark(row('administrative_exit'), 7, start)).toBe(false);
  });
});

describe('historical period uses the received server instant', () => {
  it('expires at the Moscow day boundary without a time', () => {
    const effective_period = { bounded: true, entry_date_to: '2030-01-01' };
    expect(historicalPeriodExpired({ effective_period, server_now: '2030-01-01T20:59:59Z' })).toBe(false);
    expect(historicalPeriodExpired({ effective_period, server_now: '2030-01-01T21:00:00Z' })).toBe(true);
    expect(historicalPeriodExpired({ effective_period })).toBe(false);
  });
});
