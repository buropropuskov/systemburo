import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
// mounted() поднимает real-time подписку (#840); без мока реальный eventStream
// ушёл бы в fetchTicket -> reconnect с фоновым таймером на весь прогон.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';
import FactTable from '../FactTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const STUBS = { teleport: true, transition: false, 'transition-group': false, FactPassModal: true };

const CASES = [
  {
    name: 'CarsTable',
    component: CarsTable,
    testId: 'cars-table',
    props: { tableName: 'КПП 1', tableId: 42, currentUserId: 1, currentUserName: 'Тест' },
  },
  {
    name: 'PeopleTable',
    component: PeopleTable,
    testId: 'people-table',
    props: { tableName: 'КПП-72', currentUserId: 1, currentUserName: 'Тест' },
  },
  {
    name: 'FactTable',
    component: FactTable,
    testId: 'fact-table',
    props: { tableType: 'cars', tableId: 42, currentUserId: 1, currentUserName: 'Тест' },
  },
];

describe('Режим "Сетка" (#1289)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    // Минимальные ответы, чтобы загрузка данных не падала.
    apiRequest.mockImplementation(() => Promise.resolve(okResponse([])));
  });

  CASES.forEach(({ name, component, testId, props }) => {
    it(`${name}: без пропа grid класс grid-mode отсутствует`, async () => {
      const wrapper = mount(component, { props, global: { stubs: STUBS } });
      await flushPromises();
      expect(wrapper.get(`[data-testid=${testId}]`).classes()).not.toContain('grid-mode');
    });

    it(`${name}: grid=true добавляет класс grid-mode`, async () => {
      const wrapper = mount(component, { props: { ...props, grid: true }, global: { stubs: STUBS } });
      await flushPromises();
      expect(wrapper.get(`[data-testid=${testId}]`).classes()).toContain('grid-mode');
    });

    it(`${name}: смена пропа переключает класс без перемонтирования`, async () => {
      const wrapper = mount(component, { props, global: { stubs: STUBS } });
      await flushPromises();
      await wrapper.setProps({ grid: true });
      expect(wrapper.get(`[data-testid=${testId}]`).classes()).toContain('grid-mode');
      await wrapper.setProps({ grid: false });
      expect(wrapper.get(`[data-testid=${testId}]`).classes()).not.toContain('grid-mode');
    });
  });
});
