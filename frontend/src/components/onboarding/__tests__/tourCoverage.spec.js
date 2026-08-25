import { describe, it, expect } from 'vitest';
import coverage from '../tourCoverage.json';
import { TOURS, allTourSteps, buildTourSteps } from '../tours';
import { MAIN_SECTIONS, ADMIN_GROUPS } from '@/constants/navSections';
import { authRoutePaths } from './routerGates';

/**
 * Замок реестра покрытия туров: новый роут или пункт меню обязан получить решение -
 * либо ссылку на шаг тура, либо `skip` с причиной. Отклонить нормально, промолчать
 * нет: именно молчание за год оставило тур без сквозного поиска, вопросов к заявке
 * и дополнения (см. раздел про онбординг в CLAUDE.md).
 *
 * Роуты читаются из исходника `router.js` (см. `routerGates.js`), а не импортом:
 * импорт потянул бы все представления и создал бы настоящий роутер ради списка путей.
 *
 * Второй замок, по ключам каталога прав, живёт на Go-стороне
 * (`internal/services/permission_catalog_test.go`) и читает этот же JSON.
 */

// Динамический сегмент тура охраны собирается по route фактовой таблицы: без
// заглушки его шаги (а с ними и их requires) в сборку не попадут.
const FACT_ROUTE_STUB = '/table/selector-lock';

const routeEntries = coverage.routes;
const permissionEntries = coverage.permissions;
// Элементы интерфейса, у которых нет ни своего роута, ни своего права: кнопка,
// открытая всем, кто и так видит экран. Полноту такого раздела автоматически не
// проверить (перечня «всех кнопок системы» не существует), но решение по каждому
// заведённому элементу проверяется наравне с остальными - иначе запись превращается
// в необязательный комментарий.
const elementEntries = coverage.elements ?? {};
const stepIds = new Set(allTourSteps().map((s) => s.id));
const tourKeys = new Set(TOURS.map((t) => t.key));
const navPaths = [...MAIN_SECTIONS, ...ADMIN_GROUPS.flatMap((g) => g.items)].map((i) => i.path);

/**
 * @param {object} entry запись реестра
 * @returns {boolean} ссылается ли запись на шаг тура
 */
function isCovered(entry) {
  return typeof entry?.tour === 'string';
}

describe('реестр покрытия туров - форма записей', () => {
  it('исходники прочитаны, иначе тест зелёный впустую', () => {
    expect(authRoutePaths().length).toBeGreaterThan(20);
    expect(stepIds.size).toBeGreaterThan(50);
    expect(Object.keys(permissionEntries).length).toBeGreaterThan(40);
  });

  const all = Object.entries({ ...routeEntries, ...permissionEntries, ...elementEntries });

  it('каждая запись - либо ссылка на шаг, либо skip с причиной, но не то и другое', () => {
    for (const [key, entry] of all) {
      const covered = isCovered(entry);
      expect(covered || typeof entry.skip === 'string', key).toBe(true);
      expect(covered && entry.skip !== undefined, `${key}: и tour, и skip`).toBe(false);
      if (covered) {
        expect(typeof entry.step, key).toBe('string');
      } else {
        // Причина пишется для человека, который через год спросит «почему этого нет
        // в обучении». Отписка в одно слово на этот вопрос не отвечает.
        expect(entry.skip.length, `${key}: слишком короткая причина`).toBeGreaterThan(20);
      }
    }
  });

  it('recheck - только у skip и только непустой строкой', () => {
    for (const [key, entry] of all) {
      if (entry.recheck === undefined) continue;
      expect(typeof entry.recheck, key).toBe('string');
      expect(entry.recheck.length, key).toBeGreaterThan(0);
      expect(isCovered(entry), `${key}: recheck у покрытой записи`).toBe(false);
    }
  });

  it('ссылка ведёт на существующий шаг существующего тура', () => {
    for (const [key, entry] of all.filter(([, e]) => isCovered(e))) {
      expect(tourKeys.has(entry.tour), `${key}: нет тура ${entry.tour}`).toBe(true);
      expect(stepIds.has(entry.step), `${key}: нет шага ${entry.step}`).toBe(true);
    }
  });

  it('в реестре нет записей без ключа-владельца', () => {
    const known = new Set(authRoutePaths());
    for (const key of Object.keys(routeEntries)) {
      expect(known.has(key), `${key}: роут удалён или переименован`).toBe(true);
    }
  });
});

describe('реестр покрытия туров - полнота', () => {
  it('каждый роут с requiresAuth имеет запись', () => {
    const missing = authRoutePaths().filter((p) => !routeEntries[p]);
    expect(missing, `нет записи в tourCoverage.json: ${missing.join(', ')}`).toEqual([]);
  });

  it('каждый пункт MAIN_SECTIONS и ADMIN_GROUPS имеет запись', () => {
    const missing = navPaths.filter((p) => !routeEntries[p]);
    expect(missing, `нет записи в tourCoverage.json: ${missing.join(', ')}`).toEqual([]);
  });

  it('пункт меню ведёт на реальный роут - иначе запись повисла бы в воздухе', () => {
    const known = new Set(authRoutePaths());
    const orphans = navPaths.filter((p) => !known.has(p));
    expect(orphans, `пункт меню без роута: ${orphans.join(', ')}`).toEqual([]);
  });

  it('право, объявленное шагом в requires, не числится отклонённым', () => {
    // Обратная сверка: реестр расходится с турами молча. Так `action.export.applications`
    // лежал в skip как «отчётная операция», хотя гейтит кнопку «Скачать» бланки и его
    // показывает шаг acc-detail-download. Шаг, который право ОБЪЯСНЯЕТ, и запись,
    // которая его ОТКЛОНЯЕТ, не могут сосуществовать.
    const violations = [];
    for (const tour of TOURS) {
      for (const step of buildTourSteps(tour, { factTableRoute: FACT_ROUTE_STUB })) {
        if (!step.requires) continue;
        const entry = permissionEntries[step.requires];
        if (!entry) {
          violations.push(`${step.requires}: нет записи, а её требует ${tour.key}/${step.id}`);
        } else if (!isCovered(entry)) {
          violations.push(`${step.requires}: числится skip, а его показывает ${tour.key}/${step.id}`);
        }
      }
    }
    expect([...new Set(violations)]).toEqual([]);
  });

  it('покрытие непустое: реестр не выродился в сплошные skip', () => {
    const covered = Object.values({ ...routeEntries, ...permissionEntries }).filter(isCovered);
    expect(covered.length).toBeGreaterThan(30);
  });
});
