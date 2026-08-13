import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// На телефоне живая лента не прокручивается сама (#1097 волна 5) и растёт по
// содержимому - при полном лимите 15 записей блок распирает страницу (замер
// на стенде: 1080px на ленту при вьюпорте 390). Проверяем, что подгрузка сама
// просит у бэка меньше записей на узком экране, а не только красиво обрезается
// в разметке (та резалась бы визуально, но не размером DOM/сети).
const { state } = vi.hoisted(() => ({
  state: { passages: { people: [], cars: [] }, calls: [] },
}));

vi.mock('@/api/statistics.js', () => ({
  getSummary: () => Promise.resolve({}),
  getTimeline: () => new Promise(() => {}),
  getInsights: () => Promise.resolve({}),
  getOnlinePeaks: () => Promise.resolve([]),
  getOnlineUsers: () => new Promise(() => {}),
  getRecentPassages: (limit) => {
    state.calls.push(limit);
    return Promise.resolve(state.passages);
  },
}));

import StatisticsDashboard from '../StatisticsDashboard.vue';

const origMatchMedia = window.matchMedia;

/** Тот же приём, что в ApplicationAttachmentDetailMobile.spec.js. */
function mockNarrowViewport(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const mountDashboard = () => mount(StatisticsDashboard, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: {
    stubs: {
      AnalyticsAreaChart: true, AnalyticsBarChart: true, AnalyticsDonutChart: true,
      RefreshButton: true, OnlineUsersModal: true, PushAdoptionSummary: true,
    },
  },
});

beforeEach(() => {
  state.calls.length = 0;
  state.passages = { people: [], cars: [] };
});

afterEach(() => {
  window.matchMedia = origMatchMedia;
});

describe('StatisticsDashboard — лимит живой ленты по ширине экрана (#1097 w7)', () => {
  it('на широком экране запрашивает 15 записей', async () => {
    mockNarrowViewport(false);
    mountDashboard();
    await flushPromises();
    expect(state.calls).toEqual([15]);
  });

  it('на узком экране запрашивает 5 записей вместо 15', async () => {
    mockNarrowViewport(true);
    mountDashboard();
    await flushPromises();
    expect(state.calls).toEqual([5]);
  });
});

/** N записей с уникальным ключом (feedRowKey = created_at|subject|action_type). */
function makeRows(n, prefix) {
  return Array.from({ length: n }, (_, i) => ({
    action_type: i % 2 === 0 ? 'entry' : 'exit',
    created_at: `2026-06-07T10:${String(i).padStart(2, '0')}:00Z`,
    subject: `${prefix}-${i}`,
    mark: '',
    organization: 'Ромашка',
    place: 'КПП-1',
  }));
}

describe('StatisticsDashboard — раскрытие ленты по тапу (#1097 w8)', () => {
  it('на узком экране при >=5 записях показывает "Показать ещё" у каждой ленты отдельно', async () => {
    mockNarrowViewport(true);
    state.passages = { people: makeRows(20, 'Иванов'), cars: makeRows(20, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    const feeds = wrapper.findAll('.dashboard__feed');
    expect(feeds).toHaveLength(2);
    expect(feeds[0].find('.dashboard__feed-more').exists()).toBe(true);
    expect(feeds[1].find('.dashboard__feed-more').exists()).toBe(true);
  });

  it('раскрытие ленты людей поднимает только её лимит, машины остаются свёрнуты', async () => {
    mockNarrowViewport(true);
    state.passages = { people: makeRows(20, 'Иванов'), cars: makeRows(20, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    const feeds = wrapper.findAll('.dashboard__feed');
    await feeds[0].find('.dashboard__feed-more').trigger('click');
    await flushPromises();

    // Запрос ушёл с большим из двух активных потолков (15), а не свёрнутыми 5.
    expect(state.calls.at(-1)).toBe(15);

    const feedsAfter = wrapper.findAll('.dashboard__feed');
    const peopleRows = feedsAfter[0].findAll('.dashboard__feed-row:not(.dashboard__feed-row--skeleton)');
    const carsRows = feedsAfter[1].findAll('.dashboard__feed-row:not(.dashboard__feed-row--skeleton)');
    expect(peopleRows.length).toBe(15);
    expect(carsRows.length).toBe(5);

    // Кнопка раскрытой ленты пропадает, у свёрнутой остаётся.
    expect(feedsAfter[0].find('.dashboard__feed-more').exists()).toBe(false);
    expect(feedsAfter[1].find('.dashboard__feed-more').exists()).toBe(true);
  });

  it('на широком экране кнопка "Показать ещё" не рендерится вовсе', async () => {
    mockNarrowViewport(false);
    state.passages = { people: makeRows(20, 'Иванов'), cars: makeRows(20, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.find('.dashboard__feed-more').exists()).toBe(false);
  });
});
