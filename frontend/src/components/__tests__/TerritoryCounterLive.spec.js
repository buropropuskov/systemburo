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

import { useDeletionsStore } from '@/stores/deletions';
import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const CAR_ON_TERRITORY = {
  id: 1, car_number: 'А123ВС77', car_brand: 'Камаз', organization: 'Орг', status: 1, territory_status: 1,
};
const EMPLOYEE_ON_TERRITORY = {
  id: 1, last_name: 'Иванов', first_name: 'Иван', status: 1, territory_status: 1,
};

/** Строка без territory_status - бэк такого поля не прислал (старый ответ/урезанный DTO). */
function withoutTerritory(row) {
  const copy = { ...row };
  delete copy.territory_status;
  return copy;
}

function mountCars() {
  return mount(CarsTable, {
    props: { tableName: 'КПП 1', tableId: 42, currentUserId: 1, currentUserName: 'Тест' },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

function mountPeople() {
  return mount(PeopleTable, {
    props: { tableName: 'КПП 1', currentUserId: 1, currentUserName: 'Тест' },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTable - счётчик на территории не проваливается при обновлении', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
  });

  function mockCars(rows, { statusHangs = false, carsFails = false } = {}) {
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/cars/active-for-table/')) {
        return carsFails ? Promise.reject(new Error('network')) : Promise.resolve(okResponse(rows));
      }
      // Второй запрос со статусами намеренно «висит»: до него счётчик уже обязан быть верным.
      if (url.startsWith('/cars/history/current-status')) {
        return statusHangs ? new Promise(() => {}) : Promise.resolve(okResponse([]));
      }
      return Promise.resolve(okResponse([]));
    });
  }

  it('territory_status из строки даёт верный счётчик до ответа current-status', async () => {
    mockCars([CAR_ON_TERRITORY], { statusHangs: true });
    const wrapper = mountCars();
    await flushPromises();

    expect(wrapper.vm.carsOnTerritory).toBe(1);
    wrapper.unmount();
  });

  it('строка без territory_status сохраняет прежнюю отметку, а не сбрасывается в 0', async () => {
    mockCars([CAR_ON_TERRITORY]);
    const wrapper = mountCars();
    await flushPromises();
    expect(wrapper.vm.carsOnTerritory).toBe(1);

    // Повторная загрузка отдаёт ту же машину, но уже без поля статуса.
    mockCars([withoutTerritory(CAR_ON_TERRITORY)], { statusHangs: true });
    wrapper.vm.silentRefresh();
    await flushPromises();

    expect(wrapper.vm.carsOnTerritory).toBe(1);
    wrapper.unmount();
  });

  it('сбой сети при тихом обновлении не стирает строки и счётчик', async () => {
    mockCars([CAR_ON_TERRITORY]);
    const wrapper = mountCars();
    await flushPromises();
    expect(wrapper.vm.itemsData).toHaveLength(1);

    mockCars([], { carsFails: true });
    wrapper.vm.silentRefresh();
    await flushPromises();

    expect(wrapper.vm.itemsData).toHaveLength(1);
    expect(wrapper.vm.carsOnTerritory).toBe(1);
    wrapper.unmount();
  });

  it('сбой ручного «Обновить» оставляет строки, но уведомляет об устаревших данных', async () => {
    mockCars([CAR_ON_TERRITORY]);
    const wrapper = mountCars();
    await flushPromises();

    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    mockCars([], { carsFails: true });
    await wrapper.vm.loadData();
    await flushPromises();

    expect(wrapper.vm.itemsData).toHaveLength(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    wrapper.unmount();
  });

  it('выезд машины уменьшает счётчик', async () => {
    mockCars([CAR_ON_TERRITORY]);
    const wrapper = mountCars();
    await flushPromises();
    expect(wrapper.vm.carsOnTerritory).toBe(1);

    mockCars([{ ...CAR_ON_TERRITORY, territory_status: 2 }]);
    wrapper.vm.silentRefresh();
    await flushPromises();

    expect(wrapper.vm.carsOnTerritory).toBe(0);
    wrapper.unmount();
  });
});

describe('PeopleTable - счётчик зашедших не проваливается при обновлении', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
  });

  function mockPeople(rows, { statusHangs = false } = {}) {
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) {
        return Promise.resolve(okResponse({ table: { id: 42, name: 'КПП 1', table_type: 'people', fields: [] } }));
      }
      if (url.startsWith('/system-tables')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/employees/active-for-table/')) return Promise.resolve(okResponse(rows));
      if (url.startsWith('/employees/history/current-status')) {
        return statusHangs ? new Promise(() => {}) : Promise.resolve(okResponse([]));
      }
      return Promise.resolve(okResponse([]));
    });
  }

  it('territory_status из строки даёт верный счётчик до ответа current-status', async () => {
    mockPeople([EMPLOYEE_ON_TERRITORY], { statusHangs: true });
    const wrapper = mountPeople();
    await flushPromises();

    expect(wrapper.vm.peopleOnTerritory).toBe(1);
    wrapper.unmount();
  });

  it('строка без territory_status сохраняет прежнюю отметку, а не сбрасывается в 0', async () => {
    mockPeople([EMPLOYEE_ON_TERRITORY]);
    const wrapper = mountPeople();
    await flushPromises();
    expect(wrapper.vm.peopleOnTerritory).toBe(1);

    mockPeople([withoutTerritory(EMPLOYEE_ON_TERRITORY)], { statusHangs: true });
    wrapper.vm.silentRefresh();
    await flushPromises();

    expect(wrapper.vm.peopleOnTerritory).toBe(1);
    wrapper.unmount();
  });

  it('выход человека уменьшает счётчик', async () => {
    mockPeople([EMPLOYEE_ON_TERRITORY]);
    const wrapper = mountPeople();
    await flushPromises();
    expect(wrapper.vm.peopleOnTerritory).toBe(1);

    mockPeople([{ ...EMPLOYEE_ON_TERRITORY, territory_status: 2 }]);
    wrapper.vm.silentRefresh();
    await flushPromises();

    expect(wrapper.vm.peopleOnTerritory).toBe(0);
    wrapper.unmount();
  });
});
