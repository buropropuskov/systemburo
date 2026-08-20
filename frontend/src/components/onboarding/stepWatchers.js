/**
 * Наблюдатели за DOM на время шага тура.
 *
 * Оба слушают одно и то же дерево, но с разной целью: один ведёт шаг вперёд,
 * когда человек выполнил его просьбу, другой удерживает подсветку на цели,
 * которую страница пересоздала. Живут парой, снимаются вместе - поэтому и
 * собраны в один модуль, а не разбросаны по хосту тура.
 *
 * @param {{
 *   getDriver: () => (object|null),
 *   getGen: () => number,
 *   getStep: (globalIndex: number) => (object|undefined),
 *   getIndex: () => number
 * }} deps доступ к живому состоянию хоста: инстанс driver, поколение (чтобы
 *   отложенное срабатывание не трогало уже сменённый сегмент), шаг и текущий индекс
 * @returns {{ watchAdvance: Function, watchRetarget: Function, stopAll: Function }}
 */
export function createStepWatchers({ getDriver, getGen, getStep, getIndex }) {
  let advanceObserver = null;
  let retargetObserver = null;

  function stopAdvance() {
    if (!advanceObserver) return;
    advanceObserver.disconnect();
    advanceObserver = null;
  }

  function stopRetarget() {
    if (!retargetObserver) return;
    retargetObserver.disconnect();
    retargetObserver = null;
  }

  /**
   * Шаг-приглашение к действию: ждём, пока на экране появится узел из
   * `advanceWhen`, и уходим вперёд сами. Без этого человек, выполнивший просьбу
   * шага («Откройте заявку»), оставался с подсветкой строки под открытым окном.
   *
   * @param {number} globalIndex
   */
  function watchAdvance(globalIndex) {
    stopAdvance();
    const selector = getStep(globalIndex)?.advanceWhen;
    if (!selector || typeof MutationObserver === 'undefined') return;
    // Узел мог появиться до подписки - проверяем сразу.
    if (document.querySelector(selector)) return;
    const gen = getGen();
    advanceObserver = new MutationObserver(() => {
      if (!document.querySelector(selector)) return;
      stopAdvance();
      const driver = getDriver();
      if (!driver || gen !== getGen() || getIndex() !== globalIndex) return;
      driver.obNext();
    });
    advanceObserver.observe(document.body, { childList: true, subtree: true });
  }

  /**
   * Держать подсветку на цели, которую страница пересоздала.
   *
   * Списки дорисовываются, когда приезжают данные: узел, подсвеченный секунду
   * назад, выбрасывается из DOM, а вместе с ним пропадает и подсветка - на
   * экране остаётся затемнение без выреза. Так вело себя начало сегмента
   * таблицы поста. Дожидаемся нового узла по тому же селектору и переприцеливаем.
   *
   * @param {number} globalIndex
   */
  function watchRetarget(globalIndex) {
    stopRetarget();
    const step = getStep(globalIndex);
    if (!step?.element || typeof MutationObserver === 'undefined') return;
    const gen = getGen();
    retargetObserver = new MutationObserver(() => {
      const active = getDriver()?.getActiveElement?.();
      // Цель на месте либо шаг без подсветки - трогать нечего.
      if (!active || active.isConnected) return;
      if (!document.querySelector(step.element)) return;
      stopRetarget();
      const driver = getDriver();
      if (!driver || gen !== getGen() || getIndex() !== globalIndex) return;
      driver.obRetarget(globalIndex);
    });
    retargetObserver.observe(document.body, { childList: true, subtree: true });
  }

  /** Снять оба наблюдения - зовётся на каждой смене шага и в teardown. */
  function stopAll() {
    stopAdvance();
    stopRetarget();
  }

  return { watchAdvance, watchRetarget, stopAll };
}
