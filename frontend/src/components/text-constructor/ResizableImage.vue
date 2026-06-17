<template>
  <node-view-wrapper
    class="ci-image"
    :class="{ 'is-selected': isEditable && selected }"
  >
    <span class="ci-image-frame">
      <img
        ref="imgEl"
        :src="node.attrs.src"
        :alt="node.attrs.alt || null"
        class="constructor-image"
        :style="imgStyle"
        draggable="false"
      >
      <span
        v-if="isEditable && selected"
        class="ci-resize-handle"
        title="Потяните, чтобы изменить размер"
        @mousedown="startResize"
      />
    </span>
  </node-view-wrapper>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue';
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3';

const props = defineProps(nodeViewProps);

const MIN_WIDTH = 40;

const imgEl = ref(null);
/** Ширина во время перетаскивания (px); коммитим в атрибут ноды только на mouseup. */
const dragWidth = ref(null);
let stopDrag = null;

const isEditable = computed(() => Boolean(props.editor?.isEditable));

const imgStyle = computed(() => {
  const width = dragWidth.value ?? props.node.attrs.width;
  return width ? { width: `${width}px` } : {};
});

/** Верхняя граница ширины - доступная ширина области редактора (контент-бокс ProseMirror). */
function maxWidth() {
  const pm = imgEl.value?.closest('.ProseMirror');
  if (!pm) return Number.POSITIVE_INFINITY;
  const cs = window.getComputedStyle(pm);
  const padding = (parseFloat(cs.paddingLeft) || 0) + (parseFloat(cs.paddingRight) || 0);
  return Math.max(MIN_WIDTH, pm.clientWidth - padding);
}

function startResize(event) {
  if (!isEditable.value || !imgEl.value) return;
  event.preventDefault();
  event.stopPropagation();

  const startX = event.clientX;
  const startWidth = imgEl.value.getBoundingClientRect().width;
  const limit = maxWidth();

  const onMove = (e) => {
    const next = Math.round(startWidth + (e.clientX - startX));
    dragWidth.value = Math.max(MIN_WIDTH, Math.min(next, limit));
  };
  const onUp = () => {
    if (dragWidth.value != null) {
      props.updateAttributes({ width: dragWidth.value });
      dragWidth.value = null;
    }
    teardown();
  };
  function teardown() {
    document.removeEventListener('mousemove', onMove);
    document.removeEventListener('mouseup', onUp);
    stopDrag = null;
  }

  document.addEventListener('mousemove', onMove);
  document.addEventListener('mouseup', onUp);
  stopDrag = teardown;
}

onBeforeUnmount(() => {
  if (stopDrag) stopDrag();
});
</script>

<style scoped>
.ci-image {
  display: block;
}

.ci-image-frame {
  position: relative;
  display: inline-block;
  max-width: 100%;
  line-height: 0;
}

.ci-image-frame img {
  display: block;
  max-width: 100%;
  height: auto;
  border-radius: 8px;
}

.ci-image.is-selected .ci-image-frame img {
  outline: 2px solid #4f5bdf;
  outline-offset: 2px;
}

.ci-resize-handle {
  position: absolute;
  right: -7px;
  bottom: -7px;
  width: 14px;
  height: 14px;
  background: #4f5bdf;
  border: 2px solid #fff;
  border-radius: 50%;
  cursor: nwse-resize;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.25);
}
</style>
