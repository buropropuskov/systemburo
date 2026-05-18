<template>
  <div class="marks-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Управление марками автомобилей
      </h3>
      <div class="header-controls">
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

    <div class="filter-tabs">
      <button
        class="filter-tab"
        :class="{ active: filter === 'active' }"
        @click="filter = 'active'"
      >
        Активные
      </button>
      <button
        class="filter-tab"
        :class="{ active: filter === 'archived' }"
        @click="filter = 'archived'"
      >
        Архив
      </button>
    </div>

    <div class="table-wrap">
      <table class="marks-table">
        <thead>
          <tr>
            <th class="th-id">ID</th>
            <th>Наименование</th>
            <th class="th-actions">Действия</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="m in filteredMarks"
            :key="m.id"
            data-testid="marks-row"
          >
            <td>{{ m.id }}</td>
            <td>{{ m.name }}</td>
            <td class="actions">
              <button
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
                @click="onArchive(m)"
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
            <td colspan="3" class="empty">Марок не найдено</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add / Edit modal -->
    <div
      v-if="modalMode"
      class="modal-overlay"
      data-testid="marks-modal"
      @click.self="closeModal"
    >
      <div class="modal-content">
        <h3 class="modal-title">
          {{ modalMode === 'add' ? 'Новая марка' : 'Переименование марки' }}
        </h3>
        <input
          v-model.trim="form.name"
          type="text"
          placeholder="Например, Toyota"
          maxlength="100"
          class="lk-input"
          data-testid="marks-input-name"
          @keyup.enter="onSubmit"
        >
        <div v-if="error" class="form-error">{{ error }}</div>
        <div class="modal-actions">
          <button
            class="lk-btn lk-btn--ghost"
            data-testid="marks-modal-cancel"
            @click="closeModal"
          >
            Отмена
          </button>
          <button
            class="lk-btn"
            :disabled="!form.name || isSubmitting"
            data-testid="marks-modal-save"
            @click="onSubmit"
          >
            {{ modalMode === 'add' ? 'Создать' : 'Сохранить' }}
          </button>
        </div>
      </div>
    </div>

    <MarkHistoryModal
      v-if="historyForMark"
      :mark="historyForMark"
      @close="historyForMark = null"
    />
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';
import MarkHistoryModal from './MarkHistoryModal.vue';
import { useUiStore } from '@/stores/ui';
import {
  listMarks,
  createMark,
  renameMark,
  archiveMark,
  restoreMark,
} from '@/api/marks';

export default {
  name: 'MarksManagement',
  components: { SearchComponent, RefreshButton, MarkHistoryModal },
  data() {
    return {
      marks: [],
      searchQuery: '',
      filter: 'active',
      isLoading: false,
      modalMode: null, // 'add' | 'edit' | null
      editingId: null,
      form: { name: '' },
      error: '',
      isSubmitting: false,
      historyForMark: null,
    };
  },
  computed: {
    filteredMarks() {
      const q = this.searchQuery.trim().toLowerCase();
      const filtered = this.marks.filter(m =>
        this.filter === 'archived' ? !m.is_active : m.is_active,
      );
      if (!q) return filtered;
      return filtered.filter(m => m.name.toLowerCase().includes(q));
    },
  },
  mounted() {
    this.refresh();
  },
  methods: {
    async refresh() {
      this.isLoading = true;
      try {
        const data = await listMarks({ includeArchived: true });
        this.marks = Array.isArray(data) ? data : [];
      } catch {
        useUiStore().error('Не удалось загрузить марки');
      } finally {
        this.isLoading = false;
      }
    },
    openAddModal() {
      this.modalMode = 'add';
      this.editingId = null;
      this.form.name = '';
      this.error = '';
    },
    openEditModal(mark) {
      this.modalMode = 'edit';
      this.editingId = mark.id;
      this.form.name = mark.name;
      this.error = '';
    },
    closeModal() {
      this.modalMode = null;
      this.editingId = null;
      this.form.name = '';
      this.error = '';
    },
    async onSubmit() {
      if (!this.form.name.trim()) return;
      this.isSubmitting = true;
      this.error = '';
      try {
        if (this.modalMode === 'add') {
          await createMark({ name: this.form.name });
          useUiStore().success('Марка создана');
        } else {
          await renameMark(this.editingId, { name: this.form.name });
          useUiStore().success('Марка переименована');
        }
        this.closeModal();
        await this.refresh();
      } catch (e) {
        this.error = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSubmitting = false;
      }
    },
    async onArchive(mark) {
      try {
        await archiveMark(mark.id);
        useUiStore().success('Марка архивирована');
        await this.refresh();
      } catch {
        useUiStore().error('Не удалось архивировать');
      }
    },
    async onRestore(mark) {
      try {
        await restoreMark(mark.id);
        useUiStore().success('Марка восстановлена');
        await this.refresh();
      } catch {
        useUiStore().error('Не удалось восстановить');
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

.filter-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.filter-tab {
  padding: 6px 14px;
  border-radius: 16px;
  border: 1px solid var(--color-border);
  background: var(--color-bg-secondary);
  cursor: pointer;
  font-size: 13px;
}

.filter-tab.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.table-wrap {
  border: 1px solid var(--color-border);
  border-radius: 8px;
  overflow: hidden;
}

.marks-table {
  width: 100%;
  border-collapse: collapse;
}

.marks-table th,
.marks-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
  font-size: 14px;
}

.marks-table th {
  background: var(--color-bg-secondary);
  font-weight: 600;
  color: #666;
}

.th-id {
  width: 60px;
}

.th-actions {
  width: 320px;
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

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 400px;
  max-width: 90vw;
}

.modal-title {
  margin: 0 0 16px;
  font-size: 18px;
}

.lk-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  font-size: 14px;
  margin-bottom: 12px;
}

.form-error {
  color: #d73a3a;
  font-size: 13px;
  margin-bottom: 8px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

.lk-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 0;
  background: var(--color-primary);
  color: #fff;
  cursor: pointer;
}

.lk-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.lk-btn--ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: #333;
}
</style>
