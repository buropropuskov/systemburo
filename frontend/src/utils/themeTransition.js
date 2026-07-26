/**
 * Заливка новой темы волной от точки клика (#1415).
 *
 * View Transitions API снимает кадр «до», применяет тему и отдаёт оба кадра
 * псевдоэлементами: старый лежит неподвижно, новый открывается растущей областью.
 * Штатный кроссфейд гасит assets/theme-transition.css.
 *
 * ФОРМА - ВОЛНА, ИДУЩАЯ ПО ЭКРАНУ. Радиальные версии («растущий круг от
 * курсора») юзер забраковал: круг раздувается, а в конце рывком добирает
 * площадь у дальнего угла. Здесь фронт - вертикальная волнистая кромка: от
 * точки клика она уходит вправо через весь экран (и коротким ходом влево, до
 * левого края), профиль по вертикали - сумма двух синусов, которые бегут по
 * высоте, поэтому кромка живёт, как поверхность бегущей волны.
 *
 * Кромка намеренно РЕЗКАЯ: размытие маской читалось как смазанное пятно, у
 * жидкости граница чёткая. Заодно дешевле - маску не растрируем каждый кадр.
 *
 * КООРДИНАТЫ - LAYOUT-PX. На экранах шире 1440 приложение масштабирует корень
 * через CSS `zoom` (utils/viewportScale.js), а clip-path внутри корня считается
 * в layout-px, тогда как `clientX/clientY` и `innerWidth/innerHeight` отдают
 * device-px. Без деления на zoom фигура выходит в zoom раз больше экрана: на
 * 2880x1620 (zoom 1.8) волна пробегала экран за первые кадры, юзер видел
 * заливку только у точки клика («занимает 1/4 экрана»).
 *
 * Без поддержки API (или при prefers-reduced-motion) тема применяется мгновенно:
 * эффект украшение, а не условие работы.
 */
import { getViewportZoom } from '@/utils/viewportScale';

/** Длительность прохода волны: ниже ~700мс смазывается, выше ~1100 тянется. */
export const REVEAL_DURATION = 880;

/** Кадров интерполяции: WAAPI тянет между ними линейно. */
const STEPS = 26;
/** Строк выборки по Y: по ним считается волнистая кромка фронта. */
const ROWS = 48;
/**
 * Амплитуда волны, px. Это главная примета: на весь экран 20px читаются как
 * прямая линия, поэтому берём заметные 58.
 */
const WAVE_AMPLITUDE = 58;
/** Запас за край экрана: последний кадр обязан накрыть экран целиком. */
const OVERSHOOT = 24;
/**
 * Волны кромки: длина в высотах экрана, вес, фаза, скорость бега. Разные знаки
 * скорости - гребни расходятся, и кромка не выглядит ровной гармошкой.
 */
const WAVES = [
  { periods: 1.3, weight: 0.62, phase: 0.5, speed: 2.2 },
  { periods: 2.4, weight: 0.38, phase: 2.4, speed: -1.5 },
];

/**
 * Ход фронта вправо: почти линейный, с мягким торможением у края. Проверено
 * замером покрытия по кадрам - прирост ровный от начала до конца.
 */
function sweepProgress(t) {
  const u = Math.min(1, Math.max(0, t));
  return Math.pow(u, 0.88);
}
/** Ход влево: путь короткий (клик у левого края), поэтому закрываем его быстрее. */
const BACK_PROGRESS = (t) => Math.min(1, sweepProgress(t) * 2.6);
/** Волна гаснет только к самому концу, иначе её видно лишь в начале. */
const WAVE_DECAY = (p) => Math.pow(1 - p, 0.75);

/**
 * @typedef {{ x: number, y: number }} RevealOrigin Точка в координатах вьюпорта.
 */

/**
 * Читает точку нажатия из события и переводит её в layout-px (см. заголовок
 * файла про zoom). У клика с клавиатуры координаты нулевые - тогда берём центр
 * самого пункта, иначе волна пошла бы от края экрана.
 *
 * @param {MouseEvent|undefined} event
 * @returns {RevealOrigin|null}
 */
export function originFromEvent(event) {
  if (!event) return null;
  const z = getViewportZoom() || 1;
  if (event.clientX || event.clientY) return { x: event.clientX / z, y: event.clientY / z };
  const rect = event.currentTarget?.getBoundingClientRect?.();
  if (!rect) return null;
  return { x: (rect.left + rect.width / 2) / z, y: (rect.top + rect.height / 2) / z };
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

/** Смещение кромки в строке y (доля высоты) в момент t. */
function edgeWave(ratio, t) {
  let sum = 0;
  for (const w of WAVES) {
    sum += w.weight * Math.sin(ratio * w.periods * Math.PI * 2 + w.phase + w.speed * t);
  }
  return sum;
}

/**
 * Кадры области заливки. Каждый кадр - многоугольник: сверху вниз идёт правая
 * (волнистая) кромка, снизу вверх - левая. Число точек во всех кадрах одинаковое,
 * только тогда браузер интерполирует polygon() плавно.
 *
 * @param {RevealOrigin} origin точка клика в layout-px
 * @param {number} width ширина вьюпорта в layout-px
 * @param {number} height высота вьюпорта в layout-px
 */
function fillFrames(origin, width, height) {
  const toRight = width - origin.x + OVERSHOOT + WAVE_AMPLITUDE;
  const toLeft = origin.x + OVERSHOOT + WAVE_AMPLITUDE;
  const frames = [];

  for (let i = 0; i <= STEPS; i += 1) {
    const t = i / STEPS;
    const forward = sweepProgress(t);
    const back = BACK_PROGRESS(t);
    const amp = WAVE_AMPLITUDE * WAVE_DECAY(forward);
    const rightBase = origin.x + toRight * forward;
    const leftBase = origin.x - toLeft * back;

    const right = [];
    const left = [];
    for (let r = 0; r < ROWS; r += 1) {
      const y = (height * r) / (ROWS - 1);
      const ratio = y / height;
      // Волна на правой кромке; на левой - в противофазе и слабее, чтобы обе жили.
      right.push(`${(rightBase + amp * edgeWave(ratio, t)).toFixed(1)}px ${y.toFixed(1)}px`);
      left.push(`${(leftBase - amp * edgeWave(ratio + 0.5, t) * 0.6).toFixed(1)}px ${y.toFixed(1)}px`);
    }

    frames.push(`polygon(${[...right, ...left.reverse()].join(',')})`);
  }
  return frames;
}

/** Текущий переход: повторный клик по теме обрывает предыдущую заливку. */
let running = null;

/**
 * Класс на <html> на время заливки. Гасит CSS-переходы: иначе новый кадр
 * рисуется с недоигранными переходами (раскрывающийся список в меню попал в
 * кадр схлопнутым), да и сотня одновременных color-переходов под фронтом
 * читается грязью. Правило - в assets/theme-transition.css.
 */
const REVEAL_CLASS = 'theme-reveal';

/**
 * Применяет тему с заливкой волной от точки клика.
 *
 * @param {() => void} apply синхронная смена темы (data-theme + хранилище)
 * @param {RevealOrigin|null} [origin] точка нажатия в layout-px; без неё - мгновенно
 * @returns {Promise<void>} завершение анимации (сразу, если её не было)
 */
export async function revealThemeChange(apply, origin) {
  if (!origin || !canReveal()) {
    apply();
    return;
  }

  // Незакрытый предыдущий переход держит на экране устаревший снимок.
  running?.skipTransition();

  const root = document.documentElement;
  root.classList.add(REVEAL_CLASS);
  let transition;
  try {
    transition = document.startViewTransition(apply);
  } catch {
    // Переход не поднялся (скрытая вкладка, чужой активный переход) - тема
    // важнее анимации, применяем напрямую: иначе клик остался бы без эффекта.
    root.classList.remove(REVEAL_CLASS);
    apply();
    return;
  }
  running = transition;
  try {
    await transition.ready;
    const z = getViewportZoom() || 1;
    const frames = fillFrames(origin, window.innerWidth / z, window.innerHeight / z);
    await root.animate(
      { clipPath: frames },
      {
        duration: REVEAL_DURATION,
        // Кривые уже разложены по кадрам, здесь линейно - иначе наложатся.
        easing: 'linear',
        pseudoElement: '::view-transition-new(root)',
      },
    ).finished;
  } catch {
    // Прерванный или неподдержанный переход: тема уже применена коллбэком,
    // экран останется в новой теме без анимации.
  } finally {
    root.classList.remove(REVEAL_CLASS);
    if (running === transition) running = null;
  }
}
