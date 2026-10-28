import { watch } from 'vue';

/** The store emits only after its permission batch and semantic-ID rebase. */
export function createStepSetReconciliation({
  store, recovery, cancel, cancelPendingRoute = () => {}, isBlocked = () => false,
}) {
  let sequence = 0, deferred = false;
  const accessError = () => Object.assign(new Error('Learning access is not ready'), {
    reason: 'unknown-access', stepId: store.currentStepId || store.currentStep?.id,
  });
  const isKnown = () => store.accessReady !== false;
  function reconcile() {
    const current = ++sequence;
    if (!store.isActive) return;
    const pendingRoute = store.pendingSegment;
    cancel();
    if (pendingRoute) {
      try {
        Promise.resolve(cancelPendingRoute()).catch(() => {
          if (current === sequence && store.isActive) recovery.fail(accessError());
        });
      } catch {
        return recovery.fail(accessError());
      }
    }
    // The upper dialog owns navigation as well as keyboard/focus. Re-read the
    // newest store snapshot on release instead of remembering a stale target.
    deferred = isBlocked();
    if (deferred) return;
    if (!isKnown()) return recovery.fail(accessError());
    return recovery.restartAt(store.currentStepId);
  }
  const unwatch = watch(() => store.stepsRevision, reconcile, { flush: 'sync' });
  const unwatchRun = watch(() => [store.isActive, store.activeTourKey], () => {
    sequence++; deferred = false;
  }, { flush: 'sync' });
  return {
    isKnown,
    resumeDeferred() {
      if (!deferred || isBlocked() || !store.isActive) return false;
      reconcile();
      return true;
    },
    failUnknown: () => recovery.fail(accessError()),
    async ensure(operation) {
      if (!isKnown()) await store.prepareTour(store.activeTourKey);
      if (!operation.isCurrent()) return false;
      if (!isKnown()) throw accessError();
      return true;
    },
    dispose() { sequence++; deferred = false; unwatch(); unwatchRun(); },
  };
}
