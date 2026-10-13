import { onBeforeUnmount, ref } from 'vue';

/** Задержка сворачивания: курсор успевает перейти с рельса на выехавшую подпись. */
const COLLAPSE_DELAY = 150;

/**
 * Поведение разворота рельса навменю.
 *
 * Вынесено из компонента, потому что тот упёрся в порог размера, а тема цельная:
 * разворот по наведению, отложенное сворачивание и пин.
 *
 * Рельса не бывает на экранах без мыши: там бургер-drawer (медиазапросы навменю
 * включают его и по ширине до 1024, и по `hover: none` с `pointer: coarse` - то есть
 * на планшете в альбомной ориентации тоже). Поэтому здесь чистая десктопная история,
 * и гейтить поведение по способу ввода не нужно.
 *
 * @param {import('vue').ComponentInternalInstance} instance инстанс навменю
 * @returns {{ expandMenu: () => void, collapseMenu: () => void }}
 */
export function useRailExpand(instance) {
  let hoverTimeout = null;
  const themePanelOpen = ref(false);

  const vm = () => instance.proxy;

  function expandMenu() {
    const menu = vm();

    // При открытой Админке рельс зафиксирован в иконках - наведение не разворачивает.
    if (menu.adminOpen) return;
    // Тап по бургеру синтезирует mouseenter на рельсе, как только drawer выезжает
    // под пальцем (#1097): без гейта рельс уходил бы в десктопные 248px вместо
    // мобильных 280px/85vw.
    if (menu.mobileOpen) return;
    if (hoverTimeout) {
      clearTimeout(hoverTimeout);
      hoverTimeout = null;
    }
    menu.isExpanded = true;
  }

  function collapseMenu() {
    hoverTimeout = setTimeout(() => {
      const menu = vm();
      // Keep the rail open while the pointer is over its teleported settings.
      if (menu.themePanelOpen && (menu.$refs.navigation?.matches(':hover') || menu.$refs.themeControls?.isHovered())) {
        hoverTimeout = null;
        return;
      }
      menu.isExpanded = false;
      // Схлопнулся по-настоящему (не закреплён пином) - закрываем и раскрытые
      // дропдауны, чтобы при следующем заходе они были свёрнуты.
      if (!menu.uiStore.sidebarExpanded) menu.closeAllDropdowns();
      hoverTimeout = null;
    }, COLLAPSE_DELAY);
  }

  function onThemePanelChange(open) {
    themePanelOpen.value = open;
    if (open) { expandMenu(); return; }
    if (!open && !vm().$refs.navigation?.matches(':hover')) collapseMenu();
  }

  function closeMobile() {
    const menu = vm();
    menu.$refs.themeControls?.close();
    if (!menu.mobileOpen) return;
    menu.mobileOpen = false;
    document.body.classList.remove('nav-drawer-open');
  }

  onBeforeUnmount(() => {
    if (hoverTimeout) clearTimeout(hoverTimeout);
  });

  return { expandMenu, collapseMenu, themePanelOpen, onThemePanelChange, closeMobile };
}
