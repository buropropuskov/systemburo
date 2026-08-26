/**
 * Отображение голоса согласующего - общий словарь для согласования самой заявки
 * (ApplicationConfirmation) и для раундов дополнения (SupplementPanel, #1685).
 *
 * Вынесено из ApplicationConfirmation: у обоих списков голосующих одни и те же три
 * состояния, и второй словарь разошёлся бы с первым на первом же новом статусе.
 *
 * Пустой статус остаётся «неизвестным», а не «ожиданием»: приводить null к pending -
 * дело потребителя (в детали заявки это делает нормализация responsible_users, в
 * раундах дополнения - сама панель), и словарь не должен решать это за него.
 */

const STATUS_TEXT = {
  approved: 'Согласовано',
  rejected: 'Отказано',
  pending: 'Ожидание',
};

const STATUS_CLASS = {
  approved: 'status-approved',
  rejected: 'status-rejected',
  pending: 'status-pending',
};

/**
 * Подпись голоса согласующего.
 * @param {string|null|undefined} status approved | rejected | pending
 * @returns {string}
 */
export function approvalStatusText(status) {
  return STATUS_TEXT[status] || 'Неизвестно';
}

/**
 * CSS-класс бейджа голоса. Сами классы (status-approved/rejected/pending/default)
 * объявляет компонент-потребитель - словарь задаёт только соответствие статусу.
 * @param {string|null|undefined} status
 * @returns {string}
 */
export function approvalStatusClass(status) {
  return STATUS_CLASS[status] || 'status-default';
}

/**
 * Хелперы для Options API: `setup() { return useApprovalStatus() }` открывает
 * getStatusText/getStatusClass шаблону под теми же именами, что были методами.
 * @returns {{getStatusText: typeof approvalStatusText, getStatusClass: typeof approvalStatusClass}}
 */
export function useApprovalStatus() {
  return { getStatusText: approvalStatusText, getStatusClass: approvalStatusClass };
}
