import { isTourTargetVisible } from './targetResolver';

const TRANSFORM_WINDOW_MS = 1200;
const noop = () => {};
const box = el => {
  const r = el?.getBoundingClientRect?.();
  return r ? [r.top, r.left, r.width, r.height] : null;
};

/** One active geometry lease. Navigation and Driver ownership remain with the host. */
export function createTourGeometry({
  getDriver = () => null, resolveTarget = frame => frame.target,
  scrollTarget = noop, applyShape = noop, correctPopover = noop, requestRetarget = noop,
  shouldScroll = () => false, onError = noop, restoreHighlight = noop,
  getPopover = () => getDriver()?.getState?.()?.popover?.wrapper || null,
} = {}) {
  let epoch = 0, active = null, disposed = false;
  const preparations = new Set();
  const current = lease => active === lease && lease.epoch === epoch
    && !lease.controller.signal.aborted && !lease.signal?.aborted
    && (lease.isCurrent?.() ?? true);

  async function prepare(frame, operation) {
    const prepareEpoch = epoch;
    const controller = new AbortController();
    const allowed = () => !disposed && prepareEpoch === epoch
      && !controller.signal.aborted && !operation.signal?.aborted && operation.isCurrent();
    if (!allowed() || !isTourTargetVisible(frame.target)) return null;
    const onAbort = () => controller.abort();
    operation.signal?.addEventListener('abort', onAbort, { once: true });
    preparations.add(controller);
    try {
      if (operation.signal?.aborted) controller.abort();
      if (!allowed()) return null;
      await scrollTarget(frame.target, frame.scrollTo, controller.signal);
      return allowed() && isTourTargetVisible(frame.target) ? frame.target : null;
    } finally {
      preparations.delete(controller);
      operation.signal?.removeEventListener('abort', onAbort);
    }
  }

  function stop() {
    const oldPreparations = [...preparations];
    preparations.clear();
    const lease = active;
    active = null;
    epoch += 1;
    oldPreparations.forEach(controller => controller.abort());
    if (!lease) return;
    lease.controller.abort();
    if (lease.raf !== null) cancelAnimationFrame(lease.raf);
    lease.raf = null;
    lease.resize?.disconnect();
    lease.mutation?.disconnect();
    lease.cleanups.forEach(cleanup => cleanup());
  }

  function schedule(lease) {
    if (!current(lease) || lease.raf !== null) return;
    lease.raf = requestAnimationFrame(() => {
      lease.raf = null;
      if (!current(lease)) return;
      render(lease);
      if (current(lease) && lease.transitions.size && Date.now() < lease.transformUntil) schedule(lease);
    });
  }

  function invalidate(reason = 'layout') {
    if (!active) return;
    active.reasons.add(reason);
    schedule(active);
  }

  function render(lease) {
    const frame = { target: lease.target, index: lease.index, stepId: lease.stepId, scope: lease.scope };
    const candidate = resolveTarget(frame);
    if (!current(lease)) return;
    const visible = isTourTargetVisible(candidate);
    if (lease.target && !visible) {
      if (!lease.missing) {
        lease.missing = true;
        lease.requested = new WeakSet();
        requestRetarget({ index: lease.index, stepId: lease.stepId, target: null,
          epoch: lease.epoch, reason: 'target-missing' });
      }
      return;
    }
    if (lease.missing && visible) {
      lease.missing = false;
      lease.signature = null;
      lease.requested.add(candidate);
      requestRetarget({ index: lease.index, stepId: lease.stepId, target: candidate,
        epoch: lease.epoch, reason: 'target-recovered' });
      return;
    }
    if (candidate !== lease.target) {
      if (isTourTargetVisible(candidate) && !lease.requested.has(candidate)) {
        lease.requested.add(candidate);
        requestRetarget({ index: lease.index, stepId: lease.stepId, target: candidate, epoch: lease.epoch });
      }
      return;
    }
    if (lease.target && !isTourTargetVisible(lease.target)) return;
    const view = lease.view;
    const viewport = view.visualViewport;
    const popover = getPopover();
    observePopover(lease, popover);
    // Animated position/size are Driver outputs. Track the actual content instead
    // so width -> line wrapping -> height cannot feed placement back into itself.
    const content = popover ? [popover.textContent, [...popover.querySelectorAll('img')]
      .map(img => [img.currentSrc || img.src, img.naturalWidth, img.naturalHeight])] : null;
    const signature = JSON.stringify([box(lease.target), box(lease.scope), content,
      view.innerWidth, view.innerHeight, viewport?.width, viewport?.height,
      viewport?.offsetTop, viewport?.offsetLeft, viewport?.scale]);
    const changed = signature !== lease.signature || lease.reasons.has('popover-content');
    const correction = lease.reasons.has('popover-style');
    restoreHighlight(getDriver(), lease.target);
    if (!current(lease)) return;
    if (lease.correcting) return;
    if (lease.target && !lease.manualScroll && shouldScroll(lease.target)
      && signature !== lease.correctedSignature) {
      lease.correctedSignature = signature;
      lease.correcting = true;
      const controller = new AbortController();
      lease.correctionController = controller;
      const abort = () => controller.abort();
      lease.controller.signal.addEventListener('abort', abort, { once: true });
      const settled = () => {
        lease.controller.signal.removeEventListener('abort', abort);
        if (lease.correctionController === controller) lease.correctionController = null;
        lease.correcting = false;
      };
      Promise.resolve().then(() => {
        if (current(lease) && !controller.signal.aborted) return scrollTarget(lease.target, undefined, controller.signal);
      }).then(() => {
        settled();
        if (current(lease)) invalidate('scroll-settled');
      }, error => {
        settled();
        if (current(lease) && !controller.signal.aborted) onError(error);
      });
      return;
    }
    lease.reasons.clear();
    if (!changed && !correction) return;
    lease.signature = signature;
    if (changed) {
      applyShape(getDriver(), lease.target);
      if (!current(lease)) return;
      getDriver()?.refresh?.();
    }
    if (current(lease)) correctPopover(popover);
  }

  function observePopover(lease, popover) {
    if (popover === lease.popover) return;
    if (lease.popover) lease.resize?.unobserve?.(lease.popover);
    lease.popover = popover;
    if (popover) lease.resize?.observe(popover);
  }

  function listen(lease, node, event, callback, options) {
    if (!node?.addEventListener) return;
    const guarded = e => { if (current(lease)) callback(e); };
    node.addEventListener(event, guarded, options);
    lease.cleanups.push(() => node.removeEventListener(event, guarded, options));
  }

  function commit(frame) {
    if (disposed || frame.signal?.aborted || frame.isCurrent?.() === false) return;
    const view = frame.target?.ownerDocument?.defaultView
      || frame.scope?.ownerDocument?.defaultView || globalThis.window;
    if (!view) return;
    const expectedEpoch = epoch + 1;
    stop();
    // Aborting old effects can synchronously commit a newer lease or dispose us.
    if (disposed || epoch !== expectedEpoch || frame.signal?.aborted || frame.isCurrent?.() === false) return;
    const lease = { ...frame, view, epoch, controller: new AbortController(), raf: null,
      reasons: new Set(['commit']), cleanups: [], requested: new WeakSet(), signature: null,
      manualScroll: false, transitions: new Map(), missing: false, transformUntil: 0 };
    active = lease;
    const dirty = reason => { if (current(lease)) { lease.reasons.add(reason); schedule(lease); } };
    if (view.ResizeObserver) {
      lease.resize = new view.ResizeObserver(() => dirty('resize'));
      new Set([lease.target, lease.scope]).forEach(el => { if (el?.nodeType === 1) lease.resize.observe(el); });
    }
    if (view.MutationObserver && frame.scope) {
      lease.mutation = new view.MutationObserver(() => dirty('mutation'));
      // A stable parent also observes replacement of the scoped owner itself.
      lease.mutation.observe(frame.scope.parentElement || frame.scope, { childList: true, subtree: true,
        attributes: true, attributeFilter: ['class', 'style', 'hidden', 'inert', 'aria-hidden'] });
    }
    const doc = view.document;
    listen(lease, doc.fonts, 'loadingdone', () => dirty('popover-content'));
    listen(lease, doc, 'scroll', () => dirty('scroll'), true);
    listen(lease, view, 'resize', () => dirty('viewport'));
    for (const name of ['resize', 'scroll']) listen(lease, view.visualViewport, name, () => dirty('viewport'));
    const manual = () => {
      lease.manualScroll = true;
      lease.correctionController?.abort();
      dirty('manual-scroll');
    };
    for (const name of ['wheel', 'touchmove']) listen(lease, doc, name, manual, { passive: true, capture: true });
    listen(lease, doc, 'keydown', event => {
      if (event.defaultPrevented || event.target?.closest?.('input,textarea,[contenteditable="true"],button,[role="button"]')) return;
      if (['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End', ' '].includes(event.key)) manual();
    }, true);
    const transitionScope = doc;
    const related = e => !frame.scope || e.target?.contains?.(frame.scope) || frame.scope.contains?.(e.target);
    const transformProperty = e => ['transform', 'translate', 'scale', 'rotate'].includes(e.propertyName);
    listen(lease, transitionScope, 'transitionrun', e => {
      if (!related(e) || !transformProperty(e)) return;
      if (!lease.transitions.size || Date.now() >= lease.transformUntil) {
        lease.transitions.clear();
        lease.transformUntil = Date.now() + TRANSFORM_WINDOW_MS;
      }
      const properties = lease.transitions.get(e.target) || new Set();
      properties.add(e.propertyName); lease.transitions.set(e.target, properties);
      dirty('transform');
    }, true);
    for (const name of ['transitionend', 'transitioncancel']) {
      listen(lease, transitionScope, name, e => {
        if (!related(e) || !transformProperty(e)) return;
        const properties = lease.transitions.get(e.target);
        properties?.delete(e.propertyName);
        if (!properties?.size) lease.transitions.delete(e.target);
        dirty('transform-end');
      }, true);
    }
    for (const name of ['animationend', 'animationcancel']) {
      listen(lease, transitionScope, name, e => { if (related(e)) dirty('animation-end'); }, true);
    }
    const onAbort = () => { if (active === lease) stop(); };
    frame.signal?.addEventListener('abort', onAbort, { once: true });
    lease.cleanups.push(() => frame.signal?.removeEventListener('abort', onAbort));
    if (frame.signal?.aborted) { stop(); return; }
    schedule(lease);
  }

  function dispose() { disposed = true; stop(); }
  return { prepare, commit, invalidate, stop, dispose };
}
