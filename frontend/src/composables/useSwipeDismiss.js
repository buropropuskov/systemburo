import { ref } from 'vue';

/**
 * Свайп-вниз-закрытие для bottom-sheet модалок на мобилке (#1097 W3.4).
 * Переиспользуется читалкой новостей и деталью заявки (W3.9).
 *
 * Лист тянется за пальцем только ВНИЗ и только когда жест начат либо с ползунка
 * (handleSelector), либо когда прокручиваемый контент уже вверху (getScrollTop<=0) -
 * иначе свайп внутри длинного текста = обычный скролл, а не закрытие. По отпусканию:
 * протянул дальше порога -> onDismiss(); иначе лист возвращается на место.
 *
 * Потребитель байндит handlers на корень листа и вешает inline-transform по offset:
 *   :style="offset ? { transform: `translateY(${offset}px)` } : null"
 *   :class="{ 'is-dragging': isDragging }"   // is-dragging { transition: none } - 1:1 за пальцем
 *
 * @param {() => void} onDismiss вызывается когда лист протянут дальше порога
 * @param {{ threshold?: number, getScrollTop?: () => number, handleSelector?: string }} [options]
 * @returns {{ offset: import('vue').Ref<number>, isDragging: import('vue').Ref<boolean>,
 *            onTouchStart: (e: TouchEvent) => void, onTouchMove: (e: TouchEvent) => void,
 *            onTouchEnd: () => void }}
 */
export function useSwipeDismiss(onDismiss, options = {}) {
  const { threshold = 90, getScrollTop = () => 0, handleSelector = null } = options;
  const offset = ref(0);
  const isDragging = ref(false);
  let startY = 0;
  let active = false;

  function onTouchStart(e) {
    if (!e.touches || e.touches.length !== 1) return;
    startY = e.touches[0].clientY;
    const fromHandle = !!(handleSelector && e.target?.closest?.(handleSelector));
    // Свайп-закрытие только с ползунка или когда контент прокручен вверх.
    active = fromHandle || getScrollTop() <= 0;
    offset.value = 0;
    isDragging.value = false;
  }

  function onTouchMove(e) {
    if (!active || !e.touches || e.touches.length !== 1) return;
    const dy = e.touches[0].clientY - startY;
    if (dy <= 0) {
      // Вверх не тянем - отдаём жест скроллу.
      offset.value = 0;
      isDragging.value = false;
      return;
    }
    isDragging.value = true;
    offset.value = dy;
    // Пока тянем лист - гасим прокрутку/резину страницы под ним.
    if (e.cancelable) e.preventDefault();
  }

  function onTouchEnd() {
    if (!active) return;
    const dismissed = offset.value > threshold;
    offset.value = 0;
    isDragging.value = false;
    active = false;
    if (dismissed) onDismiss();
  }

  return { offset, isDragging, onTouchStart, onTouchMove, onTouchEnd };
}
