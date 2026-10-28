// Both hooks matter: entering and leaving routes can finish in either order.
export const PAGE_LAYOUT_READY = 'page-layout-ready';

export function notifyPageLayout() {
  window.dispatchEvent(new Event(PAGE_LAYOUT_READY));
}
