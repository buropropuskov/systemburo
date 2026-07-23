import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import PassReportModal from '@/components/PassReportModal.vue';

// Суточный отчёт охранника по проходам: загрузка по открытию (show=false ->
// true, как реальный родитель), крупные карточки «заехало/выехало», разбивка по
// охранникам только при >1 строке, история свёрнута под кнопку, «Без автора»
// для user_id=0, устойчивость к ошибке бэка.

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

  it('открытие грузит живое окно и историю, рисует крупные счётчики машин', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(getPassReportLive).toHaveBeenCalledWith(7);
    expect(listPassReports).toHaveBeenCalledWith(7, {});
    expect(td(w, 'pr-total-cars-in').text()).toBe('3');
    expect(td(w, 'pr-total-cars-out').text()).toBe('2');
    // Простые слова, не жаргон.
    expect(td(w, 'pass-report-live').text()).toContain('Заехало');
    expect(td(w, 'pass-report-live').text()).toContain('Выехало');
  });

  it('одна строка охранника — разбивка «кто сколько» не показывается', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(td(w, 'pass-report-breakdown').exists()).toBe(false);
  });

  it('несколько охранников — показывает разбивку с именами', async () => {
    getPassReportLive.mockResolvedValue({
      ...liveFixture,
      rows: [
        { user_id: 5, user_name: 'Иванов Иван', car_entries: 2, car_exits: 1, people_entries: 0, people_exits: 0 },
        { user_id: 6, user_name: 'Петров Пётр', car_entries: 1, car_exits: 1, people_entries: 0, people_exits: 0 },
      ],
    });
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    const bd = td(w, 'pass-report-breakdown');
    expect(bd.exists()).toBe(true);
    expect(bd.text()).toContain('Иванов Иван');
    expect(bd.text()).toContain('Петров Пётр');
  });

  it('cars-таблица без людских событий не рендерит карточку людей', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(td(w, 'pr-total-people-in').exists()).toBe(false);
    expect(td(w, 'pass-report-live').text()).not.toContain('Зашло');
  });

  it('история свёрнута по умолчанию, раскрывается по кнопке и пишет дни по-человечески', async () => {
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    // Свёрнута: дней не видно, но кнопка-аккордеон есть.
    expect(td(w, 'pass-report-day').exists()).toBe(false);
    const toggle = td(w, 'pass-report-history-toggle');
    expect(toggle.text()).toContain('Показать прошлые дни');

    await toggle.trigger('click');
    await flushPromises();

    const day = td(w, 'pass-report-day');
    expect(day.exists()).toBe(true);
    expect(day.text()).toContain('21 июля 2026');
    expect(day.text()).toContain('Машины: заехало 5, выехало 4');
    // Разбивка по охранникам внутри дня (2 строки) с «Без автора».
    expect(day.text()).toContain('Иванов Иван');
    expect(day.text()).toContain('Без автора');
    expect(toggle.text()).toContain('Скрыть прошлые дни');
  });

  it('открытая до прихода tableId модалка дозагружает данные по его приезду', async () => {
    const w = mountModal({ tableId: null });
    await w.setProps({ show: true });
    await flushPromises();
    expect(getPassReportLive).not.toHaveBeenCalled();

    await w.setProps({ tableId: 7 });
    await flushPromises();
    expect(getPassReportLive).toHaveBeenCalledWith(7);
    expect(listPassReports).toHaveBeenCalledWith(7, {});
  });

  it('пустая история показывает заглушку после раскрытия', async () => {
    listPassReports.mockResolvedValue({ days: [] });
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();
    await td(w, 'pass-report-history-toggle').trigger('click');
    await flushPromises();

    expect(td(w, 'pass-report-days-empty').exists()).toBe(true);
  });

  it('ошибка бэка (unwrap кидает) рисует заглушку живого окна, а не крашит рендер', async () => {
    getPassReportLive.mockRejectedValue(new Error('Недостаточно прав'));
    listPassReports.mockRejectedValue(new Error('Недостаточно прав'));
    const w = mountModal();
    await w.setProps({ show: true });
    await flushPromises();

    expect(td(w, 'pass-report-live').text()).toContain('Не удалось загрузить отчёт');
    expect(td(w, 'pr-total-cars-in').exists()).toBe(false);
  });
});
