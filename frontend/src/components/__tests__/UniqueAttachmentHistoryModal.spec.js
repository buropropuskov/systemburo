import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getAttachmentHistory = vi.fn();
vi.mock('@/api/attachments', () => ({
  getAttachmentHistory: (...args) => getAttachmentHistory(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));
// notify нужен для проверки error-ветки loadHistory.
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

import UniqueAttachmentHistoryModal from '../UniqueAttachmentHistoryModal.vue';

async function mountWith(history) {
  getAttachmentHistory.mockResolvedValue(history);
  const wrapper = mount(UniqueAttachmentHistoryModal, {
    props: {
      attachment: { id: 7, name: 'Автозаявка' },
      currentUserName: 'Иванов Иван',
    },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function entry(over = {}) {
  return {
    id: 1,
    action_type: 'created',
    details: { display_name: 'Автозаявка' },
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('UniqueAttachmentHistoryModal', () => {
  beforeEach(() => {
    getAttachmentHistory.mockReset();
    notify.mockClear();
    document.body.innerHTML = '';
  });

  it('загружает историю по id вложения', async () => {
    await mountWith([entry()]);
    expect(getAttachmentHistory).toHaveBeenCalledWith(7);
  });

  it('открывается видимым (visible=true по mounted, overlay отрендерен)', async () => {
    const wrapper = await mountWith([entry()]);
    expect(wrapper.vm.visible).toBe(true);
    expect(wrapper.find('.modal-overlay').exists()).toBe(true);
  });

  it('created: "Вложение создано" + "Наименование: <имя>"', async () => {
    const wrapper = await mountWith([entry()]);

    expect(wrapper.find('.action-text').text()).toBe('Вложение создано');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Наименование:');
    expect(rows[0].text()).toContain('Автозаявка');
    expect(rows[0].find('.change-arrow').exists()).toBe(false);
  });

  it('updated: тип вложения human-readable, заголовок как old → new', async () => {
    const wrapper = await mountWith([
      entry({
        id: 2,
        action_type: 'updated',
        details: {
          attachment_type: { old: 'cars', new: 'people' },
          title: { old: 'АВТО', new: 'ЛЮДИ' },
        },
      }),
    ]);

    expect(wrapper.find('.action-text').text()).toBe('Изменены данные вложения');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(2);
    // Порядок полей фиксирован: attachment_type, name, display_name, title, instruction.
    expect(rows[0].text()).toContain('Тип вложения:');
    expect(rows[0].find('.change-old').text()).toBe('Машины');
    expect(rows[0].find('.change-new').text()).toBe('Люди');
    expect(rows[1].text()).toContain('Заголовок:');
    expect(rows[1].find('.change-old').text()).toBe('АВТО');
    expect(rows[1].find('.change-new').text()).toBe('ЛЮДИ');
  });

  it('updated: инструкция показывается как факт "изменена" (rich-text не дампим)', async () => {
    const wrapper = await mountWith([
      entry({
        id: 3,
        action_type: 'updated',
        details: { instruction: { old: '<p>старая</p>', new: '<p>новая</p>' } },
      }),
    ]);

    const row = wrapper.find('.change-row');
    expect(row.text()).toContain('Инструкция:');
    expect(row.find('.change-new').text()).toBe('изменена');
    // Без стрелки old → new - чтобы HTML инструкции не попал в timeline.
    expect(row.find('.change-arrow').exists()).toBe(false);
    expect(row.text()).not.toContain('старая');
  });

  it('updated: пустое старое значение рендерится как "—"', async () => {
    const wrapper = await mountWith([
      entry({
        id: 6,
        action_type: 'updated',
        details: { display_name: { old: '', new: 'Заявка на людей' } },
      }),
    ]);

    const row = wrapper.find('.change-row');
    expect(row.text()).toContain('Наименование:');
    expect(row.find('.change-old').text()).toBe('—');
    expect(row.find('.change-new').text()).toBe('Заявка на людей');
  });

  it('archived/restored: деталей нет (null и undefined), только текст действия', async () => {
    // Бэк опускает details для archived/restored - на фронт может прийти null или undefined.
    const wrapper = await mountWith([
      entry({ id: 4, action_type: 'archived', details: null }),
      entry({ id: 5, action_type: 'restored', details: undefined, created_at: '2026-05-01T09:00:00Z' }),
    ]);

    const items = wrapper.findAll('.history-item');
    expect(items[0].find('.action-text').text()).toBe('Вложение архивировано');
    expect(items[1].find('.action-text').text()).toBe('Вложение восстановлено из архива');
    expect(wrapper.find('.change-list').exists()).toBe(false);
    expect(items[0].find('.timeline-dot').classes()).toContain('dot-deactivate');
    expect(items[1].find('.timeline-dot').classes()).toContain('dot-activate');
  });

  it('поиск фильтрует по имени актора и тексту изменений', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_name: 'Сидоров С.С.', details: { display_name: 'Заявка на машины' } }),
      entry({ id: 2, actor_name: 'Петров П.П.', details: { display_name: 'Заявка на ТМЦ' } }),
    ]);

    expect(wrapper.findAll('.history-item')).toHaveLength(2);

    await wrapper.find('.search-input').setValue('сидоров');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);

    await wrapper.find('.search-input').setValue('тмц');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.change-row').text()).toContain('Заявка на ТМЦ');
  });

  it('фильтр пользователей собирает уникальных акторов', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_user_id: 3, actor_name: 'Петров П.П.' }),
      entry({ id: 2, actor_user_id: 5, actor_name: 'Сидоров С.С.' }),
      entry({ id: 3, actor_user_id: 3, actor_name: 'Петров П.П.' }),
    ]);

    await wrapper.find('.custom-select').trigger('click');
    // 2 уникальных пользователя + опция "Все пользователи".
    expect(wrapper.findAll('.select-option')).toHaveLength(3);
  });

  it('дропдаун пользователей закрывается по клику снаружи (ref, не $el при Teleport)', async () => {
    const wrapper = await mountWith([entry({ actor_user_id: 3, actor_name: 'Петров П.П.' })]);

    await wrapper.find('.custom-select').trigger('click');
    expect(wrapper.find('.select-dropdown').exists()).toBe(true);

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await wrapper.vm.$nextTick();
    expect(wrapper.find('.select-dropdown').exists()).toBe(false);
  });

  it('пустая история показывает "История пуста"', async () => {
    const wrapper = await mountWith([]);
    expect(wrapper.find('.history-empty').text()).toBe('История пуста');
  });

  it('updated: системное имя (name) под лейблом "Системное имя"', async () => {
    const wrapper = await mountWith([
      entry({ id: 8, action_type: 'updated', details: { name: { old: 'avto', new: 'avtozayavka' } } }),
    ]);
    const row = wrapper.find('.change-row');
    expect(row.text()).toContain('Системное имя:');
    expect(row.find('.change-old').text()).toBe('avto');
    expect(row.find('.change-new').text()).toBe('avtozayavka');
  });

  it('пустой actor_name отображается как "Система"', async () => {
    const wrapper = await mountWith([entry({ actor_user_id: 9, actor_name: '' })]);
    expect(wrapper.find('.user-name').text()).toBe('Система');
    // и в фильтре пользователей системный актор тоже как "Система"
    await wrapper.find('.custom-select').trigger('click');
    const options = wrapper.findAll('.select-option').map((o) => o.text());
    expect(options).toContain('Система');
  });

  it('сортировка: desc - новые сверху, клик по .sort-btn переключает на asc', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, action_type: 'archived', details: null, created_at: '2026-05-01T10:00:00Z' }),
      entry({ id: 2, action_type: 'restored', details: undefined, created_at: '2026-05-01T09:00:00Z' }),
    ]);
    // desc по умолчанию: новее (10:00, archived) сверху
    let texts = wrapper.findAll('.history-item .action-text').map((t) => t.text());
    expect(texts[0]).toBe('Вложение архивировано');
    expect(texts[1]).toBe('Вложение восстановлено из архива');

    await wrapper.find('.sort-btn').trigger('click');
    texts = wrapper.findAll('.history-item .action-text').map((t) => t.text());
    // asc: старее (09:00, restored) сверху
    expect(texts[0]).toBe('Вложение восстановлено из архива');
    expect(texts[1]).toBe('Вложение архивировано');
  });

  it('ошибка загрузки: notify(type:error) + history пуст + "История пуста", loading снят', async () => {
    getAttachmentHistory.mockRejectedValue(new Error('boom'));
    const wrapper = mount(UniqueAttachmentHistoryModal, {
      props: { attachment: { id: 7, name: 'Автозаявка' }, currentUserName: '' },
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
    it('requestClose прячет overlay (visible=false) - запускает leave-переход, но НЕ эмитит close сразу', async () => {
      const wrapper = await mountWith([entry()]);
      wrapper.vm.requestClose();
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.visible).toBe(false);
      // close эмитится только ПОСЛЕ leave-перехода (@after-leave), не моментально.
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
      // useOverlayClose закрывает только если и mousedown, и mouseup были на overlay
      await overlay.trigger('mousedown');
      await overlay.trigger('mouseup');
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.visible).toBe(false);
    });
  });
});
