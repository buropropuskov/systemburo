import { describe, it, expect, vi } from 'vitest';
import { useSwipeDismiss } from '../useSwipeDismiss';

// #1097 W3.4: свайп-вниз-закрытие bottom-sheet. Лист тянется только вниз, закрытие
// после порога; скролл-guard и ползунок разрешают/запрещают жест.

const touch = (y) => ({ touches: [{ clientY: y }], cancelable: true, preventDefault: vi.fn(), target: null });

describe('useSwipeDismiss', () => {
  it('свайп вниз дальше порога уводит лист вниз и закрывает после слайда', () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90 });
    s.onTouchStart(touch(100));
    s.onTouchMove({ ...touch(250), touches: [{ clientY: 250 }] });
    expect(s.isDragging.value).toBe(true);
    expect(s.offset.value).toBe(150);
    s.onTouchEnd();
    // Лист доезжает вниз (offset > свайпа), закрытие отложено до конца слайда.
    expect(s.isDragging.value).toBe(false);
    expect(s.closing.value).toBe(true);
    expect(s.offset.value).toBeGreaterThan(150);
    expect(onDismiss).not.toHaveBeenCalled();
    vi.advanceTimersByTime(300);
    expect(onDismiss).toHaveBeenCalledTimes(1);
    // Антирегресс R3-1: во время leave offset ДЕРЖИТ лист внизу (не рывок в 0),
    // иначе второй слайд. Полный reset - отложенно, после того как leave отыграл.
    expect(s.offset.value).toBeGreaterThan(150);
    expect(s.closing.value).toBe(true);
    vi.advanceTimersByTime(400);
    expect(s.offset.value).toBe(0);
    expect(s.closing.value).toBe(false);
    vi.useRealTimers();
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
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90, getScrollTop: () => 40, handleSelector: '.sheet-handle' });
    const handleEl = { closest: (sel) => (sel === '.sheet-handle' ? {} : null) };
    s.onTouchStart({ touches: [{ clientY: 100 }], target: handleEl, cancelable: true, preventDefault: vi.fn() });
    s.onTouchMove({ touches: [{ clientY: 250 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(150);
    s.onTouchEnd();
    vi.advanceTimersByTime(300);
    expect(onDismiss).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it('тач по полю ввода (textarea/input) НЕ активирует свайп - ввод/каретка не глотаются (#1097 R4-5)', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90 });
    // target внутри textarea: closest() матчит поле ввода.
    const fieldEl = { closest: (sel) => (sel.includes('textarea') ? {} : null) };
    const pd = vi.fn();
    s.onTouchStart({ touches: [{ clientY: 100 }], target: fieldEl, cancelable: true, preventDefault: vi.fn() });
    s.onTouchMove({ touches: [{ clientY: 250 }], cancelable: true, preventDefault: pd });
    // Лист не тянется, preventDefault не вызван (жест каретки/выделения не перехвачен).
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
    expect(pd).not.toHaveBeenCalled();
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('дребезг в мёртвой зоне (< slop) не перехватывает событие - не глотает тап/клик', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90, slop: 8 });
    s.onTouchStart(touch(100));
    const pd = vi.fn();
    s.onTouchMove({ touches: [{ clientY: 105 }], cancelable: true, preventDefault: pd }); // 5px < slop
    expect(s.isDragging.value).toBe(false);
    expect(s.offset.value).toBe(0);
    expect(pd).not.toHaveBeenCalled(); // click НЕ подавлен
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('после перехода порога slop тянет лист и вызывает preventDefault', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90, slop: 8 });
    s.onTouchStart(touch(100));
    const pd = vi.fn();
    s.onTouchMove({ touches: [{ clientY: 100 + 40 }], cancelable: true, preventDefault: pd }); // 40 > slop
    expect(s.isDragging.value).toBe(true);
    expect(s.offset.value).toBe(40);
    expect(pd).toHaveBeenCalled();
  });

  it('второй палец (мультитач) отменяет жест закрытия', () => {
    const onDismiss = vi.fn();
    const s = useSwipeDismiss(onDismiss, { threshold: 90 });
    s.onTouchStart(touch(100));
    s.onTouchMove({ touches: [{ clientY: 200 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(100);
    // появился второй палец
    s.onTouchMove({ touches: [{ clientY: 300 }, { clientY: 310 }], cancelable: true, preventDefault: vi.fn() });
    expect(s.offset.value).toBe(0);
    expect(s.isDragging.value).toBe(false);
    s.onTouchEnd();
    expect(onDismiss).not.toHaveBeenCalled();
  });
});
