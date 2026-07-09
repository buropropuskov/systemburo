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
});
