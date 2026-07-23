import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import PassReportModal from '@/components/PassReportModal.vue';

// Суточный отчёт охранника по проходам: загрузка по открытию (show=false ->
// true, как реальный родитель), счётчики totals, разбивка по охранникам,
// секции по типу таблицы, «Без автора» для user_id=0.

vi.mock('@/api/pass-reports', () => ({
  getPassReportLive: vi.fn(),
  listPassReports: vi.fn(),
}));
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { getPassReportLive, listPassReports } from '@/api/pass-reports';

const BaseModalStub = {
  name: 'BaseModal',
  props: ['show', 'title', 'width', 'radius', 'contentClass'],
  template: '<div v-if="show" class="base-modal-stub"><slot /></div>',
};
const DateFilterStub = { name: 'DateFilter', template: '<div class="date-filter-stub" />' };
const RefreshButtonStub = { name: 'RefreshButton', props: ['loading'], template: '<button class="refresh-stub" />' };

const liveFixture = {
  period_start: '2026-07-21T18:30:00Z',
  period_end: '2026-07-22T09:15:00Z',
  rows: [
    { user_id: 5, user_name: 'Иванов Иван', car_entries: 2, car_exits: 1, people_entries: 0, people_exits: 0 },
  ],
  totals: { car_entries: 3, car_exits: 2, people_entries: 0, people_exits: 0 },
};

const daysFixture = {
  days: [
    {
      report_date: '2026-07-21',
      period_start: '2026-07-20T18:30:00Z',
      period_end: '2026-07-21T18:30:00Z',
      rows: [
        { user_id: 5, user_name: 'Иванов Иван', car_entries: 4, car_exits: 4, people_entries: 0, people_exits: 0 },
        { user_id: 0, user_name: '', car_entries: 1, car_exits: 0, people_entries: 0, people_exits: 0 },
      ],
      totals: { car_entries: 5, car_exits: 4, people_entries: 0, people_exits: 0 },
    },
  ],
};

function mountModal(props = {}) {
  return mount(PassReportModal, {
    props: {
      show: false,
      tableId: 7,
      tableType: 'cars',
      tableDisplayName: 'КПП 1',
      ...props,
    },
    global: {
      stubs: { BaseModal: BaseModalStub, DateFilter: DateFilterStub, RefreshButton: RefreshButtonStub },
    },
  });
}

const td = (w, id) => w.find(`[data-testid="${id}"]`);

beforeEach(() => {
  vi.clearAllMocks();
  getPassReportLive.mockResolvedValue(liveFixture);
  listPassReports.mockResolvedValue(daysFixture);
});

describe('PassReportModal', () => {
  it('не грузит данные, пока модалка закрыта', () => {
    mountModal();
    expect(getPassReportLive).not.toHaveBeenCalled();
    expect(listPassReports).not.toHaveBeenCalled();
  });

  it('открытие грузит живое окно и историю, рендерит totals и строку охранника', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(getPassReportLive).toHaveBeenCalledWith(7);
    expect(listPassReports).toHaveBeenCalledWith(7, {});
    expect(td(w, 'pr-total-car-entries').text()).toBe('3');
    expect(td(w, 'pr-total-car-exits').text()).toBe('2');

    const rows = td(w, 'pass-report-rows').findAll('tbody tr');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Иванов Иван');
    expect(rows[0].text()).toContain('2');
  });

  it('cars-таблица без людских событий не рендерит счётчики людей', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(td(w, 'pr-total-people-entries').exists()).toBe(false);
    expect(w.find('[data-testid="pass-report-live"]').text()).not.toContain('Люди');
  });

  it('история рендерит день с итогами и разбивкой, user_id=0 подписан «Без автора»', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    const day = td(w, 'pass-report-day');
    expect(day.exists()).toBe(true);
    expect(day.text()).toContain('21.07.2026');
    expect(day.text()).toContain('машины 5 / 4');
    expect(day.text()).toContain('Иванов Иван');
    expect(day.text()).toContain('Без автора');
  });

  it('пустая история показывает заглушку', async () => {
    listPassReports.mockResolvedValue({ days: [] });
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(td(w, 'pass-report-days-empty').exists()).toBe(true);
  });
});
