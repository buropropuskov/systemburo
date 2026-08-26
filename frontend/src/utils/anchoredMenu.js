import { getViewportZoom } from '@/utils/viewportScale';

/**
 * Положение всплывающего меню, которое лежит в body, а привязано к кнопке внутри
 * списка или таблицы.
 *
 * Все величины приводятся к layout-px делением на масштаб страницы: rect отдаёт
 * device-px, а innerWidth/innerHeight - незумленные, и без общего знаменателя меню
 * уезжает от кнопки тем дальше, чем правее она стоит (см. тот же расчёт в
 * BaseDropdown).
 *
 * Снизу место есть не всегда - строка бывает последней в списке, и меню уходит под
 * край карточки. Не хватает - открываем вверх. По горизонтали держим меню целиком в
 * окне: считаем от правого края кнопки, но не даём левому краю уйти за поле.
 *
 * @param {HTMLElement} anchor кнопка, от которой считаем
 * @param {{width: number, height: number, gap?: number, margin?: number}} menu
 *        габариты меню (width обязан совпадать с min-width в стилях), зазор до
 *        кнопки и поле до края окна
 * @returns {{style: object, openUp: boolean}} inline-стиль и сторона раскрытия
 */
export function anchoredMenuStyle(anchor, menu) {
  const { width, height, gap = 4, margin = 8 } = menu;
  const zoom = getViewportZoom();
  const rect = anchor.getBoundingClientRect();
  const top = rect.top / zoom;
  const bottom = rect.bottom / zoom;
  const right = rect.right / zoom;
  const vw = window.innerWidth / zoom;
  const vh = window.innerHeight / zoom;

  const spaceBelow = vh - bottom - gap - margin;
  const spaceAbove = top - gap - margin;
  const openUp = spaceBelow < height && spaceAbove > spaceBelow;

  const offsetRight = Math.min(
    Math.max(margin, vw - right),
    Math.max(margin, vw - width - margin)
  );

  return {
    openUp,
    style: {
      position: 'fixed',
      right: `${Math.round(offsetRight)}px`,
      // top:'auto' в ветке вверх обязателен: обе координаты сразу растянули бы меню.
      ...(openUp
        ? { bottom: `${Math.round(vh - top + gap)}px`, top: 'auto' }
        : { top: `${Math.round(bottom + gap)}px`, bottom: 'auto' }),
    },
  };
}
