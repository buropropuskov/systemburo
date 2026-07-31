import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated, getApplicationById } from '@/api/applications';
import eventStream from '@/services/eventStream';
import { playPreset } from '@/utils/notificationSound';
import { useAuthStore } from '@/stores/auth';

// Устойчивость бесшовной подгрузки к ошибкам бэка (5xx/сеть, issue #1173): раньше при
// ошибке fetchPage автодогрузка НЕ останавливалась - sentinel оставался видим,
// IntersectionObserver/цикл full-load продолжали слать loadMore, page рос без предела
// (наблюдалось 1->36), а UI одновременно показывал "Нет заявок" и зависшую "Загрузка…".
// Теперь composable выставляет circuit-breaker (canLoadMore=false) и требует ручной retry().

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn(), getApplicationById: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => () => {}),
    onStatus: vi.fn(() => () => {}),
  },
}));

const stubs = {
  teleport: true,
  RefreshButton: true,
  ApplicationDetail: true,
  DateFilter: true,
  FilterTabs: true,
  SkeletonTransition: { template: '<div><slot /></div>' },
  SkeletonTable: true,
  LoaderSpinner: true,
  DownloadBlanksModal: true,
  Badge: true,
  BaseDropdown: true,
};

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

function makeApp(id, over = {}) {
  return {
    id,
    application_number: `A-${id}`,
    sending_datetime: '2026-01-01T10:00:00Z',
    status: 'Согласование',
    confirmation: 'Согласование',
    organization_name: 'Орг',
    sender_name: 'Иванов',
    is_read: true,
    ...over,
  };
}

let wrapper;

describe('ApplicationsCenter — устойчивость к ошибкам бэка (#1173)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue({ ok: false, text: async () => '', json: async () => [] });
    getApplicationsPaginated.mockReset();
    getApplicationById.mockReset();
    playPreset.mockClear();
    eventStream.subscribe.mockClear();
    useAuthStore().token = 'test-token';
  });

  afterEach(() => wrapper?.unmount());

  it('первичная загрузка упала - показан error+retry вместо "Заявок нет", sentinel скрыт', async () => {
    getApplicationsPaginated.mockRejectedValue(new Error('502'));
    wrapper = mountCenter();
    await flushPromises();

    expect(wrapper.vm.listError).toBe(true);
    expect(wrapper.find('[data-testid="center-list-error"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-list-error"]').text()).toContain('Не удалось загрузить');
    // Список пуст из-за ошибки, не потому что заявок нет - обычное сообщение не рендерится.
    expect(wrapper.find('.no-data-message').exists()).toBe(false);
    // hasMore корректно false (total сброшен на 0 при ошибке reset) - sentinel не висит
    // без дела, наблюдатель не подключается.
    expect(wrapper.vm.hasMoreApplications).toBe(false);
    expect(wrapper.find('[data-testid="center-scroll-sentinel"]').exists()).toBe(false);
  });

  it('ошибка догрузки: список остаётся на экране, sentinel показывает error+retry вместо зависшего спиннера', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 6, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    await flushPromises();
    expect(wrapper.vm.applications).toHaveLength(2);

    getApplicationsPaginated.mockRejectedValueOnce(new Error('502'));
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage).catch(() => {});
    await flushPromises();
    await wrapper.vm.$nextTick();

    // Прежние данные не тронуты, page не выросла до 2.
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(wrapper.vm.applicationsPage).toBe(1);
    expect(wrapper.vm.listError).toBe(true);

    // Sentinel остаётся видим (есть ещё непрочитанные страницы), но вместо спиннера -
    // компактный error+retry, а не одновременное "Нет заявок"+"Загрузка…".
    expect(wrapper.find('[data-testid="center-scroll-sentinel"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-scroll-sentinel-error"]').exists()).toBe(true);

    // Circuit-breaker: повторный автовызов (типичная картина зависшего sentinel) не
    // шлёт новый запрос бэку.
    const callsAfterFailure = getApplicationsPaginated.mock.calls.length;
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    expect(getApplicationsPaginated.mock.calls.length).toBe(callsAfterFailure);
  });

  it('кнопка "Повторить" у sentinel восстанавливает автодогрузку и дописывает данные', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 4, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    await flushPromises();

    getApplicationsPaginated.mockRejectedValueOnce(new Error('502'));
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage).catch(() => {});
    await flushPromises();
    await wrapper.vm.$nextTick();

    const retryButton = wrapper.find('[data-testid="center-scroll-sentinel-error"] button');
    expect(retryButton.exists()).toBe(true);

    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(3), makeApp(4)],
      meta: { total: 4, page: 2, per_page: 30 },
    });
    await retryButton.trigger('click');
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.listError).toBe(false);
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2, 3, 4]);
    expect(wrapper.vm.applicationsPage).toBe(2);
    // Автодогрузка возобновилась - hasMore стал false (всё загружено), sentinel скрыт,
    // а не завис на ошибке.
    expect(wrapper.vm.hasMoreApplications).toBe(false);
    expect(wrapper.find('[data-testid="center-scroll-sentinel"]').exists()).toBe(false);
  });

  it('кнопка "Повторить" в primary error-состоянии восстанавливает список', async () => {
    getApplicationsPaginated.mockRejectedValueOnce(new Error('network'));
    wrapper = mountCenter();
    await flushPromises();
    expect(wrapper.vm.listError).toBe(true);

    const retryButton = wrapper.find('[data-testid="center-list-error"] button');
    expect(retryButton.exists()).toBe(true);

    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    await retryButton.trigger('click');
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.listError).toBe(false);
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1]);
    expect(wrapper.find('[data-testid="center-list-error"]').exists()).toBe(false);
  });

  // БЛОКЕР ревью: во время in-flight retry (primary error) НЕ должно мигать "Заявок нет".
  // retry() выставляет composable listLoading, но не верхнеуровневый this.loading -
  // без ветки v-else-if="listLoading" каскад проваливался в v-else ("Заявок нет").
  it('во время in-flight retry рендерится спиннер, НЕ "Заявок нет"', async () => {
    getApplicationsPaginated.mockRejectedValueOnce(new Error('502'));
    wrapper = mountCenter();
    await flushPromises();
    expect(wrapper.vm.listError).toBe(true);

    // Deferred: retry подвисает - ловим ПРОМЕЖУТОЧНОЕ состояние (не синхронный резолв).
    let resolveRetry;
    getApplicationsPaginated.mockImplementationOnce(
      () => new Promise((r) => { resolveRetry = r; }),
    );
    await wrapper.find('[data-testid="center-list-error"] button').trigger('click');
    await wrapper.vm.$nextTick();

    // Пока retry летит: listLoading=true -> спиннер, ни error, ни "Заявок нет".
    expect(wrapper.vm.listLoading).toBe(true);
    expect(wrapper.find('[data-testid="center-list-loading"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-list-error"]').exists()).toBe(false);
    expect(wrapper.find('.no-data-message').exists()).toBe(false);

    resolveRetry({ items: [makeApp(1)], meta: { total: 1, page: 1, per_page: 30 } });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1]);
    expect(wrapper.find('[data-testid="center-list-loading"]').exists()).toBe(false);
  });

  // 🟡3 ревью: retry в режиме full-load (клиентская сортировка) обязан ДОЗАГРУЗИТЬ весь
  // набор, иначе сортировка молча по неполному списку до ручного доскролла.
  it('retry в full-load дозагружает ВЕСЬ набор после ошибки на промежуточной странице', async () => {
    let failPage2 = true;
    getApplicationsPaginated.mockImplementation((params) => {
      if (params.page === 1) {
        return Promise.resolve({ items: [makeApp(1)], meta: { total: 3, page: 1, per_page: 30 } });
      }
      if (params.page === 2) {
        if (failPage2) { failPage2 = false; return Promise.reject(new Error('502')); }
        return Promise.resolve({ items: [makeApp(2)], meta: { total: 3, page: 2, per_page: 30 } });
      }
      return Promise.resolve({ items: [makeApp(3)], meta: { total: 3, page: 3, per_page: 30 } });
    });
    wrapper = mountCenter();
    await flushPromises();
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1]);

    // Клиентская сортировка включает full-load: page1 (reset) + догрузка остатка.
    // page2 падает в середине цикла -> error, набор неполный ([1]).
    wrapper.vm.sortBy('number');
    await flushPromises();
    expect(wrapper.vm.isFullLoad).toBe(true);
    expect(wrapper.vm.listError).toBe(true);
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1]);

    // retry дописывает page2 И возобновляет loadAllRemaining -> все 3 страницы.
    await wrapper.vm.retryApplications();
    await flushPromises();

    expect(wrapper.vm.listError).toBe(false);
    expect([...wrapper.vm.applications.map((a) => a.id)].sort()).toEqual([1, 2, 3]);
    expect(wrapper.vm.hasMoreApplications).toBe(false);
  });
});
