/**
 * Наблюдение за шириной колонки тегов - вход для их свёртки (ApplicationTags).
 *
 * Замеряется ячейка ШАПКИ списка: она одна на таблицу и живёт весь срок компонента,
 * а ширину имеет ту же, что ячейки строк - у шапки и ряда общие padding и gap.
 * Меняется она и от вьюпорта, и от закрепления нав-меню, поэтому нужен наблюдатель,
 * а не разовый замер. В карточном режиме шапка скрыта, ширина остаётся нулевой - и
 * теги идут полным текстом, как и задумано для мобилки.
 *
 * Общая для Центра и личного кабинета (#2319): списки заявок в обоих разделах
 * сворачивают теги одинаково.
 *
 * @param {HTMLElement|null|undefined} el ячейка шапки колонки тегов
 * @param {(width: number) => void} onWidth вызывается при каждом изменении ширины
 * @returns {ResizeObserver|null} наблюдатель - вызывающий обязан disconnect'нуть его
 *   в beforeUnmount; null, если наблюдать нечего или среда без ResizeObserver (jsdom)
 */
export function observeTagsColumnWidth(el, onWidth) {
  if (!el || typeof ResizeObserver === 'undefined') return null;
  const observer = new ResizeObserver(([entry]) => onWidth(Math.round(entry.contentRect.width)));
  observer.observe(el);
  return observer;
}
