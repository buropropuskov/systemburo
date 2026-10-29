const clips = value => /^(auto|scroll|hidden|clip|overlay)$/.test(value);
const box = rect => ({ top: rect.top, left: rect.left, right: rect.left + rect.width,
  bottom: rect.top + rect.height, width: rect.width, height: rect.height });

/** Rects use getBoundingClientRect coordinates throughout, including CSS zoom. */
export function targetViewingArea(element) {
  const view = element?.ownerDocument?.defaultView || globalThis.window;
  const viewport = view?.visualViewport;
  const area = { left: Math.max(0, viewport?.offsetLeft || 0), top: Math.max(0, viewport?.offsetTop || 0),
    right: Math.min(view.innerWidth, (viewport?.offsetLeft || 0) + (viewport?.width || view.innerWidth)),
    bottom: Math.min(view.innerHeight, (viewport?.offsetTop || 0) + (viewport?.height || view.innerHeight)) };
  for (let node = element?.parentElement; node && node !== view.document.body && node !== view.document.documentElement; node = node.parentElement) {
    const style = view.getComputedStyle(node);
    const clipX = clips(style.overflowX || style.overflow), clipY = clips(style.overflowY || style.overflow);
    if (!clipX && !clipY) continue;
    const rect = box(node.getBoundingClientRect());
    const sx = node.offsetWidth > 0 ? rect.width / node.offsetWidth : 1;
    const sy = node.offsetHeight > 0 ? rect.height / node.offsetHeight : 1;
    const left = rect.left + node.clientLeft * sx, top = rect.top + node.clientTop * sy;
    const right = node.offsetWidth > 0 ? left + node.clientWidth * sx : rect.right;
    const bottom = node.offsetHeight > 0 ? top + node.clientHeight * sy : rect.bottom;
    if (clipX) { area.left = Math.max(area.left, left); area.right = Math.min(area.right, right); }
    if (clipY) { area.top = Math.max(area.top, top); area.bottom = Math.min(area.bottom, bottom); }
  }
  area.width = Math.max(0, area.right - area.left); area.height = Math.max(0, area.bottom - area.top);
  return area;
}

export function targetFitsViewingArea(element) {
  if (!element?.getBoundingClientRect || element.isConnected === false) return false;
  const rect = box(element.getBoundingClientRect()), area = targetViewingArea(element), epsilon = 1;
  if (!(rect.width > 0 && rect.height > 0 && area.width > 0 && area.height > 0)) return false;
  if (rect.left < area.left - epsilon || rect.right > area.right + epsilon) return false;
  if (rect.height > area.height - 48) {
    return rect.top >= area.top - epsilon && rect.top + Math.min(120, rect.height) <= area.bottom + epsilon;
  }
  return rect.top >= area.top - epsilon && rect.bottom <= area.bottom + epsilon;
}

/** One small correction after native scrolling, not an animation-frame scroll loop. */
export function correctTargetScroll(element) {
  const rect = box(element.getBoundingClientRect()), area = targetViewingArea(element);
  const dx = rect.left < area.left ? rect.left - area.left : Math.max(0, rect.right - area.right);
  const dy = rect.height > area.height - 48 ? rect.top - area.top
    : rect.top < area.top ? rect.top - area.top : Math.max(0, rect.bottom - area.bottom);
  if (Math.abs(dx) <= 1 && Math.abs(dy) <= 1) return;
  const view = element.ownerDocument?.defaultView || window;
  for (let node = element.parentElement; node && node !== view.document.body && node !== view.document.documentElement; node = node.parentElement) {
    const style = view.getComputedStyle(node), bounds = node.getBoundingClientRect();
    const sx = node.offsetWidth > 0 ? bounds.width / node.offsetWidth : 1;
    const sy = node.offsetHeight > 0 ? bounds.height / node.offsetHeight : 1;
    const x = clips(style.overflowX || style.overflow) && node.scrollWidth > node.clientWidth ? dx / sx : 0;
    const y = clips(style.overflowY || style.overflow) && node.scrollHeight > node.clientHeight ? dy / sy : 0;
    if (x || y) { node.scrollBy?.({ left: x, top: y, behavior: 'instant' }); return; }
  }
  // No scrollable ancestor: the visual viewport may be smaller than layout after focus.
  if (view.document.scrollingElement?.scrollHeight > view.innerHeight
    || view.document.scrollingElement?.scrollWidth > view.innerWidth) {
    const root = view.document.documentElement;
    const zoom = parseFloat(view.getComputedStyle(root).zoom) || 1;
    view.scrollBy({ left: dx / zoom, top: dy / zoom, behavior: 'instant' });
  }
}
