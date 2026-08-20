import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';

/**
 * Прыжок по списку шагов в поповере на шаг ДРУГОГО сегмента той же страницы.
 *
 * Тур возвращается на одну и ту же страницу несколькими сегментами: у охранника
 * «Доступные мне» идут и до таблицы поста, и после неё - финалом, у заявителя на
 * «Обзоре» так же начинается тур и заканчивается. Прежде хост решал по одному
 * только route: путь совпал - значит шаг в текущем сегменте, и звал obGoTo,
 * который умеет ходить лишь внутри поднятого сегмента. Прыжок молча не
 * происходил - в туре заявителя финал из списка был недостижим вовсе.
 */

const mocks = vi.hoisted(() => ({
  driver: { config: null, activeIndex: 0, moves: [], destroyed: 0, created: 0 },
  route: { path: '/news' },
  router: { push: vi.fn(() => Promise.resolve()), afterEach: () => () => {} },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.driver.config = config;
    mocks.driver.created += 1;
    return {
      getConfig: () => mocks.driver.config,
      setConfig: (next) => { mocks.driver.config = next; },
      getActiveIndex: () => mocks.driver.activeIndex,
      moveNext: () => mocks.driver.moves.push('next'),
      movePrevious: () => mocks.driver.moves.push('prev'),
      moveTo: (i) => mocks.driver.moves.push(`to:${i}`),
      drive: (i) => { mocks.driver.activeIndex = i; },
      // Настоящий driver.js синхронно зовёт onDestroyed - без этого мимо теста
      // прошло бы то, что снятие инстанса принимается за конец обучения.
      destroy: () => { mocks.driver.destroyed += 1; config.onDestroyed?.(); },
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

// Три сегмента: «Обзор» -> таблица поста -> снова «Обзор» с финалом. Финал делит
// route с первым сегментом, но лежит за границей другой страницы.
const steps = [
  { id: 'intro', route: '/news', element: '[data-testid="ob-intro"]', title: 'Начало', description: 'Вступление' },
  { id: 'nav', route: '/news', element: '[data-testid="ob-nav"]', title: 'Навигация', description: 'Меню' },
  { id: 'table', route: '/table/kpp', element: '[data-testid="ob-table"]', title: 'Таблица поста', description: 'Список' },
  { id: 'finish', route: '/news', element: null, title: 'Готово!', description: 'Финал', celebrate: true },
];

const store = reactive({
  steps,
  isActive: false,
  currentIndex: 0,
  isManual: true,
  pendingSegment: false,
  skippedIndexes: [],
  markSkipped(index) {
    if (!this.skippedIndexes.includes(index)) this.skippedIndexes.push(index);
  },
  statusLoaded: true,
  canShowTour: false,
  currentStep: null,
  revealOpen: null,
  setRevealOpen(v) { this.revealOpen = v || null; },
  setDemoAttachment: vi.fn(),
  setIndex(i) { this.currentIndex = i; },
  advanceSegment: vi.fn(),
  retreatSegment: vi.fn(function retreat(i) { this.currentIndex = i; this.pendingSegment = true; }),
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

async function startTour(startIndex = 0) {
  wrapper = mount(OnboardingTour);
  store.currentIndex = startIndex;
  store.isActive = true;
  await flushPromises();
}

/** @returns {Array<string>} заголовки шагов, поднятых в driver сейчас */
function shownTitles() {
  return mocks.driver.config.steps.map((s) => s.popover.title);
}

/**
 * Отрисовать поповер так, как это делает driver.js, и вернуть пункты списка
 * шагов. Прыжок проверяем через сам список - тот путь, которым ходит человек.
 *
 * @returns {Array<HTMLElement>}
 */
function renderStepList() {
  const wrapper = document.createElement('div');
  const title = document.createElement('div');
  const description = document.createElement('div');
  const footer = document.createElement('div');
  const footerButtons = document.createElement('div');
  footer.appendChild(footerButtons);
  wrapper.append(title, description, footer);
  document.body.appendChild(wrapper);
  mocks.driver.config.onPopoverRender({ wrapper, title, description, footer, footerButtons });
  return [...wrapper.querySelectorAll('.ob-popover__steps-item')];
}

describe('OnboardingTour - прыжок из списка шагов', () => {
  beforeEach(() => {
    mocks.driver.config = null;
    mocks.driver.activeIndex = 0;
    mocks.driver.moves = [];
    mocks.driver.destroyed = 0;
    mocks.driver.created = 0;
    mocks.router.push.mockClear();
    mocks.route.path = '/news';
    store.isActive = false;
    store.currentIndex = 0;
    store.skippedIndexes = [];
    store.pendingSegment = false;
    document.body.innerHTML = '<div data-testid="ob-intro"></div><div data-testid="ob-nav"></div>';
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('на старте поднят только первый сегмент', async () => {
    await startTour();
    expect(shownTitles()).toEqual(['Начало', 'Навигация']);
  });

  it('шаг соседнего сегмента с тем же route поднимает свой сегмент', async () => {
    await startTour();
    const createdBefore = mocks.driver.created;

    renderStepList()[3].click();
    await flushPromises();

    expect(store.currentIndex).toBe(3);
    // Сегмент пересобран вокруг финала, а не остался прежним.
    expect(shownTitles()).toEqual(['Готово!']);
    // Тур продолжается: снятие прежнего инстанса не должно читаться как финиш.
    expect(store.isActive).toBe(true);
    expect(mocks.driver.created).toBe(createdBefore + 1);
    // Навигации нет: страница та же, router.push тут только сломал бы тур.
    expect(mocks.router.push).not.toHaveBeenCalled();
  });

  it('прыжок внутри своего сегмента идёт как прежде, без пересборки', async () => {
    await startTour();
    const createdBefore = mocks.driver.created;

    renderStepList()[1].click();
    await flushPromises();

    expect(mocks.driver.moves).toContain('to:1');
    expect(mocks.driver.created).toBe(createdBefore);
  });

  it('шаг другой страницы по-прежнему уходит через навигацию', async () => {
    await startTour();

    renderStepList()[2].click();
    await flushPromises();

    expect(store.retreatSegment).toHaveBeenCalledWith(2);
    expect(mocks.router.push).toHaveBeenCalledWith('/table/kpp');
  });
});
