import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

/**
 * Вложенный слой обязан отличаться цветом от того, на чём лежит (#1877).
 *
 * Слоя три (конвенция в шапке tokens.css): --surface-sunken - подложка ПОД
 * карточками, --surface - карточка, --surface-2 - подложка ВНУТРИ карточки.
 * Ошибка выбора слоя не видна ни в разметке, ни в линтере, а в светлой палитре
 * --surface-2 и --surface-sunken к тому же совпадают (#f9f9f9): плашка на
 * подложке секции пропадает как форма, остаётся висеть голый текст. Именно так
 * первый заход #1877 чуть не убрал счётчик участников - он лежит прямо в .card.
 *
 * Пары собраны обходом разметки: для каждого элемента, которому #1877 задал свой
 * фон, найден первый предок с фоном (не по имени класса, а подъёмом по шаблону).
 * Список ручной - при перестановке разметки его правят вместе с ней; проверка
 * «оба селектора существуют и несут фон» ловит хотя бы переименование.
 */

const srcDir = resolve(__dirname, '../..');
const tokensCss = readFileSync(join(srcDir, 'assets/tokens.css'), 'utf8');

const THEMES = ['light', 'dark'];

/**
 * [элемент, его фактический родитель по разметке]. Обычно оба селектора лежат в
 * одном файле; четвёртым элементом задаётся файл родителя, когда элемент - корень
 * дочернего компонента, вложенного в чужую секцию.
 */
const PAIRS = [
  ['components/OrganizationsManagement.vue', '.card', '.details-column'],
  ['components/OrganizationsManagement.vue', '.count-badge', '.card'],
  ['components/OrganizationsManagement.vue', '.person', '.card'],
  ['components/OrganizationsManagement.vue', '.avatar', '.person'],
  ['components/OrganizationsManagement.vue', '.table-footer', '.table-container'],
  ['components/OrganizationsManagement.vue', '.pill-type', '.details-column'],

  ['components/CompaniesManagement.vue', '.card', '.details-column'],
  ['components/CompaniesManagement.vue', '.count-badge', '.card'],
  ['components/CompaniesManagement.vue', '.person', '.card'],
  ['components/CompaniesManagement.vue', '.avatar', '.person'],
  ['components/CompaniesManagement.vue', '.table-footer', '.table-container'],
  ['components/CompaniesManagement.vue', '.pill-type', '.details-column'],

  // Корень секции - .card, поэтому её предок лежит в файле-хозяине и в пару не идёт.
  ['components/ResponsibleUsersSection.vue', '.count-badge', '.card'],
  ['components/ResponsibleUsersSection.vue', '.no-selected-users', '.card'],
  ['components/ResponsibleUsersSection.vue', '.resp-item', '.card'],
  ['components/ResponsibleUsersSection.vue', '.user-dropdown', '.card'],
  ['components/ResponsibleUsersSection.vue', '.avatar', '.resp-item'],
  ['components/ResponsibleUsersSection.vue', '.tag-pos', '.resp-item'],
  ['components/ResponsibleUsersSection.vue', '.tag-neutral', '.resp-item'],
  ['components/ResponsibleUsersSection.vue', '.user-tag', '.user-dropdown'],
  ['components/ResponsibleUsersSection.vue', '.user-dropdown-item:hover', '.user-dropdown'],
  // Строка меняет фон при наведении, поэтому чип сверяется с обоими её состояниями:
  // на одном --surface-2 у чипа и у подсветки он пропадал при наведении (#1894).
  ['components/ResponsibleUsersSection.vue', '.user-tag', '.user-dropdown-item:hover'],

  ['components/SelectTables.vue', '.select-tables-container', '.card'],
  ['components/SelectUnloadPlaces.vue', '.unload-places-container', '.card'],

  // Жёлтый блок разбора лежит в секции `.card` справочника - у обоих хозяев своя.
  // Селектор базовый: вариант --panel правит только отступ, заливку несёт корень.
  ['components/directory/DirectoryModeration.vue', '.org-moderation', '.card', 'components/OrganizationsManagement.vue'],
  ['components/directory/DirectoryModeration.vue', '.org-moderation', '.card', 'components/CompaniesManagement.vue'],
];

const stripComments = (css) => css.replace(/\/\*[\s\S]*?\*\//g, '');

/** Переменные темы: сама палитра плюс блок legacy-алиасов --color-* без темы. */
function palette(theme) {
  const vars = {};
  for (const block of stripComments(tokensCss).matchAll(/\[data-theme(?:="([\w-]+)")?\]\s*\{([\s\S]*?)\n\}/g)) {
    if (block[1] && block[1] !== theme) continue;
    for (const m of block[2].matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) vars[m[1]] = m[2].trim();
  }
  return vars;
}

/** Раскрывает var() до литерала; null, если что-то не разрешилось. */
function resolveVars(value, vars) {
  let out = value;
  for (let i = 0; i < 10 && out.includes('var('); i += 1) {
    const next = out.replace(/var\(\s*(--[\w-]+)\s*(?:,\s*([^()]*?)\s*)?\)/g, (all, name, fallback) => {
      if (vars[name] !== undefined) return vars[name];
      return fallback !== undefined ? fallback : all;
    });
    if (next === out) return null;
    out = next;
  }
  return out.includes('var(') ? null : out.trim().toLowerCase();
}

/** Последний background в правиле ровно этого селектора. */
function backgroundOf(source, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const re = new RegExp(`(?:^|[}\\n])\\s*${escaped}\\s*\\{([^{}]*)\\}`, 'g');
  for (const rule of stripComments(source).matchAll(re)) {
    const found = [...rule[1].matchAll(/background(?:-color)?\s*:\s*([^;}]+)/g)].pop();
    if (found) return found[1].trim();
  }
  return null;
}

/**
 * Разворачивает @import в тексте стилей: правило, вынесенное в общий файл,
 * применяется к компоненту точно так же, как написанное внутри него, и проверка
 * обязана видеть оба. Без этого вынос общего оформления справочников (#871)
 * ослеплял замок - фон находился, пока правило лежало в самом .vue, и переставал
 * находиться после переезда, хотя на экране не менялось ничего.
 */
const inlineImports = (source) => {
  const imports = [...source.matchAll(/@import\s+['"]([^'"]+)['"]\s*;/g)];
  if (!imports.length) return source;
  const imported = imports
    .map(([, spec]) => {
      const rel = spec.startsWith('@/') ? spec.slice(2) : spec;
      try {
        return readFileSync(join(srcDir, rel), 'utf8');
      } catch {
        // Импорт из node_modules и прочее, чего в src нет: для проверки фона
        // такие не нужны, а падать на них замок не должен.
        return '';
      }
    })
    .join('\n');
  return `${source}\n${imported}`;
};

const sources = new Map();
const sourceOf = (file) => {
  if (!sources.has(file)) sources.set(file, inlineImports(readFileSync(join(srcDir, file), 'utf8')));
  return sources.get(file);
};

describe('вложенный слой отличается от того, на чём лежит', () => {
  it.each(PAIRS)('%s: %s и %s объявляют фон', (file, child, parent, parentFile) => {
    const host = parentFile || file;
    expect(backgroundOf(sourceOf(file), child), `${file}: у ${child} нет фона`).toBeTruthy();
    expect(backgroundOf(sourceOf(host), parent), `${host}: у ${parent} нет фона`).toBeTruthy();
  });

  it.each(THEMES)('%s: ни одна пара не сливается', (theme) => {
    const vars = palette(theme);
    const merged = [];
    for (const [file, child, parent, parentFile] of PAIRS) {
      const a = backgroundOf(sourceOf(file), child);
      const b = backgroundOf(sourceOf(parentFile || file), parent);
      if (!a || !b) continue;
      const [ra, rb] = [resolveVars(a, vars), resolveVars(b, vars)];
      // Совпасть могут и записи, и разные токены с одним значением палитры.
      if (a.toLowerCase() === b.toLowerCase() || (ra !== null && ra === rb)) {
        merged.push(`${file}: ${child} (${a}) на ${parent} (${b}) - обе ${ra ?? a}`);
      }
    }
    expect(merged, 'элемент пропадёт как форма, останется висеть его содержимое').toEqual([]);
  });
});
