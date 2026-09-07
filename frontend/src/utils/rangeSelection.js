/**
 * Выбор диапазона строк с Shift - общая часть окон «Добавить существующую(-ие)»
 * для сотрудников и машин (#2399).
 *
 * Две тонкости, ради которых это вынесено из компонентов:
 *
 * 1. Диапазон считается по ОТОБРАЖЁННОМУ списку, а не по исходному набору. При
 *    активном поиске между якорем и целью на экране лежат совсем другие записи, и
 *    человек ждёт ровно то, что видит.
 * 2. Повторный Shift-клик от того же якоря ПЕРЕОПРЕДЕЛЯЕТ диапазон, а не копит его.
 *    Поэтому слияние идёт не с текущим выбором, а со снимком, сделанным в момент
 *    установки якоря: иначе промах на пару строк уже не отменить - лишние остались бы
 *    выбранными навсегда.
 */

/**
 * Элементы между якорем и целью в порядке отображения. Недоступные для выбора
 * пропускаются молча: они не должны ни попадать в выбор, ни обрывать диапазон.
 *
 * @param {Array<object>} visible список в том порядке, в каком он на экране
 * @param {number|string} anchorId идентификатор якорной строки
 * @param {number|string} targetId идентификатор строки, по которой кликнули с Shift
 * @param {{idKey?: string, isDisabled?: (item: object) => boolean}} [options]
 * @returns {Array<object>} пусто, если якорь или цель в списке не найдены
 */
export function itemsInRange(visible, anchorId, targetId, options = {}) {
  const { idKey = 'id', isDisabled = () => false } = options;
  const от = visible.findIndex((i) => i[idKey] === anchorId);
  const до = visible.findIndex((i) => i[idKey] === targetId);
  if (от === -1 || до === -1) return [];
  const [начало, конец] = от <= до ? [от, до] : [до, от];
  return visible.slice(начало, конец + 1).filter((i) => !isDisabled(i));
}

/**
 * Слияние снимка выбора с диапазоном без дублей и с сохранением порядка снимка.
 *
 * @param {Array<object>} base снимок на момент установки якоря
 * @param {Array<object>} added элементы диапазона
 * @param {string} [idKey]
 * @returns {Array<object>}
 */
export function mergeSelection(base, added, idKey = 'id') {
  const уже = new Set(base.map((i) => i[idKey]));
  return [...base, ...added.filter((i) => !уже.has(i[idKey]))];
}
