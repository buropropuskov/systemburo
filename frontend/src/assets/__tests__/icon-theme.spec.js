/**
 * Замки темизации растровых иконок (#1415).
 *
 * Правила icon-theme.css матчат иконку по имени файла в src. Держится это на
 * трёх вещах, которые легко разъехаться: имена в CSS = имена файлов, инлайн
 * иконок отключён в vite.config (из data:-URI имя пропадает), и префиксные
 * селекторы не задевают цветных соседей. Каждую проверяем отдельно.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

import viteConfig from '../../../vite.config.js';

const ROOT = path.resolve(__dirname, '../../..');
const ICONS_DIR = path.join(ROOT, 'src/assets/icons');
const CSS = fs.readFileSync(path.join(ROOT, 'src/assets/icon-theme.css'), 'utf8');

/** Имена из блока, применяющего указанный фильтр: mono-инверсия или акцентное осветление. */
function namesFor(filterVar, pattern) {
  const blocks = [...CSS.matchAll(/([^{}]+)\{\s*filter:\s*var\((--icon-[a-z-]+)\)[^}]*\}/g)];
  const block = blocks.find((b) => b[2] === filterVar);
  if (!block) return [];
  return [...block[1].matchAll(pattern)].map((m) => m[1]);
}
const monoNames = namesFor('--icon-mono-filter', /img\[src\*="\/([^"/]+)\.png"\]/g);
/** Парные селекторы под хэшированное имя: img[src*="/edit-"]. */
const hashedNames = namesFor('--icon-mono-filter', /img\[src\*="\/([^"/]+)-"\]/g);
const accentNames = namesFor('--icon-accent-filter', /img\[src\*="\/([^"/]+)\.png"\]/g);
const accentHashed = namesFor('--icon-accent-filter', /img\[src\*="\/([^"/]+)-"\]/g);

describe('icon-theme.css', () => {
  it('перечисляет ровно живые однотонные иконки', () => {
    // Число тает с каждым глифом, переехавшим в SVG-реестр, и обнулится вместе с
    // самим файлом. Точное значение, а не «больше N»: правка списка должна быть
    // осознанной, иначе из него легко выпадает используемая иконка.
    expect(monoNames.length).toBe(12);
  });

  it('каждой иконке даёт оба селектора: исходное имя и хэшированное', () => {
    expect([...monoNames].sort()).toEqual([...hashedNames].sort());
  });

  it('называет только существующие файлы', () => {
    const missing = monoNames.filter((n) => !fs.existsSync(path.join(ICONS_DIR, `${n}.png`)));
    expect(missing, 'переименовали иконку - правило темы стало мёртвым').toEqual([]);
  });

  it('акцентные иконки перечислены отдельно и не смешаны с однотонными', () => {
    // Инверсия синий глиф испортит (станет оранжевым), поэтому фирменные иконки
    // идут своим фильтром - осветлением.
    expect([...accentNames].sort()).toEqual(['instruction', 'random', 'refresh']);
    expect([...accentNames].sort()).toEqual([...accentHashed].sort());
    expect(accentNames.filter((n) => monoNames.includes(n)),
      'иконка не может быть одновременно однотонной и акцентной').toEqual([]);
    const missing = accentNames.filter((n) => !fs.existsSync(path.join(ICONS_DIR, `${n}.png`)));
    expect(missing).toEqual([]);
  });

  it('сине-голубые значки экрана входа темизацией не трогаются', () => {
    // Они лежат на светлом островке формы входа: осветлять там нечего, а инверсия
    // и вовсе поменяла бы им цвет.
    ['email-blue', 'key-blue', 'phone-blue'].forEach((n) => {
      expect(monoNames, `${n} не должна инвертироваться`).not.toContain(n);
      expect(accentNames, `${n} не должна осветляться`).not.toContain(n);
    });
  });

  it('каждая роль фильтра объявлена и в светлой, и в тёмных темах', () => {
    ['--icon-mono-filter', '--icon-ink-filter', '--icon-accent-filter'].forEach((v) => {
      // Дважды: нейтральное значение на :root/[data-theme] и рабочее в тёмных.
      expect(CSS.match(new RegExp(`${v}:`, 'g')) ?? [], `${v} объявлена не во всех темах`)
        .toHaveLength(2);
    });
  });

  it('префиксным селектором не задевает цветных соседей', () => {
    // img[src*="/car-"] матчит и car-hash.png, и любой car-<что-то>.png:
    // цветной сосед с таким именем инвертировался бы вместе с однотонным.
    const files = fs.readdirSync(ICONS_DIR).filter((f) => f.endsWith('.png'));
    const mono = new Set(monoNames);
    const overmatched = files
      .map((f) => f.replace(/\.png$/, ''))
      .filter((name) => !mono.has(name) && monoNames.some((m) => name.startsWith(`${m}-`)));
    expect(overmatched).toEqual([]);
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
    // false = не инлайнить. Инлайн стирает имя файла и ломает icon-theme.css.
    expect(limit(path.join(ROOT, 'src/assets/icons/notifications.png'))).toBe(false);
  });

  it('прочим ассетам оставляет штатное правило по размеру', () => {
    expect(limit(path.join(ROOT, 'src/assets/onboarding/demo-applications.png'))).toBeUndefined();
    expect(limit(path.join(ROOT, 'src/assets/onboarding/demo-cars.png'))).toBeUndefined();
  });
});
