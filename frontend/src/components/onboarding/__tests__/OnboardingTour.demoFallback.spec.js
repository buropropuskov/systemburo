import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';

/**
 * Шаг, у которого на пустой системе нет цели, ведёт себя по-разному в зависимости
 * от того, есть ли у него демо-скриншот (#1736):
 * - без скриншота - молча выбрасывается (шаг «при наличии»);
 * - со скриншотом - остаётся и показывается без подсветки, с картинкой вместо
 *   живого экрана.
 *
 * Проверяем на хосте целиком: развилка живёт в prepareStep, а применяет её
 * движок в createDriver - между ними легко разъехаться.
 */

const mocks = vi.hoisted(() => ({
  driver: { config: null, activeIndex: 0, moves: [] },
  route: { path: '/news' },
  router: { push: () => Promise.resolve(), afterEach: () => () => {} },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.driver.config = config;
    return {
      getActiveIndex: () => mocks.driver.activeIndex,
      moveNext: () => mocks.driver.moves.push('next'),
      movePrevious: () => mocks.driver.moves.push('prev'),
      moveTo: (i) => mocks.driver.moves.push(`to:${i}`),
      drive: (i) => { mocks.driver.activeIndex = i; },
      destroy: () => {},
    };
  },
}));

// createRouter/createWebHistory нужны не самому хосту, а модулям, которые он
// тянет транзитивно (api-клиент через стор уведомлений). Без них мок роняет
// загрузку файла целиком, ещё до первого теста.
vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => mocks.router,
  createRouter: () => mocks.router,
  createWebHistory: () => ({}),
}));

/**
 * Ожидание цели в jsdom не работает по существу: у элементов нулевые рамки, и
 * настоящий waitForElement всегда упирался бы в таймаут. Подменяем его на
 * проверку присутствия в DOM - именно её он и выражает на живой странице.
 * Остальное (createDriver, сборка поповера) - настоящее.
 */
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
  { id: 'intro', route: '/news', element: '[data-testid="ob-intro"]', title: 'Начало', description: 'Вступление' },
  {
    id: 'empty-demo',
    route: '/news',
    element: '[data-testid="ob-list"]',
    title: 'Ваши заявки',
    description: 'Все поданные заявки собраны здесь',
    optional: true,
    demo: 'applications',
  },
  {
    id: 'empty-plain',
    route: '/news',
    element: '[data-testid="ob-extra"]',
    title: 'Доп. поля',
    description: 'Поля бланка',
    optional: true,
  },
  { id: 'outro', route: '/news', element: '[data-testid="ob-outro"]', title: 'Финал', description: 'Конец' },
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

/**
 * Поднять тур с первого шага и дождаться, пока хост соберёт сегмент.
 *
 * @param {number} [startIndex]
 */
async function startTour(startIndex = 0) {
  wrapper = mount(OnboardingTour);
  store.currentIndex = startIndex;
  store.isActive = true;
  await flushPromises();
}

/** @returns {import('driver.js').DriveStep} шаг в том виде, в каком его получил driver */
function shownStep(localIndex) {
  return mocks.driver.config.steps[localIndex];
}

describe('OnboardingTour - шаг без цели на пустой системе', () => {
  beforeEach(() => {
    mocks.driver.config = null;
    mocks.driver.activeIndex = 0;
    mocks.driver.moves = [];
    mocks.route.path = '/news';
    store.isActive = false;
    store.currentIndex = 0;
    store.skippedIndexes = [];
    document.body.innerHTML = '<div data-testid="ob-intro"></div><div data-testid="ob-outro"></div>';
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('цели нет, но есть скриншот: шаг остаётся, подсветка снята, картинка в поповере', async () => {
    await startTour();

    await mocks.driver.config.onNextClick();

    expect(mocks.driver.moves).toEqual(['next']);
    expect(shownStep(1).element).toBeUndefined();
    expect(shownStep(1).popover.description).toContain('ob-popover__demo-img');
  });

  it('цель на месте: подсветка реального элемента, скриншота нет', async () => {
    document.body.insertAdjacentHTML('beforeend', '<div data-testid="ob-list"></div>');
    await startTour();

    await mocks.driver.config.onNextClick();

    expect(mocks.driver.moves).toEqual(['next']);
    expect(shownStep(1).element).toBe('[data-testid="ob-list"]');
    expect(shownStep(1).popover.description).not.toContain('<img');
  });

  it('опциональный шаг без скриншота при отсутствии цели выбрасывается', async () => {
    await startTour();
    mocks.driver.activeIndex = 1;

    await mocks.driver.config.onNextClick();

    // Шаг «Доп. поля» человек не увидит - тур уходит сразу на финал.
    expect(mocks.driver.moves).toEqual(['to:3']);
  });

  it('первый шаг сегмента с недостижимой целью деградирует в центр-модал', async () => {
    // Обычный (не optional) шаг: цель могла не отрисоваться из-за медленных данных -
    // тур не падает и не ждёт вечно, а показывает поповер по центру.
    document.body.innerHTML = '';
    await startTour();

    expect(shownStep(0).element).toBeUndefined();
    expect(shownStep(0).popover.description).not.toContain('<img');
  });

  it('счётчик считает все шаги, пока ни один не выброшен', async () => {
    await startTour();
    mocks.driver.activeIndex = 1;
    const popover = fakePopover();

    mocks.driver.config.onPopoverRender(popover);

    // «Доп. поля» может и не выпасть - пока шаг не выброшен, он в счёте.
    expect(popover.wrapper.querySelector('.ob-popover__step-label').textContent).toBe('Шаг 2 из 4');
  });
});

/** Минимальный PopoverDOM - ровно те узлы, которые трогает onPopoverRender. */
function fakePopover() {
  const wrapper2 = document.createElement('div');
  const title = document.createElement('div');
  const description = document.createElement('div');
  const footer = document.createElement('div');
  const footerButtons = document.createElement('div');
  const previousButton = document.createElement('button');
  footer.appendChild(footerButtons);
  wrapper2.append(title, description, footer);
  return { wrapper: wrapper2, title, description, footer, footerButtons, previousButton };
}
