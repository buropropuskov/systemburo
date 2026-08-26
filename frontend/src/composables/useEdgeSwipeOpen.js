import { ref, onMounted, onBeforeUnmount } from 'vue';

/**
 * Свайп вправо, открывающий боковое меню на мобилке (#1097 W4.1).
 *
 * Мёртвая зона у левой кромки - главное здесь. На телефонах с жестовой навигацией
 * системный «Назад» живёт ровно в первых пикселях экрана, и меню, ловящее свайп от края,
 * отбирает его у системы. Поэтому жест считается нашим, только если палец лёг правее
 * `deadZone`; всё левее уходит браузеру нетронутым - `preventDefault` там не зовётся.
 *
 * Ось решается один раз, на выходе из мёртвой зоны `slop`: горизонталь должна обгонять
 * вертикаль, иначе это прокрутка страницы и жест отпускается насовсем. Тач, начатый в
 * поле ввода или внутри горизонтально прокручиваемого блока (карусель, широкая таблица),
 * не перехватывается вовсе - там свайп значит другое.
 *
 * Слушатель `touchmove` не висит на документе постоянно: он не пассивный (иначе не
 * погасить прокрутку под панелью) и на каждом скролле стоил бы кадров. Вешается после
 * валидного `touchstart`, снимается на `touchend`.
 *
 * Потребитель тянет панель за пальцем через offset и гасит на это время её transition:
 *   :style="offset ? { transform: `translateX(calc(-100% + ${offset}px))` } : null"
 *   :class="{ 'is-dragging': isDragging }"   // is-dragging { transition: none }
 *
 * @param {() => void} onOpen вызывается, когда панель протянута дальше порога
 * @param {{ deadZone?: number, threshold?: number, slop?: number, width?: number,
 *           isEnabled?: () => boolean }} [options]
 * @returns {{ offset: import('vue').Ref<number>, isDragging: import('vue').Ref<boolean>,
 *            onTouchStart: (e: TouchEvent) => void, onTouchMove: (e: TouchEvent) => void,
 *            onTouchEnd: () => void }}
 */
export function useEdgeSwipeOpen(onOpen, options = {}) {
  const {
    deadZone = 24,
    threshold = 90,
    slop = 8,
    width = 280,
    isEnabled = () => true,
  } = options;

  const offset = ref(0);
  const isDragging = ref(false);
  let startX = 0;
  let startY = 0;
  let tracking = false;
  let engaged = false;

  // Предок с собственной горизонтальной прокруткой: свайп внутри карусели типов бланка
  // или широкой таблицы листает её, а не открывает меню. Глубину ограничиваем - обход
  // идёт на каждом касании, а нужные контейнеры лежат близко к цели.
  function insideHorizontalScroller(target) {
    let node = target;
    for (let depth = 0; node && node.nodeType === 1 && depth < 24; depth += 1) {
      // Переполнение ПРЯМО СЕЙЧАС не требуем: лента, в которой пока помещается всё,
      // всё равно листается вбок, как только данных станет больше, а решение о жесте
      // принимается в момент касания (#1097 волна 6: «листал таблицу вправо - у меня
      // постоянно открывалась навигация»).
      const { overflowX } = window.getComputedStyle(node);
      if (overflowX === 'auto' || overflowX === 'scroll') return true;
      node = node.parentElement;
    }
    return false;
  }

  /**
   * Элемент заявил собственный горизонтальный жест (`data-swipe-own`) - меню в него
   * не лезет. Так панель предупреждения на подаче заявки смахивается вправо, не
   * вытягивая заодно навигацию: до этого один жест обслуживали оба обработчика.
   */
  function ownsHorizontalGesture(target) {
    return !!target?.closest?.('[data-swipe-own]');
  }

  /**
   * Страница сама прокручивается вбок - палец листает её, а не открывает меню.
   * Горизонтальный разъезд на телефоне сам по себе дефект и чинится отдельно, но
   * пока он есть, меню не должно выпрыгивать на каждое движение вбок.
   */
  function documentScrollsHorizontally() {
    const de = document.documentElement;
    return de.scrollWidth - de.clientWidth > 1;
  }

  function release() {
    detachDrag();
    tracking = false;
    engaged = false;
    isDragging.value = false;
    offset.value = 0;
  }

  function onTouchStart(e) {
    if (!isEnabled()) return;
    if (!e.touches || e.touches.length !== 1) return;
    const touch = e.touches[0];
    // Кромка принадлежит системному «Назад» - не наш жест, молча пропускаем.
    if (touch.clientX < deadZone) return;
    if (e.target?.closest?.('textarea, input, select, [contenteditable="true"]')) return;
    if (ownsHorizontalGesture(e.target)) return;
    if (insideHorizontalScroller(e.target)) return;
    if (documentScrollsHorizontally()) return;
    startX = touch.clientX;
    startY = touch.clientY;
    tracking = true;
    engaged = false;
    attachDrag();
  }

  function onTouchMove(e) {
    if (!tracking) return;
    if (!e.touches || e.touches.length !== 1) {
      // Второй палец - это pinch-zoom, масштабированию не мешаем.
      release();
      return;
    }
    const dx = e.touches[0].clientX - startX;
    const dy = e.touches[0].clientY - startY;
    if (!engaged) {
      if (Math.abs(dx) < slop && Math.abs(dy) < slop) return;
      // Решение об оси принимается один раз и обжалованию не подлежит: иначе диагональ
      // посреди прокрутки страницы вытянет меню.
      if (dx <= 0 || Math.abs(dx) <= Math.abs(dy)) {
        release();
        return;
      }
      engaged = true;
    }
    if (dx <= 0) {
      offset.value = 0;
      return;
    }
    isDragging.value = true;
    offset.value = Math.min(dx, width);
    // Пока панель идёт за пальцем - гасим прокрутку и резину страницы под ней.
    if (e.cancelable) e.preventDefault();
  }

  function onTouchEnd() {
    if (!tracking) return;
    const reached = offset.value > threshold;
    // Сначала отпускаем панель (transition включается, inline-transform снимается), и
    // только потом открываем: панель доезжает от места, где её оставил палец.
    release();
    if (reached) onOpen();
  }

  function attachDrag() {
    document.addEventListener('touchmove', onTouchMove, { passive: false });
    document.addEventListener('touchend', onTouchEnd);
    document.addEventListener('touchcancel', release);
  }

  function detachDrag() {
    document.removeEventListener('touchmove', onTouchMove);
    document.removeEventListener('touchend', onTouchEnd);
    document.removeEventListener('touchcancel', release);
  }

  onMounted(() => {
    document.addEventListener('touchstart', onTouchStart, { passive: true });
  });

  onBeforeUnmount(() => {
    document.removeEventListener('touchstart', onTouchStart);
    detachDrag();
  });

  return { offset, isDragging, onTouchStart, onTouchMove, onTouchEnd };
}
