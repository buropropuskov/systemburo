/**
 * Zoom-компенсация позиционирования поповера driver.js.
 *
 * driver считает позицию поповера КОРРЕКТНО, но целиком в device-px (rect элемента,
 * innerWidth/innerHeight) и пишет результат в inline left/top/right/bottom. Поповер
 * живёт в <body> ВНУТРИ зазумленного <html> (масштаб под 1440 на мониторах >1440),
 * где эти px трактуются как layout-px и домножаются на zoom - поповер уезжает
 * вправо-вниз пропорционально zoom НА ВСЕХ шагах (и anchored, и центро-модальных).
 * Однородно делим все четыре inset'а на zoom: выбор стороны и клэмпы driver'а
 * сохраняются, т.к. масштабирование равномерное. Оверлей/spotlight НЕ трогаем -
 * они самосогласованы (viewBox в device-px при width/height:100%).
 * MutationObserver, а не одноразовый rAF: driver перезаписывает стили на СВОИХ
 * внутренних scroll/resize-слушателях, до которых снаружи не дотянуться.
 *
 * @returns {{ attach: (wrapper: HTMLElement) => void, detach: () => void }}
 */
import { getViewportZoom } from '@/utils/viewportScale';

export function createPopoverZoomFix() {
  let zoomFixObserver = null;
  let applyingZoomFix = false;
  const zoomFixLast = {};
  const INSETS = ['left', 'top', 'right', 'bottom'];

  function apply(wrapper) {
    const z = getViewportZoom();
    if (z === 1 || !wrapper) return;
    const next = {};
    let changed = false;
    for (const prop of INSETS) {
      const raw = wrapper.style[prop];
      // 'auto'/пусто не трогаем; значение, которое записали мы сами - тоже
      // (иначе поделим повторно и поповер уползёт вверх-влево).
      if (!raw || raw === 'auto' || zoomFixLast[prop] === raw) continue;
      const px = parseFloat(raw);
      if (!Number.isFinite(px)) continue;
      next[prop] = `${Math.round(px / z)}px`;
      changed = true;
    }
    if (!changed) return;
    applyingZoomFix = true;
    // Коррекция масштаба - не переезд к новому шагу, а поправка той же позиции.
    // С анимацией она читалась бы как «поповер доезжает» после каждого шага,
    // поэтому на время правки переход выключаем.
    wrapper.classList.add('ob-popover--instant');
    for (const prop of Object.keys(next)) {
      wrapper.style[prop] = next[prop];
      zoomFixLast[prop] = next[prop];
    }
    requestAnimationFrame(() => wrapper.classList.remove('ob-popover--instant'));
    applyingZoomFix = false;
  }

  function attach(wrapper) {
    detach();
    if (!wrapper || typeof MutationObserver === 'undefined') return;
    zoomFixObserver = new MutationObserver(() => {
      if (applyingZoomFix) return;
      apply(wrapper);
    });
    zoomFixObserver.observe(wrapper, { attributes: true, attributeFilter: ['style'] });
    requestAnimationFrame(() => apply(wrapper));
  }

  function detach() {
    if (zoomFixObserver) {
      zoomFixObserver.disconnect();
      zoomFixObserver = null;
    }
    for (const prop of INSETS) delete zoomFixLast[prop];
  }

  return { attach, detach };
}
