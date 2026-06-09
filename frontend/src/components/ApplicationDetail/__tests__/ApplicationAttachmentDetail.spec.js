import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';

function car(over = {}) {
  return {
    id: 1,
    car_number: 'А123ВС',
    car_brand: 'Toyota',
    unload_places: [],
    ...over,
  };
}

function employee(over = {}) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    target_tables: [],
    ...over,
  };
}

function flag(over = {}) {
  return {
    flag_id: 10,
    matched_value: 'А124ВС Toyota',
    matched_reason: 'похожий номер',
    similarity: 0.85,
    overridden: false,
    ...over,
  };
}

function mountCars(cars) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
      cars,
    },
  });
}

function mountEmployees(employees) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'people', attachment_display_name: 'Люди' },
      employees,
    },
  });
}

describe('ApplicationAttachmentDetail — подсветка возможного обхода ЧС (#481)', () => {
  it('помеченная машина: красноватый модификатор + бейдж "похоже на ЧС"', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag() })]);
    const item = wrapper.find('.car-item');
    expect(item.classes()).toContain('car-item--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('похоже на ЧС');
    expect(badge.classes()).toContain('badge--danger');
  });

  it('чистая машина: нет модификатора и нет бейджа', () => {
    const wrapper = mountCars([car()]);
    const item = wrapper.find('.car-item');
    expect(item.classes()).not.toContain('car-item--flagged');
    expect(item.find('.blacklist-badge').exists()).toBe(false);
  });

  it('подтверждённый пропуск (overridden): без красной подсветки, нейтральный бейдж', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag({ overridden: true }) })]);
    const item = wrapper.find('.car-item');
    expect(item.classes()).not.toContain('car-item--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('пропуск подтверждён');
    expect(badge.classes()).toContain('badge--neutral');
  });

  it('tooltip содержит совпавшее значение и причину', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag() })]);
    const badge = wrapper.find('.car-item .blacklist-badge');
    expect(badge.attributes('title')).toBe('Возможный обход чёрного списка. Похоже на: А124ВС Toyota (похожий номер)');
  });

  it('помеченный сотрудник: красноватый модификатор + бейдж', () => {
    const wrapper = mountEmployees([
      employee({ blacklist_similar: flag({ matched_value: 'Иваноф Иван Иванович', matched_reason: 'опечатка в фамилии' }) }),
    ]);
    const item = wrapper.find('.employee-item');
    expect(item.classes()).toContain('employee-item--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('похоже на ЧС');
    expect(badge.attributes('title')).toContain('Иваноф Иван Иванович');
  });
});
