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
import EmployeesTableHistoryModal from '../CreateApplication/EmployeesTableHistoryModal.vue';

function page(items) {
  return {
    ok: true,
    json: async () => ({ success: true, data: items, meta: { total: items.length, page: 1, per_page: 50 } }),
  };
}

const stubs = { global: { stubs: { teleport: true, transition: false, 'transition-group': false } } };

beforeEach(() => {
  setActivePinia(createPinia());
  apiRequest.mockReset();
  apiRequest.mockResolvedValue({ ok: true, json: async () => ({ users: [], employees: [] }) });
  apiRequestRaw.mockReset();
});

// Запись прохода машины, которой больше нет в справочнике (#2485). Раньше такие строки
// журнал не показывал вовсе - их уносил INNER JOIN, - поэтому проверяем и то, что запись
// видна, и то, что она опознаётся по снимку, и пометку «запись удалена».
describe('журнал машин - проход удалённой машины', () => {
  it('показывает снимок и помечает запись', async () => {
    apiRequestRaw.mockResolvedValue(page([{
      id: 1,
      car_id: 42,
      user_name: 'Иванов И.И.',
      action_type: 'entry',
      created_at: '2026-09-12T10:00:00Z',
      car_number: null,
      car_brand: null,
      organization: null,
      subject: 'А123ВС77 Kamaz',
      entity_deleted: true,
    }]));

    const wrapper = mount(CarsTableHistoryModal, { props: { cars: [], tableId: 7 }, ...stubs });
    await flushPromises();

    const row = wrapper.find('.history-item');
    expect(row.text()).toContain('А123ВС77 Kamaz');
    expect(row.find('.deleted-badge').exists()).toBe(true);
    expect(row.text()).not.toContain('Автомобиль ID: 42');
  });

  it('без снимка остаётся опознаваемой по идентификатору', async () => {
    apiRequestRaw.mockResolvedValue(page([{
      id: 2,
      car_id: 42,
      user_name: 'Иванов И.И.',
      action_type: 'exit',
      created_at: '2026-09-12T11:00:00Z',
      car_number: null,
      subject: null,
      entity_deleted: true,
    }]));

    const wrapper = mount(CarsTableHistoryModal, { props: { cars: [], tableId: 7 }, ...stubs });
    await flushPromises();

    expect(wrapper.find('.history-item').text()).toContain('Автомобиль ID: 42');
  });

  it('живую машину показывает как раньше, без пометки', async () => {
    apiRequestRaw.mockResolvedValue(page([{
      id: 3,
      car_id: 7,
      user_name: 'Иванов И.И.',
      action_type: 'entry',
      created_at: '2026-09-12T12:00:00Z',
      car_number: 'В777ЕЕ77',
      car_brand: 'Volvo',
      organization: 'ООО Ромашка',
      subject: 'В777ЕЕ77 Volvo',
      entity_deleted: false,
    }]));

    const wrapper = mount(CarsTableHistoryModal, { props: { cars: [], tableId: 7 }, ...stubs });
    await flushPromises();

    const row = wrapper.find('.history-item');
    expect(row.text()).toContain('ООО Ромашка');
    expect(row.find('.deleted-badge').exists()).toBe(false);
  });
});

describe('журнал людей - проход удалённого сотрудника', () => {
  it('показывает снимок ФИО и помечает запись', async () => {
    apiRequestRaw.mockResolvedValue(page([{
      id: 10,
      employee_id: 99,
      user_name: 'Иванов И.И.',
      action_type: 'entry',
      created_at: '2026-09-12T10:00:00Z',
      employee_last_name: null,
      employee_first_name: null,
      employee_middle_name: null,
      subject: 'Петров Пётр Петрович',
      entity_deleted: true,
    }]));

    const wrapper = mount(EmployeesTableHistoryModal, { props: { tableId: 4 }, ...stubs });
    await flushPromises();

    const row = wrapper.find('.history-item');
    expect(row.text()).toContain('Петров Пётр Петрович');
    expect(row.find('.deleted-badge').exists()).toBe(true);
  });

  it('живого сотрудника показывает из справочника, без пометки', async () => {
    apiRequestRaw.mockResolvedValue(page([{
      id: 11,
      employee_id: 5,
      user_name: 'Иванов И.И.',
      action_type: 'entry',
      created_at: '2026-09-12T10:00:00Z',
      employee_last_name: 'Сидоров',
      employee_first_name: 'Сидор',
      subject: 'Сидоров Сидор',
      entity_deleted: false,
    }]));

    const wrapper = mount(EmployeesTableHistoryModal, { props: { tableId: 4 }, ...stubs });
    await flushPromises();

    const row = wrapper.find('.history-item');
    expect(row.text()).toContain('Сидоров Сидор');
    expect(row.find('.deleted-badge').exists()).toBe(false);
  });
});
