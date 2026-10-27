/**
 * Наблюдатели за DOM на время шага тура.
 *
 * Слушают одно и то же дерево, но с разной целью: один ведёт шаг вперёд, когда
 * человек выполнил его просьбу, другой удерживает подсветку на цели, которую
 * страница пересоздала, третий догоняет цель, уехавшую за край экрана. Живут
 * вместе и снимаются вместе - поэтому собраны в один модуль, а не разбросаны
 * по хосту тура.
 *
 * @param {{
 *   getDriver: () => (object|null),
 *   getGen: () => number,
 *   getStep: (globalIndex: number) => (object|undefined),
 *   getIndex: () => number,
 *   getRevealOpen?: () => (string|null)
 * }} deps доступ к живому состоянию хоста: инстанс driver, поколение (чтобы
 *   отложенное срабатывание не трогало уже сменённый сегмент), шаг и текущий индекс
 * @returns {{ watchStep: Function, stopAll: Function }}
 */
/** Запас у края экрана, при котором цель считается видимой. */
const VISIBILITY_MARGIN = 24;

/** Когда проверять видимость цели: сразу после показа и после подгрузки данных. */
const VISIBILITY_CHECKS_MS = [350, 1200];

/** Пауза между прокруткой к цели и пересчётом выреза - прокрутка плавная. */
const REFRESH_AFTER_SCROLL_MS = 400;

export function createStepWatchers({ getDriver, getGen, getStep, getIndex, getRevealOpen = () => null }) {
  let advanceObserver = null;
  let retargetObserver = null;
  const visibilityTimers = new Set();
  let epoch = 0;

  function isCurrent(watchEpoch, gen, globalIndex) {
    return watchEpoch === epoch && gen === getGen() && getIndex() === globalIndex;
  }

  function scheduleVisibility(callback, delay, watchEpoch) {
    const id = setTimeout(() => {
      if (watchEpoch !== epoch) return;
      visibilityTimers.delete(id);
      callback();
    }, delay);
    visibilityTimers.add(id);
  }

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
    // Тот же узел раскрывает следующий шаг, когда человек жмёт «Далее». Его
    // появление тогда - работа самого тура, а не действие человека, и толкать
    // шаг вперёд по нему нельзя: переход уже идёт, второй накладывается на него.
    // Панель уведомлений от этого открывалась и тут же гасла, а тур застревал.
    const openedByTour = getStep(globalIndex + 1)?.reveal?.open || null;
    const gen = getGen();
    const watchEpoch = epoch;
    const observer = new MutationObserver(() => {
      if (advanceObserver !== observer || !isCurrent(watchEpoch, gen, globalIndex)) return;
      if (!document.querySelector(selector)) return;
      // Наблюдение не снимаем: человек может закрыть узел и открыть его сам -
      // тогда переход по действию снова станет законным.
      if (openedByTour && getRevealOpen() === openedByTour) return;
      const driver = getDriver();
      if (!driver) return;
      stopAdvance();
      driver.obNext();
    });
    advanceObserver = observer;
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
    const watchEpoch = epoch;
    const observer = new MutationObserver(() => {
      if (retargetObserver !== observer || !isCurrent(watchEpoch, gen, globalIndex)) return;
      const active = getDriver()?.getActiveElement?.();
      // Цель на месте либо шаг без подсветки - трогать нечего.
      if (!active || active.isConnected) return;
      if (!document.querySelector(step.element)) return;
      const driver = getDriver();
      if (!driver) return;
      stopRetarget();
      driver.obRetarget(globalIndex);
    });
    retargetObserver = observer;
    retargetObserver.observe(document.body, { childList: true, subtree: true });
  }

  /**
   * Догнать цель, уехавшую за край экрана.
   *
   * Перед показом шага хост прокручивает страницу к цели, но содержимое иногда
   * дорастает уже после: карточка заявки дотягивает состав и решения согласующих
   * отдельными запросами, и блок, к которому относится шаг, уползает вниз. При
   * невысоком окне он оказывается за краем - человек видит затемнение и вырез
   * там, где ничего нет (поймано на туре согласующего при 780x493).
   *
   * Проверяем дважды: сразу после показа и ещё раз, когда данные точно приехали.
   * Прокрутив, просим driver пересчитать вырез - сам он позицию не переснимает.
   *
   * @param {number} globalIndex
   */
  function watchVisibility(globalIndex) {
    stopVisibility();
    const step = getStep(globalIndex);
    if (!step?.element) return;
    const gen = getGen();
    const watchEpoch = epoch;
    const check = () => {
      if (!isCurrent(watchEpoch, gen, globalIndex)) return;
      const driver = getDriver();
      if (!driver) return;
      const el = driver.getActiveElement?.();
      if (!el || !el.isConnected || typeof el.scrollIntoView !== 'function') return;
      const r = el.getBoundingClientRect();
      if (!r.height) return;
      const hidden = r.top > window.innerHeight - VISIBILITY_MARGIN || r.bottom < VISIBILITY_MARGIN;
      if (!hidden) return;
      el.scrollIntoView({ block: 'center', inline: 'nearest' });
      // Пересчёт выреза - не на следующем кадре, а когда прокрутка доехала:
      // driver меряет цель в момент вызова, и на кадре сразу после scrollIntoView
      // она ещё в пути (плавная прокрутка контейнера карточки).
      scheduleVisibility(() => {
        if (!isCurrent(watchEpoch, gen, globalIndex)) return;
        getDriver()?.refresh?.();
      }, REFRESH_AFTER_SCROLL_MS, watchEpoch);
    };
    VISIBILITY_CHECKS_MS.forEach((delay) => scheduleVisibility(check, delay, watchEpoch));
  }

  function stopVisibility() {
    visibilityTimers.forEach(clearTimeout);
    visibilityTimers.clear();
  }

  /**
   * Взять шаг под наблюдение целиком: переход по действию, удержание подсветки
   * на пересозданном узле и догон уехавшей цели. Хост зовёт одну функцию - что
   * именно нужно этому шагу, решают сами наблюдатели по его полям.
   *
   * @param {number} globalIndex
   */
  function watchStep(globalIndex) {
    stopAll();
    watchAdvance(globalIndex);
    watchRetarget(globalIndex);
    watchVisibility(globalIndex);
  }

  /** Снять все наблюдения - зовётся на каждой смене шага и в teardown. */
  function stopAll() {
    // Disconnect does not invalidate a callback already queued by the browser.
    epoch += 1;
    stopAdvance();
    stopRetarget();
    stopVisibility();
  }

  return { watchStep, stopAll };
}
