import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useUiStore } from '@/stores/ui';
import { useDeletionsStore } from '@/stores/deletions';
import { apiRequest } from '@/api/client';
import { removeFactPostRecord } from '../removeFactPostRecord';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

describe('removal from a factual post list', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); });

  it.each(['cars', 'employees'])('removes %s only from the current post and reloads server data', async kind => {
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ success_count: 1, error_count: 0 }) });
    const onRefresh = vi.fn();
    await removeFactPostRecord({ kind, id: 10, tableId: 4, onRefresh });
    expect(apiRequest).toHaveBeenCalledWith(`/${kind}/bulk/unbind-table`, { method: 'POST', body: '{"ids":[10],"table_id":4}' });
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it('keeps the row when the server rejects the operation', async () => {
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    apiRequest.mockResolvedValue({ ok: false, json: async () => ({ error: 'Forbidden' }) });
    const onRefresh = vi.fn();
    await removeFactPostRecord({ kind: 'employees', id: 10, tableId: 4, onRefresh });
    expect(onRefresh).not.toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('does not send a request after cancellation', async () => {
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);
    await removeFactPostRecord({ kind: 'cars', id: 10, tableId: 4, onRefresh: vi.fn() });
    expect(apiRequest).not.toHaveBeenCalled();
  });
});
