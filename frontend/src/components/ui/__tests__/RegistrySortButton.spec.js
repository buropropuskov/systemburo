import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import RegistrySortButton from '../RegistrySortButton.vue';
describe('Registry sorting button', () => {
  it('keeps a configured table column without introducing a sort icon', async () => {
    const w = mount(RegistrySortButton, { props: { field: 'organization', label: 'Организация', sortField: 'organization', sortDirection: 'desc', headerClass: 'col', icon: false } });
    expect(w.classes()).toContain('col');
    expect(w.classes()).not.toContain('header-col');
    expect(w.find('.sort-icon').exists()).toBe(false);
    expect(w.get('.sort-label').text()).toBe('Организация');
    expect(w.attributes('aria-label')).toContain('по убыванию');
    await w.trigger('click');
    expect(w.emitted('sort')).toEqual([['organization']]);
    w.unmount();
  });
  it('accepts the initial unsorted state without a Vue warning', () => {
    const warn = vi.spyOn(console, 'warn');
    const w = mount(RegistrySortButton, { props: { field: 'name', label: 'Наименование', sortField: null, sortDirection: 'asc' } });
    expect(w.attributes('aria-label')).toContain('не выбрана');
    expect(warn).not.toHaveBeenCalled();
    w.unmount();
    warn.mockRestore();
  });
  it('uses a non-submit native button and reports the selected direction', async () => {
    const w = mount(RegistrySortButton, { props: { field: 'name', label: 'Наименование', sortField: 'id', sortDirection: 'asc' } });
    expect(w.element.tagName).toBe('BUTTON');
    expect(w.element.type).toBe('button');
    expect(w.attributes('aria-label')).toContain('не выбрана');
    await w.trigger('click');
    expect(w.emitted('sort')).toEqual([['name']]);
    await w.setProps({ sortField: 'name' });
    expect(w.attributes('aria-label')).toContain('по возрастанию');
    await w.setProps({ sortDirection: 'desc' });
    expect(w.attributes('aria-label')).toContain('по убыванию');
    expect(w.get('.sort-icon').classes()).toContain('desc');
    w.unmount();
  });
});
