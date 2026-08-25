/**
 * Ожидание целей тура: появление узла и его показ на экране.
 *
 * Обе функции работают с живым DOM и не зависят от состояния тура, поэтому живут
 * отдельным модулем - хосту от них нужны только промисы.
 */

/**
 * Дождаться появления и видимости элемента в DOM. Резолвит элементом или
 * `null` по таймауту - тур никогда не падает из-за отсутствующей цели.
 * `signal` позволяет хосту отменить ожидание (teardown/logout) и не оставить
 * висящий интервал.
 *
 * @param {string} selector
 * @param {number} [timeout]
 * @param {AbortSignal} [signal]
 * @returns {Promise<Element|null>}
 */
export function waitForElement(selector, timeout = 2500, signal) {
  return new Promise((resolve) => {
    const isVisible = (el) =>
      el && (el.offsetParent !== null || el.getBoundingClientRect().width > 0);
    // Готовность по СТАБИЛЬНОЙ высоте: элемент часто появляется пустым (скелетон/
    // ещё не пришли данные) и дорастает. Если резолвить по факту появления,
    // driver подсветит пустую рамку, а данные приедут уже под оверлеем. Поэтому
    // ждём, пока высота перестанет меняться между опросами.
    const measure = (el) => {
      if (!isVisible(el)) return null;
      const h = el.getBoundingClientRect().height;
      return h > 0 ? h : null;
    };

    if (signal?.aborted) {
      resolve(null);
      return;
    }

    const start = Date.now();
    let prevEl = null;
    let prevHeight = null;
    const cleanup = () => {
      clearInterval(intervalId);
      signal?.removeEventListener('abort', onAbort);
    };
    const onAbort = () => {
      cleanup();
      resolve(null);
    };
    const tick = () => {
      const el = document.querySelector(selector);
      const h = measure(el);
      if (h !== null && el === prevEl && h === prevHeight) {
        cleanup();
        resolve(el);
        return;
      }
      prevEl = el;
      prevHeight = h;
      if (Date.now() - start >= timeout) {
        cleanup();
        // По таймауту отдаём элемент, если он хотя бы виден (пусть driver
        // подсветит как есть), иначе null - цель так и не появилась.
        resolve(isVisible(el) ? el : null);
      }
    };
    const intervalId = setInterval(tick, 120);
    signal?.addEventListener('abort', onAbort);
    tick();
  });
}

/**
 * Подвести цель в зону видимости до подсветки.
 *
 * driver.js скроллит сам, но на длинной форме заявки промахивается: после шага
 * с формой сотрудников страница остаётся прокрученной вниз, и отметка согласия
 * оказывается выше экрана - вырез рисуется за краем окна, а человек видит
 * поповер без подсветки. Скроллим до показа, поэтому рамку driver меряет уже
 * по конечному положению.
 *
 * @param {Element|null} el
 * @param {'center'|'end'|'start'} [block] куда подвести цель. 'end' прижимает её
 *   к низу экрана - так делают высокие формы, над которыми встаёт поповер.
 * @returns {Promise<void>}
 */
export function ensureInView(el, block = 'center') {
  // scrollIntoView есть не везде (jsdom в юнит-тестах) - тогда просто не скроллим.
  if (!el?.getBoundingClientRect || typeof el.scrollIntoView !== 'function') return Promise.resolve();
  const rect = el.getBoundingClientRect();
  const margin = 24;
  const fits = rect.top >= margin && rect.bottom <= window.innerHeight - margin;
  if (fits && block !== 'end') return Promise.resolve();
  el.scrollIntoView({ block, inline: 'nearest' });
  // Кадр на применение скролла: без него driver померит прежнюю позицию.
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}
