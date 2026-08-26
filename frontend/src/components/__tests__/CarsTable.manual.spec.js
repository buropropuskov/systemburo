import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import CarsTable from '../CarsTable.vue';

const previewFields = [
  { field_name: 'car_number', is_visible: true, display_order: 0 },
  { field_name: 'car_brand', is_visible: true, display_order: 1 },
  { field_name: 'application_id', is_visible: true, display_order: 2 },
];

function baseItem(overrides) {
  return {
    id: 1,
    car_number: 'А1',
    car_brand: 'BMW',
    plateNumber: 'А1',
    mark: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_time_from: '08:00',
    entry_time_to: '18:00',
    unloadPlaces: [],
    organization_name: 'ООО',
    ...overrides,
  };
}

function mountPreview(items) {
  return mount(CarsTable, {
    props: { preview: true, previewFields, previewItems: items },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTable - метка "Добавлено вручную" (#1049 S8)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('ручная машина (applicationId === null) рисует бейдж, заявочная - номер', () => {
    const wrapper = mountPreview([
      baseItem({ id: 1, applicationId: null, applicationNumber: null }),
      baseItem({ id: 2, applicationId: 5, applicationNumber: '2026-5' }),
    ]);

    const badges = wrapper.findAll('.manual-badge');
    expect(badges).toHaveLength(1);
    expect(badges[0].text()).toBe('Добавлено вручную');
    expect(wrapper.text()).toContain('2026-5');
  });

  it('isManualItem: null -> true, число -> false, undefined -> false', () => {
    const wrapper = mountPreview([baseItem({ applicationId: 5 })]);
    expect(wrapper.vm.isManualItem({ applicationId: null })).toBe(true);
    expect(wrapper.vm.isManualItem({ applicationId: 5 })).toBe(false);
    expect(wrapper.vm.isManualItem({})).toBe(false);
  });
});
