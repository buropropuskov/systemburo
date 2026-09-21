/**
 * Места прохода в карточке элемента: подписи источника и разбор двух форм ответа.
 *
 * Привязка появляется тремя путями, и по итоговому списку постов их не отличить:
 * человек видит «КПП №4» и не знает, сам он это указал при подаче, назначил принимающий
 * или добавили руками в таблице поста (#2551).
 *
 * Формы target_tables тоже две: в заявке приходят объекты с источником, в списках и
 * корзине - плоские идентификаторы. Источник не выдумывается: неизвестен - подписи нет.
 */

const PASSAGE_SOURCE_LABELS = {
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

/**
 * Активные места прохода с подписями. `getTableName` подставляет название, когда пришёл
 * голый идентификатор.
 */
export function activePassageTables(raw, getTableName) {
  return (raw || []).map((t) => {
    const плоский = typeof t === 'number';
    const id = плоский ? t : t.id;
    const source = плоский ? null : (t.source || null);
    return {
      id,
      name: (плоский ? null : t.name) || getTableName(id),
      source,
      sourceLabel: passageSourceLabel(source),
      sourceVariant: passageSourceVariant(source),
    };
  });
}

/**
 * Снятые места прохода - показываются зачёркнутыми. Берутся из истории привязок, поэтому
 * осмысленны только там, где известен источник: в заявке и корзине истории нет.
 * Активная привязка перекрывает снятую, повторные снятия одного поста не дублируются.
 */
export function removedPassageTables(history, active, getTableName) {
  if (!active.some((t) => t.source)) return [];
  const активные = new Set(active.map((t) => t.id));
  const виденные = new Set();
  const снятые = [];
  const записи = Array.isArray(history) ? history : [];
  записи.forEach((item) => {
    if (item.action_type !== 'unbound_from_table' && item.action_type !== 'moved_between_tables') return;
    const id = item.table_id;
    if (id == null || активные.has(id) || виденные.has(id)) return;
    виденные.add(id);
    снятые.push({ id, name: item.table_name || getTableName(id) });
  });
  return снятые;
}

/**
 * Подсказка к списку постов в ячейке карточки заявки: на подпись рядом с названием
 * места нет, поэтому источник живёт в подсказке.
 */
export function passageChipHint(items, names) {
  return items.map((item, i) => [names[i], passageSourceLabel(item.source)].filter(Boolean).join(' - ')).join(', ');
}
