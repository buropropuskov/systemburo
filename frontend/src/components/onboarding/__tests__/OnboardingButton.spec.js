import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import OnboardingButton from '../OnboardingButton.vue';
import { useOnboardingStore } from '@/stores/onboarding';
import { useAuthStore } from '@/stores/auth';

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

describe('OnboardingButton', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  it('не рендерит кнопку для неаутентифицированного юзера', () => {
    const wrapper = mount(OnboardingButton);
    expect(wrapper.find('[data-testid="ob-start-button"]').exists()).toBe(false);
  });

  it('рендерит кнопку когда canShowTour=true', () => {
    useAuthStore().setTokens(createMockJWT({ username: 'admin' }));
    const wrapper = mount(OnboardingButton);

    const btn = wrapper.find('[data-testid="ob-start-button"]');
    expect(btn.exists()).toBe(true);
    expect(btn.text()).toContain('Обучение');
  });

  it('клик запускает тур в ручном режиме', async () => {
    useAuthStore().setTokens(createMockJWT({ username: 'admin' }));
    const store = useOnboardingStore();
    const startSpy = vi.spyOn(store, 'start');

    const wrapper = mount(OnboardingButton);
    await wrapper.find('[data-testid="ob-start-button"]').trigger('click');

    expect(startSpy).toHaveBeenCalledWith({ manual: true });
    expect(store.isActive).toBe(true);
    expect(store.isManual).toBe(true);
  });
});
