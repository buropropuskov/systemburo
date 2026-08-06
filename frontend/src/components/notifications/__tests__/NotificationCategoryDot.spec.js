import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import NotificationCategoryDot from '../NotificationCategoryDot.vue';

describe('NotificationCategoryDot', () => {
  it('категория passage по точному коду (не по префиксу application_)', () => {
    const wrapper = mount(NotificationCategoryDot, { props: { type: 'application_expiring' } });
    expect(wrapper.classes()).toContain('notif-category-dot--passage');
    expect(wrapper.attributes('title')).toBe('Проезд');
  });

  it('настоящее событие заявки -> application', () => {
    const wrapper = mount(NotificationCategoryDot, { props: { type: 'application_created' } });
    expect(wrapper.classes()).toContain('notif-category-dot--application');
    expect(wrapper.attributes('title')).toBe('Заявка');
  });

  it('неизвестный/пустой тип -> application по умолчанию', () => {
    expect(mount(NotificationCategoryDot, { props: { type: null } }).classes()).toContain('notif-category-dot--application');
  });
});
