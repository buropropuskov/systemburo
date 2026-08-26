/**
 * Ожидание того, что страница договорила с сервером.
 *
 * Тур решал судьбу шага по таймауту: у необязательного шага на поиск цели было
 * 700 мс, и на медленном ответе строка списка просто не успевала появиться. Шаг
 * при этом не «подождал ещё», а выпадал из тура насовсем - вместе со знаменателем
 * «Шаг N из M», который у человека на глазах падал с 57 до 48. Тот же тур на
 * быстрой сети терял два шага, на медленной - девять: состав обучения зависел от
 * скорости соединения.
 *
 * Поэтому перед проверкой цели ждём, пока со страницы уйдут признаки загрузки.
 * Признаки берём по разметке общих компонентов (`LoaderSpinner`, `Skeleton*`) -
 * отдельного сигнала «страница готова» в приложении нет, а заводить его ради
 * тура значит трогать каждый вид.
 */

const BUSY_SELECTOR = [
  '.loader-spinner',
  '[class*="skeleton-"]',
  '[aria-busy="true"]',
].join(', ');

const POLL_MS = 100;

/** @returns {boolean} есть ли на странице видимый признак загрузки. */
export function isPageBusy() {
  if (typeof document === 'undefined') return false;
  return [...document.querySelectorAll(BUSY_SELECTOR)].some((el) => {
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  });
}

/**
 * Дождаться, пока признаки загрузки исчезнут. Возвращается сразу, если их нет, -
 * на быстрой странице стоит один запрос к DOM.
 *
 * @param {number} [timeout] потолок ожидания, мс
 * @param {AbortSignal} [signal]
 * @returns {Promise<boolean>} true - страница успокоилась, false - вышли по потолку
 */
export function waitForPageSettled(timeout = 5000, signal) {
  if (!isPageBusy()) return Promise.resolve(true);
  return new Promise((resolve) => {
    const start = Date.now();
    const stop = (value) => {
      clearInterval(id);
      signal?.removeEventListener('abort', onAbort);
      resolve(value);
    };
    const onAbort = () => stop(false);
    const id = setInterval(() => {
      if (!isPageBusy()) stop(true);
      else if (Date.now() - start >= timeout) stop(false);
    }, POLL_MS);
    signal?.addEventListener('abort', onAbort);
  });
}
