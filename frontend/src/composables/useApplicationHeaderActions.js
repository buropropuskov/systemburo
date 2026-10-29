import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useUiStore } from '@/stores/ui';
import { useOnboardingStore } from '@/stores/onboarding';

export const APPLICATION_ACTIONS_REVEAL = 'application-actions';
const INLINE_MIN_WIDTH = 1100;

/** One persistent action group; the tour only restores a menu it opened itself. */
export function useApplicationHeaderActions({ ui = useUiStore(), getTour = useOnboardingStore } = {}) {
  const headerRef = ref(null), headerActionsMenu = ref(null), headerActionsInline = ref(true);
  let observer, tourWatch, owned = false;
  const restore = () => { if (owned) headerActionsMenu.value?.close(); owned = false; };
  const releaseHeaderMenu = () => { owned = false; };
  const measure = () => {
    const width = headerRef.value?.clientWidth;
    if (width > 0) headerActionsInline.value = width >= INLINE_MIN_WIDTH;
  };
  const stopActive = watch(() => ui.tourActive, active => {
    tourWatch?.(); tourWatch = null;
    if (!active) { restore(); return; }
    const tour = getTour();
    tourWatch = watch(() => [tour.revealOpen, tour.revealOpenRevision, tour.currentStepId,
      tour.stepsRevision, tour.isActive, headerActionsInline.value, headerActionsMenu.value,
      headerActionsMenu.value?.disabled], () => {
      const menu = headerActionsMenu.value;
      if (!tour.isActive || headerActionsInline.value) { restore(); return; }
      const requested = tour.revealOpen === APPLICATION_ACTIONS_REVEAL;
      const current = tour.steps?.[tour.currentIndex]?.reveal?.open === APPLICATION_ACTIONS_REVEAL;
      if (requested && menu && !menu.disabled && !menu.isOpen) { menu.open(); owned = true; }
      else if (!requested && !current) restore();
    }, { immediate: true, flush: 'post' });
  }, { immediate: true, flush: 'post' });
  function onHeaderActionClick(event) {
    const button = event.target?.closest?.('button');
    if (!headerActionsInline.value && button && !button.closest('.duplicate-dropdown')) {
      headerActionsMenu.value?.close();
    }
  }
  onMounted(() => {
    measure();
    if (typeof ResizeObserver === 'function') {
      observer = new ResizeObserver(measure);
      if (headerRef.value) observer.observe(headerRef.value);
    } else window.addEventListener('resize', measure);
  });
  onBeforeUnmount(() => {
    stopActive(); tourWatch?.(); restore(); observer?.disconnect();
    window.removeEventListener('resize', measure);
  });
  return { headerRef, headerActionsMenu, headerActionsInline, releaseHeaderMenu, onHeaderActionClick };
}
