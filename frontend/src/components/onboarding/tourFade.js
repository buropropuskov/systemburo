/**
 * Плавное закрытие тура.
 *
 * driver.js делает только fade-IN, а на destroy убирает затемнение и поповер
 * мгновенно - это читается как рывок. Навешиваем класс затухания на оба узла и
 * удаляем DOM уже после анимации. Только для ЗАВЕРШЕНИЯ тура (финал, Esc,
 * крестик, «Пропустить»), не для переходов между страницами внутри тура.
 */

/** Столько же длится переход `.ob-fade-out` в onboarding.css. */
const FADE_MS = 240;

/**
 * @param {{ destroy: () => void }} driverInstance инстанс driver.js
 * @param {boolean} [reducedMotion] человек просит уменьшить анимации - гасим сразу
 */
export function fadeAndDestroy(driverInstance, reducedMotion = false) {
  const els = [
    document.querySelector('.driver-overlay'),
    document.querySelector('.driver-popover'),
  ].filter(Boolean);
  if (!els.length || reducedMotion) {
    driverInstance.destroy();
    return;
  }
  els.forEach((el) => el.classList.add('ob-fade-out'));
  setTimeout(() => driverInstance.destroy(), FADE_MS);
}
