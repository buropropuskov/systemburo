import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
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
    apiRequest.mockImplementation(() => Promise.resolve(okResponse([])));
  });

  it('с идентификатором таблицы запрашивает историю этой таблицы', async () => {
    mountModal({ tableId: 42 });
    await flushPromises();

    const urls = apiRequest.mock.calls.map(([url]) => url);
    expect(urls).toContain('/cars/history/table/42');
    expect(urls).not.toContain('/cars/history/all');
  });

  it('без идентификатора таблицы остаётся общая история', async () => {
    mountModal();
    await flushPromises();

    const urls = apiRequest.mock.calls.map(([url]) => url);
    expect(urls).toContain('/cars/history/all');
  });
});
