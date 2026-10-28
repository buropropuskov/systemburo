import { beforeEach, describe, expect, it, vi } from 'vitest';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), apiRequestRaw: vi.fn() }));
import { apiRequestRaw } from '@/api/client';
import { getAccessibleAttachments } from '../applications';

beforeEach(() => vi.clearAllMocks());
describe('accessible data readiness preserves HTTP outcomes', () => {
  it.each([403, 500])('retains HTTP %s without parsing the translated message', async status => {
    apiRequestRaw.mockResolvedValue({ ok: false, status, json: async () => ({ success: false, error: 'Недоступно' }) });
    await expect(getAccessibleAttachments()).rejects.toMatchObject({ status, message: 'Недоступно' });
  });
  it('preserves an empty successful list as data rather than an error', async () => {
    apiRequestRaw.mockResolvedValue({ ok: true, status: 200, json: async () => ({ success: true, data: [], meta: { total: 0 } }) });
    await expect(getAccessibleAttachments()).resolves.toMatchObject({ items: [], meta: { total: 0 } });
  });
});
