import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(), disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()), onStatus: vi.fn(() => vi.fn()),
  },
}));

import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';

const STUBS = { teleport: true, transition: false, 'transition-group': false };

function mountCars(props = {}) {
  return mount(CarsTable, {
    props: { tableName: 'КПП 1', tableId: 42, currentUserId: 1, currentUserName: 'Тест', ...props },
    global: { stubs: STUBS },
  });
}

// #1307: столбцы, не поместившиеся по ширине, скрываются и уходят в «Подробнее».
describe('Скрытие не поместившихся столбцов (#1307)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve({ ok: true, json: async () => [] }));
  });

  it('столбец из overflowFields помечается схлопнутым и не считается видимым', async () => {
    const wrapper = mountCars();
    await flushPromises();
    await wrapper.setData({
      fieldsVisibility: { car_number: true, company: true },
      fieldOrders: { car_number: 0, company: 1 },
      overflowFields: ['company'],
    });

    expect(wrapper.vm.fieldColClass('company')).toBe('col--collapsed');
    expect(wrapper.vm.isFieldVisible('company')).toBe(false);
    expect(wrapper.vm.fieldColClass('car_number')).toBe('');
    expect(wrapper.vm.isFieldVisible('car_number')).toBe(true);
  });

  it('скрытые по ширине попадают в «Подробнее» в порядке столбцов', async () => {
    const wrapper = mountCars();
    await flushPromises();
    await wrapper.setData({
      fieldsVisibility: { car_number: true, company: true, application_id: true },
      fieldOrders: { car_number: 0, company: 3, application_id: 1 },
      overflowFields: ['company', 'application_id'],
    });

    expect(wrapper.vm.hiddenInPortraitFields()).toEqual(['application_id', 'company']);
  });

  it('без скрытых столбцов панель «Подробнее» не нужна', async () => {
    const wrapper = mountCars();
    await flushPromises();
    await wrapper.setData({ fieldsVisibility: { car_number: true }, overflowFields: [] });

    expect(wrapper.vm.hiddenInPortraitFields()).toEqual([]);
    expect(wrapper.find('.expand-btn').exists()).toBe(false);
  });

  it('столбцу задаётся минимальная ширина, ниже которой он не сжимается', async () => {
    const wrapper = mountCars();
    await flushPromises();
    await wrapper.setData({ fieldWidths: { organization: 18 }, fieldOrders: { organization: 2 } });

    expect(wrapper.vm.getColStyle('organization')).toMatchObject({
      flexGrow: 18,
      minWidth: '150px',
    });
  });

  it('в таблице людей механизм тот же', async () => {
    const wrapper = mount(PeopleTable, {
      props: { tableName: 'КПП-72', currentUserId: 1, currentUserName: 'Тест' },
      global: { stubs: STUBS },
    });
    await flushPromises();
    await wrapper.setData({
      fieldsVisibility: { last_name: true, position: true },
      fieldOrders: { last_name: 0, position: 5 },
      overflowFields: ['position'],
    });

    expect(wrapper.vm.fieldColClass('position')).toBe('col--collapsed');
    expect(wrapper.vm.hiddenInPortraitFields()).toEqual(['position']);
  });
});
