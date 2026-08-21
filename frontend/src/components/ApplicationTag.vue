<template>
  <Badge
    :variant="tag.variant"
    size="sm"
    class="rt-tag hint-anchor hint-anchor--below"
    :class="[
      `rt-tag--${tag.key}`,
      `rt-tag--mode-${mode}`,
      { 'rt-tag--with-icon': tag.iconInText }
    ]"
    :data-hint="tag.hint"
    :data-testid="tag.testid"
  >
    <svg
      class="rt-tag__icon"
      width="13"
      height="13"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    ><path
      v-for="(d, i) in paths"
      :key="i"
      :d="d"
    /></svg>
    <span class="rt-tag__text">{{ label }}</span>
    <span
      v-if="tag.key === 'questions'"
      class="rt-tag__q-dot"
      aria-hidden="true"
    />
  </Badge>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';

const TAG_ICONS = {
  chs: ['M12 3.3 1.9 20.6h20.2z', 'M12 9.4v4.2', 'M12 17.1h.01'],
  awaiting: ['M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0', 'M12 7v5l3 2'],
  questions: ['M4 5.5h16a1 1 0 0 1 1 1v8.5a1 1 0 0 1-1 1H9.5L5.5 20v-3.5H4a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1z'],
  supplement: ['M12 5v14', 'M5 12h14'],
  roof: ['M3 11l9-7 9 7', 'M5 10v9h14v-9'],
  parking: ['M8 4h8a4 4 0 0 1 4 4v8a4 4 0 0 1-4 4H8a4 4 0 0 1-4-4V8a4 4 0 0 1 4-4z', 'M9 16V8h3.2a2.4 2.4 0 0 1 0 4.8H9'],
  important: ['M12 2 15 8.6 22 9.3 16.8 14 18.3 21 12 17.3 5.7 21 7.2 14 2 9.3 9 8.6z'],
  files: ['M19.5 11l-7.8 7.8a4 4 0 0 1-5.7-5.7l8-8a2.5 2.5 0 0 1 3.6 3.6l-7.6 7.6a1 1 0 0 1-1.5-1.4l6.8-6.8'],
};

export default {
  name: 'ApplicationTag',
  components: { Badge },
  props: {
    /** Описание тега из buildApplicationTags. */
    tag: { type: Object, required: true },
    /** text - подпись целиком, count - иконка с числом, icon - только иконка. */
    mode: { type: String, default: 'text' },
  },
  computed: {
    paths() {
      return TAG_ICONS[this.tag.key] || [];
    },
    label() {
      return this.mode === 'count' ? this.tag.countText : this.tag.text;
    },
  },
};
</script>

<style scoped>
/* Тег сжимается сам, если раскладка промахнётся мимо реальной ширины колонки
   (шрифт не успел загрузиться, нестандартный масштаб): лучше ужать подпись, чем
   вылезти поверх колонки действий. Основной механизм - свёртка в layout. */
.rt-tag {
    gap: 0;
    min-width: 0;
    max-width: 100%;
    transition: padding 0.28s ease;
}

[data-theme="dark"] .rt-tag {
    border-color: currentColor;
}

.rt-tag__text {
    display: inline-block;
    min-width: 0;
    max-width: 150px;
    opacity: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    transition: max-width 0.28s ease, opacity 0.2s ease;
}

/* Иконка по умолчанию свёрнута: в текстовом режиме подпись говорит сама за себя.
   Раскрывается шириной и прозрачностью - display не анимируется. */
.rt-tag__icon {
    display: inline-block;
    width: 0;
    height: 13px;
    opacity: 0;
    overflow: hidden;
    flex-shrink: 0;
    transition: width 0.28s ease, opacity 0.2s ease;
}

.rt-tag--mode-count .rt-tag__icon,
.rt-tag--mode-icon .rt-tag__icon,
.rt-tag--with-icon .rt-tag__icon {
    width: 13px;
    opacity: 1;
    margin-right: 3px;
}

/* Свёрнутый тег - кружок с одной иконкой: подпись схлопнута, отступ под неё не нужен. */
.rt-tag--mode-icon.badge--sm {
    width: 23px;
    height: 23px;
    padding: 0;
    border-radius: 50%;
    justify-content: center;
}

.rt-tag--mode-icon .rt-tag__icon {
    margin-right: 0;
}

.rt-tag--mode-icon .rt-tag__text {
    max-width: 0;
    opacity: 0;
}

/* Файлы - серая скрепка без подписи: признак справочный, внимания не требует. */
.rt-tag--files {
    color: var(--text-muted);
    border-color: var(--border);
    background: var(--surface);
}

.rt-tag--questions {
    position: relative;
}

/* Маркер новых вопросов виден в любом режиме (#973). */
.rt-tag__q-dot {
    position: absolute;
    top: -3px;
    right: -3px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-danger);
    border: 1.5px solid var(--surface);
    pointer-events: none;
}
</style>
