import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Шапка раздела мониторинга (#2125, S9b): показатели переехали из строки подписей
// в те же карточки, что на вкладке аналитики, сама шапка приведена к эталонным
// 50px, а кнопка обновления встала в неё - как у прочих разделов управления.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { apiRequest } from '@/api/client';
import { journalCalls, mountView, resetApiMocks, unmountAll } from './helpers/requestsView';

const VIEW = readFileSync(resolve(__dirname, '../RequestsView.vue'), 'utf8');
const STYLE = VIEW.slice(VIEW.indexOf('<style'));

/** Сколько раз раздел сходил по адресу за время теста. */
function callsTo(prefix) {
  return apiRequest.mock.calls.filter(([url]) => url.startsWith(prefix)).length;
}

/**
 * Показатели шапки: подпись карточки -> значение. Разряды в числах разделяет
 * неразрывный пробел (toLocaleString ru-RU), в сравнении он ни при чём.
 */
function kpis(wrapper) {
  return Object.fromEntries(wrapper.get('.rv-stats').findAll('[data-testid="monitoring-kpi"]')
    .map(card => [card.get('.kpi-lab').text(), card.get('.kpi-val').text().replace(/\u00a0/g, ' ')]));
}

/** Ответы шапки: показатели за час и счётчики ленты. */
function mockHeader({ stats = {}, realtime = null } = {}) {
  apiRequest.mockImplementation((path) => {
    if (path.startsWith('/request-logs/stats')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(stats) });
    }
    if (path.startsWith('/request-logs/realtime')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(realtime || {}) });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve([]) });
  });
}

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
});

describe('Мониторинг запросов, шапка раздела', () => {
  it('показатели живут в карточках, а не в строке подписей', async () => {
    mockHeader({
      stats: { total: 353093, today: 4120, median_duration: 8.4, p95_duration: 141, error_rate: 0.4, requests_per_minute: 61.2 },
      realtime: { last_second_count: 2, last_minute_count: 118 },
    });
    const { wrapper } = await mountView();
    await flushPromises();

    expect(kpis(wrapper)['Запросов всего']).toBe('353 093');
    expect(kpis(wrapper)['Доля ошибок за час'], 'час назван в подписи: у аналитики своя доля ошибок за период').toBe('0.4%');
    expect(kpis(wrapper)['Сейчас'], 'счётчик ленты показывает и секунду, и минуту').toBe('2/с 118/мин');
    // Прежняя строка подписей со значениями ушла целиком: два способа показать
    // одно число человек сверяет глазами дважды.
    expect(wrapper.find('.stats-summary').exists()).toBe(false);
  });

  it('в шапке остались заголовок и кнопка обновления', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    const header = wrapper.get('.management-header');
    expect(header.get('.management-title').text()).toBe('Мониторинг запросов');
    expect(header.find('.refresh-stub').exists(), 'кнопка обновления стоит в шапке раздела').toBe(true);
    expect(header.findAll('[data-testid="monitoring-kpi"]')).toHaveLength(0);
  });

  it('высота шапки эталонная, с разделителем под ней', () => {
    // Замок против возврата к шапке без своей высоты: показатели в строку
    // разгоняли её до двух рядов, и разделы управления расходились на вид.
    const header = STYLE.slice(STYLE.indexOf('.management-header {'));
    const rules = header.slice(0, header.indexOf('}'));
    expect(rules).toContain('height: 50px');
    expect(rules).toContain('padding: 0 20px');
    expect(rules).toContain('border-bottom: 1px solid var(--border)');
  });

  it('обновление из шапки перечитывает журнал и показатели по одному разу', async () => {
    const { wrapper } = await mountView();
    await flushPromises();
    apiRequest.mockClear();
    const before = journalCalls().length;

    await wrapper.get('.management-header .refresh-stub').trigger('click');
    await flushPromises();

    expect(journalCalls().length - before, 'список перечитан').toBe(1);
    expect(callsTo('/request-logs/stats'), 'показатели читает шапка, а не вкладка следом за ней').toBe(1);
    expect(callsTo('/request-logs/realtime')).toBe(1);
  });

  it('на вкладке аналитики обновление читает историю, а не журнал', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    await wrapper.findAll('.rv-tab').at(1).trigger('click');
    await flushPromises();
    apiRequest.mockClear();
    const before = journalCalls().length;

    await wrapper.get('.management-header .refresh-stub').trigger('click');
    await flushPromises();

    expect(callsTo('/request-logs/history'), 'обновляется открытая вкладка').toBe(1);
    expect(journalCalls().length - before, 'скрытый журнал не дёргается').toBe(0);
  });
});
