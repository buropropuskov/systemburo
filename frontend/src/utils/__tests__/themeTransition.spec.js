import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { getViewportZoom } from '@/utils/viewportScale';
import { REVEAL_DURATION, canReveal, originFromEvent, revealThemeChange } from '../themeTransition';

// Корневой zoom на широких экранах меняет координатное пространство - подменяем.
vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: vi.fn(() => 1) }));

/**
 * Ставит заглушки View Transitions + WAAPI и отдаёт снятые аргументы.
 * jsdom не умеет ни того, ни другого, поэтому проверяем контракт вызовов:
 * что тема применяется внутри перехода, а кадры заливки геометрически верны.
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

/** Кадры анимации по имени свойства - их две: контур и маска. */
function framesOf(animate, prop) {
  const call = animate.mock.calls.find(([kf]) => kf[prop]);
  return call ? { frames: call[0][prop], options: call[1] } : null;
}

/** Точки одного кадра polygon() в виде [{x, y}]. */
function pointsOf(frame) {
  return frame
    .replace(/^polygon\(|\)$/g, '')
    .split(',')
    .map((pair) => {
      const [x, y] = pair.trim().split(/\s+/).map((v) => parseFloat(v));
      return { x, y };
    });
}

/** Лежит ли точка внутри контура (трассировка луча) - замок на покрытие углов. */
function contains(points, x, y) {
  let inside = false;
  for (let i = 0, j = points.length - 1; i < points.length; j = i, i += 1) {
    const a = points[i];
    const b = points[j];
    const crosses = a.y > y !== b.y > y;
    if (crosses && x < ((b.x - a.x) * (y - a.y)) / (b.y - a.y) + a.x) inside = !inside;
  }
  return inside;
}

/** Наибольшее расстояние от точки клика до контура - «размах» кадра. */
function maxRadius(points, origin) {
  return Math.max(...points.map((p) => Math.hypot(p.x - origin.x, p.y - origin.y)));
}

/** Правая (ведущая) кромка кадра - первая половина точек. */
function frontEdge(frame) {
  const pts = pointsOf(frame);
  return pts.slice(0, pts.length / 2);
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
  const origin = { x: 200, y: 600 };

  beforeEach(() => {
    delete document.startViewTransition;
    delete document.documentElement.animate;
    document.documentElement.className = '';
    window.innerWidth = 1000;
    window.innerHeight = 800;
  });

  afterEach(() => {
    getViewportZoom.mockReturnValue(1);
    delete document.startViewTransition;
    delete document.documentElement.animate;
    delete window.matchMedia;
    document.documentElement.className = '';
    vi.restoreAllMocks();
  });

  it('без поддержки API применяет тему сразу и синхронно', () => {
    const apply = vi.fn();
    expect(canReveal()).toBe(false);

    revealThemeChange(apply, origin);

    expect(apply).toHaveBeenCalledTimes(1);
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false);
  });

  it('без точки клика не поднимает переход', async () => {
    const { start } = stubViewTransitions();
    const apply = vi.fn();

    await revealThemeChange(apply, null);

    expect(apply).toHaveBeenCalledTimes(1);
    expect(start).not.toHaveBeenCalled();
  });

  it('ведёт одну анимацию контура на новом кадре', async () => {
    const { animate, start } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    expect(start).toHaveBeenCalledTimes(1);
    // Одна анимация: маску убрали - размытая кромка читалась как смазанное пятно.
    expect(animate).toHaveBeenCalledTimes(1);
    const contour = framesOf(animate, 'clipPath');
    expect(contour).not.toBeNull();
    expect(contour.options).toMatchObject({
      duration: REVEAL_DURATION,
      pseudoElement: '::view-transition-new(root)',
    });
    expect(contour.frames[0].startsWith('polygon(')).toBe(true);
  });

  it('стартует узкой полосой у курсора во всю высоту', async () => {
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    const first = pointsOf(framesOf(animate, 'clipPath').frames[0]);
    const xs = first.map((p) => p.x);
    const ys = first.map((p) => p.y);
    // Полоса узкая (волна + запас), но сразу от верха до низа экрана.
    expect(Math.max(...xs) - Math.min(...xs)).toBeLessThan(260);
    expect(Math.min(...ys)).toBeCloseTo(0, 0);
    expect(Math.max(...ys)).toBeCloseTo(800, 0);
    expect(Math.abs((Math.max(...xs) + Math.min(...xs)) / 2 - origin.x)).toBeLessThan(120);
  });

  it('ведёт фронт слева направо через весь экран', async () => {
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    const frames = framesOf(animate, 'clipPath').frames;
    const fronts = frames.map((f) => {
      const edge = frontEdge(f);
      return edge.reduce((sum, p) => sum + p.x, 0) / edge.length;
    });
    expect(fronts.every((x, i) => i === 0 || x > fronts[i - 1]), 'фронт должен только идти вперёд').toBe(true);
    expect(Math.abs(fronts[0] - origin.x)).toBeLessThan(60);
    expect(fronts.at(-1)).toBeGreaterThan(1000);
  });

  it('держит на кромке волну, а не прямую линию', async () => {
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    const frames = framesOf(animate, 'clipPath').frames;
    const edge = frontEdge(frames[Math.round(frames.length * 0.4)]).map((p) => p.x);
    const swing = Math.max(...edge) - Math.min(...edge);
    expect(swing, 'кромка плоская - волны не видно').toBeGreaterThan(25);
  });

  it('считает кадры в layout-px, а не device-px (корневой zoom)', async () => {
    // На широких экранах корень масштабируется CSS zoom: без деления фигура
    // выходит в zoom раз больше экрана и волна пробегает его мгновенно.
    getViewportZoom.mockReturnValue(2);
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), originFromEvent({ clientX: 400, clientY: 600 }));

    const frames = framesOf(animate, 'clipPath').frames;
    const first = pointsOf(frames[0]);
    // clientX 400 при zoom 2 - это 200 в layout-px.
    expect(Math.abs((Math.max(...first.map((p) => p.x)) + Math.min(...first.map((p) => p.x))) / 2 - 200))
      .toBeLessThan(120);
    // Экран 1000x800 device -> 500x400 layout: кадры не должны выходить далеко за 500.
    const lastMax = Math.max(...pointsOf(frames.at(-1)).map((p) => p.x));
    expect(lastMax).toBeGreaterThan(500);
    expect(lastMax).toBeLessThan(700);
    expect(Math.max(...first.map((p) => p.y))).toBeCloseTo(400, 0);
    getViewportZoom.mockReturnValue(1);
  });

  it('последним кадром накрывает все четыре угла вьюпорта', async () => {
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    const last = pointsOf(framesOf(animate, 'clipPath').frames.at(-1));
    const corners = [[0, 0], [999, 0], [0, 799], [999, 799]];
    for (const [x, y] of corners) {
      expect(contains(last, x, y), `угол ${x},${y} остался незалитым`).toBe(true);
    }
  });

  it('область заливки только растёт - назад не отступает', async () => {
    const { animate } = stubViewTransitions();

    await revealThemeChange(vi.fn(), origin);

    const radii = framesOf(animate, 'clipPath').frames.map((f) => maxRadius(pointsOf(f), origin));
    expect(radii.every((r, i) => i === 0 || r > radii[i - 1])).toBe(true);
  });

  it('на время заливки метит корень классом и снимает его после', async () => {
    // Обе анимации (контур и маска) висят, пока их не отпустим.
    const release = [];
    stubViewTransitions();
    document.documentElement.animate = vi.fn(() => ({
      finished: new Promise((resolve) => { release.push(resolve); }),
    }));

    const pending = revealThemeChange(vi.fn(), origin);
    await Promise.resolve();
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(true);

    release.forEach((resolve) => resolve());
    await pending;
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false);
  });

  it('повторный выбор обрывает незакрытую заливку', async () => {
    const { transitions } = stubViewTransitions();
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

    await revealThemeChange(apply, origin);

    expect(apply).toHaveBeenCalledTimes(1);
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false);
  });

  it('при prefers-reduced-motion переключает без анимации', async () => {
    const { start } = stubViewTransitions();
    window.matchMedia = vi.fn(() => ({ matches: true }));
    const apply = vi.fn();

    await revealThemeChange(apply, origin);

    expect(apply).toHaveBeenCalledTimes(1);
    expect(start).not.toHaveBeenCalled();
  });
});
