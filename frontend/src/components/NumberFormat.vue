<template>
  <div class="number-format-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Управление форматами номеров
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
          :title="'Поиск форматов...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          data-testid="nf-add-btn"
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
      data-testid="numberformat-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
      <div class="bulk-actions">
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          data-testid="numberformat-bulk-archive"
          @click="startBulkOperation('archive')"
        >
          В архив
        </button>
        <button
          v-else
          class="pill pill-restore"
          data-testid="numberformat-bulk-restore"
          @click="startBulkOperation('restore')"
        >
          Восстановить
        </button>
        <button
          class="pill pill-ghost bulk-clear"
          data-testid="numberformat-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список форматов -->
      <div class="table-section">
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
                data-testid="numberformat-select-all"
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
              v-for="(item, index) in filteredFormats"
              :key="item.format.id"
              class="table-row rt-row"
              data-testid="nf-row"
              :class="{
                selected: selectedFormat && selectedFormat.format.id === item.format.id,
                inactive: !item.format.is_active,
              }"
              @click="selectFormat(item)"
            >
              <div
                class="table-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(item.format.id)"
                  :aria-label="`Выбрать ${item.format.name}`"
                  data-testid="numberformat-row-check"
                  @click="onRowCheck(item.format, index, $event)"
                >
              </div>
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ item.format.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="item.format.name"
                >
                  {{ item.format.name }}
                  <span
                    v-if="item.format.is_default"
                    class="default-badge"
                  >по умолчанию</span>
                  <span
                    v-if="!item.format.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
            </div>

            <div
              v-if="!filteredFormats.length && !isLoading"
              class="no-results"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !formats.length"
              class="nf-loading"
            >
              <LoaderSpinner label="Загрузка форматов..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего форматов' }}: {{ filteredFormats.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали формата -->
      <div
        v-if="selectedFormat"
        class="details-section"
        data-testid="nf-details"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedFormat.format.name }}
              </h3>
              <span class="format-preview">{{ getFormatPreview(selectedFormat) }}</span>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedFormat.format.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                data-testid="nf-history"
                @click="openHistory(selectedFormat)"
              >
                История
              </button>
              <button
                v-if="selectedFormat.format.is_active"
                class="action-btn archive-action-btn"
                data-testid="nf-archive"
                @click="onArchiveClick(selectedFormat)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                data-testid="nf-restore"
                @click="onRestore(selectedFormat)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <div class="details-body">
            <div class="form-row">
              <div class="field">
                <label class="field-label">Наименование</label>
                <input
                  v-model.trim="selectedFormat.format.name"
                  type="text"
                  class="lk-input"
                  maxlength="100"
                  placeholder="Название формата"
                  :disabled="!selectedFormat.format.is_active"
                  data-testid="nf-detail-name"
                >
              </div>
              <div class="field">
                <label class="field-label">Код страны</label>
                <input
                  v-model.trim="selectedFormat.format.country_code"
                  type="text"
                  class="lk-input"
                  maxlength="10"
                  placeholder="RU, AZ, KZ"
                  :disabled="!selectedFormat.format.is_active"
                  data-testid="nf-detail-country"
                >
              </div>
            </div>

            <label class="default-checkbox-section">
              <input
                v-model="selectedFormat.format.is_default"
                type="checkbox"
                class="default-checkbox"
                :disabled="!selectedFormat.format.is_active"
                data-testid="nf-detail-default"
              >
              <span class="default-checkbox-body">
                <span class="default-checkbox-text">Формат по умолчанию</span>
                <span class="default-checkbox-hint">Выбирается по умолчанию при создании нового Т/С</span>
              </span>
            </label>

            <div class="cells-section">
              <label class="field-label">Клетки формата номера</label>
              <div class="cells-horizontal">
                <button
                  v-for="(cell, index) in selectedFormat.cells"
                  :key="index"
                  type="button"
                  class="cell-card"
                  :disabled="!selectedFormat.format.is_active"
                  @click="editCell(index)"
                >
                  <div class="cell-card-header">
                    <span class="cell-badge">Клетка №{{ index + 1 }}</span>
                    <span
                      class="cell-type-badge"
                      :class="cell.cell_type"
                    >{{ getCellTypeLabel(cell.cell_type) }}</span>
                  </div>
                  <div class="cell-card-details">
                    <span class="cell-length">{{ cell.min_length }}-{{ cell.max_length }} симв.</span>
                    <span
                      v-if="cell.cell_type !== 'numbers' && cell.allowed_letters"
                      class="cell-letters"
                      :title="cell.allowed_letters"
                    >{{ truncateLetters(cell.allowed_letters) }}</span>
                    <span
                      v-if="cell.cell_type === 'numbers'"
                      class="cell-padding"
                    >Дополнение: {{ cell.padding_side === 'left' ? 'слева' : 'справа' }}</span>
                  </div>
                </button>
              </div>
            </div>

            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>

            <div
              v-if="selectedFormat.format.is_active"
              class="details-actions"
            >
              <button
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSaving"
                data-testid="nf-detail-save"
                @click="saveDetails"
              >
                Сохранить
              </button>
            </div>

            <div class="details-meta">
              <span>ID: {{ selectedFormat.format.id }}</span>
              <span v-if="selectedFormat.format.created_at">Создан: {{ formatDate(selectedFormat.format.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите формат номеров для просмотра</p>
      </div>
    </div>

    <!-- Модалка добавления формата -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          data-testid="nf-modal"
          @mousedown="onAddOverlayMousedown"
          @mouseup="onAddOverlayMouseup"
        >
          <div
            class="nf-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Новый формат номеров</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                data-testid="nf-modal-close"
                @click="requestCloseAdd"
              >
                ×
              </button>
            </div>

            <div class="modal-body">
              <div class="form-row">
                <div class="field">
                  <label class="field-label">Название формата</label>
                  <input
                    v-model.trim="addForm.name"
                    type="text"
                    class="lk-input"
                    maxlength="100"
                    placeholder="Российские номера"
                    data-testid="nf-input-name"
                  >
                </div>
                <div class="field">
                  <label class="field-label">Код страны</label>
                  <input
                    v-model.trim="addForm.country_code"
                    type="text"
                    class="lk-input"
                    maxlength="10"
                    placeholder="RU"
                  >
                </div>
              </div>

              <label class="default-checkbox-section">
                <input
                  v-model="addForm.is_default"
                  type="checkbox"
                  class="default-checkbox"
                >
                <span class="default-checkbox-body">
                  <span class="default-checkbox-text">Формат по умолчанию</span>
                  <span class="default-checkbox-hint">Выбирается по умолчанию при создании нового Т/С</span>
                </span>
              </label>

              <div class="cells-edit-section">
                <div class="cells-edit-header">
                  <label class="field-label">Клетки формата</label>
                  <button
                    type="button"
                    class="lk-button lk-button--secondary add-cell-btn"
                    @click="addCell"
                  >
                    + Добавить клетку
                  </button>
                </div>

                <div class="cells-edit-list">
                  <div
                    v-for="(cell, index) in addForm.cells"
                    :key="index"
                    class="cell-edit-card"
                  >
                    <div class="cell-edit-card-header">
                      <span class="cell-number">Клетка №{{ index + 1 }}</span>
                      <button
                        v-if="addForm.cells.length > 1"
                        type="button"
                        class="cell-remove-btn"
                        title="Удалить клетку"
                        @click="removeCell(index)"
                      >
                        ×
                      </button>
                    </div>
                    <div class="cell-edit-fields">
                      <div class="field">
                        <label class="field-label">Тип</label>
                        <select
                          v-model="cell.cell_type"
                          class="lk-select"
                        >
                          <option value="letters">
                            Буквы
                          </option>
                          <option value="numbers">
                            Цифры
                          </option>
                          <option value="mixed">
                            Смешанный
                          </option>
                        </select>
                      </div>
                      <div class="field length-field">
                        <label class="field-label">Длина</label>
                        <div class="length-controls">
                          <input
                            v-model.number="cell.min_length"
                            type="number"
                            min="1"
                            max="10"
                            class="lk-input length-input"
                            placeholder="мин"
                          >
                          <span class="length-dash">-</span>
                          <input
                            v-model.number="cell.max_length"
                            type="number"
                            min="1"
                            max="10"
                            class="lk-input length-input"
                            placeholder="макс"
                          >
                        </div>
                      </div>
                      <div
                        v-if="cell.cell_type !== 'numbers'"
                        class="field"
                      >
                        <label class="field-label">Алфавит</label>
                        <select
                          v-model="cell.alphabet_type"
                          class="lk-select"
                        >
                          <option value="cyrillic">
                            Кириллица
                          </option>
                          <option value="latin">
                            Латиница
                          </option>
                          <option value="both">
                            Оба
                          </option>
                        </select>
                      </div>
                      <div
                        v-if="cell.cell_type === 'numbers'"
                        class="field"
                      >
                        <label class="field-label">Дополнение</label>
                        <select
                          v-model="cell.padding_side"
                          class="lk-select"
                        >
                          <option value="left">
                            Слева
                          </option>
                          <option value="right">
                            Справа
                          </option>
                        </select>
                      </div>
                      <div
                        v-if="cell.cell_type !== 'numbers'"
                        class="field field-full"
                      >
                        <label class="field-label">Разрешённые буквы</label>
                        <input
                          v-model="cell.allowed_letters"
                          type="text"
                          class="lk-input"
                          placeholder="АВЕКМНОРСТУХ"
                        >
                      </div>
                    </div>
                  </div>
                </div>
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
                data-testid="nf-modal-cancel"
                @click="requestCloseAdd"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="!addForm.name || isAdding"
                data-testid="nf-modal-save"
                @click="submitAdd"
              >
                Добавить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модалка редактирования клетки -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showCellEditModal"
          class="modal-overlay"
          data-testid="nf-cell-modal"
          @mousedown="onCellOverlayMousedown"
          @mouseup="onCellOverlayMouseup"
        >
          <div
            class="nf-modal nf-modal--narrow"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Редактирование клетки {{ editingCellIndex + 1 }}</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                @click="showCellEditModal = false"
              >
                ×
              </button>
            </div>

            <div
              v-if="editingCell"
              class="modal-body"
            >
              <div class="cell-edit-fields">
                <div class="field">
                  <label class="field-label">Тип клетки</label>
                  <select
                    v-model="editingCell.cell_type"
                    class="lk-select"
                  >
                    <option value="letters">
                      Буквы
                    </option>
                    <option value="numbers">
                      Цифры
                    </option>
                    <option value="mixed">
                      Смешанный
                    </option>
                  </select>
                </div>
                <div class="field length-field">
                  <label class="field-label">Длина</label>
                  <div class="length-controls">
                    <input
                      v-model.number="editingCell.min_length"
                      type="number"
                      min="1"
                      max="10"
                      class="lk-input length-input"
                      placeholder="мин"
                    >
                    <span class="length-dash">-</span>
                    <input
                      v-model.number="editingCell.max_length"
                      type="number"
                      min="1"
                      max="10"
                      class="lk-input length-input"
                      placeholder="макс"
                    >
                  </div>
                </div>
                <div
                  v-if="editingCell.cell_type !== 'numbers'"
                  class="field"
                >
                  <label class="field-label">Алфавит</label>
                  <select
                    v-model="editingCell.alphabet_type"
                    class="lk-select"
                  >
                    <option value="cyrillic">
                      Кириллица
                    </option>
                    <option value="latin">
                      Латиница
                    </option>
                    <option value="both">
                      Оба
                    </option>
                  </select>
                </div>
                <div
                  v-if="editingCell.cell_type === 'numbers'"
                  class="field"
                >
                  <label class="field-label">Дополнение нулями</label>
                  <select
                    v-model="editingCell.padding_side"
                    class="lk-select"
                  >
                    <option value="left">
                      Слева
                    </option>
                    <option value="right">
                      Справа
                    </option>
                  </select>
                </div>
                <div
                  v-if="editingCell.cell_type !== 'numbers'"
                  class="field field-full"
                >
                  <label class="field-label">Разрешённые буквы</label>
                  <input
                    v-model="editingCell.allowed_letters"
                    type="text"
                    class="lk-input"
                    placeholder="АВЕКМНОРСТУХ"
                  >
                  <span class="field-hint">Пусто - используются все буквы выбранного алфавита</span>
                </div>
              </div>

              <div class="cell-preview">
                <span class="cell-preview-label">Предпросмотр</span>
                <span class="cell-preview-value">{{ getCellPreview(editingCell) }}</span>
              </div>

              <div
                v-if="cellEditError"
                class="form-error"
              >
                {{ cellEditError }}
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                @click="showCellEditModal = false"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                @click="saveCellEdit"
              >
                Применить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <ConfirmationModal
      :show="!!archiveConfirmFormat"
      title="Архивация формата"
      :message="archiveConfirmFormat ? `Архивировать формат «${archiveConfirmFormat.name}»? Его можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirmFormat = null"
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

    <LicensePlateFormatHistoryModal
      v-if="historyForFormat"
      :format="historyForFormat"
      :current-user-name="currentUserName"
      @close="historyForFormat = null"
    />
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import RefreshButton from './RefreshButton.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import LicensePlateFormatHistoryModal from './LicensePlateFormatHistoryModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { apiRequest } from '@/api/client';
import { bulkArchiveLicenseFormats, bulkRestoreLicenseFormats } from '@/api/licenseFormats';
import AppIcon from '@/components/icons/AppIcon.vue';
import { fetchCurrentUserName } from '@/utils/currentUserName';
import { openFromSearchLink } from '@/mixins/openFromSearchLink'

function defaultCell() {
  return {
    cell_order: 0,
    cell_type: 'letters',
    min_length: 1,
    max_length: 1,
    alphabet_type: 'cyrillic',
    allowed_letters: '',
    padding_side: 'left',
  };
}

const CELL_TYPE_LABELS = {
  letters: 'Буквы',
  numbers: 'Цифры',
  mixed: 'Смешанный',
};

export default {
  name: 'NumberFormat',
  mixins: [openFromSearchLink((vm) => vm.formats, 'selectFormat', (row) => row?.format?.id)],
  components: { SearchComponent, RefreshButton, ConfirmationModal, BaseDropdown, LoaderSpinner, LicensePlateFormatHistoryModal, AppIcon },
  setup() {
    // Колбэки закрытия присваиваются в created - нужен доступ к this (проверка dirty).
    const addOverlay = { close: () => {} };
    const cellOverlay = { close: () => {} };
    const a = useOverlayClose(() => addOverlay.close());
    const c = useOverlayClose(() => cellOverlay.close());
    return {
      addOverlay,
      cellOverlay,
      onAddOverlayMousedown: a.onOverlayMousedown,
      onAddOverlayMouseup: a.onOverlayMouseup,
      onCellOverlayMousedown: c.onOverlayMousedown,
      onCellOverlayMouseup: c.onOverlayMouseup,
    };
  },
  data() {
    return {
      formats: [],
      searchQuery: '',
      showArchive: false,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedFormat: null,
      originalSnapshot: '',
      detailError: '',
      isSaving: false,
      showAddModal: false,
      addForm: { name: '', country_code: '', is_default: false, cells: [defaultCell()] },
      addError: '',
      isAdding: false,
      showCellEditModal: false,
      editingCellIndex: null,
      editingCell: null,
      cellEditError: '',
      archiveConfirmFormat: null,
      historyForFormat: null,
      currentUserName: '',
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
    filteredFormats() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.formats.filter(item =>
        this.showArchive ? !item.format.is_active : item.format.is_active);
      if (variants.length) {
        list = list.filter(item => matchesSearch(
          `${item.format.name} ${item.format.id} ${item.format.country_code || ''}`,
          variants,
        ));
      }
      return this.sortList(list);
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Форматов пока нет';
    },
    isAddDirty() {
      return this.showAddModal && this.addForm.name.trim() !== '';
    },
    isDetailsDirty() {
      return !!this.selectedFormat
        && this.selectedFormat.format.is_active
        && this.detailSnapshot(this.selectedFormat) !== this.originalSnapshot;
    },
    isDirty() {
      return this.isAddDirty || this.isDetailsDirty;
    },
    allSelected() {
      return this.filteredFormats.length > 0 && this.selectedIds.length === this.filteredFormats.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление форматов' : 'Архивация форматов';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные форматы (${n})?`
        : `Архивировать выбранные форматы (${n})? Их можно будет восстановить из архива.`;
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
    // Смена фильтра/поиска/режима меняет видимый список - убираем из выбора
    // строки, которых больше не видно (реактивно, не только после refresh).
    filteredFormats() {
      this.pruneSelection();
    },
  },
  created() {
    this.addOverlay.close = () => { this.requestCloseAdd(); };
    this.cellOverlay.close = () => { this.showCellEditModal = false; };
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новый формат: "${this.addForm.name.trim()}"`];
        if (this.isDetailsDirty) return [`Изменения формата "${this.selectedFormat.format.name}"`];
        return [];
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveDetails();
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
      if (e.key !== 'Escape') return;
      if (this.showCellEditModal) this.showCellEditModal = false;
      else if (this.showAddModal) this.requestCloseAdd();
    },
    sortList(list) {
      const arr = [...list];
      if (!this.sortField) {
        return arr.sort((a, b) => a.format.name.localeCompare(b.format.name));
      }
      return arr.sort((a, b) => {
        if (this.sortField === 'id') {
          return this.sortDirection === 'asc' ? a.format.id - b.format.id : b.format.id - a.format.id;
        }
        const r = a.format.name.localeCompare(b.format.name);
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
    // Поля, нерелевантные текущему типу клетки, обнуляются перед сравнением,
    // чтобы скрытые значения не давали ложного dirty.
    detailSnapshot(item) {
      return JSON.stringify({
        name: (item.format.name || '').trim(),
        country_code: (item.format.country_code || '').trim(),
        is_default: !!item.format.is_default,
        cells: item.cells.map(c => ({
          cell_type: c.cell_type,
          min_length: c.min_length,
          max_length: c.max_length,
          allowed_letters: c.cell_type !== 'numbers' ? (c.allowed_letters || '') : '',
          alphabet_type: c.cell_type !== 'numbers' ? (c.alphabet_type || 'cyrillic') : '',
          padding_side: c.cell_type === 'numbers' ? (c.padding_side || 'left') : '',
        })),
      });
    },
    // validateCells - дружелюбная проверка длин до отправки на сервер: иначе ошибка
    // прилетела бы как сетевая 4xx без понятного пользователю текста.
    validateCells(cells) {
      for (let i = 0; i < cells.length; i += 1) {
        const min = Number(cells[i].min_length);
        const max = Number(cells[i].max_length);
        if (!Number.isInteger(min) || !Number.isInteger(max) || min < 1 || max < 1) {
          return `Клетка №${i + 1}: длина должна быть целым числом не меньше 1`;
        }
        if (min > max) {
          return `Клетка №${i + 1}: мин. длина не может превышать макс.`;
        }
      }
      return '';
    },
    buildCellPayload(cell, index, withId) {
      const payload = {
        cell_order: index,
        cell_type: cell.cell_type,
        min_length: cell.min_length,
        max_length: cell.max_length,
        allowed_letters: cell.cell_type !== 'numbers' ? (cell.allowed_letters || null) : null,
        alphabet_type: cell.cell_type !== 'numbers' ? (cell.alphabet_type || 'cyrillic') : null,
        language: 'ru',
        padding_char: '0',
        padding_side: cell.cell_type === 'numbers' ? (cell.padding_side || 'left') : null,
      };
      if (withId && cell.id) payload.id = cell.id;
      return payload;
    },
    async refresh() {
      this.isLoading = true;
      try {
        const res = await apiRequest('/license-plate-formats?include_archived=true');
        if (!res.ok) throw new Error('fetch failed');
        const data = await res.json();
        this.formats = Array.isArray(data) ? data : [];
        this.openFromSearchLink();
        if (this.selectedFormat) {
          const fresh = this.formats.find(f => f.format.id === this.selectedFormat.format.id);
          const visible = fresh && (this.showArchive ? !fresh.format.is_active : fresh.format.is_active);
          if (fresh && visible && !this.isDetailsDirty) {
            this.applySelection(fresh);
          } else if (!visible) {
            this.selectedFormat = null;
          }
        }
        this.pruneSelection();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'форматы номеров', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    applySelection(item) {
      this.selectedFormat = JSON.parse(JSON.stringify(item));
      this.originalSnapshot = this.detailSnapshot(this.selectedFormat);
      this.detailError = '';
    },
    async onArchiveModeChange(value) {
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.showArchive = value === 'archive';
      this.selectedFormat = null;
      this.detailError = '';
      this.clearSelection();
    },
    async selectFormat(item) {
      if (this.selectedFormat && this.selectedFormat.format.id === item.format.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.applySelection(item);
    },
    async saveDetails() {
      if (!this.isDetailsDirty || this.isSaving) return;
      const f = this.selectedFormat;
      const name = (f.format.name || '').trim();
      if (!name) {
        this.detailError = 'Введите название формата';
        return;
      }
      const cellErr = this.validateCells(f.cells);
      if (cellErr) {
        this.detailError = cellErr;
        return;
      }
      this.isSaving = true;
      this.detailError = '';
      try {
        const body = {
          name,
          country_code: (f.format.country_code || '').trim() || null,
          icon: f.format.icon || null,
          is_default: !!f.format.is_default,
          cells: f.cells.map((c, i) => this.buildCellPayload(c, i, true)),
        };
        const res = await apiRequest(`/license-plate-formats/${f.format.id}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        });
        if (!res.ok) {
          const err = await res.json();
          throw new Error(err?.message || 'Ошибка сохранения');
        }
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSaving = false;
      }
    },
    openAddModal() {
      this.showAddModal = true;
      this.addForm = { name: '', country_code: '', is_default: false, cells: [defaultCell()] };
      this.addError = '';
    },
    async requestCloseAdd() {
      if (this.isAddDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseAdd();
    },
    forceCloseAdd() {
      this.showAddModal = false;
      this.addForm = { name: '', country_code: '', is_default: false, cells: [defaultCell()] };
      this.addError = '';
    },
    addCell() {
      this.addForm.cells.push(defaultCell());
    },
    removeCell(index) {
      this.addForm.cells.splice(index, 1);
    },
    async submitAdd() {
      const name = this.addForm.name.trim();
      if (!name || this.isAdding) return;
      if (!this.addForm.cells.length) {
        this.addError = 'Добавьте хотя бы одну клетку';
        return;
      }
      const cellErr = this.validateCells(this.addForm.cells);
      if (cellErr) {
        this.addError = cellErr;
        return;
      }
      this.isAdding = true;
      this.addError = '';
      try {
        const body = {
          name,
          country_code: (this.addForm.country_code || '').trim() || null,
          icon: null,
          is_default: !!this.addForm.is_default,
          cells: this.addForm.cells.map((c, i) => this.buildCellPayload(c, i, false)),
        };
        const res = await apiRequest('/license-plate-formats', {
          method: 'POST',
          body: JSON.stringify(body),
        });
        if (!res.ok) {
          const err = await res.json();
          throw new Error(err?.message || 'Ошибка создания');
        }
        useDeletionsStore().notify({ prefix: 'Формат ', bold: name, suffix: ' создан' });
        this.forceCloseAdd();
        await this.refresh();
      } catch (e) {
        this.addError = e?.message || 'Не удалось создать формат';
      } finally {
        this.isAdding = false;
      }
    },
    editCell(index) {
      if (!this.selectedFormat.format.is_active) return;
      this.editingCellIndex = index;
      this.editingCell = JSON.parse(JSON.stringify(this.selectedFormat.cells[index]));
      this.cellEditError = '';
      this.showCellEditModal = true;
    },
    saveCellEdit() {
      if (this.editingCellIndex === null || !this.editingCell) return;
      const err = this.validateCells([this.editingCell]);
      if (err) {
        this.cellEditError = err;
        return;
      }
      this.selectedFormat.cells.splice(this.editingCellIndex, 1, this.editingCell);
      this.showCellEditModal = false;
      this.editingCellIndex = null;
      this.editingCell = null;
    },
    onArchiveClick(item) {
      this.archiveConfirmFormat = item.format;
    },
    async performArchive() {
      const fmt = this.archiveConfirmFormat;
      this.archiveConfirmFormat = null;
      if (!fmt) return;
      try {
        const res = await apiRequest(`/license-plate-formats/${fmt.id}`, { method: 'DELETE' });
        if (!res.ok) {
          const err = await res.json();
          throw new Error(err?.message || 'Ошибка архивации');
        }
        useDeletionsStore().notify({ prefix: 'Формат ', bold: fmt.name, suffix: ' архивирован' });
        if (this.selectedFormat && this.selectedFormat.format.id === fmt.id && !this.showArchive) {
          this.selectedFormat = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async onRestore(item) {
      const fmt = item.format;
      try {
        const res = await apiRequest(`/license-plate-formats/${fmt.id}/restore`, { method: 'POST' });
        if (!res.ok) {
          const err = await res.json();
          throw new Error(err?.message || 'Ошибка восстановления');
        }
        useDeletionsStore().notify({ prefix: 'Формат ', bold: fmt.name, suffix: ' восстановлен из архива' });
        if (this.selectedFormat && this.selectedFormat.format.id === fmt.id && this.showArchive) {
          this.selectedFormat = null;
        }
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    openHistory(item) {
      this.historyForFormat = item.format;
    },
    async fetchCurrentUser() {
      this.currentUserName = await fetchCurrentUserName();
    },
    getCellTypeLabel(type) {
      return CELL_TYPE_LABELS[type] || type;
    },
    getFormatPreview(item) {
      return item.cells.map((cell) => {
        const len = cell.max_length || 1;
        return cell.cell_type === 'numbers' ? '0'.repeat(len) : 'A'.repeat(len);
      }).join(' ');
    },
    getCellPreview(cell) {
      const len = cell.max_length || 1;
      if (cell.cell_type === 'numbers') return '0'.repeat(len);
      if (cell.allowed_letters) return cell.allowed_letters.charAt(0).repeat(len);
      return 'A'.repeat(len);
    },
    truncateLetters(letters, maxLength = 12) {
      if (!letters) return '';
      return letters.length > maxLength ? `${letters.substring(0, maxLength)}...` : letters;
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
    onRowCheck(format, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== format.id) {
        const list = this.filteredFormats.map(item => item.format);
        const anchor = list.findIndex(f => f.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(format.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = format.id;
          return;
        }
      }
      this.toggleSelect(format.id);
      this.lastSelectedId = format.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.filteredFormats.map(item => item.format.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.filteredFormats.map(item => item.format.id));
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
        result = op === 'archive' ? await bulkArchiveLicenseFormats(ids) : await bulkRestoreLicenseFormats(ids);
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
.number-format-container {
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

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

.archive-dropdown {
  min-width: 130px;
}

.add-header-button {
  padding: 8px 16px;
  background: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  white-space: nowrap;
}

.add-header-button:hover {
  background: var(--accent-hover);
}

/* Master-detail layout (эталон TableConstructor / MarksManagement) */
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

.table-container {
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
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

.header-col:hover {
  color: var(--text);
}

.header-col:hover .sort-icon {
  color: var(--text);
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.sort-icon.sorted {
  color: var(--text);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 600 !important;
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

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

.table-row.inactive {
  background: var(--surface-2);
  color: var(--text-muted);
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
  color: var(--text);
}

.table-row.inactive .id-value {
  color: var(--text-muted);
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.default-badge {
  margin-left: 6px;
  font-size: 0.7em;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent-tint);
  color: var(--accent-text);
  font-weight: 600;
  vertical-align: middle;
}

.inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: var(--text-muted);
  font-style: italic;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.nf-loading {
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

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
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

.format-preview {
  font-family: monospace;
  font-size: 0.9em;
  color: var(--text-muted);
  background: var(--accent-tint);
  padding: 4px 10px;
  border-radius: 15px;
  border: 1px solid var(--border);
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.archive-badge {
  background: var(--text-muted);
  color: var(--surface);
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
  gap: 16px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.field-full {
  flex-basis: 100%;
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.field-hint {
  font-size: 0.75em;
  color: var(--text-muted);
  line-height: 1.3;
}

/* Чекбокс по умолчанию */
.default-checkbox-section {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: var(--accent-tint);
  border-radius: 15px;
  border: 1px solid var(--border);
  cursor: pointer;
}

.default-checkbox {
  margin-top: 2px;
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--accent-text);
}

.default-checkbox-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.default-checkbox-text {
  font-size: 0.9em;
  font-weight: 500;
  color: var(--text);
}

.default-checkbox-hint {
  font-size: 0.8em;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Клетки в деталях */
.cells-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cells-horizontal {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.cell-card {
  text-align: left;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 10px 12px;
  transition: all 0.2s ease;
  cursor: pointer;
  min-width: 200px;
  max-width: 230px;
  flex: 1;
  font: inherit;
}

.cell-card:hover:not(:disabled) {
  border-color: var(--accent);
  box-shadow: 0 2px 4px rgba(79, 91, 223, 0.1);
  background: var(--accent-tint);
}

.cell-card:disabled {
  cursor: default;
  opacity: 0.7;
}

.cell-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.cell-badge {
  font-size: 0.75em;
  font-weight: 600;
  color: var(--text-muted);
}

.cell-type-badge {
  font-size: 0.7em;
  padding: 2px 8px;
  border-radius: 999px;
  font-weight: 500;
}

.cell-type-badge.letters {
  background: var(--accent-tint);
  color: var(--accent-text);
}

.cell-type-badge.numbers {
  background: var(--success-bg);
  color: var(--success-text);
}

.cell-type-badge.mixed {
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  color: var(--warning-text);
}

.cell-card-details {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 0.8em;
}

.cell-length {
  color: var(--text-muted);
  font-weight: 500;
}

.cell-letters {
  font-family: monospace;
  background: var(--accent-tint);
  padding: 2px 6px;
  border-radius: 10px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-padding {
  color: var(--text-muted);
}

.details-actions {
  display: flex;
  justify-content: flex-end;
}

.details-meta {
  display: flex;
  gap: 16px;
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

/* Модалки */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.nf-modal {
  width: 100%;
  max-width: 680px;
  max-height: calc(var(--app-vh, 1vh) * 88);
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border-radius: 30px;
  box-shadow: 0 10px 30px var(--shadow-drop);
  overflow: hidden;
}

.nf-modal--narrow {
  max-width: 460px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.modal-close:hover {
  color: var(--text);
  background: var(--surface-2);
}

.modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

/* Редактор клеток в модалке */
.cells-edit-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cells-edit-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.cells-edit-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cell-edit-card {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 12px;
}

.cell-edit-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.cell-number {
  font-size: 0.8em;
  font-weight: 600;
  color: var(--text);
}

.cell-remove-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: 50%;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  transition: all 0.2s;
}

.cell-remove-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.cell-edit-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.cell-edit-fields .field {
  flex: 1;
  min-width: 130px;
}

/* .field-full перебивается более специфичным .cell-edit-fields .field { flex:1 } -
   возвращаем полю отдельную полную строку. */
.cell-edit-fields .field-full {
  flex: 0 0 100%;
}

.length-field {
  flex: 0 0 auto;
}

.length-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}

.length-input {
  width: 64px;
  text-align: center;
}

.length-dash {
  color: var(--text-muted);
  font-weight: 500;
}

.add-cell-btn {
  padding: 6px 14px;
  font-size: 12px;
}

.cell-preview {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 15px;
}

.cell-preview-label {
  font-size: 0.8em;
  color: var(--text-muted);
  font-weight: 500;
}

.cell-preview-value {
  font-family: monospace;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--accent-text);
}

/* Анимация открытия/закрытия */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-active .nf-modal,
.modal-fade-leave-active .nf-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: transparent;
}

.modal-fade-enter-from .nf-modal,
.modal-fade-leave-to .nf-modal {
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
  .content-container {
    flex-direction: column;
    height: auto;
  }
  .table-section,
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
     .truncate-text, иначе бейдж "по умолчанию"/"(архив)" в конце строки
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
  .form-row {
    flex-direction: column;
  }
}
</style>
