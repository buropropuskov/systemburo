import { driver } from 'driver.js';
import { driverDefaults } from './driverDefaults';
import { createPopoverZoomFix } from './popoverZoom';

/** A service state of the same presenter, never a learning/completion step. */
export function createRecoveryDriver({ retry, back, close, reducedMotion, onDestroyed = () => {} }) {
  let alive = true;
  const zoom = createPopoverZoomFix();
  const act = (action) => () => { if (alive) return action?.(); };
  const instance = driver({
    ...driverDefaults(reducedMotion),
    showProgress: false,
    showButtons: back ? ['previous', 'next', 'close'] : ['next', 'close'],
    nextBtnText: 'Повторить',
    doneBtnText: 'Повторить',
    steps: [{ popover: {
      title: 'Не удалось открыть шаг',
      description: 'Данные пока недоступны. Повтори попытку или закрой обучение — твой прогресс сохранится.',
    } }],
    onNextClick: act(retry),
    onPrevClick: act(back),
    onDestroyStarted: act(close),
    onPopoverRender(popover) {
      if (back && popover.previousButton) {
        popover.previousButton.disabled = false;
        popover.previousButton.classList.remove('driver-popover-btn-disabled');
      }
      zoom.attach(popover.wrapper);
    },
    onDestroyed() { alive = false; zoom.detach(); onDestroyed(); },
  });
  const destroy = instance.destroy.bind(instance);
  instance.obCancel = () => { alive = false; zoom.detach(); };
  instance.destroy = () => { instance.obCancel(); destroy(); };
  instance.obNext = act(retry);
  instance.obPrev = act(back);
  return instance;
}
