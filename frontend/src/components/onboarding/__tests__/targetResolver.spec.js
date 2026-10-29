import { afterEach, describe, expect, it, vi } from 'vitest';
import { findTourTarget, getTourTargets, isTourTargetVisible } from '../targetResolver';
import { adminOnboardingSteps } from '../adminOnboardingSteps';
import { acceptOnboardingSteps } from '../acceptOnboardingSteps';

// jsdom has no layout. Rects model CSS boxes without treating visibility/opacity,
// aria-hidden or inert as display:none. Fixed controls legitimately have no offsetParent.
function target(parent = document.body, { top = 100, width = 80, height = 32, fixed = false } = {}) {
  const element = document.createElement('button');
  element.dataset.tour = 'target';
  Object.assign(element.style, { position: fixed ? 'fixed' : 'absolute', top: top + 'px', left: '20px', width: width + 'px', height: height + 'px' });
  const hasNoBox = () => {
    for (let node = element; node; node = node.parentElement) {
      if (node.hasAttribute('hidden') || getComputedStyle(node).display === 'none') return true;
    }
    return false;
  };
  Object.defineProperty(element, 'offsetParent', { get: () => fixed || hasNoBox() ? null : parent });
  element.getBoundingClientRect = () => {
    const style = getComputedStyle(element), noBox = hasNoBox();
    const actualWidth = noBox ? 0 : parseFloat(style.width), actualHeight = noBox ? 0 : parseFloat(style.height);
    const y = noBox ? 0 : parseFloat(style.top), x = noBox ? 0 : 20;
    return { x, y, left: x, top: y, width: actualWidth, height: actualHeight, right: x + actualWidth, bottom: y + actualHeight };
  };
  parent.append(element);
  return element;
}

afterEach(() => {
  vi.restoreAllMocks(); vi.unstubAllGlobals();
  document.body.replaceChildren();
});

const selector = '[data-tour="target"]';
it('keeps bulk assignment reachable through the actual phone and desktop controls', () => {
  const desktop = target(); desktop.dataset.testid = 'attachment-assign-all-places';
  const mobile = target(); mobile.dataset.testid = 'attachment-assign-all-open';
  const step = acceptOnboardingSteps.find(item => item.id === 'acc-detail-assign-all');
  desktop.style.display = 'none';
  expect(findTourTarget(step.element)).toBe(mobile);
  desktop.style.display = ''; mobile.style.display = 'none';
  expect(findTourTarget(step.element)).toBe(desktop);
});
it('resolves the actual settings step to its visible responsive section selector', () => {
  const desktop = target(); desktop.dataset.testid = 'ob-admin-settings'; desktop.style.display = 'none';
  const mobile = target(); mobile.dataset.testid = 'ob-admin-settings';
  const step = adminOnboardingSteps.find(item => item.id === 'admin-settings');
  expect(findTourTarget(step.element)).toBe(mobile);
  desktop.style.display = ''; mobile.style.display = 'none';
  expect(findTourTarget(step.element)).toBe(desktop);
});
const hiddenStates = [
  { name: 'display none', apply: node => { node.style.display = 'none'; } },
  { name: 'visibility hidden', apply: node => { node.style.visibility = 'hidden'; } },
  { name: 'visibility collapse', apply: node => { node.style.visibility = 'collapse'; } },
  { name: 'opacity zero', apply: node => { node.style.opacity = '0'; } },
  { name: 'hidden attribute', apply: node => { node.setAttribute('hidden', ''); } },
  { name: 'inert attribute', apply: node => { node.setAttribute('inert', ''); } },
  { name: 'aria-hidden true', apply: node => { node.setAttribute('aria-hidden', 'true'); } },
];

describe('logical visibility of a tour target', () => {
  it('accepts a connected box and uses its document style reader', () => {
    const element = target(), readStyle = vi.spyOn(element.ownerDocument.defaultView, 'getComputedStyle');
    expect(isTourTargetVisible(element)).toBe(true);
    expect(readStyle).toHaveBeenCalledWith(element);
    expect(readStyle).toHaveBeenCalledWith(document.body);
  });

  it('rejects absent values and a disconnected box', () => {
    expect(isTourTargetVisible(null)).toBe(false);
    expect(isTourTargetVisible(undefined)).toBe(false);
    const element = target(); element.remove();
    expect(isTourTargetVisible(element)).toBe(false);
  });

  it.each([{ width: 0 }, { height: 0 }])('rejects zero-size geometry %j', size => {
    expect(isTourTargetVisible(target(document.body, size))).toBe(false);
  });

  describe.each(['self', 'ancestor'])('hidden on %s', location => {
    it.each(hiddenStates)('excludes $name', state => {
      const parent = document.createElement('section'); document.body.append(parent);
      const element = target(parent);
      state.apply(location === 'self' ? element : parent);
      expect(isTourTargetVisible(element)).toBe(false);
      expect(getTourTargets(selector)).toEqual([]);
    });
  });

  it('accepts default empty opacity instead of interpreting it as numeric zero', () => {
    const element = target();
    expect(element.style.opacity).toBe('');
    expect(isTourTargetVisible(element)).toBe(true);
  });

  it('accepts disabled controls and pointer-events none as instructional targets', () => {
    const element = target(); element.disabled = true; element.style.pointerEvents = 'none';
    expect(isTourTargetVisible(element)).toBe(true);
  });

  it('accepts a normal below-fold target and a visible fixed control without offsetParent', () => {
    const below = target(document.body, { top: 2000 });
    const fixed = target(document.body, { fixed: true, top: 12 });
    expect(fixed.offsetParent).toBeNull();
    expect(isTourTargetVisible(below)).toBe(true);
    expect(isTourTargetVisible(fixed)).toBe(true);
    expect(getTourTargets(selector)).toEqual([below, fixed]);
  });
});

describe('target selection within its explicit root', () => {
  it('chooses the second rendered HTMLElement when the first desktop copy is display none', () => {
    const desktop = target(), rendered = target(); desktop.style.display = 'none';
    expect(desktop.getBoundingClientRect().width).toBe(0);
    expect(getTourTargets(selector)).toEqual([rendered]);
    expect(findTourTarget(selector)).toBe(rendered);
  });

  it('keeps the first DOM match even when another target is nearer to the viewport', () => {
    const first = target(document.body, { top: 2000 }); target();
    expect(findTourTarget(selector)).toBe(first);
  });

  it('keeps an eligible preferred match and falls back when it becomes hidden', () => {
    const first = target(), preferred = target();
    expect(findTourTarget(selector, { preferred })).toBe(preferred);
    preferred.style.opacity = '0';
    expect(findTourTarget(selector, { preferred })).toBe(first);
  });

  it('replaces a detached preferred node with the new actual match', () => {
    const previous = target(); previous.remove(); const replacement = target();
    expect(findTourTarget(selector, { preferred: previous })).toBe(replacement);
  });

  it('does not keep a preferred node outside the root or selector', () => {
    const external = target(), root = document.createElement('section'); document.body.append(root);
    const inside = target(root); const unrelated = document.createElement('button'); root.append(unrelated);
    expect(getTourTargets(selector, root)).toEqual([inside]);
    expect(findTourTarget(selector, { root, preferred: external })).toBe(inside);
    expect(findTourTarget(selector, { root, preferred: unrelated })).toBe(inside);
  });

  it('returns an empty list and null for an absent SSR root', () => {
    expect(getTourTargets(selector, null)).toEqual([]);
    expect(findTourTarget(selector, { root: null })).toBeNull();
    vi.stubGlobal('document', undefined);
    expect(getTourTargets(selector)).toEqual([]);
    expect(findTourTarget(selector)).toBeNull();
  });

  it('does not hide invalid selector configuration errors', () => {
    expect(() => getTourTargets('[')).toThrowError(expect.objectContaining({ name: 'SyntaxError' }));
    expect(() => findTourTarget('[')).toThrowError(expect.objectContaining({ name: 'SyntaxError' }));
  });
});
