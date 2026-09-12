import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
const apiRequestRaw = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
  apiRequestRaw: (...args) => apiRequestRaw(...args),
}));

import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function pageResponse(items = [], total = items.length) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: { total, page: 1, per_page: 50 } }) };
}

function mountModal(props = {}) {
  return mount(CarsTableHistoryModal, {
    props: { cars: [], ...props },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTableHistoryModal - история своей таблицы (#1307)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve(okResponse({ users: [] })));
    apiRequestRaw.mockReset();
    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse()));
  });

  it('с идентификатором таблицы запрашивает историю этой таблицы', async () => {
    mountModal({ tableId: 42 });
    await flushPromises();

    const urls = apiRequestRaw.mock.calls.map(([url]) => url);
    expect(urls.some(url => url.startsWith('/cars/history/table/42?'))).toBe(true);
    expect(urls.some(url => url.startsWith('/cars/history/all'))).toBe(false);
  });

  it('без идентификатора таблицы остаётся общая история', async () => {
    mountModal();
    await flushPromises();

    const urls = apiRequestRaw.mock.calls.map(([url]) => url);
    expect(urls.some(url => url.startsWith('/cars/history/all?'))).toBe(true);
  });

  // Список отметивших приходит отдельным запросом и сужается таблицей: собранный из
  // загруженной страницы, он предлагал бы не тех, кто отмечал (#2469).
  it('список пользователей фильтра запрашивается по своей таблице', async () => {
    mountModal({ tableId: 42 });
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/cars/history/filter-options?table_id=42', { method: 'GET' });
  });
});
