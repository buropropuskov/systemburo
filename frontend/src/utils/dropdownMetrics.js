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
 * Доступное место передаётся снаружи и считается из ограничений меню (его max-height
 * минус служебные строки), а не из clientHeight самого списка. Читать clientHeight
 * нельзя: он уже включает наше прошлое ограничение, поэтому пересчёт пришлось бы
 * начинать со сброса высоты - а на каждом событии прокрутки это разворачивало список
 * во весь рост и схлопывало обратно, из-за чего меню мигало и прыгало.
 *
 * @param {Element} box контейнер списка (с прокруткой)
 * @param {Element[]} items пункты
 * @param {number} available доступная высота под список
 * @param {number} minVisible минимум пунктов
 * @returns {number|null} высота в пикселях либо null, если ограничивать нечего
 */
export function wholeItemsHeight(box, items, available, minVisible = 2) {
  if (!box || !items || !items.length || !available) return null;
  const step = measureItemStep(items);
  if (!step) return null;

  // Список помещается целиком - ограничивать нечего, прокрутки не будет.
  if (items.length * step <= available + 0.5) return null;

  return fitWholeItems(available, step, minVisible);
}

/**
 * Сколько высоты внутри меню занимают НЕ пункты: строка поиска, кнопка сброса и т.п.
 *
 * offsetHeight, а не rect: меню открывается с анимацией, и rect отдаёт размеры под
 * трансформацией - при пересчёте на лету это давало бы разные значения на каждом кадре.
 *
 * @param {Element} menu контейнер меню
 * @param {Element} box контейнер списка (исключается из суммы)
 * @returns {number} суммарная высота служебных блоков
 */
export function measureChromeHeight(menu, box) {
  if (!menu) return 0;
  return Array.from(menu.children)
    .filter((el) => el !== box)
    .reduce((sum, el) => sum + (el.offsetHeight || 0), 0);
}
