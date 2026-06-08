import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getLicenseFormatHistory = vi.fn();
vi.mock('@/api/licenseFormats', () => ({
  getLicenseFormatHistory: (...args) => getLicenseFormatHistory(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import LicensePlateFormatHistoryModal from '../LicensePlateFormatHistoryModal.vue';

async function mountWith(history) {
  getLicenseFormatHistory.mockResolvedValue(history);
  const wrapper = mount(LicensePlateFormatHistoryModal, {
    props: {
      format: { id: 7, name: 'Российские номера' },
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
    details: { name: 'Российские номера' },
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('LicensePlateFormatHistoryModal', () => {
  beforeEach(() => {
    getLicenseFormatHistory.mockReset();
    document.body.innerHTML = '';
  });

  it('created: "Формат создан" + "Наименование: <имя>"', async () => {
    const wrapper = await mountWith([entry()]);

    expect(wrapper.find('.action-text').text()).toBe('Формат создан');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Наименование:');
    expect(rows[0].text()).toContain('Российские номера');
    expect(rows[0].find('.change-arrow').exists()).toBe(false);
  });

  it('updated: скалярные поля выводятся как old → new, is_default как Да/Нет', async () => {
    const wrapper = await mountWith([
      entry({
        id: 2,
        action_type: 'updated',
        details: {
          name: { old: 'Старое', new: 'Новое' },
          country_code: { old: '', new: 'RU' },
          is_default: { old: false, new: true },
        },
      }),
    ]);

    expect(wrapper.find('.action-text').text()).toBe('Изменены данные формата');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(3);
    // Порядок полей фиксирован: name, country_code, is_default.
    expect(rows[0].text()).toContain('Наименование:');
    expect(rows[0].find('.change-old').text()).toBe('Старое');
    expect(rows[0].find('.change-new').text()).toBe('Новое');
    // Пустой код страны рендерится как "—".
    expect(rows[1].find('.change-old').text()).toBe('—');
    expect(rows[1].find('.change-new').text()).toBe('RU');
    expect(rows[2].text()).toContain('По умолчанию:');
    expect(rows[2].find('.change-old').text()).toBe('Нет');
    expect(rows[2].find('.change-new').text()).toBe('Да');
  });

  it('updated: diff ячеек сводится в читаемую подпись', async () => {
    const wrapper = await mountWith([
      entry({
        id: 3,
        action_type: 'updated',
        details: {
          cells: {
            old: [
              { cell_order: 0, cell_type: 'letters', min_length: 1, max_length: 1, allowed_letters: 'АВ' },
              { cell_order: 1, cell_type: 'numbers', min_length: 3, max_length: 3, padding_side: 'left' },
            ],
            new: [
              { cell_order: 0, cell_type: 'letters', min_length: 1, max_length: 2, allowed_letters: 'АВЕ' },
            ],
          },
        },
      }),
    ]);

    const row = wrapper.find('.change-row');
    expect(row.text()).toContain('Клетки формата:');
    expect(row.find('.change-old').text()).toBe('№1 Буквы 1-1 [АВ]; №2 Цифры 3-3 (доп. слева)');
    expect(row.find('.change-new').text()).toBe('№1 Буквы 1-2 [АВЕ]');
  });

  it('archived/restored: деталей нет (null и undefined), только текст действия', async () => {
    // Бэк опускает details для archived/restored - на фронт может прийти null или undefined.
    const wrapper = await mountWith([
      entry({ id: 4, action_type: 'archived', details: null }),
      entry({ id: 5, action_type: 'restored', details: undefined, created_at: '2026-05-01T09:00:00Z' }),
    ]);

    const items = wrapper.findAll('.history-item');
    expect(items[0].find('.action-text').text()).toBe('Формат архивирован');
    expect(items[1].find('.action-text').text()).toBe('Формат восстановлен из архива');
    expect(wrapper.find('.change-list').exists()).toBe(false);
    expect(items[0].find('.timeline-dot').classes()).toContain('dot-deactivate');
    expect(items[1].find('.timeline-dot').classes()).toContain('dot-activate');
  });

  it('поиск фильтрует по имени актора и тексту изменений', async () => {
    const wrapper = await mountWith([
      entry({ id: 1, actor_name: 'Сидоров С.С.', details: { name: 'Казахстан' } }),
      entry({ id: 2, actor_name: 'Петров П.П.', details: { name: 'Беларусь' } }),
    ]);

    expect(wrapper.findAll('.history-item')).toHaveLength(2);

    await wrapper.find('.search-input').setValue('сидоров');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);

    await wrapper.find('.search-input').setValue('беларусь');
    expect(wrapper.findAll('.history-item')).toHaveLength(1);
    expect(wrapper.find('.change-row').text()).toContain('Беларусь');
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

  it('пустая история показывает "История пуста"', async () => {
    const wrapper = await mountWith([]);
    expect(wrapper.find('.history-empty').text()).toBe('История пуста');
  });

  it('visible=true по mounted, overlay отрендерен', async () => {
    const wrapper = await mountWith([entry()]);
    expect(wrapper.vm.visible).toBe(true);
    expect(wrapper.find('.modal-overlay').exists()).toBe(true);
    wrapper.unmount();
  });

  it('requestClose прячет overlay (visible=false), но НЕ эмитит close сразу', async () => {
    const wrapper = await mountWith([entry()]);
    wrapper.vm.requestClose();
    expect(wrapper.vm.visible).toBe(false);
    // close эмитится только ПОСЛЕ leave-перехода (@after-leave), не моментально.
    expect(wrapper.emitted('close')).toBeFalsy();
    wrapper.unmount();
  });

  it('close эмитится по завершении leave-перехода (onAfterLeave)', async () => {
    const wrapper = await mountWith([entry()]);
    wrapper.vm.onAfterLeave();
    expect(wrapper.emitted('close')).toHaveLength(1);
    wrapper.unmount();
  });

  it('Escape запускает закрытие через requestClose (visible=false)', async () => {
    const wrapper = await mountWith([entry()]);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.vm.visible).toBe(false);
    wrapper.unmount();
  });

  it('клик по overlay (mousedown+mouseup) закрывает через useOverlayClose', async () => {
    const wrapper = await mountWith([entry()]);
    const overlay = wrapper.find('.modal-overlay');
    await overlay.trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.vm.visible).toBe(false);
    wrapper.unmount();
  });
});
