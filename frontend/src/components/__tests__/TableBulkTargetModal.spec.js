import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import TableBulkTargetModal from '../TableBulkTargetModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

// Реальная форма /system-tables (GetAll -> SystemTableWithDetails): double-wrap { table: {...} }.
// Плоская фикстура маскировала баг (availableTables читал t.table_type = undefined).
const SYSTEM_TABLES = [
  { table: { id: 1, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
  { table: { id: 2, name: 'kpp-2', display_name: 'КПП 2', table_type: 'cars', status: 'active' } },
  { table: { id: 3, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' } },
];

// BaseModal не рендерит контент при show=false; watch(show) срабатывает на false->true
// и запускает fetchTables (см. AskQuestionModal.spec.js).
async function mountOpened(props = {}) {
  const wrapper = mount(TableBulkTargetModal, {
    props: {
      show: false,
      mode: 'move',
      entityType: 'cars',
      excludeTableId: 1,
      selectedCount: 2,
      submitting: false,
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

describe('TableBulkTargetModal (#1194 S4)', () => {
  beforeEach(() => {
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url === '/system-tables') return okResponse(SYSTEM_TABLES);
      return okResponse([]);
    });
  });

  it('заголовок зависит от mode', async () => {
    const move = await mountOpened({ mode: 'move' });
    expect(move.find('.base-modal__title').text()).toBe('Перенести в таблицу');

    const add = await mountOpened({ mode: 'add' });
    expect(add.find('.base-modal__title').text()).toBe('Добавить в таблицу');
  });

  it('фильтрует по entityType и исключает excludeTableId', async () => {
    const wrapper = await mountOpened({ entityType: 'cars', excludeTableId: 1 });
    const labels = wrapper.findAll('.passage__item').map((t) => t.text());
    expect(labels).toEqual(['КПП 2']);
  });

  it('entityType=people показывает только people-таблицы', async () => {
    const wrapper = await mountOpened({ entityType: 'people', excludeTableId: null });
    const labels = wrapper.findAll('.passage__item').map((t) => t.text());
    expect(labels).toEqual(['Проход 1']);
  });

  it('пустой список целей -> "Нет доступных таблиц"', async () => {
    apiRequest.mockImplementation(() => okResponse([]));
    const wrapper = await mountOpened();
    expect(wrapper.find('[data-testid="table-bulk-target-empty"]').text()).toContain('Нет доступных таблиц');
  });

  it('клик по плитке выбирает таблицу, apply эмитит массив id', async () => {
    const wrapper = await mountOpened();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');

    expect(wrapper.emitted('apply')).toEqual([[[2]]]);
  });

  it('apply дизейблен без выбора и во время submitting', async () => {
    const wrapper = await mountOpened({ submitting: true });
    expect(wrapper.find('[data-testid="table-bulk-target-apply"]').attributes('disabled')).toBeDefined();

    await wrapper.setProps({ submitting: false });
    expect(wrapper.find('[data-testid="table-bulk-target-apply"]').attributes('disabled')).toBeDefined();

    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.find('[data-testid="table-bulk-target-apply"]').attributes('disabled')).toBeUndefined();
  });

  it('отмена эмитит close', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="table-bulk-target-cancel"]').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('повторное открытие сбрасывает предыдущий выбор', async () => {
    const wrapper = await mountOpened();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.vm.tableIds).toEqual([2]);

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.vm.tableIds).toEqual([]);
  });
});
