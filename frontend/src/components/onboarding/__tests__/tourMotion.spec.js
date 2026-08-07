import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { onboardingSteps } from '../onboardingSteps';
import { allTourSteps } from '../tours';

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

describe('появление подсветки', () => {
  // Людям мешала не площадь подсветки, а её мгновенная смена: «просто огромный
  // светлый квадрат спавнится». Вырез - один SVG-путь, у которого между шагами
  // меняются только координаты, поэтому браузер интерполирует его как форму.
  it('вырез затемнения перетекает между шагами', () => {
    expect(css).toMatch(/\.driver-overlay path\s*\{[^}]*transition:\s*d\s/s);
  });

  it('при отключённой анимации вырез не перетекает', () => {
    expect(css).toMatch(/prefers-reduced-motion:\s*reduce[\s\S]*?\.driver-overlay path[\s\S]*?transition:\s*none/);
  });
});
