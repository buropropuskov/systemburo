/**
 * Типы справочников «Организации» и «Компании» (issue #1046).
 * Одни и те же 4 значения для обоих компонентов; NULL/пусто = «не указан».
 * Держим здесь, чтобы список, фильтр и подписи не разъехались между близнецами.
 */

/** @type {readonly string[]} Значения типа в порядке отображения (общие для орг и компаний). */
export const ORG_TYPES = ['Арендатор', 'Подрядчик', 'Отдел', 'Организация', 'Компания'];

/** Подпись для отсутствующего типа (NULL). */
export const ORG_TYPE_UNSPECIFIED_LABEL = 'не указан';

/** Сентинел значения «не указан» в дропдауне-фильтре шапки. */
export const ORG_TYPE_FILTER_UNSPECIFIED = '__none__';

/** Подпись фильтра шапки, когда не выбран ни один тип (#1398). */
export const ORG_TYPE_FILTER_ALL_LABEL = 'Тип: все';

/** Опции дропдауна создания: только 4 значения (тип обязателен). */
export const ORG_TYPE_CREATE_OPTIONS = ORG_TYPES.map((t) => ({ label: t, value: t }));

/** Опции дропдауна в панели деталей: 4 значения + «не указан» (снять тип, value=null). */
export const ORG_TYPE_DETAIL_OPTIONS = [
  ...ORG_TYPE_CREATE_OPTIONS,
  { label: ORG_TYPE_UNSPECIFIED_LABEL, value: null },
];

/**
 * Опции мультифильтра в шапке: значения + «не указан». Отдельной опции «все» нет -
 * её роль играет пустой выбор и встроенная строка «Сбросить выбор (N)».
 */
export const ORG_TYPE_FILTER_OPTIONS = [
  ...ORG_TYPES.map((t) => ({ label: t, value: t })),
  { label: ORG_TYPE_UNSPECIFIED_LABEL, value: ORG_TYPE_FILTER_UNSPECIFIED },
];

/**
 * Подпись типа для таблицы/деталей: пусто/NULL -> «не указан».
 * @param {string|null|undefined} type
 * @returns {string}
 */
export function orgTypeLabel(type) {
  return type || ORG_TYPE_UNSPECIFIED_LABEL;
}
