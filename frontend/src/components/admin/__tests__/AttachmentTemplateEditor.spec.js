import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getTemplate = vi.fn();
const listTemplates = vi.fn();
const getTemplateFields = vi.fn();
const getTemplateFile = vi.fn();
const getTemplateFileByID = vi.fn();
const updateTemplateParams = vi.fn();
const updateMappings = vi.fn();
vi.mock('@/api/attachment-templates', () => ({
  getTemplate: (...a) => getTemplate(...a),
  listTemplates: (...a) => listTemplates(...a),
  getTemplateFields: (...a) => getTemplateFields(...a),
  getTemplateFile: (...a) => getTemplateFile(...a),
  getTemplateFileByID: (...a) => getTemplateFileByID(...a),
  updateTemplateParams: (...a) => updateTemplateParams(...a),
  updateMappings: (...a) => updateMappings(...a),
  uploadTemplate: vi.fn(),
  deleteTemplate: vi.fn(),
  deleteTemplateByID: vi.fn(),
  setActiveTemplate: vi.fn(),
  deactivateAllTemplates: vi.fn(),
  saveBlobAs: vi.fn(),
}));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));
vi.mock('../XlsxViewer.vue', () => ({ default: { name: 'XlsxViewer', template: '<div class="xlsx-stub" />' } }));

import AttachmentTemplateEditor from '../AttachmentTemplateEditor.vue';

const TEMPLATE = {
  id: 3,
  file_path: '/app/uploads/templates/1_177.xlsx',
  original_file_name: 'Заявка на ввоз.xlsx',
  list_start_row: 30,
  list_end_row: 47,
  max_list_rows: 18,
  concat_separator: ', ',
  mappings: [
    { cell_ref: 'I21', field_path: 'car.car_number', is_list_field: true },
    { cell_ref: 'B30', field_path: 'item.name', is_list_field: true },
  ],
};

const FIELD_GROUPS = [
  {
    group: 'car',
    label: 'Автомобиль (список)',
    fields: [{ path: 'car.car_number', label: 'Номер ТС', is_list: true }],
  },
  {
    group: 'item',
    label: 'Имущество (список)',
    fields: [{ path: 'item.name', label: 'Наименование', is_list: true }],
  },
];

// Данные редактор грузит по watch на show, поэтому монтируем закрытым и открываем -
// как это делает AttachmentsManagement.
async function mountEditor(props = {}) {
  const wrapper = mount(AttachmentTemplateEditor, {
    props: {
      show: false, uniqueAttachmentId: 9, attachmentType: 'items', ...props,
    },
    global: { stubs: { Teleport: true, transition: false } },
  });
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

describe('AttachmentTemplateEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getTemplate.mockResolvedValue({ ...TEMPLATE, mappings: [...TEMPLATE.mappings] });
    listTemplates.mockResolvedValue([TEMPLATE]);
    getTemplateFields.mockResolvedValue(FIELD_GROUPS);
    getTemplateFile.mockResolvedValue(new ArrayBuffer(8));
    getTemplateFileByID.mockResolvedValue(new ArrayBuffer(8));
    updateTemplateParams.mockResolvedValue({});
  });

  it('предупреждает о привязке списка из чужой группы', async () => {
    const wrapper = await mountEditor({ attachmentType: 'items' });
    // car.* в бланке имущества заполнить нечем - об этом должно быть сказано
    expect(wrapper.text()).toContain('другой группы полей');
    expect(wrapper.text()).toContain('Номер ТС');
  });

  it('не предупреждает, когда привязки соответствуют типу вложения', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [{ cell_ref: 'B30', field_path: 'item.name', is_list_field: true }],
    });
    const wrapper = await mountEditor({ attachmentType: 'items' });
    expect(wrapper.text()).not.toContain('другой группы полей');
  });

  it('помечает поле заявки, попавшее в строки списка', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [
        { cell_ref: 'B30', field_path: 'item.name', is_list_field: true },
        { cell_ref: 'F30', field_path: 'application.organization', is_list_field: false },
        { cell_ref: 'F5', field_path: 'application.organization', is_list_field: false },
      ],
    });
    const wrapper = await mountEditor({ attachmentType: 'items' });

    const note = wrapper.find('[data-testid="template-repeat-note"]');
    expect(note.exists()).toBe(true);
    // в списке только F30, шапочная F5 туда попадать не должна
    expect(note.text()).toContain('F30');
    expect(note.text()).not.toContain('F5');
    expect(wrapper.findAll('.te-list-badge--repeat')).toHaveLength(1);
  });

  it('не помечает поля заявки вне строк списка', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [
        { cell_ref: 'B30', field_path: 'item.name', is_list_field: true },
        { cell_ref: 'F5', field_path: 'application.organization', is_list_field: false },
      ],
    });
    const wrapper = await mountEditor({ attachmentType: 'items' });
    expect(wrapper.find('[data-testid="template-repeat-note"]').exists()).toBe(false);
    expect(wrapper.findAll('.te-list-badge--repeat')).toHaveLength(0);
  });

  it('сохраняет границы списка без перезагрузки файла', async () => {
    const wrapper = await mountEditor();
    const save = wrapper.find('[data-testid="template-params-save"]');
    expect(save.exists()).toBe(true);
    // без изменений кнопка неактивна
    expect(save.attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="template-list-end"]').setValue(50);
    expect(wrapper.find('[data-testid="template-params-save"]').attributes('disabled')).toBeUndefined();

    await wrapper.find('[data-testid="template-params-save"]').trigger('click');
    await flushPromises();

    expect(updateTemplateParams).toHaveBeenCalledWith(9, {
      listStartRow: 30, listEndRow: 50, maxListRows: 18,
    });
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Границы списка сохранены' }));
  });

  it('показывает ошибку сервера при сохранении границ', async () => {
    updateTemplateParams.mockRejectedValue(new Error('Некорректный диапазон строк'));
    const wrapper = await mountEditor();
    await wrapper.find('[data-testid="template-list-start"]').setValue(60);
    await wrapper.find('[data-testid="template-params-save"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error', bold: 'Некорректный диапазон строк',
    }));
  });
});
