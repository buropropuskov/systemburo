import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationDetail from '../ApplicationDetail.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Деталь заявки на mounted дёргает loadApplicationDetails/markAsRead - глушим, нас
// интересует только гейтинг центр-действий (срез 5): forward / approve / history.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue({}) }));

const stubs = {
  teleport: true,
  ForwardModal: true,
  ApplicationActionBar: { name: 'ApplicationActionBar', template: '<div class="action-bar-stub"><slot name="user-actions" /></div>' },
  ApplicationAttachments: true,
  ApplicationMessageModal: true,
  ApplicationAttachmentDetail: true,
  ApplicationConfirmation: true,
  ApplicationHistory: true,
  ApplicationQuestions: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  BlacklistOverrideModal: true,
  Badge: true,
};

const APP = {
  id: 1,
  application_number: 'A-1',
  sending_datetime: '2026-01-01T10:00:00Z',
  status: 'Согласование',
  confirmation: 'Согласование',
  organization_name: 'Орг',
};

/** Задаёт режим/права в свежем сторе до монтирования детали. */
function seedPerms({ mode = 'normal', allow = [] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountDetail(props = {}) {
  return mount(ApplicationDetail, {
    props: { application: APP, currentUserId: 5, mode: 'center', ...props },
    global: { stubs },
  });
}

/** Делает текущего пользователя ответственным -> canForwardApplication/canLeaveComment. */
async function makeResponsible(w) {
  w.vm.responsibleUsers = [{ id: 5, approval_status: 'pending' }];
  await w.vm.$nextTick();
}

const forward = w => w.find('[data-testid="app-detail-button-forward"]');
const actionBar = w => w.find('.action-bar-stub');
const duplicate = w => w.find('.duplicate-dropdown');
const history = w => w.find('.history-button-section');
const comment = w => w.find('.comment-action-section');

describe('ApplicationDetail — гейтинг центр-действий (срез 5)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  describe('Переслать заявку (action.forward.application)', () => {
    it('normal с правом и ответственный: кнопка пересылки видна', async () => {
      seedPerms({ allow: ['action.forward.application'] });
      const w = mountDetail();
      await makeResponsible(w);
      expect(forward(w).exists()).toBe(true);
    });

    it('normal без права, но ответственный: кнопка скрыта (тумблер гейтит)', async () => {
      seedPerms({ allow: [] });
      const w = mountDetail();
      await makeResponsible(w);
      expect(forward(w).exists()).toBe(false);
    });

    it('super: кнопка видна без явного гранта', async () => {
      seedPerms({ mode: 'super' });
      const w = mountDetail();
      await makeResponsible(w);
      expect(forward(w).exists()).toBe(true);
    });
  });

  describe('Согласовать заявку (action.approve.application)', () => {
    it('normal с правом: панель действий и поле комментария видны', async () => {
      seedPerms({ allow: ['action.approve.application'] });
      const w = mountDetail();
      await makeResponsible(w);
      expect(actionBar(w).exists()).toBe(true);
      expect(comment(w).exists()).toBe(true);
    });

    it('normal без права: панель действий и поле комментария скрыты', async () => {
      seedPerms({ allow: [] });
      const w = mountDetail();
      await makeResponsible(w);
      expect(actionBar(w).exists()).toBe(false);
      expect(comment(w).exists()).toBe(false);
    });

    it('admin (mode admin): панель действий видна без явного гранта', async () => {
      seedPerms({ mode: 'admin' });
      const w = mountDetail();
      await makeResponsible(w);
      expect(actionBar(w).exists()).toBe(true);
    });

    it('режим заявителя (mode=user) без права: панель и «Продублировать» остаются (гейт согласования не трогает user-режим)', async () => {
      seedPerms({ allow: [] });
      const w = mountDetail({ mode: 'user' });
      await makeResponsible(w);
      expect(actionBar(w).exists()).toBe(true);
      expect(duplicate(w).exists()).toBe(true);
    });
  });

  describe('История заявки (center.application_history) — Q2: только по ключу', () => {
    it('normal с правом: секция истории видна', () => {
      seedPerms({ allow: ['center.application_history'] });
      expect(history(mountDetail()).exists()).toBe(true);
    });

    it('normal без права: секция истории скрыта', () => {
      seedPerms({ allow: [] });
      expect(history(mountDetail()).exists()).toBe(false);
    });

    it('режим заявителя (mode=user) без права: история своей заявки тоже скрыта (без context-исключения)', () => {
      seedPerms({ allow: [] });
      expect(history(mountDetail({ mode: 'user' })).exists()).toBe(false);
    });

    it('super: секция истории видна без явного гранта', () => {
      seedPerms({ mode: 'super' });
      expect(history(mountDetail()).exists()).toBe(true);
    });
  });
});
