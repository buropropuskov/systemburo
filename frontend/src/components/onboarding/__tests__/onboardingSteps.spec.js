import { describe, it, expect } from 'vitest';
import { onboardingSteps, ONBOARDING_VERSION, collectSegment } from '../onboardingSteps';

describe('onboardingSteps', () => {
  it('версия тура - целое число >= 1', () => {
    expect(Number.isInteger(ONBOARDING_VERSION)).toBe(true);
    expect(ONBOARDING_VERSION).toBeGreaterThanOrEqual(1);
  });

  it('каждый шаг имеет непустые id, title и строковый description', () => {
    for (const step of onboardingSteps) {
      expect(typeof step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.title).toBe('string');
      expect(step.title.length).toBeGreaterThan(0);
      expect(typeof step.description).toBe('string');
      expect(step.description.length).toBeGreaterThan(0);
    }
  });

  it('element - либо строка-селектор, либо null', () => {
    for (const step of onboardingSteps) {
      const ok = step.element === null || (typeof step.element === 'string' && step.element.length > 0);
      expect(ok).toBe(true);
    }
  });

  it('route у каждого шага - непустая строка', () => {
    for (const step of onboardingSteps) {
      expect(typeof step.route).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
    }
  });

  it('нет дублей id', () => {
    const ids = onboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('есть хотя бы один центр-модал (element null) для старта', () => {
    const hasCenterModal = onboardingSteps.some((s) => s.element === null);
    expect(hasCenterModal).toBe(true);
  });
});

describe('collectSegment', () => {
  const steps = [
    { id: 'a', route: '/news' },
    { id: 'b', route: '/news' },
    { id: 'c', route: '/personal-cabinet' },
    { id: 'd', route: '/personal-cabinet' },
    { id: 'e', route: '/carsview' },
  ];

  it('берёт подряд идущие шаги с одним route начиная с индекса', () => {
    expect(collectSegment(steps, 0, '/news').map((s) => s.id)).toEqual(['a', 'b']);
  });

  it('останавливается на границе сегмента (другой route)', () => {
    expect(collectSegment(steps, 2, '/personal-cabinet').map((s) => s.id)).toEqual(['c', 'd']);
  });

  it('пустой массив если route стартового шага не совпадает с активным', () => {
    expect(collectSegment(steps, 0, '/carsview')).toEqual([]);
  });

  it('одиночный сегмент в конце массива', () => {
    expect(collectSegment(steps, 4, '/carsview').map((s) => s.id)).toEqual(['e']);
  });

  it('реальная конфигурация: весь сегмент news собирается с нулевого индекса', () => {
    const seg = collectSegment(onboardingSteps, 0, '/news');
    expect(seg.length).toBe(onboardingSteps.filter((s) => s.route === '/news').length);
  });
});
