/**
 * Статусы раунда дополнения заявки (#1685) и их отображение. Зеркало
 * internal/models/status.go - сравнивать со строковыми литералами в компонентах нельзя,
 * иначе новый статус на бэке молча выпадет из части проверок.
 */

export const SUPPLEMENT_MERGED = 'merged';
export const SUPPLEMENT_PENDING = 'pending';
export const SUPPLEMENT_APPROVED = 'approved';
export const SUPPLEMENT_REJECTED = 'rejected';
export const SUPPLEMENT_ACCEPTED = 'accepted';
export const SUPPLEMENT_REFUSED = 'refused';
export const SUPPLEMENT_CANCELLED = 'cancelled';

/**
 * Незакрытый раунд - зеркало models.OpenSupplementStatuses. Такой у заявки максимум один
 * (партиальный уникальный индекс), и именно он блокирует подачу следующего дополнения.
 */
export const OPEN_SUPPLEMENT_STATUSES = [SUPPLEMENT_PENDING, SUPPLEMENT_APPROVED];

/**
 * Раунды, по которым голос ещё можно отозвать - зеркало supplementRevocableStatuses
 * бэка. Отклонённый раунд сюда входит: отзыв голоса открывает его заново.
 */
export const REVOCABLE_SUPPLEMENT_STATUSES = [
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
  return OPEN_SUPPLEMENT_STATUSES.includes(status);
}

const COUNT_NOUNS = {
  vehicles: ['машина', 'машины', 'машин'],
  employees: ['сотрудник', 'сотрудника', 'сотрудников'],
  items: ['позиция ТМЦ', 'позиции ТМЦ', 'позиций ТМЦ'],
};

/** Русское склонение по числу: 1 машина, 2-4 машины, 5+ машин (11-14 - третья форма). */
function plural(count, forms) {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
  return forms[2];
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
    .map(([count, forms]) => `${count} ${plural(count, forms)}`)
    .join(', ');
}
