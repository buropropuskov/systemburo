import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRouter, createMemoryHistory } from 'vue-router';
import { attachPushNavigationListener } from '../pushNavigation';

/**
 * Вторая половина перехода по push-уведомлению (#974): service worker просит
 * вкладку сменить маршрут и ждёт подтверждения. Без ответа он считает вкладку
 * глухой и перезагружает её целиком, поэтому подтверждение здесь - не
 * формальность, а условие мягкого перехода.
 */
function listenerRegistry() {
  const handlers = [];
  navigator.serviceWorker = {
    addEventListener: (name, handler) => { if (name === 'message') handlers.push(handler); },
    removeEventListener: (name, handler) => {
      const i = handlers.indexOf(handler);
      if (i >= 0) handlers.splice(i, 1);
    },
  };
  return handlers;
}

function message(data, port) {
  return { data, ports: port ? [port] : undefined };
}

describe('переход по клику на push-уведомление', () => {
  let handlers;
  let router;

  beforeEach(() => {
    handlers = listenerRegistry();
    router = { push: vi.fn().mockResolvedValue(undefined) };
  });

  afterEach(() => {
    delete navigator.serviceWorker;
    vi.restoreAllMocks();
  });

  it('ведёт роутер на адрес из сообщения и подтверждает приём worker\'у', () => {
    attachPushNavigationListener(router);
    const port = { postMessage: vi.fn() };

    handlers[0](message({ type: 'push-navigate', url: '/?open_application=121' }, port));

    expect(router.push).toHaveBeenCalledWith('/?open_application=121');
    expect(port.postMessage).toHaveBeenCalledWith({ ok: true });
  });

  it('подтверждение уходит до перехода - гард маршрута ждёт права, worker ждать не должен', () => {
    attachPushNavigationListener(router);
    const order = [];
    const port = { postMessage: vi.fn(() => order.push('ответ')) };
    router.push = vi.fn(() => { order.push('переход'); return Promise.resolve(); });

    handlers[0](message({ type: 'push-navigate', url: '/?open_application=7' }, port));

    expect(order).toEqual(['ответ', 'переход']);
  });

  it('чужие сообщения от worker\'а игнорируются', () => {
    attachPushNavigationListener(router);

    handlers[0](message({ type: 'что-то-другое', url: '/news' }));

    expect(router.push).not.toHaveBeenCalled();
  });

  it('внешний адрес не маршрутизируется', () => {
    attachPushNavigationListener(router);
    const port = { postMessage: vi.fn() };

    handlers[0](message({ type: 'push-navigate', url: 'https://example.com/phish' }, port));

    expect(router.push).not.toHaveBeenCalled();
    expect(port.postMessage).not.toHaveBeenCalled();
  });

  it('отменённая гардом навигация не считается сбоем и не пишет в консоль', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    // Настоящий роутер, а не мок: отказ навигации помечается внутренним
    // символом vue-router, подделать его объектом с полем type нельзя.
    const stub = { template: '<div />' };
    const real = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: stub }, { path: '/news', component: stub }],
    });
    real.beforeEach((to) => (to.path === '/news' ? false : true));
    attachPushNavigationListener(real);

    handlers[0](message({ type: 'push-navigate', url: '/news' }));
    await real.isReady().catch(() => {});
    await new Promise((r) => setTimeout(r, 0));

    expect(real.currentRoute.value.path).not.toBe('/news');
    expect(consoleError).not.toHaveBeenCalled();
  });

  it('снятие слушателя отключает переходы', () => {
    const detach = attachPushNavigationListener(router);
    const handler = handlers[0];
    detach();

    handler(message({ type: 'push-navigate', url: '/news' }));

    expect(handlers).toHaveLength(0);
  });

  it('браузер без service worker не роняет запуск приложения', () => {
    delete navigator.serviceWorker;

    expect(() => attachPushNavigationListener(router)()).not.toThrow();
  });
});
