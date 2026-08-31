<template>
  <div class="citizenship-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Управление гражданствами
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
          :title="'Поиск гражданств...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          data-testid="citizenship-add-btn"
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
      data-testid="citizenship-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
      <div class="bulk-actions">
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          data-testid="citizenship-bulk-archive"
          @click="startBulkOperation('archive')"
        >
          В архив
        </button>
        <button
          v-else
          class="pill pill-restore"
          data-testid="citizenship-bulk-restore"
          @click="startBulkOperation('restore')"
        >
          Восстановить
        </button>
        <button
          class="pill pill-ghost bulk-clear"
          data-testid="citizenship-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="content-container">
      <div
        class="table-section"
        :class="{ 'with-details': selectedCitizenship }"
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
                data-testid="citizenship-select-all"
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
              v-for="(c, index) in filteredCitizenships"
              :key="c.id"
              class="table-row rt-row"
              data-testid="citizenship-row"
              :class="{
                selected: selectedCitizenship && selectedCitizenship.id === c.id,
                inactive: !c.is_active,
              }"
              @click="selectCitizenship(c)"
            >
              <div
                class="table-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(c.id)"
                  :aria-label="`Выбрать ${c.name}`"
                  data-testid="citizenship-row-check"
                  @click="onRowCheck(c, index, $event)"
                >
              </div>
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ c.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="c.name"
                >
                  {{ c.name }}
                  <span
                    v-if="c.is_default"
                    class="default-badge"
                  >По умолчанию</span>
                  <span
                    v-if="!c.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
            </div>

            <div
              v-if="!filteredCitizenships.length && !isLoading"
              class="no-results"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !items.length"
              class="citizenship-loading"
            >
              <LoaderSpinner label="Загрузка гражданств..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего гражданств' }}: {{ filteredCitizenships.length }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-if="selectedCitizenship"
        class="details-section"
        data-testid="citizenship-details"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ original.name }}
              </h3>
              <span
                v-if="original.is_default"
                class="default-badge details-default-badge"
              >По умолчанию</span>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedCitizenship.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                data-testid="citizenship-history"
                @click="openHistory(selectedCitizenship)"
              >
                История
              </button>
              <button
                v-if="selectedCitizenship.is_active"
                class="action-btn archive-action-btn"
                :disabled="archiveBlocked"
                :title="archiveBlocked ? 'Сначала назначьте другое гражданство по умолчанию' : ''"
                data-testid="citizenship-archive"
                @click="onArchiveClick(selectedCitizenship)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                data-testid="citizenship-restore"
                @click="onRestore(selectedCitizenship)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <div class="details-body">
            <label class="field-label">Наименование</label>
            <input
              v-model.trim="selectedCitizenship.name"
              type="text"
              class="lk-input"
              maxlength="100"
              placeholder="Название гражданства"
              :disabled="!selectedCitizenship.is_active || isSaving"
              data-testid="citizenship-detail-name"
              @keyup.enter="saveSelected"
            >

            <div class="checkbox-section">
              <label class="checkbox-label">
                <input
                  v-model="selectedCitizenship.is_default"
                  type="checkbox"
                  class="checkbox"
                  :disabled="!selectedCitizenship.is_active || isSaving"
                  data-testid="citizenship-detail-default"
                >
                <span class="checkbox-text">Гражданство по умолчанию</span>
              </label>
              <span class="checkbox-hint">
                Будет выбрано по умолчанию при создании новых заявок
              </span>
            </div>

            <div class="checkbox-section">
              <label class="checkbox-label">
                <input
                  v-model="selectedCitizenship.patent_required"
                  type="checkbox"
                  class="checkbox"
                  :disabled="!selectedCitizenship.is_active || isSaving"
                  data-testid="citizenship-detail-patent"
                >
                <span class="checkbox-text">Требуется патент</span>
              </label>
              <span class="checkbox-hint">
                Для этого гражданства обязателен патент при оформлении заявок
              </span>
            </div>

            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>

            <div
              v-if="selectedCitizenship.is_active"
              class="details-actions"
            >
              <button
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSaving"
                data-testid="citizenship-save"
                @click="saveSelected"
              >
                Сохранить
              </button>
            </div>

            <div class="details-meta">
              <span>ID: {{ selectedCitizenship.id }}</span>
              <span v-if="selectedCitizenship.created_at">Создано: {{ formatDate(selectedCitizenship.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите гражданство для просмотра и редактирования</p>
      </div>
    </div>

    <!-- Модалка создания -->
    <BaseModal
      :show="showAddModal"
      title="Новое гражданство"
      width="440px"
      radius="30px"
      content-testid="citizenship-modal"
      @close="requestCloseAdd"
    >
      <div class="citizenship-modal-body">
        <div class="form-group">
          <label class="form-label">Название гражданства</label>
          <input
            v-model.trim="addForm.name"
            type="text"
            placeholder="Например, Российская Федерация"
            maxlength="100"
            class="lk-input"
            data-testid="citizenship-input-name"
            @keyup.enter="submitAdd"
          >
        </div>

        <div class="checkbox-section">
          <label class="checkbox-label">
            <input
              v-model="addForm.is_default"
              type="checkbox"
              class="checkbox"
            >
            <span class="checkbox-text">Гражданство по умолчанию</span>
          </label>
        </div>

        <div class="checkbox-section">
          <label class="checkbox-label">
            <input
              v-model="addForm.patent_required"
              type="checkbox"
              class="checkbox"
            >
            <span class="checkbox-text">Требуется патент</span>
          </label>
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
          data-testid="citizenship-modal-cancel"
          @click="requestCloseAdd"
        >
          Отмена
        </button>
        <button
          class="lk-button lk-button--primary"
          :disabled="!addForm.name || isAdding"
          data-testid="citizenship-modal-save"
          @click="submitAdd"
        >
          Добавить
        </button>
      </template>
    </BaseModal>

    <ConfirmationModal
      :show="!!archiveConfirm"
      title="Архивация гражданства"
      :message="archiveConfirm ? `Архивировать гражданство «${archiveConfirm.name}»? Его можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirm = null"
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

    <CitizenshipHistoryModal
      v-if="historyForCitizenship"
      :citizenship="historyForCitizenship"
      :current-user-name="currentUserName"
      @close="historyForCitizenship = null"
    />
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import RefreshButton from './RefreshButton.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import BaseModal from './ui/BaseModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import CitizenshipHistoryModal from './CitizenshipHistoryModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { apiRequest } from '@/api/client';
import {
  listCitizenships,
  createCitizenship,
  updateCitizenship,
  archiveCitizenship,
  restoreCitizenship,
  bulkArchiveCitizenships,
  bulkRestoreCitizenships,
} from '@/api/citizenships';
import AppIcon from '@/components/icons/AppIcon.vue';
import { readSearchFromRoute } from '@/utils/searchQueryParam';
import { openFromSearchLink } from '@/mixins/openFromSearchLink'

export default {
  name: 'CitizenshipManagement',
  components: { SearchComponent, RefreshButton, ConfirmationModal, BaseDropdown, BaseModal, LoaderSpinner, CitizenshipHistoryModal, AppIcon },
  mixins: [openFromSearchLink((vm) => vm.items, 'selectCitizenship')],
  data() {
    return {
      items: [],
      // Из адреса: переход из сквозного поиска приносит запрос с собой.
      searchQuery: readSearchFromRoute(this.$route),
      showArchive: false,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedCitizenship: null,
      original: { name: '', is_default: false, patent_required: false },
      detailError: '',
      isSaving: false,
      showAddModal: false,
      addForm: { name: '', is_default: false, patent_required: false },
      addError: '',
      isAdding: false,
      archiveConfirm: null,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      historyForCitizenship: null,
      currentUserName: '',
      // Групповой выбор (по id). lastSelectedId - якорь shift-диапазона.
      selectedIds: [],
      lastSelectedId: null,
      pendingBulkOp: null,
      bulkConfirmVisible: false,
      bulkSubmitting: false,
    };
  },
  computed: {
    filteredCitizenships() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.items.filter(c => (this.showArchive ? !c.is_active : c.is_active));
      if (variants.length) {
        list = list.filter(c => matchesSearch(`${c.name} ${c.id}`, variants));
      }
      return this.sortList(list);
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Гражданств пока нет';
    },
    isAddDirty() {
      return this.showAddModal
        && (this.addForm.name.trim() !== '' || this.addForm.is_default || this.addForm.patent_required);
    },
    isDetailsDirty() {
      const s = this.selectedCitizenship;
      return !!s
        && s.is_active
        && (
          s.name.trim() !== this.original.name
          || s.is_default !== this.original.is_default
          || s.patent_required !== this.original.patent_required
        );
    },
    isDirty() {
      return this.isAddDirty || this.isDetailsDirty;
    },
    // Архив дефолтного гражданства запрещён бэкендом (409). Блокируем и по
    // персистентному значению (нельзя архивировать действующий дефолт), и по
    // живому из формы (нельзя архивировать только что отмеченный дефолт, иначе
    // намерение молча потеряется).
    archiveBlocked() {
      const s = this.selectedCitizenship;
      return !!s && (s.is_default || this.original.is_default);
    },
    allSelected() {
      return this.filteredCitizenships.length > 0 && this.selectedIds.length === this.filteredCitizenships.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление гражданств' : 'Архивация гражданств';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные гражданства (${n})?`
        : `Архивировать выбранные гражданства (${n})? Их можно будет восстановить из архива.`;
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
    // Сужение списка (поиск/смена фильтра) убирает из выбора невидимые строки.
    filteredCitizenships() {
      this.pruneSelection();
    },
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новое гражданство: "${this.addForm.name.trim()}"`];
        if (this.isDetailsDirty) {
          const s = this.selectedCitizenship;
          const ch = [];
          if (s.name.trim() !== this.original.name) {
            ch.push({ label: 'Наименование', from: this.original.name, to: s.name.trim() });
          }
          if (s.is_default !== this.original.is_default) {
            ch.push({ label: 'По умолчанию', from: this.original.is_default ? 'Да' : 'Нет', to: s.is_default ? 'Да' : 'Нет' });
          }
          if (s.patent_required !== this.original.patent_required) {
            ch.push({ label: 'Требуется патент', from: this.original.patent_required ? 'Да' : 'Нет', to: s.patent_required ? 'Да' : 'Нет' });
          }
          return ch;
        }
        return [];
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveSelected();
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
    syncSelectedFrom(fresh) {
      this.selectedCitizenship = { ...fresh };
      this.original = {
        name: fresh.name,
        is_default: fresh.is_default,
        patent_required: fresh.patent_required,
      };
    },
    async refresh() {
      this.isLoading = true;
      try {
        const data = await listCitizenships({ includeArchived: true });
        this.items = Array.isArray(data) ? data : [];
        this.openFromSearchLink();
        // Подтянуть актуальные поля выбранного гражданства или снять выбор,
        // если оно больше не видно в текущем фильтре.
        if (this.selectedCitizenship) {
          const fresh = this.items.find(c => c.id === this.selectedCitizenship.id);
          const visible = fresh && (this.showArchive ? !fresh.is_active : fresh.is_active);
          if (fresh && visible && !this.isDetailsDirty) {
            this.syncSelectedFrom(fresh);
          } else if (!visible) {
            this.selectedCitizenship = null;
          }
        }
        this.pruneSelection();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'гражданства', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async onArchiveModeChange(value) {
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.showArchive = value === 'archive';
      this.selectedCitizenship = null;
      this.detailError = '';
      this.clearSelection();
    },
    async selectCitizenship(c) {
      if (this.selectedCitizenship && this.selectedCitizenship.id === c.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.syncSelectedFrom(c);
      this.detailError = '';
    },
    async saveSelected() {
      if (!this.isDetailsDirty || this.isSaving) return;
      const s = this.selectedCitizenship;
      const name = s.name.trim();
      if (!name) {
        this.detailError = 'Введите название гражданства';
        return;
      }
      this.isSaving = true;
      this.detailError = '';
      try {
        await updateCitizenship(s.id, {
          name,
          icon: s.icon ?? null,
          isDefault: s.is_default,
          patentRequired: s.patent_required,
        });
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
        s.name = name;
        this.original = { name, is_default: s.is_default, patent_required: s.patent_required };
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSaving = false;
      }
    },
    openAddModal() {
      this.showAddModal = true;
      this.addForm = { name: '', is_default: false, patent_required: false };
      this.addError = '';
    },
    async requestCloseAdd() {
      if (this.isAddDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseAdd();
    },
    forceCloseAdd() {
      this.showAddModal = false;
      this.addForm = { name: '', is_default: false, patent_required: false };
      this.addError = '';
    },
    async submitAdd() {
      const name = this.addForm.name.trim();
      if (this.isAdding) return;
      // Молча выходить нельзя: кроме кнопки (она заблокирована при пустом поле)
      // сюда приходит «Сохранить все изменения» из диалога несохранённого, и
      // тихий выход там читается как «нажал, и ничего не произошло».
      if (!name) {
        this.addError = 'Укажите название - без него гражданство не создать.';
        // Ошибку в форме не видно, когда сохранение пришло из диалога
        // несохранённого: он лежит выше окна и перекрывает её. Тост -
        // единственный слой поверх диалога.
        useDeletionsStore().notify({ prefix: this.addError, type: 'error' });
        return;
      }
      this.isAdding = true;
      this.addError = '';
      try {
        await createCitizenship({
          name,
          isDefault: this.addForm.is_default,
          patentRequired: this.addForm.patent_required,
        });
        useDeletionsStore().notify({ prefix: 'Гражданство ', bold: name, suffix: ' создано' });
        this.forceCloseAdd();
        await this.refresh();
      } catch (e) {
        this.addError = e?.message || 'Не удалось создать';
      } finally {
        this.isAdding = false;
      }
    },
    onArchiveClick(c) {
      if (this.archiveBlocked) return;
      this.archiveConfirm = c;
    },
    async performArchive() {
      const c = this.archiveConfirm;
      this.archiveConfirm = null;
      if (!c) return;
      try {
        await archiveCitizenship(c.id);
        useDeletionsStore().notify({ prefix: 'Гражданство ', bold: c.name, suffix: ' архивировано' });
        if (this.selectedCitizenship && this.selectedCitizenship.id === c.id && !this.showArchive) {
          this.selectedCitizenship = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async onRestore(c) {
      try {
        await restoreCitizenship(c.id);
        useDeletionsStore().notify({ prefix: 'Гражданство ', bold: c.name, suffix: ' восстановлено из архива' });
        if (this.selectedCitizenship && this.selectedCitizenship.id === c.id && this.showArchive) {
          this.selectedCitizenship = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    openHistory(c) {
      this.historyForCitizenship = { id: c.id, name: this.original.name || c.name };
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
    onRowCheck(citizenship, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== citizenship.id) {
        const list = this.filteredCitizenships;
        const anchor = list.findIndex(c => c.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(citizenship.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = citizenship.id;
          return;
        }
      }
      this.toggleSelect(citizenship.id);
      this.lastSelectedId = citizenship.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.filteredCitizenships.map(c => c.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.filteredCitizenships.map(c => c.id));
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
        result = op === 'archive' ? await bulkArchiveCitizenships(ids) : await bulkRestoreCitizenships(ids);
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

.citizenship-container {
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

.default-badge {
  font-size: 0.7em;
  background: var(--accent-tint);
  color: var(--accent-text);
  padding: 2px 8px;
  border-radius: 999px;
  margin-left: 6px;
  font-weight: 500;
  vertical-align: middle;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.citizenship-loading {
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

.details-default-badge {
  margin-left: 0;
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

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.archive-action-btn:hover:not(:disabled) {
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
  gap: 12px;
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.details-body .lk-input {
  max-width: 320px;
}

.checkbox-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 14px;
  background: var(--accent-tint);
  border-radius: 15px;
  border: 1px solid var(--border);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-text {
  font-size: 0.9em;
  font-weight: 500;
  color: var(--text);
}

.checkbox-hint {
  font-size: 0.8em;
  color: var(--text-muted);
  line-height: 1.4;
}

.details-actions {
  display: flex;
  gap: 10px;
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
.citizenship-modal-body {
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
  .archive-dropdown {
    min-width: 92px;
  }
  :deep(.search) {
    width: 110px;
  }
  /* rt-header-inline может сделать шапку auto-высоты (перенос controls строкой
     ниже) - фиксированный оверлей bulk-bar (height:50px) больше не накрывает
     её целиком, возвращаем панель в обычный поток. */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    overflow-x: visible;
  }
  .bulk-actions {
    flex-wrap: wrap;
  }
  /* Тач-таргет чекбокса выбора строки в card-режиме. */
  .check-col {
    min-height: 44px;
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
  /* Card-режим: у ячейки имени есть вертикаль - снимаем усечение
     .truncate-text, иначе бейдж "По умолчанию"/"(архив)" в конце строки
     режется (ellipsis прячет всё, что после него). Сам бейдж-пилюля остаётся
     цельной строкой - переносится на новую строку как единое целое, а не
     разрывается по словам. #1097 polish */
  .name-col .truncate-text {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }
  .name-col .default-badge,
  .name-col .inactive-badge {
    white-space: nowrap;
  }
  .details-body .lk-input {
    max-width: 100%;
  }
}
</style>
