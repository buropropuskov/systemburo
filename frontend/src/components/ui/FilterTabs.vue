<template>
  <div class="filter-tabs">
    <button
      v-for="tab in visibleTabs"
      :key="tab.key"
      class="filter-tab"
      :data-testid="`filter-tab-${tab.key}`"
      :class="{ 'filter-tab--active': modelValue === tab.key }"
      @click="$emit('update:modelValue', tab.key)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>

<script>
export default {
  name: 'FilterTabs',
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
  padding: 0 16px;
  height: 30px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  white-space: nowrap;
}

.filter-tab:hover {
  border-color: #4F5BDF;
}

.filter-tab--active {
  background: #4F5BDF;
  color: white;
  border-color: #4F5BDF;
}
</style>
