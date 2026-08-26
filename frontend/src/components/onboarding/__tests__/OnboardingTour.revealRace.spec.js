import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';

/**
 * Гонка раскрытия при быстром «Далее».
 *
 * Шаг про колокольчик и следующий шаг про список уведомлений идут подряд, и
 * второй раскрывает панель сам (`reveal.open`). Подготовка следующего шага
 * ставит сигнал сразу по нажатию, а подсветка ТЕКУЩЕГО доезжает позже - и её
 * фоновый прогрев применял reveal колокольчика, у которого раскрытия нет,
 * затирая только что выставленный сигнал. Панель успевала открыться и через
 * четверть секунды гасла, шаг про неё выбрасывался как «цель не появилась», а
 * тур застревал на колокольчике (замечание владельца 21.08, воспроизведено на
 * стенде: reveal=notifications на 100мс, null на 200мс, панели нет на 400мс).
 */

const mocks = vi.hoisted(() => ({
  driver: { config: null, activeIndex: 0 },
  route: { path: '/news' },
  router: { push: vi.fn(() => Promise.resolve()), afterEach: () => () => {} },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.driver.config = config;
    return {
      getConfig: () => mocks.driver.config,
      setConfig: (next) => { mocks.driver.config = next; },
      getActiveIndex: () => mocks.driver.activeIndex,
      getActiveElement: () => null,
      moveNext: () => {},
      movePrevious: () => {},
      moveTo: () => {},
      drive: (i) => { mocks.driver.activeIndex = i; },
      destroy: () => {},
      refresh: () => {},
    };
  },
}));

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => mocks.router,
  createRouter: () => mocks.router,
  createWebHistory: () => ({}),
}));

vi.mock('@/composables/useOnboarding', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...actual,
    useOnboarding: () => ({
      ...actual.useOnboarding(),
      waitForElement: (selector) => Promise.resolve(document.querySelector(selector)),
    }),
  };
});

const steps = [
  { id: 'bell', route: '/news', element: '[data-testid="ob-bell"]', title: 'Уведомления', description: 'x',
    advanceWhen: '[data-testid="ob-panel"]' },
  { id: 'panel', route: '/news', element: '[data-testid="ob-panel"]', title: 'Список уведомлений', description: 'y',
    optional: true, reveal: { open: 'notifications' } },
  { id: 'search', route: '/news', element: '[data-testid="ob-search"]', title: 'Поиск', description: 'z' },
];

const store = reactive({
  steps,
  isActive: false,
  currentIndex: 0,
  isManual: true,
  pendingSegment: false,
  skippedIndexes: [],
  markSkipped(i) { if (!this.skippedIndexes.includes(i)) this.skippedIndexes.push(i); },
  statusLoaded: true,
  canShowTour: false,
  revealOpen: null,
  get currentStep() { return this.steps[this.currentIndex]; },
  setRevealOpen(v) { this.revealOpen = v || null; },
  setDemoAttachment: vi.fn(),
  setIndex(i) { this.currentIndex = i; },
  advanceSegment: vi.fn(),
  retreatSegment: vi.fn(),
  jumpToSegment: vi.fn(),
  clearPending: vi.fn(),
  markCompleted: vi.fn(),
  stop() { this.isActive = false; },
  ensureGatingContext: vi.fn().mockResolvedValue(),
  loadStatus: vi.fn().mockResolvedValue(),
  pickAutostartTour: () => null,
});

const ui = reactive({ tourForceExpand: false, sidebarHidden: false });
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => store }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ui }));

import OnboardingTour from '@/components/onboarding/OnboardingTour.vue';

let wrapper = null;

describe('OnboardingTour - фоновый прогрев и сигнал следующего шага', () => {
  beforeEach(async () => {
    mocks.driver.config = null;
    mocks.driver.activeIndex = 0;
    mocks.route.path = '/news';
    store.isActive = false;
    store.currentIndex = 0;
    store.revealOpen = null;
    store.skippedIndexes = [];
    document.body.innerHTML = '<div data-testid="ob-bell"></div><div data-testid="ob-search"></div>';
    wrapper = mount(OnboardingTour);
    store.isActive = true;
    await flushPromises();
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('подсветка текущего шага не гасит раскрытие, уже запрошенное для следующего', async () => {
    // подготовка шага про панель поставила сигнал
    store.setRevealOpen('notifications');
    // ...и только теперь доехала подсветка ПРЕДЫДУЩЕГО шага (колокольчика)
    mocks.driver.activeIndex = 0;
    mocks.driver.config.onHighlighted();
    await flushPromises();

    expect(store.revealOpen).toBe('notifications');
  });

  it('подсветка шага без раскрытия по-прежнему гасит чужой сигнал, если он не от следующего шага', async () => {
    store.setRevealOpen('search-panel');
    mocks.driver.activeIndex = 0;
    mocks.driver.config.onHighlighted();
    await flushPromises();

    expect(store.revealOpen).toBe(null);
  });

  it('подсветка самого шага с раскрытием держит его сигнал', async () => {
    store.currentIndex = 1;
    store.setRevealOpen('notifications');
    mocks.driver.activeIndex = 1;
    mocks.driver.config.onHighlighted();
    await flushPromises();

    expect(store.revealOpen).toBe('notifications');
  });
});
