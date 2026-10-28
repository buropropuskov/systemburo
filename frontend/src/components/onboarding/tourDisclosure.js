import { onScopeDispose, watch } from 'vue';

/** Reveal through the owner state; never close a panel opened before the tour. */
export function useTourDisclosure({ requested, enabled, open }) {
  let owned = false;
  watch(open, value => { if (!value) owned = false; }, { flush: 'sync' });
  function release() {
    if (!owned) return;
    owned = false;
    open.value = false;
  }
  watch([requested, enabled], ([wanted, available]) => {
    if (!wanted || !available) { release(); return; }
    if (!open.value) { owned = true; open.value = true; }
  }, { immediate: true });
  onScopeDispose(release);
}
