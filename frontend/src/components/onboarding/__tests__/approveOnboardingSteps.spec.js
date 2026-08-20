import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { approveOnboardingSteps, APPROVE_ONBOARDING_VERSION } from '../approveOnboardingSteps';
import { collectSegment } from '../stepsFlow';
import { getTour, availableTours, buildTourSteps, allTourSteps } from '../tours';
import { useOnboardingStore } from '@/stores/onboarding';
import { usePermissionsStore } from '@/stores/permissions';

/**
 * Тур согласующего: гейт по роли, выпадение шагов по правам, целостность сегментов
 * и замок против смешения ролей.
 *
 * Смешение ролей - главный риск именно этой пары туров: согласующий голосует, а в
 * работу заявку берёт принимающий, и в коде слова стоят наоборот
 * (`models.ApplicationApprover` - это принимающий). Тур, пообещавший согласующему
 * кнопку «Принять», отправит человека искать то, чего у него нет, и ни один
 * селекторный замок этого не увидит: якорь существует, просто в чужой роли.
 */

/** Роль в согласовании, которой открыт тур (tours.js). */
const TOUR_KEY = 'approve';

/** Шаги, живущие в модалке карточки заявки: их открывает reveal, их же и пропускает. */
// Шаги, идущие поверх открытой карточки: сама карточка (first-application) и
// журнал, который из неё открывается, - для карточки это одна группа.
const CARD_OPEN_TARGETS = ['first-application', 'application-history'];
const CARD_STEPS = approveOnboardingSteps.filter((s) => CARD_OPEN_TARGETS.includes(s.reveal?.open));

/** Права, упомянутые в `requires` тура - чтобы «полный согласующий» не задавался числом. */
const STEP_RIGHTS = [...new Set(approveOnboardingSteps.filter((s) => s.requires).map((s) => s.requires))];

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

describe('approveOnboardingSteps - состав', () => {
  it('каждый шаг имеет непустые id, route, title и description', () => {
    for (const step of approveOnboardingSteps) {
      expect(typeof step.id, step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.route, step.id).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
      expect(step.title.length, step.id).toBeGreaterThan(0);
      expect(step.description.length, step.id).toBeGreaterThan(0);
    }
  });

  it('element - либо селектор по data-testid, либо null', () => {
    for (const step of approveOnboardingSteps) {
      const ok = step.element === null || /^\[data-testid="[^"]+"\]$/.test(step.element);
      expect(ok, `${step.id}: ${step.element}`).toBe(true);
    }
  });

  it('нет дублей id', () => {
    const ids = approveOnboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('открывается центр-модалом на «Обзоре» - иначе автозапуск не сработает', () => {
    // maybeAutostart (OnboardingTour) стартует тур только на /news, а startSegment
    // поднимает сегмент ТЕКУЩЕГО роута: первый шаг вне /news дал бы пустой сегмент
    // и мгновенную остановку тура.
    expect(approveOnboardingSteps[0].route).toBe('/news');
    expect(approveOnboardingSteps[0].element).toBe(null);
    expect(Number.isInteger(APPROVE_ONBOARDING_VERSION)).toBe(true);
    expect(APPROVE_ONBOARDING_VERSION).toBeGreaterThanOrEqual(1);
  });

  it('финал - последний шаг, помечен celebrate и уводит в Центр заявок', () => {
    const last = approveOnboardingSteps[approveOnboardingSteps.length - 1];
    expect(last.celebrate).toBe(true);
    expect(last.element).toBe(null);
    expect(last.requires).toBeUndefined();
    expect(last.ctaRoute).toBe('/center');
    expect(last.cta.length).toBeGreaterThan(0);
    // celebrate ровно один: два празднования в одном туре - это сломанный порядок.
    expect(approveOnboardingSteps.filter((s) => s.celebrate)).toHaveLength(1);
  });

  it('шаг по узлу рельса раскрывает бургер-drawer и держит рельс развёрнутым', () => {
    const railSteps = approveOnboardingSteps.filter((s) => s.element?.includes('nav-link-'));
    expect(railSteps.length).toBeGreaterThan(0);
    for (const s of railSteps) {
      expect(s.reveal?.mobile, s.id).toBe('nav');
      expect(s.expandRail, s.id).toBe(true);
    }
  });
});

describe('approveOnboardingSteps - сегменты и достижимость', () => {
  it('шаги одного route идут подряд - cross-page навигация не рвётся', () => {
    const seen = new Set();
    let i = 0;
    while (i < approveOnboardingSteps.length) {
      const { route } = approveOnboardingSteps[i];
      expect(seen.has(route), `route ${route} встречается вторым куском`).toBe(false);
      seen.add(route);
      i += collectSegment(approveOnboardingSteps, i, route).length;
    }
    expect(seen.size).toBe(2);
    expect([...seen]).toEqual(['/news', '/center']);
  });

  it('сегмент Центра помечен optionalSegment - без права page.center тур не виснет', () => {
    // Раздел закрыт правом page.center. Без optionalSegment на первом шаге сегмента
    // роут-гард увёл бы человека, а хост остался бы ждать недостижимую страницу.
    const centerSteps = approveOnboardingSteps.filter((s) => s.route === '/center');
    expect(centerSteps[0].optionalSegment).toBe(true);
    // Метка нужна ровно на границе сегмента - на остальных шагах она бессмысленна.
    expect(centerSteps.slice(1).some((s) => s.optionalSegment)).toBe(false);
  });

  it('внутри сегмента шаги не смотрят все в одну точку', () => {
    const byRoute = new Map();
    for (const s of approveOnboardingSteps.filter((s) => s.element)) {
      byRoute.set(s.route, [...(byRoute.get(s.route) ?? []), s.element]);
    }
    for (const [route, elements] of byRoute) {
      if (elements.length < 2) continue;
      expect(new Set(elements).size, route).toBe(elements.length);
    }
  });

  it('шаги тура попадают в общий замок существования селекторов', () => {
    // Сам замок - tourSelectors.spec.js: он обходит allTourSteps(). Здесь проверяем
    // то, чего он проверить не может: что этот тур в обход не прошёл. Тур, забытый в
    // реестре, дал бы зелёный замок при полностью выдуманных якорях.
    const guarded = new Set(allTourSteps().map((s) => s.id));
    for (const step of approveOnboardingSteps) {
      expect(guarded.has(step.id), step.id).toBe(true);
    }
  });
});

describe('approveOnboardingSteps - карточка заявки', () => {
  it('шаги карточки просят её открыть и переживают пустой Центр', () => {
    // Деталь - модалка внутри /center, тур открывает её сигналом reveal. Заявок у
    // человека может не быть вовсе, поэтому каждый такой шаг обязан быть optional:
    // иначе на пустом Центре он ждал бы цель полный таймаут и вырождался в поповер
    // по центру экрана с рассказом про несуществующую карточку.
    expect(CARD_STEPS.length).toBeGreaterThan(3);
    for (const step of CARD_STEPS) {
      expect(step.optional, step.id).toBe(true);
      expect(step.route, step.id).toBe('/center');
    }
  });

  it('reveal стоит на каждом шаге карточки, а не только на первом', () => {
    // Lookahead в resolveReveal лишь УДЕРЖИВАЕТ раскрытие внутри группы одинаковых
    // шагов. Шаг без своего значения на краю группы закрыл бы карточку посреди
    // рассказа о ней - поэтому подряд идущая группа обязана быть сплошной.
    const indexes = CARD_STEPS.map((s) => approveOnboardingSteps.indexOf(s));
    expect(indexes).toEqual(indexes.map((_, i) => indexes[0] + i));
  });

  it('шаг после карточки её не удерживает - Центр возвращается сам', () => {
    // Уведомления и финал живут в Центре, а не в карточке: собственного reveal у них
    // нет, и lookahead удержать раскрытие не может (соседи разные) - карточка закроется.
    const lastCard = approveOnboardingSteps.indexOf(CARD_STEPS[CARD_STEPS.length - 1]);
    for (const step of approveOnboardingSteps.slice(lastCard + 1)) {
      expect(step.reveal?.open, step.id).toBeUndefined();
    }
  });

  it('без карточки тур остаётся связным: остаются вход, Центр и финал', () => {
    const ids = approveOnboardingSteps.filter((s) => !CARD_STEPS.includes(s)).map((s) => s.id);
    expect(ids[0]).toBe(approveOnboardingSteps[0].id);
    expect(ids[ids.length - 1]).toBe(approveOnboardingSteps[approveOnboardingSteps.length - 1].id);
    expect(ids).toContain('apr-center-header');
    expect(ids.length).toBeGreaterThan(3);
  });
});

describe('тур согласующего в реестре', () => {
  // page.center обязателен обеим веткам гейта: тур почти целиком живёт на /center,
  // и без доступа туда показывать его нечего.
  it('виден согласующему и тому, кому выдано право согласования', () => {
    expect(availableTours(ctx({ isReviewer: true, permissions: ['page.center'] })).map((t) => t.key))
      .toContain(TOUR_KEY);
    expect(availableTours(ctx({ permissions: ['action.approve.application', 'page.center'] })).map((t) => t.key))
      .toContain(TOUR_KEY);
  });

  it('не виден согласующему без доступа в центр заявок', () => {
    expect(availableTours(ctx({ isReviewer: true })).map((t) => t.key)).not.toContain(TOUR_KEY);
  });

  it('не виден чужим ролям', () => {
    const cases = {
      'обычный пользователь': ctx(),
      охранник: ctx({ isSecurity: true }),
      принимающий: ctx({ isApprover: true }),
      администратор: ctx({ permissions: ['page.admin'] }),
    };
    for (const [who, context] of Object.entries(cases)) {
      expect(availableTours(context).map((t) => t.key), who).not.toContain(TOUR_KEY);
    }
  });

  it('реестр отдаёт ровно написанный массив', () => {
    expect(buildTourSteps(TOUR_KEY)).toBe(approveOnboardingSteps);
    expect(getTour(TOUR_KEY).version).toBe(APPROVE_ONBOARDING_VERSION);
  });
});

describe('выпадение шагов по правам', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  /**
   * @param {string[]} rights права согласующего
   * @returns {Array<object>} шаги, дошедшие до пользователя
   */
  function stepsFor(rights) {
    grant(...rights);
    const store = useOnboardingStore();
    expect(store.start({ tour: TOUR_KEY })).toBe(true);
    return store.steps;
  }

  it('в туре есть шаги под правами - иначе матрица ниже проверяет пустоту', () => {
    expect(STEP_RIGHTS.length).toBeGreaterThan(0);
    expect(STEP_RIGHTS).toEqual(expect.arrayContaining([
      'action.forward.application',
    ]));
  });

  // Права center.application_history у роли согласующего нет и не должно быть -
  // шаг про журнал заявки обещал бы кнопку, которой человек не увидит.
  it('журнала заявки в туре согласующего нет', () => {
    const ids = approveOnboardingSteps.map((s) => s.id);
    expect(ids).not.toContain('apr-detail-history');
    expect(ids).not.toContain('apr-detail-history-window');
  });

  it('согласующий со всеми правами проходит тур целиком', () => {
    const steps = stepsFor(STEP_RIGHTS);
    expect(steps.map((s) => s.id)).toEqual(approveOnboardingSteps.map((s) => s.id));
    expect(useOnboardingStore().totalSteps).toBe(approveOnboardingSteps.length);
  });

  it('у каждого гейтящего права свои шаги, и без него исчезают только они', () => {
    for (const right of STEP_RIGHTS) {
      setActivePinia(createPinia());
      const dropped = approveOnboardingSteps.filter((s) => s.requires === right).map((s) => s.id);
      expect(dropped.length, right).toBeGreaterThan(0);
      const ids = stepsFor(STEP_RIGHTS.filter((r) => r !== right)).map((s) => s.id);
      expect(ids, right).toEqual(approveOnboardingSteps.map((s) => s.id).filter((id) => !dropped.includes(id)));
    }
  });

  it('нумерация «шаг N из M» не разъезжается: выпавшие шаги не занимают номер', () => {
    const store = useOnboardingStore();
    grant();
    store.start({ tour: TOUR_KEY });

    const expected = approveOnboardingSteps.filter((s) => !s.requires);
    expect(store.totalSteps).toBe(expected.length);
    expect(store.totalSteps).toBeLessThan(approveOnboardingSteps.length);

    expected.forEach((step, index) => {
      store.setIndex(index);
      expect(store.currentStep.id).toBe(step.id);
    });
    store.setIndex(store.totalSteps - 1);
    expect(store.currentStep.celebrate).toBe(true);
  });

  it('голосование и карточка остаются объяснёнными при пустом наборе прав', () => {
    const ids = stepsFor([]).map((s) => s.id);
    expect(ids).toEqual(expect.arrayContaining([
      'apr-nav-center',
      'apr-center-header',
      'apr-detail-approvers',
      'apr-detail-vote',
      'apr-detail-questions',
    ]));
  });
});

describe('замок против смешения ролей', () => {
  /**
   * Кнопки, которые интерфейс рисует ТОЛЬКО принимающему (`isApproverUser` /
   * `canAssignPlaces` / `canDecideSupplement` в ApplicationActionBar и
   * ApplicationDetail). Шаг согласующего на такой якорь молча пропадёт у всей
   * аудитории тура - цели у неё нет.
   */
  const ACCEPTOR_ONLY_ANCHORS = [
    'app-detail-button-take-to-work',
    'attachment-assign-open',
    'supplement-button-accept',
    'supplement-button-refuse',
    'ob-org-moderation',
    'ob-center-archive',
  ];

  /** Подписи кнопок принимающего: обещать их согласующему нельзя даже словами. */
  const ACCEPTOR_ONLY_LABELS = [
    '«Принять»',
    '«Принять дополнение»',
    '«Назначить всем»',
    '«Добавить в справочник»',
    '«Отозвать из работы»',
    '«Вернуть в работу»',
  ];

  it('тур не смотрит ни на одну кнопку принимающего', () => {
    for (const step of approveOnboardingSteps.filter((s) => s.element)) {
      const anchor = step.element.match(/^\[data-testid="([^"]+)"\]$/)[1];
      expect(ACCEPTOR_ONLY_ANCHORS, step.id).not.toContain(anchor);
    }
  });

  it('тексты не выдают приём заявки в работу за работу согласующего', () => {
    for (const step of approveOnboardingSteps) {
      const text = `${step.title} ${step.description}`;
      for (const label of ACCEPTOR_ONLY_LABELS) {
        expect(text.includes(label), `${step.id}: ${label}`).toBe(false);
      }
    }
  });

  it('голосование в туре действительно есть - иначе замок стережёт пустоту', () => {
    // Ряд действий (ob-detail-actions) общий у обеих ролей - состав кнопок в нём
    // зависит от роли, поэтому сам якорь роль не доказывает. Доказывают шаги.
    const ids = approveOnboardingSteps.map((s) => s.id);
    expect(ids).toContain('apr-detail-vote');
    expect(ids).toContain('apr-detail-approvers');
    const anchors = approveOnboardingSteps.filter((s) => s.element)
      .map((s) => s.element.match(/^\[data-testid="([^"]+)"\]$/)[1]);
    expect(anchors).toContain('ob-detail-actions');
    expect(anchors).toContain('ob-detail-status');
  });
});
