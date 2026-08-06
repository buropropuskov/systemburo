import { describe, it, expect, afterEach } from 'vitest';
import { isMobileViewport } from '../useOnboarding';

/**
 * isMobileViewport - брейкпоинт (#1097 S11), по которому OnboardingTour решает,
 * раскрывать ли переехавшие цели (drawer NavMenu / overflow-меню шапки) перед
 * подсветкой шага, и createDriver - принудительно класть поповер снизу для
 * side:'left'/'right' шагов (см. onboardingSteps.js JSDoc `reveal`).
 */
describe('isMobileViewport', () => {
  const originalWidth = window.innerWidth;

  afterEach(() => {
    window.innerWidth = originalWidth;
  });

  it('false на десктопной ширине (>=769)', () => {
    window.innerWidth = 1440;
    expect(isMobileViewport()).toBe(false);
    window.innerWidth = 769;
    expect(isMobileViewport()).toBe(false);
  });

  it('true на мобильной ширине (<=768, порог включителен как CSS max-width:768px)', () => {
    // 768 - iPad-портрет: CSS уже мобильный (max-width:768px), reveal обязан сработать.
    window.innerWidth = 768;
    expect(isMobileViewport()).toBe(true);
    window.innerWidth = 767;
    expect(isMobileViewport()).toBe(true);
    window.innerWidth = 390;
    expect(isMobileViewport()).toBe(true);
  });
});
