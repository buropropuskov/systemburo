import { afterEach, describe, expect, it, vi } from 'vitest';
import { effectScope, nextTick, ref } from 'vue';
import { buildTourSteps, getTour } from '../tours';
import { onboardingSteps } from '../onboardingSteps';
import { createActiveStepSet } from '../activeStepSet';
import * as contextModule from '../tourContext';

const scopes = [];
const context = () => ({
  isAuthenticated: true, can: () => true, allowsApplicationAction: () => true,
  approvalRole: { isApprover: false, isReviewer: false }, hasOwnApplication: true,
});
afterEach(() => { scopes.splice(0).forEach(scope => scope.stop()); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe('user download lesson follows the real responsive entry point', () => {
  it('retains the existing desktop anchor and order', () => {
    expect(buildTourSteps('user', { ...context(), isPhoneViewport: false })).toEqual(onboardingSteps);
  });

  it('retains the phone lesson without waiting for a button hidden in the list', () => {
    const steps = buildTourSteps('user', { ...context(), isPhoneViewport: true });
    expect(steps.map(step => step.id)).toEqual(onboardingSteps.map(step => step.id));
    const download = steps.find(step => step.id === 'cabinet-download');
    expect(download.element).toBeNull();
    expect(download.description).toContain('шапке карточки');
    expect(onboardingSteps.find(step => step.id === download.id).element).toBe('[data-testid="ob-application-download"]');
  });

  it('keeps the same active lesson and total when the viewport changes', async () => {
    const tourContext = ref({ ...context(), isPhoneViewport: false });
    const currentIndex = ref(onboardingSteps.findIndex(step => step.id === 'cabinet-download'));
    const currentStepId = ref('cabinet-download');
    const saveProgress = vi.fn();
    const scope = effectScope(); scopes.push(scope);
    const set = scope.run(() => createActiveStepSet({
      activeTour: ref(getTour('user')), tourContext, accessReady: ref(true), isActive: ref(true),
      currentIndex, currentStepId, skippedIndexes: ref([]), saveProgress,
    }));
    const total = set.steps.value.length, index = currentIndex.value;
    set.capture();
    tourContext.value = { ...context(), isPhoneViewport: true };
    await nextTick();
    expect(currentStepId.value).toBe('cabinet-download');
    expect(currentIndex.value).toBe(index);
    expect(set.steps.value).toHaveLength(total);
    expect(set.steps.value[index].element).toBeNull();
    tourContext.value = { ...context(), isPhoneViewport: false };
    await nextTick();
    expect(currentIndex.value).toBe(index);
    expect(set.steps.value[index].element).toBe('[data-testid="ob-application-download"]');
    expect(saveProgress).toHaveBeenCalledTimes(2);
  });

  it('tracks the actual phone boundary and releases its viewport listener with the store scope', () => {
    vi.stubGlobal('innerWidth', 390);
    const remove = vi.spyOn(window, 'removeEventListener');
    const scope = effectScope(); scopes.push(scope);
    const ctx = scope.run(() => contextModule.createTourContext(
      () => ({ isAuthenticated: true, isSuperAdmin: false }),
      () => ({ hasPermission: () => true, allowsApplicationAction: () => true, loaded: true }),
      () => ({ approvalRole: ref({}), factTableRoute: ref(null), hasOwnApplication: ref(true),
        securityPostContext: ref(null), factRouteLoaded: ref(true), accessibleContext: ref({}) }),
    ));
    expect(ctx.value.isPhoneViewport).toBe(true);
    vi.stubGlobal('innerWidth', 768);
    window.dispatchEvent(new Event('resize'));
    expect(ctx.value.isPhoneViewport).toBe(false);
    scope.stop();
    expect(remove).toHaveBeenCalledWith('resize', expect.any(Function));
    vi.stubGlobal('innerWidth', 390);
    window.dispatchEvent(new Event('resize'));
    expect(ctx.value.isPhoneViewport).toBe(false);
  });
});
