/**
 * Замки темизации иконок (#1415).
 *
 * Растровых иконок в проекте не осталось: последняя партия переехала в SVG-реестр
 * (срез V5 эпика icons-license-cleanup), и поимённые правила icon-theme.css вместе
 * с ними. Файл дорабатывает свой век ради переменных-фильтров, которые ещё читает
 * FactTable; сам он уходит следующим срезом.
 *
 * Поэтому проверки сменили предмет: не «список правил совпадает с каталогом PNG»,
 * а «растр не вернулся» плюс те замки на компоненты, что от растра не зависели.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

import viteConfig from '../../../vite.config.js';

const ROOT = path.resolve(__dirname, '../../..');
const ICONS_DIR = path.join(ROOT, 'src/assets/icons');
const CSS = fs.readFileSync(path.join(ROOT, 'src/assets/icon-theme.css'), 'utf8');

describe('icon-theme.css', () => {
  it('не держит ни одного поимённого правила: растровых иконок больше нет', () => {
    // Возврат такого правила означал бы, что в проект снова положили PNG и
    // темизируют его фильтром вместо currentColor.
    const rules = [...CSS.matchAll(/img\[src\*=/g)];
    expect(rules, 'иконка снова темизируется по имени файла, а не цветом текста').toHaveLength(0);
  });

  it('каталог растровых иконок пуст', () => {
    const files = fs.existsSync(ICONS_DIR)
      ? fs.readdirSync(ICONS_DIR).filter((f) => /\.(png|jpe?g|gif|webp)$/i.test(f))
      : [];
    expect(files, 'растровая иконка вернулась в assets/icons - нарисуйте глиф в appIcons.js').toEqual([]);
  });

  it('каждая роль фильтра объявлена и в светлой, и в тёмных темах', () => {
    // Переменные ещё живы ради FactTable: пока правило на них ссылается, объявление
    // обязано быть в обеих темах, иначе фильтр в одной из них станет пустым.
    ['--icon-mono-filter', '--icon-ink-filter', '--icon-accent-filter'].forEach((v) => {
      expect(CSS.match(new RegExp(`${v}:`, 'g')) ?? [], `${v} объявлена не во всех темах`)
        .toHaveLength(2);
    });
  });
});

describe('компоненты не перекрашивают иконки в обход темы', () => {
  const files = [];
  (function walk(dir) {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, e.name);
      if (e.isDirectory()) walk(full);
      else if (e.name.endsWith('.vue')) files.push(full);
    }
  })(path.join(ROOT, 'src'));

  it('силуэт иконки задаётся токеном, а не голым brightness(0)', () => {
    // brightness(0) красит глиф в чёрный при любой теме: в тёмной это чёрное на
    // чёрном (сортировочные стрелки, #1415). Цвет силуэта даёт --icon-ink-filter.
    // Исключение - brightness(0) invert(1): белый глиф на цветной кнопке.
    const bad = files.filter((f) => /filter:\s*brightness\(0\)\s*;/.test(fs.readFileSync(f, 'utf8')))
      .map((f) => path.relative(ROOT, f));
    expect(bad, 'используйте filter: var(--icon-ink-filter)').toEqual([]);
  });

  it('пелена загрузки поверх контента - поверхность темы, а не белый литерал', () => {
    // Оверлей на всю ширину блока (position + inset/top:0) с белым фоном в тёмной
    // теме светится белым поверх тёмной таблицы (#1415). Кнопки поверх ФОТОГРАФИЙ
    // белые намеренно - у них нет растяжки на весь блок.
    const bad = [];
    for (const f of files) {
      const txt = fs.readFileSync(f, 'utf8');
      for (const m of txt.matchAll(/\{([^{}]*)\}/g)) {
        const body = m[1];
        if (!/background(-color)?:\s*rgba\(\s*255,\s*255,\s*255/.test(body)) continue;
        const stretched = /inset:\s*0/.test(body) || (/top:\s*0/.test(body) && /bottom:\s*0/.test(body));
        if (stretched) bad.push(path.relative(ROOT, f));
      }
    }
    expect([...new Set(bad)], 'используйте color-mix(in srgb, var(--surface) N%, transparent)').toEqual([]);
  });

  it('состояния колокольчика не подменяют filter иконки', () => {
    // filter у иконки занят темой: локальные grayscale/contrast делали колокольчик
    // то серым, то чёрным. Состояния выражаются подложкой и прозрачностью.
    const header = fs.readFileSync(path.join(ROOT, 'src/components/TheHeader/TheHeader.vue'), 'utf8');
    const rules = [...header.matchAll(/\.notifications__icon[^{}]*\{([^{}]*)\}/g)].map((m) => m[1]);
    expect(rules.length).toBeGreaterThan(0);
    rules.forEach((body) => expect(body).not.toMatch(/filter:/));
  });
});

describe('vite.config: инлайн иконок', () => {
  const limit = viteConfig.build?.assetsInlineLimit;

  it('оставляет иконки отдельными файлами', () => {
    expect(typeof limit, 'assetsInlineLimit должен быть функцией-исключением для иконок').toBe(
      'function',
    );
    // Правило проверяется по пути, а не по файлу: сами PNG уже удалены, а
    // исключение переживает их до сноса icon-theme.css следующим срезом.
    expect(limit(path.join(ROOT, 'src/assets/icons/notifications.png'))).toBe(false);
  });

  it('прочим ассетам оставляет штатное правило по размеру', () => {
    expect(limit(path.join(ROOT, 'src/assets/onboarding/demo-applications.png'))).toBeUndefined();
    expect(limit(path.join(ROOT, 'src/assets/onboarding/demo-cars.png'))).toBeUndefined();
  });
});
