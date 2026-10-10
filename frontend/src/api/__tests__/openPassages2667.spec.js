import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiRequest } from '../client';
import { listOpenPassages, closeOpenPassage, revertPassageCorrection } from '../openPassages';

vi.mock('../client', () => ({ apiRequest: vi.fn() }));
const stamp = '2035-01-01T12:00:00Z';
const row = { entity_id: 17, entity_kind: 'cars', server_now: stamp, effective_period: { bounded: true },
  admission: { can_enter: false, can_exit: true }, passage_state: { has_event: true, last_event_id: 81, open: true,
    entry_at: '2034-12-30T10:00:00Z', exit_recorded_at: null, grace_until: null,
    in_exit_grace: false, needs_attention: true, entry_time_known: true, can_correct: true, can_revert_correction: false } };
const data = { items: [row], counts: { all_open: 5, attention: 2, unknown_time: 1 }, page: 1, per_page: 25, total: 2, server_now: stamp };
const response = (body, status = 200) => ({ ok: status < 400, status, json: async () => body });
describe('scoped open passage API #2667', () => {
  beforeEach(() => apiRequest.mockReset());
  it('requests expired rows before server pagination and reads scoped counts', async () => {
    apiRequest.mockResolvedValue(response({ ...data, counts: { ...data.counts, expired: 2 } }));
    const result = await listOpenPassages('car', 4, { expiredOnly: true, attentionOnly: false });
    expect(apiRequest).toHaveBeenCalledWith('/cars/open-for-table/4?view=open&page=1&per_page=25&attention_only=false&expired_only=true');
    expect(result.counts.expired).toBe(2);
    apiRequest.mockResolvedValue(response({ ...data, counts: { ...data.counts, expired: -1 } }));
    await expect(listOpenPassages('car', 4)).rejects.toThrow('список');
  });
  it('uses only the scoped table path and canonical query, normalizing plural wire kind', async () => {
    apiRequest.mockResolvedValue(response(data));
    expect((await listOpenPassages('car', 4, { search: '  TEST  ', organizationID: 7 })).items[0].entity_kind).toBe('car');
    expect(apiRequest).toHaveBeenCalledWith('/cars/open-for-table/4?view=open&page=1&per_page=25&attention_only=true&search=TEST&organization_id=7');
  });
  it('loads recent corrections with separate server pagination and does not apply attention filtering', async () => {
    apiRequest.mockResolvedValue(response({ ...data, items: [] }));
    await listOpenPassages('employee', 4, { view: 'corrections' });
    expect(apiRequest).toHaveBeenCalledWith('/employees/open-for-table/4?view=corrections&page=1&per_page=25');
  });
  it.each([null, 0, -1])('does not broaden an invalid table scope %j', async table => {
    await expect(listOpenPassages('car', table)).rejects.toThrow('таблица');
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('rejects malformed admission instead of inferring a grant from legacy status', async () => {
    apiRequest.mockResolvedValue(response({ ...data, items: [{ ...row, territory_status: 1, admission: null }] }));
    await expect(listOpenPassages('car', 4)).rejects.toThrow('состояние');
  });
  it('sends a single correction with explicit unknown time and expected last event; revert has no actual time', async () => {
    apiRequest.mockResolvedValue(response(row));
    await closeOpenPassage('car', 17, 4, { reason: '  Synthetic reason ', expected_last_event_id: 81 });
    await revertPassageCorrection('car', 17, 4, { reason: 'Undo', expected_last_event_id: 82, actual_exit_at: stamp });
    expect(apiRequest).toHaveBeenNthCalledWith(1, '/cars/17/passage-close', { method: 'POST', body: JSON.stringify({ source: 'table', table_id: 4, expected_last_event_id: 81, reason: 'Synthetic reason', actual_exit_at: null }) });
    expect(apiRequest).toHaveBeenNthCalledWith(2, '/cars/17/passage-close/revert', { method: 'POST', body: JSON.stringify({ source: 'table', table_id: 4, expected_last_event_id: 82, reason: 'Undo' }) });
  });
  it('preserves stale/denied errors without automatic retries or wider paths', async () => {
    apiRequest.mockResolvedValue(response({ message: 'Changed' }, 409));
    await expect(closeOpenPassage('car', 17, 4, { reason: 'Reason', expected_last_event_id: 81 })).rejects.toMatchObject({ status: 409 });
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it('reads explicit administrative scope without a table ID and never infers it', async () => {
    apiRequest.mockResolvedValue(response(data));
    await listOpenPassages('car', null, { source: 'admin_summary' });
    expect(apiRequest).toHaveBeenCalledWith('/cars/open-admin-summary?view=open&page=1&per_page=25&attention_only=true');
    apiRequest.mockClear();
    await expect(listOpenPassages('car', 4, { source: 'admin_summary' })).rejects.toThrow('таблица');
    await expect(listOpenPassages('car', null)).rejects.toThrow('таблица');
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('sends source admin_summary explicitly, with no fabricated table context', async () => {
    apiRequest.mockResolvedValue(response(row));
    await closeOpenPassage('car', 17, null, { source: 'admin_summary', expected_last_event_id: 81, reason: 'Synthetic correction', actual_exit_at: null });
    const body = JSON.parse(apiRequest.mock.calls[0][1].body);
    expect(body).toEqual({ source: 'admin_summary', expected_last_event_id: 81, reason: 'Synthetic correction', actual_exit_at: null });
    expect(body).not.toHaveProperty('table_id');
  });
});
