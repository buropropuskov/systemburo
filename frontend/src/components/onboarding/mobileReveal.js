import bus from '@/eventBus';
import { isMobileViewport } from '@/composables/useOnboarding';

/**
 * Раскрытие переехавших на мобилке (<=768px) целей тура ПЕРЕД подсветкой (#1097 S11).
 *
 * На узком экране рельс навигации сворачивается в бургер-drawer (NavMenu:
 * transform уводит его за экран - элемент остаётся "видимым" для waitForElement,
 * тур подсветил бы пустоту за краем), а вторичные иконки шапки - в overflow-меню
 * "⋯" (TheHeader: там они `display:none` - без открытия элемент вовсе не найдётся).
 *
 * КЛЮЧЕВОЕ (RED-фикс): reveal ЭКСКЛЮЗИВЕН - drawer и overflow ВЗАИМОИСКЛЮЧАЮЩИ.
 * Их узлы физически перекрывают друг друга (drawer z-index 10000 + полноэкранный
 * `.nav-menu__backdrop` 9999 накрывают overflow-панель z-index 300): открыть оба
 * = спотлайт целит в один элемент, а показывает другой. Поэтому открываем ТОЛЬКО
 * панель, нужную ТЕКУЩЕМУ шагу, и явно ДЕРЖИМ ЗАКРЫТОЙ другую. В security-туре
 * `sec-header-notifications` (overflow) идёт вплотную перед `sec-nav-rail` (nav) -
 * без эксклюзивности drawer открывался бы уже на шаге колокольчика и перекрывал его.
 *
 * Состояние читаем из DOM (класс), не храним локальный флаг: NavMenu сама
 * закрывает drawer на смене route (свой watch route), дублировать было бы рассинхроном.
 */

const NAV_SELECTOR = '[data-testid="ob-nav-rail"]';
const NAV_OPEN_CLASS = 'nav-menu--mobile-open';
const OVERFLOW_SELECTOR = '.header__overflow';
const OVERFLOW_OPEN_CLASS = 'header__overflow--open';
const OVERFLOW_TOGGLE_SELECTOR = '.header__overflow-toggle';

// Drawer уезжает transform'ом (NavMenu: 0.28s) - его ширина/высота при этом НЕ
// меняются, поэтому waitForElement (мерит только высоту) резолвит СРАЗУ, до конца
// анимации, и driver измерил бы цель ещё офскрин. Даём анимации доехать.
// Overflow-меню шапки открывается мгновенно (display, без transition) - паузы не требует.
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

/** @returns {boolean} открыто ли overflow-меню "⋯" шапки сейчас. */
export function isHeaderOverflowOpen() {
  const el = document.querySelector(OVERFLOW_SELECTOR);
  return !!el && el.classList.contains(OVERFLOW_OPEN_CLASS);
}

/**
 * Привести overflow-меню шапки к желаемому состоянию кликом по "⋯".
 *
 * @param {boolean} shouldOpen
 * @returns {boolean} true, если реально переключили состояние
 */
export function setHeaderOverflowOpen(shouldOpen) {
  const toggle = document.querySelector(OVERFLOW_TOGGLE_SELECTOR);
  if (!toggle || isHeaderOverflowOpen() === shouldOpen) return false;
  toggle.click();
  return true;
}

/**
 * Какая панель нужна на данном шаге. ЭКСКЛЮЗИВНО по ТЕКУЩЕМУ шагу
 * (`cur.mobileReveal`) с приоритетом. Lookahead prev/next НЕ открывает панель
 * РАДИ будущего шага (иначе на шаге-колокольчике перед nav-шагом открылся бы и
 * drawer). Он лишь УДЕРЖИВАЕТ панель того же типа, когда сам шаг БЕЗ mobileReveal
 * стоит ВНУТРИ группы одинакового типа (оба соседа той же страницы и того же
 * reveal) - чтобы backward-nav внутри группы не мигал закрытием/открытием.
 * Разные соседи / другой тип / другой route -> обе панели закрыты.
 *
 * @param {Array<{route: string, mobileReveal?: string}>} steps
 * @param {number} index
 * @returns {'nav'|'header-overflow'|null}
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
 * Открыть панель, нужную ТЕКУЩЕМУ шагу, и явно ЗАКРЫТЬ другую (эксклюзивно).
 * На >=769px (десктоп) ничего не делает - переехавшие узлы там всегда на месте.
 * Await'им анимацию drawer'а только если реально его открыли (закрытие и
 * overflow-меню мгновенны).
 *
 * @param {Array<object>} steps
 * @param {number} index
 * @returns {Promise<void>}
 */
export async function applyMobileReveal(steps, index) {
  if (!isMobileViewport()) return;
  const kind = resolveMobileReveal(steps, index);
  const wantNav = kind === 'nav';
  const drawerToggled = setNavDrawerOpen(wantNav);
  setHeaderOverflowOpen(kind === 'header-overflow');
  if (wantNav && drawerToggled) {
    await new Promise((resolve) => setTimeout(resolve, NAV_DRAWER_TRANSITION_MS));
  }
}

/** Закрыть обе панели - симметрично restoreRail, зовётся на границах сегмента и teardown. */
export function restoreMobileReveal() {
  setNavDrawerOpen(false);
  setHeaderOverflowOpen(false);
}
