import { describe, it, expect } from 'vitest';
import {
  TOURS,
  getTour,
  buildTourSteps,
  isTourReady,
  availableTours,
  pickAutostartTour,
  allTourSteps,
} from '../tours';
import { onboardingSteps } from '../onboardingSteps';
import { securityOnboardingSteps } from '../securityOnboardingSteps';

/**
 * Реестр туров: гейтинг по правам, приоритет автозапуска, состав шагов. Правила
 * доступности - чистые функции от плоского контекста, поэтому проверяются без Pinia.
 */

/**
 * @param {{ permissions?: string[], security?: boolean, approver?: boolean, reviewer?: boolean, anonymous?: boolean }} role
 */
function ctx({ permissions = [], security = false, approver = false, reviewer = false, anonymous = false } = {}) {
  return {
    isAuthenticated: !anonymous,
    isSecurity: security,
    can: (key) => permissions.includes(key),
    approvalRole: { isApprover: approver, isReviewer: reviewer },
  };
}

const ROLES = {
  'заявитель (без прав)': ctx(),
  'охранник': ctx({ security: true }),
  // Обеим ролям положен доступ в центр заявок: чужие заявки согласуют и принимают
  // только оттуда. Роль без него нефункциональна, и тур ей не показывается.
  'согласующий': ctx({ reviewer: true, permissions: ['page.center'] }),
  'принимающий': ctx({ approver: true, permissions: ['page.center'] }),
  'администратор': ctx({ permissions: ['page.admin'] }),
};

/** Ожидаемая матрица доступности: роль -> ключи туров, прошедших гейт. */
const EXPECTED = {
  'заявитель (без прав)': ['user'],
  'охранник': ['user', 'guard'],
  'согласующий': ['user', 'approve'],
  'принимающий': ['user', 'accept'],
  'администратор': ['user', 'admin'],
};

/**
 * Шаг, который зовёт человека нажать, обязан доводить до шага про открывшееся.
 *
 * Затемнение тура лежит выше почти всех окон системы (z-index 12500), поэтому
 * список уведомлений, выпадающее меню или карточка, открытые по просьбе шага,
 * оказывались за тёмной пеленой: человек сделал, что просили, и остался ни с чем
 * (замечание владельца 20.08). Переход по действию (`advanceWhen`) уводит тур на
 * шаг, который эту открывшуюся часть и подсвечивает - тогда она поднимается над
 * затемнением сама.
 */
/**
 * Финал у всех пяти туров один (решение владельца 20.08): возвращаемся на «Обзор
 * и новости» и подсвечиваем кнопку «Обучение» - ту самую, которой тур запускают
 * заново, и в которой лежат туры остальных ролей. Прежде каждый тур заканчивался
 * на своей странице и предлагал кнопку-переход в раздел, так что человек оставался
 * там, где закончил, и повторный запуск ему было негде взять.
 */
describe('финал у всех туров одинаковый', () => {
  const finals = TOURS.map((t) => {
    const steps = buildTourSteps(t, { factTableRoute: '/table/kpp' });
    return { key: t.key, step: steps[steps.length - 1] };
  });

  it('последний шаг - празднование на «Обзоре» с подсветкой кнопки «Обучение»', () => {
    finals.forEach(({ key, step }) => {
      expect(step.celebrate, key).toBe(true);
      expect(step.route, key).toBe('/news');
      expect(step.element, key).toBe('[data-testid="ob-start-button"]');
    });
  });

  it('кнопки-перехода в раздел на финале нет ни у одного тура', () => {
    finals.forEach(({ key, step }) => {
      expect(step.cta, key).toBeUndefined();
      expect(step.ctaRoute, key).toBeUndefined();
    });
  });

  it('финал зовёт пройти обучение заново', () => {
    finals.forEach(({ key, step }) => {
      expect(step.description, key).toMatch(/Обучение/);
    });
  });
});

describe('призыв нажать доводит до шага', () => {
  it('во всех пяти турах у такого шага есть переход по действию', () => {
    const guilty = allTourSteps()
      .filter((s) => /[Нн]ажм|[Нн]ажат/.test(s.description || ''))
      .filter((s) => !s.advanceWhen)
      .map((s) => s.id);
    expect(guilty).toEqual([]);
  });

  it('цель перехода совпадает с целью следующего шага - иначе тур уйдёт не туда', () => {
    const steps = allTourSteps();
    for (const [i, step] of steps.entries()) {
      if (!step.advanceWhen) continue;
      const next = steps[i + 1];
      expect(next?.element, `после ${step.id} нет шага про открывшееся`).toBe(step.advanceWhen);
    }
  });
});

describe('реестр туров', () => {
  it('пять записей с уникальными ключами', () => {
    expect(TOURS.map((t) => t.key)).toEqual(['user', 'guard', 'approve', 'accept', 'admin']);
    expect(new Set(TOURS.map((t) => t.key)).size).toBe(TOURS.length);
  });

  it('каждая запись несёт название, описание, версию и гейт', () => {
    TOURS.forEach((t) => {
      expect(typeof t.title).toBe('string');
      expect(t.title.length).toBeGreaterThan(0);
      expect(typeof t.description).toBe('string');
      expect(t.description.length).toBeGreaterThan(0);
      expect(Number.isInteger(t.version)).toBe(true);
      expect(t.version).toBeGreaterThanOrEqual(1);
      expect(typeof t.isAvailable).toBe('function');
      expect(Array.isArray(t.steps)).toBe(true);
    });
  });

  it('приоритеты автозапуска уникальны и идут guard -> admin -> accept -> approve -> user', () => {
    const order = [...TOURS].sort((a, b) => a.autostartPriority - b.autostartPriority).map((t) => t.key);
    expect(order).toEqual(['guard', 'admin', 'accept', 'approve', 'user']);
    expect(new Set(TOURS.map((t) => t.autostartPriority)).size).toBe(TOURS.length);
  });

  it('getTour находит запись по ключу и отдаёт null на незнакомом', () => {
    expect(getTour('guard').key).toBe('guard');
    expect(getTour('nope')).toBe(null);
    expect(getTour(undefined)).toBe(null);
  });

  it('все пять туров реестра написаны', () => {
    TOURS.forEach((tour) => {
      expect(isTourReady(tour), tour.key).toBe(true);
    });
  });

  // Механика заготовки нужна и после того, как все пять туров написаны: следующий
  // тур снова заведут пустым файлом, и до наполнения он не должен ни висеть
  // пунктом в меню, ни автозапускаться. Проверяем на синтетической записи, а не
  // на конкретном ключе реестра - иначе тест умирает от наполнения очередного тура.
  it('тур с пустыми шагами не считается написанным', () => {
    expect(isTourReady({ key: 'draft', steps: [] })).toBe(false);
    expect(isTourReady({ key: 'draft' })).toBe(false);
    expect(isTourReady(null)).toBe(false);
  });
});

describe('гейтинг: матрица пяти ролей на пять туров', () => {
  Object.entries(ROLES).forEach(([roleName, roleCtx]) => {
    it(`${roleName}: гейт проходят ${EXPECTED[roleName].join(', ')}`, () => {
      const passed = TOURS.filter((t) => t.isAvailable(roleCtx)).map((t) => t.key);
      expect(passed).toEqual(EXPECTED[roleName]);
    });
  });

  it('неаутентифицированному не доступен ни один тур', () => {
    expect(TOURS.filter((t) => t.isAvailable(ctx({ anonymous: true }))).map((t) => t.key)).toEqual([]);
  });

  // Одного page.tables мало намеренно: шесть шагов тура и финал живут на
  // «Доступных мне», а её роут закрыт гардом requiresSecurityOrAdmin (супер-админ,
  // работник поста, право page.available). С page.tables в гейте человек видел
  // тур, проходил вступление и на первом шаге сегмента уезжал в личный кабинет.
  // Лучше не показать тур, чем оборвать его на середине - так же решено у
  // согласующего и принимающего с page.center.
  it('тур охраны берётся по page.available, но не по одному page.tables', () => {
    expect(getTour('guard').isAvailable(ctx({ permissions: ['page.available'] }))).toBe(true);
    expect(getTour('guard').isAvailable(ctx({ permissions: ['page.tables'] }))).toBe(false);
    expect(getTour('guard').isAvailable(ctx())).toBe(false);
  });

  it('тур согласующего берётся и по праву action.approve.application', () => {
    expect(getTour('approve').isAvailable(
      ctx({ permissions: ['action.approve.application', 'page.center'] }),
    )).toBe(true);
  });

  it('тур принимающего - только по роли из справочника, правом не выдаётся', () => {
    expect(getTour('accept').isAvailable(ctx({ permissions: ['action.approve.application', 'page.admin'] }))).toBe(false);
    expect(getTour('accept').isAvailable(ctx({ approver: true, permissions: ['page.center'] }))).toBe(true);
  });

  // Оба тура почти целиком живут на /center. Без доступа туда человек прошёл бы
  // два вступительных шага и упёрся в роут-гард, не увидев ни главного действия
  // роли, ни финала - такой тур не показываем вовсе.
  it('без доступа в центр заявок туры согласующего и принимающего недоступны', () => {
    expect(getTour('approve').isAvailable(ctx({ reviewer: true }))).toBe(false);
    expect(getTour('accept').isAvailable(ctx({ approver: true }))).toBe(false);
  });

  it('несколько ролей сразу - доступны все подходящие туры', () => {
    const multi = ctx({ permissions: ['page.admin', 'page.center'], approver: true, reviewer: true });
    expect(TOURS.filter((t) => t.isAvailable(multi)).map((t) => t.key))
      .toEqual(['user', 'approve', 'accept', 'admin']);
  });
});

describe('availableTours (что попадёт в меню)', () => {
  it('человек нескольких ролей видит все свои туры в порядке реестра', () => {
    const multi = ctx({ permissions: ['page.admin', 'page.center'], approver: true, reviewer: true });
    expect(availableTours(multi).map((t) => t.key)).toEqual(['user', 'approve', 'accept', 'admin']);
  });

  it('охранник видит свой тур и общий', () => {
    expect(availableTours(ctx({ security: true })).map((t) => t.key)).toEqual(['user', 'guard']);
  });

  it('пустой контекст не роняет гейтинг', () => {
    expect(availableTours(undefined)).toEqual([]);
    expect(availableTours({})).toEqual([]);
  });
});

describe('pickAutostartTour', () => {
  const none = () => false;

  it('из нескольких доступных берёт приоритетный (охрана вперёд заявителя)', () => {
    expect(pickAutostartTour(ctx({ security: true }), none).key).toBe('guard');
  });

  it('пройденный приоритетный уступает следующему', () => {
    expect(pickAutostartTour(ctx({ security: true }), (k) => k === 'guard').key).toBe('user');
  });

  it('все пройдены - null', () => {
    expect(pickAutostartTour(ctx({ security: true }), () => true)).toBe(null);
  });

  it('нет доступных - null', () => {
    expect(pickAutostartTour(ctx({ anonymous: true }), none)).toBe(null);
  });
});

describe('buildTourSteps', () => {
  it('тур заявителя отдаёт статический массив как есть', () => {
    expect(buildTourSteps('user')).toBe(onboardingSteps);
  });

  it('тур охраны собирает базу + финал; без route фактовой таблицы сегмента отметки нет', () => {
    const steps = buildTourSteps('guard', {});
    expect(steps.length).toBe(securityOnboardingSteps.length + 1);
    expect(steps[steps.length - 1].id).toBe('sec-finish');
  });

  it('с route фактовой таблицы сегмент отметки встаёт перед финалом', () => {
    const steps = buildTourSteps('guard', { factTableRoute: '/table/kpp_1' });
    expect(steps.slice(securityOnboardingSteps.length).map((s) => s.id)).toEqual([
      'sec-table-instruction',
      'sec-pass-intro',
      'sec-pass-row',
      'sec-pass-entry',
      'sec-pass-exit',
      'sec-on-territory',
      'sec-fact-intro',
      'sec-fact-report',
      'sec-fact-report-window',
      'sec-finish',
    ]);
  });

  it('незнакомый тур - пустой набор', () => {
    expect(buildTourSteps('nope')).toEqual([]);
  });
});

describe('allTourSteps', () => {
  it('покрывает и статические, и динамические шаги всех туров', () => {
    const ids = allTourSteps().map((s) => s.id);
    expect(ids).toContain('start');
    expect(ids).toContain('sec-start');
    // Динамический сегмент фактовой таблицы в статическом `steps` не лежит.
    expect(ids).toContain('sec-pass-entry');
  });

  it('id шагов уникальны внутри каждого тура', () => {
    TOURS.forEach((t) => {
      const ids = buildTourSteps(t, { factTableRoute: '/table/x' }).map((s) => s.id);
      expect(new Set(ids).size).toBe(ids.length);
    });
  });
});
