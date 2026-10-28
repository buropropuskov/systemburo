/** CSS/semantic visibility only: a target below the viewport can still be scrolled to. */
export function isTourTargetVisible(element) {
  if (element?.nodeType !== 1 || !element.isConnected) return false;
  const view = element.ownerDocument?.defaultView;
  if (!view?.getComputedStyle) return false;
  const rect = element.getBoundingClientRect();
  if (!(rect.width > 0 && rect.height > 0)) return false;

  for (let node = element; node; node = node.parentElement) {
    if (node.hasAttribute('hidden') || node.hasAttribute('inert') || node.inert === true
      || node.getAttribute('aria-hidden')?.trim().toLowerCase() === 'true') return false;
    const style = view.getComputedStyle(node);
    const opacity = style.opacity.trim();
    if (style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse'
      || (opacity !== '' && Number(opacity) === 0)) return false;
  }
  return true;
}

/** Keep DOM order; an invalid selector deliberately remains a configuration error. */
export function getTourTargets(selector, root = globalThis.document) {
  if (!root?.querySelectorAll) return [];
  return [...root.querySelectorAll(selector)].filter(isTourTargetVisible);
}

/** Preserve an eligible preferred node, otherwise use the first rendered match. */
export function findTourTarget(selector, { root = globalThis.document, preferred = null } = {}) {
  const targets = getTourTargets(selector, root);
  return targets.includes(preferred) ? preferred : (targets[0] || null);
}
