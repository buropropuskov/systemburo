/**
 * Статусы раунда дополнения заявки (#1685) - зеркало констант бэкенда
 * (`internal/models/status.go`, `models.Supplement*`).
 *
 * Строки уже успели разъехаться по нескольким компонентам, каждый со своим литералом.
 * Сравнение со строкой ошибается молча: опечатка или переименование на сервере не ломает
 * сборку и не роняет тест, а просто перестаёт подсвечивать строки - причём в одном месте
 * перестаёт, а в другом нет. Поэтому значения живут здесь одним списком.
 */

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
