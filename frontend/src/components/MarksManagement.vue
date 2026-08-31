<template>
  <div class="marks-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Управление марками автомобилей
      </h3>
      <div class="header-controls">
        <BaseDropdown
          class="archive-dropdown"
          :model-value="showArchive ? 'archive' : 'active'"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск марок...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          data-testid="marks-add-btn"
          aria-label="Добавить"
          @click="openAddModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Добавить</span>
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refresh"
        />
      </div>
    </div>

    <div
      v-if="selectedIds.length"
      class="bulk-bar"
      data-testid="marks-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
      <div class="bulk-actions">
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          data-testid="marks-bulk-archive"
          @click="startBulkOperation('archive')"
        >
          В архив
        </button>
        <button
          v-else
          class="pill pill-restore"
          data-testid="marks-bulk-restore"
          @click="startBulkOperation('restore')"
        >
          Восстановить
        </button>
        <button
          class="pill pill-ghost bulk-clear"
          data-testid="marks-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список марок -->
      <div
        class="table-section"
        :class="{ 'with-details': selectedMark }"
      >
        <div class="table-container rt-table">
          <div class="table-header rt-head-row">
            <div
              class="header-col check-col"
              @click.stop
            >
              <input
                type="checkbox"
                class="bulk-check"
                :checked="allSelected"
                :indeterminate.prop="someSelected"
                aria-label="Выбрать все"
                data-testid="marks-select-all"
                @change="toggleSelectAll"
              >
            </div>
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{ sorted: sortField === 'id', desc: sortField === 'id' && sortDirection === 'desc' }"
              />
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('name')"
            >
              <p :class="{ 'active-sort': sortField === 'name' }">
                Наименование
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{ sorted: sortField === 'name', desc: sortField === 'name' && sortDirection === 'desc' }"
              />
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="(m, index) in filteredMarks"
              :key="m.id"
              class="table-row rt-row"
              data-testid="marks-row"
              :class="{
                selected: selectedMark && selectedMark.id === m.id,
                inactive: !m.is_active,
              }"
              @click="selectMark(m)"
            >
              <div
                class="table-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(m.id)"
                  :aria-label="`Выбрать ${m.name}`"
                  data-testid="marks-row-check"
                  @click="onRowCheck(m, index, $event)"
                >
              </div>
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ m.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="m.name"
                >
                  {{ m.name }}
                  <span
                    v-if="!m.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
            </div>

            <div
              v-if="!filteredMarks.length && !isLoading"
              class="no-results"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !marks.length"
              class="marks-loading"
            >
              <LoaderSpinner label="Загрузка марок..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего марок' }}: {{ filteredMarks.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали марки -->
      <div
        v-if="selectedMark"
        class="details-section"
        data-testid="marks-details"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ originalSelectedName }}
              </h3>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedMark.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                data-testid="marks-history"
                @click="openHistory(selectedMark)"
              >
                История
              </button>
              <button
                v-if="selectedMark.is_active"
                class="action-btn archive-action-btn"
                data-testid="marks-archive"
                @click="onArchiveClick(selectedMark)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                data-testid="marks-restore"
                @click="onRestore(selectedMark)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <div class="details-body">
            <label class="field-label">Название марки</label>
            <div class="name-edit-row">
              <input
                v-model.trim="selectedMark.name"
                type="text"
                class="lk-input"
                maxlength="100"
                :disabled="!selectedMark.is_active || isSavingName"
                data-testid="marks-detail-name"
                @keyup.enter="saveSelectedName"
              >
              <button
                v-if="selectedMark.is_active"
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSavingName"
                data-testid="marks-save-name"
                @click="saveSelectedName"
              >
                Сохранить
              </button>
            </div>
            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>
            <div class="details-meta">
              <span>ID: {{ selectedMark.id }}</span>
              <span v-if="selectedMark.created_at">Создана: {{ formatDate(selectedMark.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите марку для просмотра и редактирования</p>
      </div>
    </div>

    <!-- Модалка создания -->
    <BaseModal
      :show="showAddModal"
      title="Новая марка"
      width="440px"
      radius="30px"
      content-testid="marks-modal"
      @close="requestCloseAdd"
    >
      <div class="marks-modal-body">
        <div class="form-group">
          <label class="form-label">Название марки</label>
          <input
            v-model.trim="addForm.name"
            type="text"
            placeholder="Например, Toyota"
            maxlength="100"
            class="lk-input"
            data-testid="marks-input-name"
            @keyup.enter="submitAdd"
          >
        </div>
        <div
          v-if="addError"
          class="form-error"
        >
          {{ addError }}
        </div>
      </div>

      <template #actions>
        <button
          class="lk-button lk-button--ghost"
          data-testid="marks-modal-cancel"
          @click="requestCloseAdd"
        >
          Отмена
        </button>
        <button
          class="lk-button lk-button--primary"
          :disabled="!addForm.name || isAdding"
          data-testid="marks-modal-save"
          @click="submitAdd"
        >
          Добавить
        </button>
      </template>
    </BaseModal>

    <MarkHistoryModal
      v-if="historyForMark"
      :mark="historyForMark"
      :current-user-name="currentUserName"
      @close="historyForMark = null"
    />

    <ConfirmationModal
      :show="!!archiveConfirmMark"
      title="Архивация марки"
      :message="archiveConfirmMark ? `Архивировать марку «${archiveConfirmMark.name}»? Её можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirmMark = null"
    />

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
import SearchComponent from './SearchComponent.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { readSearchFromRoute } from '@/utils/searchQueryParam';
import RefreshButton from './RefreshButton.vue';
import MarkHistoryModal from './MarkHistoryModal.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import BaseModal from './ui/BaseModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { apiRequest } from '@/api/client';
import {
  listMarks,
  createMark,
  renameMark,
  archiveMark,
  restoreMark,
  bulkArchiveMarks,
  bulkRestoreMarks,
} from '@/api/marks';
import AppIcon from '@/components/icons/AppIcon.vue';
import { openFromSearchLink } from '@/mixins/openFromSearchLink'

export default {
  name: 'MarksManagement',
  components: { SearchComponent, RefreshButton, MarkHistoryModal, ConfirmationModal, BaseDropdown, BaseModal, LoaderSpinner, AppIcon },
  mixins: [openFromSearchLink((vm) => vm.marks, 'selectMark')],
  data() {
    return {
      marks: [],
      // Из адреса: переход из сквозного поиска приносит запрос с собой.
      searchQuery: readSearchFromRoute(this.$route),
      showArchive: false,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedMark: null,
      originalSelectedName: '',
      detailError: '',
      isSavingName: false,
      showAddModal: false,
      addForm: { name: '' },
      addError: '',
      isAdding: false,
      historyForMark: null,
      currentUserName: '',
      archiveConfirmMark: null,
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
    filteredMarks() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.marks.filter(m => (this.showArchive ? !m.is_active : m.is_active));
      if (variants.length) {
        list = list.filter(m => matchesSearch(`${m.name} ${m.id}`, variants));
      }
      return this.sortList(list);
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Марок пока нет';
    },
    isAddDirty() {
      return this.showAddModal && this.addForm.name.trim() !== '';
    },
    isDetailsDirty() {
      return !!this.selectedMark
        && this.selectedMark.is_active
        && this.selectedMark.name.trim() !== this.originalSelectedName;
    },
    isDirty() {
      return this.isAddDirty || this.isDetailsDirty;
    },
    allSelected() {
      return this.filteredMarks.length > 0 && this.selectedIds.length === this.filteredMarks.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление марок' : 'Архивация марок';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные марки (${n})?`
        : `Архивировать выбранные марки (${n})? Их можно будет восстановить из архива.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Восстановить' : 'В архив';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#c62828', borderColor: '#c62828' };
    },
  },
  watch: {
    // Пользователь мог выбрать марки, затем сузить список поиском/фильтром -
    // выбранные, ушедшие из видимого списка, убираем из selectedIds (иначе
    // счётчик и bulk-запрос включали бы невидимые строки).
    filteredMarks() {
      this.pruneSelection();
    },
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новая марка: "${this.addForm.name.trim()}"`];
        if (this.isDetailsDirty) {
          return [{ label: 'Название марки', from: this.originalSelectedName, to: this.selectedMark.name.trim() }];
        }
        return [];
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveSelectedName();
      },
    });
  },
  beforeUnmount() {
    this._stopGuard?.();
  },
  methods: {

    sortList(list) {
      const arr = [...list];
      if (!this.sortField) {
        return arr.sort((a, b) => a.name.localeCompare(b.name));
      }
      return arr.sort((a, b) => {
        if (this.sortField === 'id') {
          return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
        }
        const r = a.name.localeCompare(b.name);
        return this.sortDirection === 'asc' ? r : -r;
      });
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    formatDate(s) {
      if (!s) return '';
      return new Date(s).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    },
    async refresh() {
      this.isLoading = true;
      try {
        const data = await listMarks({ includeArchived: true });
        this.marks = Array.isArray(data) ? data : [];
        this.openFromSearchLink();
        // Подтянуть актуальные поля выбранной марки или снять выбор, если её больше нет в текущем фильтре.
        if (this.selectedMark) {
          const fresh = this.marks.find(m => m.id === this.selectedMark.id);
          const visible = fresh && (this.showArchive ? !fresh.is_active : fresh.is_active);
          if (fresh && visible && !this.isDetailsDirty) {
            this.selectedMark = { ...fresh };
            this.originalSelectedName = fresh.name;
          } else if (!visible) {
            this.selectedMark = null;
          }
        }
        this.pruneSelection();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'марки', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async fetchCurrentUser() {
      // Имя нужно для футера Excel-экспорта истории ("Отчёт сформировал").
      try {
        const res = await apiRequest('/users/me');
        if (!res.ok) return;
        const u = await res.json();
        const parts = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
        this.currentUserName = parts.join(' ') || u.username || '';
      } catch {
        // Имя - необязательная деталь экспорта, молчим (footer покажет дефолт).
      }
    },
    async onArchiveModeChange(value) {
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.showArchive = value === 'archive';
      this.selectedMark = null;
      this.detailError = '';
      this.clearSelection();
    },
    async selectMark(mark) {
      if (this.selectedMark && this.selectedMark.id === mark.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.selectedMark = { ...mark };
      this.originalSelectedName = mark.name;
      this.detailError = '';
    },
    async saveSelectedName() {
      if (!this.isDetailsDirty || this.isSavingName) return;
      const name = this.selectedMark.name.trim();
      this.isSavingName = true;
      this.detailError = '';
      try {
        await renameMark(this.selectedMark.id, { name });
        useDeletionsStore().notify({ prefix: 'Марка переименована в ', bold: name });
        this.originalSelectedName = name;
        this.selectedMark.name = name;
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSavingName = false;
      }
    },
    openAddModal() {
      this.showAddModal = true;
      this.addForm.name = '';
      this.addError = '';
    },
    async requestCloseAdd() {
      if (this.isAddDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseAdd();
    },
    forceCloseAdd() {
      this.showAddModal = false;
      this.addForm.name = '';
      this.addError = '';
    },
    async submitAdd() {
      const name = this.addForm.name.trim();
      if (this.isAdding) return;
      // Молча выходить нельзя: кроме кнопки (она заблокирована при пустом поле)
      // сюда приходит «Сохранить все изменения» из диалога несохранённого, и
      // тихий выход там читается как «нажал, и ничего не произошло».
      if (!name) {
        this.addError = 'Укажите название - без него марку не создать.';
        return;
      }
      this.isAdding = true;
      this.addError = '';
      try {
        await createMark({ name });
        useDeletionsStore().notify({ prefix: 'Марка ', bold: name, suffix: ' создана' });
        this.forceCloseAdd();
        await this.refresh();
      } catch (e) {
        this.addError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isAdding = false;
      }
    },
    onArchiveClick(mark) {
      this.archiveConfirmMark = mark;
    },
    async performArchive() {
      const mark = this.archiveConfirmMark;
      this.archiveConfirmMark = null;
      if (!mark) return;
      try {
        await archiveMark(mark.id);
        useDeletionsStore().notify({ prefix: 'Марка ', bold: mark.name, suffix: ' архивирована' });
        if (this.selectedMark && this.selectedMark.id === mark.id && !this.showArchive) {
          this.selectedMark = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async onRestore(mark) {
      try {
        await restoreMark(mark.id);
        useDeletionsStore().notify({ prefix: 'Марка ', bold: mark.name, suffix: ' восстановлена из архива' });
        if (this.selectedMark && this.selectedMark.id === mark.id && this.showArchive) {
          this.selectedMark = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    openHistory(mark) {
      this.historyForMark = mark;
    },

    // --- Групповой выбор ---
    isSelected(id) {
      return this.selectedIds.includes(id);
    },
    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },
    // onRowCheck: обычный клик - toggle; shift-клик - диапазон от якоря до текущей.
    onRowCheck(mark, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== mark.id) {
        const list = this.filteredMarks;
        const anchor = list.findIndex(m => m.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(mark.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = mark.id;
          return;
        }
      }
      this.toggleSelect(mark.id);
      this.lastSelectedId = mark.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.filteredMarks.map(m => m.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.filteredMarks.map(m => m.id));
      const pruned = this.selectedIds.filter(id => visible.has(id));
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
        result = op === 'archive' ? await bulkArchiveMarks(ids) : await bulkRestoreMarks(ids);
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
      const label = op === 'restore' ? 'Восстановлено' : 'Архивировано';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.refresh();
      return true;
    },
  },
};
</script>

<style scoped>
@import '@/assets/directory-management.css';

.marks-container {
  position: relative; /* контекст для оверлей-панели .bulk-bar поверх шапки */
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
}

/* Панель групповых операций - оверлей поверх .management-header (не reflow,
   список не прыгает при выборе - урок #510). Высота = высоте шапки (50px). */
.bulk-bar {
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
  background: var(--accent-tint-solid);
  overflow-x: auto;
  overflow-y: hidden;
}

.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}

.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}

.bulk-actions .pill {
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

.bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.bulk-clear:hover {
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

.check-col {
  width: 8%;
  min-width: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 8px;
  cursor: default;
}

.bulk-check {
  width: 15px;
  height: 15px;
  cursor: pointer;
  accent-color: var(--accent-text);
  margin: 0;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
  gap: 12px;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

/* Master-detail layout (эталон TableConstructor) */
.content-container {
  display: flex;
  height: 500px;
  width: 100%;
  overflow: hidden;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--surface);
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: 0.2s;
  cursor: pointer;
  user-select: none;
}

.header-col p {
  margin: 0;
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.id-col {
  width: 22%;
  min-width: 56px;
}

.name-col {
  width: 70%;
  min-width: 150px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row.inactive .id-value {
  color: var(--text-muted);
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.marks-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
}

/* Details */
.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow: hidden;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: var(--surface);
  line-height: 1.5;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  gap: 12px;
}

.details-title-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  min-width: 0;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.2em;
  font-weight: 600;
  word-break: break-word;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.history-btn {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.history-btn:hover {
  background: var(--accent-tint);
}

.archive-action-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.archive-action-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.restore-btn {
  background: var(--success);
  color: var(--fill-text);
}

.restore-btn:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.name-edit-row {
  display: flex;
  gap: 10px;
  align-items: center;
}

.name-edit-row .lk-input {
  flex: 1;
}

.details-meta {
  display: flex;
  gap: 16px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
}

/* Модалка создания */
/* Окно, затемнение, анимация и закрытие живут в BaseModal. Здесь остаются только
   отступы содержимого: base-modal__body идёт без padding, их несёт содержимое. */
.marks-modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

@media (max-width: 767.98px) {
  /* Направление/высоту шапки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important - перебивает scoped-специфичность). */
  .management-header {
    padding: 10px var(--gutter, 16px);
  }
  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
  }
  /* rt-header-inline может сделать шапку auto-высоты (перенос controls строкой
     ниже) - фиксированный оверлей bulk-bar (height:50px) больше не накрывает
     её целиком, возвращаем панель в обычный поток (как в Orgs/Companies). */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    overflow-x: visible;
  }
  .bulk-actions {
    flex-wrap: wrap;
  }
  .check-col {
    min-height: 44px;
  }
  .archive-dropdown {
    min-width: 92px;
  }
  :deep(.search) {
    width: 110px;
  }
  .content-container {
    flex-direction: column;
    height: auto;
  }
  .table-section,
  .table-section.with-details,
  .details-section,
  .no-selection-message {
    width: 100%;
  }
  .table-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
  .table-body {
    max-height: 300px;
  }
}
</style>
