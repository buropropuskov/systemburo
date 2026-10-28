import { isTourTargetVisible } from './targetResolver';

const TERMINAL_STATES = new Set(['ready', 'empty', 'error', 'forbidden']);

/** Read only the current step's owner, never unrelated page loading indicators. */
export function readStepOwner(key, root = globalThis.document) {
  const scope = typeof root === 'function' ? root() : root;
  const candidates = [...(scope?.querySelectorAll?.('[data-ob-owner]') || [])];
  if (scope?.matches?.('[data-ob-owner]')) candidates.unshift(scope);
  const owned = candidates.filter(node => node.dataset.obOwner === key && node.isConnected);
  // Request state exists even while an empty/loading region has no layout box.
  // Prefer the rendered responsive copy, but never guess among hidden duplicates.
  const element = owned.find(isTourTargetVisible) || (owned.length === 1 ? owned[0] : null);
  return element ? { state: element.dataset.obState, element } : null;
}

/** Retry only an owned failed/pending read, through the view's existing refresh. */
export function retryStepOwner(key) {
  if (!key) return false;
  const owner = readStepOwner(key);
  if (!owner || !['loading', 'error', 'forbidden'].includes(owner.state)) return false;
  const event = new owner.element.ownerDocument.defaultView.CustomEvent('onboarding-retry', { cancelable: true });
  owner.element.dispatchEvent(event);
  return event.defaultPrevented;
}

/** Missing/loading/unknown owners remain unknown until an outcome or cancellation. */
export function waitForStepOwner(key, { timeout = 5000, signal, root = globalThis.document } = {}) {
  if (signal?.aborted) return Promise.resolve({ kind: 'cancelled' });
  if (!key) return Promise.resolve({ kind: 'unmanaged' });
  return new Promise((resolve) => {
    let interval = null, deadline = null, stopped = false;
    const finish = (result) => {
      if (stopped) return;
      stopped = true;
      clearInterval(interval); clearTimeout(deadline);
      signal?.removeEventListener('abort', abort);
      resolve(result);
    };
    const abort = () => finish({ kind: 'cancelled' });
    const check = () => {
      const owner = readStepOwner(key, root);
      if (owner && TERMINAL_STATES.has(owner.state)) finish({ kind: owner.state, element: owner.element });
    };
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) { abort(); return; }
    check();
    if (stopped) return;
    interval = setInterval(check, 50);
    deadline = setTimeout(() => {
      check();
      if (!stopped) finish({ kind: 'timeout' });
    }, timeout);
  });
}
