<template>
  <div
    v-if="tags.length"
    class="application-tags"
  >
    <Badge
      v-for="entry in layout.visible"
      :key="entry.tag.key"
      :variant="entry.tag.variant"
      size="sm"
      class="rt-tag hint-anchor hint-anchor--below"
      :class="[
        `rt-tag--${entry.tag.key}`,
        `rt-tag--mode-${entry.mode}`,
        { 'rt-tag--with-icon': entry.tag.iconInText }
      ]"
      :data-hint="entry.tag.hint"
      :data-testid="entry.tag.testid"
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
        v-for="(d, i) in icons[entry.tag.key]"
        :key="i"
        :d="d"
      /></svg>
      <span class="rt-tag__text">{{ entry.mode === 'count' ? entry.tag.countText : entry.tag.text }}</span>
      <span
        v-if="entry.tag.key === 'questions'"
        class="rt-tag__q-dot"
        aria-hidden="true"
      />
    </Badge>
    <!-- Хвост, которому не хватило места: перечень уходит в подсказку, чтобы ни один
         тег не пропал молча. -->
    <Badge
      v-if="layout.hidden.length"
      variant="neutral"
      size="sm"
      class="rt-tag rt-tag--more hint-anchor hint-anchor--below"
      :data-hint="hiddenHint"
      data-testid="center-tags-more"
    >
      <span class="rt-tag__text">+{{ layout.hidden.length }}</span>
    </Badge>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';
import { buildApplicationTags, layoutApplicationTags, hiddenTagsHint } from '@/utils/applicationTags';

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
  name: 'ApplicationTags',
  components: { Badge },
  props: {
    application: { type: Object, required: true },
    /**
     * Ширина колонки тегов в px. 0 = ограничения нет (мобильная карточка или замер
     * ещё не сделан) - теги показываются полным текстом с переносом.
     */
    availableWidth: { type: Number, default: 0 },
  },
  data() {
    return { icons: TAG_ICONS };
  },
  computed: {
    tags() {
      return buildApplicationTags(this.application);
    },
    layout() {
      return layoutApplicationTags(this.tags, this.availableWidth);
    },
    hiddenHint() {
      return hiddenTagsHint(this.layout.hidden);
    },
  },
};
</script>

<style scoped>
/* Теги строки заявки в одну строку без переноса: колонка узкая, а высота строки
   таблицы общая для всех колонок. Что не влезло - свёрнуто в иконки или в "+N"
   (раскладку считает layoutApplicationTags по реальной ширине колонки). */
.application-tags {
    display: flex;
    gap: 4px;
    flex-wrap: nowrap;
    align-items: center;
    min-width: 0;
}

/* Страховка на случай, если раскладка промахнётся мимо реальной ширины (шрифт не
   успел загрузиться, нестандартный масштаб): тег ужмётся сам, а не вылезет поверх
   колонки действий. Основной механизм - свёртка, сюда дело доходить не должно. */
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

/* Счётчик скрытых тегов держится нейтральным - он не признак заявки, а указатель,
   что за ним есть ещё. */
.rt-tag--more {
    color: var(--text-muted);
    cursor: default;
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

/* В карточке на мобилке колонка тегов занимает всю ширину строки - ограничения нет,
   теги идут полным текстом и переносятся. */
@media (max-width: 767.98px) {
    .application-tags {
        flex-wrap: wrap;
    }
}
</style>
