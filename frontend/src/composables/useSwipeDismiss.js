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
  // closing - лист уезжает вниз именно свайпом (не крестиком/overlay). Потребитель на
  // @keyframes-leave (BaseModal) гасит по нему свою leave-slide, иначе второй слайд.
  const closing = ref(false);
  let startY = 0;
  let active = false;
  let engaged = false;
  let closeTimer = null;
  let resetTimer = null;

  function clearTimers() {
    if (closeTimer) { clearTimeout(closeTimer); closeTimer = null; }
    if (resetTimer) { clearTimeout(resetTimer); resetTimer = null; }
  }

  function reset() {
    clearTimers();
    offset.value = 0;
    isDragging.value = false;
    engaged = false;
    closing.value = false;
  }

  function onTouchStart(e) {
    if (!e.touches || e.touches.length !== 1) return;
    // Быстрый повторный жест на ещё-закрывающемся листе - отменяем таймеры закрытия.
    clearTimers();
    startY = e.touches[0].clientY;
    const fromHandle = !!(handleSelector && e.target?.closest?.(handleSelector));
    // Тач, начатый в поле ввода (textarea/input/select/contenteditable), НЕ трактуем как
    // свайп-закрытие: жест каретки/выделения/скролла внутри поля не должен таскать и
    // закрывать лист - иначе на мобилке preventDefault глотает фокус/ввод, и текст не
    // набрать (#1097 R4-5). Свайп с ползунка/пустых зон работает как прежде.
    const onFormField = !!e.target?.closest?.('textarea, input, select, [contenteditable="true"]');
    // Свайп-закрытие только с ползунка или когда контент прокручен вверх, и не с поля ввода.
    active = !onFormField && (fromHandle || getScrollTop() <= 0);
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

  // Плавно доводим лист ВНИЗ до конца (не резкое исчезновение), затем закрываем.
  // isDragging=false включает transition листа, offset до высоты экрана уводит лист за
  // нижнюю кромку; onDismiss (unmount) после длительности слайда. closing=true - маркер
  // закрытия-слайдом (BaseModal гасит по нему @keyframes-leave). offset ДЕРЖИМ на
  // innerHeight во время leave: inline-transform перебивает leave-slide потребителя
  // (translateY(100%) в leave-to), поэтому второго движения нет. Полный reset откладываем
  // на 320ms после onDismiss - к этому моменту leave уже отыграл и модалка скрыта, так что
  // offset->0 не даёт рывка наверх (регресс R2-1 -> R3-1).
  function slideOutAndDismiss() {
    clearTimers();
    active = false;
    isDragging.value = false;
    engaged = false;
    closing.value = true;
    offset.value = typeof window !== 'undefined' ? window.innerHeight : 1000;
    closeTimer = setTimeout(() => {
      closeTimer = null;
      onDismiss();
      resetTimer = setTimeout(() => { resetTimer = null; reset(); }, 320);
    }, 260);
  }

  function onTouchEnd() {
    if (!active) return;
    active = false;
    if (offset.value > threshold) {
      // Протянут за порог - доводим лист вниз и закрываем.
      slideOutAndDismiss();
    } else {
      // Не дотянул до порога - лист пружинит на место (transition через is-dragging=false).
      reset();
    }
  }

  // Программное закрытие тем же слайдом-вниз, что и свайп (крестик/overlay на bottom-sheet
  // без Vue-<transition>, напр. ApplicationDetail): лист уезжает вниз, затем onDismiss.
  function dismiss() {
    slideOutAndDismiss();
  }

  return { offset, isDragging, closing, onTouchStart, onTouchMove, onTouchEnd, reset, dismiss };
}
