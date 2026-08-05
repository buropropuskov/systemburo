import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationDetail from '../ApplicationDetail.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Деталь на mounted тянет несколько ручек - отвечаем на них пустым успехом: раунды
// подтягиваются следом за успешной загрузкой детали (оттуда приходит supplements_count),
// и на отказе /details проверять было бы нечего (#1685).
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }),
}));

const getApplicationSupplements = vi.fn();
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue({}),
  getApplicationSupplements: (...args) => getApplicationSupplements(...args),
}));

const stubs = {
  teleport: true,
  ForwardModal: true,
  SupplementModal: true,
  ApplicationActionBar: true,
  ApplicationAttachments: true,
  ApplicationMessageModal: true,
  ApplicationAttachmentDetail: true,
  ApplicationConfirmation: true,
  // Деталь после успешного действия дёргает у истории loadHistory - автозаглушке
  // такого метода не досталось бы, и тест упал бы на чужом коде.
  ApplicationHistory: { name: 'ApplicationHistory', template: '<div />', methods: { loadHistory() {} } },
  ApplicationQuestions: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  BlacklistOverrideModal: true,
  Badge: true,
};

const APP = {
  id: 1,
  application_number: 'A-1',
  sending_datetime: '2026-01-01T10:00:00Z',
  status: 'В работе',
  confirmation: 'Согласовано',
  organization_name: 'Орг',
  sender_user_id: 5,
};

const ROUND = {
  id: 11,
  number: 2,
  status: 'pending',
  created_by_name: 'Сидоров П. И.',
  created_at: '2026-08-05T09:30:00Z',
  counts: { vehicles: 1, employees: 0, items: 0 },
  approvals: [{ user_id: 5, full_name: 'Иванов И. И.', required_approval: true, approval_status: 'pending' }],
};

function mountDetail(application = {}) {
  const perms = usePermissionsStore();
  perms.mode = 'super';
  perms.effective = {};
  return mount(ApplicationDetail, {
    props: { application: { ...APP, ...application }, currentUserId: 5, mode: 'user' },
    global: { stubs },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  getApplicationSupplements.mockReset().mockResolvedValue([ROUND]);
});

describe('ApplicationDetail - загрузка раундов дополнения (#1685)', () => {
  it('у заявки без дополнений список не запрашивается и панель не рисуется', async () => {
    const w = mountDetail({ supplements_count: 0 });
    await w.vm.loadSupplements();
    expect(getApplicationSupplements).not.toHaveBeenCalled();
    expect(w.findComponent({ name: 'SupplementPanel' }).exists()).toBe(false);
  });

  it('раунды подтягиваются и попадают в панель', async () => {
    const w = mountDetail({ supplements_count: 1 });
    await w.vm.loadSupplements();
    await flushPromises();

    expect(getApplicationSupplements).toHaveBeenCalledWith(1);
    expect(w.vm.supplements).toEqual([ROUND]);
    const panel = w.findComponent({ name: 'SupplementPanel' });
    expect(panel.exists()).toBe(true);
    expect(panel.props('supplements')).toEqual([ROUND]);
    expect(panel.props('error')).toBe('');
  });

  it('признак раунда приезжает только с открытым дополнением - список всё равно тянем', async () => {
    const w = mountDetail({ open_supplement: { id: 11, number: 2, status: 'pending' } });
    await w.vm.loadSupplements();
    expect(getApplicationSupplements).toHaveBeenCalledWith(1);
  });

  it('сигнал application.updated перезапрашивает раунды - чужое решение долетает без F5', async () => {
    const w = mountDetail({ supplements_count: 1 });
    await flushPromises();
    getApplicationSupplements.mockClear();
    getApplicationSupplements.mockResolvedValue([{ ...ROUND, status: 'approved' }]);

    w.vm.refreshLiveDetail();
    await flushPromises();

    expect(getApplicationSupplements).toHaveBeenCalledWith(1);
    expect(w.vm.supplements[0].status).toBe('approved');
  });

  it('успешное действие по раунду обновляет и деталь, и список раундов', async () => {
    const w = mountDetail({ supplements_count: 1 });
    await flushPromises();
    getApplicationSupplements.mockClear();

    w.vm.handleActionCompleted({ success: true, message: 'Дополнение №2 согласовано', type: 'success' });
    await flushPromises();

    expect(getApplicationSupplements).toHaveBeenCalledWith(1);
  });

  it('ошибка загрузки уходит в панель человеческим текстом и не роняет карточку', async () => {
    getApplicationSupplements.mockRejectedValue(new Error('Не удалось загрузить дополнения заявки'));
    const w = mountDetail({ supplements_count: 1 });
    await w.vm.loadSupplements();
    await flushPromises();

    expect(w.vm.supplementsError).toBe('Не удалось загрузить дополнения заявки');
    expect(w.vm.supplementsLoading).toBe(false);
    const panel = w.findComponent({ name: 'SupplementPanel' });
    expect(panel.exists()).toBe(true);
    expect(panel.props('error')).toBe('Не удалось загрузить дополнения заявки');
    // Сама карточка заявки жива - её номер по-прежнему на экране.
    expect(w.text()).toContain('A-1');
  });

  it('ответ устаревшего запроса не затирает более свежий', async () => {
    const w = mountDetail({ supplements_count: 1 });
    await flushPromises();

    let releaseStale;
    getApplicationSupplements.mockImplementationOnce(
      () => new Promise(resolve => { releaseStale = () => resolve([{ ...ROUND, status: 'pending' }]); }));
    const stale = w.vm.loadSupplements();

    getApplicationSupplements.mockResolvedValue([{ ...ROUND, status: 'accepted' }]);
    await w.vm.loadSupplements();

    releaseStale();
    await stale;
    await flushPromises();

    expect(w.vm.supplements[0].status).toBe('accepted');
  });
});
