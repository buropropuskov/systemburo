import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getApproverHistory = vi.fn();
vi.mock('@/api/approvers', () => ({
  getApproverHistory: (...args) => getApproverHistory(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));
// notify нужен для проверки error-ветки loadHistory.
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

import ApplicationApproverHistoryModal from '../ApplicationApproverHistoryModal.vue';

async function mountWith(history) {
  getApproverHistory.mockResolvedValue(history);
  const wrapper = mount(ApplicationApproverHistoryModal, {
    props: { currentUserName: 'Иванов Иван' },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function entry(over = {}) {
  return {
    id: 1,
    approver_user_id: 5,
    approver_name: 'Соколов Пётр Сергеевич',
    action_type: 'created',
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('ApplicationApproverHistoryModal', () => {
  beforeEach(() => {
    getApproverHistory.mockReset();
    notify.mockClear();
    document.body.innerHTML = '';
  });

  it('загружает глобальную историю без аргументов', async () => {
    await mountWith([entry()]);
    expect(getApproverHistory).toHaveBeenCalledWith();
  });

  it('открывается видимым (visible=true по mounted, overlay отрендерен)', async () => {
    const wrapper = await mountWith([entry()]);
    expect(wrapper.vm.visible).toBe(true);
    expect(wrapper.find('.modal-overlay').exists()).toBe(true);
  });

  it('created: "Добавлен принимающий" + имя принимающего в комментарии, dot-create', async () => {
    const wrapper = await mountWith([entry()]);
    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Добавлен принимающий');
    expect(item.find('.action-comment').text()).toContain('Соколов Пётр Сергеевич');
    expect(item.find('.user-name').text()).toBe('Петров П.П.');
    expect(item.find('.timeline-dot').classes()).toContain('dot-create');
  });

  it('deleted: "Удалён принимающий", dot-deactivate', async () => {
    const wrapper = await mountWith([
      entry({ id: 2, action_type: 'deleted', approver_name: 'Зайцев З.З.' }),
    ]);
    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Удалён принимающий');
    expect(item.find('.action-comment').text()).toContain('Зайцев З.З.');
    expect(item.find('.timeline-dot').classes()).toContain('dot-deactivate');
  });

  it('поиск фильтрует по актору и по имени принимающего', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_name: 'Сидоров С.С.', approver_name: 'Орлов О.О.' }),
      entry({ id: 2, actor_name: 'Петров П.П.', approver_name: 'Грачёв Г.Г.' }),
    ]);

    expect(wrapper.findAll('.history-item')).toHaveLength(2);

    await wrapper.find('.search-input').setValue('сидоров');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);

    await wrapper.find('.search-input').setValue('грач');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.action-comment').text()).toContain('Грачёв Г.Г.');
  });

  it('фильтр пользователей собирает уникальных акторов (не принимающих)', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_user_id: 3, actor_name: 'Петров П.П.', approver_name: 'A' }),
      entry({ id: 2, actor_user_id: 5, actor_name: 'Сидоров С.С.', approver_name: 'B' }),
      entry({ id: 3, actor_user_id: 3, actor_name: 'Петров П.П.', approver_name: 'C' }),
    ]);

    await wrapper.find('.custom-select').trigger('click');
    // 2 уникальных актора + опция "Все пользователи".
    expect(wrapper.findAll('.select-option')).toHaveLength(3);
  });

  it('выбор актора в фильтре оставляет только его записи', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_user_id: 3, actor_name: 'Петров П.П.' }),
      entry({ id: 2, actor_user_id: 5, actor_name: 'Сидоров С.С.' }),
    ]);
    wrapper.vm.selectUser(5);
    await wrapper.vm.$nextTick();
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.user-name').text()).toBe('Сидоров С.С.');
  });

  it('дропдаун пользователей закрывается по клику снаружи (ref, не $el при Teleport)', async () => {
    const wrapper = await mountWith([entry()]);

    await wrapper.find('.custom-select').trigger('click');
    expect(wrapper.find('.select-dropdown').exists()).toBe(true);

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await wrapper.vm.$nextTick();
    expect(wrapper.find('.select-dropdown').exists()).toBe(false);
  });

  it('пустой actor_name отображается как "Система" в записи и в фильтре', async () => {
    const wrapper = await mountWith([entry({ actor_user_id: 9, actor_name: '' })]);
    expect(wrapper.find('.user-name').text()).toBe('Система');
    await wrapper.find('.custom-select').trigger('click');
    const options = wrapper.findAll('.select-option').map((o) => o.text());
    expect(options).toContain('Система');
  });

  it('пустая история показывает "История пуста"', async () => {
    const wrapper = await mountWith([]);
    expect(wrapper.find('.history-empty').text()).toBe('История пуста');
  });

  it('сортировка: desc - новые сверху, клик по .sort-btn переключает на asc', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, action_type: 'created', approver_name: 'Новый', created_at: '2026-05-01T10:00:00Z' }),
      entry({ id: 2, action_type: 'deleted', approver_name: 'Старый', created_at: '2026-05-01T09:00:00Z' }),
    ]);
    // desc по умолчанию: новее (10:00, created) сверху
    let texts = wrapper.findAll('.history-item .action-text').map((t) => t.text());
    expect(texts[0]).toBe('Добавлен принимающий');
    expect(texts[1]).toBe('Удалён принимающий');

    await wrapper.find('.sort-btn').trigger('click');
    texts = wrapper.findAll('.history-item .action-text').map((t) => t.text());
    // asc: старее (09:00, deleted) сверху
    expect(texts[0]).toBe('Удалён принимающий');
    expect(texts[1]).toBe('Добавлен принимающий');
  });

  it('фильтр по периоду отсекает записи вне диапазона', async () => {
    // Дни с запасом, чтобы UTC<->локальный сдвиг не задел границу.
    const wrapper = await mountWith([
      entry({ id: 1, approver_name: 'Майский', created_at: '2026-05-01T10:00:00Z' }),
      entry({ id: 2, approver_name: 'Десятый', created_at: '2026-05-10T10:00:00Z' }),
    ]);
    expect(wrapper.findAll('.history-item')).toHaveLength(2);

    wrapper.vm.dateFrom = '2026-05-05';
    await wrapper.vm.$nextTick();
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.action-comment').text()).toContain('Десятый');

    wrapper.vm.dateFrom = '';
    wrapper.vm.dateTo = '2026-05-05';
    await wrapper.vm.$nextTick();
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.action-comment').text()).toContain('Майский');
  });

  it('ошибка загрузки: notify(type:error) + history пуст + "История пуста", loading снят', async () => {
    getApproverHistory.mockRejectedValue(new Error('boom'));
    const wrapper = mount(ApplicationApproverHistoryModal, {
      props: { currentUserName: '' },
      global: { stubs: { teleport: true } },
      attachTo: document.body,
    });
    await flushPromises();

    expect(notify).toHaveBeenCalledTimes(1);
    expect(notify.mock.calls[0][0]).toMatchObject({ type: 'error' });
    expect(wrapper.vm.loading).toBe(false);
    expect(wrapper.vm.history).toEqual([]);
    expect(wrapper.find('.history-empty').exists()).toBe(true);
  });

  describe('анимация закрытия', () => {
    it('requestClose прячет overlay (visible=false), но НЕ эмитит close сразу', async () => {
      const wrapper = await mountWith([entry()]);
      wrapper.vm.requestClose();
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.visible).toBe(false);
      expect(wrapper.emitted('close')).toBeFalsy();
    });

    it('close эмитится по завершении leave-перехода (onAfterLeave)', async () => {
      const wrapper = await mountWith([entry()]);
      wrapper.vm.requestClose();
      wrapper.vm.onAfterLeave();
      expect(wrapper.emitted('close')).toBeTruthy();
      expect(wrapper.emitted('close')).toHaveLength(1);
    });

    it('Escape запускает закрытие через requestClose (visible=false)', async () => {
      const wrapper = await mountWith([entry()]);
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.visible).toBe(false);
    });

    it('клик по overlay (mousedown+mouseup) закрывает через useOverlayClose', async () => {
      const wrapper = await mountWith([entry()]);
      const overlay = wrapper.find('.modal-overlay');
      await overlay.trigger('mousedown');
      await overlay.trigger('mouseup');
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.visible).toBe(false);
    });
  });
});
