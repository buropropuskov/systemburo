import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Волна 9 (#1097): на телефоне лента получила свою вертикальную прокрутку
// (max-height 360px, как на десктопе, помечена data-scroll-own для гейта
// мобильных инвариантов) - раздельный узкий лимит и кнопка «Показать ещё»
// больше не нужны, оба брейкпоинта запрашивают и показывают одно и то же.
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

describe('StatisticsDashboard — лимит живой ленты одинаков на обоих брейкпоинтах (#1097 w9)', () => {
  it('на широком экране запрашивает 15 записей', async () => {
    mockNarrowViewport(false);
    mountDashboard();
    await flushPromises();
    expect(state.calls).toEqual([15]);
  });

  it('на узком экране тоже запрашивает 15 записей', async () => {
    mockNarrowViewport(true);
    mountDashboard();
    await flushPromises();
    expect(state.calls).toEqual([15]);
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

describe('StatisticsDashboard — своя прокрутка ленты на узком экране (#1097 w9)', () => {
  it('на узком экране показывает все запрошенные записи без кнопки "Показать ещё"', async () => {
    mockNarrowViewport(true);
    state.passages = { people: makeRows(15, 'Иванов'), cars: makeRows(15, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    const feeds = wrapper.findAll('.dashboard__feed');
    expect(feeds).toHaveLength(2);
    expect(feeds[0].find('.dashboard__feed-more').exists()).toBe(false);
    expect(feeds[1].find('.dashboard__feed-more').exists()).toBe(false);

    const peopleRows = feeds[0].findAll('.dashboard__feed-row:not(.dashboard__feed-row--skeleton)');
    const carsRows = feeds[1].findAll('.dashboard__feed-row:not(.dashboard__feed-row--skeleton)');
    expect(peopleRows.length).toBe(15);
    expect(carsRows.length).toBe(15);
  });

  it('лента помечена data-scroll-own - её прокрутка законна для гейта мобильных инвариантов', async () => {
    mockNarrowViewport(true);
    state.passages = { people: makeRows(15, 'Иванов'), cars: makeRows(15, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    const lists = wrapper.findAll('.dashboard__feed-list');
    expect(lists).toHaveLength(2);
    for (const list of lists) {
      expect(list.attributes('data-scroll-own')).toBeDefined();
    }
  });

  it('на широком экране лента тоже без кнопки "Показать ещё"', async () => {
    mockNarrowViewport(false);
    state.passages = { people: makeRows(15, 'Иванов'), cars: makeRows(15, 'А000АА') };
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.find('.dashboard__feed-more').exists()).toBe(false);
  });
});
