import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import bus from '@/eventBus';
import {
  resolveMobileReveal,
  applyMobileReveal,
  restoreMobileReveal,
  isNavDrawerOpen,
  isHeaderOverflowOpen,
} from '../mobileReveal';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
import { onboardingSteps } from '../onboardingSteps';

/**
 * Механизм раскрытия переехавших целей (#1097 S11). Ключевой инвариант (RED-фикс):
 * drawer навигации и overflow-меню шапки ВЗАИМОИСКЛЮЧАЮЩИ - их узлы физически
 * перекрывают друг друга, открыть оба = спотлайт целит в один, показывает другой.
 * Проверяем на реальной последовательности security-тура sec-header-notifications
 * (overflow) -> sec-nav-rail (nav), где шаги идут ВПЛОТНУЮ без буфера.
 *
 * DOM/$bus мокаем поведением NavMenu (burger -> класс drawer) и TheHeader
 * (клик "⋯" -> класс overflow), чтобы applyMobileReveal реально управлял классами.
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
      <button class="header__overflow-toggle"></button>
      <div class="header__overflow"></div>
    `;
    const nav = document.querySelector('[data-testid="ob-nav-rail"]');
    const toggle = document.querySelector('.header__overflow-toggle');
    const overflow = document.querySelector('.header__overflow');
    // Эмуляция NavMenu: $bus mobile-nav-toggle -> тоггл класса drawer.
    navHandler = () => nav.classList.toggle('nav-menu--mobile-open');
    bus.on('mobile-nav-toggle', navHandler);
    // Эмуляция TheHeader: клик по "⋯" -> тоггл класса overflow.
    toggle.addEventListener('click', () => overflow.classList.toggle('header__overflow--open'));
  });

  afterEach(() => {
    bus.off('mobile-nav-toggle', navHandler);
    document.body.innerHTML = '';
    window.innerWidth = originalWidth;
    vi.useRealTimers();
  });

  const idxOf = (steps, id) => steps.findIndex((s) => s.id === id);

  describe('resolveMobileReveal (чистая логика)', () => {
    it('cur.mobileReveal имеет приоритет: колокольчик -> overflow, рельс -> nav', () => {
      const iN = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      expect(resolveMobileReveal(securityOnboardingSteps, iN)).toBe('header-overflow');
      expect(resolveMobileReveal(securityOnboardingSteps, iR)).toBe('nav');
    });

    it('шаг без reveal между РАЗНЫМИ типами -> null (applicant header-submit)', () => {
      // header-submit стоит между header-notifications(overflow) и nav-rail(nav):
      // соседи разного типа -> обе панели закрыты, не наследует чужой reveal.
      const i = idxOf(onboardingSteps, 'header-submit');
      expect(onboardingSteps[i].mobileReveal).toBeUndefined();
      expect(resolveMobileReveal(onboardingSteps, i)).toBe(null);
    });

    it('center-модал старта без reveal -> null', () => {
      const i = idxOf(securityOnboardingSteps, 'sec-start');
      expect(resolveMobileReveal(securityOnboardingSteps, i)).toBe(null);
    });
  });

  describe('applyMobileReveal - эксклюзивность на мобилке (<=768)', () => {
    it('sec-header-notifications: открыт ТОЛЬКО overflow, drawer ЗАКРЫТ', async () => {
      setViewport(390);
      const i = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      await applyMobileReveal(securityOnboardingSteps, i);
      expect(isHeaderOverflowOpen()).toBe(true);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('sec-nav-rail: открыт ТОЛЬКО drawer, overflow ЗАКРЫТ', async () => {
      setViewport(390);
      // предусловие: overflow открыт (как после предыдущего шага-колокольчика)
      document.querySelector('.header__overflow').classList.add('header__overflow--open');
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      vi.useFakeTimers();
      const p = applyMobileReveal(securityOnboardingSteps, i);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
      expect(isHeaderOverflowOpen()).toBe(false);
    });

    it('последовательность notif -> nav-rail: панели переключаются эксклюзивно (главный RED)', async () => {
      setViewport(390);
      const iN = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');

      // Шаг колокольчика: overflow открыт, drawer+backdrop закрыты (иначе перекрыл бы цель).
      await applyMobileReveal(securityOnboardingSteps, iN);
      expect(isHeaderOverflowOpen()).toBe(true);
      expect(isNavDrawerOpen()).toBe(false);

      // Переход на рельс: drawer открывается, overflow закрывается.
      vi.useFakeTimers();
      const p = applyMobileReveal(securityOnboardingSteps, iR);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
      expect(isHeaderOverflowOpen()).toBe(false);
    });

    it('на 768 (граница CSS max-width:768px) reveal активен', async () => {
      setViewport(768);
      const i = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      await applyMobileReveal(securityOnboardingSteps, i);
      expect(isHeaderOverflowOpen()).toBe(true);
    });
  });

  describe('applyMobileReveal - десктоп (>=769) не трогает панели', () => {
    it('на 1024 ни drawer, ни overflow не открываются', async () => {
      setViewport(1024);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      await applyMobileReveal(securityOnboardingSteps, i);
      expect(isNavDrawerOpen()).toBe(false);
      expect(isHeaderOverflowOpen()).toBe(false);
    });
  });

  describe('restoreMobileReveal закрывает обе панели', () => {
    it('открытые drawer и overflow гасятся', () => {
      setViewport(390);
      document.querySelector('[data-testid="ob-nav-rail"]').classList.add('nav-menu--mobile-open');
      document.querySelector('.header__overflow').classList.add('header__overflow--open');
      restoreMobileReveal();
      expect(isNavDrawerOpen()).toBe(false);
      expect(isHeaderOverflowOpen()).toBe(false);
    });
  });
});
