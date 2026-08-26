/**
 * Управление туром с клавиатуры - на стороне хоста, а не driver.js.
 *
 * driver снимает свои обработчики на время переезда подсветки и на время
 * подъёма нового сегмента (между страницами инстанса вообще нет). Нажатие,
 * попавшее в это окно - 300-400 мс, - пропадало бесследно: человек жал стрелку,
 * ничего не происходило, он ждал и жал снова. Со стороны это выглядело как
 * «шаг завис на несколько секунд».
 *
 * Поэтому команду не теряем, а откладываем: пока тур занят, помним последнее
 * направление и выполняем его сразу по готовности. Копим ровно одно нажатие -
 * очередь из десяти стрелок пролистала бы половину обучения залпом.
 *
 * @param {{
 *   isActive: () => boolean,
 *   next: () => void,
 *   prev: () => void,
 *   close: () => void,
 * }} actions
 * @returns {{ attach: () => void, detach: () => void, setBusy: (busy: boolean) => void, busyWhile: (fn: () => Promise<unknown>) => Promise<unknown> }}
 */
export function createKeyboardNav(actions) {
  let busy = false;
  let pending = 0;
  let handler = null;

  function run(dir) {
    if (dir > 0) actions.next();
    else actions.prev();
  }

  function request(dir) {
    if (busy) {
      pending = dir;
      return;
    }
    run(dir);
  }

  /**
   * Тур занят подготовкой шага или подъёмом сегмента. По снятию занятости
   * отложенное нажатие уходит в работу - ровно одно.
   */
  function setBusy(value) {
    busy = Boolean(value);
    if (busy || !pending) return;
    const dir = pending;
    pending = 0;
    run(dir);
  }

  function onKeyDown(event) {
    if (!actions.isActive()) return;
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
    // Внутри поля ввода стрелки принадлежат тексту, а не туру: на шаге про поиск
    // человек может печатать прямо в подсвеченном поле.
    const tag = event.target?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || event.target?.isContentEditable) return;
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      request(1);
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      request(-1);
    } else if (event.key === 'Escape') {
      actions.close();
    }
  }

  /**
   * Выполнить работу, на время которой тур не принимает команды (подготовка шага,
   * подъём сегмента). Нажатие, пришедшее внутри, не теряется - сработает следом.
   *
   * @template T
   * @param {() => Promise<T>} fn
   * @returns {Promise<T>}
   */
  async function busyWhile(fn) {
    setBusy(true);
    try {
      return await fn();
    } finally {
      setBusy(false);
    }
  }

  return {
    busyWhile,
    attach() {
      if (handler) return;
      handler = onKeyDown;
      document.addEventListener('keydown', handler);
    },
    detach() {
      if (!handler) return;
      document.removeEventListener('keydown', handler);
      handler = null;
      busy = false;
      pending = 0;
    },
    setBusy,
  };
}
