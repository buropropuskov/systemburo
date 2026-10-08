import { computed, onScopeDispose, ref, unref, watch } from 'vue';

/** Server instants plus monotonic elapsed time; local wall-clock changes grant nothing. */
export function usePassageClock(serverNow, { active = true, deadlines = [], onBoundary = () => {} } = {}) {
  const now = ref(null);
  let anchor = null, monotonic = 0, interval = null, boundaryTimer = null;
  const fired = new Set();
  const instant = value => typeof value === 'number' ? value : Date.parse(value || '');
  const update = () => {
    now.value = anchor === null ? null : anchor + Math.max(0, performance.now() - monotonic);
  };
  const stop = () => { clearInterval(interval); clearTimeout(boundaryTimer); interval = boundaryTimer = null; };
  const schedule = () => {
    clearTimeout(boundaryTimer);
    if (!unref(active) || now.value === null) return;
    const next = (unref(deadlines) || []).map(instant).filter(Number.isFinite)
      .filter(value => value >= now.value && !fired.has(value)).sort((a, b) => a - b)[0];
    if (next === undefined) return;
    boundaryTimer = setTimeout(() => {
      update();
      if (now.value < next) { schedule(); return; }
      fired.add(next);
      onBoundary(next);
      schedule();
    }, Math.min(2147483647, Math.max(0, next - now.value)));
  };
  watch(() => [unref(serverNow), unref(active)], ([stamp, enabled]) => {
    stop();
    const parsed = instant(stamp);
    anchor = enabled && Number.isFinite(parsed) ? parsed : null;
    monotonic = performance.now();
    fired.clear();
    update();
    if (anchor !== null) { interval = setInterval(update, 1000); schedule(); }
  }, { immediate: true });
  watch(() => unref(deadlines), schedule, { deep: true });
  onScopeDispose(stop);
  return {
    now: computed(() => now.value),
    elapsedSince: stamp => {
      const start = instant(stamp);
      return now.value === null || !Number.isFinite(start) ? null : Math.max(0, now.value - start);
    },
  };
}
