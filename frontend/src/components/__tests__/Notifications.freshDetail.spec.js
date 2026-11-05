import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), apiRequestRaw: vi.fn() }));
vi.mock('@/services/eventStream', () => ({ default: {
  connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()), onStatus: vi.fn(() => vi.fn()),
} }));
const { notify } = vi.hoisted(() => ({ notify: vi.fn() }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));
import { apiRequest, apiRequestRaw } from '@/api/client';
import UserNotifications from '../UserNotifications.vue';
import UserNotificationsInline from '../UserNotificationsInline.vue';

const response = data => ({ ok: true, json: async () => data });
const dto = (id = 1, extra = {}) => ({ id, user_id: 273, type: 'application_question',
  is_read: true, title: 'Новое обсуждение по заявке', message: 'Текущая маска, заявка 42',
  data: JSON.stringify({ application_id: 42, application_number: '42', question_id: 9 }), ...extra });
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
let wrappers;
beforeEach(() => {
  vi.resetAllMocks();
  setActivePinia(createPinia());
  wrappers = [];
  apiRequestRaw.mockResolvedValue({ ok: true, json: async () => ({ success: true, data: [], meta: { total: 0, unread_count: 0 } }) });
});
afterEach(() => { wrappers.forEach(w => w.unmount()); document.body.replaceChildren(); });

for (const [Component, method] of [[UserNotifications, 'markAsRead'], [UserNotificationsInline, 'markRead']]) {
  describe(Component.name + ' fresh details', () => {
    async function setup() {
      const push = vi.fn(async () => {});
      const wrapper = mount(Component, { props: Component === UserNotifications ? { show: true } : {},
        attachTo: document.body, global: { mocks: { $router: { push } } } });
      wrappers.push(wrapper);
      await flushPromises();
      wrapper.vm.notifications = [dto(1, { is_read: false, message: 'Старое имя', group_key: 'app:42' }), dto(2)];
      wrapper.vm.unreadCount = 4;
      return { wrapper, push, open: item => wrapper.vm[method](item || wrapper.vm.notifications[0]) };
    }

    it('always reads an already-read row and shows the new masked projection', async () => {
      const { wrapper, open } = await setup();
      wrapper.vm.notifications[0].is_read = true;
      apiRequest.mockResolvedValue(response(dto()));
      await open();
      await flushPromises();
      expect(apiRequest).toHaveBeenCalledExactlyOnceWith('/notifications/1/read', {
        method: 'PUT', body: JSON.stringify({ is_read: true }), silent403: true, silent: true, localServerErrors: true,
      });
      expect(wrapper.vm.detailNotification).toEqual(dto());
      expect(document.querySelector('.notif-detail-dialog__message').textContent).toBe(dto().message);
      expect(wrapper.vm.notifications[0]).not.toHaveProperty('group_key');
    });

    it('fully replaces a rich cached row with a generic DTO and removes its action', async () => {
      const { wrapper, open, push } = await setup();
      const generic = dto(1, { title: 'Есть обновление', message: 'Есть обновление', data: null });
      apiRequest.mockResolvedValue(response(generic));
      await open();
      await flushPromises();
      expect(wrapper.vm.detailNotification).toEqual(generic);
      expect(wrapper.vm.notifications[0]).toEqual(generic);
      expect(document.querySelector('.notif-detail-dialog__message').textContent).toBe('Есть обновление');
      expect(document.querySelector('.notif-detail-dialog__fields')).toBeNull();
      expect([...document.querySelectorAll('.notif-detail-dialog button')].some(b => b.textContent.includes('Открыть заявку'))).toBe(false);
      wrapper.vm.handleDetailAction();
      expect(push).not.toHaveBeenCalled();
    });

    it('fails closed and notifies when the read request fails', async () => {
      const { wrapper, open } = await setup();
      wrapper.vm.detailNotification = dto();
      wrapper.vm.showDetailModal = true;
      apiRequest.mockRejectedValue(new Error('network'));
      await open();
      expect(wrapper.vm.showDetailModal).toBe(false);
      expect(wrapper.vm.detailNotification).toBeNull();
      expect(notify).toHaveBeenCalledExactlyOnceWith({ type: 'error', prefix: 'Не удалось загрузить подробности уведомления' });
    });

    it('keeps B when A resolves later and ignores errors from superseded clicks', async () => {
      const { wrapper, open } = await setup();
      const a = deferred(), b = deferred();
      apiRequest.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise);
      const first = open(wrapper.vm.notifications[0]), second = open(wrapper.vm.notifications[1]);
      b.resolve(response(dto(2)));
      await second;
      a.resolve(response(dto(1)));
      await first;
      expect(wrapper.vm.detailNotification.id).toBe(2);
      const oldError = deferred();
      apiRequest.mockReturnValueOnce(oldError.promise).mockResolvedValueOnce(response(dto(2)));
      const old = open(wrapper.vm.notifications[0]);
      await open(wrapper.vm.notifications[1]);
      oldError.reject(new Error('late failure'));
      await old;
      expect(wrapper.vm.detailNotification.id).toBe(2);
      expect(notify).not.toHaveBeenCalled();
    });

    it.each(['close', 'escape', 'delete', 'clear', 'unmount'])('does not open a delayed DTO after %s', async dismiss => {
      const { wrapper, open } = await setup();
      const pending = deferred();
      apiRequest.mockReturnValueOnce(pending.promise).mockResolvedValue(response({}));
      const opening = open();
      if (dismiss === 'close') wrapper.findComponent({ name: 'NotificationDetailModal' }).vm.$emit('close');
      if (dismiss === 'escape') document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      if (dismiss === 'delete') await wrapper.vm.deleteNotification(1);
      if (dismiss === 'clear') {
        // Starting a clear confirmation also dismisses an in-flight detail open.
        const { useUiStore } = await import('@/stores/ui');
        vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);
        await wrapper.vm.clearAll();
      }
      if (dismiss === 'unmount') { wrapper.unmount(); wrappers = wrappers.filter(w => w !== wrapper); }
      pending.resolve(response(dto()));
      await opening;
      expect(wrapper.vm.showDetailModal).toBe(false);
      expect(wrapper.vm.detailNotification).toBeNull();
      expect(notify).not.toHaveBeenCalled();
    });

    if (Component === UserNotifications) it('invalidates a pending open synchronously when the panel closes', async () => {
      const { wrapper, open } = await setup();
      const pending = deferred(); apiRequest.mockReturnValueOnce(pending.promise);
      const opening = open();
      await wrapper.setProps({ show: false });
      pending.resolve(response(dto())); await opening;
      expect(wrapper.vm.showDetailModal).toBe(false);
    });

    it.each([true, false])('keeps the authoritative unread count across an older SSE list response (list first: %s)', async listFirst => {
      const { wrapper, open } = await setup();
      const list = deferred(), read = deferred(), count = deferred();
      apiRequestRaw.mockReturnValueOnce(list.promise).mockReturnValueOnce(count.promise);
      const listing = wrapper.vm.fetchNotifications();
      apiRequest.mockReturnValueOnce(read.promise);
      const opening = open();
      read.resolve(response(dto())); await opening;
      const listResponse = { ok: true, json: async () => ({ success: true, data: [dto()], meta: { total: 1, unread_count: 4 } }) };
      const countResponse = { ok: true, json: async () => ({ success: true, data: [dto()], meta: { total: 1, unread_count: 3 } }) };
      if (listFirst) { list.resolve(listResponse); await listing; count.resolve(countResponse); }
      else { count.resolve(countResponse); await flushPromises(); list.resolve(listResponse); await listing; }
      await flushPromises();
      expect(wrapper.vm.unreadCount).toBe(3);
      expect(apiRequestRaw).toHaveBeenLastCalledWith('/notifications?limit=1&offset=0&filter=all', { localServerErrors: true });
    });

    it('does not overwrite a newer SSE count or reset the loaded list with the count-only response', async () => {
      const { wrapper, open } = await setup();
      const count = deferred(), list = deferred();
      apiRequest.mockResolvedValue(response(dto()));
      apiRequestRaw.mockReturnValueOnce(count.promise).mockReturnValueOnce(list.promise);
      await open();
      const listing = wrapper.vm.fetchNotifications();
      const all = [dto(), dto(2)];
      list.resolve({ ok: true, json: async () => ({ success: true, data: all, meta: { total: 2, unread_count: 5 } }) });
      await listing;
      count.resolve({ ok: true, json: async () => ({ success: true, data: [dto()], meta: { total: 2, unread_count: 3 } }) });
      await flushPromises();
      expect(wrapper.vm.unreadCount).toBe(5);
      expect(wrapper.vm.notifications.map(n => n.id)).toEqual([1, 2]);
    });
  });
}
