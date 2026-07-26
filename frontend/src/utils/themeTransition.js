/**
 * Заливка новой темы от точки клика (#1415).
 *
 * View Transitions API снимает кадр «до», применяет тему и отдаёт оба кадра
 * псевдоэлементами: старый лежит неподвижно, новый открывается растущим от
 * курсора пятном. Штатный кроссфейд гасит assets/theme-transition.css.
 *
 * Жидкость получается из четырёх вещей, и все четыре важны:
 *   1. РОВНОЕ ПО ОЩУЩЕНИЮ ЗАПОЛНЕНИЕ. Площадь пятна растёт как r², поэтому
 *      кривая радиуса - степень 0.75 (`radiusAt`): фронт идёт от курсора сразу и
 *      тормозит у краёв. Проверено замером покрытия экрана по кадрам.
 *   2. НЕРОВНАЯ КРОМКА. Радиус по углу гуляет суммой трёх НИЗКОЧАСТОТНЫХ волн,
 *      они медленно проворачиваются, амплитуда гаснет к концу - контур дышит, как
 *      поверхность натёкшей лужи, и приходит к кругу, накрывающему экран.
 *   3. ПЕРЕКОС ПО ГРАВИТАЦИИ. Вниз пятно растекается охотнее, чем вверх
 *      (`gravityFactor`) - без этого симметричный круг читается как циркуль.
 *   4. РАЗМЫТЫЙ КРАЙ. Поверх клипа идёт радиальная маска: кромка размывается на
 *      долю радиуса (не на фиксированные пиксели - на пятне во весь экран их не
 *      видно) и перед прозрачностью идёт полупрозрачная полоса, поэтому фронт
 *      читается как смоченная граница, а не как вырезанная ножницами дуга.
 *
 * Без поддержки API (или при prefers-reduced-motion) тема применяется мгновенно:
 * эффект украшение, а не условие работы.
 */

/**
 * Длительность заливки = сколько жидкость течёт через экран: ниже ~600мс жест
 * смазывается, выше ~1100 начинает тянуться.
 */
export const REVEAL_DURATION = 880;

/**
 * Кадров интерполяции: WAAPI тянет между ними линейно. 22 хватает на всю
 * заливку, а каждый лишний кадр - это лишний пересчёт контура на 56 точек.
 */
const STEPS = 22;
/**
 * Точек контура. 56 - компромисс: волны читаются гладкими, а клип остаётся
 * дешёвым (замер стоимости кадра против пустой opacity-анимации: +2мс).
 */
const POINTS = 56;
/**
 * Размытие кромки берём долей радиуса, а не фиксированными пикселями: на пятне
 * в 900px 44px не видно вовсе, и кромка снова читается как вырезанная дуга.
 */
const FEATHER_RATIO = 0.075;
const FEATHER_MIN = 26;
const FEATHER_MAX = 110;
/** Амплитуда волн на кромке в долях радиуса (на старте; к концу гаснет до нуля). */
const WAVE_AMPLITUDE = 0.07;
/**
 * Волны НИЗКОЧАСТОТНЫЕ: два-три лепестка на весь контур. Мелкая рябь на пятне
 * во весь экран не видна, а крупные наплывы читаются как растекающаяся лужа.
 */
const WAVES = [
  { lobes: 2, weight: 0.5, phase: 0.6, spin: 0.9 },
  { lobes: 3, weight: 0.34, phase: 2.4, spin: -0.7 },
  { lobes: 5, weight: 0.16, phase: 4.2, spin: 0.45 },
];
/** Насколько пятно тянет вниз (по гравитации) и придерживает верх. */
const GRAVITY_DOWN = 0.13;
const GRAVITY_UP = 0.06;

/**
 * Ход фронта. Мерил покрытие экрана по кадрам: у симметричной кривой
 * (easeInOutSine) первые 150мс не видно ничего, а после 660мс уже нечего
 * заливать - клик отвечает с задержкой, а хвост анимации пустой. Пятно растёт
 * по площади ~ r², поэтому ровное по ощущению заполнение даёт степень 0.75:
 * фронт сразу идёт от курсора и замедляется к краям, как растекающаяся лужа.
 * Затравка в 22px - чтобы под курсором мгновенно появилась капля, а не точка.
 */
const RADIUS_POWER = 0.75;
const SEED_RADIUS = 22;
const radiusAt = (t, rMax) => SEED_RADIUS + (rMax - SEED_RADIUS) * Math.pow(t, RADIUS_POWER);
/**
 * Затухание волн: гаснут медленно (иначе к середине контур снова круг), но к
 * самому концу обязаны сойти в ноль - последний кадр должен накрыть экран без
 * зазоров во впадинах.
 */
const WAVE_DECAY = (t) => Math.pow(1 - t, 0.85);
/**
 * Ранняя сплюснутость: жидкость сперва расплывается в стороны, поэтому по
 * вертикали пятно на старте короче, к середине разница уходит.
 */
const SQUASH = (t) => 0.82 + 0.18 * Math.min(1, t * 1.6);
/**
 * Смещение по гравитации: вниз пятно уходит охотнее, чем вверх. Нарастает за
 * первую треть (иначе капля стартует уже перекошенной) и тоже гаснет к концу.
 */
function gravityFactor(angle, t) {
  const ramp = Math.min(1, t * 3) * WAVE_DECAY(t);
  const down = Math.sin(angle); // y вниз: sin>0 - нижняя половина контура
  return 1 + ramp * (down > 0 ? GRAVITY_DOWN * down : GRAVITY_UP * down);
}

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
 * Радиус, накрывающий вьюпорт из точки: до самого дальнего угла. С запасом на
 * сплюснутость и на впадины волн, иначе в последний кадр останется щель.
 *
 * @param {RevealOrigin} origin
 * @param {number} width
 * @param {number} height
 */
function coverRadius({ x, y }, width, height) {
  const dx = Math.max(x, width - x);
  const dy = Math.max(y, height - y);
  // Запас 1%: к последнему кадру волны и перекос уже погасли, поэтому радиуса
  // до дальнего угла достаточно - большой запас лишь съедал бы конец анимации.
  return Math.hypot(dx, dy) * 1.01;
}

/** Смещение волн для угла: сумма трёх лепестковых гармоник в момент t. */
function waveOffset(angle, t) {
  let sum = 0;
  for (const w of WAVES) sum += w.weight * Math.sin(w.lobes * angle + w.phase + w.spin * t);
  return sum;
}

/**
 * Кадры контура: многоугольник по POINTS точкам вокруг точки клика. Число точек в
 * кадрах одинаковое - только тогда браузер интерполирует polygon() плавно.
 */
function polygonFrames(origin, width, height) {
  const rMax = coverRadius(origin, width, height);
  const frames = [];
  for (let i = 0; i <= STEPS; i += 1) {
    const t = i / STEPS;
    const radius = radiusAt(t, rMax);
    const amp = WAVE_AMPLITUDE * WAVE_DECAY(t);
    const squash = SQUASH(t);
    const points = [];
    for (let p = 0; p < POINTS; p += 1) {
      const angle = (p / POINTS) * Math.PI * 2;
      const r = radius * (1 + amp * waveOffset(angle, t)) * gravityFactor(angle, t);
      const px = origin.x + r * Math.cos(angle);
      const py = origin.y + r * Math.sin(angle) * squash;
      points.push(`${px.toFixed(1)}px ${py.toFixed(1)}px`);
    }
    frames.push(`polygon(${points.join(',')})`);
  }
  return frames;
}

/**
 * Кадры маски: радиальный градиент того же радиуса с размытой кромкой. Перед
 * прозрачностью идёт полоса неполной непрозрачности - через неё просвечивает
 * старая тема, и фронт читается как смоченная граница, а не как срез.
 * Держим много кадров: тогда даже без интерполяции градиентов ход гладкий.
 */
function maskFrames(origin, width, height) {
  const rMax = coverRadius(origin, width, height);
  const at = `at ${origin.x}px ${origin.y}px`;
  const frames = [];
  for (let i = 0; i <= STEPS; i += 1) {
    const t = i / STEPS;
    // Маску растим с запасом: она обязана обгонять контур, иначе размытие
    // съедало бы кромку многоугольника и пятно отставало бы от расчёта.
    const front = radiusAt(t, rMax);
    const feather = Math.min(FEATHER_MAX, Math.max(FEATHER_MIN, front * FEATHER_RATIO));
    const radius = Math.max(1, front * (1 + WAVE_AMPLITUDE + GRAVITY_DOWN) + feather);
    const solid = Math.max(0, 100 - (feather / radius) * 100);
    const wet = solid + (100 - solid) * 0.45;
    frames.push(
      `radial-gradient(${radius.toFixed(1)}px ${radius.toFixed(1)}px ${at}, `
        + `#000 ${solid.toFixed(1)}%, rgba(0, 0, 0, 0.72) ${wet.toFixed(1)}%, `
        + `rgba(0, 0, 0, 0) 100%)`,
    );
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
    const { innerWidth: w, innerHeight: h } = window;
    const timing = {
      duration: REVEAL_DURATION,
      // Кривые уже разложены по кадрам, здесь линейно - иначе наложатся.
      easing: 'linear',
      pseudoElement: '::view-transition-new(root)',
    };
    const contour = root.animate({ clipPath: polygonFrames(origin, w, h) }, timing);
    // Маска идёт отдельной анимацией того же тайминга: одна анимация не может
    // вести два свойства с разными наборами кадров.
    root.animate({ maskImage: maskFrames(origin, w, h) }, timing);
    await contour.finished;
  } catch {
    // Прерванный или неподдержанный переход: тема уже применена коллбэком,
    // экран останется в новой теме без анимации.
  } finally {
    root.classList.remove(REVEAL_CLASS);
    if (running === transition) running = null;
  }
}
