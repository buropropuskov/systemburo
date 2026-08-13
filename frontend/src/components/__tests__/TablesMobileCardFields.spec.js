import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
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
import FactTable from '../FactTable.vue';

/**
 * Состав карточки строки на телефоне (волна 6).
 *
 * Подбор столбцов по ширине (#1307) для карточки не годится: поля стоят своими
 * строками и за ширину не конкурируют, а мерить он пытается строку заголовков,
 * скрытую card-правилами. Ноль на входе - и подбор скатывался в `keepAtLeast`, то
 * есть оставлял в карточке ровно два самых левых столбца. Владелец увидел это как
 * «карточка машины полу пустая» и «в карточке человека должно быть ФИО в одну
 * строку, организация, срок действия».
 *
 * Поэтому на узком экране состав карточки задан списком, а остальное уходит в
 * «Подробнее». В таблице «по факту» панели «Подробнее» нет вовсе, поэтому там
 * прятать нельзя ничего - иначе значение не видно нигде.
 *
 * jsdom раскладки не считает, но выбор полей - обычная логика компонента, и она
 * проверяется здесь; сама раскладка карточки стережётся TablesCardPattern.spec.js.
 */

const STUBS = { teleport: true, transition: false, 'transition-group': false, FactPassModal: true };

/** Телефон: тот же порог, что у card-правил responsive-tables.css. */
function fakeNarrowScreen(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const realMatchMedia = window.matchMedia;

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
  apiRequest.mockReset();
  apiRequest.mockImplementation(() => Promise.resolve({ ok: true, json: async () => [] }));
});

afterEach(() => {
  window.matchMedia = realMatchMedia;
});

const CARS_FIELDS = {
  car_number: 0,
  car_brand: 1,
  organization: 2,
  company: 3,
  application_id: 4,
  unload_place: 5,
  valid_until: 6,
  time_range: 7,
  status: 8,
};

const PEOPLE_FIELDS = {
  last_name: 0,
  first_name: 1,
  middle_name: 2,
  position: 3,
  citizenship_name: 4,
  organization: 5,
  company: 6,
  valid_until: 7,
  pass_time: 8,
  application_id: 9,
};

function visibility(orders) {
  return Object.fromEntries(Object.keys(orders).map((name) => [name, true]));
}

async function mountWithFields(component, props, orders) {
  const wrapper = mount(component, { props, global: { stubs: STUBS } });
  await flushPromises();
  await wrapper.setData({ fieldsVisibility: visibility(orders), fieldOrders: orders });
  return wrapper;
}

const CARS_PROPS = { tableName: 'КПП 1', tableId: 42, currentUserId: 1, currentUserName: 'Тест' };
const PEOPLE_PROPS = { tableName: 'КПП-72', currentUserId: 1, currentUserName: 'Тест' };

describe('Карточка машины на телефоне', () => {
  beforeEach(() => fakeNarrowScreen(true));

  it('показывает номер, марку, организацию, место, срок, время и статус', async () => {
    const wrapper = await mountWithFields(CarsTable, CARS_PROPS, CARS_FIELDS);

    ['car_number', 'car_brand', 'organization', 'unload_place', 'valid_until', 'time_range', 'status']
      .forEach((name) => {
        expect(wrapper.vm.fieldColClass(name)).toBe('');
        expect(wrapper.vm.isFieldVisible(name)).toBe(true);
      });
  });

  it('компанию и номер заявки уводит в «Подробнее», а не теряет', async () => {
    const wrapper = await mountWithFields(CarsTable, CARS_PROPS, CARS_FIELDS);

    ['company', 'application_id'].forEach((name) => {
      expect(wrapper.vm.fieldColClass(name)).toBe('col--collapsed');
    });
    expect(wrapper.vm.hiddenInPortraitFields()).toEqual(['company', 'application_id']);
  });

  it('подбор столбцов по ширине в карточке выключен', async () => {
    const wrapper = await mountWithFields(CarsTable, CARS_PROPS, CARS_FIELDS);
    await wrapper.setData({ overflowFields: ['car_number', 'organization'] });

    wrapper.vm.recalcOverflowFields();
    expect(wrapper.vm.overflowFields).toEqual([]);
  });

  it('на широком экране состав по-прежнему считает подбор', async () => {
    fakeNarrowScreen(false);
    const wrapper = await mountWithFields(CarsTable, CARS_PROPS, CARS_FIELDS);
    await wrapper.setData({ overflowFields: ['company'] });

    // Список карточки на десктоп не распространяется: там столбцы делят одну строку.
    expect(wrapper.vm.fieldColClass('company')).toBe('col--collapsed');
    expect(wrapper.vm.fieldColClass('application_id')).toBe('');
  });
});

describe('Карточка человека на телефоне', () => {
  beforeEach(() => fakeNarrowScreen(true));

  it('показывает ФИО целиком, организацию, срок действия и статус', async () => {
    const wrapper = await mountWithFields(PeopleTable, PEOPLE_PROPS, PEOPLE_FIELDS);

    // Отчество - часть ФИО: подбор по ширине оставлял в карточке фамилию с именем,
    // и «Иванович» пропадало вместе с организацией и сроком.
    ['last_name', 'first_name', 'middle_name', 'organization', 'valid_until', 'status']
      .forEach((name) => expect(wrapper.vm.fieldColClass(name)).toBe(''));
  });

  it('должность, гражданство, компанию, время и номер заявки уводит в «Подробнее»', async () => {
    const wrapper = await mountWithFields(PeopleTable, PEOPLE_PROPS, PEOPLE_FIELDS);

    expect(wrapper.vm.hiddenInPortraitFields())
      .toEqual(['position', 'citizenship_name', 'company', 'pass_time', 'application_id']);
  });

  it('подбор столбцов по ширине в карточке выключен', async () => {
    const wrapper = await mountWithFields(PeopleTable, PEOPLE_PROPS, PEOPLE_FIELDS);
    await wrapper.setData({ overflowFields: ['middle_name'] });

    wrapper.vm.recalcOverflowFields();
    expect(wrapper.vm.overflowFields).toEqual([]);
  });
});

describe('Таблица «по факту» на телефоне', () => {
  beforeEach(() => fakeNarrowScreen(true));

  it('не прячет ни одного столбца - показать их больше негде', async () => {
    const wrapper = mount(FactTable, {
      props: { tableType: 'cars', tableId: 42, currentUserId: 7, currentUserName: 'Охр' },
      global: { stubs: STUBS },
    });
    await flushPromises();
    await wrapper.setData({
      fieldsVisibility: visibility(CARS_FIELDS),
      fieldOrders: CARS_FIELDS,
      overflowFields: ['company', 'application_id'],
    });

    wrapper.vm.recalcOverflowFields();
    expect(wrapper.vm.overflowFields).toEqual([]);
    Object.keys(CARS_FIELDS).forEach((name) => {
      expect(wrapper.vm.isFieldVisible(name)).toBe(true);
    });
  });
});
