/**
 * Разбор поля `data` уведомления и сборка подробностей для модалки (#1748 S6).
 * До этого среза разбор JSON дублировался в UserNotifications.vue и
 * UserNotificationsInline.vue (только application_id для навигации) - здесь
 * единая точка + маппинг остальных полей data в подписи для модалки.
 */
import { formatDateTime } from '@/utils/datetime';

/** Русское склонение: 1 день, 2-4 дня, 5-20 дней, 21 день, 22-24 дня... */
function dayWord(n) {
  const mod100 = Math.abs(n) % 100;
  if (mod100 >= 11 && mod100 <= 14) return 'дней';
  const mod10 = Math.abs(n) % 10;
  if (mod10 === 1) return 'день';
  if (mod10 >= 2 && mod10 <= 4) return 'дня';
  return 'дней';
}

// Ключи data, показываемые в модалке подробностей, и их подписи. Порядок
// объекта = порядок вывода полей. Ключи вне маппинга (changed_by_user_id,
// question_id, application_id) намеренно не перечислены - технические
// идентификаторы человеку ничего не говорят, application_id уходит в кнопку
// действия, а не в список полей.
const FIELD_LABELS = {
  application_number: 'Заявка',
  forwarded_by: 'Передал',
  waiting_days: 'Ожидает решения',
  supplement_number: 'Дополнение',
  status: 'Решение',
  changed_at: 'Когда',
};

/**
 * Разбирает поле `data` уведомления в объект. `data` приходит строкой JSON,
 * уже объектом либо null/undefined в зависимости от пути (API отдаёт jsonb
 * строкой, локальные фикстуры и SSE-payload иногда кладут объект напрямую).
 * Битый JSON и любая нестандартная форма дают пустой объект, не исключение.
 * @param {{data?: string|object|null}|null|undefined} notification
 * @returns {Record<string, unknown>}
 */
export function parseNotificationData(notification) {
  let data = notification?.data;
  if (typeof data === 'string') {
    try {
      data = JSON.parse(data);
    } catch {
      return {};
    }
  }
  return data && typeof data === 'object' ? data : {};
}

/**
 * Список подробностей уведомления для модалки: `{ label, value }` по каждому
 * известному полю data, которое присутствует и непусто. Неизвестные поля и
 * технические идентификаторы (см. FIELD_LABELS) не попадают в результат.
 * @param {{data?: string|object|null}|null|undefined} notification
 * @returns {Array<{label: string, value: string}>}
 */
export function notificationDetailFields(notification) {
  const data = parseNotificationData(notification);
  const fields = [];
  for (const [key, label] of Object.entries(FIELD_LABELS)) {
    if (!(key in data)) continue;
    const raw = data[key];
    if (raw === null || raw === undefined || raw === '') continue;

    let value;
    if (key === 'waiting_days') {
      const n = Number(raw);
      if (!Number.isFinite(n)) continue;
      value = `${n} ${dayWord(n)}`;
    } else if (key === 'changed_at') {
      const formatted = formatDateTime(raw);
      if (!formatted) continue;
      value = formatted;
    } else {
      value = String(raw);
    }
    fields.push({ label, value });
  }
  return fields;
}

// Точные коды типов уведомлений, у которых категория расходится с префиксом
// "application_": все четыре - события проездной/пропускной логики, а не
// жизненного цикла заявки, хотя название типа начинается так же.
const EXACT_CATEGORY = {
  application_expiring: 'passage',
  application_withdrawn: 'passage',
  application_acceptor_assigned: 'passage',
  application_passage_first: 'passage',
  password_changed: 'security',
  user_banned: 'security',
  user_unbanned: 'security',
  login_blocked: 'security',
  role_changed: 'security',
  news_published: 'content',
  document_published: 'content',
  feedback_created: 'content',
  feedback_answered: 'content',
  maintenance_scheduled: 'system',
  trash_restored: 'system',
  archive_quota_warning: 'system',
  directory_entry_pending: 'system',
  directory_entry_resolved: 'system',
};

/**
 * Категория уведомления по коду типа - для визуальной группировки/бейджа в
 * модалке. Точное совпадение проверяется РАНЬШЕ префикса "application_":
 * application_expiring/withdrawn/acceptor_assigned/passage_first по имени
 * похожи на события заявки, но относятся к проезду (категория 'passage').
 * Любой не перечисленный код (включая настоящие application_* события
 * жизненного цикла заявки) - категория 'application'.
 * @param {string|null|undefined} type
 * @returns {'application'|'security'|'passage'|'content'|'system'}
 */
export function notificationCategory(type) {
  return (type && EXACT_CATEGORY[type]) || 'application';
}

/**
 * Подпись кнопки действия модалки - переход к заявке, когда data несёт
 * application_id. Без application_id действия нет (кнопка не рисуется).
 * @param {{data?: string|object|null}|null|undefined} notification
 * @returns {string|null}
 */
export function notificationActionLabel(notification) {
  const data = parseNotificationData(notification);
  return data.application_id ? 'Открыть заявку' : null;
}
