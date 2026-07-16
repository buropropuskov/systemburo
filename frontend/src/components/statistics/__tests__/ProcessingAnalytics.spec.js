import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Фабрика vi.mock поднимается над импортами — управляемое состояние в hoisted.
const { state } = vi.hoisted(() => ({
  state: { summary: {}, reject: false, deferred: [] },
}));

vi.mock('@/api/statistics.js', () => ({
  getProcessingSummary: () => {
    if (state.reject) return Promise.reject(new Error('boom'));
    if (state.hang) return new Promise((resolve) => { state.deferred.push(resolve); });
    return Promise.resolve(state.summary);
  },
}));

import ProcessingAnalytics from '../ProcessingAnalytics.vue';
import AnalyticsBarChart from '../AnalyticsBarChart.vue';

const mountTab = () => mount(ProcessingAnalytics, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: { stubs: { AnalyticsBarChart: true } },
});

const fullSummary = () => ({
  from: '2026-06-01',
  to: '2026-06-07',
  total_applications: 42,
  stages: [
    {
      key: 'approval_time',
      label: 'Время согласования',
      samples: 40,
      avg: 8100, // 2 ч 15 мин
      p90: 18000, // 5 ч
      prev_avg: 9000,
      trend: { delta_pct: -10, direction: 'down', sentiment: 'good' },
    },
    {
      key: 'acceptance_time',
      label: 'Время принятия в работу',
      samples: 30,
      avg: 3600, // 1 ч
      p90: 7200,
      prev_avg: 3000,
      trend: { delta_pct: 20, direction: 'up', sentiment: 'bad' },
    },
    {
      key: 'completion_time',
      label: 'Время до завершения',
      samples: 0,
      avg: null, // этап никто не прошёл
      p90: null,
      prev_avg: null,
    },
  ],
  quality: [
    {
      key: 'refusal_rate',
      label: 'Доля отказов',
      unit: '%',
      value: 15,
      prev_value: 20,
      trend: { delta_pct: -25, direction: 'down', sentiment: 'good' },
    },
    {
      key: 'avg_forwards',
      label: 'Среднее число пересылок',
      unit: 'раз/заявку',
      value: 1.3,
      prev_value: 1.1,
      trend: { delta_pct: 18.2, direction: 'up', sentiment: 'bad' },
    },
  ],
  slow_approvers: [
    { name: 'Иванов И.И.', avg_response_time: 14400, votes_count: 12 },
    { name: 'Петров П.П.', avg_response_time: null, votes_count: 3 },
  ],
  by_organization: [
    { label: 'ООО Ромашка', avg_processing_time: 10800, applications_count: 20 },
  ],
});

beforeEach(() => {
  state.summary = {};
  state.reject = false;
  state.hang = false;
  state.deferred = [];
});

describe('ProcessingAnalytics — KPI этапов', () => {
  it('рисует длительность этапа человекочитаемо, а не сырые секунды', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Время согласования');
    expect(text).toContain('2 ч 15 мин'); // 8100 с
    expect(text).toContain('p90: 5 ч'); // 18000 с
    expect(text).not.toContain('8100');
  });

  it('этап без выборки показывает прочерк, а не «0 мин»', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const tile = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время до завершения'));
    expect(tile).toBeTruthy();
    expect(tile.find('.proc__tile-val').text()).toBe('—');
    expect(tile.text()).not.toContain('0 мин');
  });

  it('дельту красит по тональности, а не по направлению (стрелка вниз может быть зелёной)', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    // Время согласования упало (direction=down) — это улучшение (sentiment=good) -> зелёный.
    const approval = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время согласования'));
    const delta = approval.find('.proc__delta');
    expect(delta.classes()).toContain('proc__delta--good');
    expect(delta.classes()).not.toContain('proc__delta--down');
    expect(delta.text()).toContain('-10%');

    // Принятие выросло (up) — ухудшение (bad) -> красный.
    const acceptance = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время принятия'));
    expect(acceptance.find('.proc__delta').classes()).toContain('proc__delta--bad');
  });
});

describe('ProcessingAnalytics — качество', () => {
  it('долю отказов показывает процентом, число пересылок — с единицей', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const refusal = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Доля отказов'));
    expect(refusal.find('.proc__tile-val').text()).toBe('15%');

    const forwards = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Среднее число пересылок'));
    expect(forwards.find('.proc__tile-val').text()).toBe('1,3'); // ru-RU десятичная запятая
    expect(forwards.text()).toContain('раз/заявку');
  });
});

describe('ProcessingAnalytics — узкие места', () => {
  it('передаёт в bar-chart средние времена этапов как duration с null-разрывами', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const chart = wrapper.findComponent(AnalyticsBarChart);
    expect(chart.props('valueType')).toBe('duration');
    const data = chart.props('data');
    expect(data).toEqual([
      { label: 'Время согласования', value: 8100 },
      { label: 'Время принятия в работу', value: 3600 },
      { label: 'Время до завершения', value: null }, // разрыв, не 0
    ]);
  });
});

describe('ProcessingAnalytics — таблицы', () => {
  it('согласующий без ответов показывает прочерк времени, но реальную нагрузку', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Иванов И.И.');
    expect(text).toContain('4 ч'); // 14400 с
    // Петров без времени реакции — прочерк, нагрузка 3.
    const rows = wrapper.findAll('.proc__table')[0].findAll('tbody tr');
    const petrov = rows.find((r) => r.text().includes('Петров'));
    expect(petrov.findAll('.proc__num')[0].text()).toBe('—');
    expect(petrov.findAll('.proc__num')[1].text()).toBe('3');
  });

  it('разбивку по организациям показывает временем обработки и числом заявок', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('ООО Ромашка');
    expect(text).toContain('3 ч'); // 10800 с
  });
});

describe('ProcessingAnalytics — крайние состояния', () => {
  it('пустой период показывает заглушку, а не нулевые метрики', async () => {
    state.summary = { from: '2026-06-01', to: '2026-06-07', total_applications: 0, stages: [], quality: [], slow_approvers: [], by_organization: [] };
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('заявок не подавали');
    expect(wrapper.find('.proc__tiles').exists()).toBe(false);
  });

  it('ошибка загрузки показывает сообщение', async () => {
    state.reject = true;
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('.proc__state--error').exists()).toBe(true);
    expect(wrapper.text()).toContain('boom');
  });

  it('устаревший ответ не затирает актуальный (seq-guard при смене периода)', async () => {
    state.hang = true;
    const wrapper = mountTab();
    await nextTick(); // onMounted -> loadSummary (seq 1, зависший)

    // Смена периода -> второй loadSummary (seq 2).
    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();
    expect(state.deferred).toHaveLength(2);

    // Актуальный seq 2 приходит первым, устаревший seq 1 — позже с другими данными.
    state.deferred[1](fullSummary());
    await flushPromises();
    state.deferred[0]({ total_applications: 999, stages: [], quality: [], slow_approvers: [], by_organization: [] });
    await flushPromises();

    // На экране — данные seq 2 (42 заявки), а не устаревшие seq 1 (999).
    expect(wrapper.text()).toContain('42 заявки за период');
    expect(wrapper.text()).not.toContain('999');
  });
});
