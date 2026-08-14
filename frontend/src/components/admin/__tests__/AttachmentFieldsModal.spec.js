import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getFieldConfig = vi.fn();
const saveFieldConfig = vi.fn();
const createCustomField = vi.fn();
const updateCustomField = vi.fn();
const deleteCustomField = vi.fn();
vi.mock('@/api/attachment-templates', () => ({
  getFieldConfig: (...a) => getFieldConfig(...a),
  saveFieldConfig: (...a) => saveFieldConfig(...a),
  createCustomField: (...a) => createCustomField(...a),
  updateCustomField: (...a) => updateCustomField(...a),
  deleteCustomField: (...a) => deleteCustomField(...a),
}));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

import AttachmentFieldsModal from '../AttachmentFieldsModal.vue';

function cfg(custom = []) {
  return {
    base: [
      {
        key: 'last_name', label: 'Фамилия', group: 'people', visible: true, required: true, requirable: true, locked: false,
      },
    ],
    custom,
  };
}

function cf(over = {}) {
  return {
    id: 1, label: 'Доп 1', placeholder: 'плейс', sort_order: 0, is_required: false, is_active: true, ...over,
  };
}

async function mountWith(config = cfg()) {
  getFieldConfig.mockResolvedValue(config);
  const wrapper = mount(AttachmentFieldsModal, {
    props: { uniqueAttachmentId: 42, attachmentName: 'Автозаявка' },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

describe('AttachmentFieldsModal - кастомные поля', () => {
  beforeEach(() => {
    getFieldConfig.mockReset();
    saveFieldConfig.mockReset().mockResolvedValue({});
    createCustomField.mockReset().mockResolvedValue({});
    updateCustomField.mockReset().mockResolvedValue({});
    deleteCustomField.mockReset().mockResolvedValue({});
    notify.mockClear();
    document.body.innerHTML = '';
  });

  it('загружает кастомные поля из field-config (один источник base+custom)', async () => {
    const wrapper = await mountWith(cfg([cf({
      id: 5, label: 'Гараж', placeholder: 'номер', is_required: true,
    })]));
    expect(getFieldConfig).toHaveBeenCalledWith(42);
    expect(wrapper.findAll('[data-testid^="custom-row-"]')).toHaveLength(1);
    expect(wrapper.find('[data-testid="custom-label-0"]').element.value).toBe('Гараж');
    expect(wrapper.find('[data-testid="custom-placeholder-0"]').element.value).toBe('номер');
    expect(wrapper.find('[data-testid="custom-required-0"] input').element.checked).toBe(true);
  });

  it('пустой список показывает "Дополнительных полей нет"', async () => {
    const wrapper = await mountWith(cfg([]));
    expect(wrapper.find('.custom-empty').exists()).toBe(true);
  });

  it('Сохранить задизейблен без изменений; добавление поля делает форму dirty', async () => {
    const wrapper = await mountWith(cfg([]));
    expect(wrapper.find('[data-testid="fields-save"]').attributes('disabled')).toBeDefined();
    await wrapper.find('[data-testid="custom-add"]').trigger('click');
    expect(wrapper.findAll('[data-testid^="custom-row-"]')).toHaveLength(1);
    expect(wrapper.find('[data-testid="fields-save"]').attributes('disabled')).toBeUndefined();
  });

  it('сохранение нового поля -> createCustomField с sortOrder и isRequired; базу не трогает', async () => {
    const wrapper = await mountWith(cfg([]));
    await wrapper.find('[data-testid="custom-add"]').trigger('click');
    await wrapper.find('[data-testid="custom-label-0"]').setValue('Гараж');
    await wrapper.find('[data-testid="custom-placeholder-0"]').setValue('номер');
    await wrapper.find('[data-testid="custom-required-0"] input').setValue(true);
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    expect(createCustomField).toHaveBeenCalledWith(42, {
      label: 'Гараж', placeholder: 'номер', sortOrder: 0, isRequired: true,
    });
    expect(saveFieldConfig).not.toHaveBeenCalled();
    expect(wrapper.emitted('saved')).toBeTruthy();
  });

  it('reorder стрелкой -> updateCustomField существующих с sortOrder из новой позиции', async () => {
    const wrapper = await mountWith(cfg([
      cf({ id: 1, label: 'A', sort_order: 0 }),
      cf({ id: 2, label: 'B', sort_order: 1 }),
    ]));
    await wrapper.find('[data-testid="custom-up-1"]').trigger('click'); // B вверх -> [B, A]
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    expect(updateCustomField).toHaveBeenCalledTimes(2);
    expect(updateCustomField).toHaveBeenNthCalledWith(1, 2, {
      label: 'B', placeholder: 'плейс', sortOrder: 0, isRequired: false,
    });
    expect(updateCustomField).toHaveBeenNthCalledWith(2, 1, {
      label: 'A', placeholder: 'плейс', sortOrder: 1, isRequired: false,
    });
  });

  it('удаление существующего поля -> deleteCustomField на сохранении', async () => {
    const wrapper = await mountWith(cfg([cf({ id: 9, label: 'X' })]));
    await wrapper.find('[data-testid="custom-delete-0"]').trigger('click');
    expect(wrapper.findAll('[data-testid^="custom-row-"]')).toHaveLength(0);
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    expect(deleteCustomField).toHaveBeenCalledWith(9);
    expect(updateCustomField).not.toHaveBeenCalled();
  });

  it('пустой заголовок блокирует сохранение с notify(error)', async () => {
    const wrapper = await mountWith(cfg([]));
    await wrapper.find('[data-testid="custom-add"]').trigger('click');
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    expect(createCustomField).not.toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    // Уведомление не говорит, какое поле пустое, поэтому его помечает подсветка.
    expect(wrapper.find('[data-testid="custom-label-0"]').classes()).toContain('ctable-input--invalid');
  });

  it('подсветка пустого заголовка снимается при вводе', async () => {
    const wrapper = await mountWith(cfg([]));
    await wrapper.find('[data-testid="custom-add"]').trigger('click');
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    const input = wrapper.find('[data-testid="custom-label-0"]');
    await input.setValue('Номер пропуска');

    expect(input.classes()).not.toContain('ctable-input--invalid');
  });

  it('стрелки порядка задизейблены на краях списка', async () => {
    const wrapper = await mountWith(cfg([cf({ id: 1 }), cf({ id: 2, sort_order: 1 })]));
    expect(wrapper.find('[data-testid="custom-up-0"]').attributes('disabled')).toBeDefined();
    expect(wrapper.find('[data-testid="custom-down-0"]').attributes('disabled')).toBeUndefined();
    expect(wrapper.find('[data-testid="custom-up-1"]').attributes('disabled')).toBeUndefined();
    expect(wrapper.find('[data-testid="custom-down-1"]').attributes('disabled')).toBeDefined();
  });

  it('частичный сбой commitCustom: retry не дублирует удаление/создание', async () => {
    const wrapper = await mountWith(cfg([cf({ id: 7, label: 'Старое' })]));
    await wrapper.find('[data-testid="custom-delete-0"]').trigger('click'); // удалить существующее
    await wrapper.find('[data-testid="custom-add"]').trigger('click'); // добавить новое
    await wrapper.find('[data-testid="custom-label-0"]').setValue('Новое');

    // 1-й save: delete прошёл, create упал -> notify(error), модалка открыта
    createCustomField.mockRejectedValueOnce(new Error('boom'));
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();
    expect(deleteCustomField).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.vm.isSaving).toBe(false);
    expect(wrapper.emitted('saved')).toBeFalsy();

    // 2-й save: create теперь ок -> delete не повторяется, create не дублируется
    createCustomField.mockResolvedValueOnce({ id: 99 });
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();
    expect(deleteCustomField).toHaveBeenCalledTimes(1);
    expect(createCustomField).toHaveBeenCalledTimes(2);
    expect(wrapper.emitted('saved')).toBeTruthy();
  });

  it('изменение только базового поля -> saveFieldConfig, кастомные не трогаются', async () => {
    const wrapper = await mountWith(cfg([cf({ id: 1, label: 'A' })]));
    await wrapper.find('[data-testid="field-visible-last_name"] input').setValue(false);
    await wrapper.find('[data-testid="fields-save"]').trigger('click');
    await flushPromises();

    expect(saveFieldConfig).toHaveBeenCalledTimes(1);
    expect(createCustomField).not.toHaveBeenCalled();
    expect(updateCustomField).not.toHaveBeenCalled();
    expect(deleteCustomField).not.toHaveBeenCalled();
  });
});
