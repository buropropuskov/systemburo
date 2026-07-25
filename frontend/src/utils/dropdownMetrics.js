/**
 * Шаг списка выпадающего меню - расстояние между верхами соседних пунктов.
 *
 * Нужен, чтобы ограничивать высоту списка целым числом пунктов: с произвольной
 * max-height последний пункт обрывается по середине строки и читается как дефект
 * вёрстки, а не как «здесь можно прокрутить».
 *
 * Почему именно шаг, а не высота пункта: высота одного элемента меряется до того,
 * как список получил ограничение, и расходится с итоговой раскладкой; scrollHeight,
 * делённый на число пунктов, округляется браузером до целого пикселя (480/13 = 36.92
 * при реальных 37) - обе оценки занижают шаг, и последняя строка снова срезается.
 *
 * @param {Element[]} items пункты списка в порядке отрисовки
 * @returns {number} шаг в пикселях (0, если мерить нечего)
 */
export function measureItemStep(items) {
  if (!items || !items.length) return 0;
  if (items.length === 1) return items[0].offsetHeight || 0;

  // offsetTop, а не getBoundingClientRect: меню открывается с анимацией, и рамка,
  // посчитанная через rect, приходит уменьшенной на трансформацию - шаг «растёт»
  // по кадрам (35.15 -> 37) и замер попадает мимо сетки. offsetTop - layout-величина,
  // transform на неё не влияет, поэтому ждать конца анимации не нужно.
  const step = items[1].offsetTop - items[0].offsetTop;
  return step > 0 ? step : (items[0].offsetHeight || 0);
}

/**
 * Сколько целых пунктов помещается в доступную высоту.
 * @param {number} available доступное место в пикселях
 * @param {number} itemStep шаг списка
 * @param {number} minVisible минимум пунктов (даже в тесном месте)
 * @returns {number} высота списка, кратная шагу; 0 - если считать не из чего
 */
export function fitWholeItems(available, itemStep, minVisible = 2) {
  if (!itemStep) return 0;
  const count = Math.max(minVisible, Math.floor(available / itemStep));
  // Дробное значение не округляем: округление до целого пикселя срезает у
  // последнего пункта долю строки, ради которой всё и затевалось.
  return count * itemStep;
}

/**
 * Высота списка, при которой видны только целые пункты.
 *
 * Меряет то место, которое РЕАЛЬНО досталось списку после раскладки, а не считает
 * его из чужих ограничений: список зажимают и собственная max-height, и высота
 * всплывающего блока, и соседние строки (поиск, сброс), причём их размеры
 * доезжают позже первого кадра. Единственный надёжный источник - clientHeight
 * уже отрисованного списка.
 *
 * @param {Element} box контейнер списка (с прокруткой)
 * @param {Element[]} items пункты
 * @param {number} minVisible минимум пунктов
 * @returns {number|null} высота в пикселях либо null, если ограничивать нечего
 */
export function wholeItemsHeight(box, items, minVisible = 2) {
  if (!box || !items || !items.length) return null;
  const step = measureItemStep(items);
  if (!step) return null;

  // clientHeight - тоже layout-величина, не искажается анимацией открытия.
  const available = box.clientHeight;
  if (!available) return null;

  // Список помещается целиком - ограничивать нечего, прокрутки не будет.
  if (items.length * step <= available + 0.5) return null;

  return fitWholeItems(available, step, minVisible);
}

// Сколько кадров ждём, пока раскладка меню устаканится, прежде чем поверить замеру.
const MAX_SETTLE_FRAMES = 12;

/**
 * Считает высоту списка, когда его раскладка перестала меняться, и отдаёт её в setHeight.
 *
 * Первый кадр после открытия верить нельзя: шрифты, паддинги и появление прокрутки
 * доезжают позже, и шаг списка успевает измениться (наблюдалось 40.17 -> 37). Если
 * зафиксировать высоту по раннему замеру, она не будет кратна финальной сетке и
 * последний пункт снова окажется срезан. Поэтому опрашиваем шаг по кадрам и берём
 * первый, который повторился, - тот же приём, что для подсветки шагов онбординга.
 *
 * @param {() => Element|null} getBox контейнер списка
 * @param {() => Element[]} getItems пункты
 * @param {(height: number|null) => void} setHeight куда положить результат
 * @param {number} minVisible минимум пунктов
 */
export function applyWholeItemsHeight(getBox, getItems, setHeight, minVisible = 2) {
  let prevStep = null;
  let frames = 0;

  const tick = () => {
    const box = getBox();
    const items = getItems();
    if (!box || !items || !items.length) {
      setHeight(null);
      return;
    }
    const step = measureItemStep(items);
    const settled = prevStep !== null && Math.abs(step - prevStep) < 0.5;
    if (settled || frames >= MAX_SETTLE_FRAMES) {
      setHeight(wholeItemsHeight(box, items, minVisible));
      return;
    }
    prevStep = step;
    frames += 1;
    requestAnimationFrame(tick);
  };

  requestAnimationFrame(tick);
}
