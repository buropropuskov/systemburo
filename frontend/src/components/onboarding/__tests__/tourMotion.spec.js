import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { onboardingSteps } from '../onboardingSteps';

const css = readFileSync(resolve(__dirname, '../../../assets/onboarding.css'), 'utf8');

/**
 * Замки на плавность и позиционирование тура (#1771 followup). Всё это ловится
 * только глазами, поэтому фиксируем сами правила, а не картинку.
 */
describe('движение поповера', () => {
  // Без перехода driver.js телепортирует карточку: между соседними шагами
  // замеряли прыжки в 400-900 пикселей.
  it('позиция поповера анимируется', () => {
    expect(css).toMatch(/\.driver-popover\s*\{[^}]*transition:[^}]*left/s);
    expect(css).toMatch(/\.driver-popover\s*\{[^}]*top/s);
  });

  it('первый показ в сегменте - без переезда из угла', () => {
    expect(css).toMatch(/\.driver-popover\.ob-popover--instant\s*\{\s*transition:\s*none/);
  });

  it('при отключённой анимации переездов нет', () => {
    expect(css).toMatch(/prefers-reduced-motion:\s*reduce[\s\S]*?\.driver-popover[\s\S]*?transition:\s*none/);
  });
});

describe('позиционирование шагов', () => {
  const byId = (id) => onboardingSteps.find((s) => s.id === id);

  // Два шага подряд на одной цели читаются как «Далее не сработало»: поповер
  // остаётся на месте, и человек не понимает, что произошло.
  //
  // Исключение - когда содержимое цели меняется (формы машин и сотрудников
  // живут в одном контейнере). Тогда шаг законен, но обязан сдвинуть поповер:
  // сторона или выравнивание должны отличаться, иначе визуально это всё тот же
  // застывший экран.
  it('повторный якорь у соседних шагов сдвигает поповер', () => {
    const frozen = [];
    for (let i = 1; i < onboardingSteps.length; i += 1) {
      const prev = onboardingSteps[i - 1];
      const cur = onboardingSteps[i];
      const sameTarget = cur.element && cur.element === prev.element && cur.route === prev.route;
      if (!sameTarget) continue;
      const samePlace = (cur.side || '') === (prev.side || '') && (cur.align || '') === (prev.align || '');
      if (samePlace) frozen.push(`${prev.id} -> ${cur.id}: ${cur.element}`);
    }
    expect(frozen).toEqual([]);
  });

  // Форма занимает почти весь экран - сбоку driver.js места не находит и
  // выдавливает поповер в угол поверх той самой формы.
  it('шаги широких форм ставят поповер сверху', () => {
    expect(byId('createapp-people-form').side).toBe('top');
  });
});

describe('переключение подсветки', () => {
  const engine = readFileSync(resolve(__dirname, '../../../composables/useOnboarding.js'), 'utf8');
  const css = readFileSync(resolve(__dirname, '../../../assets/onboarding.css'), 'utf8');

  // Родной переезд рамки driver.js оставляем включённым: без него подсветка
  // щёлкает с шага на шаг, и это заметили сразу.
  it('переезд подсветки анимирует сам driver.js', () => {
    expect(engine).toMatch(/animate:\s*!prefersReducedMotion\(\)/);
  });

  // driver.js снимает класс с прежней цели только в конце своей анимации: при
  // быстрых «Далее» пометки накапливались и над затемнением торчали три элемента.
  it('прежняя подсветка снимается в начале перехода', () => {
    expect(engine).toMatch(/onHighlightStarted\(element\)\s*\{[^}]*dropStaleHighlights\(element\)/);
  });

  // Зазор и скругление выреза driver.js держит только в глобальном конфиге, а
  // читает на каждом кадре: подогнать их под цель можно ровно в начале перехода.
  it('форма выреза подгоняется под цель там же, в начале перехода', () => {
    expect(engine).toMatch(/onHighlightStarted\(element\)\s*\{[^}]*applyStageShape\(driverObj, element\)/);
  });

  // Пока вырез едет, цель уже помечена классом driver.js. Если вешать на него
  // подъём над затемнением, форма протыкает затемнение раньше подсветки -
  // «сначала светятся инпуты, потом появляется блок». Поднимаем своим классом,
  // и только когда переход доехал.
  it('цель поднимается над затемнением по завершении перехода', () => {
    expect(css).toMatch(/\.ob-highlighted\s*\{[^}]*z-index/s);
    expect(css).not.toMatch(/\.driver-active-element\s*\{[^}]*z-index/s);
    expect(engine).toMatch(/onHighlighted\(\)[\s\S]{0,120}raiseActiveHighlight\(driverObj\)/);
  });
});

describe('появление подсветки', () => {
  // Переливание выреза через `transition: d` пробовали и откатили по просьбе
  // владельца: «квадрат медленно выползает». Подсветка обязана переключаться
  // мгновенно - замок держит решение, чтобы его не вернули «для плавности».
  it('вырез затемнения переключается мгновенно', () => {
    expect(css).not.toMatch(/\.driver-overlay\s+path\s*\{[^}]*transition:\s*(?!none)/s);
  });
});
