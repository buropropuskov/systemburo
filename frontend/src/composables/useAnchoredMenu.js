import { ref, onBeforeUnmount } from 'vue';
import { anchoredMenuStyle } from '@/utils/anchoredMenu';

/**
 * Меню, привязанное к кнопке внутри списка, но живущее в body.
 *
 * Телепорт в body нужен, когда ячейка узкая и с overflow: hidden - внутри неё список
 * обрезается. Плата за это - position: fixed, который сам за строкой не едет: при
 * прокрутке меню оставалось висеть на месте, оторвавшись от своей кнопки. Поэтому
 * позиция пересчитывается на scroll и resize, пока меню открыто.
 *
 * @param {{width: number, height: number, gap?: number, margin?: number}} size
 *        габариты меню для расчёта стороны раскрытия (см. anchoredMenuStyle)
 * @returns {{openId: import('vue').Ref, style: import('vue').Ref,
 *            openUp: import('vue').Ref, toggle: Function, close: Function}}
 */
export function useAnchoredMenu(size) {
  const openId = ref(null);
  const style = ref(null);
  const openUp = ref(false);
  let anchor = null;

  function update() {
    if (!anchor) {
      close();
      return;
    }
    const placed = anchoredMenuStyle(anchor, size);
    style.value = placed.style;
    openUp.value = placed.openUp;
  }

  function close() {
    openId.value = null;
    anchor = null;
    window.removeEventListener('scroll', update, true);
    window.removeEventListener('resize', update);
  }

  function toggle(id, event) {
    if (openId.value === id) {
      close();
      return;
    }
    anchor = event.currentTarget;
    openId.value = id;
    update();
    // Слушаем на фазе перехвата: прокручивается внутренний контейнер списка, а не окно.
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
  }

  onBeforeUnmount(close);

  return { openId, style, openUp, toggle, close };
}
