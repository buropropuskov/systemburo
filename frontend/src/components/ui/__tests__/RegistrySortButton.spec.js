import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import RegistrySortButton from '../RegistrySortButton.vue';
describe('Registry sorting button', () => {
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
