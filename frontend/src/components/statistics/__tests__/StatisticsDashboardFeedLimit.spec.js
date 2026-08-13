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
