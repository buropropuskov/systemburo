import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ensureInView } from '../targetWaits';

describe('отмена доводки цели не продолжает прокрутку закрытого тура', () => {
  let element;
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', (callback) => setTimeout(callback, 16));
    vi.stubGlobal('cancelAnimationFrame', (id) => clearTimeout(id));
    element = { getBoundingClientRect: () => ({ top: 2000, bottom: 2100, height: 100 }),
      scrollIntoView: vi.fn() };
  });
  afterEach(() => { vi.clearAllTimers(); vi.useRealTimers(); vi.unstubAllGlobals(); });

  it('уже отменённая подготовка не прокручивает страницу', async () => {
    const controller = new AbortController();
    controller.abort();
    await ensureInView(element, 'center', controller.signal);
    expect(element.scrollIntoView).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('отмена снимает ожидающий frame и обработчик, повторных scroll нет', async () => {
    const controller = new AbortController();
    const remove = vi.spyOn(controller.signal, 'removeEventListener');
    const waiting = ensureInView(element, 'center', controller.signal);
    expect(element.scrollIntoView).toHaveBeenCalledTimes(1);
    controller.abort();
    await waiting;
    await vi.advanceTimersByTimeAsync(2000);
    expect(element.scrollIntoView).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
    expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
  });
});
