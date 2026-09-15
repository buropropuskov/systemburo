import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

/**
 * Выгрузка суточного отчёта по проходам после перехода на общий лист (#2418).
 *
 * Спека поведенческая, а не структурная: на живом стенде клик по «Скачать в Excel»
 * ничего не делал, и надо было понять, доходит ли дело до сборки листа.
 */
vi.mock('@/api/pass-reports', () => ({
  getPassReportLive: vi.fn(),
  listPassReports: vi.fn(),
}));
const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));
const downloadExcelSheet = vi.fn();
vi.mock('@/utils/excelSheet', () => ({ downloadExcelSheet: (...a) => downloadExcelSheet(...a) }));

import PassReportModal from '@/components/PassReportModal.vue';
import { getPassReportLive, listPassReports } from '@/api/pass-reports';

const BaseModalStub = { name: 'BaseModal', props: ['show', 'title', 'width', 'radius', 'contentClass'], template: '<div v-if="show" class="m"><slot /></div>' };
const DateFilterStub = { name: 'DateFilter', template: '<div />' };
const RefreshButtonStub = { name: 'RefreshButton', props: ['loading'], template: '<button />' };

const live = { period_start: '2026-09-13T18:30:00Z', period_end: '2026-09-14T09:15:00Z', rows: [], totals: { car_entries: 0, car_exits: 0, people_entries: 0, people_exits: 0 } };
const days = {
  days: [{
    report_date: '2026-09-11',
    rows: [
      { user_id: 5, user_name: 'Иванов Иван', car_entries: 4, car_exits: 4, people_entries: 0, people_exits: 0 },
      { user_id: 7, user_name: 'Петров Пётр', car_entries: 1, car_exits: 0, people_entries: 0, people_exits: 0 },
    ],
    totals: { car_entries: 5, car_exits: 4, people_entries: 0, people_exits: 0 },
  }],
};

beforeEach(() => {
  vi.clearAllMocks();
  getPassReportLive.mockResolvedValue(live);
  listPassReports.mockResolvedValue(days);
});

async function open() {
  const wrapper = mount(PassReportModal, {
    props: { show: false, tableId: 7, tableType: 'cars', tableDisplayName: 'КПП 1', currentUserName: 'Сидоров С.С.' },
    global: { stubs: { BaseModal: BaseModalStub, DateFilter: DateFilterStub, RefreshButton: RefreshButtonStub, AppIcon: true, LoaderSpinner: true, teleport: true, transition: false } },
  });
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

describe('выгрузка отчёта по проходам (#2418)', () => {
  it('кнопка активна, когда дни загружены', async () => {
    const wrapper = await open();
    const btn = wrapper.find('[data-testid="pass-report-export"]');
    expect(btn.exists()).toBe(true);
    expect(btn.attributes('disabled')).toBeUndefined();
  });

  it('клик собирает лист: строки дня, подытог полужирным, подписи и имя файла', async () => {
    const wrapper = await open();
    await wrapper.find('[data-testid="pass-report-export"]').trigger('click');
    await flushPromises();

    expect(downloadExcelSheet).toHaveBeenCalledTimes(1);
    const [spec, filename] = downloadExcelSheet.mock.calls[0];
    expect(spec.header).toEqual(['Дата', 'Охранник', 'Машины: заехало', 'Машины: выехало', 'Люди: зашло', 'Люди: вышло']);
    expect(spec.widths).toEqual([14, 34, 16, 16, 14, 14]);
    // Две строки охранников плюс подытог по посту, и подытог отмечен полужирным.
    expect(spec.rows).toHaveLength(3);
    expect(spec.rows[2][1]).toBe('Итого по посту');
    expect(spec.boldRows).toEqual([2]);
    expect(spec.info[0][0]).toBe('Отчёт сформировал:');
    expect(filename).toMatch(/^Otchet_po_prohodam_.*\.xlsx$/);
    expect(notify).not.toHaveBeenCalled();
  });
});

/**
 * Кнопка выгрузки не должна жить внутри аккордеона «Прошлые дни» (#2517).
 *
 * Пока она стояла внутри, свёрнутая обёртка (нулевая высота, overflow: hidden) делала её
 * недостижимой: человек открывал отчёт и не видел, что выгрузка вообще есть. Проверяем
 * и структуру - кнопка вне обёртки, - и поведение при свёрнутом списке.
 */
describe('кнопка выгрузки видна сразу, а не внутри свёрнутого списка (#2517)', () => {
  it('кнопка стоит вне аккордеона прошлых дней', async () => {
    const wrapper = await open();
    const accordion = wrapper.find('.pr-history-wrap');
    expect(accordion.exists()).toBe(true);
    expect(accordion.find('[data-testid="pass-report-export"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pass-report-export"]').exists()).toBe(true);
  });

  it('при свёрнутом списке кнопка активна и выгружает', async () => {
    const wrapper = await open();
    // Аккордеон по умолчанию свёрнут - класса open на обёртке нет.
    expect(wrapper.find('.pr-history-wrap').classes()).not.toContain('open');

    await wrapper.find('[data-testid="pass-report-export"]').trigger('click');
    await flushPromises();

    expect(downloadExcelSheet).toHaveBeenCalledTimes(1);
  });
});
