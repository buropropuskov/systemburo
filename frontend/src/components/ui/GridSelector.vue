<template>
  <div class="grid-selector">
    <div v-if="loading" class="grid-selector__loading">Загрузка...</div>
    <div v-else class="grid-selector__grid">
      <div
        v-for="item in items"
        :key="item[itemKey]"
        class="grid-selector__item"
        :class="{
          'grid-selector__item--active': isSelected(item) && isActive(item),
          'grid-selector__item--attached': isAttached(item),
          'grid-selector__item--inactive': !isActive(item)
        }"
        @click="toggleItem(item)"
        @mouseenter="showTooltip($event, item)"
        @mouseleave="hideTooltip"
      >
        {{ item[itemLabel] }}
      </div>
    </div>
    <div v-if="errorMessage" class="grid-selector__error">{{ errorMessage }}</div>

    <div
      v-if="tooltip.visible"
      class="grid-selector__tooltip"
      :style="{ top: tooltip.y + 'px', left: tooltip.x + 'px' }"
    >
      {{ tooltip.text }}
    </div>
  </div>
</template>

<script>
export default {
  name: 'GridSelector',
  props: {
    items: { type: Array, required: true },
    modelValue: { type: Array, default: () => [] },
    attachedIds: { type: Array, default: () => [] },
    itemKey: { type: String, default: 'id' },
    itemLabel: { type: String, default: 'name' },
    itemStatus: { type: String, default: 'status' },
    statusComment: { type: String, default: 'status_comment' },
    loading: { type: Boolean, default: false },
    errorMessage: { type: String, default: '' }
  },
  emits: ['update:modelValue'],
  data() {
    return {
      tooltip: {
        visible: false,
        text: '',
        x: 0,
        y: 0
      }
    }
  },
  methods: {
    isSelected(item) {
      return this.modelValue.includes(item[this.itemKey])
    },
    isActive(item) {
      const status = item[this.itemStatus]
      return status === true || status === 1 || status === 'active'
    },
    isAttached(item) {
      return this.attachedIds.includes(item[this.itemKey])
    },
    toggleItem(item) {
      if (!this.isActive(item)) return

      const id = item[this.itemKey]
      const selected = [...this.modelValue]
      const index = selected.indexOf(id)

      if (index === -1) {
        selected.push(id)
      } else {
        selected.splice(index, 1)
      }

      this.$emit('update:modelValue', selected)
    },
    showTooltip(event, item) {
      if (this.isActive(item) || !item[this.statusComment]) return

      this.tooltip = {
        visible: true,
        text: item[this.statusComment],
        x: event.clientX + 12,
        y: event.clientY + 12
      }
    },
    hideTooltip() {
      this.tooltip.visible = false
    }
  }
}
</script>

<style scoped>
.grid-selector__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.grid-selector__item {
  padding: 12px 16px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  cursor: pointer;
  text-align: center;
  font-size: 13px;
  color: #333;
  background: #fff;
  transition: all 0.2s;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.grid-selector__item:hover:not(.grid-selector__item--active):not(.grid-selector__item--inactive) {
  border-color: #4F5BDF;
  background: #f0f2ff;
}

.grid-selector__item--active {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.grid-selector__item--attached {
  border-left: 3px solid #4F5BDF;
}

.grid-selector__item--inactive {
  opacity: 0.5;
  cursor: not-allowed;
  background: #f5f5f5;
}

.grid-selector__loading {
  font-size: 12px;
  color: #a2a2a2;
  text-align: center;
  padding: 20px;
}

.grid-selector__error {
  color: #ef4444;
  font-size: 12px;
  margin-top: 8px;
}

.grid-selector__tooltip {
  position: fixed;
  background: #333;
  color: #fff;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
  z-index: 2000;
  pointer-events: none;
  max-width: 250px;
  line-height: 1.4;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}
</style>
