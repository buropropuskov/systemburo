/**
 * Подписи к источнику привязки места прохода.
 *
 * Привязка появляется тремя путями, и по итоговому списку постов их не отличить:
 * человек видит «КПП №4» и не знает, сам он это указал, назначил принимающий или
 * добавили руками в таблице поста (#2549).
 */
export const PASSAGE_SOURCE_LABELS = {
  application: 'из заявки',
  approver: 'назначил принимающий',
  manual: 'добавлено вручную',
};

/** Подпись источника; пустая строка, когда источник неизвестен - подписывать нечего. */
export function passageSourceLabel(source) {
  return PASSAGE_SOURCE_LABELS[source] || '';
}

/** Вид значка: своё из заявки выделяем, добавленное потом - нейтрально. */
export function passageSourceVariant(source) {
  return source === 'application' ? 'primary' : 'neutral';
}
