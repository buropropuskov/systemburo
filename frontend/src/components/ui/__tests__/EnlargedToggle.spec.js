import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import EnlargedToggle from '../EnlargedToggle.vue';

describe('EnlargedToggle', () => {
  it('отражает modelValue=false в неактивном состоянии', () => {
    const wrapper = mount(EnlargedToggle, {
      props: { modelValue: false },
    });
    expect(wrapper.classes()).not.toContain('enlarged-toggle--active');
    expect(wrapper.get('input[type=checkbox]').element.checked).toBe(false);
  });

  it('отражает modelValue=true в активном состоянии', () => {
    const wrapper = mount(EnlargedToggle, {
      props: { modelValue: true },
    });
    expect(wrapper.classes()).toContain('enlarged-toggle--active');
    expect(wrapper.get('input[type=checkbox]').element.checked).toBe(true);
  });

  it('эмитит update:modelValue=true при включении', async () => {
    const wrapper = mount(EnlargedToggle, {
      props: { modelValue: false },
    });
    await wrapper.get('input[type=checkbox]').setValue(true);
    expect(wrapper.emitted('update:modelValue')).toEqual([[true]]);
  });

  it('эмитит update:modelValue=false при выключении', async () => {
    const wrapper = mount(EnlargedToggle, {
      props: { modelValue: true },
    });
    await wrapper.get('input[type=checkbox]').setValue(false);
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]]);
  });

  it('показывает кастомный label через prop', () => {
    const wrapper = mount(EnlargedToggle, {
      props: { modelValue: false, label: 'Большой шрифт' },
    });
    expect(wrapper.text()).toContain('Большой шрифт');
  });
});
