/**
 * Заливка новой темы от точки клика (#1415).
 *
 * View Transitions API снимает кадр «до», применяет тему и отдаёт нам оба кадра
 * псевдоэлементами: старый лежит внизу, новый мы открываем растущим клипом от
 * курсора - экран заполняется новой темой, как жидкость из точки нажатия.
 * Штатную кроссфейд-анимацию гасит assets/theme-transition.css.
 *
 * Без поддержки API (или при prefers-reduced-motion) тема применяется мгновенно:
 * эффект украшение, а не условие работы.
 */

/** Длительность заливки. Ниже ~450мс жест читается как мигание, выше ~800 - тормозит. */
export const REVEAL_DURATION = 620;

/**
 * Фронт растекается не идеальным кругом: горизонтальная полуось разгоняется
 * раньше вертикальной, поэтому пятно сперва расплывается в стороны и лишь потом
 * добирает высоту. Две разные кривые по осям и дают «жидкость» вместо циркуля.
 */
const EASE_X = (t) => 1 - Math.pow(1 - t, 3.4);
const EASE_Y = (t) => 1 - Math.pow(1 - t, 2.2);
/** Шагов интерполяции: WAAPI линейно тянет между кадрами, 8 хватает для гладкости. */
const STEPS = 8;

/**
 * @typedef {{ x: number, y: number }} RevealOrigin Точка в координатах вьюпорта.
 */

/**
 * Читает точку нажатия из события. У клика с клавиатуры координаты нулевые -
 * тогда берём центр самого пункта, иначе заливка пошла бы из угла экрана.
 *
 * @param {MouseEvent|undefined} event
 * @returns {RevealOrigin|null}
 */
export function originFromEvent(event) {
  if (!event) return null;
  if (event.clientX || event.clientY) return { x: event.clientX, y: event.clientY };
  const rect = event.currentTarget?.getBoundingClientRect?.();
  if (!rect) return null;
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
}

/**
 * @returns {boolean} доступна ли анимированная заливка
 */
export function canReveal() {
  if (typeof document === 'undefined' || typeof document.startViewTransition !== 'function') {
    return false;
  }
  if (typeof document.documentElement.animate !== 'function') return false;
  return !window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches;
}

/**
 * Радиус, при котором пятно накрывает весь вьюпорт: расстояние от точки до
 * самого дальнего угла (по каждой оси берём дальнюю сторону).
 *
 * @param {RevealOrigin} origin
 * @param {number} width
 * @param {number} height
 */
function coverRadii({ x, y }, width, height) {
  return { rx: Math.max(x, width - x), ry: Math.max(y, height - y) };
}

/** @returns {string[]} кадры clip-path от точки до полного покрытия */
function clipFrames(origin, width, height) {
  const { rx, ry } = coverRadii(origin, width, height);
  const at = `at ${origin.x}px ${origin.y}px`;
  const frames = [];
  for (let i = 0; i <= STEPS; i += 1) {
    const t = i / STEPS;
    frames.push(`ellipse(${(rx * EASE_X(t)).toFixed(1)}px ${(ry * EASE_Y(t)).toFixed(1)}px ${at})`);
  }
  return frames;
}

/** Текущий переход: повторный клик по теме обрывает предыдущую заливку. */
let running = null;

/**
 * Применяет тему с заливкой от точки клика.
 *
 * @param {() => void} apply синхронная смена темы (data-theme + хранилище)
 * @param {RevealOrigin|null} [origin] точка нажатия; без неё - мгновенно
 * @returns {Promise<void>} завершение анимации (сразу, если её не было)
 */
export async function revealThemeChange(apply, origin) {
  if (!origin || !canReveal()) {
    apply();
    return;
  }

  // Незакрытый предыдущий переход держит на экране устаревший снимок.
  running?.skipTransition();

  let transition;
  try {
    transition = document.startViewTransition(apply);
  } catch {
    // Переход не поднялся (скрытая вкладка, чужой активный переход) - тема
    // важнее анимации, применяем напрямую: иначе клик остался бы без эффекта.
    apply();
    return;
  }
  running = transition;
  try {
    await transition.ready;
    await document.documentElement.animate(
      { clipPath: clipFrames(origin, window.innerWidth, window.innerHeight) },
      {
        duration: REVEAL_DURATION,
        easing: 'linear', // Кривые уже заложены в кадры, здесь они бы наложились.
        pseudoElement: '::view-transition-new(root)',
      },
    ).finished;
  } catch {
    // Прерванный или неподдержанный переход: тема уже применена коллбэком,
    // экран останется в новой теме без анимации.
  } finally {
    if (running === transition) running = null;
  }
}
