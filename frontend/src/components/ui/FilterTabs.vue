<template>
  <div class="filter-tabs">
    <button
      v-for="tab in visibleTabs"
      :key="tab.key"
      class="filter-tab"
      :class="{ 'filter-tab--active': modelValue === tab.key }"
      :data-testid="`filter-tab-${tab.key}`"
      @click="$emit('update:modelValue', tab.key)"
    >
      <span class="filter-tab__label">{{ tab.label }}</span>
      <Badge
        v-if="tab.count != null"
        :label="String(tab.count)"
        size="sm"
        variant="neutral"
      />
    </button>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';

export default {
  name: 'FilterTabs',
  components: { Badge },
  props: {
    tabs: {
      type: Array,
      required: true,
    },
    modelValue: {
      type: String,
      required: true,
    },
  },
  emits: ['update:modelValue'],
  computed: {
    visibleTabs() {
      return this.tabs.filter((tab) => tab.visible !== false);
    },
  },
};
</script>

<style scoped>
.filter-tabs {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-tab {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 14px;
  height: 30px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 50px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  white-space: nowrap;
}

.filter-tab:hover {
  border-color: var(--accent);
}

.filter-tab--active {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}
</style>
