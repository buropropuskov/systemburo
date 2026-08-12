/**
 * Роли участника заявки: машинные ключи приходят с бэкенда
 * (`internal/services/application_participants.go`), а подпись и оформление живут
 * здесь - переименование бейджа не должно быть правкой сервера.
 *
 * Отдельный модуль, а не константы внутри окна: список участников и карточка
 * участника рисуют одни и те же роли, и второй словарь разошёлся бы с первым на
 * первой же новой роли.
 *
 * Осторожно с парой acceptor/approver - в этом проекте она перепутана исторически:
 * принимающий (взял заявку в работу) - acceptor, согласующий (голосует) - approver.
 */

/** @type {Record<string, string>} */
const ROLE_LABELS = {
  sender: 'Отправитель',
  acceptor: 'Принимающий',
  approver: 'Согласующий',
  reader: 'Читатель',
};

/**
 * Вариант бейджа роли. Цвет несёт «насколько человек влияет на судьбу заявки»:
 * автор - акцентом, взявшие её в работу и голосующие - информационным, остальные
 * нейтральны. Голос согласующего окрашен ОТДЕЛЬНЫМ бейджем рядом, поэтому саму
 * роль в цвета голоса (success/danger/warning) не красим - иначе в одной строке
 * два разных смысла одного цвета.
 * @type {Record<string, string>}
 */
const ROLE_VARIANTS = {
  sender: 'primary',
  acceptor: 'info',
  approver: 'info',
  reader: 'neutral',
};

/** @type {Record<string, string>} */
const APPROVAL_VARIANTS = {
  approved: 'success',
  rejected: 'danger',
  pending: 'warning',
};

/**
 * Русская подпись роли. Неизвестный ключ (бэкенд завёл новую роль, фронт ещё не
 * знает) остаётся «Участником»: это правда о нём, в отличие от сырого `guest`.
 * @param {string|null|undefined} role
 * @returns {string}
 */
export function participantRoleLabel(role) {
  return ROLE_LABELS[role] || 'Участник';
}

/**
 * Вариант `Badge` для роли.
 * @param {string|null|undefined} role
 * @returns {string}
 */
export function participantRoleVariant(role) {
  return ROLE_VARIANTS[role] || 'neutral';
}

/**
 * Вариант `Badge` для голоса согласующего. Подпись берётся из общего словаря
 * `useApprovalStatus` - терминология голосов одна на всю заявку.
 * @param {string|null|undefined} status approved | rejected | pending
 * @returns {string}
 */
export function approvalBadgeVariant(status) {
  return APPROVAL_VARIANTS[status] || 'neutral';
}

/**
 * Видимое имя участника.
 *
 * У скрытого по ПД работника нет ни ФИО, ни контактов, и логин ему тоже не замена:
 * рабочий `i.ivanov` - это фамилия ничуть не меньше, чем поле «Фамилия», ради
 * которой его и прячут. Поэтому имени нет вовсе, а почему - говорит подпись рядом.
 *
 * @param {{full_name?: string, username?: string, pd_hidden?: boolean}} participant
 * @returns {string}
 */
export function participantDisplayName(participant) {
  if (participant?.pd_hidden) return 'Имя скрыто';
  return participant?.full_name || participant?.username || 'Без имени';
}

/**
 * Роли человека, кроме старшей: она уже нарисована бейджем, а терять остальные
 * нельзя - автор заявки часто оказывается ещё и согласующим.
 * @param {{roles?: string[], primary_role?: string}} participant
 * @returns {string[]} подписи ролей
 */
export function secondaryRoleLabels(participant) {
  const roles = participant?.roles || [];
  return roles.filter((role) => role !== participant?.primary_role).map(participantRoleLabel);
}
