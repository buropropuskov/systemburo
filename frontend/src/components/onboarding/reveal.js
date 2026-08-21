import bus from '@/eventBus';
import { isMobileViewport } from '@/composables/useOnboarding';
import { useOnboardingStore } from '@/stores/onboarding';

/**
 * Раскрытие свёрнутых целей тура ПЕРЕД подсветкой.
 *
 * Поле шага - `reveal: { mobile?: 'nav', open?: 'admin-column'|'search-panel'|'first-application' }`.
 * Две независимые оси:
 *
 * `mobile` - узел, который на <=768px уезжает за экран (#1097 S11). Рельс навигации
 * сворачивается в бургер-drawer: transform уводит его за край, но элемент остаётся
 * в DOM "видимым" для waitForElement, и без открытия тур подсветил бы пустоту. На
 * >=769 поле не читается - там эти узлы всегда на месте.
 *
 * `open` - узел, свёрнутый на ЛЮБОЙ ширине: он появляется только по действию
 * пользователя. Резолвер поднимает сигнал в сторе, а владелец узла (NavMenu,
 * App, UserApplications) реагирует watch'ем и сам закрывает то, что открыл, когда
 * сигнал гаснет. Тот же приём, что у демо-вложения (`demoAttachment`): тур не
 * лезет в чужой DOM и не оставляет за собой открытых панелей.
 *
 * Состояние drawer читаем из DOM (класс), а не локальным флагом: NavMenu закрывает
 * его сама на смене route (свой watch), дублирование было бы рассинхроном.
 */

const NAV_SELECTOR = '[data-testid="ob-nav-rail"]';
const NAV_OPEN_CLASS = 'nav-menu--mobile-open';

// Drawer уезжает transform'ом (NavMenu: 0.28s) - его ширина/высота при этом НЕ
// меняются, поэтому waitForElement (мерит только высоту) резолвит СРАЗУ, до конца
// анимации, и driver измерил бы цель ещё офскрин. Даём анимации доехать.
const NAV_DRAWER_TRANSITION_MS = 300;

// Тот же расчёт для `open`-узлов: колонка Админки (0.25s) и панель поиска (0.24s)
// въезжают transform'ом при неизменной высоте - ждём конца въезда, иначе driver
// померит рамку по стартовой позиции анимации.
const OPEN_REVEAL_TRANSITION_MS = 300;

/** Допустимые значения `reveal.open` - расширять вместе с резолвером во владельце узла. */
export const OPEN_TARGETS = ['admin-column', 'search-panel', 'notifications', 'first-application', 'first-attachment', 'attachment-blank', 'pass-report', 'work-modes', 'application-history'];

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
 * Что раскрыть на данном шаге. Значения берём у ТЕКУЩЕГО шага; lookahead prev/next
 * НЕ раскрывает узел РАДИ будущего шага (иначе на шаге-колокольчике перед nav-шагом
 * drawer открылся бы заранее). Он лишь УДЕРЖИВАЕТ раскрытие, когда шаг БЕЗ reveal
 * стоит ВНУТРИ группы одинаковых (оба соседа той же страницы и с тем же значением) -
 * чтобы «Назад» внутри группы не мигало закрытием/открытием.
 *
 * @param {Array<{route: string, reveal?: {mobile?: string, open?: string}}>} steps
 * @param {number} index
 * @returns {{ mobile: string|null, open: string|null }}
 */
export function resolveReveal(steps, index) {
  return {
    mobile: resolveAxis(steps, index, 'mobile'),
    open: resolveAxis(steps, index, 'open'),
  };
}

/**
 * @param {Array<object>} steps
 * @param {number} index
 * @param {'mobile'|'open'} axis
 * @returns {string|null}
 */
function resolveAxis(steps, index, axis) {
  const cur = steps?.[index];
  if (!cur) return null;
  if (cur.reveal?.[axis]) return cur.reveal[axis];
  const prev = steps[index - 1];
  const next = steps[index + 1];
  if (
    prev && next
    && prev.route === cur.route && next.route === cur.route
    && prev.reveal?.[axis] && prev.reveal[axis] === next.reveal?.[axis]
  ) {
    return prev.reveal[axis];
  }
  return null;
}

/**
 * Раскрыть то, что нужно ТЕКУЩЕМУ шагу, и свернуть остальное. Ось `mobile` работает
 * только на <=768px, ось `open` - на любой ширине. Ждём анимацию лишь когда реально
 * переключили состояние (сворачивание мгновенно и ожидания не требует).
 *
 * @param {Array<object>} steps
 * @param {number} index
 * @param {{ closeOthers?: boolean }} [options] `closeOthers: false` - только раскрыть,
 *   ничего не сворачивая: так зовёт подготовка следующего шага, пока переход не состоялся
 * @returns {Promise<boolean>} раскрывали ли что-то именно на этом шаге - вызывающий
 *   по этому признаку решает, ждать ли цель долго (узел ещё едет) или коротко
 *   (узел давно открыт, и отсутствие цели значит «её тут нет»).
 */
export async function applyReveal(steps, index, { closeOthers = true } = {}) {
  const { mobile, open } = resolveReveal(steps, index);

  const store = useOnboardingStore();
  // Подготовка следующего шага только РАСКРЫВАЕТ: сворачивать узел текущего шага
  // ей рано. Прежде она снимала сигнал сразу по нажатию, и список уведомлений
  // закрывался за 400 мс до того, как шаг сменится, - человек смотрел на подсветку
  // пустого места под окном «Список уведомлений» (замечание владельца 21.08).
  // Сворачивает то, что осталось лишним, уже подсветка нового шага.
  const keepCurrent = !closeOthers && !open;
  const openChanged = !keepCurrent && store.revealOpen !== (open || null);
  if (!keepCurrent) store.setRevealOpen(open);

  let drawerOpened = false;
  if (isMobileViewport()) {
    const wantNav = mobile === 'nav';
    // Тот же порядок для drawer: закрываем его не раньше смены шага.
    if (wantNav || closeOthers) drawerOpened = setNavDrawerOpen(wantNav) && wantNav;
  }

  const revealed = drawerOpened || (openChanged && Boolean(open));
  const waitMs = Math.max(
    drawerOpened ? NAV_DRAWER_TRANSITION_MS : 0,
    openChanged && open ? OPEN_REVEAL_TRANSITION_MS : 0,
  );
  if (waitMs) await new Promise((resolve) => setTimeout(resolve, waitMs));
  return revealed;
}

/**
 * Свернуть всё, что раскрывал тур - симметрично restoreRail, зовётся на границах
 * сегмента и в teardown. Владельцы `open`-узлов закрывают их по гашению сигнала.
 */
export function restoreReveal() {
  setNavDrawerOpen(false);
  useOnboardingStore().setRevealOpen(null);
}
