/** Identity belongs to the shown step, not a pending route's numeric index. */
export function createTourRecoveryState({ getSteps, restore, present, resume, stop }) {
  let committed = null, intended = null, failure = null, showing = false, epoch = 0, interrupted = false;
  function reset() { epoch++; committed = null; intended = null; failure = null; showing = false; interrupted = false; }
  function commit(stepId) { epoch++; committed = stepId; failure = null; showing = false; }
  function prepare(stepId) {
    epoch++; intended = stepId; showing = false;
    if (failure) failure = { ...failure, stepId };
  }
  function show() {
    if (!failure) return;
    showing = true;
    present({
      retry: () => resume(failure.stepId),
      back: committed && getSteps().some(step => step.id === committed) ? () => resume(committed) : undefined,
      close: () => stop(committed),
    });
  }
  async function fail(error, operation) {
    if (interrupted) return;
    const current = ++epoch;
    failure = { stepId: error?.stepId || intended, reason: error?.reason || 'load-error' };
    await restore(committed);
    if (current !== epoch || (operation && !operation.isCurrent())) return;
    show();
  }
  function retryAt(stepId) {
    const index = getSteps().findIndex(step => step.id === stepId);
    if (index < 0) { stop(committed); return; }
    showing = false;
    return index;
  }
  async function interrupt() {
    interrupted = true;
    const current = ++epoch;
    await restore(committed);
    if (current === epoch) stop(committed);
  }
  return { reset, commit, prepare, fail, show, retryAt, interrupt,
    hasFailure: () => Boolean(failure), isShowing: () => showing };
}
