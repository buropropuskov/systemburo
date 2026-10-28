/** User-action observation only; layout/retarget jobs belong to tourGeometry. */
export function createStepWatchers({ getDriver, getGen, getStep, getIndex, getRevealOpen = () => null }) {
  let advanceObserver = null;
  let epoch = 0;
  const isCurrent = (watchEpoch, gen, index) => watchEpoch === epoch && gen === getGen() && getIndex() === index;
  function stopAdvance() {
    advanceObserver?.disconnect();
    advanceObserver = null;
  }

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

  function watchStep(globalIndex) {
    stopAll();
    watchAdvance(globalIndex);
  }
  function stopAll() {
    epoch += 1;
    stopAdvance();
  }
  return { watchStep, stopAll };
}
