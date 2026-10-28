import { findTourTarget, isTourTargetVisible } from './targetResolver';

function hasTargetMotion(element) {
  for (let node = element; node; node = node.parentElement) {
    if (node.getAnimations?.().some(animation =>
      (animation.playState === 'running' || animation.pending)
      && Number.isFinite(animation.effect?.getComputedTiming?.().endTime))) return true;
  }
  return false;
}

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
    // Ждём стабильный прямоугольник, а не только высоту: раскрывающаяся панель
    // может сохранять размер, но ещё двигаться по экрану.
    const measure = (el) => {
      if (!isTourTargetVisible(el) || hasTargetMotion(el)) return null;
      const { top, left, width, height } = el.getBoundingClientRect();
      return [top, left, width, height];
    };

    if (signal?.aborted) {
      resolve(null);
      return;
    }

    const start = Date.now();
    let prevEl = null;
    let prevRect = null;
    const cleanup = () => {
      clearInterval(intervalId);
      signal?.removeEventListener('abort', onAbort);
    };
    const onAbort = () => {
      cleanup();
      resolve(null);
    };
    const tick = () => {
      const el = findTourTarget(selector, { preferred: prevEl });
      const rect = measure(el);
      if (rect && el === prevEl && prevRect && rect.every((value, i) => Math.abs(value - prevRect[i]) < 0.5)) {
        cleanup();
        resolve(el);
        return;
      }
      prevEl = el;
      prevRect = rect;
      if (Date.now() - start >= timeout) {
        cleanup();
        // A still-moving target is not ready just because the deadline elapsed.
        resolve(rect ? el : null);
      }
    };
    const intervalId = setInterval(tick, 120);
    signal?.addEventListener('abort', onAbort);
    tick();
  });
}

/**
 * Цель на месте? Высокую цель целиком не покажешь - довольно видимого верха:
 * блок читается сверху вниз, и заголовок важнее нижнего края.
 *
 * @param {Element} el
 * @returns {boolean}
 */
export function isTargetInView(el) {
  const r = el.getBoundingClientRect();
  const margin = 24;
  if (r.height > window.innerHeight - margin * 2) {
    return r.top >= 0 && r.top <= window.innerHeight - 120;
  }
  return r.top >= 0 && r.bottom <= window.innerHeight;
}

/**
 * Подвести цель мгновенно.
 *
 * behavior: 'instant', а не 'auto': 'auto' означает «как сказано в CSS», а в
 * App.vue звёздочка ставит scroll-behavior: smooth ВСЕМ элементам. Доводка
 * растягивалась на анимацию, каждый кадр начинал её заново, и шаг показывался,
 * пока цель была ещё в пути - на окне 1280x500 блок согласования доезжал только
 * через полторы секунды после подсветки (#2622).
 *
 * @param {Element} el
 * @param {'center'|'end'|'start'} [block]
 */
function довести(el, block) {
  const выше = el.getBoundingClientRect().height > window.innerHeight - 48;
  el.scrollIntoView({ block: выше ? 'start' : (block || 'center'), inline: 'nearest', behavior: 'instant' });
}

/** Узел, который вообще можно подводить (в jsdom scrollIntoView нет). */
const подводимый = (el) => !!el?.getBoundingClientRect && typeof el.scrollIntoView === 'function';

/**
 * Подвести цель в зону видимости до подсветки.
 *
 * driver.js скроллит сам, но плавно и уже ПОСЛЕ показа шага: человек видит рамку
 * в пустоте, а цель приезжает спустя полсекунды. Поэтому доводим сами, до показа,
 * и рамку driver меряет по конечному положению.
 *
 * Шаг, попросивший подвести цель (`scrollTo`), подводится ВСЕГДА. Без просьбы
 * скроллим, только если цель не помещается: карточка заявки дорисовывается уже
 * после проверки - блок согласования подрос вместе с согласующими, и «влезает»
 * превращалось в «уехало» на глазах (#2616).
 *
 * @param {Element|null} el
 * @param {'center'|'end'|'start'} [block] куда подвести цель; не задан - только
 *   при нехватке места, по центру.
 * @returns {Promise<void>}
 */
export function ensureInView(el, block, signal) {
  if (signal?.aborted || !подводимый(el)) return Promise.resolve();
  if (!block && isTargetInView(el)) return Promise.resolve();
  довести(el, block);
  // Ждём, пока цель ДЕЙСТВИТЕЛЬНО окажется на экране, а не один кадр (#2610).
  // Потолок - чтобы не ждать недостижимую цель.
  return new Promise((resolve) => {
    let frame;
    const stop = () => {
      cancelAnimationFrame(frame);
      signal?.removeEventListener('abort', stop);
      resolve();
    };
    signal?.addEventListener('abort', stop, { once: true });
    const срок = Date.now() + 1200;
    const кадр = () => {
      if (signal?.aborted) { stop(); return; }
      if (isTargetInView(el) || Date.now() > срок) {
        stop();
        return;
      }
      довести(el, block);
      frame = requestAnimationFrame(кадр);
    };
    frame = requestAnimationFrame(кадр);
  });
}

/**
 * Придержать цель на экране после показа шага.
 *
 * Карточка заявки доверстывается и перевёрстывается уже под открытым шагом: у
 * блока согласования прокрутка перескакивала с колонки на тело карточки, и цель,
 * подведённая до показа, уезжала обратно за край окна - подсветка оставалась в
 * пустоте (#2622). Проверяем недолго и доводим мгновенно; вырез driver едет за
 * целью сам, он слушает прокрутку.
 *
 * @param {Element|null} el
 * @param {'center'|'end'|'start'} [block]
 * @param {number} [длительность] сколько присматривать, мс
 * @param {() => boolean} [пока] пока это верно, цель ещё наша (шаг не сменился)
 * @returns {() => void} отменить присмотр
 */
export function holdInView(el, block, длительность = 900, пока) {
  if (!подводимый(el)) return () => {};
  const таймер = setInterval(() => {
    if (!el.isConnected || (пока && !пока())) {
      стоп();
      return;
    }
    if (isTargetInView(el)) return;
    довести(el, block);
  }, 40);
  const стоп = () => { clearInterval(таймер); clearTimeout(потолок); };
  const потолок = setTimeout(стоп, длительность);
  return стоп;
}
