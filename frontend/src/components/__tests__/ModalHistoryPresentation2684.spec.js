import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { readFileSync } from 'node:fs';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }) }));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));
import CarHistoryModal from '../CarHistoryModal.vue';
import EmployeeHistoryModal from '../CreateApplication/EmployeeHistoryModal.vue';
beforeEach(() => setActivePinia(createPinia()));
describe('modal history presentation', () => {
  it.each([CarHistoryModal, EmployeeHistoryModal])('renders readable before and after values and author', async component => {
    const wrapper = mount(component, { props: { carId: 1 }, global: { stubs: { teleport: true, HistoryEntityCard: true, DateFilter: true } } });
    await flushPromises();
    await wrapper.setData({ loading: false, history: [{ id: 1, action_type: 'dates_changed', field_name: 'period', user_name: 'Оператор', comment: 'Причина', created_at: '2030-02-01T09:00:00+03:00', old_value: 'inherit: 01.02.2030 - 02.02.2030', new_value: 'individual: 01.02.2030 - 03.02.2030' }] });
    expect(wrapper.text()).toContain('Изменён срок действия');
    expect(wrapper.text()).toContain('Изменил: Оператор');
    expect(wrapper.text()).toContain('Было: Срок вложения:');
    expect(wrapper.text()).toContain('Стало: Индивидуальный срок:');
    expect(wrapper.text()).toContain('Причина');
    expect(wrapper.text()).not.toContain('dates_changed');
    wrapper.unmount();
  });
  it('retains mobile sheet radius while desktop defaults to 30px', () => {
    const css = readFileSync('src/components/ui/BaseModal.vue', 'utf8');
    expect(css).toContain('border-radius: var(--base-modal-radius, 30px)');
    expect(css).toContain('border-radius: 16px 16px 0 0');
    const header = readFileSync('src/assets/detail-card-header.css', 'utf8');
    expect(header).toContain('height: 50px');
    expect(header).not.toContain('70px');
  });
});
