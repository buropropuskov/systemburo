import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import ApplicationRecipientsRow from '../ApplicationRecipientsRow.vue';

// #884: блок получателей-читателей. Дефолтные согласующие неудаляемы, читателей
// (только Руководители) можно добавить/удалить, переполнение -> дропдаун.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([
      { id: 1, username: 'man1', user_type: 'Руководитель', last_name: 'Иванов', first_name: 'Иван', position: 'Директор' },
      { id: 2, username: 'man2', user_type: 'Руководитель', last_name: 'Петров', first_name: 'Пётр', position: '' },
      { id: 3, username: 'usr3', user_type: 'Пользователь', last_name: 'Сидоров', first_name: 'Сидор' },
      { id: 4, username: 'man4', user_type: 'Руководитель', last_name: 'Кузнецов', first_name: 'Кузьма' },
    ]),
  }),
}));

async function mountRow(props = {}) {
  const w = shallowMount(ApplicationRecipientsRow, {
    props: { approvers: [], readers: [], ...props },
  });
  await flushPromises();
  return w;
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ApplicationRecipientsRow (#884)', () => {
  it('fetchManagers берёт только пользователей с типом Руководитель', async () => {
    const w = await mountRow();
    expect(w.vm.managerUsers.map(u => u.userId).sort()).toEqual([1, 2, 4]);
    expect(w.vm.managerUsers.find(u => u.userId === 1).name).toBe('Иванов Иван');
  });

  it('addReader эмитит update:readers с добавленным читателем', async () => {
    const w = await mountRow();
    w.vm.addReader({ userId: 2, name: 'Петров Пётр', username: 'man2' });
    const ev = w.emitted('update:readers');
    expect(ev).toBeTruthy();
    expect(ev[0][0]).toEqual([{ user_id: 2, name: 'Петров Пётр', username: 'man2' }]);
  });

  it('removeReader эмитит список без удалённого', async () => {
    const w = await mountRow({
      readers: [
        { user_id: 2, name: 'Петров Пётр' },
        { user_id: 4, name: 'Кузнецов Кузьма' },
      ],
    });
    w.vm.removeReader(2);
    const ev = w.emitted('update:readers');
    expect(ev[0][0]).toEqual([{ user_id: 4, name: 'Кузнецов Кузьма' }]);
  });

  it('availableManagers исключает уже-согласующих и уже-читателей', async () => {
    const w = await mountRow({
      approvers: [{ user_id: 1, name: 'Иванов Иван' }],
      readers: [{ user_id: 2, name: 'Петров Пётр' }],
    });
    expect(w.vm.availableManagers.map(u => u.userId)).toEqual([4]);
  });

  it('согласующие неудаляемы (removable=false), читатели removable=true', async () => {
    const w = await mountRow({
      approvers: [{ user_id: 1, name: 'Иванов Иван' }],
      readers: [{ user_id: 2, name: 'Петров Пётр' }],
    });
    const chips = w.vm.allChips;
    expect(chips.find(c => c.userId === 1).removable).toBe(false);
    expect(chips.find(c => c.userId === 2).removable).toBe(true);
  });

  it('переполнение: чипы сверх лимита уходят в overflowChips', async () => {
    const approvers = Array.from({ length: 6 }, (_, i) => ({ user_id: 100 + i, name: `Согл ${i}` }));
    const w = await mountRow({ approvers });
    expect(w.vm.visibleChips.length).toBe(4);
    expect(w.vm.overflowChips.length).toBe(2);
  });

  it('addReader не дублирует уже добавленного', async () => {
    const w = await mountRow({ readers: [{ user_id: 2, name: 'Петров Пётр' }] });
    w.vm.addReader({ userId: 2, name: 'Петров Пётр', username: 'man2' });
    expect(w.emitted('update:readers')).toBeFalsy();
  });
});
