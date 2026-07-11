import { describe, it, expect } from 'vitest';
import {
  securityOnboardingSteps,
  resolveFactTableRoute,
  buildSecurityFactSteps,
  buildSecurityFinalStep,
} from '../securityOnboardingSteps';
import { collectSegment } from '../onboardingSteps';

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
      'sec-header-feedback',
      'sec-header-time',
      'sec-header-notifications',
      'sec-nav-rail',
      'sec-nav-accessible',
      'sec-nav-tables',
    ]);
  });

  it('подсвечивает nav «Доступные мне» и «Таблицы» по реальным testid', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-nav-accessible').element).toBe('[data-testid="nav-link-accessible-attachments"]');
    expect(byId('sec-nav-tables').element).toBe('[data-testid="nav-link-tables"]');
  });

  it('шаги шапки несут целевой селектор и поповер снизу', () => {
    for (const s of securityOnboardingSteps.filter((x) => x.id.startsWith('sec-header-'))) {
      expect(typeof s.element).toBe('string');
      expect(s.side).toBe('bottom');
    }
  });
});

describe('mobileReveal (#1097 S11 - переехавшие на <768 цели)', () => {
  it('значение только nav или header-overflow', () => {
    for (const s of securityOnboardingSteps) {
      if (s.mobileReveal !== undefined) {
        expect(['nav', 'header-overflow']).toContain(s.mobileReveal);
      }
    }
  });

  it('sec-header-* просят overflow-меню шапки', () => {
    for (const s of securityOnboardingSteps.filter((x) => x.id.startsWith('sec-header-'))) {
      expect(s.mobileReveal).toBe('header-overflow');
    }
  });

  it('sec-nav-* просят раскрытие drawer (nav)', () => {
    for (const s of securityOnboardingSteps.filter((x) => x.id.startsWith('sec-nav-'))) {
      expect(s.mobileReveal).toBe('nav');
    }
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
      'sec-aa-preview',
    ]);
  });

  it('реюзит существующие aa-* testid страницы «Доступные мне»', () => {
    const byId = (id) => securityOnboardingSteps.find((s) => s.id === id);
    expect(byId('sec-aa-filters').element).toBe('[data-testid="aa-filters"]');
    expect(byId('sec-aa-card').element).toBe('[data-testid="aa-card"]');
    expect(byId('sec-aa-detail').element).toBe('[data-testid="aa-detail"]');
    expect(byId('sec-aa-preview').element).toBe('[data-testid="aa-preview-blank"]');
  });

  it('карточка, деталь и предпросмотр опциональны (нет выбранной карточки - шаг пропускается)', () => {
    for (const id of ['sec-aa-card', 'sec-aa-detail', 'sec-aa-preview']) {
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
  it('берёт первую активную фактовую таблицу машин (форма { table })', () => {
    const tables = [
      { table: { name: 'people_1', table_type: 'people', show_fact_table: true, is_active: true } },
      { table: { name: 'kpp_1', table_type: 'cars', show_fact_table: true, is_active: true } },
      { table: { name: 'kpp_2', table_type: 'cars', show_fact_table: true, is_active: true } },
    ];
    expect(resolveFactTableRoute(tables)).toBe('/table/kpp_1');
  });

  it('поддерживает плоскую форму элемента (без вложенного table)', () => {
    const tables = [{ name: 'kpp_3', table_type: 'cars', show_fact_table: true, is_active: true }];
    expect(resolveFactTableRoute(tables)).toBe('/table/kpp_3');
  });

  it('пропускает неактивные, не-фактовые и не-cars таблицы', () => {
    const tables = [
      { table: { name: 'a', table_type: 'cars', show_fact_table: true, is_active: false } },
      { table: { name: 'b', table_type: 'cars', show_fact_table: false, is_active: true } },
      { table: { name: 'c', table_type: 'people', show_fact_table: true, is_active: true } },
    ];
    expect(resolveFactTableRoute(tables)).toBe(null);
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

  it('строит intro + строка + Въезд + Выезд на переданный route', () => {
    const steps = buildSecurityFactSteps('/table/kpp_1');
    expect(steps.map((s) => s.id)).toEqual([
      'sec-fact-intro',
      'sec-fact-row',
      'sec-fact-entry',
      'sec-fact-exit',
    ]);
    expect(steps.every((s) => s.route === '/table/kpp_1')).toBe(true);
  });

  it('первый шаг - optionalSegment-центр-модал (граница для грациозной деградации)', () => {
    const [intro] = buildSecurityFactSteps('/table/kpp_1');
    expect(intro.element).toBe(null);
    expect(intro.optionalSegment).toBe(true);
  });

  it('строка и кнопки опциональны и несут реюзимые ob-fact-* / fact-table якоря', () => {
    const byId = (id) => buildSecurityFactSteps('/table/kpp_1').find((s) => s.id === id);
    expect(byId('sec-fact-row').element).toBe('[data-testid="ob-fact-row"]');
    expect(byId('sec-fact-entry').element).toBe('[data-testid="ob-fact-entry"]');
    expect(byId('sec-fact-exit').element).toBe('[data-testid="ob-fact-exit"]');
    for (const id of ['sec-fact-row', 'sec-fact-entry', 'sec-fact-exit']) {
      expect(byId(id).optional).toBe(true);
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
});
