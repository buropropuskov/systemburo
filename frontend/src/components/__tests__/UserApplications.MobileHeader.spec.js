import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { nextTick } from 'vue';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(() => Promise.resolve({ items: [], meta: { total: 0, page: 1, per_page: 30 } })),
  getApplicationById: vi.fn(() => Promise.resolve({ message: 'Не найдена' })),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));

import UserApplications from '../UserApplications.vue';
import SearchComponent from '../SearchComponent.vue';

// jsdom не реализует matchMedia - мокаем, иначе initMobileWatcher выходит по гарду и
// isMobileHeader навсегда false (тогда мобильная ветка рендера не проверяется).
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

function mountUA() {
  return shallowMount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(() => Promise.resolve()), push: vi.fn() } } },
  });
}

describe('UserApplications: мобильная шапка (поиск иконкой) vs десктоп (#1097 W3 срез 7)', () => {
  let origMatchMedia;
  beforeEach(() => {
    setActivePinia(createPinia());
    origMatchMedia = window.matchMedia;
  });
  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('мобилка (matchMedia matches): иконка-тоггл поиска, всегда-видимый SearchComponent скрыт', async () => {
    mockMatchMedia(true);
    const w = mountUA();
    await nextTick();
    expect(w.vm.isMobileHeader).toBe(true);
    expect(w.find('[data-testid="cabinet-search-icon"]').exists()).toBe(true);
    expect(w.findComponent(SearchComponent).exists()).toBe(false);
  });

  it('десктоп (matchMedia не matches): SearchComponent виден, без мобильной иконки/оверлея', async () => {
    mockMatchMedia(false);
    const w = mountUA();
    await nextTick();
    expect(w.vm.isMobileHeader).toBe(false);
    expect(w.findComponent(SearchComponent).exists()).toBe(true);
    expect(w.find('[data-testid="cabinet-search-icon"]').exists()).toBe(false);
    expect(w.find('.cabinet__search-overlay').exists()).toBe(false);
  });

  it('toggleMobileSearch раскрывает поле поиска оверлеем', async () => {
    mockMatchMedia(true);
    const w = mountUA();
    await nextTick();
    expect(w.vm.showMobileSearch).toBe(false);
    expect(w.find('[data-testid="cabinet-input-search"]').exists()).toBe(false);
    w.vm.toggleMobileSearch();
    await nextTick();
    expect(w.vm.showMobileSearch).toBe(true);
    expect(w.find('[data-testid="cabinet-input-search"]').exists()).toBe(true);
  });

  it('крестик очистки: рендерится по вводу, кликом очищает запрос и сворачивает оверлей', async () => {
    mockMatchMedia(true);
    const w = mountUA();
    await nextTick();
    w.vm.toggleMobileSearch();
    await nextTick();
    // пусто -> крестика нет
    expect(w.find('.cabinet__search-clear').exists()).toBe(false);
    // ввод -> крестик появляется
    w.vm.searchQuery = 'заявка';
    await nextTick();
    const clear = w.find('.cabinet__search-clear');
    expect(clear.exists()).toBe(true);
    // клик по DOM-кнопке (проверяет проводку @click) -> очистка + закрытие
    await clear.trigger('click');
    await nextTick();
    expect(w.vm.searchQuery).toBe('');
    expect(w.vm.showMobileSearch).toBe(false);
    expect(w.find('.cabinet__search-clear').exists()).toBe(false);
  });
});
