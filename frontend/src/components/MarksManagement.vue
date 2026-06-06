<template>
  <div class="marks-container dashboard-card">
    <div class="management-header">
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
          class="add-header-button"
          data-testid="marks-add-btn"
          @click="openAddModal"
        >
          Добавить
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refresh"
        />
      </div>
    </div>

    <div
      v-if="isLoading && !marks.length"
      class="marks-loading"
    >
      <LoaderSpinner label="Загрузка марок..." />
    </div>

    <div
      v-else
      class="table-wrap"
    >
      <table class="marks-table">
        <thead>
          <tr>
            <th
              class="th-id sortable"
              @click="sortBy('id')"
            >
              <span :class="{ 'active-sort': sortField === 'id' }">ID</span>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'id', desc: sortField === 'id' && sortDirection === 'desc' }"
              >
            </th>
            <th
              class="sortable"
              @click="sortBy('name')"
            >
              <span :class="{ 'active-sort': sortField === 'name' }">Наименование</span>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'name', desc: sortField === 'name' && sortDirection === 'desc' }"
              >
            </th>
            <th class="th-actions">
              Действия
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="m in filteredMarks"
            :key="m.id"
            data-testid="marks-row"
            :class="{ inactive: !m.is_active }"
          >
            <td>{{ m.id }}</td>
            <td>
              <span class="mark-name">{{ m.name }}</span>
              <span
                v-if="!m.is_active"
                class="inactive-badge"
              >(архив)</span>
            </td>
            <td class="actions">
              <button
                v-if="m.is_active"
                class="link-btn"
                data-testid="marks-edit"
                @click="openEditModal(m)"
              >
                Переименовать
              </button>
              <button
                class="link-btn"
                data-testid="marks-history"
                @click="openHistory(m)"
              >
                История
              </button>
              <button
                v-if="m.is_active"
                class="link-btn danger"
                data-testid="marks-archive"
                @click="onArchiveClick(m)"
              >
                В архив
              </button>
              <button
                v-else
                class="link-btn"
                data-testid="marks-restore"
                @click="onRestore(m)"
              >
                Восстановить
              </button>
            </td>
          </tr>
          <tr v-if="!filteredMarks.length && !isLoading">
            <td
              colspan="3"
              class="empty"
            >
              {{ emptyText }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add / Edit modal -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="modalMode"
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
              <h3>{{ modalMode === 'add' ? 'Новая марка' : 'Переименование марки' }}</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                data-testid="marks-modal-close"
                @click="requestCloseModal"
              >
                ×
              </button>
            </div>

            <div class="modal-body">
              <div class="form-group">
                <label class="form-label">Название марки</label>
                <input
                  v-model.trim="form.name"
                  type="text"
                  placeholder="Например, Toyota"
                  maxlength="100"
                  class="lk-input"
                  data-testid="marks-input-name"
                  @keyup.enter="onSubmit"
                >
              </div>
              <div
                v-if="error"
                class="form-error"
              >
                {{ error }}
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                data-testid="marks-modal-cancel"
                @click="requestCloseModal"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="!form.name || isSubmitting"
                data-testid="marks-modal-save"
                @click="onSubmit"
              >
                {{ modalMode === 'add' ? 'Добавить' : 'Сохранить' }}
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
    // Колбэк закрытия присваивается в created - нужен доступ к this с проверкой dirty.
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
      modalMode: null, // 'add' | 'edit' | null
      editingId: null,
      originalName: '',
      form: { name: '' },
      error: '',
      isSubmitting: false,
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
      const q = this.searchQuery.trim().toLowerCase();
      let list = this.marks.filter(m => (this.showArchive ? !m.is_active : m.is_active));
      if (q) {
        list = list.filter(m => m.name.toLowerCase().includes(q) || String(m.id).includes(q));
      }
      return this.sortList(list);
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Марок пока нет';
    },
    isFormDirty() {
      if (!this.modalMode) return false;
      return this.form.name.trim() !== (this.originalName || '');
    },
  },
  created() {
    this.overlay.close = () => { this.requestCloseModal(); };
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isFormDirty,
      getChanges: () => (this.modalMode === 'add'
        ? [`Новая марка: "${this.form.name.trim()}"`]
        : [{ label: 'Название марки', from: this.originalName, to: this.form.name.trim() }]),
      save: async () => { await this.onSubmit(); },
    });
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    this._stopGuard?.();
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.modalMode) this.requestCloseModal();
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
    async refresh() {
      this.isLoading = true;
      try {
        const data = await listMarks({ includeArchived: true });
        this.marks = Array.isArray(data) ? data : [];
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
    onArchiveModeChange(value) {
      this.showArchive = value === 'archive';
    },
    openAddModal() {
      this.modalMode = 'add';
      this.editingId = null;
      this.originalName = '';
      this.form.name = '';
      this.error = '';
    },
    openEditModal(mark) {
      this.modalMode = 'edit';
      this.editingId = mark.id;
      this.originalName = mark.name;
      this.form.name = mark.name;
      this.error = '';
    },
    async requestCloseModal() {
      if (this.isFormDirty) {
        const ok = await confirmIfAnyDirty();
        if (!ok) return;
      }
      this.forceCloseModal();
    },
    forceCloseModal() {
      this.modalMode = null;
      this.editingId = null;
      this.originalName = '';
      this.form.name = '';
      this.error = '';
    },
    async onSubmit() {
      const name = this.form.name.trim();
      if (!name || this.isSubmitting) return;
      // Редактирование без изменений - просто закрыть, не дёргать API.
      if (this.modalMode === 'edit' && name === this.originalName) {
        this.forceCloseModal();
        return;
      }
      this.isSubmitting = true;
      this.error = '';
      try {
        if (this.modalMode === 'add') {
          await createMark({ name });
          useDeletionsStore().notify({ prefix: 'Марка ', bold: name, suffix: ' создана' });
        } else {
          await renameMark(this.editingId, { name });
          useDeletionsStore().notify({ prefix: 'Марка переименована в ', bold: name });
        }
        this.forceCloseModal();
        await this.refresh();
      } catch (e) {
        this.error = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSubmitting = false;
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
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async onRestore(mark) {
      try {
        await restoreMark(mark.id);
        useDeletionsStore().notify({ prefix: 'Марка ', bold: mark.name, suffix: ' восстановлена из архива' });
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
  padding: 20px;
}

.management-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  gap: 12px;
  flex-wrap: wrap;
}

.management-title {
  margin: 0;
  font-size: 18px;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

.archive-dropdown {
  width: 150px;
}

.table-wrap {
  border: 1px solid var(--color-border);
  border-radius: 15px;
  overflow: hidden;
}

.marks-table {
  width: 100%;
  border-collapse: collapse;
}

.marks-table th,
.marks-table td {
  padding: 10px 14px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
  font-size: 14px;
}

.marks-table th {
  background: var(--color-bg-secondary);
  font-weight: 600;
  color: #666;
}

.marks-table th.sortable {
  cursor: pointer;
  user-select: none;
}

.marks-table th.sortable span {
  vertical-align: middle;
}

.marks-table th .active-sort {
  color: var(--color-primary);
}

.sort-icon {
  width: 12px;
  height: 12px;
  margin-left: 6px;
  opacity: 0.35;
  vertical-align: middle;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.sort-icon.sorted {
  opacity: 1;
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.th-id {
  width: 70px;
}

.th-actions {
  width: 320px;
}

.marks-table tr.inactive td {
  color: #9aa0a6;
  background: #fafafa;
}

.inactive-badge {
  margin-left: 6px;
  font-size: 11px;
  color: #9aa0a6;
}

.actions {
  display: flex;
  gap: 12px;
}

.link-btn {
  background: none;
  border: 0;
  color: var(--color-primary);
  cursor: pointer;
  font-size: 13px;
  padding: 0;
}

.link-btn.danger {
  color: #d73a3a;
}

.empty {
  text-align: center;
  color: #999;
  padding: 30px 0;
}

.marks-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 50px 0;
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

/* Модалка добавления/переименования */
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

.form-error {
  color: #d73a3a;
  font-size: 0.85em;
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
</style>
