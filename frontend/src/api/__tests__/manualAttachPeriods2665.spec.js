import { beforeEach, describe, expect, it, vi } from 'vitest';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
import { apiRequest } from '@/api/client';
import { attachToApplication } from '../attachments';

describe('manual attach period contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ application_id: 7, attachment_id: 8 }) });
  });
  it.each([
    [{ applicationId: 7 }, { application_id: 7 }],
    [{ targetAttachmentId: 8 }, { target_attachment_id: 8 }],
  ])('preserves finite legacy body %j', async (target, body) => {
    await attachToApplication(9, target);
    expect(apiRequest).toHaveBeenCalledWith('/attachments/9/attach-to-application', { method: 'POST', body: JSON.stringify(body) });
  });
  it('sends explicit source without individual fields', async () => {
    await attachToApplication(9, { targetAttachmentId: 8, periodChoice: 'source', sourceAttachmentId: 10, period: { unwanted: true } });
    expect(JSON.parse(apiRequest.mock.calls[0][1].body)).toEqual({ target_attachment_id: 8, period_choice: 'source', source_attachment_id: 10 });
  });
  it('sends complete explicit individual without source ID', async () => {
    const period = { entry_date_from: '2030-10-08', entry_date_to: '2030-10-09', entry_time_from: '08:00:00', entry_time_to: '20:00:00' };
    await attachToApplication(9, { applicationId: 7, periodChoice: 'individual', sourceAttachmentId: 10, period });
    expect(JSON.parse(apiRequest.mock.calls[0][1].body)).toEqual({ application_id: 7, period_choice: 'individual', period });
  });
  it('does not silently select a period policy', async () => {
    await attachToApplication(9, { applicationId: 7, sourceAttachmentId: 10, period: {} });
    expect(JSON.parse(apiRequest.mock.calls[0][1].body)).toEqual({ application_id: 7 });
  });
  it('retains server rejection instead of returning success', async () => {
    apiRequest.mockResolvedValue({ ok: false, json: async () => ({ message: 'Выберите источник срока' }) });
    await expect(attachToApplication(9, { applicationId: 7 })).rejects.toThrow('Выберите источник срока');
  });
});
