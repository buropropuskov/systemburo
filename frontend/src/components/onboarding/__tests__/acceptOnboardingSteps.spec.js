import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { acceptOnboardingSteps, ACCEPT_ONBOARDING_VERSION } from '../acceptOnboardingSteps';
import { collectSegment } from '../stepsFlow';
import { getTour, availableTours, buildTourSteps, allTourSteps } from '../tours';
import { useOnboardingStore } from '@/stores/onboarding';
import { usePermissionsStore } from '@/stores/permissions';

/**
 * Тур принимающего: гейт по роли, выпадение шагов по правам, целостность сегментов
 * и замок против смешения ролей.
 *
 * Парный к approveOnboardingSteps.spec.js. Принимающий берёт заявку в работу и
 * распределяет её по постам, голосуют - согласующие; путаница тем опаснее, что в
 * коде слова стоят наоборот (`models.ApplicationApprover` - принимающий), а
 * селекторный замок смешение ролей не ловит: якорь существует, просто в чужой роли.
 */

/** Роль в согласовании, которой открыт тур (tours.js). */
const TOUR_KEY = 'accept';

/** Шаги, живущие в модалке карточки заявки: их открывает reveal, их же и пропускает. */
const CARD_STEPS = acceptOnboardingSteps.filter((s) => s.reveal?.open === 'first-application');

/** Права, упомянутые в `requires` тура - чтобы «полный принимающий» не задавался числом. */
const STEP_RIGHTS = [...new Set(acceptOnboardingSteps.filter((s) => s.requires).map((s) => s.requires))];

/**
 * @param {{ permissions?: string[], isSecurity?: boolean, isApprover?: boolean, isReviewer?: boolean }} [role]
 * @returns {object} контекст гейтинга туров
 */
function ctx({ permissions = [], isSecurity = false, isApprover = false, isReviewer = false } = {}) {
  return {
    isAuthenticated: true,
    isSecurity,
    can: (key) => permissions.includes(key),
    approvalRole: { isApprover, isReviewer },
  };
}

/** Выдать перечисленные права текущему пользователю (режим normal). */
function grant(...keys) {
  const permissions = usePermissionsStore();
  permissions.mode = 'normal';
  permissions.effective = Object.fromEntries(keys.map((k) => [k, { value: 'allow', source: 'base' }]));
}

describe('acceptOnboardingSteps - состав', () => {
  it('каждый шаг имеет непустые id, route, title и description', () => {
    for (const step of acceptOnboardingSteps) {
      expect(typeof step.id, step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.route, step.id).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
      expect(step.title.length, step.id).toBeGreaterThan(0);
      expect(step.description.length, step.id).toBeGreaterThan(0);
    }
  });

  it('element - либо селектор по data-testid, либо null', () => {
    for (const step of acceptOnboardingSteps) {
      const ok = step.element === null || /^\[data-testid="[^"]+"\]$/.test(step.element);
      expect(ok, `${step.id}: ${step.element}`).toBe(true);
    }
  });

  it('нет дублей id', () => {
    const ids = acceptOnboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('открывается центр-модалом на «Обзоре» - иначе автозапуск не сработает', () => {
    // maybeAutostart (OnboardingTour) стартует тур только на /news, а startSegment
    // поднимает сегмент ТЕКУЩЕГО роута: первый шаг вне /news дал бы пустой сегмент
    // и мгновенную остановку тура.
    expect(acceptOnboardingSteps[0].route).toBe('/news');
    expect(acceptOnboardingSteps[0].element).toBe(null);
    expect(Number.isInteger(ACCEPT_ONBOARDING_VERSION)).toBe(true);
    expect(ACCEPT_ONBOARDING_VERSION).toBeGreaterThanOrEqual(1);
  });

  it('финал - последний шаг, помечен celebrate и уводит в Центр заявок', () => {
    const last = acceptOnboardingSteps[acceptOnboardingSteps.length - 1];
    expect(last.celebrate).toBe(true);
    expect(last.element).toBe(null);
    expect(last.requires).toBeUndefined();
    expect(last.ctaRoute).toBe('/center');
    expect(last.cta.length).toBeGreaterThan(0);
    expect(acceptOnboardingSteps.filter((s) => s.celebrate)).toHaveLength(1);
  });

  it('шаг по узлу рельса раскрывает бургер-drawer и держит рельс развёрнутым', () => {
    const railSteps = acceptOnboardingSteps.filter((s) => s.element?.includes('nav-link-'));
    expect(railSteps.length).toBeGreaterThan(0);
    for (const s of railSteps) {
      expect(s.reveal?.mobile, s.id).toBe('nav');
      expect(s.expandRail, s.id).toBe(true);
    }
  });
});

describe('acceptOnboardingSteps - сегменты и достижимость', () => {
  it('шаги одного route идут подряд - cross-page навигация не рвётся', () => {
    const seen = new Set();
    let i = 0;
    while (i < acceptOnboardingSteps.length) {
      const { route } = acceptOnboardingSteps[i];
      expect(seen.has(route), `route ${route} встречается вторым куском`).toBe(false);
      seen.add(route);
      i += collectSegment(acceptOnboardingSteps, i, route).length;
    }
    expect(seen.size).toBe(2);
    expect([...seen]).toEqual(['/news', '/center']);
  });

  it('сегмент Центра помечен optionalSegment - без права page.center тур не виснет', () => {
    const centerSteps = acceptOnboardingSteps.filter((s) => s.route === '/center');
    expect(centerSteps[0].optionalSegment).toBe(true);
    expect(centerSteps.slice(1).some((s) => s.optionalSegment)).toBe(false);
  });

  it('внутри сегмента шаги не смотрят все в одну точку', () => {
    const byRoute = new Map();
    for (const s of acceptOnboardingSteps.filter((s) => s.element)) {
      byRoute.set(s.route, [...(byRoute.get(s.route) ?? []), s.element]);
    }
    for (const [route, elements] of byRoute) {
      if (elements.length < 2) continue;
      expect(new Set(elements).size, route).toBe(elements.length);
    }
  });

  it('шаги тура попадают в общий замок существования селекторов', () => {
    // Сам замок - tourSelectors.spec.js по allTourSteps(). Здесь проверяем то, чего
    // он проверить не может: что этот тур в обход не прошёл. Тур, забытый в реестре,
    // дал бы зелёный замок при полностью выдуманных якорях.
    const guarded = new Set(allTourSteps().map((s) => s.id));
    for (const step of acceptOnboardingSteps) {
      expect(guarded.has(step.id), step.id).toBe(true);
    }
  });
});

describe('acceptOnboardingSteps - карточка заявки', () => {
  it('шаги карточки просят её открыть и переживают пустой Центр', () => {
    expect(CARD_STEPS.length).toBeGreaterThan(3);
    for (const step of CARD_STEPS) {
      expect(step.optional, step.id).toBe(true);
      expect(step.route, step.id).toBe('/center');
    }
  });

  it('reveal стоит на каждом шаге карточки, а не только на первом', () => {
    const indexes = CARD_STEPS.map((s) => acceptOnboardingSteps.indexOf(s));
    expect(indexes).toEqual(indexes.map((_, i) => indexes[0] + i));
  });

  it('шаг после карточки её не удерживает - Центр возвращается сам', () => {
    // Архив и финал живут в Центре, а не в карточке: собственного reveal у них нет,
    // и lookahead удержать раскрытие не может (соседи разные) - карточка закроется.
    const lastCard = acceptOnboardingSteps.indexOf(CARD_STEPS[CARD_STEPS.length - 1]);
    for (const step of acceptOnboardingSteps.slice(lastCard + 1)) {
      expect(step.reveal?.open, step.id).toBeUndefined();
    }
  });

  it('без карточки тур остаётся связным: остаются вход, Центр, архив и финал', () => {
    const ids = acceptOnboardingSteps.filter((s) => !CARD_STEPS.includes(s)).map((s) => s.id);
    expect(ids[0]).toBe(acceptOnboardingSteps[0].id);
    expect(ids[ids.length - 1]).toBe(acceptOnboardingSteps[acceptOnboardingSteps.length - 1].id);
    expect(ids).toEqual(expect.arrayContaining(['acc-center-header', 'acc-center-archive']));
  });
});

describe('тур принимающего в реестре', () => {
  // page.center обязателен: приём заявок идёт из центра, там же весь тур.
  it('виден принимающему', () => {
    expect(availableTours(ctx({ isApprover: true, permissions: ['page.center'] })).map((t) => t.key))
      .toContain(TOUR_KEY);
  });

  it('не виден принимающему без доступа в центр заявок', () => {
    expect(availableTours(ctx({ isApprover: true })).map((t) => t.key)).not.toContain(TOUR_KEY);
  });

  it('не виден чужим ролям', () => {
    const cases = {
      'обычный пользователь': ctx(),
      охранник: ctx({ isSecurity: true }),
      согласующий: ctx({ isReviewer: true }),
      администратор: ctx({ permissions: ['page.admin'] }),
    };
    for (const [who, context] of Object.entries(cases)) {
      expect(availableTours(context).map((t) => t.key), who).not.toContain(TOUR_KEY);
    }
  });

  it('право согласования тур принимающего не открывает', () => {
    // Гейт тура - членство в справочнике принимающих, а не право: у согласующего
    // право action.approve.application есть, и по нему ему открывается СВОЙ тур.
    expect(getTour(TOUR_KEY).isAvailable(ctx({ permissions: ['action.approve.application'] }))).toBe(false);
  });

  it('реестр отдаёт ровно написанный массив', () => {
    expect(buildTourSteps(TOUR_KEY)).toBe(acceptOnboardingSteps);
    expect(getTour(TOUR_KEY).version).toBe(ACCEPT_ONBOARDING_VERSION);
  });
});

describe('выпадение шагов по правам', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  /**
   * @param {string[]} rights права принимающего
   * @returns {Array<object>} шаги, дошедшие до пользователя
   */
  function stepsFor(rights) {
    grant(...rights);
    const store = useOnboardingStore();
    expect(store.start({ tour: TOUR_KEY })).toBe(true);
    return store.steps;
  }

  it('в туре есть шаги под правами - иначе матрица ниже проверяет пустоту', () => {
    expect(STEP_RIGHTS).toEqual(expect.arrayContaining([
      'application.organization.moderate',
      'action.export.applications',
      'center.archive',
    ]));
  });

  it('принимающий со всеми правами проходит тур целиком', () => {
    const steps = stepsFor(STEP_RIGHTS);
    expect(steps.map((s) => s.id)).toEqual(acceptOnboardingSteps.map((s) => s.id));
    expect(useOnboardingStore().totalSteps).toBe(acceptOnboardingSteps.length);
  });

  it('у каждого гейтящего права свои шаги, и без него исчезают только они', () => {
    for (const right of STEP_RIGHTS) {
      setActivePinia(createPinia());
      const dropped = acceptOnboardingSteps.filter((s) => s.requires === right).map((s) => s.id);
      expect(dropped.length, right).toBeGreaterThan(0);
      const ids = stepsFor(STEP_RIGHTS.filter((r) => r !== right)).map((s) => s.id);
      expect(ids, right).toEqual(acceptOnboardingSteps.map((s) => s.id).filter((id) => !dropped.includes(id)));
    }
  });

  it('без права на архив исчезает только шаг архива', () => {
    const ids = stepsFor(STEP_RIGHTS.filter((r) => r !== 'center.archive')).map((s) => s.id);
    expect(ids).not.toContain('acc-center-archive');
    expect(ids).toContain('acc-detail-take');
    expect(ids[ids.length - 1]).toBe('acc-finish');
  });

  it('нумерация «шаг N из M» не разъезжается: выпавшие шаги не занимают номер', () => {
    const store = useOnboardingStore();
    grant();
    store.start({ tour: TOUR_KEY });

    const expected = acceptOnboardingSteps.filter((s) => !s.requires);
    expect(store.totalSteps).toBe(expected.length);
    expect(store.totalSteps).toBeLessThan(acceptOnboardingSteps.length);

    expected.forEach((step, index) => {
      store.setIndex(index);
      expect(store.currentStep.id).toBe(step.id);
    });
    store.setIndex(store.totalSteps - 1);
    expect(store.currentStep.celebrate).toBe(true);
  });

  it('приём в работу и назначение постов остаются объяснёнными при пустом наборе прав', () => {
    const ids = stepsFor([]).map((s) => s.id);
    expect(ids).toEqual(expect.arrayContaining([
      'acc-nav-center',
      'acc-center-header',
      'acc-detail-take',
      'acc-detail-assign',
      'acc-detail-supplement',
    ]));
  });
});

describe('замок против смешения ролей', () => {
  /**
   * Кнопки, которые интерфейс рисует ТОЛЬКО согласующему (`isResponsibleUser` в
   * ApplicationActionBar, `canForwardApplication` и `can-override` в
   * ApplicationDetail). Шаг принимающего на такой якорь молча пропадёт у всей
   * аудитории тура - цели у неё нет.
   */
  const REVIEWER_ONLY_ANCHORS = [
    'app-detail-button-approve',
    'app-detail-button-revoke-approval',
    'app-detail-button-forward',
    'supplement-button-approve',
    'supplement-button-reject',
    'blacklist-override-btn',
  ];

  /** Подписи кнопок согласующего: обещать их принимающему нельзя даже словами. */
  const REVIEWER_ONLY_LABELS = [
    '«Согласовать»',
    '«Согласовать и принять»',
    '«Согласовать дополнение»',
    '«Отказать в дополнении»',
    '«Отозвать своё решение»',
    '«Переслать»',
    '«Пропустить»',
  ];

  it('тур не смотрит ни на одну кнопку согласующего', () => {
    for (const step of acceptOnboardingSteps.filter((s) => s.element)) {
      const anchor = step.element.match(/^\[data-testid="([^"]+)"\]$/)[1];
      expect(REVIEWER_ONLY_ANCHORS, step.id).not.toContain(anchor);
    }
  });

  it('тексты не выдают голосование за работу принимающего', () => {
    for (const step of acceptOnboardingSteps) {
      const text = `${step.title} ${step.description}`;
      for (const label of REVIEWER_ONLY_LABELS) {
        expect(text.includes(label), `${step.id}: ${label}`).toBe(false);
      }
    }
  });

  it('приём в работу в туре действительно есть - иначе замок стережёт пустоту', () => {
    // Ряд действий (ob-detail-actions) общий у обеих ролей - состав кнопок в нём
    // зависит от роли, поэтому сам якорь роль не доказывает. Доказывают шаги.
    const ids = acceptOnboardingSteps.map((s) => s.id);
    expect(ids).toContain('acc-detail-take');
    const anchors = acceptOnboardingSteps.filter((s) => s.element)
      .map((s) => s.element.match(/^\[data-testid="([^"]+)"\]$/)[1]);
    expect(anchors).toContain('ob-detail-actions');
    // Доназначение постов - якорь, которого у согласующего нет вовсе.
    expect(anchors).toContain('attachment-assign-open');
  });
});
