/**
 * Форма выреза подсветки и подъём цели над затемнением.
 *
 * driver.js 1.4 держит `stagePadding`/`stageRadius` только в глобальном конфиге -
 * в `DriveStep` их нет. Но читает оба на каждом кадре отрисовки, поэтому подмена
 * конфига в начале перехода застаёт и анимацию, и итоговый вырез.
 */

/**
 * Зазор и скругление выреза по умолчанию. С 5px мелкие цели (галочка согласия)
 * смотрелись обрезанными по краю выреза - «больше воздуха вокруг».
 */
export const STAGE_PADDING = 10;
export const STAGE_RADIUS = 30;

/**
 * Обводить ли цель встык - без зазора и скругления.
 *
 * Панель сквозного поиска и рельс навигации занимают экран во всю высоту (а
 * мобильный drawer - и во всю ширину). Общий зазор уводил их вырез за границу
 * окна: у рельса он начинался на `y = -10` и кончался на `910` при высоте окна
 * 900, то есть подсветка обрезалась краем экрана вместо того, чтобы обвести
 * панель. Скругление 30px рисовало круглые углы там, где сама панель прямая и
 * упирается в край.
 *
 * Судим по геометрии, а не по списку селекторов: цель, дотянувшаяся до
 * противоположных краёв окна, ведёт себя так на любой ширине - и на 1440, и на
 * 390, где во всю ширину растягивается уже панель поиска.
 *
 * @param {Element|null|undefined} element подсвечиваемая цель
 * @returns {boolean}
 */
export function isFlushTarget(element) {
  if (!element || typeof element.getBoundingClientRect !== 'function') return false;
  if (typeof window === 'undefined') return false;
  const r = element.getBoundingClientRect();
  if (!r.width || !r.height) return false;
  const spansHeight = r.top <= STAGE_PADDING && r.bottom >= window.innerHeight - STAGE_PADDING;
  const spansWidth = r.left <= STAGE_PADDING && r.right >= window.innerWidth - STAGE_PADDING;
  return spansHeight || spansWidth;
}

/**
 * Подогнать форму выреза под цель ПЕРЕД тем, как driver начнёт её обводить.
 *
 * Конфиг заменяется целиком, отсюда разлив поверх `getConfig()`: без него
 * потерялись бы хуки и шаги (тем же приёмом пользуется `setSteps`).
 *
 * @param {import('driver.js').Driver|null} driverObj
 * @param {Element|undefined} element цель шага (undefined у центр-модалки)
 */
export function applyStageShape(driverObj, element) {
  if (!driverObj) return;
  const flush = isFlushTarget(element);
  const stagePadding = flush ? 0 : STAGE_PADDING;
  const stageRadius = flush ? 0 : STAGE_RADIUS;
  const config = driverObj.getConfig();
  if (config.stagePadding === stagePadding && config.stageRadius === stageRadius) return;
  driverObj.setConfig({ ...config, stagePadding, stageRadius });
}

/**
 * Поднять доехавшую цель над затемнением.
 *
 * driver.js вешает свой класс в НАЧАЛЕ перехода, когда вырез ещё едет к новой
 * рамке, и цель успевала протыкать затемнение раньше подсветки. Свой класс
 * ставим по завершении перехода - см. `.ob-highlighted` в onboarding.css.
 *
 * @param {import('driver.js').Driver|null} driverObj
 */
export function raiseActiveHighlight(driverObj) {
  const active = driverObj?.getActiveElement?.();
  if (active && active.id !== 'driver-dummy-element') active.classList.add('ob-highlighted');
}
