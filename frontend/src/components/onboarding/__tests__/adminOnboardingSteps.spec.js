import { describe, it, expect, beforeEach } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { setActivePinia, createPinia } from 'pinia';
import { adminOnboardingSteps, ADMIN_ONBOARDING_VERSION } from '../adminOnboardingSteps';
import { collectSegment } from '../stepsFlow';
import { getTour, availableTours, buildTourSteps } from '../tours';
import { useOnboardingStore } from '@/stores/onboarding';
import { usePermissionsStore } from '@/stores/permissions';
import { routeGate } from './routerGates';

/**
 * Тур администратора: гейт, выпадение шагов по правам, целостность сегментов.
 *
 * Ключевая проверка тут - не «шаг существует», а «шаг достижим»: у Админки права
 * нарезаны по разделам, и шаг, прошедший фильтр `requires`, но ведущий на страницу
 * с более строгим гейтом, развернул бы администратора на /403 посреди тура.
 */

/** Право, которым закрыт сам тур (tours.js) - его имеет любой участник сценария. */
const TOUR_GATE = 'page.admin';

/**
 * @param {{ permissions?: string[] }} [role]
 * @returns {object} контекст гейтинга туров
 */
function ctx({ permissions = [] } = {}) {
  return {
    isAuthenticated: true,
    isSecurity: false,
    can: (key) => permissions.includes(key),
    approvalRole: { isApprover: false, isReviewer: false },
  };
}

/** Выдать перечисленные права текущему пользователю (режим normal). */
function grant(...keys) {
  const permissions = usePermissionsStore();
  permissions.mode = 'normal';
  permissions.effective = Object.fromEntries(keys.map((k) => [k, { value: 'allow', source: 'base' }]));
}

/** Все права, упомянутые в `requires` тура - чтобы «полный админ» не задавался числом. */
const STEP_RIGHTS = [...new Set(adminOnboardingSteps.filter((s) => s.requires).map((s) => s.requires))];

describe('adminOnboardingSteps - состав', () => {
  it('каждый шаг имеет непустые id, route, title и description', () => {
    for (const step of adminOnboardingSteps) {
      expect(typeof step.id, step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.route, step.id).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
      expect(step.title.length, step.id).toBeGreaterThan(0);
      expect(step.description.length, step.id).toBeGreaterThan(0);
    }
  });

  it('element - либо селектор по data-testid, либо null', () => {
    for (const step of adminOnboardingSteps) {
      const ok = step.element === null || /^\[data-testid="[^"]+"\]$/.test(step.element);
      expect(ok, `${step.id}: ${step.element}`).toBe(true);
    }
  });

  it('нет дублей id', () => {
    const ids = adminOnboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('открывается центр-модалом, версия - целое число', () => {
    expect(adminOnboardingSteps[0].element).toBe(null);
    expect(Number.isInteger(ADMIN_ONBOARDING_VERSION)).toBe(true);
    expect(ADMIN_ONBOARDING_VERSION).toBeGreaterThanOrEqual(1);
  });

  it('раскрытие колонки Админки просят только шаги, которые на неё смотрят', () => {
    const revealing = adminOnboardingSteps.filter((s) => s.reveal?.open);
    expect(revealing.length).toBeGreaterThan(0);
    for (const s of revealing) {
      expect(s.reveal.open, s.id).toBe('admin-column');
      // Цель шага должна жить В колонке - иначе раскрытие ничего не даёт.
      expect(['ob-admin-groups', 'ob-admin-search'].some((id) => s.element.includes(id)), s.id).toBe(true);
    }
  });

  it('шаг по узлу рельса раскрывает бургер-drawer и держит рельс развёрнутым', () => {
    // Рельс на <=768px уезжает за край: цель в нём без reveal.mobile подсвечивалась
    // бы офскрин. Цели в колонке Админки - отдельный fixed-слой, им это не нужно.
    const railSteps = adminOnboardingSteps.filter((s) => s.element?.includes('nav-link-'));
    expect(railSteps.length).toBeGreaterThan(0);
    for (const s of railSteps) {
      expect(s.reveal?.mobile, s.id).toBe('nav');
      expect(s.expandRail, s.id).toBe(true);
    }
    for (const s of adminOnboardingSteps.filter((s) => s.reveal?.open)) {
      expect(s.reveal.mobile, s.id).toBeUndefined();
    }
  });

  /**
   * Шаг-перечисление обещан реестром покрытия как место, где назван журнал
   * обращений: `tourCoverage.json` ссылается на него и роутом `/admin/requests`,
   * и правом `page.admin.monitoring`. Раздел там пропустили - реестр обещал
   * объяснение, которого в тексте не было. Проверяем поимённо, иначе такая
   * запись снова станет формальной.
   */
  it('шаг-перечисление называет разделы, которые обещает реестр покрытия', () => {
    const more = adminOnboardingSteps.find((s) => s.id === 'admin-more');
    ['Конструктор таблиц', 'Типы вложений', 'Файловый архив', 'Мониторинг запросов', 'Журнал отказов']
      .forEach((label) => expect(more.description, label).toContain(label));
  });

  it('финал - последний шаг: празднование и кнопка «Обучение» на «Обзоре»', () => {
    const last = adminOnboardingSteps[adminOnboardingSteps.length - 1];
    expect(last.celebrate).toBe(true);
    expect(last.route).toBe('/news');
    // Подсвечиваем саму кнопку запуска: финал говорит «пройти можно заново - вот отсюда».
    expect(last.element).toBe('[data-testid="ob-start-button"]');
    expect(last.requires).toBeUndefined();
    // Кнопки-перехода в раздел на финале больше нет (решение владельца 20.08).
    expect(last.cta).toBeUndefined();
    expect(last.ctaRoute).toBeUndefined();
    expect(adminOnboardingSteps.filter((s) => s.celebrate)).toHaveLength(1);
  });

  it('финал достижим любому участнику тура: его route и CTA закрыты гейтом тура', () => {
    const last = adminOnboardingSteps[adminOnboardingSteps.length - 1];
    expect(last.requires).toBeUndefined();
    expect(routeGate(last.route) ?? TOUR_GATE).toBe(TOUR_GATE);
    expect(routeGate(last.ctaRoute) ?? TOUR_GATE).toBe(TOUR_GATE);
  });
});

describe('adminOnboardingSteps - достижимость страниц', () => {
  // Гейт берём у РОУТА, а не у пункта меню: пункт «Обзор и новости» закрыт
  // page.news, а сам /news открыт любому вошедшему - по пункту вышло бы, что
  // вступительные шаги тура недостижимы.
  it('шаг с requires не ведёт на страницу, закрытую более строгим правом', () => {
    for (const step of adminOnboardingSteps) {
      const gate = routeGate(step.route);
      if (!gate || gate === TOUR_GATE) continue;
      expect(step.requires, `${step.id} -> ${step.route}`).toBe(gate);
    }
  });

  it('шаг без requires живёт только там, куда пускает гейт тура', () => {
    for (const step of adminOnboardingSteps.filter((s) => !s.requires)) {
      expect(routeGate(step.route) ?? TOUR_GATE, `${step.id} -> ${step.route}`).toBe(TOUR_GATE);
    }
  });

  it('шаг про раздел вне Админки гейтится правом этого раздела', () => {
    // Аналитику выдают и тем, кто Админку не открывает. Шаг рассказывает про неё,
    // стоя на странице настроек, поэтому право берётся у САМОГО раздела: без этого
    // администратор без page.statistics слушал бы про недоступный ему инструмент.
    const analytics = adminOnboardingSteps.find((s) => s.id === 'admin-analytics');
    expect(analytics.requires).toBe(routeGate('/analytics'));
    // Роут шага при этом остаётся под гейтом тура - выпадение права по правам не
    // должно уносить соседей по сегменту на недостижимую страницу.
    expect(routeGate(analytics.route) ?? TOUR_GATE).toBe(TOUR_GATE);
  });

  it('внутри сегмента шаги не смотрят все в одну точку', () => {
    // Один якорь на весь сегмент - тихая деградация: замок на селекторы зелёный,
    // а тур три шага подряд подсвечивает одну и ту же карточку целиком.
    const byRoute = new Map();
    for (const s of adminOnboardingSteps.filter((s) => s.element)) {
      byRoute.set(s.route, [...(byRoute.get(s.route) ?? []), s.element]);
    }
    for (const [route, elements] of byRoute) {
      if (elements.length < 2) continue;
      expect(new Set(elements).size, route).toBeGreaterThan(1);
    }
  });

  it('шаги одного route идут подряд; вернуться можно только финалом', () => {
    const seen = new Set();
    const segments = [];
    let i = 0;
    while (i < adminOnboardingSteps.length) {
      const { route } = adminOnboardingSteps[i];
      const length = collectSegment(adminOnboardingSteps, i, route).length;
      segments.push({ route, start: i, length });
      i += length;
    }
    segments.forEach((seg, idx) => {
      // Единственный разрешённый повтор - финальное возвращение на «Обзор»:
      // там живёт кнопка «Обучение», которой тур запускают заново.
      const isFinalSegment = idx === segments.length - 1;
      expect(seen.has(seg.route) && !isFinalSegment, `route ${seg.route} встречается вторым куском`).toBe(false);
      seen.add(seg.route);
    });
    // Сегментов заметно меньше, чем шагов: тур не бегает по странице на шаг.
    expect(segments.length).toBeLessThan(adminOnboardingSteps.length);
  });
});

describe('adminOnboardingSteps - якоря существуют в исходниках', () => {
  const SRC_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
  const SCANNED = new Set(['.vue', '.js', '.ts']);

  /**
   * Литеральные значения data-testid из разметки. Вхождения `[data-testid="x"]`
   * (CSS-селектор) отсекаются скобкой: иначе замок стерёг бы собственный текст.
   *
   * @returns {Set<string>}
   */
  function collectTestIds() {
    const found = new Set();
    const walk = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== '__tests__') walk(full);
          continue;
        }
        if (!SCANNED.has(path.extname(entry.name)) || entry.name.includes('.spec.')) continue;
        for (const m of fs.readFileSync(full, 'utf8').matchAll(/(^|[^[])data-testid\s*=\s*["']([^"'`${}]+)["']/gm)) {
          found.add(m[2]);
        }
      }
    };
    walk(SRC_DIR);
    return found;
  }

  const testIds = collectTestIds();

  it('исходники просканированы (иначе тест зелёный впустую)', () => {
    expect(testIds.size).toBeGreaterThan(100);
    expect(testIds.has('ob-anchor-that-never-existed')).toBe(false);
  });

  adminOnboardingSteps.filter((s) => s.element).forEach((step) => {
    it(`${step.id} -> ${step.element}`, () => {
      expect(testIds.has(step.element.match(/^\[data-testid="([^"]+)"\]$/)[1])).toBe(true);
    });
  });
});

describe('тур администратора в реестре', () => {
  it('виден по праву page.admin и не виден без него', () => {
    expect(availableTours(ctx({ permissions: [TOUR_GATE] })).map((t) => t.key)).toContain('admin');
    expect(availableTours(ctx()).map((t) => t.key)).not.toContain('admin');
  });

  it('право на отдельный раздел тура доступа не открывает', () => {
    expect(getTour('admin').isAvailable(ctx({ permissions: ['page.admin.users'] }))).toBe(false);
  });

  it('реестр отдаёт ровно написанный массив', () => {
    expect(buildTourSteps('admin')).toBe(adminOnboardingSteps);
  });
});

describe('выпадение шагов по правам', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  /**
   * @param {string[]} rights права администратора
   * @returns {Array<object>} шаги, дошедшие до пользователя
   */
  function stepsFor(rights) {
    grant(...rights);
    const store = useOnboardingStore();
    expect(store.start({ tour: 'admin' })).toBe(true);
    return store.steps;
  }

  it('полный администратор проходит тур целиком', () => {
    const steps = stepsFor([TOUR_GATE, ...STEP_RIGHTS]);
    expect(steps.map((s) => s.id)).toEqual(adminOnboardingSteps.map((s) => s.id));
    expect(useOnboardingStore().totalSteps).toBe(adminOnboardingSteps.length);
  });

  it('администратор без права на пользователей не получает ни одного шага про них', () => {
    const rights = [TOUR_GATE, ...STEP_RIGHTS.filter((r) => r !== 'page.admin.users')];
    const ids = stepsFor(rights).map((s) => s.id);
    expect(ids.some((id) => id.startsWith('admin-users-'))).toBe(false);
    expect(ids).toContain('admin-roles');
    expect(ids).toContain('admin-finish');
  });

  it('у каждого гейтящего права свои шаги, и без него исчезают только они', () => {
    for (const right of STEP_RIGHTS) {
      setActivePinia(createPinia());
      const dropped = adminOnboardingSteps.filter((s) => s.requires === right).map((s) => s.id);
      expect(dropped.length, right).toBeGreaterThan(0);
      const ids = stepsFor([TOUR_GATE, ...STEP_RIGHTS.filter((r) => r !== right)]).map((s) => s.id);
      expect(ids, right).toEqual(adminOnboardingSteps.map((s) => s.id).filter((id) => !dropped.includes(id)));
    }
  });

  it('нумерация «шаг N из M» не разъезжается: выпавшие шаги не занимают номер', () => {
    const store = useOnboardingStore();
    grant(TOUR_GATE);
    store.start({ tour: 'admin' });

    // Голый администратор: остаются только шаги без requires.
    const expected = adminOnboardingSteps.filter((s) => !s.requires);
    expect(store.totalSteps).toBe(expected.length);
    expect(store.totalSteps).toBeLessThan(adminOnboardingSteps.length);

    expected.forEach((step, index) => {
      store.setIndex(index);
      expect(store.currentStep.id).toBe(step.id);
    });
    // Последний номер - всё ещё финал, а не «дыра» от выпавшего шага.
    store.setIndex(store.totalSteps - 1);
    expect(store.currentStep.celebrate).toBe(true);
  });

  it('колонка Админки остаётся объяснённой при любом наборе прав', () => {
    const ids = stepsFor([TOUR_GATE]).map((s) => s.id);
    expect(ids).toEqual(expect.arrayContaining(['admin-open', 'admin-groups', 'admin-search']));
  });
});
