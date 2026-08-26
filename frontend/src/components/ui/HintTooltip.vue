<template>
  <span
    ref="anchorEl"
    v-bind="$attrs"
    class="hint-tooltip"
    :class="{ 'hint-tooltip--icon': !hasAnchorSlot }"
    tabindex="0"
    role="note"
    :aria-label="text"
    @mouseenter="open"
    @mouseleave="close"
    @focus="open"
    @blur="close"
  ><slot>i</slot></span>
  <Teleport to="body">
    <div
      v-if="visible"
      ref="bubbleEl"
      class="hint-tooltip__bubble"
      role="tooltip"
      :style="bubbleStyle"
    >
      {{ text }}
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, nextTick, onBeforeUnmount, useSlots } from 'vue';
import { getViewportZoom } from '@/utils/viewportScale';

// Шаблон двухкорневой (якорь + Teleport), автоматический fallthrough в таком
// случае не работает: без явного v-bind атрибуты потребителя (data-testid, aria-*,
// обработчики) молча теряются.
defineOptions({ inheritAttrs: false });

/*
 * Подсказка «что считается» в стиле проекта (#333), но пузырёк рендерится в <body>
 * с position:fixed, а не псевдоэлементом ::after у иконки.
 *
 * Почему teleport: страницы админки лежат в контейнерах с overflow (замерено -
 * .statistics__body auto, .statistics.dashboard-card hidden, .admin-page auto), и
 * абсолютный ::after ими обрезался - подсказка уезжала под край карточки (#1251
 * polish, п.1). Fixed-пузырёк в body не обрезается ничем.
 */
const props = defineProps({
  text: { type: String, required: true },
  /** Максимальная ширина пузырька, px (клампится по вьюпорту). */
  width: { type: Number, default: 260 },
});

/*
 * Якорем может быть либо собственная иконка «i» (по умолчанию), либо содержимое
 * слота - тогда подсказка висит прямо на значении, без лишнего значка в строке.
 * Круглые «иконочные» стили в этом случае не применяются, иначе значение оказалось
 * бы в кружке 14px.
 */
const slots = useSlots();
const hasAnchorSlot = computed(() => Boolean(slots.default));

const anchorEl = ref(null);
const bubbleEl = ref(null);
const visible = ref(false);
const bubbleStyle = ref({});

const GAP = 8; // зазор между иконкой и пузырьком
const EDGE = 8; // минимальный отступ от краёв вьюпорта
// Ниже глобальных диалогов (ConfirmDialog 20000) - подсказка не должна перекрывать
// блокирующие окна, но обязана быть выше контента страницы.
const Z_INDEX = 15000;

function bubbleWidth(zoom) {
  const z = zoom || getViewportZoom();
  return Math.min(props.width, window.innerWidth / z - EDGE * 2);
}

/*
 * getBoundingClientRect отдаёт device-px, а innerWidth/innerHeight - незумленные;
 * И координаты якоря, И размеры вьюпорта делим на zoom, чтобы кламп считался в
 * одной системе координат (тот же приём, что в BaseDropdown). Если поделить только
 * якорь, на 1920x1080 (проект ставит zoom 1.2) кламп идёт по чужой ширине и
 * пузырёк у правой плитки вылезает за край экрана.
 */
function updatePosition() {
  const el = anchorEl.value;
  if (!el) return;
  const z = getViewportZoom();
  const r = el.getBoundingClientRect();
  const anchorTop = r.top / z;
  const anchorBottom = r.bottom / z;
  const anchorCenter = (r.left + r.width / 2) / z;

  const vw = window.innerWidth / z;
  const vh = window.innerHeight / z;
  const w = bubbleWidth(z);
  const h = bubbleEl.value ? bubbleEl.value.offsetHeight : 0;

  // Над иконкой; если сверху не влезает - под ней. И в любом случае не ниже
  // нижнего края: длинная подсказка при флипе вниз иначе уезжает за экран.
  const above = anchorTop - GAP - h;
  const top = Math.min(
    above >= EDGE ? above : anchorBottom + GAP,
    Math.max(EDGE, vh - h - EDGE),
  );

  bubbleStyle.value = {
    position: 'fixed',
    width: `${w}px`,
    left: `${Math.max(EDGE, Math.min(anchorCenter - w / 2, vw - w - EDGE))}px`,
    top: `${top}px`,
    zIndex: Z_INDEX,
  };
}

async function open() {
  // Первый кадр - нужной ширины, но за экраном: чтобы замерить высоту до показа и
  // не мигнуть пузырьком в неправильном месте.
  bubbleStyle.value = {
    position: 'fixed',
    width: `${bubbleWidth()}px`,
    left: '0px',
    top: '-9999px',
    zIndex: Z_INDEX,
  };
  visible.value = true;
  await nextTick();
  updatePosition();
  window.addEventListener('scroll', updatePosition, true);
  window.addEventListener('resize', updatePosition);
}

function close() {
  visible.value = false;
  window.removeEventListener('scroll', updatePosition, true);
  window.removeEventListener('resize', updatePosition);
}

onBeforeUnmount(close);
</script>

<style scoped>
.hint-tooltip {
  display: inline-flex;
  align-items: center;
  cursor: help;
  user-select: none;
}

/* Собственная иконка «i» - когда якорь не передан слотом. */
.hint-tooltip--icon {
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  color: var(--color-text-muted);
  font-size: 9px;
  font-weight: 700;
  font-style: normal;
  line-height: 1;
  flex-shrink: 0;
}

.hint-tooltip--icon:hover,
.hint-tooltip--icon:focus {
  border-color: var(--accent);
  color: var(--accent-text);
  outline: none;
}

/* Слотовый якорь фокус-кольцо всё же получает: значение в ячейке - единственный
   способ добраться до подсказки с клавиатуры. */
.hint-tooltip:not(.hint-tooltip--icon):focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  border-radius: 4px;
}
</style>

<style>
/* Пузырёк живёт в <body> (Teleport) - scoped-хэш до него не достаёт. */
.hint-tooltip__bubble {
  padding: 8px 10px;
  background: var(--hint-bg);
  color: var(--hint-text);
  font-size: 11px;
  font-weight: 400;
  line-height: 1.4;
  text-align: left;
  white-space: normal;
  border-radius: 8px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  pointer-events: none;
}
</style>
