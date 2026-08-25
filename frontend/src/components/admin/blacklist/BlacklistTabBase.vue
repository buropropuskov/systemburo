<template>
  <div class="bl-tab">
    <div class="bl-header rt-header-inline">
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
          class="lk-button lk-button--primary rt-btn-compact"
          aria-label="Создать запись"
          @click="$emit('create')"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Создать запись</span>
        </button>
        <button
          class="lk-button lk-button--ghost"
          @click="$emit('history-all')"
        >
          История
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="fetchData"
        />
      </div>
    </div>

    <div
      v-if="bulkEnabled && selectedIds.length"
      class="bl-bulk-bar"
      :data-testid="`${testidPrefix}-bulk-bar`"
    >
      <span class="bl-bulk-count">Выбрано: {{ selectedIds.length }}</span>
      <div class="bl-bulk-actions">
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          :data-testid="`${testidPrefix}-bulk-archive`"
          @click="startBulkOperation('archive')"
        >
          Убрать из ЧС
        </button>
        <button
          v-else
          class="pill pill-restore"
          :data-testid="`${testidPrefix}-bulk-restore`"
          @click="startBulkOperation('restore')"
        >
          Вернуть в ЧС
        </button>
        <button
          class="pill pill-ghost bl-bulk-clear"
          :data-testid="`${testidPrefix}-bulk-clear`"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="bl-content">
      <div
        class="bl-list-section"
        :class="{ 'with-details': selected }"
      >
        <div
          v-if="bulkEnabled && filteredItems.length"
          class="bl-list-toolbar"
        >
          <label class="bl-select-all">
            <input
              type="checkbox"
              class="bulk-check"
              :checked="allSelected"
              :indeterminate.prop="someSelected"
              aria-label="Выбрать все"
              :data-testid="`${testidPrefix}-select-all`"
              @change="toggleSelectAll"
            >
            Выбрать все
          </label>
        </div>
        <div class="bl-list">
          <div
            v-for="(item, index) in filteredItems"
            :key="item.id"
            class="bl-row"
            :class="{ selected: selected && selected.id === item.id, inactive: !item.is_active }"
            @click="selectItem(item)"
          >
            <div
              v-if="bulkEnabled"
              class="bl-row-check"
              @click.stop
            >
              <input
                type="checkbox"
                class="bulk-check"
                :checked="isSelected(item.id)"
                :aria-label="`Выбрать ${getPrimaryText(item)}`"
                :data-testid="`${testidPrefix}-row-check`"
                @click="onRowCheck(item, index, $event)"
              >
            </div>
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
          <div class="bl-details-heading">
            <div
              v-if="entityIcon"
              class="bl-details-icon"
            >
              <AppIcon
                :name="entityIcon"
                class="bl-details-icon-img"
              />
            </div>
            <h3 class="bl-details-title">
              {{ getPrimaryText(selected) }}
            </h3>
          </div>
          <div class="bl-details-actions">
            <button
              v-if="lookupCard"
              class="lk-button lk-button--secondary"
              :disabled="cardLoading || !cardEntity"
              :title="cardButtonTitle"
              @click="$emit('open-card', cardEntity)"
            >
              Открыть карточку
            </button>
            <button
              v-if="selected.is_active"
              class="lk-button lk-button--danger"
              @click="$emit('archive', selected)"
            >
              Убрать из ЧС
            </button>
            <template v-else>
              <button
                class="lk-button lk-button--secondary"
                @click="$emit('restore', selected)"
              >
                Вернуть в ЧС
              </button>
              <button
                class="lk-button lk-button--danger"
                @click="$emit('purge', selected)"
              >
                Удалить навсегда
              </button>
            </template>
          </div>
        </div>
        <div class="bl-details-body">
          <div class="bl-status-row">
            <div
              class="bl-status-banner"
              :class="selected.is_active ? 'is-active' : 'is-archived'"
            >
              <span class="bl-status-dot" />
              {{ selected.is_active ? 'В чёрном списке' : 'Снято с ЧС - в архиве' }}
            </div>
            <button
              v-if="selected.is_active"
              class="lk-button lk-button--secondary"
              @click="$emit('edit', selected)"
            >
              Редактировать
            </button>
          </div>

          <div
            v-if="reasonRow"
            class="bl-reason"
          >
            <span class="bl-reason-label">{{ reasonRow.label }}</span>
            <p class="bl-reason-text">
              {{ reasonRow.value || '-' }}
            </p>
          </div>

          <dl
            v-if="metaRows.length"
            class="bl-def-list"
          >
            <div
              v-for="(row, idx) in metaRows"
              :key="idx"
              class="bl-def-row"
            >
              <dt class="bl-def-label">
                {{ row.label }}
              </dt>
              <dd class="bl-def-value">
                {{ row.value || '-' }}
              </dd>
            </div>
          </dl>
        </div>
      </div>
      <div
        v-else
        class="bl-no-selection"
      >
        <p>Выберите запись для просмотра</p>
      </div>
    </div>

    <ConfirmationModal
      :show="bulkConfirmVisible"
      :title="bulkConfirmTitle"
      :message="bulkConfirmMessage"
      :confirm-text="bulkConfirmText"
      cancel-text="Отмена"
      :confirm-button-style="bulkConfirmButtonStyle"
      @confirm="applyBulkArchiveRestore"
      @cancel="cancelBulkConfirm"
    />
  </div>
</template>

<script>
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { readSearchFromRoute } from '@/utils/searchQueryParam';
import { openItemFromRoute } from '@/utils/openQueryParam';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

/**
 * Базовая вкладка чёрного списка (#443): шапка (архив-режим, поиск, обновление),
 * список слева + панель деталей справа. Сущность-специфика (тексты строк, поля
 * деталей, API-метод) приходит через props - так машины и люди переиспользуют один
 * layout. Действия create/archive/restore эмитятся наружу (обработка - в обёртке);
 * история - отдельный срез.
 */
export default {
  name: 'BlacklistTabBase',
  components: { BaseDropdown, SearchComponent, RefreshButton, LoaderSpinner, ConfirmationModal, AppIcon },
  props: {
    searchPlaceholder: { type: String, default: 'Поиск...' },
    emptyNoun: { type: String, default: 'записей' },
    // Имя глифа реестра appIcons (не путь к файлу): значок красится цветом текста.
    entityIcon: { type: String, default: '' },
    apiList: { type: Function, required: true },
    getPrimaryText: { type: Function, required: true },
    getDetailRows: { type: Function, required: true },
    // Опц. async (item) => entity|null. Если задан - в панели появляется кнопка
    // "Открыть карточку" (disabled, пока запись в реестре не найдена).
    lookupCard: { type: Function, default: null },
    // Групповые операции (#443 bulk): (ids) => Promise<BulkOpResult>. Обе функции
    // обязаны быть заданы вместе - без них колонка чекбоксов и bulk-bar не рендерятся.
    bulkArchiveFn: { type: Function, default: null },
    bulkRestoreFn: { type: Function, default: null },
    // Префикс data-testid для bulk-элементов - обе вкладки (машины/люди) смонтированы
    // одновременно (v-show в BlacklistView), общий "bl-" дал бы дублирующиеся testid.
    testidPrefix: { type: String, default: 'bl' },
    // Своя вкладка ('vehicles'/'persons'): обе смонтированы разом, а id записей у них
    // независимые - без этого переход из поиска раскрывал бы запись и в чужой вкладке.
    tabKey: { type: String, default: '' },
    // Существительное во мн.ч. для предупреждения о каскаде в confirm-сообщениях
    // ("сотрудники"/"машины" - кого затронет архивация/восстановление).
    cascadeNounPlural: { type: String, default: 'записи' },
  },
  emits: ['count', 'create', 'archive', 'restore', 'history-all', 'open-card', 'edit', 'purge'],
  data() {
    return {
      items: [],
      // Из адреса: переход из сквозного поиска приносит запрос с собой.
      searchQuery: readSearchFromRoute(this.$route),
      showArchive: false,
      isLoading: false,
      selected: null,
      cardEntity: null,
      cardLoading: false,
      cardReqId: 0,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      // Групповой выбор (по id). lastSelectedId - якорь shift-диапазона.
      selectedIds: [],
      lastSelectedId: null,
      pendingBulkOp: null,
      bulkConfirmVisible: false,
      bulkSubmitting: false,
    };
  },
  computed: {
    filteredItems() {
      const variants = buildSearchVariants(this.searchQuery);
      return this.items.filter((item) => {
        if (this.showArchive ? item.is_active : !item.is_active) return false;
        if (!variants.length) return true;
        const haystack = `${this.getPrimaryText(item)} ${item.reason || ''}`;
        return matchesSearch(haystack, variants);
      });
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по фильтру';
      return this.showArchive ? 'В архиве пусто' : `Записей нет`;
    },
    detailData() {
      return this.selected ? this.getDetailRows(this.selected) : [];
    },
    reasonRow() {
      return this.detailData.find((r) => r.kind === 'reason') || null;
    },
    metaRows() {
      return this.detailData.filter((r) => r.kind !== 'reason');
    },
    cardButtonTitle() {
      if (this.cardLoading) return 'Поиск записи в реестре...';
      if (!this.cardEntity) return 'Запись в реестре не найдена';
      return 'Открыть карточку с историей';
    },
    bulkEnabled() {
      return !!(this.bulkArchiveFn && this.bulkRestoreFn);
    },
    allSelected() {
      return this.filteredItems.length > 0 && this.selectedIds.length === this.filteredItems.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Вернуть в чёрный список' : 'Убрать из чёрного списка';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Вернуть выбранные записи (${n}) в чёрный список? Совпадающие активные ${this.cascadeNounPlural} будут заблокированы.`
        : `Убрать выбранные записи (${n}) из чёрного списка? Совпадающие ${this.cascadeNounPlural} с активной заявкой снова станут активными.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Вернуть' : 'Убрать';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#dc3545', borderColor: '#dc3545' };
    },
  },
  watch: {
    // Пользователь мог выбрать записи, затем сузить список поиском/архивным фильтром -
    // выбранные, ушедшие из видимого списка, убираем из selectedIds (иначе счётчик и
    // bulk-запрос включали бы невидимые строки).
    filteredItems() {
      this.pruneSelection();
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
        this.openFromSearchLink();
        this.$emit('count', this.items.filter((i) => i.is_active).length);
        if (this.selected) {
          this.selected = this.items.find((i) => i.id === this.selected.id) || null;
        }
        this.pruneSelection();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: this.emptyNoun, type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    onArchiveModeChange(value) {
      this.showArchive = value === 'archive';
      this.selected = null;
      this.cardEntity = null;
      this.cardLoading = false;
      this.cardReqId++; // инвалидируем in-flight лукап
      this.clearSelection();
    },
    selectItem(item) {
      this.selected = item;
      this.resolveCard(item);
    },

    /** Переход из сквозного поиска: `?tab` выбирает вкладку, `?open` - запись в ней. */
    openFromSearchLink() {
      // Компонент монтируют и в тестах без роутера - обращаться к query напрямую нельзя.
      if (this.$route?.query?.tab !== this.tabKey) return;
      openItemFromRoute({ router: this.$router, route: this.$route, items: this.items, open: this.selectItem });
    },
    async resolveCard(item) {
      this.cardEntity = null;
      if (!this.lookupCard || !item) {
        this.cardLoading = false;
        return;
      }
      // Счётчик запросов: применяем результат только последнего лукапа (защита от гонки
      // при быстром переключении строк - устаревший ответ не сбросит loading и не подставит
      // чужую запись).
      const reqId = ++this.cardReqId;
      this.cardLoading = true;
      try {
        const entity = await this.lookupCard(item);
        if (reqId === this.cardReqId) {
          this.cardEntity = entity || null;
          this.cardLoading = false;
        }
      } catch {
        if (reqId === this.cardReqId) {
          this.cardEntity = null;
          this.cardLoading = false;
        }
      }
    },

    // --- Групповой выбор (#443 bulk) ---
    isSelected(id) {
      return this.selectedIds.includes(id);
    },
    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },
    // onRowCheck: обычный клик - toggle; shift-клик - диапазон от якоря до текущей.
    onRowCheck(item, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== item.id) {
        const list = this.filteredItems;
        const anchor = list.findIndex((i) => i.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(item.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = item.id;
          return;
        }
      }
      this.toggleSelect(item.id);
      this.lastSelectedId = item.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.filteredItems.map((i) => i.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.filteredItems.map((i) => i.id));
      const pruned = this.selectedIds.filter((id) => visible.has(id));
      if (pruned.length !== this.selectedIds.length) this.selectedIds = pruned;
    },
    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      this.bulkConfirmVisible = true;
    },
    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },
    async applyBulkArchiveRestore() {
      const ids = [...this.selectedIds];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!ids.length || (op !== 'archive' && op !== 'restore')) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = op === 'archive' ? await this.bulkArchiveFn(ids) : await this.bulkRestoreFn(ids);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, ids.length)) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify, частичный -> ui.warning с
    // перечнем непрошедших. false при ошибке-envelope (держим модалку для повтора).
    handleBulkResult(op, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = op === 'restore' ? 'Возвращено в чёрный список: ' : 'Убрано из чёрного списка: ';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map((e) => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: label, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.fetchData();
      return true;
    },
  },
};
</script>

<style scoped>
.bl-tab {
  position: relative; /* контекст для оверлей-панели .bl-bulk-bar поверх шапки */
  display: flex;
  flex-direction: column;
}

.bl-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid var(--border);
  gap: 12px;
}

.bl-header-left {
  display: flex;
  align-items: center;
  gap: 16px;
  /* заголовок и табы не сжимаем - место на узкой ширине освобождают контролы справа */
  flex-shrink: 0;
}

.bl-header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
  /* группа контролов может сжиматься; её элементы по умолчанию НЕ сжимаются (ниже),
     кроме дропдауна "Активные" и поиска - они и уступают место на узкой ширине. */
  min-width: 0;
}

.bl-header-controls > * {
  flex-shrink: 0;
}

/* "Активные"/"Архив": до 150px, но сжимается до 90px чтобы освободить место. */
.bl-archive-dropdown {
  width: 150px;
  min-width: 90px;
  flex-shrink: 1;
}

/* Поиск сжимается с 220px до 110px (его внутренний input - width:100%). */
.bl-header-controls :deep(.search) {
  min-width: 110px;
  flex-shrink: 1;
}

/* Панель групповых операций - оверлей поверх .bl-header (не reflow, список не
   прыгает при выборе - урок #510). Высота = высоте шапки (50px). */
.bl-bulk-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint);
  overflow-x: auto;
  overflow-y: hidden;
}

.bl-bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}

.bl-bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}

.bl-bulk-actions .pill {
  flex: 0 0 auto;
  white-space: nowrap;
}

.pill {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 14px;
  border-radius: 50px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  font-family: inherit;
  white-space: nowrap;
  transition: background 0.2s, border-color 0.2s;
}

.pill-ghost {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.pill-ghost:hover {
  background: var(--accent-tint);
}

.bl-bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.bl-bulk-clear:hover {
  background: var(--surface-2);
}

.pill-danger {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.pill-danger:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.pill-restore {
  background: var(--success);
  color: var(--fill-text);
}

.pill-restore:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

/* Тулбар "Выбрать все" - над списком, не скроллится вместе с ним (список - без
   head-row, в отличие от Marks/TableConstructor - однострочные .bl-row без колонок). */
.bl-list-toolbar {
  display: flex;
  align-items: center;
  padding: 8px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}

.bl-select-all {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  user-select: none;
}

.bl-row-check {
  display: flex;
  align-items: center;
  cursor: default;
}

.bulk-check {
  width: 15px;
  height: 15px;
  cursor: pointer;
  accent-color: var(--accent-text);
  margin: 0;
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
  border-right: 1px solid var(--border);
  background: var(--surface);
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
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  font-size: 14px;
  transition: background-color 0.2s ease;
}

.bl-row:hover {
  background-color: var(--surface-2);
}

.bl-row.selected {
  background-color: var(--accent-tint);
}

.bl-row.inactive {
  background: var(--surface-2);
  color: var(--text-muted);
}

.bl-row-id {
  min-width: 32px;
  color: var(--text-muted);
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
  color: var(--text-muted);
  font-style: italic;
}

.bl-empty,
.bl-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  color: var(--text-muted);
  font-size: 14px;
}

.bl-footer {
  height: 42px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  border-top: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 13px;
}

.bl-details {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow-y: auto;
}

.bl-details-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.bl-details-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.bl-details-icon {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--color-bg);
  display: flex;
  align-items: center;
  justify-content: center;
}

.bl-details-icon-img {
  width: 20px;
  height: 20px;
  color: var(--text);
}

.bl-details-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
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
  gap: 16px;
}

.bl-status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.bl-status-banner {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-weight: 500;
}

.bl-status-banner.is-active {
  background: var(--danger-bg);
  color: var(--danger-text);
}

.bl-status-banner.is-archived {
  background: var(--accent-tint);
  color: var(--text-muted);
}

.bl-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.bl-status-banner.is-active .bl-status-dot {
  background: var(--color-danger);
}

.bl-status-banner.is-archived .bl-status-dot {
  background: var(--accent);
}

.bl-reason {
  padding: 12px 16px;
  background: var(--danger-bg);
  border-radius: var(--radius-md);
}

.bl-reason-label {
  display: block;
  margin-bottom: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--danger-text);
}

.bl-reason-text {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.bl-def-list {
  margin: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.bl-def-row {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 12px;
  padding: 11px 16px;
}

.bl-def-row:not(:last-child) {
  border-bottom: 1px solid var(--border);
}

.bl-def-label {
  font-size: 13px;
  color: var(--color-text-muted);
}

.bl-def-value {
  margin: 0;
  font-size: 14px;
  color: var(--color-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.bl-no-selection {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 14px;
}

/* Список записей ЧС - однострочные .bl-row (id+название), без head-row и колонок -
   rt-table/data-label не подходят (нечего скрывать/подписывать, см. responsive-tables.css).
   Master-detail 40/60 стекаем в колонку по образцу DocumentsManagement/CitizenshipManagement
   (#1097 S9), .bl-row получает границу и радиус карточки. */
@media (max-width: 767.98px) {
  .bl-header-controls {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  /* rt-header-inline может сделать шапку auto-высоты (перенос controls строкой
     ниже) - фиксированный оверлей bl-bulk-bar (height:50px) больше не накрывает
     её целиком, возвращаем панель в обычный поток (как в Marks/Orgs/Companies). */
  .bl-bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    overflow-x: visible;
  }

  .bl-bulk-actions {
    flex-wrap: wrap;
  }

  .bl-row-check {
    min-height: 44px;
  }

  .bl-select-all {
    min-height: 44px;
  }

  .bl-content {
    flex-direction: column;
    height: auto;
  }

  .bl-list-section,
  .bl-details,
  .bl-no-selection {
    width: 100%;
  }

  .bl-list-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }

  .bl-list {
    padding: 8px;
    max-height: 300px;
  }

  .bl-row {
    height: auto;
    min-height: 44px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 15px);
    margin-bottom: 8px;
  }

  .bl-row:last-child {
    margin-bottom: 0;
  }

  /* Десктопный flex-shrink:0 держит заголовок+табы одной недробимой единицей (место
     под узкую ширину раньше уступали только контролы справа) - на мобилке контролы
     уже перенесены целиком строкой ниже (rt-header-inline), но сам bl-header-left
     остаётся несжимаемым и вылезает за .blacklist-card{overflow:hidden} МОЛЧА (без
     скролла document, ловится только замером самого элемента, не scrollWidth). Даём
     ему сжиматься и растягиваться на всю строку - тогда title+FilterTabs (уже
     flex-wrap:wrap в BlacklistView.vue) реально переносятся, а не обрезаются. */
  .bl-header-left {
    flex-shrink: 1;
    min-width: 0;
    width: 100%;
  }

  .bl-details-header {
    flex-wrap: wrap;
  }

  .bl-details-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .bl-status-row {
    flex-wrap: wrap;
  }
}
</style>
