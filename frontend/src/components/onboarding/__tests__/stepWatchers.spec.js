import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createStepWatchers } from '../stepWatchers';

/**
 * Переход по действию не должен срабатывать на раскрытие, сделанное самим туром.
 *
 * У шага про колокольчик стоит `advanceWhen` на панель уведомлений, а следующий
 * шаг эту же панель раскрывает сам (`reveal.open`). Когда человек жмёт «Далее»,
 * панель появляется по воле тура - и наблюдатель принимал это за действие
 * человека и толкал шаг вперёд поверх уже идущего перехода. На стенде это
 * выглядело так: панель открылась, через четверть секунды исчезла, тур застрял
 * на шаге с колокольчиком (замечание владельца 21.08).
 */
describe('watchStep - переход по действию не ловит собственное раскрытие тура', () => {
  let driver;
  let revealOpen;

  const steps = {
    5: { element: '[data-testid="bell"]', advanceWhen: '[data-testid="panel"]' },
    6: { element: '[data-testid="panel"]', reveal: { open: 'notifications' } },
  };

  function makeWatchers() {
    return createStepWatchers({
      getDriver: () => driver,
      getGen: () => 1,
      getStep: (i) => steps[i],
      getIndex: () => 5,
      getRevealOpen: () => revealOpen,
    });
  }

  function appearPanel() {
    const el = document.createElement('div');
    el.dataset.testid = 'panel';
    document.body.appendChild(el);
    return el;
  }

  beforeEach(() => {
    vi.useFakeTimers();
    revealOpen = null;
    driver = { obNext: vi.fn(), getActiveElement: () => null, refresh: vi.fn() };
    document.body.innerHTML = '';
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.innerHTML = '';
  });

  it('панель раскрыл тур - шаг вперёд не толкаем', async () => {
    const watchers = makeWatchers();
    watchers.watchStep(5);
    revealOpen = 'notifications';
    appearPanel();
    await vi.advanceTimersByTimeAsync(50);

    expect(driver.obNext).not.toHaveBeenCalled();
    watchers.stopAll();
  });

  it('панель открыл человек - переход по действию работает как прежде', async () => {
    const watchers = makeWatchers();
    watchers.watchStep(5);
    revealOpen = null;
    appearPanel();
    await vi.advanceTimersByTimeAsync(50);

    expect(driver.obNext).toHaveBeenCalled();
    watchers.stopAll();
  });

  it('тур раскрыл, человек закрыл и открыл сам - переход снова срабатывает', async () => {
    const watchers = makeWatchers();
    watchers.watchStep(5);
    revealOpen = 'notifications';
    const el = appearPanel();
    await vi.advanceTimersByTimeAsync(50);
    expect(driver.obNext).not.toHaveBeenCalled();

    el.remove();
    revealOpen = null;
    appearPanel();
    await vi.advanceTimersByTimeAsync(50);

    expect(driver.obNext).toHaveBeenCalled();
    watchers.stopAll();
  });
});
