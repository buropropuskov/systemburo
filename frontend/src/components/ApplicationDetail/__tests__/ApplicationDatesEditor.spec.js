import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import { usePermissionsStore } from '@/stores/permissions';
import { useDeletionsStore } from '@/stores/deletions';
import ApplicationDatesEditor from '../ApplicationDatesEditor.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const PERIOD = { entry_date_from: '2099-10-01', entry_date_to: '2099-10-03', entry_time_from: '09:00:00', entry_time_to: '18:00:00' };
const ATTACHMENTS = [{ id: 1, attachment_name: 'Вложение 1', ...PERIOD }, { id: 2, attachment_name: 'Вложение 2', ...PERIOD }];
const REVISION = 'a'.repeat(64);
const wrappers = [];
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data });
function result(ids = [1, 2], policy = 'preserve', revision = REVISION) {
  return { application_id: 42, attachment_ids: ids, attachments: ids.map(id => ({ attachment_id: id, old_period: PERIOD,
    employee_ids: [17], car_ids: [18], individual_employee_ids: [17], individual_car_ids: [] })),
    attachment_count: ids.length, employee_count: ids.length, car_count: ids.length, individual_count: ids.length,
    individual_policy: policy, new_period: PERIOD, period_revision: revision, approvals_reset: false,
    changed_attachment_ids: ids, changed_employee_ids: [], changed_car_ids: [] };
}
function mountEditor(props = {}) {
  const wrapper = mount(ApplicationDatesEditor, { props: {
    application: { id: 42, status: 'В обработке', confirmation: 'Согласование' }, attachments: ATTACHMENTS, isApprover: false, ...props,
  }, global: { stubs: { Teleport: true } } });
  wrappers.push(wrapper);
  return wrapper;
}
async function openWithReason(wrapper) {
  await wrapper.find('[data-testid="app-detail-change-dates"]').trigger('click');
  await wrapper.find('[data-testid="dates-editor-reason"]').setValue('  Исправление срока  ');
}
async function preview(wrapper, data = result()) {
  apiRequest.mockResolvedValueOnce(response(data));
  await wrapper.find('[data-testid="dates-editor-preview-button"]').trigger('click');
  await flushPromises();
}

describe('ApplicationDatesEditor selected periods #2665', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2099-09-30T00:00:00Z'));
    setActivePinia(createPinia());
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'application.period.change': { value: 'allow' } } });
    apiRequest.mockReset();
  });
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers(); });

  it('uses the managed permission instead of hardcoded approver identity', () => {
    expect(mountEditor().find('[data-testid="app-detail-change-dates"]').exists()).toBe(true);
    usePermissionsStore().effective = { 'application.period.change': { value: 'deny' } };
    expect(mountEditor({ isApprover: true }).find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it.each(['В работе', 'Завершено', 'Отозвано', 'Отказано'])('retains the old group lifecycle boundary: %s', status => {
    expect(mountEditor({ application: { id: 42, status } }).find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
  });
  it('permits Unread/Processing including approved; rejects empty or nonpersisted IDs', () => {
    expect(mountEditor({ application: { id: 42, status: 'Непрочитано', confirmation: 'Согласовано' } }).find('button').exists()).toBe(true);
    expect(mountEditor({ attachments: [] }).find('button').exists()).toBe(false);
    expect(mountEditor({ application: { id: '42', status: 'В обработке' } }).find('button').exists()).toBe(false);
    expect(mountEditor({ attachments: [{ id: -1 }] }).find('button').exists()).toBe(false);
  });
  it('opens with all attachments and preserve, requires a reason and a selection', async () => {
    const wrapper = mountEditor();
    await wrapper.find('[data-testid="app-detail-change-dates"]').trigger('click');
    expect(wrapper.vm.selectedIDs).toEqual([1, 2]);
    expect(wrapper.vm.individualPolicy).toBe('preserve');
    expect(wrapper.find('[data-testid="dates-editor-current"]').text()).toContain('01.10.2099 09:00 - 03.10.2099 18:00');
    expect(wrapper.find('[data-testid="dates-editor-preview-button"]').attributes('disabled')).toBeDefined();
    await wrapper.find('[data-testid="dates-editor-reason"]').setValue('Причина');
    await wrapper.find('[data-testid="dates-editor-all"]').setValue(false);
    expect(wrapper.find('[role="alert"]').text()).toContain('Выберите хотя бы одно вложение');
    expect(wrapper.find('[data-testid="dates-editor-preview-button"]').attributes('disabled')).toBeDefined();
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('validates all four date fields before preview', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    wrapper.vm.form.endTime = '';
    await wrapper.vm.$nextTick();
    await wrapper.find('[data-testid="dates-editor-preview-button"]').trigger('click');
    expect(wrapper.vm.shownErrors.endTime).toBeTruthy();
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('previews only selected attachments and saves exactly that request with server revision', async () => {
    const wrapper = mountEditor();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await openWithReason(wrapper);
    await wrapper.find('[data-testid="dates-editor-attachment-2"]').setValue(false);
    await preview(wrapper, result([1]));
    const body = { attachment_ids: [1], period: PERIOD, individual_policy: 'preserve', reason: 'Исправление срока' };
    expect(apiRequest).toHaveBeenNthCalledWith(1, '/applications/42/attachment-period/preview', { method: 'POST', body: JSON.stringify(body) });
    expect(wrapper.find('[data-testid="dates-editor-preview"]').text()).toContain('Людей: 1');
    expect(wrapper.find('[data-testid="dates-editor-preview"]').text()).toContain('Машин: 1');
    apiRequest.mockResolvedValueOnce(response(result([1])));
    await wrapper.find('[data-testid="dates-editor-save"]').trigger('click');
    await flushPromises();
    expect(apiRequest).toHaveBeenNthCalledWith(2, '/applications/42/attachment-period', { method: 'PUT', body: JSON.stringify({ ...body, expected_revision: REVISION }) });
    expect(wrapper.emitted('changed')).toEqual([[result([1])]]);
    expect(wrapper.vm.show).toBe(false);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
  });
  it('requires a new preview after explicit replace or any input edit', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    await preview(wrapper);
    await wrapper.find('[data-testid="dates-editor-policy"]').setValue('replace');
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.find('[data-testid="dates-editor-save"]').exists()).toBe(false);
    await preview(wrapper, result([1, 2], 'replace'));
    expect(JSON.parse(apiRequest.mock.calls[1][1].body).individual_policy).toBe('replace');
    expect(wrapper.find('[data-testid="dates-editor-preview"]').text()).toContain('будут заменены');
    await wrapper.find('[data-testid="dates-editor-reason"]').setValue('Другая причина');
    expect(wrapper.vm.snapshot).toBeNull();
  });
  it('does not retry a 409; refresh obtains a revision and still requires explicit save', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    await preview(wrapper);
    apiRequest.mockResolvedValueOnce(response({ message: 'stale' }, 409));
    await wrapper.find('[data-testid="dates-editor-save"]').trigger('click');
    await flushPromises();
    expect(apiRequest).toHaveBeenCalledTimes(2);
    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(wrapper.vm.conflict).toBe(true);
    expect(wrapper.find('[data-testid="dates-editor-preview-button"]').text()).toBe('Обновить данные');
    const fresh = result([1, 2], 'preserve', 'b'.repeat(64));
    await preview(wrapper, fresh);
    expect(apiRequest).toHaveBeenCalledTimes(3);
    expect(wrapper.vm.snapshot.period_revision).toBe(fresh.period_revision);
    expect(wrapper.find('[data-testid="dates-editor-save"]').exists()).toBe(true);
    expect(wrapper.emitted('changed')).toBeUndefined();
  });
  it('keeps the window open and displays a preview error without saving', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    apiRequest.mockResolvedValueOnce(response({ message: 'Недостаточно прав' }, 403));
    await wrapper.find('[data-testid="dates-editor-preview-button"]').trigger('click');
    await flushPromises();
    expect(wrapper.vm.error).toBe('Недостаточно прав');
    expect(wrapper.vm.show).toBe(true);
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.emitted('changed')).toBeUndefined();
  });
  it('rejects a malformed or mismatched preview response', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    await preview(wrapper, { ...result(), application_id: 43 });
    expect(wrapper.vm.error).toContain('подтверждение');
    expect(wrapper.vm.snapshot).toBeNull();
  });
  it('cancels without writing and ignores a stale preview after application switch', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    wrapper.vm.close();
    expect(wrapper.vm.show).toBe(false);
    expect(apiRequest).not.toHaveBeenCalled();
    await openWithReason(wrapper);
    let resolve;
    apiRequest.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    await wrapper.find('[data-testid="dates-editor-preview-button"]').trigger('click');
    await wrapper.setProps({ application: { id: 43, status: 'В обработке' } });
    resolve(response(result()));
    await flushPromises();
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.vm.show).toBe(false);
  });
  it('closes and invalidates preview when the managed permission is revoked', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    await preview(wrapper);
    usePermissionsStore().effective = {};
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.show).toBe(false);
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
});
