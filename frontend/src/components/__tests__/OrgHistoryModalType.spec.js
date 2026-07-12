import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import OrgHistoryModal from '../OrgHistoryModal.vue';

async function mountWith(history) {
  apiRequest.mockResolvedValue({ ok: true, json: async () => history });
  const wrapper = mount(OrgHistoryModal, {
    props: {
      organization: { id: 5, name: 'Альфа' },
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
    action_type: 'renamed',
    details: { name: 'Альфа' },
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

function actionText(wrapper) {
  const a = wrapper.find('.action-text');
  return a.exists() ? a.text() : '';
}
function commentText(wrapper) {
  const c = wrapper.find('.action-comment');
  return c.exists() ? c.text() : '';
}

describe('OrgHistoryModal — действия и тип в истории', () => {
  beforeEach(() => {
    apiRequest.mockReset();
    document.body.innerHTML = '';
  });

  it('retyped: «Тип организации изменён» + «Тип: X», без слова про наименование', async () => {
    const wrapper = await mountWith([entry({ action_type: 'retyped', details: { name: 'Альфа', type: 'Отдел' } })]);
    expect(actionText(wrapper)).toBe('Тип организации изменён');
    expect(commentText(wrapper)).toContain('Тип: Отдел');
    expect(commentText(wrapper)).not.toContain('наименование');
  });

  it('retyped со снятым типом (null): «Тип: не указан»', async () => {
    const wrapper = await mountWith([entry({ action_type: 'retyped', details: { name: 'Альфа', type: null } })]);
    expect(commentText(wrapper)).toContain('Тип: не указан');
  });

  it('renamed: «Организация переименована» + только наименование (без типа)', async () => {
    const wrapper = await mountWith([entry({ action_type: 'renamed', details: { name: 'Бета', type: 'Отдел' } })]);
    expect(actionText(wrapper)).toBe('Организация переименована');
    expect(commentText(wrapper)).toContain('Новое наименование: Бета');
    expect(commentText(wrapper)).not.toContain('Тип:');
  });

  it('updated: «Организация изменена» + наименование И тип', async () => {
    const wrapper = await mountWith([entry({ action_type: 'updated', details: { name: 'Гамма', type: 'Подрядчик' } })]);
    expect(actionText(wrapper)).toBe('Организация изменена');
    const text = commentText(wrapper);
    expect(text).toContain('Новое наименование: Гамма');
    expect(text).toContain('Тип: Подрядчик');
  });

  it('created: наименование И тип', async () => {
    const wrapper = await mountWith([entry({ action_type: 'created', details: { name: 'Альфа', type: 'Организация' } })]);
    expect(commentText(wrapper)).toContain('Наименование: Альфа');
    expect(commentText(wrapper)).toContain('Тип: Организация');
  });

  it('responsibles_changed: добавлены (с согласованием) / убраны / смена согласования', async () => {
    const wrapper = await mountWith([entry({
      action_type: 'responsibles_changed',
      details: {
        added: [{ username: 'ivan', name: 'Иванов И.', required_approval: true }],
        removed: [{ username: 'petr', name: 'Петров П.' }],
        approval_changed: [{ username: 'sid', name: 'Сидоров С.', from: false, to: true }],
      },
    })]);
    expect(actionText(wrapper)).toBe('Ответственные изменены');
    const text = commentText(wrapper);
    expect(text).toContain('Добавлены: Иванов И. (согласование)');
    expect(text).toContain('Убраны: Петров П.');
    expect(text).toContain('Согласование изменено: Сидоров С. (согласование: нет → да)');
  });

  it('responsibles_changed: смена главного ответственного (primary_changed)', async () => {
    const wrapper = await mountWith([entry({
      action_type: 'responsibles_changed',
      details: { primary_changed: { from: { username: 'ivan', name: 'Иванов И.' }, to: { username: 'petr', name: 'Петров П.' } } },
    })])
    expect(actionText(wrapper)).toBe('Ответственные изменены')
    expect(commentText(wrapper)).toContain('Главный ответственный: Иванов И. → Петров П.')
  })

  it('primary_changed nil-ветки: первое назначение и снятие главного', async () => {
    const wAssigned = await mountWith([entry({ action_type: 'responsibles_changed', details: { primary_changed: { to: { username: 'p', name: 'Петров П.' } } } })])
    expect(commentText(wAssigned)).toContain('Главный ответственный: не был назначен → Петров П.')

    const wRemoved = await mountWith([entry({ action_type: 'responsibles_changed', details: { primary_changed: { from: { username: 'i', name: 'Иванов И.' } } } })])
    expect(commentText(wRemoved)).toContain('Главный ответственный: Иванов И. → снят')
  })

  it('unload_places_changed / tables_changed: added/removed по именам', async () => {
    const wPlaces = await mountWith([entry({ action_type: 'unload_places_changed', details: { added: ['Склад 1'], removed: ['Склад 2'] } })]);
    expect(actionText(wPlaces)).toBe('Места разгрузки изменены');
    expect(commentText(wPlaces)).toContain('Добавлены: Склад 1');
    expect(commentText(wPlaces)).toContain('Убраны: Склад 2');

    const wTables = await mountWith([entry({ action_type: 'tables_changed', details: { added: ['Таблица А'] } })]);
    expect(actionText(wTables)).toBe('Таблицы изменены');
    expect(commentText(wTables)).toContain('Добавлены: Таблица А');
    expect(commentText(wTables)).not.toContain('Убраны');
  });

  it('retyped/updated с from: рендерит «было → стало»', async () => {
    const wType = await mountWith([entry({ action_type: 'retyped', details: { type: 'Отдел', from: { name: 'Альфа', type: 'Организация' } } })]);
    expect(commentText(wType)).toContain('Тип: Организация → Отдел');

    const wName = await mountWith([entry({ action_type: 'updated', details: { name: 'Гамма', type: 'Отдел', from: { name: 'Бета', type: 'Отдел' } } })]);
    expect(commentText(wName)).toContain('Наименование: Бета → Гамма');
  });
});
