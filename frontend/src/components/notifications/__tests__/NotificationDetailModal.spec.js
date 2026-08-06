import { describe, it, expect, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import NotificationDetailModal from '../NotificationDetailModal.vue';

function mountModal(notification, show = true) {
  return mount(NotificationDetailModal, {
    props: { show, notification },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
}

describe('NotificationDetailModal', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('рендерит полный текст сообщения без обрезки (без line-clamp)', () => {
    const longMessage = 'Очень длинное сообщение уведомления. '.repeat(10);
    const wrapper = mountModal({ id: 1, title: 'Заголовок', message: longMessage, created_at: '2026-08-06T10:00:00', data: null });

    const messageEl = wrapper.find('.notif-detail-dialog__message');
    expect(messageEl.exists()).toBe(true);
    // wrapper.text() тримит крайние пробелы - сравниваем по содержанию, не по краям строки.
    expect(messageEl.text()).toBe(longMessage.trim());
    // line-clamp - модалка обязана показывать текст целиком, не обрезая его.
    expect(messageEl.attributes('style') || '').not.toContain('-webkit-line-clamp');
  });

  it('кнопки действия нет, когда в data нет application_id', () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: JSON.stringify({ status: 'ok' }) });
    expect(wrapper.find('.lk-button--primary').exists()).toBe(false);
  });

  it('кнопка действия есть, когда в data есть application_id, и эмитит action', async () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: JSON.stringify({ application_id: 42 }) });
    const btn = wrapper.find('.lk-button--primary');
    expect(btn.exists()).toBe(true);
    expect(btn.text()).toBe('Открыть заявку');
    await btn.trigger('click');
    expect(wrapper.emitted('action')).toBeTruthy();
  });

  it('эмитит unread по клику «В непрочитанные»', async () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null });
    const buttons = wrapper.findAll('button').filter((b) => b.text() === 'В непрочитанные');
    expect(buttons.length).toBe(1);
    await buttons[0].trigger('click');
    expect(wrapper.emitted('unread')).toBeTruthy();
  });

  it('эмитит delete по клику «Удалить»', async () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null });
    const buttons = wrapper.findAll('button').filter((b) => b.text() === 'Удалить');
    expect(buttons.length).toBe(1);
    await buttons[0].trigger('click');
    expect(wrapper.emitted('delete')).toBeTruthy();
  });

  it('эмитит close по клику на крестик', async () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null });
    await wrapper.find('.notif-detail-dialog__close').trigger('click');
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('эмитит close по Escape', () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null });
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('не рендерит содержимое при show=false', () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null }, false);
    expect(wrapper.find('.notif-detail-overlay').exists()).toBe(false);
  });

  it('поля из notificationDetailFields рендерятся списком', () => {
    const wrapper = mountModal({
      id: 1,
      title: 'x',
      message: 'y',
      created_at: '2026-08-06T10:00:00',
      data: JSON.stringify({ application_number: 'A-100', status: 'Согласовано' }),
    });
    const fieldsEl = wrapper.find('.notif-detail-dialog__fields');
    expect(fieldsEl.exists()).toBe(true);
    expect(fieldsEl.text()).toContain('Заявка');
    expect(fieldsEl.text()).toContain('A-100');
    expect(fieldsEl.text()).toContain('Решение');
    expect(fieldsEl.text()).toContain('Согласовано');
  });

  it('count>1 (#1748 S7) показывает число событий рядом со временем со склонением', () => {
    const wrapper = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null, count: 3 });
    expect(wrapper.find('.notif-detail-dialog__time-events').text()).toBe('3 события');
  });

  it('count=1 или отсутствует - индикатора повторов нет', () => {
    const single = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null, count: 1 });
    expect(single.find('.notif-detail-dialog__time-events').exists()).toBe(false);
    const noCount = mountModal({ id: 1, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00', data: null });
    expect(noCount.find('.notif-detail-dialog__time-events').exists()).toBe(false);
  });
});
