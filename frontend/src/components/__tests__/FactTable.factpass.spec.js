import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import FactTable from '../FactTable.vue';
import FactPassModal from '../FactPassModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

// Одна фактовая машина в ответе /cars/fact-for-table -> fetchCarsData её замапит
// (entry_checked/exit_checked=false, status='В работе'), строка отрисуется штатно.
const RAW_CAR = { id: 5, car_number: 'по факту', car_brand: '', organization: 'X', application_id: 1 };

function mountTable(props = {}) {
  return mount(FactTable, {
    props: { tableType: 'cars', tableId: 42, currentUserId: 7, currentUserName: 'Охр', ...props },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false, FactPassModal: true },
    },
  });
}

describe('FactTable - пропуск "по факту" (#1132)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.includes('/cars/fact-for-table')) return Promise.resolve(okResponse([RAW_CAR]));
      if (/\/(unload-places|organizations|companies|marks)/.test(url)) return Promise.resolve(okResponse([]));
      if (url.includes('/cars/fact') || url.includes('/cars/history')) return Promise.resolve(okResponse([]));
      if (url.includes('/license-plate-formats')) return Promise.resolve(okResponse([]));
      return Promise.resolve(okResponse({}));
    });
  });

  it('клик "Въезд" открывает модалку и НЕ шлёт territory-status сразу', async () => {
    const wrapper = mountTable();
    await flushPromises();

    await wrapper.get('.entry-btn').trigger('click');

    expect(wrapper.vm.showPassModal).toBe(true);
    const territoryCalls = apiRequest.mock.calls.filter((c) => String(c[0]).includes('/territory-status'));
    expect(territoryCalls).toHaveLength(0);
  });

  it('confirm модалки шлёт territory-status=1 с данными пропуска и ставит флаг въезда', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.get('.entry-btn').trigger('click');

    const pass = { number: 'А 123 ВС', format_id: 1, format_name: 'Стд', mark_id: null, mark_name: null };
    wrapper.findComponent(FactPassModal).vm.$emit('confirm', pass);
    await flushPromises();

    const call = apiRequest.mock.calls.find((c) => String(c[0]).includes('/cars/5/territory-status'));
    expect(call).toBeTruthy();
    expect(call[1].method).toBe('PUT');
    const body = JSON.parse(call[1].body);
    expect(body.territory_status).toBe(1);
    expect(body.pass).toEqual(pass);

    expect(wrapper.vm.showPassModal).toBe(false);
    expect(wrapper.vm.factData[0].entry_checked).toBe(true);
  });

  it('если сохранение упало — модалка остаётся, флаг въезда НЕ ставится, показан passError', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.get('.entry-btn').trigger('click');

    // territory-status отвечает ошибкой.
    apiRequest.mockImplementation((url) => {
      if (url.includes('/territory-status')) {
        return Promise.resolve({ ok: false, text: async () => 'boom' });
      }
      return Promise.resolve(okResponse([]));
    });

    const pass = { number: 'А 123 ВС', format_id: 1, format_name: 'Стд', mark_id: null, mark_name: null };
    wrapper.findComponent(FactPassModal).vm.$emit('confirm', pass);
    await flushPromises();

    expect(wrapper.vm.showPassModal).toBe(true);
    expect(wrapper.vm.passError).toBeTruthy();
    expect(wrapper.vm.factData[0].entry_checked).toBe(false);
  });

  it('"Выезд" шлёт territory-status=2 напрямую, без модалки', async () => {
    const wrapper = mountTable();
    await flushPromises();
    // Машина уже на территории -> кнопка "Выезд" доступна. Состояние подменяем через
    // setData: запись по ссылке (`vm.factData[0].x = ...`) до перерисовки не доходит,
    // как только у компонента появляется setup() - vm отдаёт данные через свой прокси.
    await wrapper.setData({ factData: [{ ...wrapper.vm.factData[0], entry_checked: true }] });

    await wrapper.get('.exit-btn').trigger('click');
    await flushPromises();

    expect(wrapper.vm.showPassModal).toBe(false);
    const call = apiRequest.mock.calls.find((c) => String(c[0]).includes('/cars/5/territory-status'));
    expect(call).toBeTruthy();
    const body = JSON.parse(call[1].body);
    expect(body.territory_status).toBe(2);
    expect(body.pass).toBeUndefined();
  });
});
