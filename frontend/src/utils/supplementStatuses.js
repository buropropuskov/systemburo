/**
 * Статусы раунда дополнения заявки (#1685) - зеркало констант бэкенда
 * (`internal/models/status.go`, `models.Supplement*`).
 *
 * Строки уже успели разъехаться по нескольким компонентам, каждый со своим литералом.
 * Сравнение со строкой ошибается молча: опечатка или переименование на сервере не ломает
 * сборку и не роняет тест, а просто перестаёт подсвечивать строки - причём в одном месте
 * перестаёт, а в другом нет. Поэтому значения живут здесь одним списком.
 */

import { COUNT_NOUNS, pluralRu } from './entityCount';

/** Дополнение влито в текущий круг согласования: отдельного раунда у него нет. */
export const SUPPLEMENT_MERGED = 'merged';
/** Раунд ждёт голосов согласующих. */
export const SUPPLEMENT_PENDING = 'pending';
/** Раунд согласован и ждёт решения принимающего. */
export const SUPPLEMENT_APPROVED = 'approved';
/** Обязательный согласующий отказал. */
export const SUPPLEMENT_REJECTED = 'rejected';
/** Раунд принят: его строки активированы и видны на КПП. */
export const SUPPLEMENT_ACCEPTED = 'accepted';
/** Принимающий отказал. */
export const SUPPLEMENT_REFUSED = 'refused';
/** Раунд снят автором либо системой при закрытии заявки. */
export const SUPPLEMENT_CANCELLED = 'cancelled';

/**
 * Раунд закрыт отрицательным решением: строки так и не попали на КПП и уже не попадут.
 * Отличается от `accepted` и `merged` - те тоже терминальны, но означают допуск.
 */
export const SUPPLEMENT_CLOSED_STATUSES = [
    SUPPLEMENT_REJECTED,
    SUPPLEMENT_REFUSED,
    SUPPLEMENT_CANCELLED,
];

/**
 * Раунд ещё не закрыт: идёт согласование либо ждём принимающего.
 */
export const SUPPLEMENT_OPEN_STATUSES = [SUPPLEMENT_PENDING, SUPPLEMENT_APPROVED];

/**
 * Раунды, по которым голос ещё можно отозвать. Отклонённый сюда входит намеренно:
 * отзыв голоса открывает круг заново.
 */
export const SUPPLEMENT_REVOCABLE_STATUSES = [
    SUPPLEMENT_PENDING,
    SUPPLEMENT_APPROVED,
    SUPPLEMENT_REJECTED,
];


const STATUS_TEXT = {
  [SUPPLEMENT_MERGED]: 'Влито в заявку',
  [SUPPLEMENT_PENDING]: 'На согласовании',
  [SUPPLEMENT_APPROVED]: 'Согласовано',
  [SUPPLEMENT_REJECTED]: 'Отказано в согласовании',
  [SUPPLEMENT_ACCEPTED]: 'Принято',
  [SUPPLEMENT_REFUSED]: 'Отказано',
  [SUPPLEMENT_CANCELLED]: 'Снято',
};

const STATUS_CLASS = {
  [SUPPLEMENT_MERGED]: 'supplement-status--neutral',
  [SUPPLEMENT_PENDING]: 'supplement-status--pending',
  [SUPPLEMENT_APPROVED]: 'supplement-status--approved',
  [SUPPLEMENT_REJECTED]: 'supplement-status--rejected',
  [SUPPLEMENT_ACCEPTED]: 'supplement-status--accepted',
  [SUPPLEMENT_REFUSED]: 'supplement-status--rejected',
  [SUPPLEMENT_CANCELLED]: 'supplement-status--neutral',
};

/**
 * Подпись статуса раунда.
 * @param {string|null|undefined} status
 * @returns {string}
 */
export function supplementStatusText(status) {
  return STATUS_TEXT[status] || 'Неизвестно';
}

/**
 * CSS-класс бейджа статуса раунда; сами классы объявляет компонент-потребитель.
 * @param {string|null|undefined} status
 * @returns {string}
 */
export function supplementStatusClass(status) {
  return STATUS_CLASS[status] || 'supplement-status--neutral';
}

/**
 * Раунд ещё идёт (ждёт голосов или решения принимающего).
 * @param {string|null|undefined} status
 * @returns {boolean}
 */
export function isOpenSupplement(status) {
  return SUPPLEMENT_OPEN_STATUSES.includes(status);
}

/**
 * Состав раунда одной строкой: «2 машины, 1 сотрудник». Нулевые типы опускаем -
 * «0 машин» в перечислении добавленного не несёт смысла.
 * @param {{vehicles?: number, employees?: number, items?: number}} counts
 * @returns {string} пустая строка, если раунд пуст
 */
export function supplementCountsLabel(counts) {
  return Object.entries(COUNT_NOUNS)
    .map(([key, forms]) => [Number(counts?.[key]) || 0, forms])
    .filter(([count]) => count > 0)
    .map(([count, forms]) => `${count} ${pluralRu(count, forms)}`)
    .join(', ');
}
