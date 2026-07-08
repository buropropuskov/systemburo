import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getApprovers = vi.fn();
const getAllUsers = vi.fn();
const updateApprover = vi.fn();
vi.mock('@/api/approvers', () => ({
  getApprovers: (...a) => getApprovers(...a),
  getAllUsers: (...a) => getAllUsers(...a),
  addApprover: vi.fn(),
  updateApprover: (...a) => updateApprover(...a),
  deleteApprover: vi.fn(),
}));
// fetchCurrentUser дёргает apiRequest('/users/me') - не тестируем, отдаём ok:false.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
const notify = vi.hoisted(() => vi.fn());
const enqueue = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify, enqueue }) }));
vi.mock('@/composables/useOverlayClose', () => ({
  useOverlayClose: () => ({ onOverlayMousedown: vi.fn(), onOverlayMouseup: vi.fn() }),
}));

import ApplicationApprovers from '../ApplicationApprovers.vue';

function approver(over = {}) {
  return {
    id: 1, user_id: 5, username: 'myakotnyh',
    last_name: 'Мякотных', first_name: 'Сергей', middle_name: 'Михайлович',
    position: 'Оператор', organization: 'ООО', company: null,
    display_name: null, created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

async function mountWith(list) {
  getApprovers.mockResolvedValue(list);
  getAllUsers.mockResolvedValue([]);
  const wrapper = mount(ApplicationApprovers, {
    global: { stubs: { teleport: true, SearchComponent: true, RefreshButton: true, ApplicationApproverHistoryModal: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('ApplicationApprovers - маска отображаемого имени', () => {
  beforeEach(() => {
    getApprovers.mockReset();
    getAllUsers.mockReset();
    updateApprover.mockReset();
    notify.mockClear();
  });

  it('selectApprover заполняет maskDraft текущей маской', async () => {
    const wrapper = await mountWith([approver({ display_name: 'Оператор Бюро' })]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    expect(wrapper.vm.maskDraft).toBe('Оператор Бюро');
  });

  it('maskChanged: true при отличии от текущей, false при совпадении', async () => {
    const wrapper = await mountWith([approver({ display_name: 'Оператор Бюро' })]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    expect(wrapper.vm.maskChanged).toBe(false);
    wrapper.vm.maskDraft = 'Дежурный';
    expect(wrapper.vm.maskChanged).toBe(true);
  });

  it('saveMask отправляет trim-строку в updateApprover и уведомляет', async () => {
    const wrapper = await mountWith([approver()]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    wrapper.vm.maskDraft = '  Оператор Бюро  ';
    getApprovers.mockResolvedValue([approver({ display_name: 'Оператор Бюро' })]);
    await wrapper.vm.saveMask();
    await flushPromises();
    expect(updateApprover).toHaveBeenCalledWith(1, 'Оператор Бюро');
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Оператор Бюро' }));
  });

  it('saveMask с пустым значением снимает маску (null)', async () => {
    const wrapper = await mountWith([approver({ display_name: 'Оператор Бюро' })]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    wrapper.vm.maskDraft = '   ';
    getApprovers.mockResolvedValue([approver({ display_name: null })]);
    await wrapper.vm.saveMask();
    await flushPromises();
    expect(updateApprover).toHaveBeenCalledWith(1, null);
  });

  it('saveMask no-op при неизменной маске', async () => {
    const wrapper = await mountWith([approver({ display_name: 'Оператор Бюро' })]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    await wrapper.vm.saveMask();
    expect(updateApprover).not.toHaveBeenCalled();
  });

  it('ошибка saveMask уведомляет об ошибке', async () => {
    const wrapper = await mountWith([approver()]);
    wrapper.vm.selectApprover(wrapper.vm.approvers[0]);
    wrapper.vm.maskDraft = 'Оператор';
    updateApprover.mockRejectedValue(new Error('fail'));
    await wrapper.vm.saveMask();
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('строка списка показывает бейдж "маска" при заданном display_name', async () => {
    const wrapper = await mountWith([approver({ display_name: 'Оператор Бюро' })]);
    expect(wrapper.find('.mask-tag').exists()).toBe(true);
  });
});
