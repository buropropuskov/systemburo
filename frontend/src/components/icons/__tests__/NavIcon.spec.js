import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import NavIcon from '../NavIcon.vue';
import { navIcons, navIconNames } from '../navIcons.js';

describe('NavIcon', () => {
  it('рендерит svg обводкой currentColor и размером из пропа', () => {
    const wrapper = mount(NavIcon, { props: { name: 'users', size: 24 } });
    const svg = wrapper.find('svg');
    expect(svg.exists()).toBe(true);
    expect(svg.attributes('stroke')).toBe('currentColor');
    expect(svg.attributes('fill')).toBe('none');
    expect(svg.attributes('width')).toBe('24');
    expect(svg.attributes('height')).toBe('24');
    expect(svg.attributes('viewBox')).toBe('0 0 24 24');
  });

  it('размер по умолчанию - 18', () => {
    const wrapper = mount(NavIcon, { props: { name: 'center' } });
    expect(wrapper.find('svg').attributes('width')).toBe('18');
  });

  it('aria-hidden - иконка декоративная', () => {
    const wrapper = mount(NavIcon, { props: { name: 'tables' } });
    expect(wrapper.find('svg').attributes('aria-hidden')).toBe('true');
  });

  it('неизвестное имя не роняет рендер (пустой svg)', () => {
    const wrapper = mount(NavIcon, { props: { name: 'does-not-exist' } });
    expect(wrapper.find('svg').exists()).toBe(true);
    expect(wrapper.find('svg').html()).not.toContain('<path');
  });

  it('все иконки реестра рендерят непустую разметку', () => {
    for (const name of navIconNames) {
      const wrapper = mount(NavIcon, { props: { name } });
      const inner = wrapper.find('svg').element.innerHTML;
      expect(inner.length, `иконка "${name}" пустая`).toBeGreaterThan(0);
      expect(navIcons[name]).toBeTruthy();
    }
  });
});
