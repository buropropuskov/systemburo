import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { onboardingSteps, ONBOARDING_VERSION } from '@/components/onboarding/onboardingSteps';
import { securityOnboardingSteps } from '@/components/onboarding/securityOnboardingSteps';
import { getOnboardingStatus, markOnboardingComplete } from '@/api/onboarding';

vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn(),
  markOnboardingComplete: vi.fn(),
}));

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

describe('onboarding store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getOnboardingStatus.mockReset();
    markOnboardingComplete.mockReset();
    markOnboardingComplete.mockResolvedValue({ message: 'ok' });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('start / stop', () => {
    it('start() активирует тур и сбрасывает индекс на 0', () => {
      const store = useOnboardingStore();
      store.setIndex(3);
      store.start();

      expect(store.isActive).toBe(true);
      expect(store.currentIndex).toBe(0);
    });

    it('start() идемпотентен при уже активном туре', () => {
      const store = useOnboardingStore();
      store.start();
      store.setIndex(2);
      store.start();

      expect(store.isActive).toBe(true);
      expect(store.currentIndex).toBe(2);
    });

    it('start({ manual: true }) выставляет isManual', () => {
      const store = useOnboardingStore();
      store.start({ manual: true });
      expect(store.isManual).toBe(true);
    });

    it('start() без аргумента оставляет isManual false', () => {
      const store = useOnboardingStore();
      store.start();
      expect(store.isManual).toBe(false);
    });

    it('stop() деактивирует тур', () => {
      const store = useOnboardingStore();
      store.start();
      store.stop();
      expect(store.isActive).toBe(false);
    });
  });

  describe('setIndex / reset', () => {
    it('setIndex меняет currentIndex', () => {
      const store = useOnboardingStore();
      store.setIndex(2);
      expect(store.currentIndex).toBe(2);
      expect(store.currentStep).toBe(onboardingSteps[2]);
    });

    it('reset сбрасывает активность, индекс и per-user статус', () => {
      const store = useOnboardingStore();
      store.start();
      store.setIndex(2);
      store.markCompleted();
      store.reset();

      expect(store.isActive).toBe(false);
      expect(store.currentIndex).toBe(0);
      expect(store.completedVersion).toBe(null);
      expect(store.statusLoaded).toBe(false);
    });
  });

  describe('totalSteps', () => {
    it('равен длине onboardingSteps', () => {
      const store = useOnboardingStore();
      expect(store.totalSteps).toBe(onboardingSteps.length);
    });
  });

  describe('ветвление сценария по типу пользователя', () => {
    it('обычный пользователь получает applicant-тур', () => {
      const auth = useAuthStore();
      auth.userTypeCode = 'organization';
      const store = useOnboardingStore();
      expect(store.steps).toBe(onboardingSteps);
      expect(store.totalSteps).toBe(onboardingSteps.length);
    });

    it('охранник (isSecurity) получает security-тур', () => {
      const auth = useAuthStore();
      auth.userTypeCode = 'security';
      const store = useOnboardingStore();
      expect(auth.isSecurity).toBe(true);
      expect(store.steps).toBe(securityOnboardingSteps);
      expect(store.currentStep).toBe(securityOnboardingSteps[0]);
    });

    it('смена типа пользователя реактивно переключает набор шагов в обе стороны', () => {
      const auth = useAuthStore();
      const store = useOnboardingStore();
      expect(store.steps).toBe(onboardingSteps);
      auth.userTypeCode = 'security';
      expect(store.steps).toBe(securityOnboardingSteps);
      auth.userTypeCode = 'organization';
      expect(store.steps).toBe(onboardingSteps);
    });
  });

  describe('loadStatus / hasCompleted (per-user через API)', () => {
    it('loadStatus тянет статус и ставит completedVersion + statusLoaded', async () => {
      getOnboardingStatus.mockResolvedValue({ completed_version: ONBOARDING_VERSION });
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(getOnboardingStatus).toHaveBeenCalledOnce();
      expect(store.completedVersion).toBe(ONBOARDING_VERSION);
      expect(store.statusLoaded).toBe(true);
    });

    it('конкурентные loadStatus шлют один GET (guard от гонки)', async () => {
      let resolve;
      getOnboardingStatus.mockReturnValue(new Promise((r) => { resolve = r; }));
      const store = useOnboardingStore();
      const p1 = store.loadStatus();
      const p2 = store.loadStatus();
      resolve({ completed_version: null });
      await Promise.all([p1, p2]);

      expect(getOnboardingStatus).toHaveBeenCalledOnce();
    });

    it('loadStatus с completed_version=null -> не пройден', async () => {
      getOnboardingStatus.mockResolvedValue({ completed_version: null });
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(store.statusLoaded).toBe(true);
      expect(store.hasCompleted()).toBe(false);
    });

    it('loadStatus при ошибке сети оставляет statusLoaded=false (fail-safe)', async () => {
      getOnboardingStatus.mockRejectedValue(new Error('network'));
      const store = useOnboardingStore();
      await store.loadStatus();

      expect(store.statusLoaded).toBe(false);
      expect(store.hasCompleted()).toBe(false);
    });

    it('hasCompleted true когда пройдена версия >= текущей', async () => {
      getOnboardingStatus.mockResolvedValue({ completed_version: ONBOARDING_VERSION });
      const store = useOnboardingStore();
      await store.loadStatus();
      expect(store.hasCompleted()).toBe(true);
    });

    it('hasCompleted false когда пройдена старая версия (тур обновился)', async () => {
      getOnboardingStatus.mockResolvedValue({ completed_version: ONBOARDING_VERSION - 1 });
      const store = useOnboardingStore();
      await store.loadStatus();
      expect(store.hasCompleted()).toBe(false);
    });

    it('markCompleted ставит версию локально сразу и шлёт на бэкенд', () => {
      const store = useOnboardingStore();
      store.markCompleted();

      expect(store.completedVersion).toBe(ONBOARDING_VERSION);
      expect(store.hasCompleted()).toBe(true);
      expect(markOnboardingComplete).toHaveBeenCalledWith(ONBOARDING_VERSION);
    });

    it('markCompleted не падает при ошибке записи на бэкенд (fire-and-forget)', () => {
      markOnboardingComplete.mockRejectedValue(new Error('network'));
      const store = useOnboardingStore();
      expect(() => store.markCompleted()).not.toThrow();
      // локально статус всё равно проставлен
      expect(store.hasCompleted()).toBe(true);
    });
  });

  describe('canShowTour', () => {
    it('false когда юзер не аутентифицирован', () => {
      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(false);
    });

    it('true когда auth.isAuthenticated и зеркалит его значение', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin' }, 3600));

      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(auth.isAuthenticated);
      expect(store.canShowTour).toBe(true);
    });

    it('false при истёкшем токене', () => {
      const auth = useAuthStore();
      auth.setTokens(createMockJWT({ username: 'admin' }, -100));

      const store = useOnboardingStore();
      expect(store.canShowTour).toBe(false);
    });
  });

  describe('cross-page переходы', () => {
    it('advanceSegment сдвигает индекс на 1 и поднимает pendingSegment', () => {
      const store = useOnboardingStore();
      store.start();
      store.setIndex(4);
      store.advanceSegment();
      expect(store.currentIndex).toBe(5);
      expect(store.pendingSegment).toBe(true);
    });

    it('clearPending сбрасывает флаг ожидания навигации', () => {
      const store = useOnboardingStore();
      store.advanceSegment();
      expect(store.pendingSegment).toBe(true);
      store.clearPending();
      expect(store.pendingSegment).toBe(false);
    });

    it('reset чистит pendingSegment вместе с состоянием', () => {
      const store = useOnboardingStore();
      store.advanceSegment();
      store.reset();
      expect(store.pendingSegment).toBe(false);
      expect(store.isActive).toBe(false);
      expect(store.currentIndex).toBe(0);
    });
  });

  describe('isManual (авто vs ручной запуск)', () => {
    it('start() без аргумента - авто (isManual=false)', () => {
      const store = useOnboardingStore();
      store.start();
      expect(store.isManual).toBe(false);
    });

    it('start({ manual: true }) - ручной (флаг для записи completed не ставится)', () => {
      const store = useOnboardingStore();
      store.start({ manual: true });
      expect(store.isManual).toBe(true);
    });
  });
});
