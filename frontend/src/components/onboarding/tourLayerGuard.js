import { hasModalAbove, setModalOpen, releaseModal, subscribeModalStack } from '@/utils/modalStack';
import { isTourTargetVisible } from './targetResolver';

export const TOUR_POPOVER_LAYER = 12502;

// Legacy dialogs may not be registered above the tour in modalStack.
const GLOBAL_OVERLAYS = '.pdc-overlay, .dirty-overlay, .confirm-overlay, [data-testid="download-blanks-modal"]';

export function createTourLayerGuard(onChange, { getRevealOpen = () => null } = {}) {
  const owner = {};
  let active = false, blocked = false, observer = null, unsubscribe = null;
  const selectors = () => [GLOBAL_OVERLAYS, getRevealOpen() !== 'application-actions'
    && document.querySelector('.detail-actions-menu[data-dropdown-open="true"]')
    && '.base-dropdown__menu .detail-header-actions--compact'].filter(Boolean).join(', ');
  const check = () => {
    const next = active && (hasModalAbove(TOUR_POPOVER_LAYER)
      || [...document.querySelectorAll(selectors())].some(isTourTargetVisible));
    if (next === blocked) return;
    blocked = next;
    // A hidden tour must not retain Escape ownership over the user's dialog.
    setModalOpen(owner, !blocked, TOUR_POPOVER_LAYER);
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
      attributeFilter: ['class', 'style', 'hidden', 'aria-hidden', 'data-dropdown-open'] });
    check();
  }
  function preserveGlobalFocus(work) {
    const focused = document.activeElement;
    let above = false;
    for (let node = focused; node; node = node.parentElement) {
      if (Number.parseInt(getComputedStyle(node).zIndex, 10) > TOUR_POPOVER_LAYER) above = true;
    }
    const inDialog = focused?.closest?.(selectors());
    const keep = (above || inDialog) && !focused?.closest?.('.driver-popover');
    work();
    if (keep && focused.isConnected && document.activeElement !== focused) focused.focus({ preventScroll: true });
  }
  return { activate, deactivate, dispose: deactivate, isBlocked: () => blocked, preserveGlobalFocus };
}
