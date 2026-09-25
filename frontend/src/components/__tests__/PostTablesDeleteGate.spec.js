import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { usePermissionsStore } from '@/stores/permissions';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));
vi.mock('@/services/eventStream', () => ({
  default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()), onStatus: vi.fn(() => vi.fn()) },
}));

import CarsTable from '../CarsTable.vue';
import FactTable from '../FactTable.vue';

// Бэк снимает элемент с поста только при праве table.<пост>.delete (#2600), поэтому
// кнопка удаления видна по нему же. entity.cars.delete есть у базовой роли - это право
// на свой реестр машин, а не на пост, и его одного для кнопки мало.

const okResponse = (data) => ({ ok: true, json: async () => data });
const stubs = { teleport: true, transition: false, 'transition-group': false, FactPassModal: true };

function grant(keys) {
  const store = usePermissionsStore();
  store.mode = 'normal';
  store.effective = Object.fromEntries(keys.map((k) => [k, { value: 'allow', source: 'test' }]));
}

const carRow = { id: 1, car_number: 'А1', car_brand: 'BMW', plateNumber: 'А1', mark: 'BMW', status: 'В работе',
  entry_date_to: '2026-06-05', entry_time_from: '08:00', entry_time_to: '18:00', unloadPlaces: [],
  organization_name: 'ООО', target_tables_count: 1 };
const factRow = { id: 1, organization_name: 'Ромашка', car_brand: 'Toyota', company: '', status: 'В работе',
  entry_time_from: '', entry_time_to: '', pass_time: '', entry_date_to: '' };

async function mountCars() {
  const w = mount(CarsTable, { props: { tableName: 'kpp1', tableId: 42, currentUserId: 1, currentUserName: 'Т' }, global: { stubs } });
  await flushPromises();
  await w.setData({ itemsData: [carRow] });
  return w;
}

async function mountFact() {
  const w = mount(FactTable, {
    props: { tableType: 'cars', tableId: 42, tableData: { table: { name: 'kpp1' } }, currentUserId: 7, currentUserName: 'О' },
    global: { stubs },
  });
  await flushPromises();
  w.vm.factData = [factRow];
  await w.vm.$nextTick();
  return w;
}

describe('таблицы постов: кнопка удаления по праву поста (#2600)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve(okResponse([])));
  });

  it('CarsTable: только entity.cars.delete - кнопки нет', async () => {
    grant(['entity.cars.delete']);
    const w = await mountCars();
    expect(w.find('.item-row').exists()).toBe(true);
    expect(w.find('.delete-btn').exists()).toBe(false);
  });

  it('CarsTable: право удаления на этот пост - кнопка есть', async () => {
    grant(['table.kpp1.delete']);
    const w = await mountCars();
    expect(w.find('.delete-btn').exists()).toBe(true);
  });

  it('FactTable: без права поста кнопки нет, с правом - есть', async () => {
    grant([]);
    let w = await mountFact();
    expect(w.findAll('.actions-col').length).toBeGreaterThan(0);
    expect(w.find('.delete-btn').exists()).toBe(false);

    grant(['table.kpp1.delete']);
    w = await mountFact();
    expect(w.find('.delete-btn').exists()).toBe(true);
  });
});
