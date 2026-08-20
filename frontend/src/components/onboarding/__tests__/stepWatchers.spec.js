import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createStepWatchers } from '../stepWatchers';

/**
 * Догон цели, уехавшей за край экрана.
 *
 * Хост прокручивает страницу к цели ПЕРЕД показом шага, но содержимое иногда
 * дорастает уже после: карточка заявки дотягивает состав и решения согласующих
 * отдельными запросами. При невысоком окне блок уползает за нижний край, и
 * человек видит вырез там, где ничего нет - поймано на туре согласующего при
 * 780x493 (цель `ob-detail-status` оказалась на y=831 при высоте окна 493).
 */

function makeTarget({ top, height = 100 }) {
  const el = document.createElement('div');
  el.scrollIntoView = vi.fn(() => {
    // после прокрутки цель встаёт в середину окна - как настоящий scrollIntoView
    el.__top = Math.round((window.innerHeight - height) / 2);
  });
  el.__top = top;
  el.getBoundingClientRect = () => ({
    top: el.__top, bottom: el.__top + height, left: 0, right: 400, width: 400, height,
    x: 0, y: el.__top,
  });
  document.body.appendChild(el);
  return el;
}

describe('watchStep - догон уехавшей цели', () => {
  let driver;
  let watchers;

  beforeEach(() => {
    vi.useFakeTimers();
    window.innerHeight = 493;
    driver = { getActiveElement: () => null, refresh: vi.fn() };
    watchers = createStepWatchers({
      getDriver: () => driver,
      getGen: () => 1,
      getStep: () => ({ element: '[data-testid="ob-detail-status"]' }),
      getIndex: () => 5,
    });
  });

  afterEach(() => {
    watchers.stopAll();
    vi.useRealTimers();
    document.body.innerHTML = '';
  });

  it('цель за нижним краем - прокручиваем к ней и просим пересчитать вырез', async () => {
    const el = makeTarget({ top: 831, height: 394 });
    driver.getActiveElement = () => el;

    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(400);

    expect(el.scrollIntoView).toHaveBeenCalledWith({ block: 'center', inline: 'nearest' });
    // Пересчёт выреза ждёт, пока прокрутка доедет, - иначе driver померит цель в пути.
    expect(driver.refresh).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(450);
    expect(driver.refresh).toHaveBeenCalled();
  });

  it('цель на экране - не трогаем ни прокрутку, ни подсветку', async () => {
    const el = makeTarget({ top: 200, height: 150 });
    driver.getActiveElement = () => el;

    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(1500);

    expect(el.scrollIntoView).not.toHaveBeenCalled();
    expect(driver.refresh).not.toHaveBeenCalled();
  });

  it('проверяем и второй раз - данные приезжают уже после показа шага', async () => {
    const el = makeTarget({ top: 200, height: 150 });
    driver.getActiveElement = () => el;

    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(400);
    expect(el.scrollIntoView).not.toHaveBeenCalled();

    // карточка дотянула состав, блок уехал вниз
    el.__top = 900;
    await vi.advanceTimersByTimeAsync(900);
    expect(el.scrollIntoView).toHaveBeenCalledTimes(1);
  });

  it('шаг сменился, пока ждали - чужую цель не трогаем', async () => {
    const el = makeTarget({ top: 831, height: 394 });
    driver.getActiveElement = () => el;
    let index = 5;
    watchers = createStepWatchers({
      getDriver: () => driver,
      getGen: () => 1,
      getStep: () => ({ element: '[data-testid="x"]' }),
      getIndex: () => index,
    });

    watchers.watchStep(5);
    index = 6;
    await vi.advanceTimersByTimeAsync(1500);

    expect(el.scrollIntoView).not.toHaveBeenCalled();
  });

  it('шаг без цели наблюдать нечего', async () => {
    watchers = createStepWatchers({
      getDriver: () => driver,
      getGen: () => 1,
      getStep: () => ({ element: null }),
      getIndex: () => 5,
    });
    const el = makeTarget({ top: 831 });
    driver.getActiveElement = () => el;

    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(1500);

    expect(el.scrollIntoView).not.toHaveBeenCalled();
  });
});
