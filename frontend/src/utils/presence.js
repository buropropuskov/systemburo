/**
 * presence.js - присутствие пользователя по users.last_seen (#1569).
 *
 * last_seen пишет бэк на каждом авторизованном запросе (internal/middleware/last_seen.go,
 * троттлинг 60с). Здесь то же окно онлайна, что у плитки дашборда и модалки «кто онлайн»
 * на бэке, чтобы таблица пользователей и статистика не давали разных ответов.
 */

import { formatDateTime, formatTimeAgo } from '@/utils/datetime.js';

/**
 * Окно «онлайн» в минутах. ДУБЛЬ серверной константы onlineWindowMinutes
 * (internal/services/statistics_service.go) - держится на клиенте, чтобы точка
 * присутствия гасла по таймеру, без запроса к бэку. Меняя здесь, менять и там.
 */
export const ONLINE_WINDOW_MINUTES = 5;

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

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
 * Компактная подпись присутствия под узкую колонку таблицы: «в сети», «12 мин»,
 * «2 ч», «3 дн», «-» если пользователь не заходил ни разу. Полная формулировка -
 * в seenTitle (подсказка ячейки), здесь ширины хватает только на число с единицей.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs
 * @returns {string}
 */
export function formatSeenShort(user, nowMs) {
  const ms = seenMs(user);
  if (ms === null) return '-';
  if (isOnline(user, nowMs)) return 'в сети';

  // Будущее значение (перекос часов клиента и сервера) читаем как «только что»,
  // иначе отрицательная разница дала бы «0 мин» и выглядела бы поломкой.
  const diff = Math.max(0, nowMs - ms);
  if (diff < HOUR_MS) return `${Math.max(1, Math.floor(diff / MINUTE_MS))} мин`;
  if (diff < DAY_MS) return `${Math.floor(diff / HOUR_MS)} ч`;
  return `${Math.floor(diff / DAY_MS)} дн`;
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
  // nowMs пробрасываем в formatTimeAgo: иначе подпись считалась бы от Date.now(),
  // а статус рядом - от тикающего presenceNow, и в одной ячейке разъезжались бы
  // «в сети» и «час назад».
  if (isOnline(user, nowMs)) return `В сети: ${formatTimeAgo(user.last_seen, nowMs)}`;
  return `Был в сети: ${formatTimeAgo(user.last_seen, nowMs)} (${formatDateTime(user.last_seen)})`;
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
