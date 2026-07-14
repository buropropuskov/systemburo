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
 * @param {{ threshold?: number, slop?: number, getScrollTop?: () => number, handleSelector?: string }} [options]
 * @returns {{ offset: import('vue').Ref<number>, isDragging: import('vue').Ref<boolean>,
 *            onTouchStart: (e: TouchEvent) => void, onTouchMove: (e: TouchEvent) => void,
 *            onTouchEnd: () => void }}
 */
export function useSwipeDismiss(onDismiss, options = {}) {
  const { threshold = 90, slop = 8, getScrollTop = () => 0, handleSelector = null } = options;
  const offset = ref(0);
  const isDragging = ref(false);
  let startY = 0;
  let active = false;
  let engaged = false;

  function reset() {
    offset.value = 0;
    isDragging.value = false;
    engaged = false;
  }

  function onTouchStart(e) {
    if (!e.touches || e.touches.length !== 1) return;
    startY = e.touches[0].clientY;
    const fromHandle = !!(handleSelector && e.target?.closest?.(handleSelector));
    // Свайп-закрытие только с ползунка или когда контент прокручен вверх.
    active = fromHandle || getScrollTop() <= 0;
    reset();
  }

  function onTouchMove(e) {
    if (!active) return;
    if (!e.touches || e.touches.length !== 1) {
      // Второй палец (pinch/zoom) - отменяем жест закрытия, не мешаем масштабированию.
      active = false;
      reset();
      return;
    }
    const dy = e.touches[0].clientY - startY;
    if (!engaged) {
      // Мёртвая зона: пока палец не ушёл вниз дальше slop, НЕ перехватываем событие
      // (иначе preventDefault на дребезге глотает click по кнопкам/ссылкам внутри листа).
      if (dy <= slop) {
        reset();
        return;
      }
      engaged = true;
    }
    if (dy <= 0) {
      reset();
      return;
    }
    isDragging.value = true;
    offset.value = dy;
    // Пока тянем лист - гасим прокрутку/резину страницы под ним.
    if (e.cancelable) e.preventDefault();
  }

  function onTouchEnd() {
    if (!active) return;
    active = false;
    if (offset.value > threshold) {
      // Протянут за порог: плавно доводим лист ВНИЗ до конца (не резкое исчезновение),
      // затем закрываем. isDragging=false включает transition листа, offset до высоты
      // экрана уводит лист за нижнюю кромку; onDismiss (unmount) после длительности слайда.
      isDragging.value = false;
      engaged = false;
      offset.value = typeof window !== 'undefined' ? window.innerHeight : 1000;
      setTimeout(() => {
        onDismiss();
        reset();
      }, 260);
    } else {
      // Не дотянул до порога - лист пружинит на место (transition через is-dragging=false).
      reset();
    }
  }

  return { offset, isDragging, onTouchStart, onTouchMove, onTouchEnd };
}
