import { apiRequest } from '@/api/client';

/**
 * Отметка прохода на КПП и её отмена (#2437).
 *
 * Вынесено из таблиц: отметку ставят три компонента (люди, машины, машины «по
 * факту»), и каждый повторял свой вызов. Отмена прибавила бы к ним по третьему.
 */

/** Направление прохода в терминах API: 1 - вошёл/въехал, 2 - вышел/выехал. */
export const PASSAGE_STATUS = { entry: 1, exit: 2 };

/**
 * Готовые причины отмены. Свободный текст тоже принимается, но набор из трёх
 * закрывает почти все случаи, а на посту печатать некогда.
 */
export const PASSAGE_REVERT_REASONS = [
  'Отметил не того',
  'Не пропустили на территорию',
  'Отметка продублирована',
];

/**
 * Можно ли отменить последнюю отметку строки прямо сейчас.
 *
 * Само правило (своя, свежая, либо администратор) считает бэк и присылает
 * can_revert в текущем статусе - здесь только то, чего бэк знать не может:
 * открыт ли сейчас тот пост, на котором отметка поставлена.
 *
 * @param {object} item строка таблицы
 * @param {number|null} tableId пост, открытый в таблице
 * @returns {boolean}
 */
export function canRevertMark(item, tableId) {
  if (!item?.can_revert) return false;
  if (item.last_mark_table_id == null || tableId == null) return true;
  return item.last_mark_table_id === tableId;
}

/**
 * Направление последней отметки строки: вход отменяют, пока человек «на
 * территории», выход - пока «вышел».
 *
 * @param {object} item строка таблицы
 * @returns {'entry'|'exit'|null}
 */
export function lastMarkDirection(item) {
  if (item?.territory_status === 1) return 'entry';
  if (item?.territory_status === 2) return 'exit';
  return null;
}

/**
 * Ставит отметку прохода.
 *
 * Автора не передаём: сервер берёт его из токена (#2443). Раньше клиент присылал
 * `user_id`, и через консоль браузера проход записывался на чужую фамилию.
 *
 * @param {{kind: 'employees'|'cars', id: number, direction: 'entry'|'exit', tableId: number, pass?: object}} params
 * @returns {Promise<Response>}
 */
export function markPassage({ kind, id, direction, tableId, pass }) {
  const body = {
    territory_status: PASSAGE_STATUS[direction],
    table_id: tableId,
  };
  if (pass) body.pass = pass;
  return apiRequest(`/${kind}/${id}/territory-status`, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

/**
 * Отменяет последнюю отметку прохода.
 *
 * Автора отмены не передаём: его ставит сервер из токена, иначе правило «своя,
 * последняя, свежая» снималось бы подменой поля в запросе.
 *
 * @param {{kind: 'employees'|'cars', id: number, direction: 'entry'|'exit', tableId: number, reason: string}} params
 * @returns {Promise<{ok: boolean, error: string}>} error - текст отказа с бэка
 */
export async function revertPassage({ kind, id, direction, tableId, reason }) {
  const response = await apiRequest(`/${kind}/${id}/territory-status/revert`, {
    method: 'PUT',
    body: JSON.stringify({
      territory_status: PASSAGE_STATUS[direction],
      table_id: tableId,
      reason,
    }),
  });
  if (response.ok) return { ok: true, error: '' };

  // Отказы здесь осмысленные и адресованы человеку («поставил другой», «поздно»,
  // «отметка изменилась»), поэтому текст с бэка показываем как есть.
  let error = 'Не удалось отменить отметку';
  try {
    const body = await response.json();
    if (body?.error) error = body.error;
  } catch {
    // Тело не разобралось - остаётся общая формулировка выше.
  }
  return { ok: false, error };
}
