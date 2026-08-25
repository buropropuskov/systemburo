import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationDetail from '../ApplicationDetail.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Деталь на mounted дёргает loadApplicationDetails/loadCommonData/markAsRead - глушим,
// проверяем только условия показа кнопки "Дополнить" (#1685).
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue({}) }));

const stubs = {
  teleport: true,
  ForwardModal: true,
  SupplementModal: true,
  ApplicationActionBar: {
    name: 'ApplicationActionBar',
    template: '<div class="action-bar-stub"><slot name="user-actions" /></div>',
  },
  ApplicationAttachments: true,
  ApplicationMessageModal: true,
  ApplicationAttachmentDetail: true,
  ApplicationConfirmation: true,
  ApplicationHistory: true,
  ApplicationQuestions: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  BlacklistOverrideModal: true,
  Badge: true,
};

const AUTHOR_ID = 5;

const APP = {
  id: 1,
  application_number: 'A-1',
  sending_datetime: '2026-01-01T10:00:00Z',
  status: 'В работе',
  confirmation: 'Согласовано',
  organization_name: 'Орг',
  sender_user_id: AUTHOR_ID,
};

function seedPerms({ mode = 'normal', allow = ['action.supplement.application'] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountDetail(application = {}, props = {}) {
  return mount(ApplicationDetail, {
    props: {
      application: { ...APP, ...application },
      currentUserId: AUTHOR_ID,
      mode: 'user',
      ...props,
    },
    global: { stubs },
  });
}

const supplementBtn = w => w.find('[data-testid="app-detail-button-supplement"]');

describe('ApplicationDetail - кнопка «Дополнить» (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('автор с правом и без открытого дополнения: кнопка видна', () => {
    seedPerms();
    const w = mountDetail();
    expect(w.vm.canSupplementApplication).toBe(true);
    expect(supplementBtn(w).exists()).toBe(true);
  });

  it('поля open_supplement нет в ответе: кнопка всё равно работает', () => {
    seedPerms();
    const w = mountDetail();
    // Именно этот случай отдаёт бэк до параллельного среза - undefined не должен
    // читаться как «раунд открыт».
    expect('open_supplement' in w.vm.applicationData).toBe(false);
    expect(w.vm.canSupplementApplication).toBe(true);
  });

  it('у заявки открытое дополнение: кнопка скрыта', async () => {
    seedPerms();
    const w = mountDetail({ open_supplement: { id: 7, status: 'pending', number: 1 } });
    expect(w.vm.canSupplementApplication).toBe(false);
    await w.vm.$nextTick();
    expect(supplementBtn(w).exists()).toBe(false);
  });

  it('закрытые статусы: кнопка скрыта', () => {
    seedPerms();
    for (const status of ['Завершено', 'Отказано', 'Отозвана', 'Не согласовано']) {
      const w = mountDetail({ status });
      expect(w.vm.canSupplementApplication, status).toBe(false);
    }
  });

  it('допустимые статусы совпадают с белым списком бэка', () => {
    seedPerms();
    for (const status of ['Непрочитано', 'В обработке', 'В работе']) {
      const w = mountDetail({ status });
      expect(w.vm.canSupplementApplication, status).toBe(true);
    }
  });

  it('не автор заявки: кнопка скрыта даже с правом', () => {
    seedPerms();
    const w = mountDetail({ sender_user_id: AUTHOR_ID + 1 });
    expect(w.vm.canSupplementApplication).toBe(false);
    expect(supplementBtn(w).exists()).toBe(false);
  });

  it('автор без права action.supplement.application: кнопка скрыта', () => {
    seedPerms({ allow: [] });
    const w = mountDetail();
    expect(w.vm.canSupplementApplication).toBe(false);
    expect(supplementBtn(w).exists()).toBe(false);
  });

  it('супер-админ проходит без явного гранта', () => {
    seedPerms({ mode: 'super', allow: [] });
    const w = mountDetail();
    expect(w.vm.canSupplementApplication).toBe(true);
  });

  it('клик открывает окно, отправка дополнения перезапрашивает деталь', async () => {
    seedPerms();
    const w = mountDetail();
    await supplementBtn(w).trigger('click');
    expect(w.vm.showSupplementModal).toBe(true);

    const reload = vi.spyOn(w.vm, 'loadApplicationDetails').mockResolvedValue(undefined);
    w.vm.onSupplementSubmitted();
    expect(w.vm.showSupplementModal).toBe(false);
    expect(reload).toHaveBeenCalledWith(w.vm.applicationData, { preserveSelection: true });
    expect(w.emitted('application-changed')).toBeTruthy();
  });
});
