import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { usePermissionsStore } from '@/stores/permissions';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));
import EmployeeHistoryModal from '../EmployeeHistoryModal.vue';

describe('EmployeeHistoryModal document facts', () => {
  let wrapper;
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().effective = { 'detail.full_history': { value: 'allow' } };
    apiRequest.mockReset();
  });
  afterEach(() => wrapper?.unmount());

  it.each([
    ['passport_series_number', 'Изменены паспортные данные'],
    ['patent_number', 'Изменён номер патента'],
    ['other_permission', 'Изменено иное разрешение на работы'],
  ])('hides legacy %s values and keeps the ordinary diff', async (field, text) => {
    const common = { created_at: '2026-01-15T10:00:00Z', user_name: 'Fixture operator' };
    apiRequest.mockResolvedValue({ ok: true, json: async () => [
      { ...common, id: 1, action_type: 'data_changed', field_name: field,
        old_value: 'SYNTHETIC-OLD-DOC', new_value: 'SYNTHETIC-NEW-DOC' },
      { ...common, id: 2, action_type: 'update', field_name: 'position',
        old_value: 'Fixture old role', new_value: 'Fixture new role' },
    ] });
    wrapper = mount(EmployeeHistoryModal, {
      props: { lastName: 'Synthetic', firstName: 'Fixture' },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();
    expect(wrapper.text()).toContain(text);
    expect(wrapper.text()).not.toContain('SYNTHETIC-OLD-DOC');
    expect(wrapper.text()).not.toContain('SYNTHETIC-NEW-DOC');
    expect(wrapper.text()).not.toContain('пусто');
    expect(wrapper.findAll('.value-change')).toHaveLength(1);
    expect(wrapper.find('.value-change').text()).toContain('Fixture old role');
    expect(wrapper.find('.value-change').text()).toContain('Fixture new role');
  });
});
