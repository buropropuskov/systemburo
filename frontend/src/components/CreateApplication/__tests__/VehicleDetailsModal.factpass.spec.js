import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import VehicleDetailsModal from '../VehicleDetailsModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function mountModal(props = {}) {
  return shallowMount(VehicleDetailsModal, {
    props: {
      show: false,
      vehicle: { id: 5, car_number: 'по факту' },
      source: 'facttable',
      showCarFeatures: true,
      ...props,
    },
    global: { stubs: { teleport: true, transition: false } },
  });
}

describe('VehicleDetailsModal - данные пропуска "по факту" (#1132)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('passInfo возвращает снимок только для записи с number, иначе null', () => {
    const wrapper = mountModal();
    const vm = wrapper.vm;
    expect(vm.passInfo({ action_type: 'entry', metadata: { number: 'А 123 ВС', mark_name: 'BMW', format_name: 'Стд' } }))
      .toEqual({ number: 'А 123 ВС', mark_name: 'BMW', format_name: 'Стд' });
    expect(vm.passInfo({ action_type: 'entry', metadata: null })).toBeNull();
    expect(vm.passInfo({ action_type: 'exit' })).toBeNull();
    expect(vm.passInfo({})).toBeNull();
  });

  // application_number реальных заявок уже начинается с «№» (проверено на стенде:
  // «№ 20260808/027»), свой знак давал «Заявка №№ 20260808/027».
  it('предупреждение об активной заявке печатает номер как есть, без второго «№»', () => {
    const wrapper = mountModal({
      show: true,
      activeInfo: {
        application_number: '№ 20260808/027',
        entry_date_to: '2026-08-10',
        entry_time_to: '18:00:00',
        organization_name: 'ООО Ромашка',
      },
    });

    const text = wrapper.find('.active-warning-section').text();
    expect(text).toContain('Заявка № 20260808/027');
    expect(text).not.toContain('№№');
  });

  it('для source=facttable история грузится по одной машине (/cars/:id/history), не unified', async () => {
    apiRequest.mockImplementation((url) => {
      if (url === '/cars/5/history') {
        return Promise.resolve(okResponse([
          { id: 1, action_type: 'entry', metadata: { number: 'А 123 ВС', mark_name: 'BMW', format_name: 'Стд' } },
        ]));
      }
      return Promise.resolve(okResponse([]));
    });

    const wrapper = mountModal();
    await wrapper.vm.loadCarHistory();

    const urls = apiRequest.mock.calls.map((c) => String(c[0]));
    expect(urls).toContain('/cars/5/history');
    expect(urls.some((u) => u.includes('/cars/history/unified'))).toBe(false);

    expect(wrapper.vm.entryExitHistory).toHaveLength(1);
    expect(wrapper.vm.passInfo(wrapper.vm.entryExitHistory[0])).toEqual({
      number: 'А 123 ВС',
      mark_name: 'BMW',
      format_name: 'Стд',
    });
  });
});
