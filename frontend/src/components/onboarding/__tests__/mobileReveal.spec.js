import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import bus from '@/eventBus';
import {
  resolveMobileReveal,
  applyMobileReveal,
  restoreMobileReveal,
  isNavDrawerOpen,
} from '../mobileReveal';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
import { onboardingSteps } from '../onboardingSteps';

/**
 * Механизм раскрытия переехавших целей (#1097 S11). После правки волны 3 остался
 * единственный reveal - 'nav' (бургер-drawer): рельс/группы уезжают transform'ом за
 * экран, но остаются в DOM "видимыми" для waitForElement, поэтому drawer нужно открыть
 * ПЕРЕД подсветкой. Меню "⋯" убрано совсем, feedback переехал в drawer (reveal 'nav',
 * W3.3), часы удалены. Колокольчик `*-header-notifications` reveal НЕ несёт - он в самой
 * шапке, на его шаге drawer ЗАКРЫТ.
 *
 * DOM/$bus мокаем поведением NavMenu (burger -> класс drawer), чтобы applyMobileReveal
 * реально управлял классом.
 */
describe('mobileReveal', () => {
  const originalWidth = window.innerWidth;
  let navHandler;

  function setViewport(w) {
    window.innerWidth = w;
  }

  beforeEach(() => {
    document.body.innerHTML = `
      <nav data-testid="ob-nav-rail" class="nav-menu"></nav>
    `;
    const nav = document.querySelector('[data-testid="ob-nav-rail"]');
    // Эмуляция NavMenu: $bus mobile-nav-toggle -> тоггл класса drawer.
    navHandler = () => nav.classList.toggle('nav-menu--mobile-open');
    bus.on('mobile-nav-toggle', navHandler);
  });

  afterEach(() => {
    bus.off('mobile-nav-toggle', navHandler);
    document.body.innerHTML = '';
    window.innerWidth = originalWidth;
    vi.useRealTimers();
  });

  const idxOf = (steps, id) => steps.findIndex((s) => s.id === id);

  describe('resolveMobileReveal (чистая логика)', () => {
    it('cur.mobileReveal имеет приоритет: рельс -> nav', () => {
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      expect(resolveMobileReveal(securityOnboardingSteps, iR)).toBe('nav');
    });

    it('шаг колокольчика reveal НЕ несёт -> null (вынесен в шапку, #1097 W3.2)', () => {
      const iApp = idxOf(onboardingSteps, 'header-notifications');
      const iSec = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      expect(onboardingSteps[iApp].mobileReveal).toBeUndefined();
      expect(securityOnboardingSteps[iSec].mobileReveal).toBeUndefined();
      expect(resolveMobileReveal(onboardingSteps, iApp)).toBe(null);
      expect(resolveMobileReveal(securityOnboardingSteps, iSec)).toBe(null);
    });

    it('шаг без reveal между РАЗНЫМИ типами -> null (applicant header-submit)', () => {
      // header-submit стоит между header-notifications(без reveal) и nav-rail(nav):
      // соседи разного типа -> drawer закрыт, не наследует чужой reveal.
      const i = idxOf(onboardingSteps, 'header-submit');
      expect(onboardingSteps[i].mobileReveal).toBeUndefined();
      expect(resolveMobileReveal(onboardingSteps, i)).toBe(null);
    });

    it('center-модал старта без reveal -> null', () => {
      const i = idxOf(securityOnboardingSteps, 'sec-start');
      expect(resolveMobileReveal(securityOnboardingSteps, i)).toBe(null);
    });
  });

  describe('applyMobileReveal - drawer на мобилке (<=768)', () => {
    it('sec-nav-rail: drawer открыт', async () => {
      setViewport(390);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      vi.useFakeTimers();
      const p = applyMobileReveal(securityOnboardingSteps, i);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
    });

    it('sec-header-notifications: drawer ЗАКРЫТ - шаг колокольчика (RED-1)', async () => {
      setViewport(390);
      // предусловие: drawer открыт (как после предыдущего nav-шага)
      document.querySelector('[data-testid="ob-nav-rail"]').classList.add('nav-menu--mobile-open');
      const i = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      await applyMobileReveal(securityOnboardingSteps, i);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('переход nav -> без reveal закрывает drawer', async () => {
      setViewport(390);
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      const iN = idxOf(securityOnboardingSteps, 'sec-header-notifications');

      vi.useFakeTimers();
      const p = applyMobileReveal(securityOnboardingSteps, iR);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);

      // Переход на колокольчик (без reveal): drawer закрывается.
      await applyMobileReveal(securityOnboardingSteps, iN);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('на 768 (граница CSS max-width:768px) reveal активен', async () => {
      setViewport(768);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      vi.useFakeTimers();
      const p = applyMobileReveal(securityOnboardingSteps, i);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
    });
  });

  describe('applyMobileReveal - десктоп (>=769) не трогает drawer', () => {
    it('на 1024 drawer не открывается', async () => {
      setViewport(1024);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      await applyMobileReveal(securityOnboardingSteps, i);
      expect(isNavDrawerOpen()).toBe(false);
    });
  });

  describe('restoreMobileReveal закрывает drawer', () => {
    it('открытый drawer гасится', () => {
      setViewport(390);
      document.querySelector('[data-testid="ob-nav-rail"]').classList.add('nav-menu--mobile-open');
      restoreMobileReveal();
      expect(isNavDrawerOpen()).toBe(false);
    });
  });
});
