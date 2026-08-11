import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Модули, которые исполняются на экранах, открытых любому вошедшему.
 *
 * Точки входа - компоненты роутов без `meta.permission` и без дополнительного
 * мета-флага (их читаем из `router.js` тем же способом, что `routerGates.js`), плюс
 * `App.vue` и `NavMenu.vue`: эти двое живут на каждой странице. Дальше идём по
 * импортам.
 *
 * Обёртки `src/api/**` в обход НЕ включаются: модуль там - словарь запросов, и факт
 * импорта ничего не говорит о том, какие из них зовёт экран. Их разбирает второй
 * замок - по именам импортированных функций.
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const SRC_ROOT = path.resolve(HERE, '../../..');
const ROUTER_FILE = path.join(SRC_ROOT, 'router.js');

const ALWAYS_MOUNTED = ['App.vue', 'components/NavMenu.vue'];

/** @returns {string[]} импортные пути компонентов роутов, открытых любому вошедшему */
function ungatedRouteComponents() {
  const src = fs.readFileSync(ROUTER_FILE, 'utf8');
  // Компоненты роутов объявлены двумя способами: ленивым import() прямо в объекте
  // роута и статическим импортом по имени в шапке файла.
  const staticImports = {};
  for (const m of src.matchAll(/import\s+(\w+)\s+from\s+'([^']+)'/g)) staticImports[m[1]] = m[2];

  const marks = [...src.matchAll(/path:\s*'([^']+)'/g)];
  const specs = [];
  marks.forEach((mark, i) => {
    const chunk = src.slice(mark.index, i + 1 < marks.length ? marks[i + 1].index : src.length);
    if (!/requiresAuth:\s*true/.test(chunk)) return;
    if (/permission:/.test(chunk) || /requires(SuperAdmin|SecurityOrAdmin):\s*true/.test(chunk)) return;
    const lazy = chunk.match(/component:\s*\(\)\s*=>\s*import\('([^']+)'\)/);
    if (lazy) { specs.push(lazy[1]); return; }
    const named = chunk.match(/component:\s*(\w+)/);
    if (named && staticImports[named[1]]) specs.push(staticImports[named[1]]);
  });
  return specs;
}

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

/**
 * Вызовы `apiRequest`/`apiRequestRaw` с литеральным путём: путь с `:x` вместо
 * подставляемых значений, метод и полный текст аргументов (в нём ищем `silent403`).
 *
 * @param {string} text исходник модуля
 * @returns {Array<{ path: string, method: string, args: string, line: number }>}
 */
export function apiCallsIn(text) {
  const calls = [];
  const re = /apiRequest(?:Raw)?\(\s*(?:`([^`]*)`|'([^']*)'|"([^"]*)")/g;
  for (const m of text.matchAll(re)) {
    const raw = m[1] ?? m[2] ?? m[3];
    const args = argumentsAfter(text, m.index + m[0].indexOf('('));
    calls.push({
      path: raw.replace(/\$\{[^}]*\}/g, ':x').split('?')[0],
      method: args.match(/method:\s*['"](\w+)['"]/)?.[1] ?? 'GET',
      args,
      line: text.slice(0, m.index).split('\n').length,
    });
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
