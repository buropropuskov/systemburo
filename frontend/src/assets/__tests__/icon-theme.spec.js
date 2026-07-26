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

/** Имена однотонных иконок из селекторов вида img[src*="/edit.png"]. */
const monoNames = [...CSS.matchAll(/img\[src\*="\/([^"/]+)\.png"\]/g)].map((m) => m[1]);
/** Парные селекторы под хэшированное имя: img[src*="/edit-"]. */
const hashedNames = [...CSS.matchAll(/img\[src\*="\/([^"/]+)-"\]/g)].map((m) => m[1]);

describe('icon-theme.css', () => {
  it('перечисляет непустой список однотонных иконок', () => {
    expect(monoNames.length).toBeGreaterThan(20);
  });

  it('каждой иконке даёт оба селектора: исходное имя и хэшированное', () => {
    expect([...monoNames].sort()).toEqual([...hashedNames].sort());
  });

  it('называет только существующие файлы', () => {
    const missing = monoNames.filter((n) => !fs.existsSync(path.join(ICONS_DIR, `${n}.png`)));
    expect(missing, 'переименовали иконку - правило темы стало мёртвым').toEqual([]);
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
    expect(limit(path.join(ROOT, 'src/assets/background.png'))).toBeUndefined();
    expect(limit(path.join(ROOT, 'src/assets/onboarding/demo-cars.png'))).toBeUndefined();
  });
});
