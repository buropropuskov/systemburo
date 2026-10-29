import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPopoverZoomFix } from '../popoverZoom';
const { zoom } = vi.hoisted(() => ({ zoom: { value: 1 } }));
vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => zoom.value }));
const fixes = [];
function wrapper({ left = 0, top = 80, right, bottom, width = 358, height = 200, touch = true, gutter = 16 } = {}) {
  const node = document.createElement('div');
  node.classList.toggle('ob-touch', touch);
  node.style.left = `${left}px`; node.style.top = `${top}px`;
  if (right !== undefined) { node.style.left = 'auto'; node.style.right = `${right}px`; }
  if (bottom !== undefined) { node.style.top = 'auto'; node.style.bottom = `${bottom}px`; }
  node.style.setProperty('--ob-touch-gutter', `${gutter / zoom.value}px`);
  node.getBoundingClientRect = () => {
    const x = Number.isFinite(parseFloat(node.style.left)) ? parseFloat(node.style.left) * zoom.value
      : innerWidth - parseFloat(node.style.right) * zoom.value - width;
    const y = Number.isFinite(parseFloat(node.style.top)) ? parseFloat(node.style.top) * zoom.value
      : innerHeight - parseFloat(node.style.bottom) * zoom.value - height;
    return { x, y, left: x, top: y, right: x + width, bottom: y + height, width, height };
  };
  document.body.append(node); return node;
}
function apply(node) { const fix = createPopoverZoomFix(); fixes.push(fix); fix.apply(node); return fix; }
beforeEach(() => {
  zoom.value = 1;
  vi.stubGlobal('innerWidth', 390); vi.stubGlobal('innerHeight', 844);
});
afterEach(() => { fixes.splice(0).forEach(fix => fix.detach()); document.body.replaceChildren(); vi.unstubAllGlobals(); });
describe('single-owner touch popover bounds', () => {
  it('corrects the first Driver position in the mutation microtask before the next paint', async () => {
    zoom.value = 1.4289;
    const node = wrapper({ touch: false, left: 0 });
    const fix = createPopoverZoomFix(); fixes.push(fix); fix.attach(node);
    // Driver positions only after onPopoverRender has returned.
    node.style.left = '1000px';
    await Promise.resolve();
    expect(parseFloat(node.style.left)).toBe(Math.round(1000 / zoom.value));
    node.style.left = '900px';
    await Promise.resolve();
    expect(parseFloat(node.style.left)).toBe(Math.round(900 / zoom.value));
  });
  it('keeps a real 16px gutter on a phone without changing its width or target', () => {
    const node = wrapper(); apply(node);
    expect(node.getBoundingClientRect()).toMatchObject({ left: 16, right: 374 });
    expect(node.style.width).toBe('');
  });
  it('clamps the right and bottom edges to the visual viewport', () => {
    vi.stubGlobal('visualViewport', { width: 390, height: 500, offsetLeft: 20, offsetTop: 100 });
    const node = wrapper({ left: 100, top: 550, width: 300, height: 180 }); apply(node);
    expect(node.getBoundingClientRect()).toMatchObject({ left: 94, right: 394, top: 404, bottom: 584 });
  });
  it('converts a tablet gutter to CSS pixels after correcting root zoom', () => {
    zoom.value = 2; vi.stubGlobal('innerWidth', 1024); vi.stubGlobal('innerHeight', 768);
    const node = wrapper({ left: 0, width: 976, gutter: 24 }); apply(node);
    expect(node.style.left).toBe('12px');
    expect(node.getBoundingClientRect()).toMatchObject({ left: 24, right: 1000 });
  });
  it('does not accumulate correction when the same frame is measured again', () => {
    const node = wrapper(); const fix = apply(node);
    fix.apply(node); fix.apply(node);
    expect(node.style.left).toBe('16px');
  });
  it('leaves the desktop position and a hidden touch wrapper unchanged', () => {
    const desktop = wrapper({ touch: false }); apply(desktop); expect(desktop.style.left).toBe('0px');
    const hidden = wrapper({ width: 0, height: 0 }); apply(hidden); expect(hidden.style.left).toBe('0px');
  });
  it('preserves Driver right/bottom anchors while clamping at root zoom', () => {
    zoom.value = 2; vi.stubGlobal('innerWidth', 1024); vi.stubGlobal('innerHeight', 768);
    const node = wrapper({ right: 0, bottom: 0, width: 976, gutter: 24 }); const fix = apply(node);
    expect(node.style.left).toBe('auto'); expect(node.style.top).toBe('auto');
    expect(node.style.right).toBe('12px'); expect(node.style.bottom).toBe('12px');
    expect(node.getBoundingClientRect()).toMatchObject({ left: 24, right: 1000, bottom: 744 });
    fix.apply(node);
    expect(node.style.right).toBe('12px'); expect(node.style.bottom).toBe('12px');
  });
  it('clamps a bottom-anchored no-fit fallback to an offset visual viewport', () => {
    vi.stubGlobal('visualViewport', { width: 390, height: 500, offsetLeft: 20, offsetTop: 100 });
    const node = wrapper({ right: 0, bottom: 0, width: 300, height: 180 }); apply(node);
    expect(node.getBoundingClientRect()).toMatchObject({ bottom: 584, top: 404 });
    expect(node.style.top).toBe('auto'); expect(node.style.bottom).toBe('260px');
  });
  it('corrects an out-of-bounds destination even while CSS still displays the old valid position', () => {
    const node = wrapper();
    node.getBoundingClientRect = () => {
      const left = node.classList.contains('ob-popover--instant') ? parseFloat(node.style.left) : 16;
      return { left, top: 80, width: 358, height: 200, right: left + 358, bottom: 280 };
    };
    vi.stubGlobal('getComputedStyle', element => ({
      left: element.classList.contains('ob-popover--instant') ? element.style.left : '16px',
      right: 'auto', top: '80px', bottom: 'auto',
      getPropertyValue: name => element.style.getPropertyValue(name),
    }));
    apply(node);
    expect(node.style.left).toBe('16px');
  });
});
