import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Гейты роутов, прочитанные из исходника `router.js`.
 *
 * Читаем текстом, а не импортом: импорт потянул бы все представления и создал бы
 * настоящий роутер ради списка путей. Тот же приём, что у замка на селекторы шагов
 * (`tourSelectors.spec.js`).
 *
 * Источник правды по достижимости страницы - именно `meta.permission` роута, а не
 * `permission` пункта в `navSections.js`: последний гейтит только видимость пункта
 * в меню. У `/news` пункт закрыт `page.news`, а сам роут открыт любому вошедшему.
 */

const ROUTER_FILE = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../../../router.js',
);

/** @returns {Array<{ path: string, permission: string|null }>} роуты с requiresAuth */
function parseAuthRoutes() {
  const src = fs.readFileSync(ROUTER_FILE, 'utf8');
  const marks = [...src.matchAll(/path:\s*'([^']+)'/g)];
  const routes = [];
  marks.forEach((m, i) => {
    const chunk = src.slice(m.index, i + 1 < marks.length ? marks[i + 1].index : src.length);
    if (!/requiresAuth:\s*true/.test(chunk)) return;
    routes.push({ path: m[1], permission: chunk.match(/permission:\s*'([^']+)'/)?.[1] ?? null });
  });
  return routes;
}

const authRoutes = parseAuthRoutes();

/** @returns {string[]} пути роутов, требующих авторизации */
export function authRoutePaths() {
  return authRoutes.map((r) => r.path);
}

/**
 * @param {string} routePath
 * @returns {string|null} право, закрывающее роут; null - открыт любому вошедшему
 */
export function routeGate(routePath) {
  return authRoutes.find((r) => r.path === routePath)?.permission ?? null;
}
