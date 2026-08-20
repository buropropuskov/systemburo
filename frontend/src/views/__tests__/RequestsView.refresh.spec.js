import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Показ загрузки в разделе мониторинга (#2125). Раньше любая загрузка списка
// поднимала пелену на весь раздел - вместе с шапкой, показателями, вкладками и
// графиком, причём и на самообновлении ленты раз в десять секунд. Проверяем
// наблюдаемое: что накрыто, что осталось на экране и где видно обновление.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import { apiRequest, apiRequestRaw } from '@/api/client';
import { JOURNAL_REFRESH_MS } from '@/utils/requestLogsLive';
import { logsPage, mountView, resetApiMocks, unmountAll } from './helpers/requestsView';

const LOG = { id: 7, url: '/api/applications', method: 'GET', response_status: 200, duration_us: 900 };

/** Ответ истории с узнаваемым числом запросов в сводке. */
function historyResponse(total) {
  return { ok: true, json: () => Promise.resolve({ totals: { requests: total }, daily: [], coverage: null }) };
}

/** Подвешивает следующий ответ списка: возвращает отпускающую его функцию. */
function holdNextJournalCall(page = logsPage([LOG])) {
  let release;
  apiRequestRaw.mockImplementationOnce(() => new Promise(resolve => {
    release = () => resolve(page);
  }));
  return () => release();
}

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
  apiRequestRaw.mockResolvedValue(logsPage([LOG]));
});

describe('RequestsView, показ загрузки журнала', () => {
  it('первая загрузка показывает лоадер вместо строк, а не пелену поверх раздела', async () => {
    const release = holdNextJournalCall();
    const { wrapper } = await mountView();
    await flushPromises();

    expect(wrapper.find('.table-loading').exists(), 'пока строк нет, на их месте лоадер').toBe(true);
    expect(wrapper.find('.refresh-overlay').exists(), 'плёнке обновления накрывать нечего').toBe(false);
    expect(wrapper.find('.management-header').isVisible(), 'шапка раздела остаётся на экране').toBe(true);
    expect(wrapper.find('.rv-stats').isVisible(), 'показатели остаются на экране').toBe(true);

    release();
    await flushPromises();

    expect(wrapper.findAll('.table-row')).toHaveLength(1);
    expect(wrapper.find('.table-loading').exists(), 'дождавшись строк, лоадер уходит').toBe(false);
  });

  it('обновление по кнопке кроет плёнкой только таблицу, строки остаются на месте', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    const release = holdNextJournalCall();
    await wrapper.get('.refresh-stub').trigger('click');
    await flushPromises();

    expect(wrapper.findAll('.refresh-overlay'), 'плёнка в разделе одна').toHaveLength(1);
    expect(wrapper.find('.table-container .refresh-overlay').exists(), 'и лежит внутри таблицы').toBe(true);
    expect(wrapper.findAll('.table-row'), 'строки не подменяются лоадером').toHaveLength(1);
    expect(wrapper.find('.refresh-stub').classes(), 'кнопка показывает обновление').toContain('is-loading');

    release();
    await flushPromises();

    expect(wrapper.find('.refresh-overlay').exists(), 'ответ пришёл - плёнка ушла').toBe(false);
    expect(wrapper.find('.refresh-stub').classes()).not.toContain('is-loading');
  });

  it('самообновление ленты идёт молча: ни плёнки, ни занятой кнопки', async () => {
    vi.useFakeTimers();
    try {
      const { wrapper } = await mountView();
      await flushPromises();

      const release = holdNextJournalCall();
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();

      expect(wrapper.find('.refresh-overlay').exists(), 'лента тикает молча').toBe(false);
      expect(wrapper.find('.table-loading').exists(), 'и строки не подменяет').toBe(false);
      // Кнопка игнорирует клик, пока держит loading: с поднятым признаком фоновый
      // тик глотал бы обновление, которое человек только что запросил.
      expect(wrapper.find('.refresh-stub').classes(), 'кнопка остаётся свободной').not.toContain('is-loading');

      release();
      await flushPromises();
    } finally {
      vi.useRealTimers();
    }
  });

  it('смена отбора после тика ленты плёнку возвращает', async () => {
    vi.useFakeTimers();
    try {
      const { wrapper } = await mountView();
      await flushPromises();

      const releaseTick = holdNextJournalCall();
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();
      releaseTick();
      await flushPromises();

      const release = holdNextJournalCall();
      await wrapper.get('.refresh-stub').trigger('click');
      await flushPromises();

      expect(wrapper.find('.refresh-overlay').exists(), 'тихий тик не выключает плёнку насовсем').toBe(true);
      expect(wrapper.find('.refresh-stub').classes(), 'ручное обновление кнопку занимает').toContain('is-loading');
      release();
      await flushPromises();
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('RequestsView, показ загрузки аналитики', () => {
  /** Подвешивает ответ истории, остальные разделы шапки отвечают как обычно. */
  function holdHistory() {
    let release;
    apiRequest.mockImplementation(url => {
      if (String(url).startsWith('/request-logs/history')) {
        return new Promise(resolve => {
          release = () => resolve({ ok: true, json: () => Promise.resolve({}) });
        });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) });
    });
    return () => release();
  }

  it('первый показ вкладки кроет пустые панели наглухо, отбор периода остаётся доступен', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    const release = holdHistory();
    await wrapper.findAll('.rv-tab')[1].trigger('click');
    await flushPromises();

    const overlay = wrapper.find('.analytics-tab .refresh-overlay');
    expect(overlay.exists(), 'на месте панелей - плёнка загрузки').toBe(true);
    expect(overlay.classes(), 'непрозрачная: под ней панели врут «нет данных»').toContain('refresh-overlay--solid');
    expect(wrapper.find('.analytics-toolbar').isVisible(), 'отбор периода не гаснет').toBe(true);

    release();
    await flushPromises();
    expect(wrapper.find('.analytics-tab .refresh-overlay').exists(), 'данные пришли - плёнка ушла').toBe(false);
  });

  it('запоздавший ответ прошлого периода не затирает свежие числа', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    // Первый показ вкладки читает историю за всё время - ответ подвешен.
    let releaseSlow;
    apiRequest.mockImplementation(url => {
      if (String(url).startsWith('/request-logs/history')) {
        return new Promise(resolve => {
          releaseSlow = () => resolve(historyResponse(111));
        });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) });
    });
    await wrapper.findAll('.rv-tab')[1].trigger('click');
    await flushPromises();

    // Человек, не дождавшись, просит другой период - тулбар над плёнкой доступен.
    apiRequest.mockImplementation(url => (String(url).startsWith('/request-logs/history')
      ? Promise.resolve(historyResponse(222))
      : Promise.resolve({ ok: true, json: () => Promise.resolve([]) })));
    await wrapper.get('.analytics-toolbar .lk-button').trigger('click');
    await flushPromises();

    releaseSlow();
    await flushPromises();

    expect(wrapper.find('.analytics-kpis').text(), 'на экране числа последнего отбора').toContain('222');
    expect(wrapper.find('.analytics-kpis').text(), 'запоздавший ответ на экран не попал').not.toContain('111');
    expect(wrapper.find('.analytics-tab .refresh-overlay').exists(), 'и не зажигает загрузку заново').toBe(false);
  });

  it('повторное чтение показывает полупрозрачную плёнку поверх прежних чисел', async () => {
    const { wrapper } = await mountView();
    await flushPromises();
    await wrapper.findAll('.rv-tab')[1].trigger('click');
    await flushPromises();

    const release = holdHistory();
    await wrapper.get('.refresh-stub').trigger('click');
    await flushPromises();

    const overlay = wrapper.find('.analytics-tab .refresh-overlay');
    expect(overlay.exists()).toBe(true);
    expect(overlay.classes(), 'прежние числа остаются читаемыми').not.toContain('refresh-overlay--solid');

    release();
    await flushPromises();
  });
});

describe('RequestsView, отказ сети в тихом тике', () => {
  it('сообщается один раз за серию, а не каждые десять секунд', async () => {
    vi.useFakeTimers();
    try {
      const { wrapper } = await mountView();
      await flushPromises();
      notify.mockClear();

      apiRequestRaw.mockRejectedValue(new Error('offline'));
      for (let i = 0; i < 3; i++) {
        vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
        await flushPromises();
      }

      expect(notify, 'три тика без сети - одно сообщение').toHaveBeenCalledTimes(1);
      expect(wrapper.find('.refresh-overlay').exists(), 'и по-прежнему без плёнки').toBe(false);

      // Ручное обновление - ответ на действие человека, о нём сообщают всегда.
      await wrapper.get('.refresh-stub').trigger('click');
      await flushPromises();
      expect(notify, 'на свой клик человек получает ответ').toHaveBeenCalledTimes(2);

      // Сеть вернулась - отметка снимается, следующий обрыв снова слышно.
      apiRequestRaw.mockResolvedValue(logsPage([LOG]));
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();
      apiRequestRaw.mockRejectedValue(new Error('offline'));
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();
      expect(notify, 'новая серия отказов сообщается заново').toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('RequestsView, пелена на весь раздел не возвращается', () => {
  it('в стилях оболочки нет растянутого во весь раздел слоя', () => {
    const source = readFileSync(resolve(__dirname, '../RequestsView.vue'), 'utf8');
    const styles = source.slice(source.indexOf('<style'));
    const stretched = [...styles.matchAll(/\{[^{}]*\}/g)]
      .map(match => match[0])
      .filter(rule => /position:\s*absolute/.test(rule))
      .filter(rule => /inset:\s*0/.test(rule) || (/top:\s*0/.test(rule) && /bottom:\s*0/.test(rule)));

    expect(stretched, 'загрузку показывают вкладки у себя, а не оболочка поверх всего').toEqual([]);
  });
});
