import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Управляемые промисы getTimeline: фабрика vi.mock поднимается над импортами,
// поэтому состояние выносим в hoisted.
const { state } = vi.hoisted(() => ({ state: { deferred: [] } }));

vi.mock('@/api/statistics.js', () => ({
  getSummary: () => Promise.resolve({}),
  getRecentPassages: () => Promise.resolve({ people: [], cars: [] }),
  getTimeline: () => new Promise((resolve) => { state.deferred.push(resolve); }),
}));

import StatisticsDashboard from '../StatisticsDashboard.vue';
import RealTimeChart from '@/components/RealTimeChart.vue';

describe('StatisticsDashboard — гонка таймлайна', () => {
  it('при быстрой смене метрики рисует данные последнего запроса, медленный предыдущий игнорирует', async () => {
    state.deferred.length = 0;
    const wrapper = mount(StatisticsDashboard, {
      props: { from: '2026-06-01', to: '2026-06-07' },
      global: { stubs: { RealTimeChart: true, RefreshButton: true } },
    });
    await nextTick(); // onMounted -> loadTimeline (seq 1, deferred)

    // Быстрое переключение метрики -> второй loadTimeline (seq 2).
    await wrapper.findAll('.dashboard__seg-btn')[1].trigger('click');
    await nextTick();

    expect(state.deferred).toHaveLength(2);

    // Последний запрос (seq 2) приходит ПЕРВЫМ, устаревший seq 1 — позже.
    state.deferred[1]([{ date: '2026-06-02', count: 222 }]);
    await flushPromises();
    state.deferred[0]([{ date: '2026-06-01', count: 111 }]);
    await flushPromises();

    const chart = wrapper.findComponent(RealTimeChart);
    const data = chart.props('data');
    expect(data).toHaveLength(1);
    expect(data[0].count).toBe(222); // не 111 от устаревшего ответа
  });
});
