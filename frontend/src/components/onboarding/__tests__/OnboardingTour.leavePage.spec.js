import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';

/**
 * Уход со страницы во время тура.
 *
 * Тур ведёт по страницам сам и ждёт навигацию только тогда, когда сам её и
 * затеял. Если человек ушёл своим ходом - нажал пункт меню, ссылку или «Назад»
 * браузера, - шаги остаются от прежней страницы: поповер висит поверх чужого
 * экрана и подсвечивает то, чего здесь нет. Поймано руками на стенде: уход в
 * личный кабинет с четвёртого шага тура охранника оставлял шаг «Поиск по
 * системе» висеть поверх кабинета.
 */

const mocks = vi.hoisted(() => ({
  driver: { config: null, activeIndex: 0 },
  route: { path: '/news' },
  afterEachHandlers: [],
  router: {
    push: vi.fn(() => Promise.resolve()),
    afterEach: (fn) => { mocks.afterEachHandlers.push(fn); return () => {}; },
  },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.driver.config = config;
    return {
      getConfig: () => mocks.driver.config,
      setConfig: (next) => { mocks.driver.config = next; },
      getActiveIndex: () => mocks.driver.activeIndex,
      moveNext: () => {},
      movePrevious: () => {},
      moveTo: () => {},
      drive: (i) => { mocks.driver.activeIndex = i; },
      destroy: () => { config.onDestroyed?.(); },
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
  { id: 'a', route: '/news', element: '[data-testid="ob-a"]', title: 'Первый', description: 'x' },
  { id: 'b', route: '/news', element: '[data-testid="ob-b"]', title: 'Второй', description: 'y' },
  { id: 'c', route: '/center', element: '[data-testid="ob-c"]', title: 'Третий', description: 'z' },
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

/** Сообщить хосту о состоявшейся навигации - тем же путём, что и роутер. */
async function navigateTo(path) {
  mocks.route.path = path;
  mocks.afterEachHandlers.forEach((fn) => fn({ path }));
  await flushPromises();
}

describe('OnboardingTour - уход со страницы во время тура', () => {
  beforeEach(async () => {
    mocks.driver.config = null;
    mocks.driver.activeIndex = 0;
    mocks.afterEachHandlers = [];
    mocks.route.path = '/news';
    store.isActive = false;
    store.currentIndex = 0;
    store.pendingSegment = false;
    store.skippedIndexes = [];
    document.body.innerHTML = '<div data-testid="ob-a"></div><div data-testid="ob-b"></div>';
    wrapper = mount(OnboardingTour);
    store.isActive = true;
    await flushPromises();
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('человек ушёл на чужую страницу - тур завершается, а не висит поверх неё', async () => {
    expect(store.isActive).toBe(true);

    await navigateTo('/personal-cabinet');

    expect(store.isActive).toBe(false);
  });

  it('переход, затеянный самим туром, обучение не прерывает', async () => {
    store.pendingSegment = true;
    store.currentIndex = 2;

    await navigateTo('/center');

    expect(store.isActive).toBe(true);
  });

  it('навигация на ту же страницу тур не трогает', async () => {
    await navigateTo('/news');

    expect(store.isActive).toBe(true);
  });
});
