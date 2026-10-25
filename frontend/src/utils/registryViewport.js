// The tablet navigation layout ends at the same 1366px breakpoint.
export const registryUsesPageScroll = (width) => width <= 1366;

export const registryScrollRoot = (body, width) => registryUsesPageScroll(width) ? null : body;
import { getViewportZoom } from '@/utils/viewportScale';


export function applyRegistryViewport(vm, sentinelTestId, reconnect) {
  const el = vm.$refs.root;
  if (!el) return;
  const pageScroll = registryUsesPageScroll(window.innerWidth);
  if (vm._pageScrollMode !== pageScroll) {
    vm._pageScrollMode = pageScroll;
    reconnect(el.querySelector?.(`[data-testid="${sentinelTestId}"]`) || null);
  }
  if (pageScroll) {
    el.style.height = '';
    vm._lastHeight = -1;
    return;
  }
  const height = Math.max(0, Math.round((window.innerHeight - el.getBoundingClientRect().top) / getViewportZoom()));
  if (height === vm._lastHeight) return;
  vm._lastHeight = height;
  el.style.height = `${height}px`;
}
