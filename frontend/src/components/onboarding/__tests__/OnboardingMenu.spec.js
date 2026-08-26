import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import OnboardingMenu from '../OnboardingMenu.vue';
import { useOnboardingStore } from '@/stores/onboarding';
import { useAuthStore } from '@/stores/auth';
import { usePermissionsStore } from '@/stores/permissions';
import { ONBOARDING_VERSION } from '../onboardingSteps';
import { SECURITY_ONBOARDING_VERSION } from '../securityOnboardingSteps';
import { getOnboardingStatus } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';

vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn(),
  markOnboardingComplete: vi.fn().mockResolvedValue({ message: 'ok' }),
  getSecurityFactRoute: vi.fn().mockResolvedValue(null),
}));

vi.mock('@/api/approvers', () => ({
  getMyApprovalRole: vi.fn(),
}));

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

/** Меню телепортит список в body - искать пункты нужно там, а не в поддереве обёртки. */
const menuItem = (key) => document.querySelector(`[data-testid="ob-tour-${key}"]`);
const menuIsOpen = () => Boolean(document.querySelector('.base-dropdown__menu'));

let wrapper;

function login({ security = false } = {}) {
  const auth = useAuthStore();
  auth.setTokens(createMockJWT({ username: 'ivanov' }));
  if (security) auth.userTypeCode = 'security';
}

async function mountMenu() {
  wrapper = mount(OnboardingMenu, { attachTo: document.body });
  await flushPromises();
  return wrapper;
}

describe('OnboardingMenu', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getOnboardingStatus.mockResolvedValue({ completed: {} });
    getMyApprovalRole.mockResolvedValue({ is_approver: false, is_reviewer: false });
  });

  afterEach(() => {
    wrapper?.unmount();
    document.body.innerHTML = '';
  });

  describe('видимость кнопки', () => {
    it('не рендерит кнопку для неаутентифицированного юзера', async () => {
      await mountMenu();
      expect(document.querySelector('[data-testid="ob-start-button"]')).toBe(null);
    });

    it('рендерит кнопку «Обучение», когда есть хотя бы один доступный тур', async () => {
      login();
      await mountMenu();

      const btn = document.querySelector('[data-testid="ob-start-button"]');
      expect(btn).not.toBe(null);
      expect(btn.textContent).toContain('Обучение');
      expect(btn.classList.contains('lk-button')).toBe(true);
      expect(btn.classList.contains('lk-button--secondary')).toBe(true);
    });
  });

  describe('единственный доступный тур запускается без меню', () => {
    it('клик по кнопке стартует тур сразу, список не открывается', async () => {
      login();
      const store = useOnboardingStore();
      await mountMenu();

      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      expect(store.isActive).toBe(true);
      expect(store.activeTourKey).toBe('user');
      expect(store.isManual).toBe(true);
      expect(menuIsOpen()).toBe(false);
    });
  });

  describe('несколько доступных туров - список', () => {
    it('клик открывает меню, а не запускает тур', async () => {
      login({ security: true });
      const store = useOnboardingStore();
      await mountMenu();

      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      expect(store.isActive).toBe(false);
      expect(menuIsOpen()).toBe(true);
      expect(menuItem('user')).not.toBe(null);
      expect(menuItem('guard')).not.toBe(null);
    });

    it('недоступные и ненаписанные туры отсутствуют физически, а не скрыты', async () => {
      login({ security: true });
      await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      ['approve', 'accept', 'admin'].forEach((key) => {
        expect(menuItem(key)).toBe(null);
      });
      expect(document.querySelectorAll('.base-dropdown__item').length).toBe(2);
    });

    it('пункт несёт название и описание тура', async () => {
      login({ security: true });
      await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      const guard = menuItem('guard');
      expect(guard.textContent).toContain('Охранник');
      expect(guard.querySelector('.ob-menu__description').textContent.length).toBeGreaterThan(0);
    });

    it('клик по пункту запускает именно его тур в ручном режиме', async () => {
      login({ security: true });
      const store = useOnboardingStore();
      await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      menuItem('guard').closest('.base-dropdown__item').click();
      await flushPromises();

      expect(store.activeTourKey).toBe('guard');
      expect(store.isManual).toBe(true);
    });
  });

  describe('бейджи состояния', () => {
    it('пройденный тур помечен «Пройден»', async () => {
      getOnboardingStatus.mockResolvedValue({
        completed: { user: ONBOARDING_VERSION, guard: null },
        finished: ['user'],
      });
      login({ security: true });
      await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      expect(menuItem('user').querySelector('.ob-menu__badge').textContent).toBe('Пройден');
      expect(menuItem('guard').querySelector('.ob-menu__badge')).toBe(null);
    });

    it('тур с выросшей версией помечен «Обновлён»', async () => {
      getOnboardingStatus.mockResolvedValue({
        completed: { user: ONBOARDING_VERSION, guard: SECURITY_ONBOARDING_VERSION - 1 },
      });
      login({ security: true });
      await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      const badge = menuItem('guard').querySelector('.ob-menu__badge');
      expect(badge.textContent).toBe('Обновлён');
      expect(badge.classList.contains('ob-menu__badge--updated')).toBe(true);
    });
  });

  describe('загрузка контекста гейтинга', () => {
    it('при монтировании тянет роль согласования и статус прохождения', async () => {
      login();
      await mountMenu();

      expect(getMyApprovalRole).toHaveBeenCalledOnce();
      expect(getOnboardingStatus).toHaveBeenCalledOnce();
    });

    it('уже загруженный статус повторно не запрашивается', async () => {
      login();
      const store = useOnboardingStore();
      await store.loadStatus();
      getOnboardingStatus.mockClear();

      await mountMenu();
      expect(getOnboardingStatus).not.toHaveBeenCalled();
    });

    it('право на раздел добавляет тур в список без смены типа пользователя', async () => {
      login();
      const permissions = usePermissionsStore();
      permissions.mode = 'normal';
      // page.available, а не page.tables: тур охранника гейтится доступом к
      // «Доступным мне», где живёт большая часть его шагов. Смысл проверки тот же -
      // тур попадает в меню по ПРАВУ, без смены типа учётной записи.
      permissions.effective = { 'page.available': { value: 'allow', source: 'base' } };
      await mountMenu();

      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();
      expect(menuItem('guard')).not.toBe(null);
    });
  });
});
