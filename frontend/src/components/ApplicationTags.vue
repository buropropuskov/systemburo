<template>
  <div
    v-if="tags.length"
    class="application-tags"
  >
    <ApplicationTag
      v-for="entry in layout.visible"
      :key="entry.tag.key"
      :tag="entry.tag"
      :mode="entry.mode"
    />
    <!-- Хвост, которому не хватило места. По клику раскрывается списком под чипом:
         подсказки для этого мало - в ней теги были бы перечислением слов, а не тем,
         что пользователь ищет глазами в строке. -->
    <Badge
      v-if="layout.hidden.length"
      ref="moreChip"
      variant="neutral"
      size="sm"
      class="rt-tag rt-tag--more"
      :class="{ 'rt-tag--more-open': moreOpen }"
      role="button"
      tabindex="0"
      :aria-expanded="String(moreOpen)"
      :aria-label="`Показать ещё теги: ${layout.hidden.length}`"
      data-testid="center-tags-more"
      @click.stop="toggleMore"
      @keydown.enter.stop.prevent="toggleMore"
      @keydown.space.stop.prevent="toggleMore"
    >
      <span class="rt-tag__text">+{{ layout.hidden.length }}</span>
    </Badge>
    <Teleport to="body">
      <div
        v-if="moreOpen"
        ref="popover"
        class="tags-popover"
        :style="popoverStyle"
        data-testid="center-tags-popover"
        @click.stop
      >
        <ApplicationTag
          v-for="tag in layout.hidden"
          :key="tag.key"
          :tag="tag"
          mode="text"
        />
      </div>
    </Teleport>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';
import ApplicationTag from '@/components/ApplicationTag.vue';
import { buildApplicationTags, layoutApplicationTags, hiddenTagsPanelWidth } from '@/utils/applicationTags';
import { getViewportZoom } from '@/utils/viewportScale';

/**
 * Что панель тратит на себя помимо тегов: отступы 8px с каждой стороны, рамка
 * (box-sizing: border-box - она съедает ширину контента) и 2px запаса на
 * округление замеренных ширин. Без этого запаса расчёт говорит «влезает в строку»,
 * а браузер выдавливает последний тег на вторую - и получается ровно та рваная
 * раскладка, ради которой панель и считается.
 */
const POPOVER_PADDING = 20;
const POPOVER_GAP = 6;
const SCREEN_MARGIN = 8;
/** Предел ширины строки тегов: шире панель начинает читаться простынёй. */
const MAX_ROW_WIDTH = 420;
/** Оценка высоты панели, пока она не отрисована (первый кадр, jsdom). */
const POPOVER_ROW = 29;

export default {
  name: 'ApplicationTags',
  components: { Badge, ApplicationTag },
  props: {
    application: { type: Object, required: true },
    /**
     * Ширина колонки тегов в px. 0 = ограничения нет (мобильная карточка или замер
     * ещё не сделан) - теги показываются полным текстом с переносом.
     */
    availableWidth: { type: Number, default: 0 },
  },
  data() {
    return { moreOpen: false, popoverStyle: {} };
  },
  computed: {
    tags() {
      return buildApplicationTags(this.application);
    },
    layout() {
      return layoutApplicationTags(this.tags, this.availableWidth);
    },
    /**
     * Ширина панели: теги лежат в ней строкой и переносятся, поэтому ширину
     * подбираем под содержимое - одна строка, либо две примерно равные.
     */
    popoverWidth() {
      const limit = typeof window === 'undefined'
        ? MAX_ROW_WIDTH
        : Math.min(MAX_ROW_WIDTH, window.innerWidth - 2 * SCREEN_MARGIN - POPOVER_PADDING);
      return hiddenTagsPanelWidth(this.layout.hidden, limit) + POPOVER_PADDING;
    },
  },
  watch: {
    // Колонка стала шире (сняли закрепление меню, растянули окно) - скрытых тегов
    // может не остаться вовсе, и раскрытый список повис бы пустым над строкой.
    'layout.hidden.length'(count) {
      if (!count) this.closeMore();
    },
  },
  beforeUnmount() {
    this.closeMore();
  },
  methods: {
    toggleMore() {
      if (this.moreOpen) {
        this.closeMore();
        return;
      }
      this.moreOpen = true;
      this.$nextTick(this.positionPopover);
      document.addEventListener('mousedown', this.onDocumentDown);
      document.addEventListener('keydown', this.onKeydown);
      // capture: список скроллит .table-body, а не окно - без него поповер
      // остался бы висеть на месте, пока строка уезжает вверх.
      window.addEventListener('scroll', this.positionPopover, true);
      window.addEventListener('resize', this.positionPopover);
    },
    closeMore() {
      if (!this.moreOpen) return;
      this.moreOpen = false;
      document.removeEventListener('mousedown', this.onDocumentDown);
      document.removeEventListener('keydown', this.onKeydown);
      window.removeEventListener('scroll', this.positionPopover, true);
      window.removeEventListener('resize', this.positionPopover);
    },
    onDocumentDown(e) {
      const chip = this.$refs.moreChip?.$el;
      const popover = this.$refs.popover;
      if (chip?.contains(e.target) || popover?.contains(e.target)) return;
      this.closeMore();
    },
    onKeydown(e) {
      if (e.key === 'Escape') this.closeMore();
    },
    /**
     * Позиция списка под чипом. Координаты приводим к CSS-пикселям: под корневым
     * zoom (мониторы шире 1440) getBoundingClientRect отдаёт device-px, а
     * innerWidth/innerHeight остаются незумленными - без деления список уезжает
     * тем дальше, чем крупнее масштаб.
     */
    positionPopover() {
      const chip = this.$refs.moreChip?.$el;
      if (!chip) return;
      const zoom = getViewportZoom();
      const r = chip.getBoundingClientRect();
      const top = r.bottom / zoom;
      const bottom = r.top / zoom;
      const vw = window.innerWidth / zoom;
      const vh = window.innerHeight / zoom;
      // Высота нужна только для выбора стороны. Пока панель не отрисована, берём
      // оценку по числу тегов; дальше - её настоящий размер, он зависит от переноса.
      const rendered = this.$refs.popover?.getBoundingClientRect();
      const height = rendered?.height
        ? rendered.height / zoom
        : this.layout.hidden.length * POPOVER_ROW + POPOVER_PADDING;
      const width = this.popoverWidth;
      const openUp = top + POPOVER_GAP + height > vh - SCREEN_MARGIN && bottom - height > SCREEN_MARGIN;

      this.popoverStyle = {
        position: 'fixed',
        // Прижимаем к правому краю чипа: колонка тегов последняя перед действиями,
        // и список, выровненный по левому краю, вылезал бы за окно.
        left: `${Math.round(Math.max(SCREEN_MARGIN, Math.min(r.right / zoom - width, vw - width - SCREEN_MARGIN)))}px`,
        width: `${width}px`,
        ...(openUp
          ? { bottom: `${Math.round(vh - bottom + POPOVER_GAP)}px`, top: 'auto' }
          : { top: `${Math.round(top + POPOVER_GAP)}px`, bottom: 'auto' }),
        // Выше строк списка и его шапок, но ниже модалки заявки (10002) - список
        // тегов не должен всплывать над открытой карточкой.
        zIndex: 2000,
      };
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

/* Счётчик скрытых тегов держится нейтральным - он не признак заявки, а указатель,
   что за ним есть ещё. */
.rt-tag--more {
    color: var(--text-muted);
    cursor: pointer;
    user-select: none;
    transition: background-color 0.15s ease, color 0.15s ease;
}

.rt-tag--more:hover,
.rt-tag--more-open {
    color: var(--text);
    background: var(--surface-2);
}

.rt-tag--more:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
}

/* Список скрытых тегов под чипом: теги идут строкой и переносятся на вторую, когда
   не помещаются - ширину под это считает popoverWidth. Позицию задаёт
   positionPopover, teleport в body уводит панель из-под overflow списка. */
.tags-popover {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    align-content: flex-start;
    gap: 6px 4px;
    padding: 8px;
    background: var(--surface);
    border: 1px solid var(--color-border);
    border-radius: 15px;
    box-shadow: 0 6px 20px var(--shadow-drop);
    animation: tags-popover-in 0.15s ease-out;
}

@keyframes tags-popover-in {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
}

/* В карточке на мобилке колонка тегов занимает всю ширину строки - ограничения нет,
   теги идут полным текстом и переносятся. */
@media (max-width: 767.98px) {
    .application-tags {
        flex-wrap: wrap;
    }
}
</style>
