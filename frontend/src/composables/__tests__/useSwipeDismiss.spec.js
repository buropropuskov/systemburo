import { describe, it, expect, vi } from 'vitest';
import { useSwipeDismiss } from '../useSwipeDismiss';

// #1097 W3.4: свайп-вниз-закрытие bottom-sheet. Лист тянется только вниз, закрытие
// после порога; скролл-guard и ползунок разрешают/запрещают жест.

const touch = (y) => ({ touches: [{ clientY: y }], cancelable: true, preventDefault: vi.fn(), target: null });

describe('useSwipeDismiss', () => {
  it('свайп вниз дальше порога вызывает onDismiss', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90 });
    s.onTouchStart(touch(100));
    s.onTouchMove({ ...touch(250), touches: [{ clientY: 250 }] });
    expect(s.isDragging.value).toBe(true);
    expect(s.offset.value).toBe(150);
    s.onTouchEnd();
    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
  });

  it('свайп вниз не дальше порога возвращает лист, не закрывает', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90 });
    s.onTouchStart(touch(100));
    s.onTouchMove({ touches: [{ clientY: 150 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(50);
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
    expect(s.offset.value).toBe(0);
  });

  it('движение вверх не тянет лист', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss);
    s.onTouchStart(touch(200));
    s.onTouchMove({ touches: [{ clientY: 120 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('когда контент прокручен вниз (scrollTop>0), жест игнорируется (скролл, не закрытие)', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90, getScrollTop: () => 40 });
    s.onTouchStart(touch(100));
    s.onTouchMove({ touches: [{ clientY: 300 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(0);
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('жест с ползунка закрывает даже при прокрученном контенте', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90, getScrollTop: () => 40, handleSelector: '.sheet-handle' });
    const handleEl = { closest: (sel) => (sel === '.sheet-handle' ? {} : null) };
    s.onTouchStart({ touches: [{ clientY: 100 }], target: handleEl, cancelable: true, preventDefault: vi.fn() });
    s.onTouchMove({ touches: [{ clientY: 250 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(150);
    s.onTouchEnd();
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
