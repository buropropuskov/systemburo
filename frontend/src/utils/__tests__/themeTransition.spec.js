import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { REVEAL_DURATION, canReveal, originFromEvent, revealThemeChange } from '../themeTransition';

/**
 * Ставит заглушки View Transitions + WAAPI и отдаёт снятые аргументы.
 * jsdom не умеет ни того, ни другого, поэтому проверяем контракт вызовов:
 * что тема применяется внутри перехода, а клип открывается от нужной точки.
 */
function stubViewTransitions() {
  const finished = Promise.resolve();
  const animate = vi.fn(() => ({ finished }));
  const transitions = [];
  const start = vi.fn((callback) => {
    callback();
    const t = { ready: Promise.resolve(), skipTransition: vi.fn() };
    transitions.push(t);
    return t;
  });
  document.startViewTransition = start;
  document.documentElement.animate = animate;
  return { animate, start, transitions };
}

describe('originFromEvent', () => {
  it('берёт координаты курсора', () => {
    expect(originFromEvent({ clientX: 120, clientY: 340 })).toEqual({ x: 120, y: 340 });
  });

  it('для клика с клавиатуры берёт центр пункта', () => {
    const event = {
      clientX: 0,
      clientY: 0,
      currentTarget: { getBoundingClientRect: () => ({ left: 10, top: 20, width: 100, height: 40 }) },
    };
    expect(originFromEvent(event)).toEqual({ x: 60, y: 40 });
  });

  it('без события возвращает null', () => {
    expect(originFromEvent(undefined)).toBeNull();
  });
});

describe('revealThemeChange', () => {
  beforeEach(() => {
    delete document.startViewTransition;
    delete document.documentElement.animate;
    window.innerWidth = 1000;
    window.innerHeight = 800;
  });

  afterEach(() => {
    delete document.startViewTransition;
    delete document.documentElement.animate;
    delete window.matchMedia;
    vi.restoreAllMocks();
  });

  it('без поддержки API применяет тему сразу и синхронно', () => {
    const apply = vi.fn();
    expect(canReveal()).toBe(false);

    revealThemeChange(apply, { x: 10, y: 10 });

    expect(apply).toHaveBeenCalledTimes(1);
  });

  it('без точки клика не поднимает переход', async () => {
    const { start } = stubViewTransitions();
    const apply = vi.fn();

    await revealThemeChange(apply, null);

    expect(apply).toHaveBeenCalledTimes(1);
    expect(start).not.toHaveBeenCalled();
  });

  it('заливает новый кадр от точки клика до дальнего угла', async () => {
    const { animate, start } = stubViewTransitions();
    const apply = vi.fn();

    await revealThemeChange(apply, { x: 200, y: 600 });

    expect(start).toHaveBeenCalledTimes(1);
    expect(apply).toHaveBeenCalledTimes(1);

    const [keyframes, options] = animate.mock.calls[0];
    expect(options).toMatchObject({
      duration: REVEAL_DURATION,
      pseudoElement: '::view-transition-new(root)',
    });
    // Дальний угол от (200, 600) при вьюпорте 1000x800: 800 по X, 600 по Y.
    expect(keyframes.clipPath[0]).toBe('ellipse(0.0px 0.0px at 200px 600px)');
    expect(keyframes.clipPath.at(-1)).toBe('ellipse(800.0px 600.0px at 200px 600px)');
    // Промежуточные кадры строго растут - фронт не откатывается назад.
    const radii = keyframes.clipPath.map((f) => parseFloat(f.slice('ellipse('.length)));
    expect(radii.every((r, i) => i === 0 || r > radii[i - 1])).toBe(true);
  });

  it('повторный выбор обрывает незакрытую заливку', async () => {
    const { transitions } = stubViewTransitions();
    // Анимация «висит», пока её не отпустим: имитируем клик посреди заливки.
    const release = [];
    document.documentElement.animate = vi.fn(() => ({
      finished: new Promise((resolve) => { release.push(resolve); }),
    }));

    const first = revealThemeChange(vi.fn(), { x: 0, y: 0 });
    await Promise.resolve();
    const second = revealThemeChange(vi.fn(), { x: 50, y: 50 });
    await Promise.resolve();

    expect(transitions[0].skipTransition).toHaveBeenCalled();
    release.forEach((resolve) => resolve());
    await Promise.all([first, second]);
  });

  it('применяет тему, даже если переход не поднялся', async () => {
    stubViewTransitions();
    document.startViewTransition = vi.fn(() => { throw new Error('hidden document'); });
    const apply = vi.fn();

    await revealThemeChange(apply, { x: 10, y: 10 });

    expect(apply).toHaveBeenCalledTimes(1);
  });

  it('при prefers-reduced-motion переключает без анимации', async () => {
    const { start } = stubViewTransitions();
    window.matchMedia = vi.fn(() => ({ matches: true }));
    const apply = vi.fn();

    await revealThemeChange(apply, { x: 10, y: 10 });

    expect(apply).toHaveBeenCalledTimes(1);
    expect(start).not.toHaveBeenCalled();
  });
});
