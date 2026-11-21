import { describe, it, expect, vi, beforeEach } from 'vitest';
import { loadApplicationChanges, acknowledgeApplicationChanges } from '../applicationChanges';
import { apiRequest } from '@/api/client';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

describe('application changes', () => {
  beforeEach(() => vi.clearAllMocks());
  it('loads unacknowledged changes for this application, including read bell entries', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => [
      { id: 4, is_read: true, data: JSON.stringify({ application_id: 7, changes: [{ index: 0, acknowledged: false }, { index: 1, acknowledged: true }] }) },
      { id: 5, data: JSON.stringify({ application_id: 8, changes: [{ index: 0 }] }) },
      { id: 6, data: null },
    ] });
    expect(await loadApplicationChanges(7)).toEqual([{ index: 0, acknowledged: false, notificationId: 4, key: '4:0' }]);
  });
  it('acknowledges only displayed and older events of this field, not a newer event or another row', async () => {
    apiRequest.mockResolvedValue({ ok: true });
    const event = { entity_id: 3, entity_kind: 'cars', field: 'places', index: 0 };
    const selected = { ...event, notificationId: 11 };
    const events = [selected, { ...event, notificationId: 10 }, { ...event, notificationId: 12 }, { ...event, notificationId: 11, field: 'tables' }, { ...event, notificationId: 11, entity_id: 9 }];
    await acknowledgeApplicationChanges(events, selected);
    expect(apiRequest.mock.calls.map(call => call[0])).toEqual(['/notifications/11/read', '/notifications/10/read']);
    expect(JSON.parse(apiRequest.mock.calls[0][1].body)).toEqual({ acknowledged_change: 0 });
  });
  it('does not silently dismiss an acknowledgement failure', async () => {
    apiRequest.mockResolvedValue({ ok: false });
    const change = { entity_id: 3, entity_kind: 'cars', field: 'places', index: 0, notificationId: 11 };
    await expect(acknowledgeApplicationChanges([change], change)).rejects.toThrow('Не удалось подтвердить изменение');
  });
});
