import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listTemplateSources = vi.fn();
const copyMappings = vi.fn();
vi.mock('@/api/attachment-templates', () => ({
  listTemplateSources: (...a) => listTemplateSources(...a),
  copyMappings: (...a) => copyMappings(...a),
}));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

import AttachmentMappingCopyModal from '../AttachmentMappingCopyModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const SOURCES = [
  {
    template_id: 7,
    unique_attachment_id: 1,
    attachment_name: 'Автозаявка',
    attachment_type: 'cars',
    original_file_name: 'Автозаявка.xlsx',
    mappings_count: 13,
    is_active: true,
  },
  {
    template_id: 8,
    unique_attachment_id: 5,
    attachment_name: 'Работы',
    attachment_type: 'people',
    original_file_name: 'Работы.xlsx',
    mappings_count: 11,
    is_active: true,
  },
  // второй файл ТОГО ЖЕ вложения: перенос между файлами - основной случай
  {
    template_id: 9,
    unique_attachment_id: 9,
    attachment_name: 'Автозаявка',
    attachment_type: 'cars',
    original_file_name: 'Автозаявка старая.xlsx',
    mappings_count: 6,
    is_active: false,
  },
  {
    template_id: 10,
    unique_attachment_id: 11,
    attachment_name: 'Пустой',
    attachment_type: 'cars',
    original_file_name: 'Пустой.xlsx',
    mappings_count: 0,
    is_active: true,
  },
];

async function mountModal(props = {}) {
  const wrapper = mount(AttachmentMappingCopyModal, {
    props: {
      show: false,
      uniqueAttachmentId: 9,
      attachmentType: 'cars',
      currentMappingsCount: 2,
      currentTemplateId: 3,
      targetFileName: 'Автозаявка.xlsx',
      ...props,
    },
    global: { stubs: { Teleport: true, transition: false } },
  });
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

// Выбор источника идёт через BaseDropdown - эмитим его событие, а не пишем во vm.
async function selectSource(wrapper, templateId) {
  wrapper.findComponent(BaseDropdown).vm.$emit('update:modelValue', templateId);
  await flushPromises();
}

describe('AttachmentMappingCopyModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listTemplateSources.mockResolvedValue([...SOURCES]);
    copyMappings.mockResolvedValue({
      copied: 4, skipped_foreign_list: 1, skipped_custom: 1, remapped_custom: 1, skipped_duplicates: 0, params_copied: true,
    });
  });

  it('другие файлы этого же вложения идут первыми, пустые не берём', async () => {
    const wrapper = await mountModal();
    const options = wrapper.findComponent(BaseDropdown).props('options');
    // 9 - второй файл этого вложения, дальше чужие типы; 10 без привязок отброшен
    expect(options.map(o => o.template_id)).toEqual([9, 7, 8]);
    expect(options[0].label).toBe('Автозаявка старая.xlsx - 6 прив.');
    expect(options[1].label).toContain('Автозаявка (автомобили)');
  });

  it('активный шаблон источником не предлагает', async () => {
    const wrapper = await mountModal({ currentTemplateId: 9 });
    const options = wrapper.findComponent(BaseDropdown).props('options');
    expect(options.map(o => o.template_id)).toEqual([7, 8]);
  });

  it('показывает, в какой файл переносим', async () => {
    const wrapper = await mountModal();
    expect(wrapper.find('.mc-target').text()).toContain('Автозаявка.xlsx');
  });

  it('переносит привязки выбранного шаблона и сообщает о пропусках', async () => {
    const wrapper = await mountModal();
    await selectSource(wrapper, 7);
    await wrapper.find('[data-testid="copy-params"]').setValue(true);
    await wrapper.find('[data-testid="copy-submit"]').trigger('click');
    await flushPromises();

    expect(copyMappings).toHaveBeenCalledWith(9, {
      sourceTemplateID: 7, replace: true, copyParams: true,
    });
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: '4',
      suffix: expect.stringContaining('списка чужого типа: 1'),
    }));
    expect(wrapper.emitted('copied')).toBeTruthy();
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('предупреждает о другом типе вложения у источника', async () => {
    const wrapper = await mountModal({ attachmentType: 'cars' });
    expect(wrapper.find('[data-testid="copy-type-warning"]').exists()).toBe(false);
    await selectSource(wrapper, 8);
    expect(wrapper.find('[data-testid="copy-type-warning"]').text()).toContain('сотрудники');
  });

  it('предупреждает о несохранённых привязках', async () => {
    const wrapper = await mountModal({ unsavedChanges: true });
    expect(wrapper.find('[data-testid="copy-unsaved-warning"]').exists()).toBe(true);
  });

  it('показывает ошибку сервера и не эмитит перенос', async () => {
    copyMappings.mockRejectedValue(new Error('Шаблон-источник не найден'));
    const wrapper = await mountModal();
    await selectSource(wrapper, 7);
    await wrapper.find('[data-testid="copy-submit"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error', bold: 'Шаблон-источник не найден',
    }));
    expect(wrapper.emitted('copied')).toBeFalsy();
  });

  it('сообщает об ошибке списка источников', async () => {
    listTemplateSources.mockRejectedValue(new Error('нет доступа'));
    const wrapper = await mountModal();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.findComponent(BaseDropdown).props('options')).toEqual([]);
  });
});
