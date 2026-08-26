import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ungatedRouteComponents } from '../../../components/onboarding/__tests__/routerGates.js';

/**
 * Модули, которые исполняются на экранах, открытых любому вошедшему.
 *
 * Точки входа - компоненты роутов без `meta.permission` и без дополнительного
 * мета-флага (их отдаёт `routerGates.js`), плюс `App.vue` и `NavMenu.vue`: эти двое
 * живут на каждой странице. Дальше идём по импортам.
 *
 * Обёртки `src/api/**` в обход НЕ включаются: модуль там - словарь запросов, и факт
 * импорта ничего не говорит о том, какие из них зовёт экран. Их разбирает второй
 * замок - по именам импортированных функций.
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const SRC_ROOT = path.resolve(HERE, '../../..');
const ROUTER_FILE = path.join(SRC_ROOT, 'router.js');

const ALWAYS_MOUNTED = ['App.vue', 'components/NavMenu.vue'];

/**
 * @param {string} spec путь импорта (`@/...`, `./...`)
 * @param {string} fromFile файл, в котором импорт написан
 * @returns {string|null} абсолютный путь файла либо null для пакета/несуществующего
 */
function resolveImport(spec, fromFile) {
  let base;
  if (spec.startsWith('@/')) base = path.join(SRC_ROOT, spec.slice(2));
  else if (spec.startsWith('.')) base = path.resolve(path.dirname(fromFile), spec);
  else return null;

  const candidates = [base, `${base}.vue`, `${base}.js`, path.join(base, 'index.js')];
  return candidates.find((c) => fs.existsSync(c) && fs.statSync(c).isFile()) ?? null;
}

const IMPORT_RE = /(?:from\s*['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\))/g;

/** @param {string} file @returns {boolean} обёртка запросов из `src/api` */
export function isApiWrapper(file) {
  return path.relative(SRC_ROOT, file).startsWith(`api${path.sep}`);
}

/**
 * @returns {string[]} абсолютные пути модулей пользовательских экранов
 */
export function userFlowModules() {
  const seeds = [
    ...ungatedRouteComponents().map((spec) => resolveImport(spec, ROUTER_FILE)),
    ...ALWAYS_MOUNTED.map((rel) => path.join(SRC_ROOT, rel)),
  ].filter(Boolean);

  const seen = new Set();
  const queue = [...seeds];
  while (queue.length) {
    const file = queue.pop();
    if (seen.has(file)) continue;
    seen.add(file);
    if (isApiWrapper(file)) continue;

    const text = fs.readFileSync(file, 'utf8');
    for (const m of text.matchAll(IMPORT_RE)) {
      const resolved = resolveImport(m[1] ?? m[2], file);
      if (resolved && !seen.has(resolved)) queue.push(resolved);
    }
  }
  // Сами обёртки в список не идут: их запросы разбирает второй замок, по вызовам.
  return [...seen].filter((f) => /\.(vue|js)$/.test(f) && !isApiWrapper(f));
}

const PATH_LITERAL_RE = /`([^`]*)`|'([^']*)'|"([^"]*)"/g;

/** @param {string} literal @returns {string} путь с `:x` вместо подставляемых значений */
function normalizePath(literal) {
  return literal.replace(/\$\{[^}]*\}/g, ':x').split('?')[0];
}

/**
 * Пути, которые может принять переменная, переданная в `apiRequest` вместо литерала.
 * Собираем строковые литералы из её объявлений (`const endpoint = ...`, в том числе
 * из тернарника): без этого вызов через переменную выглядит как «пути нет», а
 * закрытый правом запрос проезжает мимо замка.
 *
 * @param {string} text
 * @param {string} name
 * @returns {string[]}
 */
function pathsOfVariable(text, name) {
  const paths = [];
  for (const decl of text.matchAll(new RegExp(`\\b(?:const|let|var)\\s+${name}\\s*=([^;\\n]*(?:\\n\\s*[^;\\n]*)*?);`, 'g'))) {
    for (const literal of decl[1].matchAll(PATH_LITERAL_RE)) {
      const value = literal[1] ?? literal[2] ?? literal[3];
      if (value.startsWith('/')) paths.push(normalizePath(value));
    }
  }
  return [...new Set(paths)];
}

/**
 * Вызовы `apiRequest`/`apiRequestRaw`: путь, метод и полный текст аргументов (в нём
 * ищем `silent403`). У вызова через неразобранную переменную `path` равен null -
 * такие спека перечисляет отдельно, чтобы «не смог прочитать» не читалось как
 * «проверено и чисто».
 *
 * @param {string} text исходник модуля
 * @returns {Array<{ path: string|null, expression: string, method: string, args: string, line: number }>}
 */
export function apiCallsIn(text) {
  const calls = [];
  for (const m of text.matchAll(/apiRequest(?:Raw)?\(/g)) {
    const args = argumentsAfter(text, m.index + m[0].length - 1);
    const first = args.trim().split(',')[0].trim();
    const line = text.slice(0, m.index).split('\n').length;
    const method = args.match(/method:\s*['"](\w+)['"]/)?.[1] ?? 'GET';

    const literal = first.match(/^(?:`([^`]*)`|'([^']*)'|"([^"]*)")$/);
    const paths = literal
      ? [normalizePath(literal[1] ?? literal[2] ?? literal[3])]
      : (/^\w+$/.test(first) ? pathsOfVariable(text, first) : []);

    if (!paths.length) { calls.push({ path: null, expression: first, method, args, line }); continue; }
    paths.forEach((p) => calls.push({ path: p, expression: first, method, args, line }));
  }
  return calls;
}

/** @param {string} text @param {number} openIndex позиция `(` @returns {string} текст аргументов */
function argumentsAfter(text, openIndex) {
  let depth = 0;
  for (let i = openIndex; i < text.length; i += 1) {
    if (text[i] === '(') depth += 1;
    else if (text[i] === ')') {
      depth -= 1;
      if (depth === 0) return text.slice(openIndex + 1, i);
    }
  }
  return text.slice(openIndex + 1, openIndex + 400);
}
