<template>
  <div
    :id="anchorId"
    class="collapsible-section dashboard-row"
    :class="{ 'is-collapsed': collapsed }"
  >
    <button
      type="button"
      class="collapsible-section__toggle"
      :aria-expanded="!collapsed"
      :aria-controls="contentId"
      :title="collapsed ? `Развернуть: ${title}` : `Свернуть: ${title}`"
      :aria-label="collapsed ? `Развернуть: ${title}` : `Свернуть: ${title}`"
      @click="collapsed = !collapsed"
    >
      <svg
        class="collapsible-section__chevron"
        :class="{ rotated: !collapsed }"
        viewBox="0 0 24 24"
        width="20"
        height="20"
        aria-hidden="true"
      >
        <path
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M6 9l6 6 6-6"
        />
      </svg>
    </button>
    <transition
      name="collapse"
      @enter="onEnter"
      @after-enter="onAfterEnter"
      @leave="onLeave"
    >
      <div
        v-show="!collapsed"
        :id="contentId"
        ref="content"
        class="collapsible-section__content"
      >
        <slot />
      </div>
    </transition>
  </div>
</template>

<script>
let uid = 0;

export default {
  name: 'CollapsibleSection',
  props: {
    title: { type: String, required: true },
    anchorId: { type: String, default: '' },
    initiallyCollapsed: { type: Boolean, default: false }
  },
  data() {
    uid++;
    return {
      collapsed: this.initiallyCollapsed,
      contentId: `collapsible-content-${uid}`
    };
  },
  methods: {
    onEnter(el) {
      el.style.height = 'auto';
      const target = el.scrollHeight + 'px';
      el.style.height = '0';
      requestAnimationFrame(() => { el.style.height = target; });
    },
    onAfterEnter(el) { el.style.height = ''; },
    onLeave(el) {
      el.style.height = el.scrollHeight + 'px';
      requestAnimationFrame(() => { el.style.height = '0'; });
    }
  }
};
</script>

<style scoped>
.collapsible-section {
  position: relative;
}

/* Кнопка-chevron в правом верхнем углу секции - не дублирует существующий
   заголовок внутри слота, остаётся над ним. */
.collapsible-section__toggle {
  position: absolute;
  top: 12px;
  right: 14px;
  z-index: 5;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid #e6e6e6;
  background: #fff;
  cursor: pointer;
  border-radius: 8px;
  color: #6e7280;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.collapsible-section__toggle:hover {
  background: #eef0ff;
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.collapsible-section__toggle:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.25);
}

.collapsible-section__chevron {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.collapsible-section__chevron.rotated {
  transform: rotate(180deg);
}

.collapsible-section__content {
  overflow: hidden;
}

.collapse-enter-active,
.collapse-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.collapsible-section.is-collapsed .collapsible-section__content {
  display: none;
}
</style>
