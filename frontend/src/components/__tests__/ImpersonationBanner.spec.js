import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ImpersonationBanner from '../ImpersonationBanner.vue';
import { useAuthStore } from '@/stores/auth';

// Настоящий push возвращает промис, и компонент вешает на него catch -
// заглушка обязана вести себя так же, иначе падает не компонент, а мок.
const push = vi.fn(() => Promise.resolve());

function mountBanner() {
  return mount(ImpersonationBanner, {
    attachTo: document.body,
    global: { mocks: { $router: { push } } },
  });
}

function startSession(store, { minutesLeft = 30, fullName = 'Иванов Иван' } = {}) {
  store.impersonation = {
    id: 42,
    username: 'ivanov',
    fullName,
    expiresAt: new Date(Date.now() + minutesLeft * 60 * 1000).toISOString(),
  };
}

describe('ImpersonationBanner - полоса режима «войти как пользователь» (#1912)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    push.mockReset();
    push.mockImplementation(() => Promise.resolve());
    document.body.innerHTML = '';
  });
  afterEach(() => { document.body.innerHTML = ''; });

  it('вне режима не рисуется', () => {
    const wrapper = mountBanner();
    expect(document.body.querySelector('[data-testid="impersonation-bar"]')).toBeNull();
    wrapper.unmount();
  });

  it('в режиме называет, от чьего имени идёт работа, и остаток срока', async () => {
    const auth = useAuthStore();
    startSession(auth, { minutesLeft: 30 });
    const wrapper = mountBanner();
    await wrapper.vm.$nextTick();

    expect(document.body.querySelector('[data-testid="impersonation-bar"]')).not.toBeNull();
    expect(document.body.querySelector('[data-testid="impersonation-bar-name"]').textContent)
      .toBe('Иванов Иван');
    expect(document.body.querySelector('[data-testid="impersonation-bar-timer"]').textContent)
      .toContain('30 мин');
    wrapper.unmount();
  });

  it('без заполненного ФИО показывает логин - полоса не может остаться безымянной', async () => {
    const auth = useAuthStore();
    startSession(auth, { fullName: '' });
    const wrapper = mountBanner();
    await wrapper.vm.$nextTick();

    expect(document.body.querySelector('[data-testid="impersonation-bar-name"]').textContent)
      .toBe('ivanov');
    wrapper.unmount();
  });

  it('истёкший срок не показывает таймер, но полосу оставляет на месте', async () => {
    const auth = useAuthStore();
    startSession(auth, { minutesLeft: -5 });
    const wrapper = mountBanner();
    await wrapper.vm.$nextTick();

    expect(document.body.querySelector('[data-testid="impersonation-bar"]')).not.toBeNull();
    expect(document.body.querySelector('[data-testid="impersonation-bar-timer"]')).toBeNull();
    wrapper.unmount();
  });

  it('кнопка возврата закрывает режим и уводит на стартовый экран', async () => {
    const auth = useAuthStore();
    startSession(auth);
    const end = vi.spyOn(auth, 'endImpersonation').mockImplementation(async () => {
      auth.impersonation = null;
      return true;
    });

    const wrapper = mountBanner();
    await wrapper.vm.$nextTick();
    document.body.querySelector('[data-testid="impersonation-bar-back"]')
      .dispatchEvent(new Event('click', { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    await wrapper.vm.$nextTick();

    expect(end).toHaveBeenCalled();
    expect(push).toHaveBeenCalledWith('/news');
    expect(document.body.querySelector('[data-testid="impersonation-bar"]')).toBeNull();
    wrapper.unmount();
  });

  it('неудачный возврат уводит на вход, а не оставляет в чужой учётной записи', async () => {
    const auth = useAuthStore();
    startSession(auth);
    vi.spyOn(auth, 'endImpersonation').mockImplementation(async () => {
      auth.impersonation = null;
      return false;
    });

    const wrapper = mountBanner();
    await wrapper.vm.$nextTick();
    document.body.querySelector('[data-testid="impersonation-bar-back"]')
      .dispatchEvent(new Event('click', { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(push).toHaveBeenCalledWith('/');
    wrapper.unmount();
  });
});
