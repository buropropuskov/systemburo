import { describe, it, expect, beforeEach, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Service worker живёт в public/ и в приложение не импортируется, поэтому до
 * сих пор не был покрыт ничем. Проверка на живом стенде (#974) показала, что
 * уведомления о заявке приходили молча, мимо человека: браузер заменяет
 * уведомление с прежним тегом без всплытия и звука, если не потребовать
 * повторного оповещения. Здесь файл читается с диска и выполняется в песочнице
 * с подставным self - так проверяется ровно то, что уедет на стенд.
 */
function loadServiceWorker() {
  const source = readFileSync(resolve(__dirname, '../../public/sw.js'), 'utf8');
  const listeners = {};
  const showNotification = vi.fn().mockResolvedValue(undefined);
  const self = {
    addEventListener: (name, handler) => { listeners[name] = handler; },
    registration: { showNotification },
    clients: { matchAll: vi.fn().mockResolvedValue([]), openWindow: vi.fn().mockResolvedValue(undefined) },
  };
  // eslint-disable-next-line no-new-func
  new Function('self', source)(self);
  return { listeners, showNotification };
}

function pushEvent(payload) {
  return {
    data: { json: () => payload, text: () => JSON.stringify(payload) },
    waitUntil: (p) => p,
  };
}

describe('service worker: показ push-уведомления', () => {
  let sw;

  beforeEach(() => {
    sw = loadServiceWorker();
  });

  it('уведомление о заявке требует повторного оповещения, иначе замена проходит молча', () => {
    sw.listeners.push(pushEvent({
      title: 'Требуется согласование',
      message: 'Заявка ждёт решения',
      type: 'application_approval_required',
      application_id: 124,
    }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.renotify).toBe(true);
    expect(options.tag).toBeTruthy();
  });

  it('разные события по одной заявке не затирают друг друга', () => {
    sw.listeners.push(pushEvent({
      title: 'Требуется согласование', type: 'application_approval_required', application_id: 124,
    }));
    sw.listeners.push(pushEvent({
      title: 'Новый вопрос по заявке', type: 'application_question', application_id: 124,
    }));

    const [, first] = sw.showNotification.mock.calls[0];
    const [, second] = sw.showNotification.mock.calls[1];
    expect(first.tag).not.toBe(second.tag);
  });

  it('повтор одного события по одной заявке схлопывается в одну запись', () => {
    const event = {
      title: 'Требуется согласование', type: 'application_approval_required', application_id: 124,
    };
    sw.listeners.push(pushEvent(event));
    sw.listeners.push(pushEvent(event));

    const [, first] = sw.showNotification.mock.calls[0];
    const [, second] = sw.showNotification.mock.calls[1];
    expect(first.tag).toBe(second.tag);
  });

  it('уведомление без заявки показывается отдельным, без группировки', () => {
    sw.listeners.push(pushEvent({ title: 'Пароль изменён', type: 'password_changed' }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.tag).toBeUndefined();
    expect(options.renotify).toBe(false);
  });

  it('адрес перехода ведёт на нейтральный вход, а не сразу в кабинет', () => {
    sw.listeners.push(pushEvent({ title: 'Требуется согласование', application_id: 124 }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.data.url).toContain('open_application=124');
    expect(options.data.url).not.toContain('personal-cabinet');
  });

  it('битое тело не роняет обработчик - уведомление всё равно показывается', () => {
    sw.listeners.push({
      data: { json: () => { throw new Error('не JSON'); }, text: () => 'просто текст' },
      waitUntil: (p) => p,
    });

    expect(sw.showNotification).toHaveBeenCalledTimes(1);
    const [title, options] = sw.showNotification.mock.calls[0];
    expect(title).toBe('Бюро пропусков');
    expect(options.body).toBe('просто текст');
  });
});
