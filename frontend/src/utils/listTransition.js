/**
 * Закрепление уходящих элементов списка на их местах.
 *
 * Приём «position: absolute на .*-leave-active» выводит уходящий элемент из
 * потока, чтобы соседи начали смыкаться сразу, не дожидаясь конца его
 * исчезновения. Для ОДНОГО элемента это работает: без top/left он встаёт на
 * свою статическую позицию, то есть остаётся там же, где был.
 *
 * При массовом уходе приём ломается молча. Когда из потока за один кадр
 * вынимают десятки строк, статическая позиция у всех схлопывается к началу
 * контейнера: элементы накладываются друг на друга стопкой наверху. Переходы
 * при этом честно отрабатывают - классы применяются, transform меняется, - но
 * видно этого уже никому. Снаружи список просто меняется мгновенно.
 *
 * Ровно так и вышло в Центре заявок: смена фильтра уводила 26 строк разом, и
 * анимация проигрывалась в невидимой стопке.
 *
 * Лечится тем, что позицию каждой уходящей строки фиксируют ДО того, как она
 * покинет поток: тогда она уезжает со своего места, а соседи подтягиваются.
 *
 * Вешается на TransitionGroup хуком before-leave:
 *   <TransitionGroup name="app-row" @before-leave="pinLeavingElement">
 *
 * Требование к разметке: у контейнера списка должен быть position: relative -
 * иначе absolute отсчитается от другого предка. У всех наших списков он уже
 * стоит.
 *
 * @param {HTMLElement} el уходящий элемент
 */
/**
 * Снимок раскладки контейнера на момент, когда из него начали уходить строки.
 *
 * Хук before-leave вызывается для каждой уходящей отдельно, и первая же покидает
 * поток раньше, чем очередь дойдёт до второй. Мерить вторую в этот момент поздно:
 * её место в потоке уже сместилось вверх, закрепление сажает её на новую позицию,
 * Vue видит разницу со старой и добавляет перелёт по вертикали ПОВЕРХ ухода. Строки
 * начинают уезжать по диагонали в левый верхний угол вместо ровного ухода влево -
 * замер на стенде: top 498 -> 466 одновременно с left 71 -> 18.
 *
 * Снимок снимается один раз на пачку и живёт до конца кадра.
 */
const layoutSnapshots = new WeakMap();

/** Прямоугольники контейнера и его детей до того, как кто-то вышел из потока. */
function layoutSnapshot(parent) {
  const cached = layoutSnapshots.get(parent);
  if (cached) return cached;

  const snapshot = { parent: parent.getBoundingClientRect(), children: new Map() };
  for (const child of Array.from(parent.children || [])) {
    snapshot.children.set(child, child.getBoundingClientRect());
  }
  layoutSnapshots.set(parent, snapshot);
  requestAnimationFrame(() => layoutSnapshots.delete(parent));
  return snapshot;
}

export function pinLeavingElement(el) {
  const parent = el.parentElement;
  if (!parent) return;

  const snapshot = layoutSnapshot(parent);
  const rect = snapshot.children.get(el) || el.getBoundingClientRect();
  const parentRect = snapshot.parent;

  // Прокрутка контейнера входит в координаты: без неё строки, уходящие из
  // прокрученного списка, закрепились бы выше своего места на величину скролла.
  el.style.top = `${rect.top - parentRect.top + parent.scrollTop}px`;
  el.style.left = `${rect.left - parentRect.left + parent.scrollLeft}px`;
  el.style.width = `${rect.width}px`;
  el.style.height = `${rect.height}px`;
  // Размеры заданы явно, поэтому padding и border не должны прибавляться сверху.
  el.style.boxSizing = 'border-box';
  // Уходящие не должны перекрывать остающиеся: те едут на свои новые места.
  el.style.zIndex = '0';
  el.style.margin = '0';
}

/**
 * Удержание высоты контейнера на время ухода строк.
 *
 * Уходящая строка выходит из потока сразу, поэтому контейнер укорачивается тем
 * же кадром: при отсеве двух заявок из четырёх высота падала с 225 до 112 за
 * один кадр, и подпись «Показано N» под списком прыгала вверх ровно в тот
 * момент, когда строки только начинали уезжать. Уход получался смазан рывком
 * соседнего элемента.
 *
 * Пока высота держится, вторая фаза (оставшиеся занимают освободившиеся места)
 * начинается с чистого кадра. Снимаем ровно к её старту - там движение вверх
 * идёт уже по всему списку, и сокращение высоты читается как его часть.
 *
 * Высота ставится один раз на пачку: первый уходящий её и фиксирует, остальные
 * попадают на уже выставленное значение и таймер не перезаводят.
 *
 * @param {HTMLElement} el уходящий элемент
 * @param {number} ms сколько держать, миллисекунды
 */
export function holdParentHeight(el, ms) {
  const parent = el.parentElement;
  if (!parent || parent.dataset.heightHeld === '1') return;

  parent.dataset.heightHeld = '1';
  parent.style.minHeight = `${parent.getBoundingClientRect().height}px`;

  setTimeout(() => {
    parent.style.minHeight = '';
    delete parent.dataset.heightHeld;
  }, ms);
}
