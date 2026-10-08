import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiRequest } from '../client';
import { getManualAttachContext, getManualAttachAttachments, previewManualAttachSingle, executeManualAttachSingle } from '../manualAttachSingle';

vi.mock('../client', () => ({ apiRequest: vi.fn() }));
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data });
const flags = { roof_access: true, free_parking: false, individual_roof_access: false, individual_free_parking: false };
const context = { entity_id: 17, entity_kind: 'employees', attachment_id: 8, is_manual: true, application_id: null,
  period_mode: 'inherit', effective_period: { bounded: false }, requires_period_choice: true, can_assign_period: true };

describe('single manual attach API #2665', () => {
  beforeEach(() => apiRequest.mockReset());
  it('reads the actual entity and table context using the unwrapped API response', async () => {
    apiRequest.mockResolvedValue(response(context));
    expect(await getManualAttachContext('employee', 17, 4)).toEqual({ ...context, entity_kind: 'employee' });
    expect(apiRequest).toHaveBeenCalledWith('/employees/17/manual-attach-context?table_id=4');
  });
  it.each([['employee', 'employees'], ['car', 'cars']])('loads narrow attachment metadata for %s without using general application GET', async (kind, plural) => {
    const attachments = [{ id: 31, application_id: 7, status: 1, is_manual: false, attachment_type: 'people',
      attachment_name: 'Synthetic', attachment_display_name: 'Synthetic', entry_date_from: null, entry_date_to: null,
      entry_time_from: null, entry_time_to: null }];
    apiRequest.mockResolvedValue(response(attachments));
    expect(await getManualAttachAttachments(kind, 17, 4, 7)).toEqual(attachments);
    expect(apiRequest).toHaveBeenCalledWith(`/${plural}/17/manual-attach-attachments?application_id=7&table_id=4`);
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it.each([{ application_id: 8 }, { status: 0 }, { is_manual: true }, { id: 0 }, { attachment_type: 'other' }])('rejects malformed narrow metadata %j', async patch => {
    apiRequest.mockResolvedValue(response([{ id: 31, application_id: 7, status: 1, is_manual: false, attachment_type: 'cars', ...patch }]));
    await expect(getManualAttachAttachments('car', 17, 4, 7)).rejects.toThrow('вложения для привязки');
  });
  it('preserves a metadata denial without attempting a wider fallback', async () => {
    apiRequest.mockResolvedValue(response({ message: 'Denied' }, 403));
    await expect(getManualAttachAttachments('car', 17, 4, 7)).rejects.toMatchObject({ status: 403 });
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it.each([
    { entity_id: 18 }, { entity_kind: 'cars' }, { application_id: 9 }, { is_manual: false },
    { attachment_id: 0 }, { can_assign_period: undefined }, { requires_period_choice: false },
  ])('fails closed for mismatched context %j', async patch => {
    apiRequest.mockResolvedValue(response({ ...context, ...patch }));
    await expect(getManualAttachContext('employee', 17, 4)).rejects.toThrow('ручное основание');
  });
  it.each([['employee', 0, 4], ['car', 17, null], ['other', 17, 4], ['car', 17, -1]])('rejects invalid identity %s/%s/%s before network', async (kind, id, table) => {
    await expect(getManualAttachContext(kind, id, table)).rejects.toThrow('Некорректная');
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('previews and executes a single fixed car path, with authoritative table and revision', async () => {
    apiRequest.mockResolvedValue(response({ entity_id: 17, entity_kind: 'cars', current_flags: flags, new_flags: flags }));
    expect(await previewManualAttachSingle('car', 17, 4, { application_id: 7, reason: 'Reason', table_id: 99 })).toMatchObject({ entity_kind: 'car' });
    expect(await executeManualAttachSingle('car', 17, 4, { target_attachment_id: 8, expected_revision: 'a'.repeat(64), reason: 'Reason' })).toMatchObject({ entity_kind: 'car' });
    expect(apiRequest).toHaveBeenNthCalledWith(1, '/cars/17/attach-to-application/preview', {
      method: 'POST', body: JSON.stringify({ application_id: 7, reason: 'Reason', table_id: 4 }),
    });
    expect(apiRequest).toHaveBeenNthCalledWith(2, '/cars/17/attach-to-application', {
      method: 'POST', body: JSON.stringify({ target_attachment_id: 8, expected_revision: 'a'.repeat(64), reason: 'Reason', table_id: 4 }),
    });
  });
  it('rejects malformed command identity before emitting a parent event', async () => {
    apiRequest.mockResolvedValue(response({ entity_id: 17, entity_kind: 'car', current_flags: flags, new_flags: flags }));
    await expect(executeManualAttachSingle('car', 17, 4, {})).rejects.toMatchObject({ status: 409 });
  });
  it('keeps authoritative current and future car flags without rewriting effective values', async () => {
    apiRequest.mockResolvedValueOnce(response({ ...context, entity_kind: 'cars', flags }));
    expect(await getManualAttachContext('car', 17, 4)).toMatchObject({ entity_kind: 'car', flags });
    const next = { ...flags, individual_roof_access: true, free_parking: true };
    apiRequest.mockResolvedValue(response({ entity_id: 17, entity_kind: 'cars', current_flags: flags, new_flags: next }));
    expect(await previewManualAttachSingle('car', 17, 4, {})).toMatchObject({ current_flags: flags, new_flags: next });
  });
  it.each([undefined, { ...flags, roof_access: 'true' }, { ...flags, individual_roof_access: undefined },
    { ...flags, individual_free_parking: true }])('fails closed on malformed car flags %j', async badFlags => {
    apiRequest.mockResolvedValue(response({ entity_id: 17, entity_kind: 'cars', current_flags: flags, new_flags: badFlags }));
    await expect(executeManualAttachSingle('car', 17, 4, {})).rejects.toMatchObject({ status: 409 });
    apiRequest.mockResolvedValue(response({ ...context, entity_kind: 'cars', flags: badFlags }));
    await expect(getManualAttachContext('car', 17, 4)).rejects.toThrow('ручное основание');
  });
  it('does not demand car flags on an employee command', async () => {
    apiRequest.mockResolvedValue(response({ entity_id: 17, entity_kind: 'employees' }));
    expect(await previewManualAttachSingle('employee', 17, 4, {})).toEqual({ entity_id: 17, entity_kind: 'employee' });
  });
  it('preserves status for explicit stale handling and hides server internals on 500', async () => {
    apiRequest.mockResolvedValueOnce(response({ message: 'Changed' }, 409));
    await expect(previewManualAttachSingle('car', 17, 4, {})).rejects.toMatchObject({ status: 409, message: 'Changed' });
    apiRequest.mockResolvedValueOnce(response({ message: 'secret SQL' }, 500));
    await expect(getManualAttachContext('car', 17, 4)).rejects.toMatchObject({ status: 500, message: 'Не удалось проверить возможность привязки' });
  });
});
