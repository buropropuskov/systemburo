import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import ApplicationRecipientsRow from '../ApplicationRecipientsRow.vue';
import { apiRequest } from '@/api/client';

// #884: блок получателей-читателей. Дефолтные согласующие неудаляемы, читателей
// можно добавить/удалить, переполнение -> дропдаун.
// Срез fe-recipients: список приходит из GET /users/recipient-candidates (#1921) -
// коллеги и руководители, отобранные бэком; клиентского фильтра по типу больше нет.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}));

// Форма ответа - models.RecipientCandidate: без user_type, с pd_hidden.
const CANDIDATES = [
  { id: 1, username: 'man1', last_name: 'Иванов', first_name: 'Иван', position: 'Директор', pd_hidden: false },
  { id: 2, username: 'man2', last_name: 'Петров', first_name: 'Пётр', position: '', pd_hidden: false },
  { id: 4, username: 'man4', last_name: 'Кузнецов', first_name: 'Кузьма', position: null, pd_hidden: false },
];

function okJson(data) {
  return { ok: true, json: vi.fn().mockResolvedValue(data) };
}

async function mountRow(props = {}) {
  const w = shallowMount(ApplicationRecipientsRow, {
    props: { approvers: [], readers: [], ...props },
  });
  await flushPromises();
  return w;
}

beforeEach(() => {
  vi.clearAllMocks();
  apiRequest.mockResolvedValue(okJson(CANDIDATES));
});

describe('ApplicationRecipientsRow (#884)', () => {
  it('список берётся из /users/recipient-candidates без фильтра по типу на клиенте', async () => {
    const w = await mountRow();
    expect(apiRequest).toHaveBeenCalledWith('/users/recipient-candidates', expect.any(Object));
    expect(w.vm.candidateUsers.map(u => u.userId)).toEqual([1, 2, 4]);
    expect(w.vm.candidateUsers.find(u => u.userId === 1).name).toBe('Иванов Иван');
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

  it('availableCandidates исключает уже-согласующих и уже-читателей', async () => {
    const w = await mountRow({
      approvers: [{ user_id: 1, name: 'Иванов Иван' }],
      readers: [{ user_id: 2, name: 'Петров Пётр' }],
    });
    expect(w.vm.availableCandidates.map(u => u.userId)).toEqual([4]);
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

describe('ApplicationRecipientsRow - гейт кнопки «+ получатель»', () => {
  it('кандидаты есть - кнопка на месте', async () => {
    const w = await mountRow();
    expect(w.find('.recipients-add__btn').exists()).toBe(true);
  });

  it('кандидатов нет - ни кнопки, ни попытки открыть список', async () => {
    apiRequest.mockResolvedValue(okJson([]));
    const w = await mountRow();
    expect(w.vm.canAddRecipients).toBe(false);
    expect(w.find('.recipients-add__btn').exists()).toBe(false);
  });

  it('403: список пуст, кнопки нет, тост подавлен через silent403', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 403, json: vi.fn() });
    const w = await mountRow();
    expect(apiRequest).toHaveBeenCalledWith('/users/recipient-candidates', { silent403: true });
    expect(w.vm.candidateUsers).toEqual([]);
    expect(w.find('.recipients-add__btn').exists()).toBe(false);
  });

  it('все кандидаты уже в строке - кнопка уходит вместе с открытым окном', async () => {
    const w = await mountRow({ approvers: [{ user_id: 1, name: 'a' }, { user_id: 2, name: 'b' }] });
    await w.setData({ showAdd: true });
    expect(w.find('.recipients-add__btn').exists()).toBe(true);

    await w.setProps({ readers: [{ user_id: 4, name: 'Кузнецов Кузьма' }] });
    expect(w.vm.showAdd).toBe(false);
    expect(w.find('.recipients-add__btn').exists()).toBe(false);
  });

  it('поиск без совпадений не прячет кнопку и поле ввода', async () => {
    const w = await mountRow();
    await w.setData({ showAdd: true, search: 'такого нет' });
    expect(w.vm.availableCandidates).toEqual([]);
    expect(w.find('.recipients-add__btn').exists()).toBe(true);
    expect(w.find('.recipients-search').attributes('placeholder')).toBe('Поиск');
    expect(w.find('.recipients-add-empty').text()).toBe('Пользователей нет');
  });
});

describe('ApplicationRecipientsRow - доработка отображения', () => {
  it('shortName: полное ФИО -> Фамилия И.О.', async () => {
    const w = await mountRow();
    expect(w.vm.shortName('Иванов Иван Иванович')).toBe('Иванов И.И.');
    expect(w.vm.shortName('Иванов Иван')).toBe('Иванов И.');
    expect(w.vm.shortName('Иванов')).toBe('Иванов');
  });

  it('allChips: согласующие isApprover=true, читатели false', async () => {
    const w = await mountRow({
      approvers: [{ user_id: 1, name: 'Иванов Иван' }],
      readers: [{ user_id: 2, name: 'Петров Пётр' }],
    });
    const chips = w.vm.allChips;
    expect(chips.find(c => c.userId === 1).isApprover).toBe(true);
    expect(chips.find(c => c.userId === 2).isApprover).toBe(false);
  });

  it('чип: короткое ФИО в тексте, полное в data-hint, у согласующего класс is-approver', async () => {
    const w = await mountRow({ approvers: [{ user_id: 1, name: 'Иванов Иван Иванович' }] });
    const chip = w.find('.recipient-chip');
    expect(chip.find('.recipient-chip__name').text()).toBe('Иванов И.И.');
    expect(chip.attributes('data-hint')).toBe('Иванов Иван Иванович');
    expect(chip.classes()).toContain('is-approver');
  });

  it('скрытые ПД: вместо ФИО логин и подпись, почему имени нет', async () => {
    apiRequest.mockResolvedValue(okJson([
      { id: 7, username: 'hidden7', last_name: null, first_name: null, middle_name: null, position: '', pd_hidden: true },
    ]));
    const w = await mountRow();
    await w.setData({ showAdd: true });
    expect(w.find('.recipients-add-item__name').text()).toBe('hidden7');
    expect(w.find('.recipients-add-item__masked').text()).toContain('скрыто до согласия');
  });

  it('клик вне "+ получатель" закрывает дропдаун', async () => {
    const w = await mountRow();
    await w.setData({ showAdd: true });
    w.vm.handleOutside({ target: document.body });
    expect(w.vm.showAdd).toBe(false);
  });
});
