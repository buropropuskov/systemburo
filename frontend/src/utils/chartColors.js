/**
 * Цвета холста: чтение переменных темы и прозрачность.
 *
 * Живут отдельно от `statistics/useChartCanvas.js`, потому что тот при импорте
 * тянет и регистрирует Chart.js. Ручным холстам (лента запросов) библиотека не
 * нужна, а цвета темы нужны всем.
 */

/**
 * Составляющие цвета из шестнадцатеричной записи.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @returns {number[]|null} null для незнакомой записи
 */
function parseHex(color) {
  const hex = String(color ?? '').trim();
  const short = /^#([\da-f])([\da-f])([\da-f])$/i.exec(hex);
  const full = /^#([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i.exec(hex);
  if (!short && !full) return null;
  return short
    ? short.slice(1).map((c) => parseInt(c + c, 16))
    : full.slice(1).map((c) => parseInt(c, 16));
}

/**
 * Значение переменной темы у элемента.
 *
 * @param {HTMLElement|null|undefined} el элемент, с которого снимается тема
 * @param {string} name имя переменной
 * @param {string} fallback значение, когда темы нет
 * @returns {string}
 */
export function cssVariable(el, name, fallback) {
  const view = el?.ownerDocument?.defaultView;
  const value = view?.getComputedStyle?.(el)?.getPropertyValue?.(name);
  return value?.trim() || fallback;
}

/**
 * Цвет с заданной прозрачностью для градиентной заливки.
 *
 * Принимает шестнадцатеричную запись, которой пользуются вызывающие компоненты
 * (`color: '#4F5BDF'`). Незнакомую запись возвращает как есть: подмешать к ней
 * прозрачность нельзя, но и ронять график из-за оформления незачем.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @param {number} alpha прозрачность от 0 до 1
 * @returns {string}
 */
export function withAlpha(color, alpha) {
  const parts = parseHex(color);
  if (!parts) return String(color ?? '').trim();
  return `rgba(${parts.join(', ')}, ${alpha})`;
}

/**
 * Цвет, подмешанный к белому, - подсветка сегмента под курсором.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @param {number} amount доля белого от 0 до 1
 * @returns {string}
 */
export function lighten(color, amount) {
  const parts = parseHex(color);
  if (!parts) return String(color ?? '').trim();
  const mixed = parts.map((c) => Math.round(c + (255 - c) * amount));
  return `rgb(${mixed.join(', ')})`;
}

/**
 * Наблюдатель за сменой темы. Холст перерисовывает себя сам: тему переключает
 * атрибут `data-theme` на корне документа, и без этого график остаётся в
 * прежней палитре до следующего обновления данных.
 *
 * @param {HTMLElement|null|undefined} el элемент холста
 * @param {() => void} redraw перерисовка
 * @returns {MutationObserver|null} null там, где наблюдателя нет (юниты)
 */
export function watchTheme(el, redraw) {
  const root = el?.ownerDocument?.documentElement;
  const view = el?.ownerDocument?.defaultView;
  if (!root || typeof view?.MutationObserver !== 'function') return null;
  const observer = new view.MutationObserver(redraw);
  observer.observe(root, { attributes: true, attributeFilter: ['data-theme'] });
  return observer;
}
