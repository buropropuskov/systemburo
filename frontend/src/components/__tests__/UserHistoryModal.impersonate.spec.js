import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import UserHistoryModal from '../UserHistoryModal.vue';

async function mountWith(history) {
  apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(history) });
  const wrapper = mount(UserHistoryModal, {
    props: { user: { id: 11, username: 'target' }, currentUserName: 'Админ' },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

describe('UserHistoryModal - режим работы от имени работника', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('вход в режим показан словами, а не сырым кодом действия', async () => {
    const wrapper = await mountWith([{
      id: 1,
      action_type: 'impersonate_start',
      details: { actor_username: 'admin_ivanov', target_username: 'target', expires_at: '2026-05-01T10:30:00Z' },
      actor_user_id: 3,
      actor_name: 'Иванов И.И.',
      created_at: '2026-05-01T10:00:00Z',
    }]);

    const text = wrapper.text();
    expect(text).toContain('Вход в систему от имени работника');
    expect(text).not.toContain('impersonate_start');
  });

  it('в комментарии виден администратор и срок доступа', async () => {
    const wrapper = await mountWith([{
      id: 1,
      action_type: 'impersonate_start',
      details: { actor_username: 'admin_ivanov', expires_at: '2026-05-01T10:30:00Z' },
      actor_user_id: 3,
      actor_name: 'Иванов И.И.',
      created_at: '2026-05-01T10:00:00Z',
    }]);

    const text = wrapper.text();
    // До какого момента действовал чужой доступ - главное в этой записи: по ней
    // разбирают, чьи действия в этом окне.
    expect(text).toContain('admin_ivanov');
    expect(text).toContain('Доступ до');
  });

  it('выход из режима тоже подписан', async () => {
    const wrapper = await mountWith([{
      id: 2,
      action_type: 'impersonate_stop',
      details: { actor_username: 'admin_ivanov' },
      actor_user_id: 3,
      actor_name: 'Иванов И.И.',
      created_at: '2026-05-01T10:20:00Z',
    }]);

    expect(wrapper.text()).toContain('Выход из режима работы от имени работника');
    expect(wrapper.text()).not.toContain('impersonate_stop');
  });
});
