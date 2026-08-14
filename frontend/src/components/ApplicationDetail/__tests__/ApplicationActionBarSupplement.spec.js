import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) })),
}));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: vi.fn(() => Promise.resolve(true)) }) }));

const approveSupplement = vi.fn();
const revokeSupplementApproval = vi.fn();
const decideSupplement = vi.fn();
const cancelSupplement = vi.fn();
vi.mock('@/api/applications', () => ({
  approveSupplement: (...args) => approveSupplement(...args),
  revokeSupplementApproval: (...args) => revokeSupplementApproval(...args),
  decideSupplement: (...args) => decideSupplement(...args),
  cancelSupplement: (...args) => cancelSupplement(...args),
}));

import ApplicationActionBar from '../ApplicationActionBar.vue';

const ROW = '[data-testid="supplement-actions"]';
const APPROVE = '[data-testid="supplement-button-approve"]';
const REJECT = '[data-testid="supplement-button-reject"]';
const REVOKE = '[data-testid="supplement-button-revoke"]';
const ACCEPT = '[data-testid="supplement-button-accept"]';
const REFUSE = '[data-testid="supplement-button-refuse"]';
const CANCEL = '[data-testid="supplement-button-cancel"]';
const MY_VOTE = '[data-testid="supplement-my-vote"]';
const CONFIRM = '[data-testid="confirmation-confirm"]';
const COMMENT = '[data-testid="supplement-decision-comment"]';

// Роли: 1 - согласующий раунда, 2 - принимающий, 3 - автор заявки.
const APPROVER_ROLE_ID = 2;
const AUTHOR_ID = 3;

const APPLICATION = { id: 42, confirmation: 'Согласовано', status: 'В работе', sender_user_id: AUTHOR_ID };

function round(overrides = {}) {
  return {
    id: 11,
    number: 2,
    status: 'pending',
    counts: { vehicles: 1, employees: 0, items: 0 },
    approvals: [{ user_id: 1, full_name: 'Иванов И. И.', required_approval: true, approval_status: 'pending' }],
    ...overrides,
  };
}

function mountBar(props = {}) {
  return mount(ApplicationActionBar, {
    props: {
      application: APPLICATION,
      currentUserId: 1,
      // Согласующий заявки уже проголосовал, заявка в работе - её собственные кнопки
      // при этом остаются на месте, и ряд дополнения их не подменяет.
      responsibleUsers: [{ id: 1, approval_status: 'approved' }],
      approvers: [{ user_id: APPROVER_ROLE_ID }],
      supplements: [round()],
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

beforeEach(() => {
  approveSupplement.mockReset().mockResolvedValue({ supplement_id: 11, number: 2, status: 'approved', my_status: 'approved' });
  revokeSupplementApproval.mockReset().mockResolvedValue({ supplement_id: 11, number: 2, status: 'pending', my_status: 'pending' });
  decideSupplement.mockReset().mockResolvedValue({ supplement_id: 11, number: 2, status: 'accepted', activated: 3 });
  cancelSupplement.mockReset().mockResolvedValue({ supplement_id: 11, number: 2, status: 'cancelled', activated: 0 });
});

describe('ApplicationActionBar - кнопки раунда дополнения по роли и статусу (#1685)', () => {
  it('согласующий раунда в pending видит согласование и отказ', () => {
    const wrapper = mountBar();
    expect(wrapper.find(APPROVE).exists()).toBe(true);
    expect(wrapper.find(REJECT).exists()).toBe(true);
    expect(wrapper.find(REVOKE).exists()).toBe(false);
    // Номер раунда несёт сама кнопка - отдельного бейджа с номером в ряду нет
    // (владелец: убрать бейдж, шапка заявки уже называет раунд).
    expect(wrapper.find(APPROVE).text()).toBe('Согласовать доп. №2');
  });

  it('проголосовавшему согласующему повторное голосование не предлагается - бейдж и отзыв', () => {
    const wrapper = mountBar({
      supplements: [round({ approvals: [{ user_id: 1, approval_status: 'approved' }] })],
    });
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(REJECT).exists()).toBe(false);
    expect(wrapper.find(REVOKE).exists()).toBe(true);
    expect(wrapper.find(MY_VOTE).text()).toContain('вы согласовали');
  });

  it('по отклонённому раунду отзыв голоса остаётся - он открывает круг заново', () => {
    const wrapper = mountBar({
      supplements: [round({ status: 'rejected', approvals: [{ user_id: 1, approval_status: 'rejected' }] })],
    });
    expect(wrapper.find(REVOKE).exists()).toBe(true);
    expect(wrapper.find(MY_VOTE).text()).toContain('вы отказали');
  });

  it('бейджа с номером раунда в ряду нет - дубля с шапкой заявки не заводим', () => {
    // Шапка заявки уже показывает "+ Дополнение №N на согласовании"
    // (ApplicationDetail.vue openSupplementBadge), а ряд решения стоит прямо под ней -
    // повторный бейдж "Доп. №N" здесь только дублировал ту же надпись и на мобилке
    // растягивался на всю ширину (владелец: "убери его вообще").
    const wrapper = mountBar();
    expect(wrapper.find('[data-testid="supplement-round-badge"]').exists()).toBe(false);
    expect(wrapper.find(ROW).text()).not.toContain('Дополнение №2');
  });

  it('бейдж голоса раскрашен по исходу: согласовал - success, отказал - danger', () => {
    const approved = mountBar({
      supplements: [round({ approvals: [{ user_id: 1, approval_status: 'approved' }] })],
    });
    expect(approved.find(MY_VOTE).classes()).toContain('badge--success');

    const rejected = mountBar({
      supplements: [round({ status: 'rejected', approvals: [{ user_id: 1, approval_status: 'rejected' }] })],
    });
    expect(rejected.find(MY_VOTE).classes()).toContain('badge--danger');
  });

  it('принимающий получает решение только по согласованному раунду', () => {
    const pending = mountBar({ currentUserId: APPROVER_ROLE_ID, responsibleUsers: [] });
    expect(pending.find(ACCEPT).exists()).toBe(false);

    const approved = mountBar({
      currentUserId: APPROVER_ROLE_ID,
      responsibleUsers: [],
      supplements: [round({ status: 'approved', approvals: [] })],
    });
    expect(approved.find(ACCEPT).exists()).toBe(true);
    // Тот же приём, что у "Согласовать": номер раунда несёт кнопка, не бейдж рядом.
    expect(approved.find(ACCEPT).text()).toBe('Принять доп. №2');
    expect(approved.find(REFUSE).exists()).toBe(true);
  });

  it('автор заявки может отозвать идущий раунд', () => {
    const wrapper = mountBar({ currentUserId: AUTHOR_ID, responsibleUsers: [], approvers: [] });
    expect(wrapper.find(CANCEL).exists()).toBe(true);
    expect(wrapper.find(APPROVE).exists()).toBe(false);
  });

  it('по закрытому раунду ряд действий не рисуется вовсе', () => {
    for (const status of ['accepted', 'refused', 'cancelled', 'merged']) {
      const wrapper = mountBar({
        currentUserId: AUTHOR_ID,
        responsibleUsers: [],
        approvers: [],
        supplements: [round({ status, approvals: [{ user_id: AUTHOR_ID, approval_status: 'approved' }] })],
      });
      expect(wrapper.find(ROW).exists(), `статус ${status}`).toBe(false);
    }
  });

  it('посторонний участник заявки кнопок раунда не видит', () => {
    const wrapper = mountBar({ currentUserId: 99, responsibleUsers: [], approvers: [] });
    expect(wrapper.find(ROW).exists()).toBe(false);
  });

  it('без раундов ряд не появляется', () => {
    const wrapper = mountBar({ supplements: [] });
    expect(wrapper.find(ROW).exists()).toBe(false);
  });

  it('ряд дополнения не отключает и не подменяет действия самой заявки', () => {
    // Совмещённая роль: у неё по заявке в работе есть собственная кнопка «Отозвать из
    // работы» - именно она не должна пропасть или заблокироваться из-за идущего раунда.
    const wrapper = mountBar({ approvers: [{ user_id: 1 }, { user_id: APPROVER_ROLE_ID }] });
    expect(wrapper.find(ROW).exists()).toBe(true);
    const revokeFromWork = wrapper.findAll('button').find(b => b.text() === 'Отозвать из работы');
    expect(revokeFromWork).toBeDefined();
    expect(revokeFromWork.attributes('disabled')).toBeUndefined();
  });
});

describe('ApplicationActionBar - выполнение действий по раунду (#1685)', () => {
  it('согласование уходит на бэк с комментарием и рапортует успех номером раунда', async () => {
    const wrapper = mountBar();
    await wrapper.find(APPROVE).trigger('click');
    await wrapper.find(COMMENT).setValue('  всё проверил  ');
    await wrapper.find(CONFIRM).trigger('click');
    await flushPromises();

    expect(approveSupplement).toHaveBeenCalledWith(42, 11, { status: 'approved', comment: 'всё проверил' });
    expect(wrapper.emitted('action-completed')[0][0]).toEqual({
      success: true, message: 'Дополнение №2 согласовано', type: 'success',
    });
    // Окно закрылось - повторное подтверждение по инерции невозможно.
    expect(wrapper.find(CONFIRM).exists()).toBe(false);
  });

  it('принятие сообщает, сколько строк реально встало на пост', async () => {
    const wrapper = mountBar({
      currentUserId: APPROVER_ROLE_ID,
      responsibleUsers: [],
      supplements: [round({ status: 'approved', approvals: [] })],
    });
    await wrapper.find(ACCEPT).trigger('click');
    await wrapper.find(CONFIRM).trigger('click');
    await flushPromises();

    expect(decideSupplement).toHaveBeenCalledWith(42, 11, { action: 'accept', comment: null });
    expect(wrapper.emitted('action-completed')[0][0].message)
      .toBe('Дополнение №2 принято, строк добавлено на пост: 3');
  });

  it('отзыв голоса и снятие раунда бьют в свои ручки', async () => {
    const voted = mountBar({ supplements: [round({ approvals: [{ user_id: 1, approval_status: 'approved' }] })] });
    await voted.find(REVOKE).trigger('click');
    await voted.find(CONFIRM).trigger('click');
    await flushPromises();
    expect(revokeSupplementApproval).toHaveBeenCalledWith(42, 11, { comment: null });

    const author = mountBar({ currentUserId: AUTHOR_ID, responsibleUsers: [], approvers: [] });
    await author.find(CANCEL).trigger('click');
    await author.find(CONFIRM).trigger('click');
    await flushPromises();
    expect(cancelSupplement).toHaveBeenCalledWith(42, 11, { comment: null });
    expect(author.emitted('action-completed')[0][0].message).toBe('Дополнение №2 отозвано');
  });

  it('отмена подтверждения запрос не отправляет', async () => {
    const wrapper = mountBar();
    await wrapper.find(APPROVE).trigger('click');
    await wrapper.find('[data-testid="confirmation-cancel"]').trigger('click');
    await flushPromises();
    expect(approveSupplement).not.toHaveBeenCalled();
    expect(wrapper.find(CONFIRM).exists()).toBe(false);
  });

  it('закрытие гасит окно через show, не снимая его родительским v-if - иначе уход не проиграется', async () => {
    const wrapper = mountBar();
    await wrapper.find(APPROVE).trigger('click');
    const modal = wrapper.findComponent({ name: 'BaseModal' });
    expect(modal.props('show')).toBe(true);

    await wrapper.find('[data-testid="confirmation-cancel"]').trigger('click');
    expect(wrapper.findComponent({ name: 'BaseModal' }).exists()).toBe(true);
    expect(wrapper.findComponent({ name: 'BaseModal' }).props('show')).toBe(false);
  });

  it('ошибка бэка уходит наверх человеческим текстом, кнопки остаются рабочими', async () => {
    approveSupplement.mockRejectedValue(
      Object.assign(new Error('По этому дополнению голосование уже закрыто'), { status: 409 }));
    const wrapper = mountBar();
    await wrapper.find(APPROVE).trigger('click');
    await wrapper.find(CONFIRM).trigger('click');
    await flushPromises();

    expect(wrapper.emitted('action-completed')[0][0]).toEqual({
      success: false, message: 'По этому дополнению голосование уже закрыто', type: 'error',
    });
    // Окно с введённым комментарием живо, повтор возможен без потери ввода.
    expect(wrapper.find(CONFIRM).exists()).toBe(true);
    expect(wrapper.find(APPROVE).attributes('disabled')).toBeUndefined();
  });

  it('окно решения по раунду держит общий контракт (#1097): BaseModal, ползунок-свайп, скругление 30px', async () => {
    const wrapper = mountBar();
    await wrapper.find(APPROVE).trigger('click');

    const modal = wrapper.findComponent({ name: 'BaseModal' });
    expect(modal.exists()).toBe(true);
    // Bottom-sheet со свайпом-вниз и закрытие по оверлею включены по умолчанию -
    // замок ловит, если их когда-нибудь явно отключат (:sheet-swipe="false" и т.п.).
    expect(modal.props('sheetSwipe')).toBe(true);
    expect(modal.props('closeOnOverlay')).toBe(true);
    expect(modal.props('radius')).toBe('30px');
    expect(wrapper.find('.sheet-handle').exists()).toBe(true);
  });
});
