import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import BlacklistTabBase from '../BlacklistTabBase.vue';
import { useDeletionsStore } from '@/stores/deletions';

function seedItems() {
  return [
    { id: 1, is_active: true, reason: 'r1' },
    { id: 2, is_active: true, reason: 'r2' },
    { id: 3, is_active: true, reason: 'r3' },
  ];
}

function mountBase({ items = seedItems(), bulkArchiveFn = vi.fn(), bulkRestoreFn = vi.fn(), ...props } = {}) {
  return mount(BlacklistTabBase, {
    props: {
      apiList: vi.fn().mockResolvedValue(items),
      getPrimaryText: (i) => `Запись ${i.id}`,
      getDetailRows: () => [],
      bulkArchiveFn,
      bulkRestoreFn,
      testidPrefix: 'test-bl',
      ...props,
    },
    global: {
      stubs: { BaseDropdown: true, SearchComponent: true, RefreshButton: true, LoaderSpinner: true },
    },
  });
}

const rowChecks = (w) => w.findAll('[data-testid="test-bl-row-check"]');
const bulkBar = (w) => w.find('[data-testid="test-bl-bulk-bar"]');

describe('BlacklistTabBase - групповой выбор и bulk архив/восстановление (#443)', () => {
  let wrapper;
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });
  afterEach(() => wrapper?.unmount());

  it('без bulkArchiveFn/bulkRestoreFn чекбоксы и bulk-bar не рендерятся', async () => {
    wrapper = mountBase({ bulkArchiveFn: null, bulkRestoreFn: null });
    await flushPromises();
    expect(rowChecks(wrapper)).toHaveLength(0);
    expect(wrapper.find('[data-testid="test-bl-select-all"]').exists()).toBe(false);
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(bulkBar(wrapper).exists()).toBe(false);
  });

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountBase();
    await flushPromises();
    expect(bulkBar(wrapper).exists()).toBe(false);

    await rowChecks(wrapper)[0].trigger('click');
    expect(bulkBar(wrapper).exists()).toBe(true);
    expect(bulkBar(wrapper).find('.bl-bulk-count').text()).toBe('Выбрано: 1');
    expect(wrapper.vm.selectedIds).toEqual([1]);
  });

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountBase();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true });
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3]);
  });

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountBase();
    await flushPromises();
    await wrapper.find('[data-testid="test-bl-select-all"]').trigger('change');
    expect(wrapper.vm.selectedIds).toHaveLength(3);
    await wrapper.find('[data-testid="test-bl-select-all"]').trigger('change');
    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    const bulkArchiveFn = vi.fn().mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    wrapper = mountBase({ bulkArchiveFn });
    await flushPromises();

    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[1].trigger('click');
    await wrapper.find('[data-testid="test-bl-bulk-archive"]').trigger('click');
    expect(wrapper.vm.bulkConfirmVisible).toBe(true);

    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(bulkArchiveFn).toHaveBeenCalledWith([1, 2]);
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkConfirmVisible).toBe(false);
  });

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    const bulkArchiveFn = vi.fn().mockResolvedValue({
      success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Запись 2', error: 'не найдена' }],
    });
    wrapper = mountBase({ bulkArchiveFn });
    await flushPromises();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[1].trigger('click');
    wrapper.vm.startBulkOperation('archive');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('Запись 2') }));
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('ошибка-envelope ({message}) -> error-notify, выбор НЕ сброшен, модалка держится', async () => {
    const bulkArchiveFn = vi.fn().mockResolvedValue({ message: 'Не выбраны записи' });
    wrapper = mountBase({ bulkArchiveFn });
    await flushPromises();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await rowChecks(wrapper)[0].trigger('click');
    wrapper.vm.startBulkOperation('archive');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.vm.selectedIds).toEqual([1]);
    expect(wrapper.vm.bulkConfirmVisible).toBe(true);
  });

  it('в архивном режиме кнопка "Вернуть в ЧС" зовёт restore-fn', async () => {
    const bulkRestoreFn = vi.fn().mockResolvedValue({ success_count: 1, error_count: 0, errors: [] });
    wrapper = mountBase({ items: [{ id: 5, is_active: false, reason: 'r' }], bulkRestoreFn });
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();

    await rowChecks(wrapper)[0].trigger('click');
    expect(wrapper.find('[data-testid="test-bl-bulk-restore"]').exists()).toBe(true);
    wrapper.vm.startBulkOperation('restore');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(bulkRestoreFn).toHaveBeenCalledWith([5]);
  });

  it('переключение режима Активные/Архив сбрасывает выбор', async () => {
    wrapper = mountBase();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    expect(wrapper.vm.selectedIds).toEqual([1]);
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('строка, ушедшая из видимого списка по поиску, убирается из selectedIds', async () => {
    wrapper = mountBase();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    expect(wrapper.vm.selectedIds).toEqual([1]);
    wrapper.vm.searchQuery = 'запись 2';
    await flushPromises();
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('кнопка "Снять выбор" очищает selectedIds', async () => {
    wrapper = mountBase();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    await wrapper.find('[data-testid="test-bl-bulk-clear"]').trigger('click');
    expect(wrapper.vm.selectedIds).toEqual([]);
  });
});
