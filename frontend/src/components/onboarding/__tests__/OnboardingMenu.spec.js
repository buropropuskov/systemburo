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
import { getMyPermissions } from '@/api/permissions';
import { getMe } from '@/api/auth';
import { getUserApplicationsPaginated } from '@/api/applications';
import { useDeletionsStore } from '@/stores/deletions';
import OnboardingStartDialog from '../OnboardingStartDialog.vue';

vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/api/auth', async original => ({ ...await original(), getMe: vi.fn(async () => ({ user_type_code: 'user' })) }));
vi.mock('@/api/applications', () => ({ getUserApplicationsPaginated: vi.fn() }));

vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn(),
  markOnboardingComplete: vi.fn().mockResolvedValue({ message: 'ok' }),
  getSecurityPostContext: vi.fn().mockResolvedValue(null),
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
  auth.setTokens(createMockJWT({ username: 'ivanov', user_id: 101 }));
  auth.userTypeCode = 'user';
  if (security) auth.userTypeCode = 'security';
  getMe.mockResolvedValue({ user_type_code: security ? 'security' : 'user' });
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
    getMe.mockResolvedValue({ user_type_code: 'user' });
    getMyPermissions.mockResolvedValue({ mode: 'normal', permissions: [], denied: [], banned: false });
    getUserApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0 } });
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
    it('клик открывает последовательный старт, затем «Пройти сначала» запускает тур', async () => {
      login();
      const store = useOnboardingStore();
      await mountMenu();

      await wrapper.find('[data-testid="ob-start-button"]').trigger('click');
      await flushPromises();

      expect(store.isActive).toBe(false);
      document.querySelector('[data-testid="ob-start-full"]').click();
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
      document.querySelector('[data-testid="ob-start-full"]').click();
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
    it('отмена окна во время подготовки не запускает тур и не открывает окно заново', async () => {
      login(); await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click'); await flushPromises();
      const store = useOnboardingStore(), start = vi.spyOn(store, 'start');
      let release;
      vi.spyOn(store, 'prepareTour').mockReturnValueOnce(new Promise(resolve => { release = resolve; }));
      const dialog = wrapper.findComponent(OnboardingStartDialog);
      dialog.vm.$emit('start'); await flushPromises();
      expect(dialog.props('busy')).toBe(true);
      dialog.vm.$emit('close'); await flushPromises();
      release({ status: 'ready' }); await flushPromises();
      expect(start).not.toHaveBeenCalled(); expect(store.isActive).toBe(false);
      expect(dialog.props('show')).toBe(false); expect(store.hasFinished('user')).toBe(false);
    });
    it('повторное открытие того же тура не оживляет отменённую попытку', async () => {
      login(); await mountMenu();
      const trigger = wrapper.find('[data-testid="ob-start-button"]');
      await trigger.trigger('click'); await flushPromises();
      const store = useOnboardingStore(), start = vi.spyOn(store, 'start');
      let release;
      vi.spyOn(store, 'prepareTour').mockReturnValueOnce(new Promise(resolve => { release = resolve; }));
      const dialog = wrapper.findComponent(OnboardingStartDialog);
      dialog.vm.$emit('start'); await flushPromises(); dialog.vm.$emit('close'); await flushPromises();
      await trigger.trigger('click'); await flushPromises();
      expect(dialog.props('show')).toBe(true);
      release({ status: 'ready' }); await flushPromises();
      expect(start).not.toHaveBeenCalled(); expect(dialog.props('show')).toBe(true);
      dialog.vm.$emit('start'); await flushPromises();
      expect(start).toHaveBeenCalledOnce(); expect(store.isActive).toBe(true);
    });
    it('передаёт смысловой ID выбранной главы настоящему store без полного завершения', async () => {
      login(); await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click'); await flushPromises();
      const dialog=wrapper.findComponent(OnboardingStartDialog), chapters=dialog.props('chapters');
      expect(chapters.length).toBeGreaterThan(1);
      const id=chapters[1].id;
      dialog.vm.$emit('start',{fromStepId:id}); await flushPromises();
      const store=useOnboardingStore();
      expect(store.isActive).toBe(true);expect(store.currentStepId).toBe(id);
      expect(store.hasFinished('user')).toBe(false);
    });
    it('не запускает тур, если загрузка прав завершилась ошибкой', async () => {
      getMyPermissions.mockRejectedValue(new Error('Permissions unavailable'));
      login(); await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click'); await flushPromises();
      expect(usePermissionsStore().loaded).toBe(false);
      expect(useOnboardingStore().isActive).toBe(false);
    });
    it('при доступном Центре тянет роль согласования и статус прохождения', async () => {
      getMyPermissions.mockResolvedValue({ mode: 'normal', permissions: [{ key: 'page.center', value: 'allow' }], denied: [] });
      login();
      await mountMenu();

      expect(getMyApprovalRole).toHaveBeenCalledOnce();
      expect(getOnboardingStatus).toHaveBeenCalledOnce();
    });

    it('объясняет неудачу ручного запуска и повторяет подготовку по следующему клику', async () => {
      getUserApplicationsPaginated.mockRejectedValueOnce(new Error('offline'));
      login(); await mountMenu();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click'); await flushPromises();
      expect(useOnboardingStore().isActive).toBe(false);
      expect(useDeletionsStore().items.some(item => item.type === 'error')).toBe(true);
      expect(wrapper.find('[data-testid="ob-start-button"]').attributes('disabled')).toBeUndefined();
      await wrapper.find('[data-testid="ob-start-button"]').trigger('click'); await flushPromises();
      document.querySelector('[data-testid="ob-start-full"]').click(); await flushPromises();
      expect(useOnboardingStore().isActive).toBe(true);
      expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(2);
    });
    it('одна ожидающая подготовка блокирует повторный клик', async () => {
      let release;
      getUserApplicationsPaginated.mockReturnValueOnce(new Promise(resolve => { release = resolve; }));
      login(); await mountMenu();
      const button = wrapper.find('[data-testid="ob-start-button"]');
      await button.trigger('click'); await flushPromises();
      expect(button.attributes('disabled')).toBeDefined();
      await button.trigger('click'); await flushPromises();
      expect(getUserApplicationsPaginated).toHaveBeenCalledOnce();
      release({ items: [], meta: { total: 0 } }); await flushPromises();
      expect(useOnboardingStore().isActive).toBe(false);
      document.querySelector('[data-testid="ob-start-full"]').click(); await flushPromises();
      expect(useOnboardingStore().isActive).toBe(true);
      expect(button.attributes('disabled')).toBeUndefined();
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
      permissions.loaded = true;
      permissions.fetchedAt = Date.now();
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
