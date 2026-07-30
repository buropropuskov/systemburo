/**
 * presence.js - присутствие пользователя по users.last_seen (#1569).
 *
 * last_seen пишет бэк на каждом авторизованном запросе (internal/middleware/last_seen.go,
 * троттлинг 60с). Здесь то же окно онлайна, что у плитки дашборда и модалки «кто онлайн»
 * на бэке, чтобы таблица пользователей и статистика не давали разных ответов.
 */

import { formatDateTime } from '@/utils/datetime.js';

/**
 * Окно «онлайн» в минутах. ДУБЛЬ серверной константы onlineWindowMinutes
 * (internal/services/statistics_service.go) - держится на клиенте, чтобы точка
 * присутствия гасла по таймеру, без запроса к бэку. Меняя здесь, менять и там.
 */
export const ONLINE_WINDOW_MINUTES = 5;

const SECOND_MS = 1000;
const MINUTE_MS = 60 * SECOND_MS;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
// Месяц и год - приближения (30 и 365 дней): подпись отвечает на «как давно», а не
// на «какая была дата» - точный момент лежит в подсказке ячейки (seenTitle).
const MONTH_MS = 30 * DAY_MS;
const YEAR_MS = 365 * DAY_MS;

// Шкала единиц от старшей к младшей: на подпись идут ДВЕ старшие непустые.
const SEEN_UNITS = [
  { ms: YEAR_MS, label: 'г' },
  { ms: MONTH_MS, label: 'мес' },
  { ms: DAY_MS, label: 'дн' },
  { ms: HOUR_MS, label: 'ч' },
  { ms: MINUTE_MS, label: 'мин' },
  { ms: SECOND_MS, label: 'с' },
];

/**
 * Момент last_seen в миллисекундах. Пустое/невалидное значение -> null.
 * @param {{last_seen?: string|null}|null|undefined} user
 * @returns {number|null}
 */
function seenMs(user) {
  if (!user || !user.last_seen) return null;
  const ms = new Date(user.last_seen).getTime();
  return Number.isNaN(ms) ? null : ms;
}

/**
 * Онлайн ли пользователь. Повторяет серверный предикат onlineUserScope: активный,
 * не забанен и с активностью внутри окна. Забаненного отсекаем не только для
 * согласованности со счётчиком дашборда: BanCheck не даёт ему обновлять last_seen,
 * но свежая метка, записанная до блокировки, держала бы его «в сети» до конца окна.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs - «сейчас» в мс; передаётся снаружи, чтобы тикающий таймер
 *   компонента пересчитывал статус реактивно, а не через Date.now() внутри computed
 * @returns {boolean}
 */
export function isOnline(user, nowMs) {
  if (!user || user.is_active === false || user.is_banned === true) return false;
  const ms = seenMs(user);
  if (ms === null) return false;
  return nowMs - ms < ONLINE_WINDOW_MINUTES * MINUTE_MS;
}

/**
 * Сколько прошло с последней активности - двумя старшими единицами: «12 с»,
 * «3 мин 20 с», «2 ч 15 мин», «3 дн 4 ч», «5 мес 12 дн», «2 г 3 мес». «-» если
 * человек не заходил ни разу.
 *
 * Подпись даётся ВСЕМ, включая присутствующих: «в сети» словами занимало ту же
 * ячейку и прятало секунды с минутами (внутри окна онлайна время не показывалось
 * вовсе). Факт присутствия несёт точка рядом, а ячейка отвечает на «когда был
 * последний раз». Младшая единица опускается при нуле, чтобы не писать «2 ч 0 мин».
 *
 * Точность самой метки ограничена троттлингом записи last_seen (60 с на бэке),
 * поэтому секунды показывают «когда прошёл последний учтённый запрос», а не
 * тиканье в реальном времени.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs
 * @returns {string}
 */
export function formatSeenShort(user, nowMs) {
  const ms = seenMs(user);
  if (ms === null) return '-';

  // Будущее значение (перекос часов клиента и сервера) читаем как ноль, иначе
  // отрицательная разница дала бы «-3 мин» и выглядела бы поломкой.
  const diff = Math.max(0, nowMs - ms);

  const topIndex = SEEN_UNITS.findIndex(u => diff >= u.ms);
  // Меньше секунды - самая младшая единица с нулём, «0 с» честнее пустой ячейки.
  if (topIndex === -1) return '0 с';

  const top = SEEN_UNITS[topIndex];
  const topValue = Math.floor(diff / top.ms);
  const rest = diff - topValue * top.ms;

  const next = SEEN_UNITS[topIndex + 1];
  const nextValue = next ? Math.floor(rest / next.ms) : 0;

  return nextValue > 0
    ? `${topValue} ${top.label} ${nextValue} ${next.label}`
    : `${topValue} ${top.label}`;
}

/**
 * Полная подпись для title ячейки: относительное время плюс точная дата, чтобы
 * из «3 дн» можно было получить конкретный момент, не открывая карточку.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs
 * @returns {string}
 */
export function seenTitle(user, nowMs) {
  if (seenMs(user) === null) return 'Ни разу не заходил';
  // Относительную часть берём из formatSeenShort, а не из formatTimeAgo: у них разная
  // грануляция, и подсказка «11 мин назад» под ячейкой «11 мин 30 с» читалась бы как
  // расхождение. Точный момент рядом - из него видно, что метка та же.
  const ago = `${formatSeenShort(user, nowMs)} назад`;
  const exact = formatDateTime(user.last_seen);
  return isOnline(user, nowMs)
    ? `В сети. Последняя активность: ${ago} (${exact})`
    : `Был в сети: ${ago} (${exact})`;
}

/**
 * Ключ сортировки по свежести активности. Не заходившие получают -Infinity и
 * всегда собираются в одном конце списка, а не рассыпаются как нули.
 * @param {{last_seen?: string|null}|null|undefined} user
 * @returns {number}
 */
export function lastSeenSortKey(user) {
  const ms = seenMs(user);
  return ms === null ? -Infinity : ms;
}
