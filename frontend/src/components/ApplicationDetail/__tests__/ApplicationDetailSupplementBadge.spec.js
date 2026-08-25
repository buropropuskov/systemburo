import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationDetail from '../ApplicationDetail.vue';
import { usePermissionsStore } from '@/stores/permissions';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue({}) }));

// ApplicationActionBar НЕ глушим: именно он рисует бейдж согласования, а проверить надо,
// что второй бейдж встал рядом с ним, а не вместо него.
const stubs = {
  teleport: true,
  ForwardModal: true,
  SupplementModal: true,
  ApplicationAttachments: true,
  ApplicationMessageModal: true,
  ApplicationAttachmentDetail: true,
  ApplicationConfirmation: true,
  ApplicationHistory: true,
  ApplicationQuestions: true,
  ForwardMessages: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  BlacklistOverrideModal: true,
};

const SUPPLEMENT_BADGE = '[data-testid="app-detail-supplement-round-badge"]';
const APPROVED_BADGE = '.status-approved-badge';

const APP = {
  id: 1,
  application_number: 'A-1',
  sending_datetime: '2026-01-01T10:00:00Z',
  status: 'Непрочитано',
  confirmation: 'Согласовано',
  organization_name: 'Орг',
  sender_user_id: 5,
};

async function mountDetail(application = {}) {
  const perms = usePermissionsStore();
  perms.mode = 'super';
  perms.effective = {};

  const wrapper = mount(ApplicationDetail, {
    props: {
      application: { ...APP, ...application },
      // Не согласующий и не принимающий - тогда ActionBar показывает информационный
      // бейдж согласования, а не кнопки голосования.
      currentUserId: 99,
      mode: 'center',
    },
    global: { stubs },
  });
  // Пока роли и голоса грузятся, панель действий держит лоадер вместо бейджей.
  wrapper.vm.actionsReady = true;
  await wrapper.vm.$nextTick();
  return wrapper;
}

describe('ApplicationDetail — бейдж открытого дополнения в шапке (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('без open_supplement второго бейджа нет, бейдж согласования на месте', async () => {
    const wrapper = await mountDetail();

    expect(wrapper.find(APPROVED_BADGE).text()).toBe('Согласовано');
    expect(wrapper.find(SUPPLEMENT_BADGE).exists()).toBe(false);
  });

  it('раунд на согласовании: бейдж «+ Дополнение №N на согласовании» рядом с «Согласовано»', async () => {
    const wrapper = await mountDetail({
      open_supplement: { id: 7, number: 2, status: 'pending', counts: { vehicles: 1, employees: 0, items: 0 } },
    });

    expect(wrapper.find(SUPPLEMENT_BADGE).text()).toBe('+ Дополнение №2 на согласовании');
    expect(wrapper.find(SUPPLEMENT_BADGE).classes()).toContain('badge--warning');
    // Статус заявки подменять нельзя: от него зависит допуск уже выданных пропусков.
    expect(wrapper.find(APPROVED_BADGE).text()).toBe('Согласовано');
    expect(wrapper.vm.applicationData.confirmation).toBe('Согласовано');
  });

  // Состояние, в котором фича живёт на самом деле: отдельный раунд заводится только у
  // заявки, УЖЕ принятой в работу. Ряд бейджей в этот момент рисует «В работе» - до
  // ветки «Согласовано» очередь не доходит, статус проверяется раньше согласования.
  // Соседний тест берёт согласованную, но ещё не открытую заявку - тоже допустимое
  // сочетание, просто не то, при котором появляется раунд.
  it('заявка в работе: бейдж дополнения встаёт рядом со статусом, не подменяя его', async () => {
    const wrapper = await mountDetail({
      status: 'В работе',
      open_supplement: { id: 9, number: 1, status: 'pending', counts: { vehicles: 0, employees: 2, items: 0 } },
    });

    expect(wrapper.find(SUPPLEMENT_BADGE).text()).toBe('+ Дополнение №1 на согласовании');
    // Статус заявки не тронут: от него зависит допуск уже выданных пропусков.
    expect(wrapper.vm.applicationData.status).toBe('В работе');
    expect(wrapper.vm.applicationData.confirmation).toBe('Согласовано');
  });

  it('раунд согласован: бейдж «+ Дополнение №N ждёт принятия»', async () => {
    const wrapper = await mountDetail({
      open_supplement: { id: 7, number: 3, status: 'approved' },
    });

    expect(wrapper.find(SUPPLEMENT_BADGE).text()).toBe('+ Дополнение №3 ждёт принятия');
    expect(wrapper.find(SUPPLEMENT_BADGE).classes()).toContain('badge--info');
    expect(wrapper.find(APPROVED_BADGE).exists()).toBe(true);
  });

  it('бейдж стоит в шапке, в одном ряду с панелью действий', async () => {
    const wrapper = await mountDetail({ open_supplement: { id: 7, number: 2, status: 'pending' } });

    // Без прямого потомка: @vue/test-utils подставляет transition-stub, в проде
    // <transition> своего узла в DOM не даёт.
    const badge = wrapper.find('.detail-header .detail-header-right .supplement-round-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.attributes('data-testid')).toBe('app-detail-supplement-round-badge');
  });

  it('бейдж исчезает, когда раунд закрылся', async () => {
    const wrapper = await mountDetail({ open_supplement: { id: 7, number: 2, status: 'pending' } });
    expect(wrapper.find(SUPPLEMENT_BADGE).exists()).toBe(true);

    wrapper.vm.applicationData = { ...wrapper.vm.applicationData, open_supplement: null };
    await wrapper.vm.$nextTick();

    expect(wrapper.find(SUPPLEMENT_BADGE).exists()).toBe(false);
    expect(wrapper.find(APPROVED_BADGE).text()).toBe('Согласовано');
  });

  it('раунд без номера не печатает «№undefined»', async () => {
    const wrapper = await mountDetail({ open_supplement: { id: 7, status: 'pending' } });

    expect(wrapper.find(SUPPLEMENT_BADGE).text()).toBe('+ Дополнение на согласовании');
  });

  it('подсказка объясняет, почему статус заявки не изменился', async () => {
    const wrapper = await mountDetail({ open_supplement: { id: 7, number: 2, status: 'approved' } });

    expect(wrapper.find(SUPPLEMENT_BADGE).attributes('title'))
      .toContain('Статус самой заявки не менялся');
  });
});
