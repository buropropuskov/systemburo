import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reactive } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import { resolveStepIndex } from '../tourProgress';
import { setModalOpen, releaseModal } from '@/utils/modalStack';

const mocks = vi.hoisted(() => ({ store: null, ui: null, handles: [], heldTargets: new Map() }));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => mocks.store }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => mocks.ui }));
vi.mock('driver.js', () => ({ driver: initialConfig => {
  let config = initialConfig, index = 0;
  const handle = {
    moves: [], destroyed: false,
    getConfig: () => config, setConfig: value => { config = value; },
    getActiveIndex: () => index,
    getActiveElement: () => {
      const element = config.steps[index]?.element;
      return typeof element === 'function' ? element() : element || null;
    },
    drive: value => { index = value; config.onHighlighted?.(); },
    moveNext: () => { handle.moves.push('next'); index++; config.onHighlighted?.(); },
    movePrevious: () => { handle.moves.push('previous'); index--; config.onHighlighted?.(); },
    moveTo: value => { handle.moves.push(value); index = value; config.onHighlighted?.(); },
    destroy: () => { handle.destroyed = true; config.onDestroyed?.(); }, refresh: vi.fn(),
  };
  mocks.handles.push(handle); return handle;
} }));
vi.mock('@/composables/useOnboarding', async original => {
  const actual = await original();
  return { ...actual, useOnboarding: () => ({ ...actual.useOnboarding(),
    prefersReducedMotion: () => true,
    ensureInView: async () => {},
    waitForElement: selector => mocks.heldTargets.get(selector)?.promise || Promise.resolve(document.querySelector(selector)),
  }) };
});
import OnboardingTour from '../OnboardingTour.vue';

let wrapper, router;
const info = (id, route = '/news') => ({ id, route, element: null, title: id, description: 'Learning' });
function deferred() {
  let resolve, reject;
  const promise = new Promise((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}
function target(selector = 'pending') {
  const node = document.createElement('div'); node.id = selector;
  node.getBoundingClientRect = () => ({ left: 20, top: 20, right: 120, bottom: 60, width: 100, height: 40 });
  document.body.append(node); return node;
}
function replaceSteps(next, ready = true) {
  const store = mocks.store, ids = store.steps.map(step => step.id);
  if (ready) {
    const index = resolveStepIndex(store.currentStepId, ids, next, store.currentIndex);
    store.steps = next; store.currentIndex = index; store.currentStepId = next[index]?.id || null;
  }
  store.accessReady = ready; store.stepsRevision++;
}
async function start(steps, index = 0) {
  mocks.store = reactive({
    steps, isActive: false, activeTourKey: 'user', currentIndex: index, currentStepId: steps[index]?.id,
    accessReady: true, stepsRevision: 0, isManual: true, pendingSegment: false,
    skippedIndexes: [], statusLoaded: true, canShowTour: false, revealOpen: null, demoAttachmentType: null,
    get currentStep() { return this.steps[this.currentIndex]; },
    setIndex(value) { this.currentIndex = value; this.currentStepId = this.steps[value]?.id || null; },
    markSkipped(value) { this.skippedIndexes.push(value); },
    setDemoAttachment(value) { this.demoAttachmentType = value; },
    setRevealOpen(value) { this.revealOpen = value; },
    clearPending() { this.pendingSegment = false; },
    retreatSegment(value) { this.currentIndex = value; this.currentStepId = this.steps[value]?.id; this.pendingSegment = true; },
    advanceSegment() { this.currentIndex++; this.currentStepId = this.currentStep?.id; this.pendingSegment = true; },
    stop() { this.isActive = false; }, markCompleted: vi.fn(),
    prepareTour: vi.fn(async () => ({ status: 'blocked' })), loadStatus: vi.fn(), pickAutostartTour: () => null,
  });
  wrapper = mount(OnboardingTour, { global: { plugins: [router] } });
  mocks.store.isActive = true; await flushPromises();
  return mocks.store;
}
function latest() { return mocks.handles.at(-1); }

beforeEach(async () => {
  mocks.handles = []; mocks.heldTargets.clear();
  mocks.ui = reactive({ sidebarHidden: false, tourForceExpand: false });
  document.body.replaceChildren();
  router = createRouter({ history: createMemoryHistory(), routes: ['/news', '/carsview', '/employeesview']
    .map(path => ({ path, component: { template: '<div />' } })) });
  await router.push('/news'); await router.isReady();
});
afterEach(() => { wrapper?.unmount(); wrapper = null; document.body.replaceChildren(); });

describe('host follows completed semantic step-set revisions', () => {
  it('rebuilds the same ID after an earlier chapter disappears', async () => {
    const store = await start([info('intro'), info('cars'), info('employees')], 1), old = latest();
    replaceSteps([info('cars'), info('employees')]); await flushPromises();
    expect(old.destroyed).toBe(true); expect(latest()).not.toBe(old);
    expect(store.currentIndex).toBe(0); expect(store.currentStepId).toBe('cars');
    expect(latest().getConfig().steps[0].popover.title).toBe('cars');
    expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('prepares the next remaining ID when the current step is removed', async () => {
    const store = await start([info('intro'), info('early'), info('cars'), info('employees')], 2);
    replaceSteps([info('intro'), info('employees'), info('after')]); await flushPromises();
    expect(store.currentStepId).toBe('employees'); expect(store.currentIndex).toBe(1);
    expect(latest().getActiveIndex()).toBe(1); expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('pauses unknown access, ignores the late preparation, and does not open a route', async () => {
    const node = target(), held = deferred(); mocks.heldTargets.set('#pending', held);
    const store = await start([info('intro'), { ...info('cars'), element: '#pending' }]), old = latest();
    const pending = old.getConfig().onNextClick(); await flushPromises();
    const ids = store.steps.map(step => step.id); replaceSteps([], false); await flushPromises();
    expect(store.steps.map(step => step.id)).toEqual(ids);
    expect(latest().getConfig().steps[0].popover.title).toBe('Не удалось открыть шаг');
    held.resolve(node); await pending; await flushPromises();
    expect(old.moves).toEqual([]); expect(store.currentStepId).toBe('intro');
    expect(router.currentRoute.value.path).toBe('/news');
    expect(store.markCompleted).not.toHaveBeenCalled();
    await latest().getConfig().onNextClick(); await flushPromises();
    expect(store.prepareTour).toHaveBeenCalledOnce();
    expect(latest().getConfig().steps[0].popover.title).toBe('Не удалось открыть шаг');
    expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('re-prepares the same ID after unknown access becomes known again', async () => {
    const store = await start([info('intro'), info('cars')], 1);
    replaceSteps([], false); await flushPromises();
    replaceSteps([info('intro'), info('cars')], true); await flushPromises();
    expect(store.currentStepId).toBe('cars'); expect(latest().getConfig().steps[1].popover.title).toBe('cars');
    expect(store.isActive).toBe(true); expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('cancels a denied pending target without committing its old Driver intent', async () => {
    const node = target(), held = deferred(); mocks.heldTargets.set('#pending', held);
    const store = await start([info('intro'), { ...info('cars'), element: '#pending' }, info('employees')]), old = latest();
    const pending = old.getConfig().onNextClick(); await flushPromises();
    replaceSteps([info('intro'), info('employees')]); await flushPromises();
    held.resolve(node); await pending; old.getConfig().onHighlighted(); await flushPromises();
    expect(old.destroyed).toBe(true); expect(old.moves).toEqual([]);
    expect(store.currentStepId).toBe('intro'); expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('cancels the real memory-router pending route when the rebased target stays on this page', async () => {
    const held = deferred();
    router.beforeEach(to => to.path === '/carsview' ? held.promise : undefined);
    const store = await start([info('intro'), info('cars', '/carsview'), info('employees')]), old = latest();
    const pending = old.getConfig().onNextClick(); await flushPromises();
    expect(store.pendingSegment).toBe(true);
    replaceSteps([info('intro'), info('employees')]); await flushPromises();
    held.resolve(); await pending; await flushPromises();
    expect(router.currentRoute.value.path).toBe('/news'); expect(store.currentStepId).toBe('employees');
    expect(store.isActive).toBe(true); expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('treats an empty eligible set as a technical stop, not a completed auto tour', async () => {
    const store = await start([info('intro')]); store.isManual = false;
    replaceSteps([]); await flushPromises();
    expect(store.isActive).toBe(false); expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it('defers the latest rebased route until the global dialog releases ownership', async () => {
    const store = await start([info('intro'), info('cars'), info('employees', '/employeesview')], 1);
    const upper = {};
    setModalOpen(upper, true, 22000); await flushPromises();
    try {
      replaceSteps([info('intro'), info('employees', '/employeesview')]); await flushPromises();
      expect(router.currentRoute.value.path).toBe('/news');
      expect(store.currentStepId).toBe('employees');
      replaceSteps([info('intro'), info('employees', '/carsview')]); await flushPromises();
      expect(router.currentRoute.value.path).toBe('/news');
    } finally { releaseModal(upper); }
    await flushPromises();
    expect(router.currentRoute.value.path).toBe('/carsview');
    expect(store.currentStepId).toBe('employees');
    expect(latest().getConfig().steps[0].popover.title).toBe('employees');
    expect(store.markCompleted).not.toHaveBeenCalled();
  });

  it.each(['user', 'accept'])('ignores an old route-cancellation rejection after restarting %s', async tourKey => {
    const store = await start([info('intro')]);
    const cancellation = deferred();
    vi.spyOn(router, 'push').mockImplementationOnce(() => cancellation.promise);
    store.pendingSegment = true;
    replaceSteps([info('intro')]); await flushPromises();
    store.isActive = false; await flushPromises();
    store.activeTourKey = tourKey; store.isActive = true; await flushPromises();
    const current = latest();
    // Reject explicitly, independently from VueRouter's normally resolved failures.
    cancellation.reject(new Error('Old route cancellation failed')); await flushPromises();
    expect(latest()).toBe(current);
    expect(current.destroyed).toBe(false);
    expect(current.getConfig().steps[0].popover.title).toBe('intro');
    expect(store.markCompleted).not.toHaveBeenCalled();
  });
});
