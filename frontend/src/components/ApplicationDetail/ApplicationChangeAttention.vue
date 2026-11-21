<template>
  <span ref="anchor" class="change-attention" :class="{ 'change-attention--changed': change }" @click="focus">
    <slot />
  </span>
  <Teleport to="body">
    <div v-if="change && active" ref="bubble" class="change-attention__hint" :style="position" role="dialog" :aria-label="text">
      <span>{{ text }}</span>
      <button type="button" class="lk-button lk-button--ghost" :disabled="busy" @click.stop="emit('acknowledge', change)">Понятно</button>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, watch, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { getViewportZoom } from '@/utils/viewportScale';
const props = defineProps({ change: { type: Object, default: null }, active: Boolean, busy: Boolean, text: { type: String, required: true } });
const emit = defineEmits(['acknowledge', 'focus-change']);
const anchor = ref(null), bubble = ref(null), position = ref({});
let observer, frame, observing = false;
function focus(event) { if (props.change) { event.stopPropagation(); emit('focus-change', props.change); } }
function place() {
  const badge = anchor.value?.querySelector('.chip') || anchor.value?.firstElementChild;
  if (!badge || !bubble.value) return;
  const zoom = getViewportZoom();
  const rect = badge.getBoundingClientRect();
  const width = bubble.value.offsetWidth;
  const left = Math.max(8, Math.min(rect.left / zoom, window.innerWidth / zoom - width - 8));
  position.value = { left: `${left}px`, top: `${rect.bottom / zoom + 7}px`, '--arrow-left': `${Math.max(12, Math.min(rect.left / zoom + rect.width / zoom / 2 - left, width - 14))}px` };
}
function schedule() { if (!props.change || !props.active) return; cancelAnimationFrame(frame); frame = requestAnimationFrame(place); }
function observe() {
  if (observing || !props.change || !props.active) return;
  observing = true;
  anchor.value?.firstElementChild?.scrollIntoView?.({ block: 'center', inline: 'nearest', behavior: 'smooth' });
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(schedule);
    if (anchor.value?.parentElement) observer.observe(anchor.value.parentElement);
  }
  window.addEventListener('resize', schedule);
  window.addEventListener('scroll', schedule, true);
  document.addEventListener('animationend', schedule, true);
  document.fonts?.ready.then(schedule);
  schedule();
}
watch(() => [props.change, props.active], async () => { await nextTick(); observe(); schedule(); }, { deep: true });
onMounted(observe);
onBeforeUnmount(() => { observer?.disconnect(); cancelAnimationFrame(frame); window.removeEventListener('resize', schedule); window.removeEventListener('scroll', schedule, true); document.removeEventListener('animationend', schedule, true); });
</script>

<style scoped>
.change-attention { display: contents; }
.change-attention--changed :deep(.chip), .change-attention--changed :deep(.badge) { border-color: var(--danger-text); box-shadow: 0 0 0 1px color-mix(in srgb, var(--danger) 25%, transparent); }
.change-attention__hint { position: fixed; z-index: 15000; display: flex; align-items: center; gap: 10px; width: max-content; max-width: calc(100vw - 16px); padding: 5px 7px 5px 11px; border: 1px solid #4b4b4b; border-radius: 10px; background: #333; color: #f2f2f2; box-shadow: 0 4px 12px #0004; font-size: 11px; white-space: nowrap; }
.change-attention__hint::before { content: ''; position: absolute; top: -5px; left: var(--arrow-left, 22px); width: 8px; height: 8px; background: #333; transform: translateX(-50%) rotate(45deg); border-left: 1px solid #4b4b4b; border-top: 1px solid #4b4b4b; }
.change-attention__hint button { min-height: 26px; padding: 4px 9px; font-size: 11px; white-space: nowrap; color: inherit; border-color: #727681; background: transparent; }
@media (max-width: 600px) { .change-attention__hint { font-size: 10px; gap: 9px; } }
</style>
