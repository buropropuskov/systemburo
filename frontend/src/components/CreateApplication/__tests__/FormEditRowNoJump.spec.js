import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

/**
 * Строка кнопок формы не должна прыгать при входе в режим редактирования.
 *
 * «Отменить» появляется по v-if только в режиме правки и встаёт рядом с
 * «Добавить»/«Применить». Контейнер строки - flex без заданной высоты, поэтому
 * его высоту диктует самая высокая кнопка. Пока у «Добавить» стоял border: none,
 * а у «Отменить» - border: 1px solid var(--border), разница в 2px (31 против 33)
 * поглощаться было нечем: строка, а с ней и вся форма, дёргалась на каждый вход
 * в редактирование. Лечится приёмом эталонной кнопки проекта (forms.css: .lk-button
 * объявлен с border: 1px solid transparent) - рамка держит место всегда.
 *
 * Замок текстовый: jsdom не считает раскладку, померить высоту кнопки в нём
 * нельзя. Зато видно причину: сверяем, что обе кнопки объявляют рамку и совпадают
 * в остальных слагаемых высоты - вертикальном padding и кегле.
 */

const componentsDir = resolve(__dirname, '..');

/** Формы с парой «Отменить» + «Добавить» и класс их строки кнопок. */
const FORMS = [
  { file: 'EmployeeForm.vue', actionsRow: '.citizenship-actions' },
  { file: 'VehicleForm.vue', actionsRow: '.format-actions' },
  { file: 'ItemsForm.vue', actionsRow: '.completion__actions' },
];

/** Содержимое единственного <style scoped> компонента, без комментариев. */
function styleOf(source) {
  const match = source.match(/<style[^>]*>([\s\S]*)<\/style>/);
  // Комментарии снимаем сразу: двоеточие внутри пояснения читалось бы как
  // начало объявления и склеивалось с идущим следом свойством.
  return match ? match[1].replace(/\/\*[\s\S]*?\*\//g, '') : '';
}

/** CSS без @media-блоков: остаются только базовые (десктопные) правила. */
function withoutAtRules(css) {
  let out = '';
  for (let i = 0; i < css.length; i += 1) {
    if (css[i] !== '@') {
      out += css[i];
      continue;
    }
    const open = css.indexOf('{', i);
    if (open < 0) break;
    let depth = 0;
    let j = open;
    for (; j < css.length; j += 1) {
      if (css[j] === '{') depth += 1;
      else if (css[j] === '}' && (depth -= 1) === 0) break;
    }
    i = j;
  }
  return out;
}

/** Объявления правила с ровно таким селектором, или null. */
function ruleOf(css, selector) {
  const rules = css.matchAll(/([^{}]+)\{([^{}]*)\}/g);
  for (const [, selectors, body] of rules) {
    const list = selectors.split(',').map((s) => s.trim());
    if (!list.includes(selector)) continue;
    const declarations = {};
    for (const line of body.split(';')) {
      const colon = line.indexOf(':');
      if (colon < 0) continue;
      declarations[line.slice(0, colon).trim()] = line.slice(colon + 1).trim();
    }
    return declarations;
  }
  return null;
}

describe('формы заявки: строка кнопок не прыгает при входе в редактирование', () => {
  it.each(FORMS)('$file: «Добавить» и «Отменить» одной высоты на десктопе', ({ file }) => {
    const base = withoutAtRules(styleOf(readFileSync(join(componentsDir, file), 'utf8')));
    const add = ruleOf(base, '.add-button');
    const cancel = ruleOf(base, '.cancel-edit-btn');

    expect(add, `${file}: базовое правило .add-button не найдено`).toBeTruthy();
    expect(cancel, `${file}: базовое правило .cancel-edit-btn не найдено`).toBeTruthy();

    for (const [name, rule] of [['.add-button', add], ['.cancel-edit-btn', cancel]]) {
      expect(
        rule.border,
        `${file}: у ${name} не объявлена рамка - соседняя кнопка с рамкой окажется выше на 2px, и строка прыгнет при появлении «Отменить»`,
      ).toBeTruthy();
      expect(
        /^(none|0)\b/.test(rule.border),
        `${file}: у ${name} рамка снята (border: ${rule.border}) - место под неё не держится; прозрачная рамка, как у .lk-button в forms.css`,
      ).toBe(false);
    }

    expect(
      add.padding,
      `${file}: padding кнопок разошёлся (.add-button ${add.padding} против .cancel-edit-btn ${cancel.padding}) - высота строки снова зависит от режима`,
    ).toBe(cancel.padding);
    expect(
      add['font-size'],
      `${file}: кегль кнопок разошёлся (.add-button ${add['font-size']} против .cancel-edit-btn ${cancel['font-size']}) - разные строки текста дадут разную высоту`,
    ).toBe(cancel['font-size']);
  });

  it.each(FORMS)('$file: на мобилке обе кнопки держит один min-height', ({ file, actionsRow }) => {
    const css = styleOf(readFileSync(join(componentsDir, file), 'utf8'));
    const add = ruleOf(css, `${actionsRow} .add-button`);
    const cancel = ruleOf(css, `${actionsRow} .cancel-edit-btn`);

    expect(add, `${file}: не найдено мобильное правило ${actionsRow} .add-button`).toBeTruthy();
    expect(cancel, `${file}: не найдено мобильное правило ${actionsRow} .cancel-edit-btn`).toBeTruthy();

    // На мобилке кегль и padding у кнопок разные, разницу гасит общий тач-таргет:
    // при box-sizing: border-box рамка входит внутрь min-height и высоту не меняет.
    expect(
      add['min-height'],
      `${file}: тач-таргет кнопок разошёлся (${add['min-height']} против ${cancel['min-height']}) - на мобилке строка тоже начнёт прыгать`,
    ).toBe(cancel['min-height']);
  });
});
