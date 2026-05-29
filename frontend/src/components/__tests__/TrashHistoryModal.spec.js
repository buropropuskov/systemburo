import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getTrashHistory = vi.fn();
vi.mock('@/api/trash', () => ({
  getTrashHistory: (...args) => getTrashHistory(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import TrashHistoryModal from '../TrashHistoryModal.vue';

async function mountWith(history) {
  getTrashHistory.mockResolvedValue(history);
  const wrapper = mount(TrashHistoryModal, {
    props: { tableId: 1, tableDisplayName: 'КПП №4', currentUserName: 'Иванов Иван' },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function entry(over = {}) {
  return {
    id: 1,
    action_type: 'cleared',
    affected_count: 1,
    details: [{ id: 10, label: 'М 001 АА 77 Киа' }],
    user_name: 'Иванов И.И.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('TrashHistoryModal', () => {
  beforeEach(() => {
    getTrashHistory.mockReset();
    document.body.innerHTML = '';
  });

  it('одна деталь: заголовок inline "Удалено: <label>" без кнопки Раскрыть', async () => {
    const wrapper = await mountWith([entry()]);

    expect(wrapper.find('.action-title').text()).toBe('Удалено: М 001 АА 77 Киа');
    expect(wrapper.find('.detail-toggle').exists()).toBe(false);
  });

  it('несколько деталей: "Удалено N элемент(ов)" + Раскрыть -> список -> Свернуть', async () => {
    const wrapper = await mountWith([
      entry({ affected_count: 2, details: [{ id: 1, label: 'A' }, { id: 2, label: 'B' }] }),
    ]);

    expect(wrapper.find('.action-title').text()).toBe('Удалено 2 элемент(ов)');
    const toggle = wrapper.find('.detail-toggle');
    expect(toggle.text()).toBe('Раскрыть (2)');
    expect(wrapper.find('.detail-list').exists()).toBe(false);

    await toggle.trigger('click');
    expect(wrapper.find('.detail-list').exists()).toBe(true);
    expect(wrapper.findAll('.detail-list li')).toHaveLength(2);
    expect(wrapper.find('.detail-toggle').text()).toBe('Свернуть');

    await wrapper.find('.detail-toggle').trigger('click');
    expect(wrapper.find('.detail-list').exists()).toBe(false);
  });

  it('восстановление использует глагол "Восстановлено"', async () => {
    const wrapper = await mountWith([
      entry({ action_type: 'bulk_restored', details: [{ id: 5, label: 'Петров П.П.' }] }),
    ]);

    expect(wrapper.find('.action-title').text()).toBe('Восстановлено: Петров П.П.');
  });

  it('поиск фильтрует историю по label детали', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, details: [{ id: 1, label: 'В 746 КУ 964 БМВ' }] }),
      entry({ id: 2, details: [{ id: 2, label: 'М 001 АА 77 Киа' }] }),
    ]);

    expect(wrapper.findAll('.history-item')).toHaveLength(2);

    await wrapper.find('.hf-input').setValue('бмв');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.action-title').text()).toContain('БМВ');
  });

  it('>10 деталей: показывается "Показать ещё", по клику список расширяется', async () => {
    const details = Array.from({ length: 12 }, (_, i) => ({ id: i + 1, label: `Авто ${i + 1}` }));
    const wrapper = await mountWith([entry({ affected_count: 12, details })]);

    await wrapper.find('.detail-toggle').trigger('click');
    expect(wrapper.findAll('.detail-list li')).toHaveLength(10);
    const more = wrapper.find('.detail-more');
    expect(more.exists()).toBe(true);

    await more.trigger('click');
    expect(wrapper.findAll('.detail-list li')).toHaveLength(12);
    expect(wrapper.find('.detail-more').exists()).toBe(false);
  });

  it('пустая история показывает "История пуста"', async () => {
    const wrapper = await mountWith([]);
    expect(wrapper.find('.history-empty').text()).toBe('История пуста');
  });
});
