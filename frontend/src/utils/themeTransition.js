/**
 * Заливка новой темы от точки клика (#1415).
 *
 * View Transitions API снимает кадр «до», применяет тему и отдаёт оба кадра
 * псевдоэлементами: старый лежит неподвижно, новый открывается растущей областью.
 * Штатный кроссфейд гасит assets/theme-transition.css.
 *
 * ЖИДКОСТЬ - ЭТО НЕ РАСТУЩИЙ КРУГ. Две прошлые версии росли радиусом от курсора,
 * и обе читались как надувающийся шар: у круга нет ни низа, ни поверхности, а
 * добирая остаток площади у дальнего угла, он в конце «взрывается». Здесь форма
 * описана как настоящая жидкость - тремя стадиями, которые идут внахлёст:
 *
 *   1. КАПЛЯ. Из точки клика расходится линза: вниз охотнее, чем вверх
 *      (`DROP_UP_RATIO`), потому что вниз тянет вес.
 *   2. ЛУЖА. Дно линзы досаживается на низ экрана (`floorMix`) - жидкость
 *      доливается до пола и растекается по нему во всю ширину.
 *   3. УРОВЕНЬ. Дальше поднимается поверхность (`levelAt`) с длинной волной,
 *      которая гаснет к концу (`WAVES`, `waveAmplitude`). Экран заполняется
 *      снизу вверх, последним закрывается верхний край - ровно, без рывка.
 *
 * Кромка намеренно РЕЗКАЯ: размытие маской читалось как смазанное пятно, а у
 * жидкости граница чёткая. Заодно это дешевле на кадр - маску не растрируем.
 *
 * Без поддержки API (или при prefers-reduced-motion) тема применяется мгновенно:
 * эффект украшение, а не условие работы.
 */

/**
 * Длительность. Заливка снизу вверх читается как течение, ей нужно время:
 * ниже ~800мс уровень взлетает, выше ~1300 начинает тянуться.
 */
export const REVEAL_DURATION = 880;

/** Кадров интерполяции: WAAPI тянет между ними линейно. */
const STEPS = 26;
/** Столбцов выборки по X: по ним считаются верхняя и нижняя границы области. */
const COLUMNS = 56;
/** Вверх капля идёт вдвое неохотнее, чем вниз - это и читается как вес. */
const DROP_UP_RATIO = 0.5;
/** Стартовые полуоси капли, px: под курсором сразу видно каплю, а не точку. */
const DROP_SEED = 24;
/**
 * Запас за верхний край. Нужен, чтобы впадина волны не оставила полоску у верха,
 * но большим быть не должен: уровень тогда уходит за экран задолго до конца, и
 * последние кадры уже нечего заливать (замер: 90px давали 90мс пустого хвоста).
 */
const OVERSHOOT = 26;
/**
 * Амплитуда волны на поверхности, px. Это главная примета жидкости: на 900px
 * экране 30px читаются как лёгкая рябь, поэтому берём заметные 46.
 */
const WAVE_AMPLITUDE = 46;
/**
 * Волны поверхности: длина в экранах, вес, фаза, скорость бега. Длинные (1.1 и
 * 2.3 периода на ширину) и с разным знаком скорости - гребни расходятся, и
 * поверхность живёт, а не колеблется симметрично.
 */
const WAVES = [
  { periods: 1.1, weight: 0.62, phase: 0.4, speed: 2.4 },
  { periods: 2.3, weight: 0.38, phase: 2.1, speed: -1.7 },
];

/** Падение капли: вниз она уходит быстро - вес. */
const DROP_FALL = (t) => 1 - Math.pow(1 - Math.min(1, t / 0.22), 2.6);
/** Растекание в стороны отстаёт от падения: сперва лужа, потом разлив. */
const DROP_SPREAD = (t) => 1 - Math.pow(1 - Math.min(1, t / 0.36), 2.2);
/** Досадка дна на низ экрана - позже растекания, поэтому лужа сперва провисает. */
const FLOOR_MIX = (t) => Math.min(1, Math.max(0, (t - 0.14) / 0.3));
/**
 * Подъём уровня. Стартует сразу, как капля коснулась дна, и идёт ПОЧТИ ЛИНЕЙНО:
 * у smoothstep разгон был такой ленивый, что между лужей и подъёмом получалась
 * пауза (замер заполнения: +9%, потом +2% за кадр - на экране это ступор), а к
 * концу заполнение упиралось в 99% задолго до финала. Степень 0.9 даёт ровный
 * ход с мягким торможением: экран заполняется равномерно и до самого конца.
 */
function levelProgress(t) {
  const u = Math.min(1, Math.max(0, (t - 0.1) / 0.9));
  return Math.pow(u, 0.9);
}
/**
 * Волна гаснет к концу (иначе в последнем кадре у верхнего края останется
 * зазор), но гаснет ПОЗДНО: при быстром затухании волну видно только в начале.
 */
const WAVE_DECAY = (p) => Math.pow(1 - p, 0.7);

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

/** Смещение поверхности в точке x (доля ширины) в момент t. */
function surfaceWave(ratio, t) {
  let sum = 0;
  for (const w of WAVES) {
    sum += w.weight * Math.sin(ratio * w.periods * Math.PI * 2 + w.phase + w.speed * t);
  }
  return sum;
}

/**
 * Кадры области заливки. Каждый кадр - многоугольник: сверху идёт граница
 * жидкости слева направо, снизу возвращаемся справа налево. Число точек во всех
 * кадрах одинаковое - только тогда браузер интерполирует polygon() плавно.
 *
 * @param {RevealOrigin} origin
 * @param {number} width
 * @param {number} height
 */
function fillFrames(origin, width, height) {
  // Полуось капли по горизонтали должна дотянуться до дальнего края экрана.
  const spanMax = Math.max(origin.x, width - origin.x) * 1.15;
  const downMax = height - origin.y + 40;
  const frames = [];

  for (let i = 0; i <= STEPS; i += 1) {
    const t = i / STEPS;
    const spread = DROP_SPREAD(t);
    const fall = DROP_FALL(t);
    const floor = FLOOR_MIX(t);
    const level = levelProgress(t);
    const rx = DROP_SEED + spanMax * spread;
    const ryDown = DROP_SEED + downMax * fall;
    const ryUp = ryDown * DROP_UP_RATIO;
    // Поверхность идёт от точки клика вверх за край экрана.
    const surface = origin.y - (origin.y + OVERSHOOT) * level;
    const amp = WAVE_AMPLITUDE * WAVE_DECAY(level);

    const left = Math.max(0, origin.x - rx);
    const right = Math.min(width, origin.x + rx);
    const top = [];
    const bottom = [];
    for (let c = 0; c < COLUMNS; c += 1) {
      const x = left + ((right - left) * c) / (COLUMNS - 1);
      const dx = (x - origin.x) / rx;
      // Профиль линзы: 0 у краёв капли, 1 под курсором.
      const lens = Math.sqrt(Math.max(0, 1 - dx * dx));
      const lensTop = origin.y - ryUp * lens;
      const lensBottom = origin.y + ryDown * lens;
      const waved = surface + amp * surfaceWave(x / width, t);
      // Выше - то, что раньше добралось: линза капли или поднявшийся уровень.
      top.push({ x, y: Math.min(lensTop, waved) });
      // Дно линзы досаживается на низ экрана, пока лужа растекается по полу.
      bottom.push({ x, y: lensBottom + (height - lensBottom) * floor });
    }

    const points = [
      ...top.map((p) => `${p.x.toFixed(1)}px ${p.y.toFixed(1)}px`),
      ...bottom.reverse().map((p) => `${p.x.toFixed(1)}px ${p.y.toFixed(1)}px`),
    ];
    frames.push(`polygon(${points.join(',')})`);
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
    await root.animate(
      { clipPath: fillFrames(origin, window.innerWidth, window.innerHeight) },
      {
        duration: REVEAL_DURATION,
        // Кривые стадий уже разложены по кадрам, здесь линейно - иначе наложатся.
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
