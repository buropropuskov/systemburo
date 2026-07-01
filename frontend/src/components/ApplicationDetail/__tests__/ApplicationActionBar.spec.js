import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) })),
}));

import ApplicationActionBar from '../ApplicationActionBar.vue';

const APPROVE = '[data-testid="app-detail-button-approve"]';
const REJECT = '[data-testid="app-detail-button-reject"]';
const HINT = '[data-testid="app-detail-blacklist-gate-hint"]';

function mountBar(props = {}) {
  return mount(ApplicationActionBar, {
    props: {
      application: { id: 1, confirmation: 'Согласование', status: 'Непрочитано' },
      currentUserId: 1,
      responsibleUsers: [{ id: 1, approval_status: 'pending' }],
      approvers: [],
      ...props,
    },
  });
}

describe('ApplicationActionBar - гейт ЧС (#481, срез 6b)', () => {
  it('при непереопределённом флаге кнопка "Согласовать" заблокирована и видна подсказка', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.find(APPROVE).attributes('disabled')).toBeDefined();
    expect(wrapper.find(HINT).exists()).toBe(true);
  });

  it('без флагов кнопка "Согласовать" активна и подсказки нет', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: false });
    expect(wrapper.find(APPROVE).attributes('disabled')).toBeUndefined();
    expect(wrapper.find(HINT).exists()).toBe(false);
  });

  it('гейт не блокирует "Отказать" - отклонить помеченную заявку можно сразу', () => {
    const wrapper = mountBar({ hasUnoverriddenBlacklistFlags: true });
    expect(wrapper.find(REJECT).attributes('disabled')).toBeUndefined();
  });

  it('для совмещённой роли блокируется "Согласовать и принять"', () => {
    const wrapper = mountBar({
      hasUnoverriddenBlacklistFlags: true,
      approvers: [{ user_id: 1 }],
    });
    const approve = wrapper.find(APPROVE);
    expect(approve.text()).toContain('Согласовать и принять');
    expect(approve.attributes('disabled')).toBeDefined();
    expect(wrapper.find(HINT).exists()).toBe(true);
  });

  it('после голоса approve-кнопки нет - гейт и подсказка не показываются', () => {
    const wrapper = mountBar({
      hasUnoverriddenBlacklistFlags: true,
      responsibleUsers: [{ id: 1, approval_status: 'approved' }],
    });
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(HINT).exists()).toBe(false);
  });
});

describe('ApplicationActionBar - отозванная заявка (#951)', () => {
  it('approver/responsible-действия скрыты, виден бейдж "Отозвана"', () => {
    const wrapper = mountBar({
      application: { id: 1, confirmation: 'Согласование', status: 'Отозвана' },
      responsibleUsers: [{ id: 1, approval_status: 'pending' }],
      approvers: [{ user_id: 1 }],
    });
    expect(wrapper.find(APPROVE).exists()).toBe(false);
    expect(wrapper.find(REJECT).exists()).toBe(false);
    expect(wrapper.text()).toContain('Отозвана');
  });
});
