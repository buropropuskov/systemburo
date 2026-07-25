import { describe, it, expect } from 'vitest';
import {
  measureItemStep, fitWholeItems, wholeItemsHeight, measureChromeHeight,
} from '@/utils/dropdownMetrics';

// Высота выпадающего списка должна быть кратна пункту: с произвольной max-height
// последний пункт обрывается по середине строки.

// offsetTop/offsetHeight, а не rect: меню открывается с анимацией, и rect в этот
// момент отдаёт размеры под трансформацией.
const makeItems = (count, step, height = step) => Array.from({ length: count }, (_, i) => ({
  offsetTop: i * step,
  offsetHeight: height,
}));

const makeBox = (clientHeight) => ({ clientHeight });

describe('measureItemStep', () => {
  it('берёт шаг из расстояния между верхами соседних пунктов', () => {
    // Высота пункта 36, но шаг сетки 37 (border/зазор) - ограничивать надо по шагу.
    expect(measureItemStep(makeItems(5, 37, 36))).toBe(37);
  });

  it('для единственного пункта берёт его высоту', () => {
    expect(measureItemStep(makeItems(1, 40))).toBe(40);
  });

  it('пустой список даёт 0', () => {
    expect(measureItemStep([])).toBe(0);
    expect(measureItemStep(null)).toBe(0);
  });

  it('нулевой шаг (пункты друг на друге) откатывается на высоту пункта', () => {
    expect(measureItemStep(makeItems(3, 0, 30))).toBe(30);
  });

  it('игнорирует трансформацию анимации: считает по layout-метрикам', () => {
    // rect во время анимации отдал бы 35.15 вместо 37 - берём offsetTop и не зависим от неё
    const animating = makeItems(5, 37).map((i) => ({
      ...i,
      getBoundingClientRect: () => ({ height: 35.15, top: 0, bottom: 35.15 }),
    }));
    expect(measureItemStep(animating)).toBe(37);
  });
});

describe('fitWholeItems', () => {
  it('отбрасывает остаток: 250 при шаге 37 -> 6 пунктов', () => {
    expect(fitWholeItems(250, 37)).toBe(6 * 37);
  });

  it('не округляет до целого пикселя - дробный шаг сохраняется', () => {
    expect(fitWholeItems(200, 36.5)).toBeCloseTo(5 * 36.5, 5);
  });

  it('в тесном месте оставляет минимум пунктов', () => {
    expect(fitWholeItems(10, 37)).toBe(2 * 37);
  });
});

describe('wholeItemsHeight', () => {
  it('обрезает доступную высоту до целых пунктов', () => {
    // списку досталось 281px при шаге 37 -> 7 пунктов (259), а не 7.6
    expect(wholeItemsHeight(makeBox(281), makeItems(13, 37), 281)).toBe(7 * 37);
  });

  it('не ограничивает, когда список помещается целиком', () => {
    expect(wholeItemsHeight(makeBox(300), makeItems(5, 37), 300)).toBeNull();
  });

  it('пустой список и нулевая высота ограничения не дают', () => {
    expect(wholeItemsHeight(makeBox(300), [], 300)).toBeNull();
    expect(wholeItemsHeight(makeBox(0), makeItems(10, 37), 0)).toBeNull();
  });

  // Регресс: пересчёт зовётся на каждое событие прокрутки. Если он даёт то же
  // значение, Vue не перерисовывает список и меню не дёргается.
  it('идемпотентна: повторный расчёт при той же доступной высоте даёт то же число', () => {
    const first = wholeItemsHeight(makeBox(281), makeItems(13, 37), 281);
    const second = wholeItemsHeight(makeBox(281), makeItems(13, 37), 281);
    expect(second).toBe(first);
  });
});

describe('measureChromeHeight', () => {
  const el = (offsetHeight) => ({ offsetHeight });

  it('складывает высоты служебных блоков, исключая сам список', () => {
    const box = el(200);
    const menu = { children: [el(57), el(33), box] };
    expect(measureChromeHeight(menu, box)).toBe(90);
  });

  it('без меню возвращает 0', () => {
    expect(measureChromeHeight(null, null)).toBe(0);
  });
});
