<template>
  <BaseModal :show="show" :title="title" width="900px" @close="$emit('close')">
    <div class="selection-modal">
      <!-- Search -->
      <div class="selection-modal__search">
        <SearchComponent :title="searchPlaceholder" @search="handleSearch" />
      </div>

      <!-- Filter tabs -->
      <div v-if="tabs.length" class="selection-modal__filters">
        <div class="selection-modal__tabs">
          <button
            v-for="tab in visibleTabs"
            :key="tab.key"
            class="selection-modal__tab"
            :class="{ 'selection-modal__tab--active': activeFilter === tab.key }"
            @click="switchFilter(tab.key)"
          >
            {{ tab.label }}
          </button>
        </div>
        <span class="selection-modal__count" v-if="tempSelected.length">
          Выбрано: {{ tempSelected.length }}
        </span>
      </div>

      <!-- Table -->
      <div class="selection-modal__table">
        <div class="selection-modal__header">
          <div class="selection-modal__cell selection-modal__cell--checkbox"></div>
          <slot name="columns"></slot>
        </div>
        <div class="selection-modal__body">
          <div v-if="loading" class="selection-modal__loading">Загрузка...</div>
          <div v-else-if="!items.length" class="selection-modal__empty">{{ emptyText }}</div>
          <div
            v-else
            v-for="item in items"
            :key="item.id"
            class="selection-modal__row"
            :class="{
              'selection-modal__row--selected': isSelected(item),
              'selection-modal__row--disabled': isDisabled(item)
            }"
            @click="toggleItem(item)"
          >
            <div class="selection-modal__cell selection-modal__cell--checkbox">
              <input
                type="checkbox"
                :checked="isSelected(item)"
                :disabled="isDisabled(item)"
              />
            </div>
            <slot name="row" :item="item" :isSelected="isSelected(item)" :isDisabled="isDisabled(item)"></slot>
          </div>
        </div>
      </div>
    </div>

    <template #actions>
      <button class="selection-modal__cancel" @click="$emit('close')">Отмена</button>
      <button
        class="selection-modal__confirm"
        :disabled="!tempSelected.length"
        @click="confirmSelection"
      >
        {{ confirmLabel }} ({{ tempSelected.length }})
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from './BaseModal.vue'
import SearchComponent from '@/components/SearchComponent.vue'

export default {
  name: 'SelectionModal',
  components: { BaseModal, SearchComponent },
  props: {
    show: { type: Boolean, required: true },
    title: { type: String, default: 'Выбор' },
    searchPlaceholder: { type: String, default: 'Поиск...' },
    tabs: { type: Array, default: () => [] },
    items: { type: Array, required: true },
    loading: { type: Boolean, default: false },
    disabledIds: { type: Array, default: () => [] },
    confirmLabel: { type: String, default: 'Добавить' },
    emptyText: { type: String, default: 'Нет данных' },
  },
  emits: ['close', 'confirm', 'filter-change', 'search'],
  data() {
    return {
      tempSelected: [],
      activeFilter: '',
    }
  },
  computed: {
    visibleTabs() {
      return this.tabs.filter(tab => tab.visible !== false)
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.tempSelected = []
        this.activeFilter = this.tabs.length ? this.tabs[0].key : ''
      }
    },
  },
  methods: {
    toggleItem(item) {
      if (this.isDisabled(item)) return
      const idx = this.tempSelected.findIndex(i => i.id === item.id)
      if (idx >= 0) {
        this.tempSelected.splice(idx, 1)
      } else {
        this.tempSelected.push(item)
      }
    },
    isSelected(item) {
      return this.tempSelected.some(i => i.id === item.id)
    },
    isDisabled(item) {
      return this.disabledIds.includes(item.id)
    },
    confirmSelection() {
      this.$emit('confirm', this.tempSelected)
      this.$emit('close')
    },
    switchFilter(key) {
      this.activeFilter = key
      this.$emit('filter-change', key)
    },
    handleSearch(searchVariants) {
      this.$emit('search', searchVariants)
    },
  },
}
</script>

<style scoped>
.selection-modal__search {
  margin-bottom: 16px;
}

.selection-modal__filters {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.selection-modal__tabs {
  display: flex;
  gap: 8px;
}

.selection-modal__tab {
  padding: 6px 16px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.selection-modal__tab:hover {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.selection-modal__tab--active {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.selection-modal__count {
  font-size: 13px;
  color: #666;
}

.selection-modal__table {
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  overflow: hidden;
}

.selection-modal__header {
  display: flex;
  background: #f8f8f8;
  padding: 10px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid #e6e6e6;
}

.selection-modal__body {
  max-height: 300px;
  overflow-y: auto;
}

.selection-modal__row {
  display: flex;
  padding: 10px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.2s;
}

.selection-modal__row:last-child {
  border-bottom: none;
}

.selection-modal__row:hover {
  background: #f8f8ff;
}

.selection-modal__row--selected {
  background: #f0f1ff;
}

.selection-modal__row--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.selection-modal__cell {
  flex: 1;
  display: flex;
  align-items: center;
}

.selection-modal__cell--checkbox {
  flex: 0 0 40px;
  justify-content: center;
}

.selection-modal__loading,
.selection-modal__empty {
  padding: 30px;
  text-align: center;
  color: #999;
  font-size: 14px;
}

.selection-modal__cancel {
  padding: 10px 20px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.selection-modal__cancel:hover {
  background: #f5f5f5;
}

.selection-modal__confirm {
  padding: 10px 20px;
  background: #4F5BDF;
  color: #fff;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.2s;
}

.selection-modal__confirm:hover {
  opacity: 0.9;
}

.selection-modal__confirm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
