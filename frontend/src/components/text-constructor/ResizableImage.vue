<template>
  <node-view-wrapper
    class="ci-image"
    :class="[{ 'is-selected': isEditable && selected }, alignClass]"
  >
    <span class="ci-image-frame">
      <img
        ref="imgEl"
        :src="node.attrs.src"
        :alt="node.attrs.alt || null"
        class="constructor-image"
        :style="imgStyle"
        draggable="false"
        data-drag-handle
      >
      <span
        v-for="handle in HANDLES"
        v-show="isEditable && selected"
        :key="handle.key"
        class="ci-resize-handle"
        :class="`ci-handle-${handle.key}`"
        title="Потяните, чтобы изменить размер (Shift - пропорционально)"
        @mousedown="(event) => startResize(event, handle)"
      />
    </span>
  </node-view-wrapper>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue';
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3';

const props = defineProps(nodeViewProps);

const MIN_SIZE = 40;

/**
 * 8 маркеров ресайза. `sx`/`sy` - знак влияния перемещения курсора на ширину/высоту:
 * ширина += sx * dx, высота += sy * dy. Для левого/верхнего ребра знак отрицательный,
 * чтобы движение наружу увеличивало размер (как в Word). Стороны меняют одну ось.
 */
const HANDLES = [
  { key: 'nw', sx: -1, sy: -1 },
  { key: 'n', sx: 0, sy: -1 },
  { key: 'ne', sx: 1, sy: -1 },
  { key: 'e', sx: 1, sy: 0 },
  { key: 'se', sx: 1, sy: 1 },
  { key: 's', sx: 0, sy: 1 },
  { key: 'sw', sx: -1, sy: 1 },
  { key: 'w', sx: -1, sy: 0 },
];

const imgEl = ref(null);
/** Размер во время перетаскивания (px); коммитим в атрибуты ноды только на mouseup. */
const dragSize = ref(null);
let stopDrag = null;

const isEditable = computed(() => Boolean(props.editor?.isEditable));

const alignClass = computed(() =>
  props.node.attrs.align ? `img-align-${props.node.attrs.align}` : ''
);

const imgStyle = computed(() => {
  const width = dragSize.value?.width ?? props.node.attrs.width;
  const height = dragSize.value?.height ?? props.node.attrs.height;
  const style = {};
  if (width) style.width = `${width}px`;
  if (height) style.height = `${height}px`;
  return style;
});

/** Верхняя граница ширины - доступная ширина области редактора (контент-бокс ProseMirror). */
function maxWidth() {
  const pm = imgEl.value?.closest('.ProseMirror');
  if (!pm) return Number.POSITIVE_INFINITY;
  const cs = window.getComputedStyle(pm);
  const padding = (parseFloat(cs.paddingLeft) || 0) + (parseFloat(cs.paddingRight) || 0);
  return Math.max(MIN_SIZE, pm.clientWidth - padding);
}

function startResize(event, handle) {
  if (!isEditable.value || !imgEl.value) return;
  if (stopDrag) stopDrag();
  // Гасим всплытие, чтобы перетаскивание маркера не запускало drag-перемещение ноды.
  event.preventDefault();
  event.stopPropagation();

  const rect = imgEl.value.getBoundingClientRect();
  const startX = event.clientX;
  const startY = event.clientY;
  const startWidth = rect.width;
  const startHeight = rect.height;
  const ratio = startHeight > 0 ? startWidth / startHeight : 1;
  const limit = maxWidth();

  const onMove = (e) => {
    let width = startWidth + handle.sx * (e.clientX - startX);
    let height = startHeight + handle.sy * (e.clientY - startY);

    if (e.shiftKey) {
      // Пропорция: сторона, меняющая только высоту, ведёт ширину; в остальных случаях ведёт ширина.
      if (handle.sx === 0) {
        width = height * ratio;
      } else {
        height = width / ratio;
      }
    }

    width = Math.max(MIN_SIZE, Math.min(Math.round(width), limit));
    height = Math.max(MIN_SIZE, Math.round(height));
    // При Shift держим пропорцию даже у границы редактора: высоту всегда пересчитываем из
    // финальной (возможно урезанной лимитом) ширины - симметрично для угловых и боковых маркеров.
    if (e.shiftKey) {
      height = Math.max(MIN_SIZE, Math.round(width / ratio));
    }

    dragSize.value = { width, height };
  };

  const onUp = () => {
    if (dragSize.value) {
      props.updateAttributes({ width: dragSize.value.width, height: dragSize.value.height });
      dragSize.value = null;
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

.ci-image.img-align-left {
  float: left;
  margin: 0 14px 10px 0;
  max-width: 100%;
}

.ci-image.img-align-right {
  float: right;
  margin: 0 0 10px 14px;
  max-width: 100%;
}

.ci-image.img-align-center {
  display: block;
  float: none;
  margin: 10px auto;
  text-align: center;
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
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  cursor: move;
}

.ci-resize-handle {
  position: absolute;
  width: 12px;
  height: 12px;
  background: var(--accent);
  border: 2px solid var(--surface);
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.25);
  z-index: 1;
}

.ci-handle-nw { top: -6px; left: -6px; cursor: nwse-resize; }
.ci-handle-ne { top: -6px; right: -6px; cursor: nesw-resize; }
.ci-handle-se { right: -6px; bottom: -6px; cursor: nwse-resize; }
.ci-handle-sw { bottom: -6px; left: -6px; cursor: nesw-resize; }
.ci-handle-n { top: -6px; left: 50%; transform: translateX(-50%); cursor: ns-resize; }
.ci-handle-s { bottom: -6px; left: 50%; transform: translateX(-50%); cursor: ns-resize; }
.ci-handle-e { top: 50%; right: -6px; transform: translateY(-50%); cursor: ew-resize; }
.ci-handle-w { top: 50%; left: -6px; transform: translateY(-50%); cursor: ew-resize; }
</style>
