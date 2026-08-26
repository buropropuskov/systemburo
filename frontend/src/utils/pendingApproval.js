/**
 * Бейдж "ждёт согласования N дней" в списках заявок (#1315 S2, парный к
 * напоминаниям зависшим согласующим). Считается на фронте из полей строки списка:
 * заявка ждёт согласования, пока confirmation === 'Согласование', а срок ожидания -
 * дни с момента подачи. Используется в ApplicationsCenter и UserApplications.
 */

// БЕЛЫЙ список статусов, при которых заявка реально ждёт согласования (урок #1083:
// гейт вести белым списком активного допуска, а не чёрным перечнем терминальных).
// confirmation инициализируется 'Согласование' и остаётся им, пока согласование не
// завершено ИЛИ пока у заявки вовсе нет назначенных согласующих; в последнем случае
// её могут принять в работу (status 'В работе') с неснятым confirmation - тогда
// ждать уже нечего. Реально "на согласовании" заявка только на раннем этапе.
const AWAITING_STATUSES = ['Непрочитано', 'В обработке'];

/**
 * Число полных суток, что заявка ждёт согласования, либо null если не ждёт
 * (согласована/отклонена/закрыта или нет даты подачи). Свежие заявки текущего дня
 * дают 0 - бейдж по ним не рисуем (все свежие ждут, это шум), показываем с 1 дня.
 * @param {{confirmation?: string, status?: string, sending_datetime?: string}} application
 * @returns {number|null}
 */
export function pendingApprovalDays(application) {
  if (!application || application.confirmation !== 'Согласование') return null;
  if (!AWAITING_STATUSES.includes(application.status)) return null;
  if (!application.sending_datetime) return null;

  const sent = new Date(application.sending_datetime);
  if (Number.isNaN(sent.getTime())) return null;

  const days = Math.floor((Date.now() - sent.getTime()) / 86_400_000);
  return days > 0 ? days : null;
}

/** Русское склонение: 1 день, 2-4 дня, 5-20 дней, 21 день, 22-24 дня... */
function dayWord(n) {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 14) return 'дней';
  const mod10 = n % 10;
  if (mod10 === 1) return 'день';
  if (mod10 >= 2 && mod10 <= 4) return 'дня';
  return 'дней';
}

/**
 * Полный текст для подсказки (data-hint): "Ждёт согласования: N дней" со склонением.
 * @param {number} days
 * @returns {string}
 */
export function pendingApprovalLabel(days) {
  return `Ждёт согласования: ${days} ${dayWord(days)}`;
}

/**
 * Компактная надпись на самом бейдже: "N дн." - колонка тегов в Центре жёсткой
 * ширины (nowrap), полный текст туда не влезает рядом с другими тегами. Полное
 * пояснение уходит в data-hint (pendingApprovalLabel). "дн." без склонения -
 * общепринятое сокращение, годится для любого числа.
 * @param {number} days
 * @returns {string}
 */
export function pendingApprovalShort(days) {
  return `${days} дн.`;
}

/**
 * Заявка всё ещё ждёт согласования - то же условие, что у бейджа списков
 * (confirmation === 'Согласование' и статус в белом списке ожидающих). Отдельная
 * функция, потому что карточке заявки нужен этот гейт без расчёта дней от подачи.
 * @param {{confirmation?: string, status?: string}} application
 * @returns {boolean}
 */
export function isAwaitingApproval(application) {
  return !!application
    && application.confirmation === 'Согласование'
    && AWAITING_STATUSES.includes(application.status);
}

/** Русское склонение слова "раз": 1 раз, 2-4 раза, 5+ раз. */
function timesWord(n) {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 14) return 'раз';
  const mod10 = n % 10;
  if (mod10 >= 2 && mod10 <= 4) return 'раза';
  return 'раз';
}

/**
 * Число полных суток молчания согласующего - с момента его НАЗНАЧЕНИЯ (created_at
 * строки, не подача заявки: согласующего могли добавить позже, так же считает
 * ReminderService в BE). Возвращает null, если нет даты назначения или ещё нет
 * полных суток (в первый день "не отвечает 0 дней" - шум, метку не рисуем).
 * @param {{created_at?: string}} user
 * @returns {number|null}
 */
export function approverSilenceDays(user) {
  if (!user || !user.created_at) return null;
  const assigned = new Date(user.created_at);
  if (Number.isNaN(assigned.getTime())) return null;
  const days = Math.floor((Date.now() - assigned.getTime()) / 86_400_000);
  return days > 0 ? days : null;
}

/**
 * Текст метки молчащего согласующего для карточки заявки (#1315 S3):
 * "Не отвечает N дней" + ", напомнили K раз", если напоминания уже уходили.
 * @param {number} days
 * @param {number} reminderCount
 * @returns {string}
 */
export function approverSilenceLabel(days, reminderCount) {
  let text = `Не отвечает ${days} ${dayWord(days)}`;
  if (reminderCount > 0) {
    text += `, напомнили ${reminderCount} ${timesWord(reminderCount)}`;
  }
  return text;
}
