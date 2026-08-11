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

/** @param {string} goConst @returns {string} читаемый ключ права либо имя константы */
function permissionName(goConst) {
  return permissionKeys[goConst] ?? goConst;
}

/**
 * Права, которыми закрыт кусок объявления роута или группы: и через переменную
 * (`requireUsers`), и напрямую (`mw.RequirePermissionV2(..., services.KeyX)`).
 *
 * @param {string} chunk
 * @param {Record<string, string>} middlewareVars имя переменной -> Go-константа права
 * @returns {string[]}
 */
function permissionsIn(chunk, middlewareVars) {
  const found = new Set();
  for (const [varName, goConst] of Object.entries(middlewareVars)) {
    if (new RegExp(`\\b${varName}\\b`).test(chunk)) found.add(permissionName(goConst));
  }
  for (const m of chunk.matchAll(/services\.(Key\w+)/g)) found.add(permissionName(m[1]));
  return [...found];
}

/** @returns {BackendRoute[]} */
function parseRoutes() {
  const src = fs.readFileSync(ROUTER_GO, 'utf8');

  const middlewareVars = {};
  for (const m of src.matchAll(/(\w+)\s*:=\s*mw\.RequirePermissionV2\([^,]+,[^,]+,\s*services\.(Key\w+)\)/g)) {
    middlewareVars[m[1]] = m[2];
  }

  // Группы наследуют и префикс пути, и права родителя: `/file-archive` закрыта целиком,
  // а внутри отдельные методы добавляют своё право поверх.
  const groupPrefix = { protected: '' };
  const groupPermissions = { protected: [] };
  for (const m of src.matchAll(/(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\s*(?:,\s*([^)]*))?\)/g)) {
    const [, name, parent, prefix, rest] = m;
    groupPrefix[name] = (groupPrefix[parent] ?? '') + prefix;
    groupPermissions[name] = [
      ...new Set([...(groupPermissions[parent] ?? []), ...permissionsIn(rest ?? '', middlewareVars)]),
    ];
  }

  const routes = [];
  for (const line of src.split('\n')) {
    const m = line.trim().match(/^(\w+)\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"\s*,\s*[\w.]+\s*(?:,\s*(.*?))?\)\s*$/);
    if (!m) continue;
    const [, group, method, routePath, rest] = m;
    if (!(group in groupPrefix)) continue;
    routes.push({
      method,
      path: groupPrefix[group] + routePath,
      permissions: [...new Set([...groupPermissions[group], ...permissionsIn(rest ?? '', middlewareVars)])],
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
 * Сегмент `:x` во фронтовом пути - подставляемое значение, он совпадает только с
 * параметром роута, но не с литеральным сегментом: иначе `/organizations/:x` ложно
 * совпал бы с `/organizations/suggest` и утащил чужой гейт.
 *
 * @param {string} frontendPath путь без префикса `/api` и без query
 * @param {string} [method] метод запроса; у одного пути методы гейтятся по-разному
 *   (`GET /citizenships` открыт всем, `POST /citizenships` - под правом администратора)
 * @returns {string[]} права; пустой массив - путь открыт любому вошедшему
 */
export function gatesForPath(frontendPath, method = 'GET') {
  const segments = `/api${frontendPath}`.replace(/\/$/, '').split('/');
  const gates = new Set();
  for (const route of routes) {
    if (route.method !== method) continue;
    const routeSegments = route.path.replace(/\/$/, '').split('/');
    if (routeSegments.length !== segments.length) continue;
    const fits = routeSegments.every((routeSegment, i) => {
      const called = segments[i];
      if (called === ':x') return routeSegment.startsWith(':');
      return routeSegment.startsWith(':') || routeSegment === called;
    });
    if (!fits) continue;
    // Путь совпал с открытым роутом (например тем же путём под другим методом) -
    // значит фронт мог звать именно его, и требовать тишины не за что.
    if (!route.permissions.length) return [];
    route.permissions.forEach((p) => gates.add(p));
  }
  return [...gates];
}
