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

describe('Тумблер "Сетка" в шапке таблицы (#1289)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve(okResponse([])));
  });

  const OWNERS = [
    { name: 'CarsTable', component: CarsTable, props: CASES[0].props },
    { name: 'PeopleTable', component: PeopleTable, props: CASES[1].props },
  ];

  OWNERS.forEach(({ name, component, props }) => {
    it(`${name}: тумблер стоит рядом с "Увеличенным режимом" и эмитит update:grid`, async () => {
      const wrapper = mount(component, { props, global: { stubs: STUBS } });
      await flushPromises();

      const toggle = wrapper.get('[data-testid=grid-toggle]');
      expect(toggle.text()).toContain('Сетка');
      expect(wrapper.find('[data-testid=enlarged-toggle]').exists()).toBe(true);

      await toggle.get('input').setValue(true);
      expect(wrapper.emitted('update:grid')).toEqual([[true]]);
    });

    it(`${name}: тумблер отражает переданный проп, своего состояния не держит`, async () => {
      const wrapper = mount(component, { props: { ...props, grid: true }, global: { stubs: STUBS } });
      await flushPromises();

      expect(wrapper.get('[data-testid=grid-toggle] input').element.checked).toBe(true);

      // Состояние держит страница: тумблер только сообщает о переключении наверх.
      await wrapper.get('[data-testid=grid-toggle] input').setValue(false);
      expect(wrapper.emitted('update:grid')).toEqual([[false]]);
      expect(wrapper.vm.$data.grid).toBeUndefined();
    });
  });

  it('FactTable своего тумблера не имеет - режимом управляет страница', async () => {
    const wrapper = mount(FactTable, { props: CASES[2].props, global: { stubs: STUBS } });
    await flushPromises();
    expect(wrapper.find('[data-testid=grid-toggle]').exists()).toBe(false);
  });
});
