/**
 * Разбор маршрута тура: группировка шагов по разделам для списка в поповере и
 * нарезка на сегменты (подряд идущие шаги одной страницы). Общее для всех пяти
 * туров, поэтому лежит отдельно от самих шагов.
 */
import { MAIN_SECTIONS, ADMIN_GROUPS } from '@/constants/navSections';

/**
 * Человеческое имя раздела по его route - подпись группы в списке шагов тура.
 * Берём из навигации системы, чтобы названия совпадали с тем, что человек видит
 * в меню; для страниц вне меню (таблицы постов) отдаём запасное имя.
 *
 * @param {string} route
 * @returns {string}
 */
export function sectionTitleFor(route) {
  const inMain = MAIN_SECTIONS.find((s) => s.path === route);
  if (inMain) return inMain.label;
  for (const group of ADMIN_GROUPS) {
    const item = group.items.find((i) => i.path === route);
    if (item) return `${group.title}: ${item.label}`;
  }
  if (route === '/admin/settings') return 'Настройки Бюро';
  if (route?.startsWith('/table/')) return 'Таблица поста';
  return 'Раздел системы';
}

/**
 * Шаги тура, сгруппированные по разделам, - для списка «перейти к шагу». Считаем
 * от полного набора: пользователь должен видеть и то, что уже прошёл, и то, что
 * впереди. Выброшенные в этом прохождении шаги (их целей на экране нет) из списка
 * убираем - прыгнуть на них всё равно некуда.
 *
 * @param {Array<{route: string, title: string}>} steps
 * @param {Array<number>|Set<number>} [skipped] индексы выброшенных шагов
 * @returns {Array<{ route: string, title: string, items: Array<{ index: number, title: string }> }>}
 */
export function groupStepsBySection(steps, skipped = []) {
  const dropped = skipped instanceof Set ? skipped : new Set(skipped);
  const groups = [];
  steps.forEach((step, index) => {
    if (dropped.has(index)) return;
    const last = groups[groups.length - 1];
    if (last && last.route === step.route) last.items.push({ index, title: step.title });
    else groups.push({ route: step.route, title: sectionTitleFor(step.route), items: [{ index, title: step.title }] });
  });
  return groups;
}

/**
 * Подряд идущие шаги начиная с `startIndex`, чей `route` совпадает с активной
 * страницей. Граница сегмента - первый шаг с другим route (cross-page переход).
 *
 * @param {Array<{ route: string }>} steps
 * @param {number} startIndex глобальный индекс первого шага сегмента
 * @param {string} routePath активный путь роутера
 * @returns {Array<object>}
 */
export function collectSegment(steps, startIndex, routePath) {
  const segment = [];
  for (let i = startIndex; i < steps.length; i += 1) {
    if (steps[i].route !== routePath) break;
    segment.push(steps[i]);
  }
  return segment;
}

/**
 * Индекс первого шага ПОСЛЕ непрерывного блока шагов с данным `route`, начиная с
 * `fromIndex`. Нужен, чтобы перепрыгнуть недостижимый optional-сегмент
 * (фактовая таблица, роут-гард которой редиректит охранника) к следующему шагу
 * тура - финалу-празднованию на достижимой странице. Возвращает -1, если за
 * блоком шагов не осталось.
 *
 * @param {Array<{ route: string }>} steps
 * @param {number} fromIndex индекс первого шага недостижимого блока
 * @param {string} route route недостижимого блока
 * @returns {number} индекс следующего шага за блоком или -1
 */
export function indexAfterRoute(steps, fromIndex, route) {
  let i = fromIndex;
  while (i < steps.length && steps[i].route === route) i += 1;
  return i < steps.length ? i : -1;
}

/**
 * Главы тура - те же группы по разделам, что и в списке шагов, но с границами по
 * НАБОРУ (без учёта выброшенного), чтобы номер главы не менялся по ходу.
 *
 * Обучение заявителя идёт под шестьдесят шагов, минут семь подряд. Человеку нужно
 * знать, где он в этом пути и где ближайшее место, чтобы прерваться, - поэтому
 * поповер называет главу, а на её последнем шаге предлагает продолжить позже.
 *
 * @param {Array<{route: string}>} steps
 * @returns {Array<{ title: string, start: number, end: number }>} включительные границы
 */
export function tourChapters(steps) {
  const chapters = [];
  (steps || []).forEach((step, index) => {
    const last = chapters[chapters.length - 1];
    if (last && last.route === step.route) last.end = index;
    else chapters.push({ route: step.route, title: sectionTitleFor(step.route), start: index, end: index });
  });
  return chapters;
}

/**
 * В какой главе находится шаг.
 *
 * @param {Array<{route: string}>} steps
 * @param {number} index
 * @returns {{ title: string, number: number, total: number, start: number, end: number }|null}
 */
export function chapterOf(steps, index) {
  const chapters = tourChapters(steps);
  const at = chapters.findIndex((c) => index >= c.start && index <= c.end);
  if (at < 0) return null;
  return { ...chapters[at], number: at + 1, total: chapters.length };
}

/**
 * Последний ли это шаг главы - там уместно предложить прерваться.
 *
 * @param {Array<{route: string}>} steps
 * @param {number} index
 * @returns {boolean}
 */
export function isChapterEnd(steps, index) {
  const chapter = chapterOf(steps, index);
  return Boolean(chapter) && chapter.end === index && chapter.number < chapter.total;
}
