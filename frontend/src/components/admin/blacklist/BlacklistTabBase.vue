<template>
  <div class="bl-tab">
    <div class="bl-header">
      <div class="bl-header-left">
        <slot name="header-left" />
      </div>
      <div class="bl-header-controls">
        <BaseDropdown
          class="bl-archive-dropdown"
          :model-value="showArchive ? 'archive' : 'active'"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="searchPlaceholder"
        />
        <button
          class="lk-button lk-button--primary"
          @click="$emit('create')"
        >
          Создать запись
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="fetchData"
        />
      </div>
    </div>

    <div class="bl-content">
      <div
        class="bl-list-section"
        :class="{ 'with-details': selected }"
      >
        <div class="bl-list">
          <div
            v-for="item in filteredItems"
            :key="item.id"
            class="bl-row"
            :class="{ selected: selected && selected.id === item.id, inactive: !item.is_active }"
            @click="selectItem(item)"
          >
            <span class="bl-row-id">{{ item.id }}</span>
            <span
              class="bl-row-title"
              :title="getPrimaryText(item)"
            >
              {{ getPrimaryText(item) }}
              <span
                v-if="!item.is_active"
                class="bl-inactive-badge"
              >(архив)</span>
            </span>
          </div>

          <div
            v-if="!filteredItems.length && !isLoading"
            class="bl-empty"
          >
            {{ emptyText }}
          </div>
          <div
            v-if="isLoading && !items.length"
            class="bl-loading"
          >
            <LoaderSpinner label="Загрузка..." />
          </div>
        </div>

        <div class="bl-footer">
          {{ showArchive ? 'В архиве' : 'Всего' }}: {{ filteredItems.length }}
        </div>
      </div>

      <div
        v-if="selected"
        class="bl-details"
      >
        <div class="bl-details-header">
          <h3 class="bl-details-title">
            {{ getPrimaryText(selected) }}
          </h3>
          <div class="bl-details-actions">
            <span
              v-if="!selected.is_active"
              class="bl-archive-pill"
            >В архиве</span>
            <button
              class="lk-button lk-button--ghost"
              @click="$emit('history', selected)"
            >
              История
            </button>
            <button
              v-if="selected.is_active"
              class="lk-button lk-button--danger"
              @click="$emit('archive', selected)"
            >
              Снять с ЧС
            </button>
            <button
              v-else
              class="lk-button lk-button--secondary"
              @click="$emit('restore', selected)"
            >
              Вернуть в ЧС
            </button>
          </div>
        </div>
        <div class="bl-details-body">
          <div
            v-for="(row, idx) in getDetailRows(selected)"
            :key="idx"
            class="bl-field"
          >
            <span class="bl-field-label">{{ row.label }}</span>
            <span class="bl-field-value">{{ row.value || '-' }}</span>
          </div>
        </div>
      </div>
      <div
        v-else
        class="bl-no-selection"
      >
        <p>Выберите запись для просмотра</p>
      </div>
    </div>
  </div>
</template>

<script>
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Базовая вкладка чёрного списка (#443): шапка (архив-режим, поиск, обновление),
 * список слева + панель деталей справа. Сущность-специфика (тексты строк, поля
 * деталей, API-метод) приходит через props - так машины и люди переиспользуют один
 * layout. Действия create/archive/restore эмитятся наружу (обработка - в обёртке);
 * история - отдельный срез.
 */
export default {
  name: 'BlacklistTabBase',
  components: { BaseDropdown, SearchComponent, RefreshButton, LoaderSpinner },
  props: {
    searchPlaceholder: { type: String, default: 'Поиск...' },
    emptyNoun: { type: String, default: 'записей' },
    apiList: { type: Function, required: true },
    getPrimaryText: { type: Function, required: true },
    getDetailRows: { type: Function, required: true },
  },
  emits: ['count', 'create', 'archive', 'restore', 'history'],
  data() {
    return {
      items: [],
      searchQuery: '',
      showArchive: false,
      isLoading: false,
      selected: null,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
    };
  },
  computed: {
    filteredItems() {
      const q = this.searchQuery.trim().toLowerCase();
      return this.items.filter((item) => {
        if (this.showArchive ? item.is_active : !item.is_active) return false;
        if (!q) return true;
        const haystack = `${this.getPrimaryText(item)} ${item.reason || ''}`.toLowerCase();
        return haystack.includes(q);
      });
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по фильтру';
      return this.showArchive ? 'В архиве пусто' : `Записей нет`;
    },
  },
  mounted() {
    this.fetchData();
  },
  methods: {
    async fetchData() {
      this.isLoading = true;
      try {
        const data = await this.apiList({ includeArchived: true });
        this.items = Array.isArray(data) ? data : [];
        this.$emit('count', this.items.filter((i) => i.is_active).length);
        if (this.selected) {
          this.selected = this.items.find((i) => i.id === this.selected.id) || null;
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: this.emptyNoun, type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    onArchiveModeChange(value) {
      this.showArchive = value === 'archive';
      this.selected = null;
    },
    selectItem(item) {
      this.selected = item;
    },
  },
};
</script>

<style scoped>
.bl-tab {
  display: flex;
  flex-direction: column;
}

.bl-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid #e6e6e6;
  gap: 12px;
}

.bl-header-left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.bl-header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-shrink: 0;
}

.bl-content {
  display: flex;
  height: 460px;
  width: 100%;
  overflow: hidden;
}

.bl-list-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
  background: #fff;
}

.bl-list {
  flex: 1;
  overflow-y: auto;
}

.bl-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 20px;
  height: 42px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  font-size: 14px;
  transition: background-color 0.2s ease;
}

.bl-row:hover {
  background-color: #fafafa;
}

.bl-row.selected {
  background-color: #f8f9ff;
}

.bl-row.inactive {
  background: #fafafa;
  color: #6b7280;
}

.bl-row-id {
  min-width: 32px;
  color: #a2a2a2;
  font-size: 13px;
}

.bl-row-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bl-inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: #a2a2a2;
  font-style: italic;
}

.bl-empty,
.bl-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  color: #a2a2a2;
  font-size: 14px;
}

.bl-footer {
  height: 42px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  border-top: 1px solid #e6e6e6;
  color: #6b7280;
  font-size: 13px;
}

.bl-details {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow-y: auto;
}

.bl-details-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 20px;
  border-bottom: 1px solid #e6e6e6;
}

.bl-details-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #000;
}

.bl-archive-pill {
  background: #6b7280;
  color: #fff;
  padding: 4px 10px;
  border-radius: 50px;
  font-size: 0.75em;
  font-weight: 500;
  white-space: nowrap;
}

.bl-details-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.bl-details-body {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.bl-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.bl-field-label {
  font-size: 12px;
  color: #a2a2a2;
}

.bl-field-value {
  font-size: 14px;
  color: #333;
  white-space: pre-wrap;
  word-break: break-word;
}

.bl-no-selection {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-size: 14px;
}
</style>
