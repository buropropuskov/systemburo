import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { ensureInView } from '../targetWaits';

/**
 * Доводка цели до экрана. Главный случай - цель ВЫШЕ окна: блок согласования в
 * карточке заявки (456 px) не влезает в свой прокручиваемый контейнер (347 px), и
 * прежнее условие «видно целиком» было недостижимо - тур ждал потолок ожидания и
 * показывал шаг с подсветкой за краем окна (#2622).
 */

/**
 * Заглушка узла: rect отдаёт по текущему смещению, scrollIntoView сдвигает его
 * ровно как браузер - верхом к верху экрана либо по центру.
 *
 * @param {{ top: number, height: number, scrollable?: boolean }} init
 */
function узел({ top, height, scrollable = true }) {
  const state = { top, height, вызовы: [] };
  return {
    state,
    getBoundingClientRect: () => ({ top: state.top, bottom: state.top + state.height, height: state.height }),
    scrollIntoView: (opts) => {
      state.вызовы.push(opts.block);
      if (!scrollable) return;
      state.top = opts.block === 'start'
        ? 0
        : Math.round((window.innerHeight - state.height) / 2);
    },
  };
}

describe('ensureInView', () => {
  beforeEach(() => {
    window.innerHeight = 500;
    vi.stubGlobal('requestAnimationFrame', (cb) => setTimeout(cb, 0));
  });
  afterEach(() => vi.unstubAllGlobals());

  it('цель ниже экрана подводится по центру и ожидание заканчивается', async () => {
    const el = узел({ top: 700, height: 120 });
    await ensureInView(el);
    expect(el.state.вызовы[0]).toBe('center');
    expect(el.state.top).toBe(190);
  });

  it('цель выше окна подводится верхом - середину блока показывать нечестно', async () => {
    const el = узел({ top: 697, height: 456 });
    await ensureInView(el, 'center');
    expect(el.state.вызовы).toContain('start');
    expect(el.state.top).toBe(0);
  });

  it('цель на месте не трогаем, пока шаг сам не попросил', async () => {
    const el = узел({ top: 100, height: 120 });
    await ensureInView(el);
    expect(el.state.вызовы).toEqual([]);
    await ensureInView(el, 'center');
    expect(el.state.вызовы).toEqual(['center']);
  });

  it('недостижимая цель не вешает шаг дольше потолка', async () => {
    const el = узел({ top: 900, height: 120, scrollable: false });
    const начало = Date.now();
    await ensureInView(el, 'center');
    expect(Date.now() - начало).toBeLessThan(2500);
    expect(el.state.вызовы.length).toBeGreaterThan(1);
  });
});
