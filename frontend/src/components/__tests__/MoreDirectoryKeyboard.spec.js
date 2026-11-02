import { beforeEach, afterEach, describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })) }));
vi.mock('@/utils/dirtyTracker', () => ({ registerDirtyTracker: () => () => {}, confirmIfAnyDirty: vi.fn(async () => true) }));
import Citizenship from '../CitizenshipManagement.vue';
import Marks from '../MarksManagement.vue';
import Formats from '../NumberFormat.vue';
import Attachments from '../AttachmentsManagement.vue';
const wrappers = [];
const record = { id: 1, name: 'Альфа', display_name: 'Альфа', title: 'АЛЬФА', is_active: true, attachment_type: 'cars', instruction: '' };
const cases = [
  { name: 'citizenship', Component: Citizenship, state: 'items', selected: 'selectedCitizenship', field: 'name' },
  { name: 'marks', Component: Marks, state: 'marks', selected: 'selectedMark', field: 'name' },
  { name: 'formats', Component: Formats, state: 'formats', selected: 'selectedFormat', field: 'name', nested: true },
  { name: 'attachments', Component: Attachments, state: 'items', selected: 'selectedAttachment', field: 'display_name' },
];
beforeEach(() => setActivePinia(createPinia()));
afterEach(() => { wrappers.forEach(w => w.unmount()); wrappers.length = 0; vi.restoreAllMocks(); });
describe.each(cases)('Keyboard actions in $name', ({ Component, state, selected, field, nested }) => {
  async function setup() {
    const w = mount(Component, { global: { stubs: {
      TextConstructor: true, AttachmentTemplateEditor: true, AttachmentFieldsModal: true,
    } } });
    wrappers.push(w);
    await flushPromises();
    const rows = [record, { ...record, id: 2, name: 'Бета', display_name: 'Бета' }];
    await w.setData({ [state]: nested ? rows.map(format => ({ format, cells: [] })) : rows, isLoading: false });
    return w;
  }
  it('opens with a native name button and keeps checkbox selection independent', async () => {
    const w = await setup();
    const row = w.get('.table-row');
    const open = row.get('.directory-record-open');
    expect(open.element.tagName).toBe('BUTTON');
    expect(open.attributes('type')).toBe('button');
    expect(open.find('button,input').exists()).toBe(false);
    const checkbox = row.find('input[type=checkbox]');
    if (checkbox.exists()) {
      await checkbox.setValue(true);
      expect(w.vm[selected]).toBeNull();
    }
    await open.trigger('click');
    await flushPromises();
    const value = w.vm[selected];
    expect(nested ? value.format.id : value.id).toBe(1);
    if (checkbox.exists()) expect(checkbox.element.checked).toBe(true);
  });
  it('sorts through the native header and exposes the current direction', async () => {
    const w = await setup();
    const sort = w.get(`button[data-field="${field}"]`);
    expect(sort.attributes('type')).toBe('button');
    await sort.trigger('click');
    expect(w.vm.sortField).toBe(field);
    const direction = w.vm.sortDirection;
    await sort.trigger('click');
    expect(w.vm.sortDirection).not.toBe(direction);
    expect(sort.attributes('aria-label')).toContain(w.vm.sortDirection === 'asc' ? 'по возрастанию' : 'по убыванию');
  });
});
