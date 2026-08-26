import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
  // Список порциями (#1748 S7): apiRequestRaw возвращает envelope с data+meta,
  // apiRequest не годится - не несёт meta.unread_count/total.
  apiRequestRaw: vi.fn(() => Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ success: true, data: [], meta: { total: 0, unread_count: 0 } }),
  })),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import UserNotifications from '../UserNotifications.vue';

function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function mountN() {
  setActivePinia(createPinia());
  return mount(UserNotifications, {
    props: { show: true },
    attachTo: document.body,
    global: { mocks: { $router: { push: vi.fn(() => Promise.resolve()) } } },
  });
}

describe('UserNotifications - bottom-sheet на мобилке (#1097 W3)', () => {
  let origMatchMedia;
  beforeEach(() => {
    origMatchMedia = window.matchMedia;
    document.body.innerHTML = '';
  });
  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('мобилка (matchMedia matches): панель = sheet, подложка в body, клик по ней закрывает', async () => {
    mockMatchMedia(true);
    const wrapper = mountN();
    await flushPromises();

    expect(wrapper.vm.isSheet).toBe(true);
    const panel = document.body.querySelector('.notifications');
    expect(panel).not.toBeNull();
    expect(panel.classList.contains('notifications--sheet')).toBe(true);

    const backdrop = document.body.querySelector('.notifications-backdrop');
    expect(backdrop).not.toBeNull();

    backdrop.dispatchEvent(new Event('click', { bubbles: true }));
    expect(wrapper.emitted('close')).toBeTruthy();

    wrapper.unmount();
  });

  it('десктоп (matchMedia не matches): inline-дропдаун без подложки и без sheet-класса', async () => {
    mockMatchMedia(false);
    const wrapper = mountN();
    await flushPromises();

    expect(wrapper.vm.isSheet).toBe(false);
    const panel = wrapper.find('.notifications');
    expect(panel.exists()).toBe(true);
    expect(panel.classes()).not.toContain('notifications--sheet');
    expect(document.body.querySelector('.notifications-backdrop')).toBeNull();

    wrapper.unmount();
  });

  it('Escape закрывает панель когда show', async () => {
    mockMatchMedia(true);
    const wrapper = mountN();
    await flushPromises();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toBeTruthy();

    wrapper.unmount();
  });
});
