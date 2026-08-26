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
  organization: 'От организации',
  sender_name: 'Отправитель',
  actor_name: 'Решение принял',
  decision_comment: 'Комментарий',
  forwarded_by: 'Передал',
  waiting_days: 'Ожидает решения',
  supplement_number: 'Дополнение',
  status: 'Решение',
  attempts: 'Неудачных попыток',
  locked_until: 'Вход открыт с',
  reason: 'Причина',
  role_name: 'Новая роль',
  news_title: 'Заголовок',
  document_name: 'Документ',
  changed_at: 'Когда',
  banned_at: 'Когда',
};

// Поля, значение которых открывает связанную сущность по клику. Пока такая
// сущность одна - заявка: номер ведёт туда же, куда кнопка действия, но кликнуть
// по самому номеру привычнее, чем искать кнопку внизу окна.
const FIELD_ACTIONS = {
  application_number: 'application',
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
 * @returns {Array<{key: string, label: string, value: string, action: string|null}>}
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
    } else if (key === 'changed_at' || key === 'banned_at' || key === 'locked_until') {
      const formatted = formatDateTime(raw);
      if (!formatted) continue;
      value = formatted;
    } else {
      value = String(raw);
    }
    fields.push({ key, label, value, action: FIELD_ACTIONS[key] || null });
  }
  return fields;
}

// Точные коды типов уведомлений, у которых категория расходится с префиксом
// "application_": все четыре - события проездной/пропускной логики, а не
// жизненного цикла заявки, хотя название типа начинается так же.
const EXACT_CATEGORY = {
  application_expiring: 'passage',
  application_withdrawn: 'passage',
  application_passage_first: 'passage',
  password_changed: 'security',
  user_banned: 'security',
  user_unbanned: 'security',
  login_blocked: 'security',
  // Новых уведомлений этого типа не создаётся (#974, убрано по решению владельца), но
  // ранее отправленные лежат в ленте до истечения срока хранения - без записи они
  // потеряли бы иконку и цвет категории.
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
  error_spike: 'system',
};

/**
 * Категория уведомления по коду типа - для визуальной группировки/бейджа в
 * модалке. Точное совпадение проверяется РАНЬШЕ префикса "application_":
 * application_expiring/withdrawn/passage_first по имени
 * похожи на события заявки, но относятся к проезду (категория 'passage').
 * Любой не перечисленный код (включая настоящие application_* события
 * жизненного цикла заявки) - категория 'application'.
 * @param {string|null|undefined} type
 * @returns {'application'|'security'|'passage'|'content'|'system'}
 */
export function notificationCategory(type) {
  return (type && EXACT_CATEGORY[type]) || 'application';
}

const CATEGORY_LABELS = {
  application: 'Заявка',
  security: 'Безопасность',
  passage: 'Проезд',
  content: 'Публикации',
  system: 'Система',
};

/**
 * Русская подпись категории - единая точка для бейджа в модалке (#1748 S6) и
 * цветовой метки в списке (#1748 S7), чтобы подписи не разъезжались по местам.
 * @param {'application'|'security'|'passage'|'content'|'system'} category
 * @returns {string}
 */
export function notificationCategoryLabel(category) {
  return CATEGORY_LABELS[category] || CATEGORY_LABELS.application;
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

function startOfDay(date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

/**
 * Разделитель дня для группировки уведомлений в списке: "Сегодня", "Вчера",
 * иначе дата ДД.ММ.ГГГГ. Момент группировки - тот же, что момент сортировки
 * ленты на бэке (последнее событие в схлопнутой группе). Поле называется
 * last_event_at либо updated_at в зависимости от того, как назвал его бэк
 * (см. #1748 S2/S7 - на момент написания ещё не смержен) - читаем оба имени,
 * иначе созданное раньше created_at.
 * @param {{created_at?: string, last_event_at?: string, updated_at?: string}|null|undefined} notification
 * @returns {string}
 */
export function notificationDayLabel(notification) {
  const value = notification?.last_event_at || notification?.updated_at || notification?.created_at;
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';

  const diffDays = Math.round((startOfDay(new Date()) - startOfDay(d)) / 86_400_000);
  if (diffDays === 0) return 'Сегодня';
  if (diffDays === 1) return 'Вчера';

  const pad = (n) => String(n).padStart(2, '0');
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
}

/**
 * Группирует УЖЕ отсортированную свежими-сверху ленту уведомлений по дню
 * (notificationDayLabel) для заголовков-разделителей "Сегодня"/"Вчера"/дата.
 * Сегментирует последовательные записи с одинаковой меткой, порядок внутри
 * дня и между днями не меняет - сортировку задаёт бэк (last_event_at).
 * @param {object[]} notifications
 * @returns {Array<{label: string, items: object[]}>}
 */
export function groupNotificationsByDay(notifications) {
  const groups = [];
  let current = null;
  for (const n of notifications || []) {
    const label = notificationDayLabel(n);
    if (!current || current.label !== label) {
      current = { label, items: [] };
      groups.push(current);
    }
    current.items.push(n);
  }
  return groups;
}
