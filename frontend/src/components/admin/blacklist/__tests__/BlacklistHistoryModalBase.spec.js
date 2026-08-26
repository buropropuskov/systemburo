import {
  describe, it, expect, vi,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import BlacklistHistoryModalBase from '../BlacklistHistoryModalBase.vue';

const ACTION_TEXTS = {
  created: 'Добавлена в чёрный список',
  archived: 'Снята с чёрного списка',
  restored: 'Возвращена в чёрный список',
};

const FIELD_LABELS = {
  reason: 'Причина',
  cars_deactivated: 'Деактивировано машин',
  cars_reactivated: 'Возвращено машин в оборот',
};

const history = [
  {
    id: 1,
    action_type: 'created',
    user_id: 7,
    user_name: 'Иванов Иван',
    created_at: '2026-06-01T10:00:00Z',
    details: {
      car_number: 'A1', mark_name: 'BMW', reason: 'угон', cars_deactivated: 2,
    },
  },
  {
    id: 2,
    action_type: 'archived',
    user_id: 8,
    user_name: 'Петров Пётр',
    created_at: '2026-06-02T10:00:00Z',
    details: { car_number: 'A1', mark_name: 'BMW', cars_reactivated: 0 },
  },
  {
    id: 3,
    action_type: 'restored',
    user_id: 7,
    user_name: 'Иванов Иван',
    created_at: '2026-06-03T10:00:00Z',
    details: { car_number: 'A1', mark_name: 'BMW', cars_deactivated: 1 },
  },
];

function mountModal(overrides = {}) {
  return mount(BlacklistHistoryModalBase, {
    props: {
      show: true,
      title: 'История машины «A1 BMW»',
      entityLabel: 'A1 BMW',
      loadFn: vi.fn().mockResolvedValue(history),
      actionTexts: ACTION_TEXTS,
      fieldLabels: FIELD_LABELS,
      ...overrides,
    },
    global: {
      stubs: { teleport: true, LoaderSpinner: true },
    },
  });
}

describe('BlacklistHistoryModalBase', () => {
  it('загружает историю через loadFn и рендерит тексты действий', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.findAll('.history-item')).toHaveLength(3);
    expect(wrapper.text()).toContain('Добавлена в чёрный список');
    expect(wrapper.text()).toContain('Снята с чёрного списка');
    expect(wrapper.text()).toContain('Возвращена в чёрный список');
  });

  it('детали показывают только ключи из fieldLabels, нулевые счётчики скрыты', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.getActionComment(history[0])).toBe('Причина: угон / Деактивировано машин: 2');
    // archived: cars_reactivated=0 -> пусто (car_number/mark_name не в fieldLabels)
    expect(wrapper.vm.getActionComment(history[1])).toBe('');
    expect(wrapper.vm.getActionComment(history[2])).toBe('Деактивировано машин: 1');
  });

  it('updated: порядок Было -> Стало детерминирован по fieldLabels, не по порядку details', async () => {
    const wrapper = mountModal({
      fieldLabels: { reason_old: 'Было', reason_new: 'Стало' },
    });
    await flushPromises();
    // details сознательно в обратном порядке (reason_new раньше) - вывод всё равно "Было / Стало"
    const item = { action_type: 'updated', details: { reason_new: 'новая', reason_old: 'старая' } };
    expect(wrapper.vm.getActionComment(item)).toBe('Было: старая / Стало: новая');
  });

  it('цвет точки маппится по типу действия', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.getActionClass('created')).toBe('dot-create');
    expect(wrapper.vm.getActionClass('archived')).toBe('dot-deactivate');
    expect(wrapper.vm.getActionClass('restored')).toBe('dot-activate');
    expect(wrapper.vm.getActionClass('updated')).toBe('dot-update');
    expect(wrapper.vm.getActionClass('purged')).toBe('dot-delete');
    expect(wrapper.vm.getActionClass('unknown')).toBe('dot-default');
  });

  it('commentExcludeKeys убирает ключи из комментария (рендерятся отдельно)', async () => {
    const wrapper = mountModal({
      fieldLabels: { reason: 'Причина', reason_old: 'Было', reason_new: 'Стало' },
      commentExcludeKeys: ['reason_old', 'reason_new'],
    });
    await flushPromises();
    const item = { action_type: 'updated', details: { reason_old: 'старая', reason_new: 'новая' } };
    expect(wrapper.vm.getActionComment(item)).toBe('');
  });

  it('getReasonDiff: для updated возвращает from/to, для прочих - null', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.getReasonDiff({ action_type: 'updated', details: { reason_old: 'a', reason_new: 'b' } }))
      .toEqual({ from: 'a', to: 'b' });
    expect(wrapper.vm.getReasonDiff({ action_type: 'created', details: { reason: 'x' } })).toBe(null);
  });

  it('updated рендерит diff со стрелкой (.action-diff), а не текст "Стало:"', async () => {
    const updated = [{
      id: 9,
      action_type: 'updated',
      user_id: 7,
      user_name: 'Иванов Иван',
      created_at: '2026-06-04T10:00:00Z',
      details: { car_number: 'A1', mark_name: 'BMW', reason_old: 'старая', reason_new: 'новая' },
    }];
    const wrapper = mountModal({
      loadFn: vi.fn().mockResolvedValue(updated),
      actionTexts: { updated: 'Изменена причина' },
      fieldLabels: { reason_old: 'Было', reason_new: 'Стало' },
      commentExcludeKeys: ['car_number', 'mark_name', 'reason_old', 'reason_new'],
      entityLabelFn: (it) => `${it.details.car_number} ${it.details.mark_name}`,
    });
    await flushPromises();
    expect(wrapper.find('.action-diff').exists()).toBe(true);
    expect(wrapper.find('.diff-old').text()).toBe('старая');
    expect(wrapper.find('.diff-new').text()).toBe('новая');
    expect(wrapper.find('.diff-arrow').exists()).toBe(true);
    expect(wrapper.find('.history-entity').text()).toBe('A1 BMW');
    expect(wrapper.text()).not.toContain('Стало:');
  });

  it('по умолчанию сортировка desc (новые сверху), toggle переключает на asc', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.filteredHistory[0].id).toBe(3);
    wrapper.vm.toggleSortOrder();
    expect(wrapper.vm.filteredHistory[0].id).toBe(1);
  });

  it('поиск фильтрует по имени пользователя', async () => {
    const wrapper = mountModal();
    await flushPromises();
    wrapper.vm.searchQuery = 'Петров';
    await flushPromises();
    expect(wrapper.vm.filteredHistory).toHaveLength(1);
    expect(wrapper.vm.filteredHistory[0].id).toBe(2);
  });

  it('фильтр по пользователю оставляет только его записи', async () => {
    const wrapper = mountModal();
    await flushPromises();
    wrapper.vm.selectUser(7);
    await flushPromises();
    expect(wrapper.vm.filteredHistory).toHaveLength(2);
    expect(wrapper.vm.filteredHistory.every((i) => i.user_id === 7)).toBe(true);
  });

  it('уникальные пользователи собираются из истории', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.uniqueUsers.map((u) => u.id).sort()).toEqual([7, 8]);
  });

  it('клик вне селекта закрывает дропдаун пользователей (через ref, не $el)', async () => {
    const wrapper = mountModal();
    await flushPromises();
    wrapper.vm.userDropdownOpen = true;
    const inside = wrapper.find('.custom-select').element;
    wrapper.vm.handleClickOutside({ target: inside });
    expect(wrapper.vm.userDropdownOpen).toBe(true);
    wrapper.vm.handleClickOutside({ target: document.body });
    expect(wrapper.vm.userDropdownOpen).toBe(false);
  });

  it('крестик эмитит close', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.find('.close-btn').trigger('click');
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('пустая история показывает заглушку', async () => {
    const wrapper = mountModal({ loadFn: vi.fn().mockResolvedValue([]) });
    await flushPromises();
    expect(wrapper.find('.history-empty').exists()).toBe(true);
  });
});
