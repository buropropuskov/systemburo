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
 * Признаки браузеров, которые на iOS работают на том же движке WebKit, но НЕ умеют
 * ставить сайт на экран "Домой" так, чтобы у него был push: ярлык из них открывается
 * обычной вкладкой. Отличить их можно только по этим меткам - Safari своей метки не
 * ставит, а слово Safari в строке несут все.
 * @param {Navigator} nav
 * @returns {boolean}
 */
export function isIosSafari(nav = navigator) {
  if (!isIOS(nav)) return false;
  const ua = nav.userAgent || '';
  return !/(CriOS|FxiOS|EdgiOS|YaBrowser|YaSearchBrowser|OPiOS|OPT\/|DuckDuckGo|Coast|mercury)/i.test(ua);
}

/**
 * @param {Navigator} nav
 * @param {Window} win
 * @returns {boolean} человек на iOS открыл сайт в стороннем браузере - подключить push
 * оттуда нельзя вообще, и инструкция про экран "Домой" ему бесполезна: сначала нужно
 * перейти в Safari. Проверка на живом iPhone (#974) показала ровно эту дыру - Chrome
 * получал подсказку для Safari и выполнить её не мог.
 */
export function iosNeedsSafari(nav = navigator, win = window) {
  return isIOS(nav) && !isStandalone(win) && !isIosSafari(nav);
}

/**
 * @param {Navigator} nav
 * @param {Window} win
 * @returns {boolean} нужно показать подсказку "добавьте сайт на экран Домой" - то есть
 * человек уже в Safari, но сайт ещё не установлен.
 */
export function needsIosHomeScreenInstall(nav = navigator, win = window) {
  return isIOS(nav) && !isStandalone(win) && isIosSafari(nav);
}
