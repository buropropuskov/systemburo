import { describe, it, expect } from 'vitest';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
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
});
