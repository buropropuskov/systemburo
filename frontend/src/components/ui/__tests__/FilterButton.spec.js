import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import FilterButton from '@/components/ui/FilterButton.vue';

describe('FilterButton', () => {
  it('рендерит подпись «Фильтр» по умолчанию', () => {
    const w = mount(FilterButton);
    expect(w.text()).toContain('Фильтр');
  });

  it('без active нет точки-индикатора и класса --active', () => {
    const w = mount(FilterButton, { props: { active: false } });
    expect(w.find('.filter-btn__dot').exists()).toBe(false);
    expect(w.classes()).not.toContain('filter-btn--active');
  });

  it('active: показывает точку и класс --active', () => {
    const w = mount(FilterButton, { props: { active: true } });
    expect(w.find('.filter-btn__dot').exists()).toBe(true);
    expect(w.classes()).toContain('filter-btn--active');
  });

  it('клик эмитит click', async () => {
    const w = mount(FilterButton);
    await w.trigger('click');
    expect(w.emitted('click')).toBeTruthy();
  });

  it('кастомная подпись через prop label', () => {
    const w = mount(FilterButton, { props: { label: 'Настроить' } });
    expect(w.text()).toContain('Настроить');
  });
});
