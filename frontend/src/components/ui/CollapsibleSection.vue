<template>
  <div
    :id="anchorId"
    class="collapsible-section dashboard-row"
    :class="{ 'is-collapsed': collapsed }"
  >
    <button
      type="button"
      class="collapsible-section__bar"
      :aria-expanded="!collapsed"
      :aria-controls="contentId"
      @click="collapsed = !collapsed"
    >
      <span class="collapsible-section__title">{{ title }}</span>
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

.collapsible-section__bar {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 16px;
  cursor: pointer;
  color: #1f2937;
  font-family: inherit;
  font-size: 16px;
  font-weight: 600;
  text-align: left;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.collapsible-section__bar:hover {
  background: #f8f9ff;
  border-color: #c5cbf4;
}

.collapsible-section__bar:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.25);
}

.collapsible-section:not(.is-collapsed) .collapsible-section__bar {
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
  border-bottom: 1px solid #f3f4f6;
}

.collapsible-section__title {
  flex: 1;
}

.collapsible-section__chevron {
  flex-shrink: 0;
  color: #6e7280;
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

/* Внутри CollapsibleSection заголовок секции живёт в полосе-шапке -
   убираем дубликаты-заголовки в Management-компонентах. */
.collapsible-section :deep(.management-title) {
  display: none;
}
</style>
