import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CompanyHistoryModal from '../CompanyHistoryModal.vue';

async function mountWith(history) {
  apiRequest.mockResolvedValue({ ok: true, json: async () => history });
  const wrapper = mount(CompanyHistoryModal, {
    props: {
      company: { id: 5, name: 'Альфа' },
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
    details: { name: 'Альфа', type: 'Подрядчик' },
    actor_user_id: 3,
    actor_name: 'Петров П.П.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

function commentText(wrapper) {
  const c = wrapper.find('.action-comment');
  return c.exists() ? c.text() : '';
}

describe('CompanyHistoryModal — тип в истории изменений', () => {
  beforeEach(() => {
    apiRequest.mockReset();
    document.body.innerHTML = '';
  });

  it('renamed: комментарий показывает наименование И тип', async () => {
    const wrapper = await mountWith([entry()]);
    const text = commentText(wrapper);
    expect(text).toContain('Альфа');
    expect(text).toContain('Тип: Подрядчик');
  });

  it('renamed со снятым типом (null): "Тип: не указан"', async () => {
    const wrapper = await mountWith([entry({ details: { name: 'Альфа', type: null } })]);
    expect(commentText(wrapper)).toContain('Тип: не указан');
  });

  it('старая запись без ключа type: тип не показывается', async () => {
    const wrapper = await mountWith([entry({ details: { name: 'Альфа' } })]);
    expect(commentText(wrapper)).not.toContain('Тип:');
  });

  it('created: тоже показывает тип', async () => {
    const wrapper = await mountWith([
      entry({ action_type: 'created', details: { name: 'Альфа', type: 'Отдел' } }),
    ]);
    expect(commentText(wrapper)).toContain('Тип: Отдел');
  });
});
