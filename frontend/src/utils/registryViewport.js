// The tablet navigation layout ends at the same 1366px breakpoint.
export const registryUsesPageScroll = (width) => width <= 1366;

export const registryScrollRoot = (body, width) => registryUsesPageScroll(width) ? null : body;
import { getViewportZoom } from '@/utils/viewportScale';
import { PAGE_LAYOUT_READY } from '@/utils/pageLayout';

export function attachRegistryViewport(vm) {
  vm._lastHeight = -1;
  vm.$nextTick(vm._applyHeight);
  window.addEventListener('resize', vm._applyHeight);
  window.addEventListener(PAGE_LAYOUT_READY, vm._applyHeight);
  const header = document.querySelector('.theheader');
  if (header && typeof ResizeObserver !== 'undefined') {
    vm._headerObs = new ResizeObserver(vm._applyHeight);
    vm._headerObs.observe(header);
  }
}

export function detachRegistryViewport(vm) {
  window.removeEventListener('resize', vm._applyHeight);
  window.removeEventListener(PAGE_LAYOUT_READY, vm._applyHeight);
  vm._headerObs?.disconnect();
  vm._headerObs = null;
}


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
