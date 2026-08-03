import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import StatusBadge from '../StatusBadge.vue';

describe('StatusBadge', () => {
  describe('status to color mapping', () => {
    it.each([
      ['В работе', 'status-badge--blue'],
      ['Согласовано', 'status-badge--green'],
      ['Не согласовано', 'status-badge--red'],
      ['В обработке', 'status-badge--yellow'],
      ['Активен', 'status-badge--green'],
      ['Неактивен', 'status-badge--gray'],
      ['Отклонено', 'status-badge--red'],
      ['Чёрный список', 'status-badge--red'],
      ['Непрочитано', 'status-badge--yellow'],
      ['В архиве', 'status-badge--green'],
      ['В очереди', 'status-badge--yellow'],
      ['Ошибка', 'status-badge--red'],
    ])('"%s" renders with class "%s"', (status, expectedClass) => {
      const wrapper = mount(StatusBadge, { props: { status } });
      expect(wrapper.classes()).toContain(expectedClass);
    });
  });

  it('renders default gray for unknown status', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'Неизвестный' } });
    expect(wrapper.classes()).toContain('status-badge--gray');
  });

  it('renders status text', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'В работе' } });
    expect(wrapper.text()).toContain('В работе');
  });

  describe('variants', () => {
    it('applies badge variant class by default', () => {
      const wrapper = mount(StatusBadge, { props: { status: 'Активен' } });
      expect(wrapper.classes()).toContain('status-badge--badge');
    });

    it('applies dot variant class', () => {
      const wrapper = mount(StatusBadge, {
        props: { status: 'Активен', variant: 'dot' },
      });
      expect(wrapper.classes()).toContain('status-badge--dot');
    });

    it('renders dot element for dot variant', () => {
      const wrapper = mount(StatusBadge, {
        props: { status: 'Активен', variant: 'dot' },
      });
      expect(wrapper.find('.status-badge__dot').exists()).toBe(true);
    });

    it('does not render dot element for badge variant', () => {
      const wrapper = mount(StatusBadge, {
        props: { status: 'Активен', variant: 'badge' },
      });
      expect(wrapper.find('.status-badge__dot').exists()).toBe(false);
    });
  });
});
