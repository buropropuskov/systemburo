/**
 * Кадр: область снимка, воздух по краям, проверка пропорции и приведение к
 * печатному размеру.
 */

import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

const run = promisify(execFile);

/** Воздух по краям кадра по умолчанию, CSS-пиксели. */
export const DEFAULT_PAD = 40;

/** Печатное разрешение снимков. */
export const PRINT_DPI = 300;

/**
 * Предельная ширина готового файла. 16.5 см - ширина текста на листе:
 * 16.5 / 2.54 * 300 = 1949 с копейками. Столько же было у руководств прошлого
 * поколения (1925 px при 300 ppi).
 *
 * Снимок шире этого уменьшается, уже - остаётся как есть: растягивать мелкий
 * кадр на всю ширину текста значит размывать его. Мелкий кадр сборщик и
 * поставит меньшего размера, посчитав ширину от числа точек.
 */
export const TARGET_WIDTH_PX = 1950;

/**
 * Рамка кадра: без неё снимок светлого интерфейса растворяется в белом листе, и
 * читателю не видно, где кончается экран и начинается страница. Цвет взят от
 * линий интерфейса (--border, #e6e6e6) и затемнён: на печати линия в цвет
 * интерфейса не проступает вовсе.
 */
export const FRAME_COLOR = '#dcdcdc';

/** Толщина рамки в точках готового файла: 4 px при 300 ppi - примерно 0,3 мм. */
export const FRAME_WIDTH_PX = 4;

/**
 * Предельное отношение высоты к ширине. Картинка всегда растягивается сборщиком
 * на ширину текста 16.5 см, поэтому при отношении выше высота вылезает за поле
 * листа (16.5 * 1.5 = 24.75 см при доступных примерно 25.7 см).
 */
export const MAX_ASPECT = 1.5;

/**
 * Ждёт, пока положение элементов перестанет меняться.
 *
 * Погашенных переходов мало: выезжающая панель на первом кадре ещё стоит за
 * краем окна со сдвигом, и снятая в этот момент обводка ложится мимо, а кадр
 * выходит шириной в остаток экрана. Признак «элемент виден» тут не помогает -
 * сдвинутый элемент виден. Поэтому ждём именно устойчивых координат.
 *
 * @param {import('playwright').Page} page
 * @param {string[]} selectors
 */
/**
 * Ждёт, пока цели проявятся: непрозрачны и без сдвига.
 *
 * Устойчивость координат этого не гарантирует. Появление экрана входа сделано на
 * opacity и translateX по таймеру, а часы кадра зафиксированы (page.clock),
 * поэтому до срабатывания таймера элемент неподвижен, невидим и сдвинут - для
 * проверки устойчивости состояние «устоялось». Обводка ложилась по этим
 * координатам, а снимок с animations: 'disabled' дожимал переход, и поле уезжало
 * из-под линии на несколько точек вниз.
 *
 * @param {import('playwright').Page} page
 * @param {string[]} selectors
 */
export async function waitForRevealed(page, selectors, { tries = 30, pause = 100 } = {}) {
  /*
   * Шрифты - раньше всего остального: пока подставлен запасной, метрики строк
   * другие, и вся раскладка стоит на несколько точек выше. Снимок дожидается
   * шрифтов сам, поэтому обводка, снятая до них, оказывалась выше поля.
   */
  await page.evaluate(() => document.fonts.ready);
  if (selectors.length === 0) return;
  for (let attempt = 0; attempt < tries; attempt += 1) {
    const pending = await page.evaluate((list) => list.filter((selector) => {
      const element = document.querySelector(selector);
      if (!element) return false;
      const cs = getComputedStyle(element);
      if (parseFloat(cs.opacity) < 0.99) return true;
      const matrix = cs.transform;
      if (!matrix || matrix === 'none') return false;
      const parts = matrix.match(/-?[\d.]+/g);
      if (!parts) return false;
      // Последние две величины матрицы - сдвиг по осям.
      const dx = Math.abs(parseFloat(parts[parts.length - 2]));
      const dy = Math.abs(parseFloat(parts[parts.length - 1]));
      return dx > 0.5 || dy > 0.5;
    }), selectors);
    if (pending.length === 0) return;
    await page.waitForTimeout(pause);
  }
}

export async function waitForStableRects(page, selectors, { tries = 20, pause = 100 } = {}) {
  if (selectors.length === 0) return;
  /*
   * В снимок состояния входят прозрачность и сдвиг, а не только координаты.
   * Появление формы входа сделано на opacity и translateX: до срабатывания
   * таймера элемент стоит на месте (координаты «устойчивы»), но невидим и
   * сдвинут, а к моменту съёмки прыгает в конечное положение. Обводка, снятая
   * по такому «устойчивому» состоянию, оставалась висеть выше поля.
   */
  const read = () =>
    page.evaluate(
      (list) =>
        list
          .map((selector) => {
            const element = document.querySelector(selector);
            if (!element) return 'нет';
            const r = element.getBoundingClientRect();
            const cs = getComputedStyle(element);
            return [
              Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height),
              cs.opacity, cs.transform,
            ].join(',');
          })
          .join('|'),
      selectors,
    );

  let previous = await read();
  for (let attempt = 0; attempt < tries; attempt += 1) {
    await page.waitForTimeout(pause);
    const current = await read();
    if (current === previous) return;
    previous = current;
  }
}

/**
 * Считает область снимка: объединяющий прямоугольник целей, расширенный на pad
 * и подрезанный по окну.
 *
 * @param {import('playwright').Page} page
 * @param {{selector?: string, nth?: number, pad?: number}} clipSpec
 * @param {Array<{selector: string, nth?: number}>} highlightTargets цели обводки
 */
export async function computeClip(page, clipSpec, highlightTargets) {
  /*
   * Воздух задаётся либо одним числом, либо парой: узкий высокий элемент
   * (боковое меню, столбец формы) в кадр «по контуру» не годится - картинка
   * выходит выше допустимого, и её приходится расширять вбок, захватывая
   * соседнее содержимое.
   */
  const rawPad = clipSpec?.pad ?? DEFAULT_PAD;
  const padX = typeof rawPad === 'object' ? (rawPad.x ?? DEFAULT_PAD) : rawPad;
  const padY = typeof rawPad === 'object' ? (rawPad.y ?? DEFAULT_PAD) : rawPad;

  /*
   * Обводимые элементы входят в кадр всегда, даже когда задан контейнер:
   * контейнер задаёт минимальную область, а не границу. Кадр, из которого
   * торчит обведённый элемент, - брак при любом раскладе.
   */
  /*
   * Обзорный кадр берёт окно целиком: на нём показывают устройство экрана, и
   * подрезка по содержимому обрубила бы как раз то, ради чего он снят.
   */
  if (clipSpec?.full) {
    const size = page.viewportSize();
    return { clip: { x: 0, y: 0, width: size.width, height: size.height }, warnings: [] };
  }

  const parts = clipSpec?.selector
    ? [{ selector: clipSpec.selector, nth: clipSpec.nth ?? 0 }, ...highlightTargets]
    : highlightTargets;

  const rect = await page.evaluate((parts) => {
    let left = Infinity;
    let top = Infinity;
    let right = -Infinity;
    let bottom = -Infinity;

    for (const part of parts) {
      const element = document.querySelectorAll(part.selector)[part.nth ?? 0];
      if (!element) continue;
      const r = element.getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      left = Math.min(left, r.left);
      top = Math.min(top, r.top);
      right = Math.max(right, r.right);
      bottom = Math.max(bottom, r.bottom);
    }

    if (left === Infinity) return null;
    return { left, top, right, bottom, viewW: window.innerWidth, viewH: window.innerHeight };
  }, parts);

  if (!rect) {
    throw new Error(
      `область кадра не найдена: ${parts.map((part) => part.selector).join(', ')}`,
    );
  }

  const x = Math.max(0, Math.floor(rect.left - padX));
  const y = Math.max(0, Math.floor(rect.top - padY));
  const width = Math.min(rect.viewW, Math.ceil(rect.right + padX)) - x;
  const height = Math.min(rect.viewH, Math.ceil(rect.bottom + padY)) - y;

  const warnings = [];
  if (height > width * MAX_ASPECT) {
    warnings.push(
      `кадр выше полутора ширин (${width}x${height}) - на листе такая картинка не поместится по высоте`,
    );
  }

  return { clip: { x, y, width, height }, warnings };
}

/**
 * Вырезает область кадра из снимка всего окна.
 *
 * Браузер умеет снимать область сам, но снятая им область приходит с браком:
 * содержимое за наложенным окном попадает в кадр отражённым по вертикали.
 * Снимок всего окна в том же состоянии выходит чистым, поэтому область
 * вырезается уже из готового файла. Клип задан в единицах вёрстки, а снимок
 * идёт в двойном масштабе - отсюда умножение.
 *
 * @param {string} path путь к PNG, файл перезаписывается на месте
 * @param {{x:number,y:number,width:number,height:number}} clip
 * @param {number} scale масштаб съёмки
 */
export async function cropToClip(path, clip, scale) {
  const box = [clip.width, clip.height, clip.x, clip.y].map((value) => Math.round(value * scale));
  await run('magick', [path, '-crop', `${box[0]}x${box[1]}+${box[2]}+${box[3]}`, '+repage', path]);
}

/**
 * Проверяет, что кадр не пустой.
 *
 * Список, не успевший проявиться, даёт почти однотонную картинку: данные
 * загружены, счётчик внизу их считает, а карточек на снимке нет. Так вышло,
 * когда анимации на время съёмки выключались вместо доигрывания, и брак заметил
 * владелец, а не прогон. Разброс яркости отличает пустой лист от заполненного
 * дешевле любой другой проверки.
 *
 * @param {string} path путь к готовому PNG
 * @returns {Promise<string[]>} предупреждения
 */
export async function checkNotBlank(path) {
  const { stdout } = await run('magick', [path, '-colorspace', 'Gray', '-format', '%[fx:standard_deviation]', 'info:']);
  const spread = parseFloat(stdout) * 255;
  if (Number.isFinite(spread) && spread < 12) {
    return [`кадр почти однотонный (разброс ${spread.toFixed(1)}) - похоже, содержимое не проявилось`];
  }
  return [];
}

/**
 * Приводит снимок к печатному размеру. Снимается в двойном масштабе и
 * уменьшается: текст после уменьшения чище, чем при съёмке сразу в целевом
 * размере, а файл выходит вдвое легче.
 *
 * @param {string} path путь к PNG, файл перезаписывается на месте
 */
export async function normalize(path) {
  /*
   * -strip здесь не годится: он выполняется при записи и сносит в том числе
   * запись о разрешении, из-за чего файл уходит с 72 ppi независимо от порядка
   * флагов. Вместо него исключается только отметка времени - она меняется при
   * каждой пересъёмке и засоряет разницу версий.
   */
  /*
   * Палитра в 256 цветов уменьшает файл втрое. Снимок интерфейса состоит из
   * плашек и текста, и разницы не видно даже на фотографическом фоне страницы
   * входа - проверено сравнением. При двух сотнях кадров на комплект это
   * разница между двадцатью мегабайтами в хранилище и семьюдесятью.
   */
  await run('magick', [
    path,
    '-resize',
    `${TARGET_WIDTH_PX}x>`,
    '-bordercolor',
    FRAME_COLOR,
    '-border',
    `${FRAME_WIDTH_PX}`,
    '-colors',
    '256',
    '-dither',
    'None',
    '-units',
    'PixelsPerInch',
    '-density',
    `${PRINT_DPI}`,
    '-define',
    'png:exclude-chunk=time',
    path,
  ]);
}
