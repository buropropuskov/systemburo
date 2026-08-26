/**
 * Рельс навигации во время тура.
 *
 * Держим его развёрнутым на nav-шаге И на шаге ПЕРЕД ним: разворачиваем заранее
 * (overlay через `tourForceExpand`, без сдвига контента), чтобы к моменту
 * подсветки рельс уже доехал до полной ширины и driver померил рамку сразу
 * верно - без ре-замера и без моргания выреза.
 *
 * Прежнее состояние запоминаем при первом развороте и возвращаем в конце: тур не
 * должен оставлять после себя чужой интерфейс раскрытым.
 *
 * @param {{ tourForceExpand: boolean, sidebarHidden: boolean }} ui стор интерфейса
 * @param {(index: number) => object|undefined} getStep доступ к шагу по индексу
 * @returns {{ apply: (index: number) => void, restore: () => void }}
 */
export function createRailControl(ui, getStep) {
  let saved = null;

  function needed(index) {
    return Boolean(getStep(index)?.expandRail || getStep(index + 1)?.expandRail);
  }

  function apply(index) {
    if (!needed(index)) {
      restore();
      return;
    }
    if (!saved) saved = { force: ui.tourForceExpand, hidden: ui.sidebarHidden };
    ui.tourForceExpand = true;
    ui.sidebarHidden = false;
  }

  function restore() {
    if (!saved) return;
    ui.tourForceExpand = saved.force;
    ui.sidebarHidden = saved.hidden;
    saved = null;
  }

  return { apply, restore };
}
