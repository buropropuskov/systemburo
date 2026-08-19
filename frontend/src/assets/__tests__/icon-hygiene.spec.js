/**
 * Замки гигиены значков (#1415).
 *
 * Растровых иконок в проекте не осталось: последняя партия переехала в SVG-реестр
 * appIcons, где цвет наследуется от текста. Вместе с ними ушёл icon-theme.css -
 * костыль темизации растра на поимённых списках и invert()-фильтрах.
 *
 * Предмет проверок - обратный ход: PNG в assets/icons, перекраска значка фильтром
 * вместо currentColor, возвращённый файл темизации. Покрытие реестра стережёт
 * components/icons/__tests__/AppIcon.spec.js.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const ROOT = path.resolve(__dirname, '../../..');
const SRC = path.join(ROOT, 'src');

const sourceFiles = [];
(function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) walk(full);
    else if (/\.(vue|css|js)$/.test(e.name)) sourceFiles.push(full);
  }
})(SRC);

const rel = (f) => path.relative(ROOT, f);
const read = (f) => fs.readFileSync(f, 'utf8');

describe('растр не возвращается', () => {
  it('каталога растровых иконок нет', () => {
    const dir = path.join(SRC, 'assets/icons');
    const files = fs.existsSync(dir)
      ? fs.readdirSync(dir).filter((f) => /\.(png|jpe?g|gif|webp)$/i.test(f))
      : [];
    expect(files, 'растровая иконка вернулась в assets/icons - нарисуйте глиф в appIcons.js').toEqual([]);
  });

  it('ни один исходник не ссылается на assets/icons', () => {
    // Ссылка переживает удаление файла: сборка отдаст 404, а не упадёт.
    const bad = sourceFiles.filter((f) => /@\/assets\/icons\//.test(read(f))).map(rel);
    expect(bad, 'значок берётся из реестра: <AppIcon name="..." />').toEqual([]);
  });

  it('файл темизации растра не вернулся', () => {
    // Он существовал только потому, что PNG нельзя перекрасить. Возврат означал бы,
    // что значок снова красят по имени файла, а не цветом текста.
    expect(fs.existsSync(path.join(SRC, 'assets/icon-theme.css'))).toBe(false);
    expect(read(path.join(SRC, 'main.js'))).not.toMatch(/icon-theme\.css/);
  });
});

describe('компоненты не перекрашивают иконки в обход темы', () => {
  it('цвет значка задаётся color, а не фильтром', () => {
    // brightness(0) красит глиф в чёрный при любой теме: в тёмной это чёрное на
    // чёрном (сортировочные стрелки, #1415). У SVG-глифа цвет берётся из color.
    // Исключение - brightness(0) invert(1): белый глиф на цветной кнопке.
    const bad = sourceFiles
      .filter((f) => /filter:\s*(brightness\(0\)\s*;|var\(--icon-[\w-]*filter\))/.test(read(f)))
      .map(rel);
    expect(bad, 'используйте color: var(--text) на самом значке').toEqual([]);
  });

  it('пелена загрузки поверх контента - поверхность темы, а не белый литерал', () => {
    // Оверлей на всю ширину блока (position + inset/top:0) с белым фоном в тёмной
    // теме светится белым поверх тёмной таблицы (#1415). Кнопки поверх ФОТОГРАФИЙ
    // белые намеренно - у них нет растяжки на весь блок.
    const bad = [];
    for (const f of sourceFiles.filter((f) => f.endsWith('.vue'))) {
      const txt = read(f);
      for (const m of txt.matchAll(/\{([^{}]*)\}/g)) {
        const body = m[1];
        if (!/background(-color)?:\s*rgba\(\s*255,\s*255,\s*255/.test(body)) continue;
        const stretched = /inset:\s*0/.test(body) || (/top:\s*0/.test(body) && /bottom:\s*0/.test(body));
        if (stretched) bad.push(rel(f));
      }
    }
    expect([...new Set(bad)], 'используйте color-mix(in srgb, var(--surface) N%, transparent)').toEqual([]);
  });

  it('состояния колокольчика не подменяют filter иконки', () => {
    // Локальные grayscale/contrast делали колокольчик то серым, то чёрным.
    // Состояния выражаются подложкой и прозрачностью.
    const header = read(path.join(SRC, 'components/TheHeader/TheHeader.vue'));
    const rules = [...header.matchAll(/\.notifications__icon[^{}]*\{([^{}]*)\}/g)].map((m) => m[1]);
    expect(rules.length).toBeGreaterThan(0);
    rules.forEach((body) => expect(body).not.toMatch(/filter:/));
  });
});
