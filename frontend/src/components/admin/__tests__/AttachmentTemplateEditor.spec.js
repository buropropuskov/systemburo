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
  listTemplateSources: vi.fn().mockResolvedValue([]),
  copyMappings: vi.fn(),
}));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));
vi.mock('../XlsxViewer.vue', () => ({ default: { name: 'XlsxViewer', template: '<div class="xlsx-stub" />' } }));

import AttachmentTemplateEditor from '../AttachmentTemplateEditor.vue';
import AttachmentMappingCopyModal from '../AttachmentMappingCopyModal.vue';

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
    group: 'application',
    label: 'Заявка',
    fields: [
      { path: 'application.sender.phone', label: 'Телефон отправителя' },
      { path: 'application.organization', label: 'Организация' },
    ],
  },
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

  it('помечает бейджем поле заявки, попавшее в строки списка', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [
        { cell_ref: 'B30', field_path: 'item.name', is_list_field: true },
        { cell_ref: 'F30', field_path: 'application.organization', is_list_field: false },
        { cell_ref: 'F5', field_path: 'application.organization', is_list_field: false },
      ],
    });
    const wrapper = await mountEditor({ attachmentType: 'items' });

    // пометка только у привязки внутри строк списка, отдельной плашки-пояснения нет
    expect(wrapper.findAll('.te-list-badge--repeat')).toHaveLength(1);
    expect(wrapper.find('[data-testid="template-repeat-note"]').exists()).toBe(false);
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
    expect(wrapper.findAll('.te-list-badge--repeat')).toHaveLength(0);
  });

  it('открывает перенос привязок и перечитывает шаблон после него', async () => {
    const wrapper = await mountEditor();
    const copyModal = wrapper.findComponent(AttachmentMappingCopyModal);
    expect(copyModal.props('show')).toBe(false);
    expect(copyModal.props('currentMappingsCount')).toBe(TEMPLATE.mappings.length);

    await wrapper.find('[data-testid="template-copy-open"]').trigger('click');
    expect(wrapper.findComponent(AttachmentMappingCopyModal).props('show')).toBe(true);

    getTemplate.mockClear();
    copyModal.vm.$emit('copied', { copied: 2 });
    await flushPromises();
    expect(getTemplate).toHaveBeenCalledWith(9);
  });

  it('предупреждает модалку о несохранённых привязках', async () => {
    const wrapper = await mountEditor();
    expect(wrapper.findComponent(AttachmentMappingCopyModal).props('unsavedChanges')).toBe(false);

    wrapper.vm.mappings.push({ cell_ref: 'C40', field_path: 'application.status', is_list_field: false });
    await wrapper.vm.$nextTick();
    expect(wrapper.findComponent(AttachmentMappingCopyModal).props('unsavedChanges')).toBe(true);
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
      listStartRow: 30, listEndRow: 50, maxListRows: 18, itemsMaxListRows: 0,
    });
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Границы списка сохранены' }));
  });

  // Начало таблицы ТМЦ берётся из привязки (в фикстуре item.name стоит в B30),
  // руками задаётся только число строк.
  it('сохраняет число строк таблицы ТМЦ вместе со списком', async () => {
    const wrapper = await mountEditor({ attachmentType: 'people' });
    expect(wrapper.vm.itemsSectionStart).toBe(30);

    await wrapper.find('[data-testid="template-items-rows"]').setValue(8);
    await wrapper.find('[data-testid="template-params-save"]').trigger('click');
    await flushPromises();

    expect(updateTemplateParams).toHaveBeenCalledWith(9, expect.objectContaining({
      itemsMaxListRows: 8,
    }));
  });

  // Без привязок к ТМЦ настройка таблицы не нужна и не показывается.
  it('прячет настройку таблицы ТМЦ, когда привязок к ТМЦ нет', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [{ cell_ref: 'B30', field_path: 'employee.last_name', is_list_field: true }],
    });
    const wrapper = await mountEditor({ attachmentType: 'people' });
    expect(wrapper.find('[data-testid="template-items-rows"]').exists()).toBe(false);
  });

  // У бланка самого ввоза строки списка и так заполняются его ТМЦ.
  it('не показывает настройку таблицы ТМЦ у бланка ввоза', async () => {
    const wrapper = await mountEditor({ attachmentType: 'items' });
    expect(wrapper.find('[data-testid="template-items-rows"]').exists()).toBe(false);
  });

  // Привязки к ТМЦ в бланке работ без заданных строк таблицы молча ничего не заполнят -
  // редактор обязан об этом сказать, иначе админ ждёт данные и видит пустые ячейки.
  it('подсказывает задать строки таблицы, когда есть привязки к ТМЦ', async () => {
    // В фикстуре шаблона уже есть привязка item.name, а строки таблицы не заданы.
    const wrapper = await mountEditor({ attachmentType: 'people' });
    expect(wrapper.find('[data-testid="template-items-range-hint"]').exists()).toBe(true);

    wrapper.vm.form.itemsMaxListRows = 8;
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="template-items-range-hint"]').exists()).toBe(false);
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
  it('показывает порядок склейки у совмещённой ячейки и меняет его', async () => {
    getTemplate.mockResolvedValue({
      ...TEMPLATE,
      mappings: [
        { cell_ref: 'A43', field_path: 'application.sender.phone', is_list_field: false },
        { cell_ref: 'A43', field_path: 'application.organization', is_list_field: false },
        { cell_ref: 'B30', field_path: 'item.name', is_list_field: true },
      ],
    });
    const wrapper = await mountEditor({ attachmentType: 'items' });

    // позиция видна только у совмещённой ячейки
    const orders = wrapper.findAll('.te-mapping-order').map(o => o.text());
    expect(orders).toEqual(['1/2', '2/2']);

    // порядок склейки = порядок привязок ячейки, он же уходит в предпросмотр
    const cellOrder = () => wrapper.vm.mappings.filter(m => m.cell_ref === 'A43').map(m => m.field_path);
    expect(cellOrder()).toEqual(['application.sender.phone', 'application.organization']);
    expect(wrapper.findAll('.te-mapping-field').map(f => f.text())).toEqual([
      'Телефон отправителя', 'Организация', 'Наименование',
    ]);

    // «позже в склейке» у первой привязки меняет её местами со второй
    await wrapper.findAll('[data-testid="mapping-move-down"]')[0].trigger('click');
    expect(cellOrder()).toEqual(['application.organization', 'application.sender.phone']);
    expect(wrapper.findAll('.te-mapping-order').map(o => o.text())).toEqual(['1/2', '2/2']);
  });

});
