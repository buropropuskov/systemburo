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

    <div class="content-container">
      <!-- Левая часть - список марок -->
      <div
        class="table-section"
        :class="{ 'with-details': selectedMark }"
      >
        <div class="table-container rt-table">
          <div class="table-header rt-head-row">
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'id', desc: sortField === 'id' && sortDirection === 'desc' }"
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('name')"
            >
              <p :class="{ 'active-sort': sortField === 'name' }">
                Наименование
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'name', desc: sortField === 'name' && sortDirection === 'desc' }"
              >
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="m in filteredMarks"
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
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          data-testid="marks-modal"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <div
            class="marks-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Новая марка</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                data-testid="marks-modal-close"
                @click="requestCloseAdd"
              >
                ×
              </button>
            </div>

            <div class="modal-body">
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

            <div class="modal-footer">
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
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

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
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import RefreshButton from './RefreshButton.vue';
import MarkHistoryModal from './MarkHistoryModal.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { apiRequest } from '@/api/client';
import {
  listMarks,
  createMark,
  renameMark,
  archiveMark,
  restoreMark,
} from '@/api/marks';

export default {
  name: 'MarksManagement',
  components: { SearchComponent, RefreshButton, MarkHistoryModal, ConfirmationModal, BaseDropdown, LoaderSpinner },
  setup() {
    // Колбэк закрытия модалки присваивается в created - нужен доступ к this с проверкой dirty.
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      marks: [],
      searchQuery: '',
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
  },
  created() {
    this.overlay.close = () => { this.requestCloseAdd(); };
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
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    this._stopGuard?.();
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.showAddModal) this.requestCloseAdd();
    },
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
      if (!name || this.isAdding) return;
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
  },
};
</script>

<style scoped>
.marks-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
  gap: 12px;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

.archive-dropdown {
  min-width: 130px;
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
  border-right: 1px solid #e6e6e6;
  background: #fff;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
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

.header-col:hover {
  color: #000;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #000 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 25%;
  min-width: 60px;
}

.name-col {
  width: 75%;
  min-width: 160px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f8f9ff;
}

.table-row.inactive {
  background: #fafafa;
  color: #6b7280;
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: #000;
}

.table-row.inactive .id-value {
  color: #6b7280;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: #a2a2a2;
  font-style: italic;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
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
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

/* Details */
.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fff;
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
  color: #000;
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

.archive-badge {
  background: #6b7280;
  color: #fff;
  padding: 4px 10px;
  border-radius: 50px;
  font-size: 0.75em;
  font-weight: 500;
  white-space: nowrap;
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
  background: #fff;
  color: #4F5BDF;
  border: 1px solid #4F5BDF;
}

.history-btn:hover {
  background: #eef0ff;
}

.archive-action-btn {
  background: #fff;
  color: #dc3545;
  border: 1px solid #fecaca;
}

.archive-action-btn:hover {
  background: #fff1f2;
  border-color: #dc3545;
}

.restore-btn {
  background: #10b981;
  color: #fff;
}

.restore-btn:hover {
  background: #0da271;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 0.85em;
  color: #666;
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
  color: #a2a2a2;
}

.form-error {
  color: #d73a3a;
  font-size: 0.85em;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
}

.add-header-button {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.add-header-button:hover {
  background: #3a45b2;
}

/* Модалка создания */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.marks-modal {
  width: 100%;
  max-width: 440px;
  background: #fff;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.modal-close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: #999;
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.modal-close:hover {
  color: #333;
  background: #f5f5f5;
}

.modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 0.85em;
  color: #666;
  font-weight: 500;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid #e6e6e6;
}

/* Анимация открытия/закрытия */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-active .marks-modal,
.modal-fade-leave-active .marks-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: rgba(0, 0, 0, 0);
}

.modal-fade-enter-from .marks-modal,
.modal-fade-leave-to .marks-modal {
  opacity: 0;
  transform: translateY(20px);
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
    border-bottom: 1px solid #e6e6e6;
  }
  .table-body {
    max-height: 300px;
  }
}
</style>
