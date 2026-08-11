import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Гейты роутов бэкенда, прочитанные из исходника `internal/router/router.go`.
 *
 * Читаем текстом по образцу `onboarding/__tests__/routerGates.js`: поднимать Go ради
 * списка путей незачем, а знать, какой путь закрыт правом, фронту нужно - иначе
 * фоновый запрос из пользовательского экрана упирается в 403 и показывает тост о
 * действии, которого человек не совершал (#1928: форма подачи звала `/users/all`).
 *
 * Разбор идёт по балансу скобок, а не построчно: объявление роута занимает две
 * строки, когда мидлварей несколько, и построчное чтение теряло бы их молча -
 * потерянный роут выглядит как открытый, то есть замок зеленеет ровно там, где
 * дыра.
 *
 * @typedef {{ method: string, path: string, permissions: string[] }} BackendRoute
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROUTER_GO = path.resolve(HERE, '../../../../../internal/router/router.go');
// Ключи прав объявлены в двух файлах: страницы и действия в permission_keys.go,
// права вокруг заявок - рядом с каталогом.
const PERMISSION_KEY_FILES = [
  path.resolve(HERE, '../../../../../internal/services/permission_keys.go'),
  path.resolve(HERE, '../../../../../internal/services/permission_catalog.go'),
];

/**
 * Мидлварь-ограничитель частоты закрывает роут 429, а не 403, и правом не является:
 * принять её за гейт значило бы требовать тишины там, где отказа в правах не бывает.
 */
const RATE_LIMITER_RE = /limiter/i;

/** Имена, по которым узнаём мидлварь-гейт, чьё право вычисляется в рантайме. */
const DYNAMIC_GATE_RE = /(Gate|require[A-Z]|Only|Permission)/;

/** @returns {Record<string, string>} имя Go-константы -> строковый ключ права */
function parsePermissionKeys() {
  const keys = {};
  for (const file of PERMISSION_KEY_FILES) {
    const src = fs.readFileSync(file, 'utf8');
    for (const m of src.matchAll(/(Key\w+)\s*=\s*"([^"]+)"/g)) keys[m[1]] = m[2];
  }
  return keys;
}

const permissionKeys = parsePermissionKeys();

/**
 * Комментарии из исходника: в них встречаются и примеры вызовов роутов, и слова,
 * похожие на имена мидлварей, - разбирать их нельзя.
 *
 * @param {string} src
 * @returns {string} тот же текст, где содержимое комментариев заменено пробелами
 */
function stripComments(src) {
  let out = '';
  let mode = 'code';
  for (let i = 0; i < src.length; i += 1) {
    const two = src.slice(i, i + 2);
    if (mode === 'code') {
      if (two === '//') { mode = 'line'; out += '  '; i += 1; continue; }
      if (two === '/*') { mode = 'block'; out += '  '; i += 1; continue; }
      if (src[i] === '"' || src[i] === '`') mode = src[i];
      out += src[i];
      continue;
    }
    if (mode === 'line') {
      if (src[i] === '\n') { mode = 'code'; out += '\n'; continue; }
      out += ' ';
      continue;
    }
    if (mode === 'block') {
      if (two === '*/') { mode = 'code'; out += '  '; i += 1; continue; }
      out += src[i] === '\n' ? '\n' : ' ';
      continue;
    }
    // внутри строкового литерала
    if (src[i] === mode) mode = 'code';
    out += src[i];
  }
  return out;
}

/**
 * @param {string} src
 * @param {number} openIndex позиция `(`
 * @returns {{ text: string, end: number }} содержимое скобок и позиция закрывающей
 */
function balanced(src, openIndex) {
  let depth = 0;
  for (let i = openIndex; i < src.length; i += 1) {
    if (src[i] === '(') depth += 1;
    else if (src[i] === ')') {
      depth -= 1;
      if (depth === 0) return { text: src.slice(openIndex + 1, i), end: i };
    }
  }
  return { text: src.slice(openIndex + 1), end: src.length };
}

/** @param {string} text @returns {string[]} аргументы верхнего уровня */
function splitArguments(text) {
  const args = [];
  let depth = 0;
  let current = '';
  let quote = null;
  for (const ch of text) {
    if (quote) {
      current += ch;
      if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === '`') { quote = ch; current += ch; continue; }
    if ('([{'.includes(ch)) depth += 1;
    if (')]}'.includes(ch)) depth -= 1;
    if (ch === ',' && depth === 0) { args.push(current.trim()); current = ''; continue; }
    current += ch;
  }
  if (current.trim()) args.push(current.trim());
  return args;
}

/**
 * Мидлвари, собранные в слайс до объявления роута (`handlers := []echo.MiddlewareFunc{...}`
 * плюс последующие `append`): в роут они приходят как `handlers...`, и без раскрытия
 * право потерялось бы - так закрыт приём заполненного бланка (`action.import.list`).
 *
 * @param {string} src
 * @returns {Record<string, string[]>} имя слайса -> элементы
 */
function parseMiddlewareSlices(src) {
  const slices = {};
  for (const m of src.matchAll(/(\w+)\s*:=\s*\[\]echo\.MiddlewareFunc\{([^}]*)\}/g)) {
    slices[m[1]] = splitArguments(m[2]).filter(Boolean);
  }
  for (const m of src.matchAll(/(\w+)\s*=\s*append\(\s*\1\s*,([^)]*)\)/g)) {
    if (!slices[m[1]]) continue;
    slices[m[1]].push(...splitArguments(m[2]).filter(Boolean));
  }
  return slices;
}

/**
 * Права, которыми закрыт один аргумент-мидлварь.
 *
 * @param {string} arg
 * @param {Record<string, string>} middlewareVars имя переменной -> Go-константа права
 * @param {Record<string, string[]>} slices раскрытие `handlers...`
 * @returns {string[]}
 */
function permissionsOfMiddleware(arg, middlewareVars, slices) {
  const spread = arg.match(/^(\w+)\.\.\.$/);
  if (spread && slices[spread[1]]) {
    return slices[spread[1]].flatMap((item) => permissionsOfMiddleware(item, middlewareVars, slices));
  }

  const found = new Set();
  for (const [varName, goConst] of Object.entries(middlewareVars)) {
    if (new RegExp(`\\b${varName}\\b`).test(arg)) found.add(permissionKeys[goConst] ?? goConst);
  }
  for (const m of arg.matchAll(/services\.(Key\w+)/g)) found.add(permissionKeys[m[1]] ?? m[1]);
  if (found.size) return [...found];

  // Гейт, чьё право собирается в рантайме (права таблиц постов - `d.TableReportGate`
  // и родня). Ключа в тексте нет, но роут закрыт, и замку важно именно это.
  if (DYNAMIC_GATE_RE.test(arg) && !RATE_LIMITER_RE.test(arg)) return [`гейт в рантайме (${arg})`];
  return [];
}

/** @returns {BackendRoute[]} */
function parseRoutes() {
  const src = stripComments(fs.readFileSync(ROUTER_GO, 'utf8'));

  const middlewareVars = {};
  for (const m of src.matchAll(/(\w+)\s*:=\s*mw\.RequirePermissionV2\([^,]+,[^,]+,\s*services\.(Key\w+)\)/g)) {
    middlewareVars[m[1]] = m[2];
  }
  const slices = parseMiddlewareSlices(src);

  // Корень - сам экземпляр Echo (`e`), от него отходит `api := e.Group("/api")`,
  // а от неё - `protected` с проверкой токена.
  const groupPrefix = { e: '' };
  const groupPermissions = { e: [] };
  const routes = [];

  // Группы и роуты разбираем одним проходом: группа наследует префикс и права
  // родителя, поэтому к моменту объявления роута её строка уже прочитана.
  for (const m of src.matchAll(/(?:(\w+)\s*:=\s*)?(\w+)\.(Group|GET|POST|PUT|DELETE|PATCH)\(/g)) {
    const [, assignedTo, receiver, kind] = m;
    const { text } = balanced(src, m.index + m[0].length - 1);
    const args = splitArguments(text);
    const literal = args[0]?.match(/^"([^"]*)"$/);
    if (!literal) continue;

    if (kind === 'Group') {
      if (!assignedTo || !(receiver in groupPrefix)) continue;
      groupPrefix[assignedTo] = groupPrefix[receiver] + literal[1];
      groupPermissions[assignedTo] = [...new Set([
        ...groupPermissions[receiver],
        ...args.slice(1).flatMap((a) => permissionsOfMiddleware(a, middlewareVars, slices)),
      ])];
      continue;
    }

    if (!(receiver in groupPrefix)) continue;
    routes.push({
      method: kind,
      path: groupPrefix[receiver] + literal[1],
      permissions: [...new Set([
        ...groupPermissions[receiver],
        // args[1] - сам обработчик, гейты идут после него
        ...args.slice(2).flatMap((a) => permissionsOfMiddleware(a, middlewareVars, slices)),
      ])],
    });
  }
  return routes;
}

const routes = parseRoutes();

/** @returns {BackendRoute[]} все разобранные роуты - для проверки, что парсер жив */
export function backendRoutes() {
  return routes;
}

/**
 * Права, закрывающие путь, каким его зовёт фронт (`/users/all`, `/attachments/:x/cars`).
 *
 * Сегмент `:x` во фронтовом пути - подставляемое значение, он совпадает только с
 * параметром роута, но не с литеральным сегментом: иначе `/organizations/:x` ложно
 * совпал бы с `/organizations/suggest` и утащил чужой гейт. В обратную сторону
 * литерал важнее параметра, как и в самом Echo: `/attachments/all` идёт в свой
 * закрытый роут, а не в открытый `/attachments/:id`.
 *
 * @param {string} frontendPath путь без префикса `/api` и без query
 * @param {string} [method] метод запроса; у одного пути методы гейтятся по-разному
 *   (`GET /citizenships` открыт всем, `POST /citizenships` - под правом администратора)
 * @returns {string[]} права; пустой массив - путь открыт любому вошедшему
 */
export function gatesForPath(frontendPath, method = 'GET') {
  const segments = `/api${frontendPath}`.replace(/\/$/, '').split('/');
  let bestScore = -1;
  let gates = [];
  for (const route of routes) {
    if (route.method !== method) continue;
    const routeSegments = route.path.replace(/\/$/, '').split('/');
    if (routeSegments.length !== segments.length) continue;

    let score = 0;
    const fits = routeSegments.every((routeSegment, i) => {
      const called = segments[i];
      if (called === ':x') return routeSegment.startsWith(':');
      if (routeSegment === called) { score += 1; return true; }
      return routeSegment.startsWith(':');
    });
    if (!fits) continue;

    if (score > bestScore) { bestScore = score; gates = [...route.permissions]; }
    else if (score === bestScore) gates = [...new Set([...gates, ...route.permissions])];
  }
  return gates;
}
