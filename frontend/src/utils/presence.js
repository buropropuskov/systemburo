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
// Сокращения с точкой - как принято в русском тексте («5 мес. 12 дн.»); секунды
// пишем «сек.», а не «с.», чтобы одиночная буква не читалась как другое сокращение.
const SEEN_UNITS = [
  { ms: YEAR_MS, label: 'г.' },
  { ms: MONTH_MS, label: 'мес.' },
  { ms: DAY_MS, label: 'дн.' },
  { ms: HOUR_MS, label: 'ч.' },
  { ms: MINUTE_MS, label: 'мин.' },
  { ms: SECOND_MS, label: 'сек.' },
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
 * Давность в двух старших единицах. Отдельно от formatSeenShort, потому что
 * подсказке цифры нужны и для присутствующих - в ячейке у них бейдж «Онлайн».
 * @param {number} diffMs
 * @returns {string}
 */
function formatAgo(diffMs) {
  // Будущее значение (перекос часов клиента и сервера) читаем как ноль, иначе
  // отрицательная разница дала бы «-3 мин.» и выглядела бы поломкой.
  const diff = Math.max(0, diffMs);

  const smallest = SEEN_UNITS[SEEN_UNITS.length - 1];
  const topIndex = SEEN_UNITS.findIndex(u => diff >= u.ms);
  // Меньше секунды - младшая единица с нулём, «0 сек.» честнее пустой ячейки.
  if (topIndex === -1) return `0 ${smallest.label}`;

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
 * Подпись ячейки для тех, кого сейчас нет в системе: сколько прошло с последней
 * активности («47 мин», «6 дн 21 ч», «2 г 3 мес»), «-» у тех, кто не заходил ни
 * разу. Присутствующим ячейка рисует бейдж «Онлайн» и эту подпись не зовёт -
 * цифры там не нужны, важен сам факт (они остаются в подсказке).
 *
 * Точность метки ограничена троттлингом записи last_seen (60 с на бэке), поэтому
 * секунды означают «когда прошёл последний учтённый запрос», а не тиканье в
 * реальном времени.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs
 * @returns {string}
 */
export function formatSeenShort(user, nowMs) {
  const ms = seenMs(user);
  if (ms === null) return '-';
  return formatAgo(nowMs - ms);
}

/**
 * Полная подпись для title ячейки: относительное время плюс точная дата, чтобы
 * из «3 дн» можно было получить конкретный момент, не открывая карточку.
 * @param {{last_seen?: string|null, is_active?: boolean, is_banned?: boolean}|null|undefined} user
 * @param {number} nowMs
 * @returns {string}
 */
export function seenTitle(user, nowMs) {
  const ms = seenMs(user);
  if (ms === null) return 'Ни разу не заходил';
  // Давность берём из formatAgo напрямую, не через formatSeenShort: у присутствующих
  // тот отдаёт «В сети», и подсказка вышла бы «В сети назад». Именно здесь цифры и
  // нужны - в ячейке у них только статус.
  const ago = `${formatAgo(nowMs - ms)} назад`;
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
