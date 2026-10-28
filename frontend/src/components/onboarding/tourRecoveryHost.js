import { createRecoveryDriver } from './tourRecovery';
import { createTourRecoveryState } from './tourRecoveryState';
import { restoreReveal } from './reveal';
import { retryStepOwner } from './ownerReadiness';

/** Host adapter: recovery owns no completed progress or business data. */
export function createTourRecoveryHost({
  store, keys, watchers, rail, getGeometry, getDriver, setDriver, advanceGeneration, getGeneration,
  getRoute, cancelTransitions, startSegment, pushRoute, reducedMotion, isBlocked, stop,
}) {
  function restore(committedId) {
    keys.detach(); watchers.stopAll(); getGeometry()?.stop();
    advanceGeneration(); const previous = getDriver(); setDriver(null); previous?.destroy();
    store.clearPending(); rail.restore(); restoreReveal();
    const index = store.steps.findIndex(step => step.id === committedId);
    if (index >= 0) store.setIndex(index);
    store.setDemoAttachment(index >= 0 ? store.steps[index]?.demoAttachment || null : null);
  }
  function resume(stepId) {
    const index = state.retryAt(stepId);
    if (index === undefined) return;
    cancelTransitions(); restore(null);
    retryStepOwner(store.steps[index].dataOwner);
    // Pending selection does not persist the desired step as already viewed.
    store.retreatSegment(index);
    if (store.steps[index].route !== getRoute()) return pushRoute(store.steps[index].route);
    store.clearPending(); return startSegment();
  }
  const state = createTourRecoveryState({
    getSteps: () => store.steps, restore, resume, stop,
    present(controls) {
      if (!store.isActive || isBlocked()) return;
      const gen = getGeneration();
      const instance = createRecoveryDriver({ ...controls, reducedMotion: reducedMotion(),
        onDestroyed: () => { if (gen === getGeneration() && store.isActive) stop(); } });
      setDriver(instance); keys.attach(); instance.drive(0);
    },
  });
  return { ...state, stop,
    restartAt(stepId) {
      if (!store.isActive) return;
      if (!store.steps.some(step => step.id === stepId)) return stop();
      state.prepare(stepId);
      return resume(stepId);
    },
    interrupt() { cancelTransitions(); return state.interrupt(); },
  };
}
