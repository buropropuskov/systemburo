import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Управляемые промисы getInsights: фабрика vi.mock поднимается над импортами,
// поэтому очередь резолверов выносим в hoisted.
const { state } = vi.hoisted(() => ({ state: { deferred: [] } }));

vi.mock('@/api/statistics.js', () => ({
  getInsights: vi.fn(() => new Promise((resolve) => { state.deferred.push(resolve); })),
}));

import { getInsights } from '@/api/statistics.js';
import InsightsTab from '../InsightsTab.vue';

const FULL = {
  peak_hours: [
    {
      metric: 'car_entries_count',
      label: 'Въезды машин',
      unit: 'въезд',
      peak_hour: 14,
      peak_value: 9,
      hourly: [{ hour: 8, value: 2 }, { hour: 14, value: 9 }],
    },
  ],
  comparisons: [
    { metric: 'applications_count', label: 'Заявки', current: 34, previous: 20, delta_pct: 70, direction: 'up' },
    { metric: 'car_entries_count', label: 'Въезды машин', current: 9, previous: 12, delta_pct: -25, direction: 'down' },
  ],
  top_places: [{ metric: 'car_entries_count', label: 'Дебаркадер №1', value: 9 }],
  top_orgs: [{ metric: 'applications_count', label: 'ООО Ромашка', value: 20 }],
  trends: [{ metric: 'applications_count', label: 'Заявки', direction: 'up', series: [1, 2, 3, 5] }],
};

function mountTab(props = { from: '2026-06-01', to: '2026-06-07' }) {
  return mount(InsightsTab, {
    props,
    global: { stubs: { ReportChart: true } },
  });
}

describe('InsightsTab', () => {
  it('рендерит все блоки инсайтов из ответа API', async () => {
    state.deferred.length = 0;
    getInsights.mockClear();
    const wrapper = mountTab();
    await nextTick();

    state.deferred[0](FULL);
    await flushPromises();

    // Сравнение с прошлым периодом
    const cmps = wrapper.findAll('.insights__cmp');
    expect(cmps).toHaveLength(2);
    expect(cmps[0].find('.insights__cmp-current').text()).toBe('34');
    expect(cmps[0].find('.insights__delta').text()).toContain('+70%');
    const down = cmps[1].find('.insights__delta');
    expect(down.classes()).toContain('insights__delta--down');
    expect(down.text()).toContain('-25%');

    // Пик по часам -> ReportChart + бейдж пика
    expect(wrapper.findAll('.insights__peak')).toHaveLength(1);
    expect(wrapper.find('.insights__peak-badge').text()).toContain('14:00');

    // Топы
    expect(wrapper.text()).toContain('Дебаркадер №1');
    expect(wrapper.text()).toContain('ООО Ромашка');

    // Тренды
    expect(wrapper.find('.insights__trend-dir').text()).toContain('рост');
  });

  it('пустой ответ показывает плейсхолдер, а не блоки', async () => {
    state.deferred.length = 0;
    getInsights.mockClear();
    const wrapper = mountTab();
    await nextTick();

    state.deferred[0]({ peak_hours: [], comparisons: [], top_places: [], top_orgs: [], trends: [] });
    await flushPromises();

    expect(wrapper.find('.insights__cmp').exists()).toBe(false);
    expect(wrapper.find('.insights__state').text()).toContain('нет данных');
  });

  it('ошибка загрузки показывает сообщение и кнопку повтора', async () => {
    getInsights.mockImplementationOnce(() => Promise.reject(new Error('Network error')));
    const wrapper = mountTab();
    await flushPromises();

    const err = wrapper.find('.insights__state--error');
    expect(err.exists()).toBe(true);
    expect(err.text()).toContain('Network error');
    expect(err.find('button').text()).toContain('Повторить');
    expect(wrapper.find('.insights__cmp').exists()).toBe(false);
  });

  it('при быстрой смене периода рисует данные последнего запроса, медленный предыдущий игнорирует', async () => {
    state.deferred.length = 0;
    getInsights.mockClear();
    const wrapper = mountTab();
    await nextTick(); // onMounted -> load (seq 1, deferred)

    // Смена периода -> watch -> второй load (seq 2).
    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();

    expect(state.deferred).toHaveLength(2);

    const mk = (current) => ({
      peak_hours: [], top_places: [], top_orgs: [], trends: [],
      comparisons: [{ metric: 'applications_count', label: 'Заявки', current, previous: 0, delta_pct: 100, direction: 'up' }],
    });

    // Последний запрос (seq 2) приходит ПЕРВЫМ, устаревший seq 1 — позже.
    state.deferred[1](mk(222));
    await flushPromises();
    state.deferred[0](mk(111));
    await flushPromises();

    expect(wrapper.find('.insights__cmp-current').text()).toBe('222'); // не 111
  });
});
