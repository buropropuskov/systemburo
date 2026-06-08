import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getCitizenshipHistory = vi.fn();
vi.mock('@/api/citizenships', () => ({
  getCitizenshipHistory: (...args) => getCitizenshipHistory(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CitizenshipHistoryModal from '../CitizenshipHistoryModal.vue';

async function mountWith(history) {
  getCitizenshipHistory.mockResolvedValue(history);
  const wrapper = mount(CitizenshipHistoryModal, {
    props: {
      citizenship: { id: 7, name: 'Россия' },
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
    details: { name: 'Россия' },
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('CitizenshipHistoryModal', () => {
  beforeEach(() => {
    getCitizenshipHistory.mockReset();
    document.body.innerHTML = '';
  });

  it('загружает историю по id гражданства', async () => {
    await mountWith([entry()]);
    expect(getCitizenshipHistory).toHaveBeenCalledWith(7);
  });

  it('created: "Гражданство создано" + "Наименование: <имя>"', async () => {
    const wrapper = await mountWith([entry()]);

    expect(wrapper.find('.action-text').text()).toBe('Гражданство создано');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Наименование:');
    expect(rows[0].text()).toContain('Россия');
    expect(rows[0].find('.change-arrow').exists()).toBe(false);
  });

  it('updated: имя как old → new, флаги is_default/patent_required как Да/Нет', async () => {
    const wrapper = await mountWith([
      entry({
        id: 2,
        action_type: 'updated',
        details: {
          name: { old: 'Старое', new: 'Новое' },
          is_default: { old: false, new: true },
          patent_required: { old: true, new: false },
        },
      }),
    ]);

    expect(wrapper.find('.action-text').text()).toBe('Изменены данные гражданства');
    const rows = wrapper.findAll('.change-row');
    expect(rows).toHaveLength(3);
    // Порядок полей фиксирован: name, is_default, patent_required (icon отсутствует в diff).
    expect(rows[0].text()).toContain('Наименование:');
    expect(rows[0].find('.change-old').text()).toBe('Старое');
    expect(rows[0].find('.change-new').text()).toBe('Новое');
    expect(rows[1].text()).toContain('По умолчанию:');
    expect(rows[1].find('.change-old').text()).toBe('Нет');
    expect(rows[1].find('.change-new').text()).toBe('Да');
    expect(rows[2].text()).toContain('Требуется патент:');
    expect(rows[2].find('.change-old').text()).toBe('Да');
    expect(rows[2].find('.change-new').text()).toBe('Нет');
  });

  it('updated: пустая иконка рендерится как "—"', async () => {
    const wrapper = await mountWith([
      entry({
        id: 6,
        action_type: 'updated',
        details: { icon: { old: '', new: 'ru' } },
      }),
    ]);

    const row = wrapper.find('.change-row');
    expect(row.text()).toContain('Иконка:');
    expect(row.find('.change-old').text()).toBe('—');
    expect(row.find('.change-new').text()).toBe('ru');
  });

  it('archived/restored: деталей нет (null и undefined), только текст действия', async () => {
    // Бэк опускает details для archived/restored - на фронт может прийти null или undefined.
    const wrapper = await mountWith([
      entry({ id: 4, action_type: 'archived', details: null }),
      entry({ id: 5, action_type: 'restored', details: undefined, created_at: '2026-05-01T09:00:00Z' }),
    ]);

    const items = wrapper.findAll('.history-item');
    expect(items[0].find('.action-text').text()).toBe('Гражданство архивировано');
    expect(items[1].find('.action-text').text()).toBe('Гражданство восстановлено из архива');
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
});
