// Notification delivery is outside host navigation tests; keep router/API isolated.
import { installScrollIntoView } from './fixtureScrollIntoView';
let restoreScrollIntoView;
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }));
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';
import { setModalOpen, releaseModal } from '@/utils/modalStack';

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
  driver: { config: null, activeIndex: 0, destroys: 0, creates: 0 },
  route: { path: '/news' },
  afterEach: null,
  router: { push: vi.fn(() => Promise.resolve()), afterEach: callback => { mocks.afterEach = callback; return () => {}; } },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.driver.creates += 1;
    mocks.driver.config = config;
    return {
      getConfig: () => mocks.driver.config,
      setConfig: (next) => { mocks.driver.config = next; },
      getActiveIndex: () => mocks.driver.activeIndex,
      getActiveElement: () => {
        const element = config.steps[mocks.driver.activeIndex]?.element;
        return typeof element === 'function' ? element() : element || null;
      },
      moveNext: () => {},
      movePrevious: () => {},
      moveTo: () => {},
      drive: (i) => { mocks.driver.activeIndex = i; },
      destroy: () => { mocks.driver.destroys += 1; config.onDestroyed?.(); },
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
  demoAttachmentType: null,
  setDemoAttachment: vi.fn(function (value) { this.demoAttachmentType = value; }),
  setIndex(i) { this.currentIndex = i; },
  advanceSegment: vi.fn(),
  retreatSegment(i) { this.currentIndex = i; this.pendingSegment = true; },
  jumpToSegment: vi.fn(),
  clearPending() { this.pendingSegment = false; },
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
    restoreScrollIntoView = installScrollIntoView();
    mocks.driver.config = null;
    mocks.driver.activeIndex = 0;
    mocks.route.path = '/news';
    store.isActive = false;
    store.isManual = true;
    store.markCompleted.mockClear();
    store.currentIndex = 0;
    store.pendingSegment = false;
    store.revealOpen = null;
    store.demoAttachmentType = null;
    store.skippedIndexes = [];
    document.body.innerHTML = '<div data-testid="ob-bell"></div><div data-testid="ob-search"></div>';
    document.body.querySelectorAll('[data-testid]').forEach(node => {
      node.getBoundingClientRect = () => ({ left: 20, top: 20, right: 100, bottom: 60, width: 80, height: 40 });
    });
    wrapper = mount(OnboardingTour);
    store.isActive = true;
    await flushPromises();
  });

  afterEach(() => {
    restoreScrollIntoView?.();
    wrapper?.unmount();
    wrapper = null;
  });

  async function failNextStep() {
    mocks.driver.config.onHighlighted();
    steps[1].dataOwner = 'recovery-fixture';
    const owner = document.createElement('div'); owner.dataset.obOwner = 'recovery-fixture'; owner.dataset.obState = 'error';
    owner.getBoundingClientRect = () => ({ left: 20, top: 20, right: 100, bottom: 60, width: 80, height: 40 });
    document.body.append(owner);
    await mocks.driver.config.onNextClick(); await flushPromises();
    expect(mocks.driver.config.steps[0].popover.title, JSON.stringify({ active: store.isActive,
      index: store.currentIndex, creates: mocks.driver.creates, destroys: mocks.driver.destroys,
      skipped: store.skippedIndexes, pending: store.pendingSegment })).toBe('Не удалось открыть шаг');
    return owner;
  }

  it('restores Arrow keys and Escape after retry actually shows its intended step', async () => {
    try {
      const owner = await failNextStep(); owner.dataset.obState = 'ready';
      owner.dataset.testid = 'ob-panel';
      await mocks.driver.config.onNextClick(); await flushPromises();
      expect(store.currentIndex).toBe(1);
      mocks.driver.config.onHighlighted();
      const next = vi.fn(); mocks.driver.config.onNextClick = next;
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }));
      expect(next).toHaveBeenCalledOnce();
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      expect(store.isActive).toBe(false);
    } finally { delete steps[1].dataOwner; }
  });

  it.each(['unmount', 'external-route'])('does not mark an auto tour completed after recovery %s', async action => {
    store.isManual = false;
    try {
      await failNextStep();
      if (action === 'unmount') { wrapper.unmount(); wrapper = null; }
      else { mocks.afterEach({ path: '/cabinet' }, { path: '/news' }); await flushPromises(); }
      expect(store.markCompleted).not.toHaveBeenCalled();
    } finally { delete steps[1].dataOwner; }
  });

  it('destroys the paused Driver and restores the current demo after an interrupted preparation', async () => {
    const upper = {}, before = mocks.driver.creates, destroyed = mocks.driver.destroys;
    steps[0].demoAttachment = 'cars'; steps[1].demoAttachment = 'employees';
    try {
      const next = mocks.driver.config.onNextClick();
      expect(store.setDemoAttachment).toHaveBeenLastCalledWith('employees');
      setModalOpen(upper, true, 22000);
      await next;
      expect(mocks.driver.destroys).toBe(destroyed + 1);
      expect(store.isActive).toBe(true); expect(store.currentIndex).toBe(0);
      releaseModal(upper); await flushPromises();
      expect(mocks.driver.creates).toBe(before + 1);
      expect(store.setDemoAttachment).toHaveBeenLastCalledWith('cars');
      expect(store.currentIndex).toBe(0); expect(store.skippedIndexes).toEqual([]);
      expect(store.markCompleted).not.toHaveBeenCalled();
    } finally { releaseModal(upper); delete steps[0].demoAttachment; delete steps[1].demoAttachment; }
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

  it('поздняя подсветка не возвращает прежний демо-бланк во время подготовки следующего', async () => {
    steps[0].demoAttachment = 'cars';
    steps[1].demoAttachment = 'employees';
    store.setDemoAttachment.mockClear();
    const next = mocks.driver.config.onNextClick();
    try {
      expect(store.setDemoAttachment).toHaveBeenLastCalledWith('employees');
      mocks.driver.config.onHighlighted();
      expect(store.setDemoAttachment).toHaveBeenLastCalledWith('employees');
    } finally {
      await next;
      delete steps[0].demoAttachment;
      delete steps[1].demoAttachment;
    }
  });

  it('stop и новый запуск в одном событии заменяют инстанс, не продолжая старый', async () => {
    const previous = mocks.driver.config;
    const moving = previous.onNextClick();
    store.stop();
    store.isActive = true;
    await flushPromises();
    await moving;
    expect(mocks.driver.config).not.toBe(previous);
    expect(store.currentIndex).toBe(0);
    expect(store.skippedIndexes).toEqual([]);
  });
});
