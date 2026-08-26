import { isNavigationFailure } from 'vue-router';

/**
 * Мост от service worker к роутеру (#974). По клику на push-уведомление worker
 * просит уже открытую вкладку перейти к заявке - это переход внутри приложения,
 * без перезагрузки, и он работает даже когда вкладка worker'у не подчиняется
 * (открыта до включения push). Ответ обязателен: по нему worker понимает, что
 * слушатель на этой стороне есть, и не станет открывать второе окно.
 *
 * @param {import('vue-router').Router} router
 * @returns {() => void} снять слушателя (нужно только тестам - в приложении он живёт всё время)
 */
export function attachPushNavigationListener(router) {
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return () => {};

  const handler = (event) => {
    if (event.data?.type !== 'push-navigate') return;
    const url = typeof event.data.url === 'string' ? event.data.url : '/';
    // Только внутренние адреса. Сообщение приходит от своего worker'а, но
    // маршрутизировать по внешнему URL всё равно нечего, а router.push с ним
    // молча уводит на несуществующий маршрут.
    if (!url.startsWith('/')) return;

    // Отвечаем до перехода: гард маршрута дожидается прав (fetchPermissions),
    // и worker не должен всё это время считать вкладку глухой.
    event.ports?.[0]?.postMessage({ ok: true });

    router.push(url).catch((err) => {
      // Гард штатно уводит в другое место: гостя - на вход, носителя без
      // page.center - в личный кабинет. Это не сбой навигации.
      if (!isNavigationFailure(err)) console.error('[push] переход по уведомлению не удался:', err);
    });
  };

  navigator.serviceWorker.addEventListener('message', handler);
  return () => navigator.serviceWorker.removeEventListener('message', handler);
}
