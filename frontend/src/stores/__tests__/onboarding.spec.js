import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { onboardingSteps, ONBOARDING_VERSION } from '@/components/onboarding/onboardingSteps';

const STORAGE_KEY = 'onboarding-tour';

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
    localStorage.clear();
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

    it('reset сбрасывает активность и индекс', () => {
      const store = useOnboardingStore();
      store.start();
      store.setIndex(2);
      store.reset();

      expect(store.isActive).toBe(false);
      expect(store.currentIndex).toBe(0);
    });
  });

  describe('totalSteps', () => {
    it('равен длине onboardingSteps', () => {
      const store = useOnboardingStore();
      expect(store.totalSteps).toBe(onboardingSteps.length);
    });
  });

  describe('markCompleted / hasCompleted', () => {
    it('markCompleted пишет корректный JSON-флаг в localStorage', () => {
      const store = useOnboardingStore();
      store.markCompleted();

      const raw = localStorage.getItem(STORAGE_KEY);
      expect(JSON.parse(raw)).toEqual({ completed: true, version: ONBOARDING_VERSION });
    });

    it('hasCompleted true после markCompleted', () => {
      const store = useOnboardingStore();
      store.markCompleted();
      expect(store.hasCompleted()).toBe(true);
    });

    it('hasCompleted false когда флага нет', () => {
      const store = useOnboardingStore();
      expect(store.hasCompleted()).toBe(false);
    });

    it('hasCompleted false при чужой версии флага', () => {
      const store = useOnboardingStore();
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ completed: true, version: 0 }));
      expect(store.hasCompleted()).toBe(false);
    });

    it('hasCompleted false при битом JSON', () => {
      const store = useOnboardingStore();
      localStorage.setItem(STORAGE_KEY, 'not-json');
      expect(store.hasCompleted()).toBe(false);
    });

    it('markCompleted не падает когда localStorage.setItem кидает', () => {
      const store = useOnboardingStore();
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('storage disabled');
      });

      expect(() => store.markCompleted()).not.toThrow();
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
});
