/**
 * One operation owns preparation and commit. Callers must check isCurrent()
 * after an await and before applying a result to the tour or its UI.
 */
export function createStepTransition({ isActive = () => true, onBusyChange = () => {}, onError } = {}) {
  let current = null;
  let sequence = 0;

  function isBusy() {
    return current !== null;
  }

  function cancel() {
    if (!current) return;
    const previous = current;
    current = null;
    previous.controller.abort();
    // An abort listener may have started another operation.
    if (!current) onBusyChange(false);
  }

  function run(work, { replace = false, intent = null } = {}) {
    if (!isActive() || (current && !replace)) return undefined;
    const previous = current;
    const operation = { controller: new AbortController() };
    current = operation;
    previous?.controller.abort();
    if (current === operation) onBusyChange(true);
    const context = {
      operationId: ++sequence,
      intent,
      signal: operation.controller.signal,
      isCurrent: () => current === operation && !operation.controller.signal.aborted && isActive(),
    };
    const cleanup = () => {
      if (current !== operation) return;
      current = null;
      onBusyChange(false);
    };
    const handleError = (error) => {
      if (!onError) throw error;
      // A late failure belongs to its cancelled operation, never the new step.
      if (context.isCurrent()) return onError(error, context);
    };
    // A busy observer or previous abort listener may cancel/replace this operation.
    if (!context.isCurrent()) {
      cleanup();
      return Promise.resolve(undefined);
    }
    try {
      return Promise.resolve(work(context)).catch(handleError).finally(cleanup);
    } catch (error) {
      return Promise.reject(error).catch(handleError).finally(cleanup);
    }
  }

  return { run, cancel, isBusy };
}
