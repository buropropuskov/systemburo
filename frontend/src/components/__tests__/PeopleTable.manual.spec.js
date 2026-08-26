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

import PeopleTable from '../PeopleTable.vue';

const previewFields = [
  { field_name: 'last_name', is_visible: true, display_order: 0 },
  { field_name: 'first_name', is_visible: true, display_order: 1 },
  { field_name: 'application_id', is_visible: true, display_order: 2 },
];

function baseItem(overrides) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    citizenship_name: 'РФ',
    organization_name: 'ООО',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    pass_time: '08:00',
    ...overrides,
  };
}

function mountPreview(items) {
  return mount(PeopleTable, {
    props: { preview: true, previewFields, previewItems: items },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('PeopleTable - метка "Добавлено вручную" (#1049 S9)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('ручной сотрудник (applicationId === null) рисует бейдж, заявочный - номер', () => {
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
