/**
 * Подписи событий в карточке истории машины и сотрудника.
 *
 * Словари были копиями внутри двух модалок, и новое действие приходилось дописывать
 * дважды - иначе в одной карточке событие показывалось по-русски, а в соседней сырым
 * кодом. Отсюда же берутся события отмены ошибочной отметки (#2437).
 */

/** Общие для машины и сотрудника действия: различаются только словом-подлежащим. */
function commonActions(subject) {
  return {
    create: `Подана заявка на ${subject === 'Автомобиль' ? 'автомобиль' : 'сотрудника'}`,
    update: 'Данные обновлены',
    delete: `${subject} удалён`,
    activate: `${subject} введён в работу`,
    deactivate: `${subject} выведен из работы`,
    restore: `${subject} восстановлен`,
    blacklisted: 'Добавлен в чёрный список',
    unblacklisted: 'Снят с чёрного списка',
    blacklist_override: 'Пропущен несмотря на подозрение в обходе ЧС',
    blacklist_override_revoke: 'Отменено подтверждение пропуска (обход ЧС)',
    added_to_table: 'Добавлен в таблицу проходной',
    moved_between_tables: 'Перенесён между таблицами',
    unbound_from_table: 'Снят с таблицы',
  };
}

export const CAR_HISTORY_ACTIONS = {
  ...commonActions('Автомобиль'),
  entry: 'Отметил о прибытии',
  exit: 'Машина уехала',
  entry_revert: 'Отметка о прибытии отменена',
  exit_revert: 'Отметка об убытии отменена',
};

export const EMPLOYEE_HISTORY_ACTIONS = {
  ...commonActions('Сотрудник'),
  entry: 'Проход на территорию',
  exit: 'Выход с территории',
  entry_revert: 'Отметка о проходе отменена',
  exit_revert: 'Отметка о выходе отменена',
};

/**
 * Подпись события. Правка поля показывается с именем поля, всё неизвестное - своим
 * кодом: молча пустая строка спрятала бы событие целиком.
 *
 * @param {{action_type: string, field_name?: string}} item запись истории
 * @param {Record<string, string>} actions словарь раздела
 * @returns {string}
 */
export function historyActionText(item, actions) {
  if (item?.action_type === 'update' && item.field_name) {
    return `Изменено поле "${item.field_name}"`;
  }
  return actions[item?.action_type] || item?.action_type || '';
}
