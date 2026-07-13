import bus from '@/eventBus';
import { isMobileViewport } from '@/composables/useOnboarding';

/**
 * Раскрытие переехавших на мобилке (<=768px) целей тура ПЕРЕД подсветкой (#1097 S11).
 *
 * На узком экране рельс навигации сворачивается в бургер-drawer (NavMenu: transform
 * уводит его за экран - элемент остаётся "видимым" для waitForElement, тур подсветил
 * бы пустоту за краем). Такие шаги несут `mobileReveal: 'nav'` - хост открывает drawer
 * перед подсветкой и держит закрытым на остальных.
 *
 * Меню "⋯" (overflow) шапки убрано совсем (правка волны 3): часы удалены, а
 * «Сообщить о проблеме» переехало в drawer (reveal 'nav', #1097 W3.3). Поэтому
 * reveal теперь один - 'nav'; overflow-механики больше нет. Колокольчик
 * (`*-header-notifications`, #1097 W3.2) reveal НЕ несёт - он в самой шапке, на его
 * шаге drawer закрыт.
 *
 * Состояние читаем из DOM (класс), не храним локальный флаг: NavMenu сама
 * закрывает drawer на смене route (свой watch route), дублировать было бы рассинхроном.
 */

const NAV_SELECTOR = '[data-testid="ob-nav-rail"]';
const NAV_OPEN_CLASS = 'nav-menu--mobile-open';

// Drawer уезжает transform'ом (NavMenu: 0.28s) - его ширина/высота при этом НЕ
// меняются, поэтому waitForElement (мерит только высоту) резолвит СРАЗУ, до конца
// анимации, и driver измерил бы цель ещё офскрин. Даём анимации доехать.
const NAV_DRAWER_TRANSITION_MS = 300;

/** @returns {boolean} открыт ли бургер-drawer навигации сейчас. */
export function isNavDrawerOpen() {
  const el = document.querySelector(NAV_SELECTOR);
  return !!el && el.classList.contains(NAV_OPEN_CLASS);
}

/**
 * Привести drawer к желаемому состоянию через $bus (тот же тоггл, что у burger-кнопки).
 *
 * @param {boolean} shouldOpen
 * @returns {boolean} true, если реально переключили состояние (был не таким)
 */
export function setNavDrawerOpen(shouldOpen) {
  if (isNavDrawerOpen() === shouldOpen) return false;
  bus.emit('mobile-nav-toggle');
  return true;
}

/**
 * Нужен ли drawer на данном шаге. По ТЕКУЩЕМУ шагу (`cur.mobileReveal`) с приоритетом.
 * Lookahead prev/next НЕ открывает drawer РАДИ будущего шага (иначе на шаге-колокольчике
 * перед nav-шагом drawer открылся бы). Он лишь УДЕРЖИВАЕТ drawer, когда сам шаг БЕЗ
 * mobileReveal стоит ВНУТРИ nav-группы (оба соседа той же страницы и тоже 'nav') - чтобы
 * backward-nav внутри группы не мигал закрытием/открытием. Разные соседи / другой route
 * -> drawer закрыт.
 *
 * @param {Array<{route: string, mobileReveal?: string}>} steps
 * @param {number} index
 * @returns {'nav'|null}
 */
export function resolveMobileReveal(steps, index) {
  const cur = steps?.[index];
  if (!cur) return null;
  if (cur.mobileReveal) return cur.mobileReveal;
  const prev = steps[index - 1];
  const next = steps[index + 1];
  if (
    prev && next
    && prev.route === cur.route && next.route === cur.route
    && prev.mobileReveal && prev.mobileReveal === next.mobileReveal
  ) {
    return prev.mobileReveal;
  }
  return null;
}

/**
 * Открыть drawer, если он нужен ТЕКУЩЕМУ шагу, иначе закрыть.
 * На >=769px (десктоп) ничего не делает - переехавшие узлы там всегда на месте.
 * Await'им анимацию drawer'а только если реально его открыли (закрытие мгновенно).
 *
 * @param {Array<object>} steps
 * @param {number} index
 * @returns {Promise<void>}
 */
export async function applyMobileReveal(steps, index) {
  if (!isMobileViewport()) return;
  const wantNav = resolveMobileReveal(steps, index) === 'nav';
  const drawerToggled = setNavDrawerOpen(wantNav);
  if (wantNav && drawerToggled) {
    await new Promise((resolve) => setTimeout(resolve, NAV_DRAWER_TRANSITION_MS));
  }
}

/** Закрыть drawer - симметрично restoreRail, зовётся на границах сегмента и teardown. */
export function restoreMobileReveal() {
  setNavDrawerOpen(false);
}
