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

/**
 * Мета-флаги, которыми доступность роута закрыта ПОМИМО `permission`: гард
 * проверяет их отдельной ветвью, и по одному `permission` такой роут выглядит
 * открытым любому вошедшему. Замки достижимости обязаны их видеть, иначе шаг,
 * ведущий на закрытую страницу, проходит проверку и разворачивает человека
 * посреди тура (#7: так четыре шага тура администратора уехали на /403, а тур
 * охранника терял шесть шагов и финал у работника с одним page.tables).
 */
const EXTRA_GATE_FLAGS = ['requiresSuperAdmin', 'requiresSecurityOrAdmin'];

/**
 * @returns {Array<{ path: string, permission: string|null, extraGate: string|null, component: string|null }>}
 *   роуты с requiresAuth; `component` - путь импорта, если его удалось прочитать
 */
function parseAuthRoutes() {
  const src = fs.readFileSync(ROUTER_FILE, 'utf8');
  // Компоненты роутов объявлены двумя способами: ленивым import() прямо в объекте
  // роута и статическим импортом по имени в шапке файла.
  const staticImports = {};
  for (const m of src.matchAll(/import\s+(\w+)\s+from\s+'([^']+)'/g)) staticImports[m[1]] = m[2];

  const marks = [...src.matchAll(/path:\s*'([^']+)'/g)];
  const routes = [];
  marks.forEach((m, i) => {
    const chunk = src.slice(m.index, i + 1 < marks.length ? marks[i + 1].index : src.length);
    if (!/requiresAuth:\s*true/.test(chunk)) return;
    const lazy = chunk.match(/component:\s*\(\)\s*=>\s*import\('([^']+)'\)/)?.[1];
    const named = chunk.match(/component:\s*(\w+)/)?.[1];
    routes.push({
      path: m[1],
      permission: chunk.match(/permission:\s*'([^']+)'/)?.[1] ?? null,
      // Гейт бывает и функцией (`(to) => \`table.${to.params.tableName}.view\``) -
      // ключа в тексте нет, но страница закрыта, и считать её открытой нельзя.
      gated: /permission:/.test(chunk),
      extraGate: EXTRA_GATE_FLAGS.find((f) => new RegExp(`${f}:\\s*true`).test(chunk)) ?? null,
      component: lazy ?? staticImports[named] ?? null,
    });
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

/**
 * Имя мета-флага, которым роут закрыт помимо `permission`. Гейт тура обязан
 * покрывать этот флаг: пропустить его - значит пустить в тур человека, которого
 * роут-гард развернёт на первом же шаге такой страницы.
 *
 * @param {string} routePath
 * @returns {string|null} имя флага либо null, если дополнительного гейта нет
 */
export function routeExtraGate(routePath) {
  return authRoutes.find((r) => r.path === routePath)?.extraGate ?? null;
}

/**
 * Пути импорта компонентов, чьи страницы открыты любому вошедшему: ни `permission`,
 * ни дополнительного мета-флага. С них и начинается граф пользовательских экранов в
 * замке `api/__tests__/adminEndpointsFromUserFlows.spec.js` - разбирать `router.js`
 * во второй раз незачем, разошлись бы списком дополнительных флагов.
 *
 * @returns {string[]}
 */
export function ungatedRouteComponents() {
  return authRoutes.filter((r) => !r.gated && !r.extraGate && r.component).map((r) => r.component);
}
