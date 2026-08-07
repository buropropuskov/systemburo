/**
 * Детект платформы для Web Push (#974), без внешних библиотек. На iOS/iPadOS
 * Push API становится реально рабочим только для сайта, добавленного на экран
 * "Домой" (WebKit требует установленный PWA-контекст) - в обычной вкладке
 * Safari `PushManager`/`Notification` могут формально существовать в window,
 * но `subscribe()` откажет. Платформенный признак надёжнее feature-детекта:
 * решаем показать подсказку про установку ДО попытки подписаться, а не по
 * пойманной ошибке subscribe().
 */

/**
 * @param {Navigator} nav
 * @returns {boolean}
 */
export function isIOS(nav = navigator) {
  const ua = nav.userAgent || '';
  // iPadOS 13+ выдаёт себя за Mac в UA, но, в отличие от настоящего Mac,
  // несёт мультитач - у ноутбуков/десктопов maxTouchPoints равен 0 или 1.
  const isIPadOS13Plus = nav.platform === 'MacIntel' && nav.maxTouchPoints > 1;
  return /iPhone|iPad|iPod/.test(ua) || isIPadOS13Plus;
}

/**
 * @param {Window} win
 * @returns {boolean} сайт запущен как установленное веб-приложение (добавлен
 * на экран "Домой"), а не открыт обычной вкладкой браузера.
 */
export function isStandalone(win = window) {
  const matchesDisplayMode = typeof win.matchMedia === 'function'
    && win.matchMedia('(display-mode: standalone)').matches;
  // navigator.standalone - нестандартный, но единственный надёжный признак на iOS Safari.
  return Boolean(matchesDisplayMode || win.navigator?.standalone === true);
}

/**
 * @param {Navigator} nav
 * @param {Window} win
 * @returns {boolean} нужно показать подсказку "добавьте сайт на экран Домой"
 */
export function needsIosHomeScreenInstall(nav = navigator, win = window) {
  return isIOS(nav) && !isStandalone(win);
}
