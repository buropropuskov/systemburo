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
function loadServiceWorker({ windows = [] } = {}) {
  const source = readFileSync(resolve(__dirname, '../../public/sw.js'), 'utf8');
  const listeners = {};
  const showNotification = vi.fn().mockResolvedValue(undefined);
  const openWindow = vi.fn().mockResolvedValue(undefined);
  const claim = vi.fn().mockResolvedValue(undefined);
  const skipWaiting = vi.fn();
  const self = {
    addEventListener: (name, handler) => { listeners[name] = handler; },
    skipWaiting,
    registration: { showNotification },
    clients: {
      matchAll: vi.fn().mockResolvedValue(windows),
      openWindow,
      claim,
    },
  };
  new Function('self', source)(self);
  return { listeners, showNotification, openWindow, claim, skipWaiting };
}

/**
 * Вкладка приложения. `answers: false` - бандл, загруженный до выката: слушателя
 * сообщения у него нет, и на просьбу перейти он молчит.
 */
function windowClient({ answers = true, navigateFails = false } = {}) {
  return {
    focus: vi.fn().mockResolvedValue(undefined),
    navigate: navigateFails
      ? vi.fn().mockRejectedValue(new TypeError('client not controlled'))
      : vi.fn().mockResolvedValue(undefined),
    postMessage: vi.fn((message, transfer) => {
      if (!answers) return;
      transfer?.[0]?.postMessage({ ok: true });
    }),
  };
}

function clickEvent(url) {
  const waits = [];
  const event = {
    notification: { close: vi.fn(), data: url === undefined ? undefined : { url } },
    waitUntil: (p) => { waits.push(p); return p; },
  };
  return { event, done: () => Promise.all(waits) };
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

  it('картинки уведомления - лёгкие файлы под свой размер, а не логотип с подписью', () => {
    sw.listeners.push(pushEvent({ title: 'Требуется согласование', application_id: 124 }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.icon).toBe('/notification-icon.png');
    // Без badge в строке состояния Android рисует значок браузера, а не наш.
    expect(options.badge).toBe('/notification-badge.png');
  });

  // Значок говорит о поводе раньше, чем человек прочтёт текст: в шторке сначала видно
  // картинку. Плашка у всех одна, символ внутри разный.
  it.each([
    ['application_pending_acceptance', '/notification-icon-application.png'],
    ['application_approval_required', '/notification-icon-approval.png'],
    ['application_question', '/notification-icon-question.png'],
    ['password_changed', '/notification-icon-security.png'],
    ['news_published', '/notification-icon-content.png'],
  ])('тип %s получает свою картинку и свой значок', (type, expected) => {
    sw.listeners.push(pushEvent({ title: 'T', type }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.icon).toBe(expected);
    // Значок строки состояния тоже свой: иначе в шторке все уведомления
    // выглядят одинаково, пока их не развернёшь.
    expect(options.badge).toBe(expected.replace('notification-icon-', 'notification-badge-'));
  });

  it('незнакомый тип получает общий знак системы, а не пустоту', () => {
    sw.listeners.push(pushEvent({ title: 'T', type: 'нечто_неизвестное' }));

    const [, options] = sw.showNotification.mock.calls[0];
    expect(options.icon).toBe('/notification-icon.png');
    expect(options.badge).toBe('/notification-badge.png');
  });

  it('worker забирает управление сразу, не дожидаясь закрытия вкладок', async () => {
    sw.listeners.install({});
    await sw.listeners.activate({ waitUntil: (p) => p });

    expect(sw.skipWaiting).toHaveBeenCalled();
    expect(sw.claim).toHaveBeenCalled();
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

/**
 * Проверка на стенде: уведомление о заявке всплывало, по нажатию окно
 * приходило в фокус - и оставалось на прежней странице. Причина в том, что
 * navigate() запрещён для вкладки, которая этому worker'у не подчиняется
 * (открыта до включения push), а другого пути к заявке у обработчика не было.
 */
describe('service worker: переход по нажатию на уведомление', () => {
  it('открытую вкладку просит перейти саму, без перезагрузки', async () => {
    const client = windowClient();
    const sw = loadServiceWorker({ windows: [client] });
    const { event, done } = clickEvent('/?open_application=121');

    sw.listeners.notificationclick(event);
    await done();

    expect(client.focus).toHaveBeenCalled();
    expect(client.postMessage).toHaveBeenCalledWith(
      { type: 'push-navigate', url: '/?open_application=121' },
      expect.any(Array),
    );
    expect(client.navigate).not.toHaveBeenCalled();
    expect(sw.openWindow).not.toHaveBeenCalled();
  });

  it('вкладка со старым бандлом молчит - переход идёт перезагрузкой', async () => {
    const client = windowClient({ answers: false });
    const sw = loadServiceWorker({ windows: [client] });
    const { event, done } = clickEvent('/?open_application=121');

    sw.listeners.notificationclick(event);
    await done();

    expect(client.navigate).toHaveBeenCalledWith('/?open_application=121');
    expect(sw.openWindow).not.toHaveBeenCalled();
  });

  it('отказ navigate не оставляет человека на прежней странице - открывается новое окно', async () => {
    const client = windowClient({ answers: false, navigateFails: true });
    const sw = loadServiceWorker({ windows: [client] });
    const { event, done } = clickEvent('/?open_application=121');

    sw.listeners.notificationclick(event);
    await done();

    expect(sw.openWindow).toHaveBeenCalledWith('/?open_application=121');
  });

  it('открытых вкладок нет - адрес открывается новым окном', async () => {
    const sw = loadServiceWorker({ windows: [] });
    const { event, done } = clickEvent('/?open_application=121');

    sw.listeners.notificationclick(event);
    await done();

    expect(sw.openWindow).toHaveBeenCalledWith('/?open_application=121');
  });

  it('уведомление без адреса ведёт на корень, а не в никуда', async () => {
    const sw = loadServiceWorker({ windows: [] });
    const { event, done } = clickEvent(undefined);

    sw.listeners.notificationclick(event);
    await done();

    expect(sw.openWindow).toHaveBeenCalledWith('/');
  });
});
