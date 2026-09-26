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
 * driver.js скроллит сам, но с задержкой и уже ПОСЛЕ показа шага: человек видит
 * рамку в пустоте, а цель приезжает спустя полсекунды. Поэтому доводим сами,
 * до показа, и рамку driver меряет по конечному положению.
 *
 * Шаг, попросивший подвести цель (`scrollTo`), подводится ВСЕГДА. Без просьбы
 * скроллим, только если цель не помещается: карточка заявки дорисовывается уже
 * после проверки - блок согласования подрос вместе с согласующими, и «влезает»
 * превращалось в «уехало» на глазах (#2616).
 *
 * Просим прокрутку на каждом кадре, пока цель не встанет: карточка заявки
 * прокручивается вложенным контейнером, и тот узнаёт свою высоту позже цели.
 * Одного вызова не хватало - блок согласования (456 px в контейнере 347 px)
 * оставался за краем окна, потому что в момент вызова контейнеру нечего было
 * прокручивать: scrollHeight равнялся clientHeight (#2622).
 *
 * @param {Element|null} el
 * @param {'center'|'end'|'start'} [block] куда подвести цель; не задан - только
 *   при нехватке места, по центру.
 * @returns {Promise<void>}
 */
export function ensureInView(el, block) {
  // scrollIntoView есть не везде (jsdom в юнит-тестах) - тогда просто не скроллим.
  if (!el?.getBoundingClientRect || typeof el.scrollIntoView !== 'function') return Promise.resolve();
  const margin = 24;
  const хвостВидим = 120;
  // Цель выше окна целиком не покажешь: довольно, чтобы был виден её верх -
  // блок читается сверху вниз, и заголовок важнее нижнего края.
  const выше = () => el.getBoundingClientRect().height > window.innerHeight - margin * 2;
  const наМесте = () => {
    const r = el.getBoundingClientRect();
    if (выше()) return r.top >= 0 && r.top <= window.innerHeight - хвостВидим;
    return r.top >= margin && r.bottom <= window.innerHeight - margin;
  };
  if (!block && наМесте()) return Promise.resolve();
  const довести = () => {
    // behavior: 'auto' обязателен: на html стоит scroll-behavior: smooth, и без
    // явного указания доводка цели растягивалась на полторы секунды - шаг успевал
    // показаться с подсветкой в пустоте (#2618).
    el.scrollIntoView({ block: выше() ? 'start' : (block || 'center'), inline: 'nearest', behavior: 'auto' });
  };
  довести();
  // Ждём, пока цель ДЕЙСТВИТЕЛЬНО окажется на экране, а не один кадр (#2610).
  // Потолок - чтобы не ждать недостижимую цель.
  return new Promise((resolve) => {
    const срок = Date.now() + 1200;
    const кадр = () => {
      if (наМесте() || Date.now() > срок) {
        resolve();
        return;
      }
      довести();
      requestAnimationFrame(кадр);
    };
    requestAnimationFrame(кадр);
  });
}
