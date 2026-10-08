import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiRequest } from '../client';
import { previewApplicationPeriods, changeApplicationPeriods } from '../applicationPeriodCommands';

vi.mock('../client', () => ({ apiRequest: vi.fn() }));
const period = { entry_date_from: '2099-10-01', entry_date_to: '2099-10-03', entry_time_from: '09:00:00', entry_time_to: '18:00:00' };
const request = { attachment_ids: [2, 1], period, individual_policy: 'preserve', reason: 'Исправление срока' };
function result() {
  return { application_id: 42, attachment_ids: [1, 2], attachments: [{ attachment_id: 1, old_period: period }, { attachment_id: 2, old_period: period }],
    attachment_count: 2, employee_count: 1, car_count: 1, individual_count: 1, individual_policy: 'preserve',
    new_period: period, period_revision: 'a'.repeat(64), approvals_reset: false };
}
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data });

describe('application period commands #2665', () => {
  beforeEach(() => apiRequest.mockReset());
  it('posts a preview and puts a change using the already unwrapped data', async () => {
    apiRequest.mockResolvedValue(response(result()));
    expect(await previewApplicationPeriods(42, request)).toEqual(result());
    expect(apiRequest).toHaveBeenLastCalledWith('/applications/42/attachment-period/preview', { method: 'POST', body: JSON.stringify(request) });
    const change = { ...request, expected_revision: 'a'.repeat(64) };
    expect(await changeApplicationPeriods(42, change)).toEqual(result());
    expect(apiRequest).toHaveBeenLastCalledWith('/applications/42/attachment-period', { method: 'PUT', body: JSON.stringify(change) });
  });
  it('preserves HTTP 409 without retrying or writing another request', async () => {
    apiRequest.mockResolvedValue(response({ message: 'Срок изменился' }, 409));
    await expect(changeApplicationPeriods(42, request)).rejects.toMatchObject({ status: 409, message: 'Срок изменился' });
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it.each([
    data => ({ ...data, period_revision: 'invalid' }),
    data => ({ ...data, attachment_ids: [1, 3] }),
    data => ({ ...data, attachments: [data.attachments[0], data.attachments[0]] }),
    data => ({ ...data, individual_policy: 'replace' }),
    data => ({ ...data, employee_count: -1 }),
    data => ({ ...data, approvals_reset: true }),
    data => ({ ...data, application_id: 99 }),
  ])('rejects invalid preview identity or confirmation', async mutate => {
    apiRequest.mockResolvedValue(response(mutate(result())));
    await expect(previewApplicationPeriods(42, request)).rejects.toThrow('подтверждение');
  });
  it('does not expose server errors or call an invalid application ID', async () => {
    await expect(previewApplicationPeriods('42', request)).rejects.toThrow('Некорректная заявка');
    expect(apiRequest).not.toHaveBeenCalled();
    apiRequest.mockResolvedValue(response({ message: 'private SQL trace' }, 500));
    await expect(previewApplicationPeriods(42, request)).rejects.toThrow('Не удалось изменить срок вложений');
  });
});
