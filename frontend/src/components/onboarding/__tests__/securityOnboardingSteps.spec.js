import { describe, it, expect } from 'vitest';
import { securityOnboardingSteps, SECURITY_ONBOARDING_VERSION } from '../securityOnboardingSteps';
import {
  resolveFactTableRoute,
  buildSecurityFactSteps,
  buildSecurityFinalStep,
} from '../securityFactSteps';
import { collectSegment } from '../stepsFlow';
import { resolveReveal } from '../reveal';
import { buildTourSteps, getTour } from '../tours';
import { routeGate, routeExtraGate } from './routerGates';

describe('securityOnboardingSteps', () => {
  it('каждый шаг имеет непустые id, title и строковый description', () => {
    for (const step of securityOnboardingSteps) {
      expect(typeof step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.title).toBe('string');
      expect(step.title.length).toBeGreaterThan(0);
      expect(typeof step.description).toBe('string');
      expect(step.description.length).toBeGreaterThan(0);
    }
  });

  it('element - либо строка-селектор, либо null', () => {
    for (const step of securityOnboardingSteps) {
      const ok = step.element === null || (typeof step.element === 'string' && step.element.length > 0);
      expect(ok).toBe(true);
    }
  });

  it('route у каждого шага - непустая строка', () => {
    for (const step of securityOnboardingSteps) {
      expect(typeof step.route).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
    }
  });

  it('нет дублей id', () => {
    const ids = securityOnboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('есть центр-модал (element null) для старта', () => {
    expect(securityOnboardingSteps[0].element).toBe(null);
  });

  it('expandRail только true и только у nav-шагов', () => {
    const railSteps = securityOnboardingSteps.filter((s) => s.expandRail);
    expect(railSteps.length).toBeGreaterThan(0);
    for (const s of railSteps) {
      expect(s.expandRail).toBe(true);
      expect(s.id.startsWith('sec-nav-')).toBe(true);
    }
  });

  it('тур охранника НЕ ведёт к подаче заявки', () => {
    const ids = securityOnboardingSteps.map((s) => s.id);
    // нет шагов оформления заявки и кнопки «Подать заявку» в шапке
    expect(ids.some((id) => id.startsWith('createapp-'))).toBe(false);
    expect(ids).not.toContain('header-submit');
    for (const s of securityOnboardingSteps) {
      expect(s.element).not.toBe('[data-testid="header-button-submit-app"]');
      expect(s.description).not.toMatch(/Подать заявку/);
    }
    // финал-CTA на оформление заявки в этом сценарии нет
    expect(securityOnboardingSteps.some((s) => s.cta)).toBe(false);
  });
});

describe('securityOnboardingSteps - сегмент /news', () => {
  it('собирается из приветствия, шапки и навигации', () => {
    const seg = collectSegment(securityOnboardingSteps, 0, '/news');
    expect(seg.map((s) => s.id)).toEqual([
      'sec-start',
      'sec-header-notifications',
      'sec-header-notifications-panel',
      'sec-header-search',
      'sec-header-search-panel',
      'sec-header-feedback',
      'sec-nav-rail',
      // «Таблицы» раньше «Доступных мне»: последний шаг сегмента обещает переход,
      // и обещание должно исполняться следующим же шагом.
      'sec-nav-tables',
      'sec-nav-accessible',
    ]);
  });

  // Панель выезжает поверх шапки и закрывает саму кнопку: раскрытие просит
  // ОТДЕЛЬНЫЙ шаг про панель, иначе подсветки не видно вовсе.
  it('сквозной поиск: кнопка шапки без раскрытия, панель - отдельным шагом', () => {
    const step = securityOnboardingSteps.find((s) => s.id === 'sec-header-search');
    expect(step.element).toBe('[data-testid="header-button-search"]');
    expect(step.reveal).toBeUndefined();
    const panel = securityOnboardingSteps.find((s) => s.id === 'sec-header-search-panel');
    expect(panel.element).toBe('[data-testid="global-search-panel"]');
    expect(panel.reveal).toEqual({ open: 'search-panel' });
    expect(panel.optional).toBe(true);
    // Кнопка поиска правами не гейтится (в шапке нет v-if по can) - requires не нужен.
    expect(step.requires).toBeUndefined();
    // Смысл шага для охраны: машину ищут и по марке, не только по госномеру -
    // это рассказывает шаг про саму панель.
    expect(panel.description).toMatch(/марке/);
  });

  it('шаг панели поиска не наследует раскрытие drawer от соседей (панель поверх шапки)', () => {
    const index = securityOnboardingSteps.findIndex((s) => s.id === 'sec-header-search-panel');
    expect(resolveReveal(securityOnboardingSteps, index)).toEqual({
      mobile: null,
      open: 'search-panel',
    });
  });

  // Пункт «Таблицы» есть в меню только у того, кому доступна хотя бы одна таблица
  // поста, а тур открывается и по праву «Доступные мне». Без пометки шаг ждал
  // цель четыре секунды и рассказывал про раздел, которого у человека нет.
  it('шаг «Таблицы» помечен как «может отсутствовать»', () => {
    const step = securityOnboardingSteps.find((s) => s.id === 'sec-nav-tables');
    expect(step.optional).toBe(true);
  });

  it('подсвечивает nav «Доступные мне» и «Таблицы» по реальным testid', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-nav-accessible').element).toBe('[data-testid="nav-link-accessible-attachments"]');
    expect(byId('sec-nav-tables').element).toBe('[data-testid="nav-link-tables"]');
  });

  it('шаги шапки несут целевой селектор и поповер снизу', () => {
    // Панель поиска - исключение: она сама занимает правый край, и поповер
    // ставится слева от неё, а не под кнопкой.
    const headerSteps = securityOnboardingSteps
      .filter((x) => x.id.startsWith('sec-header-') && x.id !== 'sec-header-search-panel');
    for (const s of headerSteps) {
      expect(typeof s.element).toBe('string');
      expect(s.side).toBe('bottom');
    }
  });
});

describe('reveal (#1097 S11 - переехавшие на <768 цели)', () => {
  it('единственное значение оси mobile - nav', () => {
    for (const s of securityOnboardingSteps) {
      if (s.reveal?.mobile !== undefined) {
        expect(s.reveal.mobile).toBe('nav');
      }
    }
  });

  it('sec-header-feedback переехал в drawer (nav); колокольчик - в шапке (без reveal)', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    // #1097 W3.3: «Сообщить о проблеме» вынесено из "⋯" в бургер-drawer.
    expect(byId('sec-header-feedback').reveal).toEqual({ mobile: 'nav' });
    // #1097 W3.2: колокольчик вынесен из "⋯" в саму шапку - reveal не нужен.
    expect(byId('sec-header-notifications').reveal).toBeUndefined();
  });

  /**
   * Шаг, который зовёт нажать, обязан вести к шагу про открывшееся: список
   * уведомлений лежит ниже затемнения тура, и без перехода человек, выполнивший
   * просьбу, получал панель за тёмной пеленой (замечание владельца 20.08).
   */
  it('колокольчик ведёт к разбору списка: переход по действию и раскрытие сам', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    const bell = byId('sec-header-notifications');
    const panel = byId('sec-header-notifications-panel');
    expect(bell.advanceWhen).toBe('[data-testid="ob-notifications-panel"]');
    expect(panel.element).toBe('[data-testid="ob-notifications-panel"]');
    expect(panel.reveal).toEqual({ open: 'notifications' });
    // Шаги идут подряд - иначе переход по действию уводил бы не туда.
    const ids = securityOnboardingSteps.map((s) => s.id);
    expect(ids.indexOf(panel.id)).toBe(ids.indexOf(bell.id) + 1);
  });

  it('шаги тура не зовут нажимать там, куда тур не переходит', () => {
    for (const step of securityOnboardingSteps) {
      if (!/[Нн]ажм|[Нн]ажат/.test(step.description)) continue;
      expect(step.advanceWhen, `шаг ${step.id} зовёт нажать без перехода`).toBeTruthy();
    }
  });

  it('sec-nav-* просят раскрытие drawer (nav)', () => {
    for (const s of securityOnboardingSteps.filter((x) => x.id.startsWith('sec-nav-'))) {
      expect(s.reveal).toEqual({ mobile: 'nav' });
    }
  });
});

describe('requires (шаги, зависящие от прав)', () => {
  it('«Сообщить о проблеме» гейтится тем же правом, что и в туре заявителя', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-header-feedback').requires).toBe('header.report_problem');
  });

  it('версия тура охраны своя и поднята под новый состав шагов', () => {
    expect(Number.isInteger(SECURITY_ONBOARDING_VERSION)).toBe(true);
    // 2 - поиск, объяснение скрытого ФИО, пропуск по факту и отчёт по проходам:
    // без подъёма прошедшие тур охранники новых шагов не увидели бы.
    expect(SECURITY_ONBOARDING_VERSION).toBeGreaterThanOrEqual(2);
  });
});

describe('securityOnboardingSteps - сегмент /accessible-attachments', () => {
  it('отделён границей route и идёт после сегмента /news', () => {
    const first = securityOnboardingSteps.findIndex((s) => s.route === '/accessible-attachments');
    expect(first).toBeGreaterThan(0);
    expect(securityOnboardingSteps[first - 1].route).toBe('/news');
    expect(collectSegment(securityOnboardingSteps, first, '/accessible-attachments').map((s) => s.id)).toEqual([
      'sec-aa-intro',
      'sec-aa-filters',
      'sec-aa-card',
      'sec-aa-detail',
      'sec-aa-elements',
      'sec-aa-preview',
      'sec-aa-blank',
    ]);
  });

  // Охране объясняли, почему у отправителя показан логин вместо ФИО. Владелец
  // убрал: на посту это лишнее, а главное - состав вложения, то есть кого
  // пропускать.
  it('показывает состав вложения, а не разбор маскировки ФИО', () => {
    const ids = securityOnboardingSteps.map((s) => s.id);
    expect(ids).not.toContain('sec-aa-sender');
    const step = securityOnboardingSteps.find((s) => s.id === 'sec-aa-elements');
    expect(step.element).toBe('[data-testid="attachment-elements"]');
    expect(step.reveal).toEqual({ open: 'first-attachment' });
  });

  // Рассказывать про кнопку и не открывать файл - половина объяснения.
  it('тур открывает бланк и отчёт, а не только называет кнопки', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-aa-preview').advanceWhen).toBe('[data-testid="ob-blank-preview"]');
    expect(byId('sec-aa-blank').reveal).toEqual({ open: 'attachment-blank' });
    const report = buildSecurityFactSteps('/table/kpp_1').find((s) => s.id === 'sec-fact-report-window');
    expect(report.reveal).toEqual({ open: 'pass-report' });
  });

  it('реюзит существующие aa-* testid страницы «Доступные мне»', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-aa-filters').element).toBe('[data-testid="aa-filters"]');
    expect(byId('sec-aa-card').element).toBe('[data-testid="aa-card"]');
    expect(byId('sec-aa-detail').element).toBe('[data-testid="aa-detail"]');
    expect(byId('sec-aa-preview').element).toBe('[data-testid="aa-preview-blank"]');
  });

  it('карточка, деталь и предпросмотр опциональны (нет выбранной карточки - шаг пропускается)', () => {
    for (const id of ['sec-aa-card', 'sec-aa-detail', 'sec-aa-elements', 'sec-aa-preview', 'sec-aa-blank']) {
      expect(securityOnboardingSteps.find((s) => s.id === id).optional).toBe(true);
    }
  });

  it('последний обязательный шаг сегмента - фильтры (хвост опционален, тур завершается на нём)', () => {
    const seg = collectSegment(
      securityOnboardingSteps,
      securityOnboardingSteps.findIndex((s) => s.route === '/accessible-attachments'),
      '/accessible-attachments',
    );
    const lastRequired = [...seg].reverse().find((s) => !s.optional);
    expect(lastRequired.id).toBe('sec-aa-filters');
  });

  it('базовый массив не содержит шагов фактовой таблицы (добавляются динамически)', () => {
    expect(securityOnboardingSteps.some((s) => s.id.startsWith('sec-fact-'))).toBe(false);
  });
});

describe('resolveFactTableRoute', () => {
  const table = (name, type, extra = {}) => ({
    table: { name, table_type: type, show_fact_table: true, is_active: true, ...extra },
  });

  it('при обоих типах предпочитает машины, даже когда таблица людей идёт раньше', () => {
    const tables = [table('people_1', 'people'), table('kpp_1', 'cars'), table('kpp_2', 'cars')];
    expect(resolveFactTableRoute(tables)).toBe('/table/kpp_1');
  });

  it('без машинной таблицы берёт таблицу людей (иначе сегмент отметки пропадал)', () => {
    expect(resolveFactTableRoute([table('people_1', 'people')])).toBe('/table/people_1');
  });

  it('тип, которого нет в словаре, тоже годится - фактовая таблица есть фактовая таблица', () => {
    expect(resolveFactTableRoute([table('misc_1', 'items')])).toBe('/table/misc_1');
  });

  it('поддерживает плоскую форму элемента (без вложенного table)', () => {
    const tables = [{ name: 'kpp_3', table_type: 'cars', show_fact_table: true, is_active: true }];
    expect(resolveFactTableRoute(tables)).toBe('/table/kpp_3');
  });

  it('пропускает неактивные, не-фактовые и безымянные таблицы', () => {
    const tables = [
      table('a', 'cars', { is_active: false }),
      table('b', 'cars', { show_fact_table: false }),
      table('c', 'people', { name: '' }),
    ];
    expect(resolveFactTableRoute(tables)).toBe(null);
  });

  it('canView отсеивает недоступные таблицы: машина без права уступает людям с правом', () => {
    const tables = [table('kpp_1', 'cars'), table('people_1', 'people')];
    expect(resolveFactTableRoute(tables, (name) => name !== 'kpp_1')).toBe('/table/people_1');
  });

  it('canView отвергает всё - null, сегмент отметки не добавляется', () => {
    expect(resolveFactTableRoute([table('kpp_1', 'cars')], () => false)).toBe(null);
  });

  it('без предиката доступность не проверяется (права ещё не загружены)', () => {
    expect(resolveFactTableRoute([table('kpp_1', 'cars')])).toBe('/table/kpp_1');
  });

  it('null на пустом списке и не-массиве', () => {
    expect(resolveFactTableRoute([])).toBe(null);
    expect(resolveFactTableRoute(null)).toBe(null);
    expect(resolveFactTableRoute(undefined)).toBe(null);
  });
});

describe('buildSecurityFactSteps', () => {
  it('пустой массив без route', () => {
    expect(buildSecurityFactSteps(null)).toEqual([]);
    expect(buildSecurityFactSteps('')).toEqual([]);
  });

  // Порядок отражает работу поста: сперва список «по заявке», где отмечают
  // проезд ожидаемых машин, и только потом «по факту» - ручной ввод для тех,
  // кого в заявке не было. Раньше тур рассказывал только про «по факту».
  it('строит список по заявке, отметки, ручной ввод и отчёт на переданный route', () => {
    const steps = buildSecurityFactSteps('/table/kpp_1');
    expect(steps.map((s) => s.id)).toEqual([
      'sec-table-instruction',
      'sec-pass-intro',
      'sec-pass-row',
      'sec-pass-entry',
      'sec-pass-exit',
      'sec-on-territory',
      'sec-fact-intro',
      'sec-fact-report',
      'sec-fact-report-window',
    ]);
    expect(steps.every((s) => s.route === '/table/kpp_1')).toBe(true);
  });

  it('отметки смотрят на кнопки списка «по заявке», а не на блок ручного ввода', () => {
    const byId = (id) => buildSecurityFactSteps('/table/kpp_1').find((s) => s.id === id);
    expect(byId('sec-pass-intro').element).toBe('[data-testid="cars-table"]');
    expect(byId('sec-pass-row').element).toBe('[data-testid="ob-pass-row"]');
    expect(byId('sec-pass-entry').element).toBe('[data-testid="ob-pass-entry"]');
    expect(byId('sec-pass-exit').element).toBe('[data-testid="ob-pass-exit"]');
    expect(byId('sec-fact-intro').element).toBe('[data-testid="fact-table"]');
  });

  it('тексты нейтральны к типу таблицы: сегмент строится и для людей', () => {
    const steps = buildSecurityFactSteps('/table/people_1');
    // Формулировки не должны утверждать, что таблица про машины: у поста может
    // быть закреплён список людей.
    expect(steps.find((s) => s.id === 'sec-pass-intro').description).toMatch(/кому пропуск уже согласован/);
    expect(steps.find((s) => s.id === 'sec-pass-row').description).toMatch(/ФИО человека/);
  });

  // Отдельный шаг про окно ручного пропуска висел на кнопке «Въезд» блока «по
  // факту»: у пустого блока её нет, шаг выбрасывался, и общее число шагов на
  // глазах падало. Рассказ переехал в шаг про сам блок.
  it('ручной пропуск объясняется в шаге про блок «по факту», без отдельного шага', () => {
    const steps = buildSecurityFactSteps('/table/kpp_1');
    expect(steps.some((s) => s.id === 'sec-fact-pass')).toBe(false);
    const block = steps.find((s) => s.id === 'sec-fact-intro');
    expect(block.description).toMatch(/номер Т\/С/);
  });

  it('первый шаг подсвечивает инструкцию поста и остаётся границей деградации сегмента', () => {
    const [intro] = buildSecurityFactSteps('/table/kpp_1');
    expect(intro.element).toBe('[data-testid="ob-table-instruction"]');
    expect(intro.optional).toBe(true);
    expect(intro.optionalSegment).toBe(true);
  });

  it('строка и кнопки опциональны - на пустом посту и в таблице людей их нет', () => {
    const byId = (id) => buildSecurityFactSteps('/table/kpp_1').find((s) => s.id === id);
    // На таблице людей кнопок «Въезд»/«Выезд» нет вовсе, а на новом посту пуст и
    // сам список - без optional тур ждал бы цель, которой в разметке не будет.
    for (const id of ['sec-pass-row', 'sec-pass-entry', 'sec-pass-exit', 'sec-fact-intro']) {
      expect(byId(id).optional, id).toBe(true);
    }
  });

  it('сегмент отметки не ведёт к подаче заявки (нет cta)', () => {
    const steps = buildSecurityFactSteps('/table/kpp_1');
    expect(steps.some((s) => s.cta)).toBe(false);
    for (const s of steps) {
      expect(s.description).not.toMatch(/Подать заявку/);
    }
  });
});

describe('buildSecurityFinalStep', () => {
  it('финальный центр-модал с празднованием на достижимом /accessible-attachments', () => {
    const step = buildSecurityFinalStep();
    expect(step.id).toBe('sec-finish');
    expect(step.element).toBe(null);
    expect(step.celebrate).toBe(true);
    // финал всегда на достижимой странице, НЕ на route фактовой таблицы
    expect(step.route).toBe('/accessible-attachments');
    expect(typeof step.title).toBe('string');
    expect(step.title.length).toBeGreaterThan(0);
    expect(typeof step.description).toBe('string');
    expect(step.description.length).toBeGreaterThan(0);
  });

  it('CTA ведёт в «Доступные мне», а НЕ на подачу заявки', () => {
    const step = buildSecurityFinalStep();
    expect(typeof step.cta).toBe('string');
    expect(step.cta.length).toBeGreaterThan(0);
    expect(step.cta).not.toMatch(/Подать заявку/);
    expect(step.ctaRoute).toBe('/accessible-attachments');
    expect(step.description).not.toMatch(/Подать заявку/);
  });

  it('базовый массив не содержит финального шага (строится динамически)', () => {
    expect(securityOnboardingSteps.some((s) => s.id === 'sec-finish')).toBe(false);
  });

  it('без фактовой таблицы сегмент отметки не добавляется, а финал остаётся достижим', () => {
    const steps = buildTourSteps('guard', { factTableRoute: null });
    expect(steps.some((s) => s.id.startsWith('sec-fact-'))).toBe(false);
    const last = steps[steps.length - 1];
    expect(last.id).toBe('sec-finish');
    // Финал живёт на «Доступных мне» - странице, достижимой охранником всегда.
    expect(last.route).toBe('/accessible-attachments');
  });
});

/**
 * Достижимость страниц тура. Гейт тура обязан покрывать гейты всех страниц, на
 * которых стоят его шаги, - иначе человек видит тур в меню «Обучение», проходит
 * вступление и уезжает с первого шага недостижимого сегмента.
 *
 * Проверка идёт и по `permission` роута, и по мета-флагам вроде
 * `requiresSecurityOrAdmin`: последние гард проверяет отдельной ветвью, и по
 * одному `permission` такая страница выглядит открытой любому вошедшему.
 */
describe('securityOnboardingSteps - достижимость страниц участником тура', () => {
  const guard = getTour('guard');

  /** @returns {object} контекст гейтинга: одно основание доступа за раз */
  function ctx({ permissions = [], isSecurity = false } = {}) {
    return {
      isAuthenticated: true,
      isSecurity,
      can: (key) => permissions.includes(key),
      approvalRole: { isApprover: false, isReviewer: false },
    };
  }

  /** Модель гардов роутера, выраженных мета-флагом, а не правом. */
  const EXTRA_GATE_MODEL = {
    requiresSecurityOrAdmin: (c) => c.isSecurity || c.can('page.available'),
    requiresSuperAdmin: () => false,
  };

  /** Основания, каждое из которых само по себе даёт доступ к туру или не даёт. */
  const CANDIDATES = [
    { name: 'работник поста (тип security)', c: ctx({ isSecurity: true }) },
    { name: 'право «Доступные мне»', c: ctx({ permissions: ['page.available'] }) },
    { name: 'право «Таблицы постов»', c: ctx({ permissions: ['page.tables'] }) },
    { name: 'без прав', c: ctx() },
  ];

  it('«Доступные мне» закрыты мета-флагом, а не правом - замок обязан это видеть', () => {
    expect(routeExtraGate('/accessible-attachments')).toBe('requiresSecurityOrAdmin');
    expect(routeGate('/accessible-attachments')).toBeNull();
  });

  it('одного права «Таблицы постов» для тура недостаточно: шесть шагов и финал живут на «Доступных мне»', () => {
    expect(guard.isAvailable(ctx({ permissions: ['page.tables'] }))).toBe(false);
    expect(guard.isAvailable(ctx({ permissions: ['page.available'] }))).toBe(true);
    expect(guard.isAvailable(ctx({ isSecurity: true }))).toBe(true);
  });

  it('каждый статический роут шагов достижим любому, кого гейт тура пускает', () => {
    const routes = [
      ...new Set(buildTourSteps('guard', { factTableRoute: null }).map((s) => s.route)),
    ];
    for (const { name, c } of CANDIDATES) {
      if (!guard.isAvailable(c)) continue;
      for (const route of routes) {
        const gate = routeGate(route);
        if (gate) expect(c.can(gate), `${name} -> ${route} (${gate})`).toBe(true);
        const extra = routeExtraGate(route);
        if (extra) expect(EXTRA_GATE_MODEL[extra](c), `${name} -> ${route} (${extra})`).toBe(true);
      }
    }
  });
});
