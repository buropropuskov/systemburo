import { hasModalAbove, setModalOpen, releaseModal, subscribeModalStack } from '@/utils/modalStack';
import { isTourTargetVisible } from './targetResolver';

export const TOUR_POPOVER_LAYER = 12502;

// Consent and the legacy dirty-confirm layer are not registered in modalStack.
const GLOBAL_OVERLAYS = '.pdc-overlay, .dirty-overlay, .confirm-overlay';

export function createTourLayerGuard(onChange) {
  const owner = {};
  let active = false, blocked = false, observer = null, unsubscribe = null;
  const check = () => {
    const next = active && (hasModalAbove(TOUR_POPOVER_LAYER)
      || [...document.querySelectorAll(GLOBAL_OVERLAYS)].some(isTourTargetVisible));
    if (next === blocked) return;
    blocked = next;
    onChange(blocked);
  };
  function deactivate() {
    active = false;
    observer?.disconnect(); observer = null;
    unsubscribe?.(); unsubscribe = null;
    releaseModal(owner);
    blocked = false;
  }
  function activate() {
    if (active) return;
    active = true;
    setModalOpen(owner, true, TOUR_POPOVER_LAYER);
    unsubscribe = subscribeModalStack(check);
    observer = new MutationObserver(check);
    observer.observe(document.body, { childList: true, subtree: true, attributes: true,
      attributeFilter: ['class', 'style', 'hidden', 'aria-hidden'] });
    check();
  }
  function preserveGlobalFocus(work) {
    const focused = document.activeElement;
    let above = false;
    for (let node = focused; node; node = node.parentElement) {
      if (Number.parseInt(getComputedStyle(node).zIndex, 10) > TOUR_POPOVER_LAYER) above = true;
    }
    const keep = above && !focused?.closest?.('.driver-popover');
    work();
    if (keep && focused.isConnected && document.activeElement !== focused) focused.focus({ preventScroll: true });
  }
  return { activate, deactivate, dispose: deactivate, isBlocked: () => blocked, preserveGlobalFocus };
}
