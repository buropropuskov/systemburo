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
export function isPageBusy(scope = globalThis.document) {
  scope = typeof scope === 'function' ? scope() : scope;
  if (!scope?.querySelectorAll) return false;
  const candidates = [...scope.querySelectorAll(BUSY_SELECTOR)];
  if (scope.matches?.(BUSY_SELECTOR)) candidates.unshift(scope);
  // An already mounted control may belong to an owner still fetching data.
  // Do not treat unrelated sibling/page spinners as that owner's readiness.
  for (let owner = scope.parentElement; owner; owner = owner.parentElement) {
    if (owner.matches('[aria-busy="true"]')) candidates.push(owner);
  }
  return candidates.some((el) => {
    const r = el.getBoundingClientRect();
    if (!(r.width > 0 && r.height > 0)) return false;
    // aria-hidden/inert affect interaction, not whether a loader is visibly busy.
    const view = el.ownerDocument?.defaultView;
    if (!view?.getComputedStyle) return false;
    for (let node = el; node; node = node.parentElement) {
      const style = view.getComputedStyle(node);
      const opacity = style.opacity.trim();
      if (node.hasAttribute('hidden') || style.display === 'none'
        || style.visibility === 'hidden' || style.visibility === 'collapse'
        || (opacity !== '' && Number(opacity) === 0)) return false;
    }
    return true;
  });
}

/**
 * Дождаться, пока признаки загрузки исчезнут. Возвращается сразу, если их нет, -
 * на быстрой странице стоит один запрос к DOM.
 *
 * @param {number} [timeout] потолок ожидания, мс
 * @param {AbortSignal} [signal]
 * @param {Document|Element|(() => Document|Element)} [scope] область или её живой поиск
 * @returns {Promise<boolean>} true - страница успокоилась, false - отмена или потолок
 */
export function waitForPageSettled(timeout = 5000, signal, scope = globalThis.document) {
  if (signal?.aborted) return Promise.resolve(false);
  if (!isPageBusy(scope)) return Promise.resolve(true);
  return new Promise((resolve) => {
    const start = Date.now();
    let id = null;
    let stopped = false;
    const stop = (value) => {
      if (stopped) return;
      stopped = true;
      clearInterval(id);
      signal?.removeEventListener('abort', onAbort);
      resolve(value);
    };
    const onAbort = () => stop(false);
    signal?.addEventListener('abort', onAbort);
    if (signal?.aborted) {
      stop(false);
      return;
    }
    id = setInterval(() => {
      if (!isPageBusy(scope)) stop(true);
      else if (Date.now() - start >= timeout) stop(false);
    }, POLL_MS);
  });
}
