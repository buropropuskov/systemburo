<template>
  <BaseModal
    :show="show"
    title="Excel-бланк"
    width="95vw"
    closable
    :close-on-overlay="!rebindingMode"
    @close="onClose"
  >
    <div
      ref="modalBody"
      class="te-modal-body"
    >
      <!-- Баннер перепривязки -->
      <div
        v-if="rebindingMode"
        class="te-rebind-banner"
      >
        <span>
          Перепривязка: <strong>{{ getFieldLabel(rebindMapping.field_path) }}</strong>
          ({{ rebindMapping.old_cell_ref }}) - кликните на новую ячейку
        </span>
        <button
          class="lk-button lk-button--ghost te-btn-sm"
          @click="cancelRebind"
        >
          Отмена
        </button>
      </div>

      <!-- SVG линии поверх всего -->
      <svg
        v-if="showPaths && pathLines.length"
        ref="pathSvg"
        class="te-path-overlay"
        :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
      >
        <path
          v-for="line in pathLines"
          :key="line.id"
          :d="line.d"
          class="te-path-line"
          :class="{
            'te-path-dimmed': hoveredFieldPath && line.fieldPath !== hoveredFieldPath,
            'te-path-highlighted': hoveredFieldPath === line.fieldPath,
          }"
          :style="{ stroke: line.color, opacity: line.opacity }"
          @mouseenter="onPathHover(line)"
          @mouseleave="onPathLeave"
          @click="onPathClick(line, $event)"
        />
        <circle
          v-for="line in pathLines"
          :key="line.id + '-dot-l'"
          :cx="line.x2"
          :cy="line.y2"
          r="3"
          :fill="line.color"
          class="te-path-dot"
          :class="{ 'te-path-dimmed': hoveredFieldPath && line.fieldPath !== hoveredFieldPath }"
          :style="{ opacity: line.opacity }"
        />
        <circle
          v-for="line in pathLines"
          :key="line.id + '-dot-r'"
          :cx="line.x1"
          :cy="line.y1"
          r="3"
          :fill="line.color"
          class="te-path-dot"
          :class="{ 'te-path-dimmed': hoveredFieldPath && line.fieldPath !== hoveredFieldPath }"
          :style="{ opacity: line.opacity }"
        />
        <!-- Кнопка удаления на пути -->
        <foreignObject
          v-if="pathPopup"
          :x="pathPopup.x - 36"
          :y="pathPopup.y - 14"
          width="72"
          height="28"
          style="pointer-events: auto;"
        >
          <button
            class="te-path-delete-btn"
            @click="confirmPathDelete"
          >
            Удалить
          </button>
        </foreignObject>
      </svg>

      <!-- Левая панель: превью документа -->
      <div
        ref="previewPanel"
        class="te-preview-panel"
      >
        <XlsxViewer
          v-if="enabled && templateFileBuffer"
          ref="xlsxViewer"
          :file-buffer="templateFileBuffer"
          :mappings="enrichedMappings"
          :selected-cell="activeCellRef"
          :cell-colors="cellColorMap"
          @cell-click="onCellClick"
        />
        <div
          v-else
          class="te-preview-empty"
        >
          <span>Загрузите .xlsx файл для предпросмотра</span>
        </div>
      </div>

      <!-- Правая панель: настройки -->
      <div
        ref="settingsPanel"
        class="te-settings-panel"
      >
        <!-- Генерация бланка -->
        <div class="te-section">
          <ToggleSwitch
            :model-value="enabled"
            @update:model-value="onToggleEnabled"
          >
            Генерация бланка
          </ToggleSwitch>
        </div>

        <!-- Файл шаблона -->
        <div
          v-if="enabled"
          class="te-section"
        >
          <div
            v-if="template && template.file_path && !showUpload"
            class="te-file-block"
          >
            <div class="te-file-info">
              <span class="te-file-name">{{ template.original_file_name || 'template.xlsx' }}</span>
              <span class="te-file-meta">
                строки {{ template.list_start_row }}-{{ template.list_end_row }}
              </span>
            </div>
            <div class="te-file-actions">
              <button
                class="lk-button lk-button--ghost te-btn-sm"
                @click="downloadCurrentTemplate"
              >
                Скачать
              </button>
              <button
                class="lk-button lk-button--ghost te-btn-sm"
                @click="showUpload = true"
              >
                Заменить
              </button>
              <button
                class="lk-button lk-button--danger te-btn-sm"
                @click="onDeleteTemplate"
              >
                Удалить
              </button>
            </div>
          </div>

          <!-- Drag & Drop загрузка -->
          <div
            v-if="!template || !template.file_path || showUpload"
            class="te-upload-area"
          >
            <div
              class="te-dropzone"
              :class="{ 'te-dropzone--active': isDragging, 'te-dropzone--has-file': form.file }"
              @dragenter.prevent="isDragging = true"
              @dragover.prevent
              @dragleave.prevent="isDragging = false"
              @drop.prevent="onDrop"
            >
              <div
                v-if="form.file"
                class="te-dropzone__file"
              >
                <span class="te-dropzone__filename">{{ form.file.name }}</span>
                <button
                  class="te-dropzone__clear"
                  @click.stop="form.file = null"
                >
                  &times;
                </button>
              </div>
              <div
                v-else
                class="te-dropzone__placeholder"
              >
                <span class="te-dropzone__hint">Перетащите .xlsx файл сюда</span>
                <span class="te-dropzone__or">или</span>
                <label class="lk-button lk-button--ghost te-btn-sm te-dropzone__browse">
                  Выберите файл
                  <input
                    type="file"
                    accept=".xlsx"
                    hidden
                    @change="onFileChange"
                  >
                </label>
              </div>
            </div>
            <div class="te-upload-fields">
              <div class="te-form-field">
                <label>Начало списка</label>
                <input
                  v-model.number="form.listStartRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  required
                >
              </div>
              <div class="te-form-field">
                <label>Конец списка</label>
                <input
                  v-model.number="form.listEndRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  required
                >
              </div>
              <div class="te-form-field">
                <label>Макс. записей</label>
                <input
                  v-model.number="form.maxListRows"
                  type="number"
                  min="0"
                  class="lk-input te-compact-input"
                  placeholder="авто"
                >
              </div>
            </div>
            <div class="te-upload-actions">
              <button
                v-if="template && template.file_path"
                type="button"
                class="lk-button lk-button--ghost te-btn-sm"
                @click="showUpload = false"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary te-btn-sm"
                :disabled="!form.file || uploading"
                @click="onUpload"
              >
                {{ uploading ? 'Загрузка...' : 'Загрузить' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Поиск и фильтры -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-section te-section--compact"
        >
          <input
            v-model="searchQuery"
            type="text"
            class="lk-input te-compact-input"
            placeholder="Поиск полей..."
          >
          <div class="te-category-filter">
            <button
              v-for="g in fieldGroups"
              :key="g.group"
              class="te-cat-btn"
              :class="{ active: activeCategory === g.group }"
              @click="activeCategory = activeCategory === g.group ? '' : g.group"
            >
              {{ g.label }}
            </button>
          </div>
        </div>

        <!-- Поля для привязки -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-field-picker"
        >
          <div class="te-picker-header">
            <h4>Поля</h4>
            <span
              v-if="pendingFieldPath"
              class="te-pick-hint"
            >
              Кликните на ячейку
            </span>
          </div>
          <div
            ref="fieldPickerScroll"
            class="te-field-picker-scroll"
          >
            <div
              v-for="g in filteredFieldGroups"
              :key="g.group"
              class="te-field-group"
            >
              <span class="te-field-group-label">{{ g.label }}</span>
              <div class="te-field-chips">
                <button
                  v-for="f in g.fields"
                  :key="f.path"
                  :data-field-path="f.path"
                  class="te-field-chip"
                  :class="{
                    active: pendingFieldPath === f.path,
                    used: fieldPathUsed(f.path),
                  }"
                  :style="chipStyle(f.path)"
                  @click="selectField(f)"
                  @mouseenter="onChipHover(f.path)"
                  @mouseleave="onChipHover('')"
                >
                  <span class="te-chip-label">{{ f.label }}</span>
                  <span
                    v-if="fieldPathUsed(f.path)"
                    class="te-chip-ref"
                  >
                    {{ fieldCellRefs(f.path) }}
                  </span>
                  <span
                    v-if="fieldPathUsed(f.path)"
                    class="te-chip-remove"
                    @click.stop="removeMappingsByPath(f.path)"
                  >
                    &times;
                  </span>
                </button>
              </div>
            </div>
            <div
              v-if="filteredFieldGroups.length === 0"
              class="te-no-results"
            >
              Поля не найдены
            </div>
          </div>
        </div>

        <!-- Привязки (collapsible) -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-mappings-section"
        >
          <button
            class="te-section-toggle"
            @click="showMappings = !showMappings"
          >
            <h4>Привязки</h4>
            <span class="te-mappings-count">{{ mappings.length }}</span>
            <span
              class="te-chevron"
              :class="{ open: showMappings }"
            >&#9656;</span>
          </button>
          <div
            v-show="showMappings"
            class="te-mappings-body"
          >
            <div
              v-if="!mappings.length"
              class="te-empty-state"
            >
              Нет привязок
            </div>
            <div
              v-for="(m, idx) in enrichedMappings"
              :key="idx"
              class="te-mapping-row"
              :class="{ highlight: activeCellRef === m.cell_ref }"
            >
              <span class="te-mapping-cell">{{ m.cell_ref }}</span>
              <span class="te-mapping-field">{{ m.fieldLabel || m.field_path }}</span>
              <span
                v-if="m.is_list_field"
                class="te-list-badge"
              >список</span>
              <button
                class="te-mapping-remove"
                title="Удалить привязку"
                @click="removeMapping(idx)"
              >
                &times;
              </button>
            </div>
          </div>
        </div>

        <!-- Показать пути -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-section te-section--compact"
        >
          <ToggleSwitch v-model="showPaths">
            Показать пути
          </ToggleSwitch>
        </div>

        <!-- Сохранить -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-save-area"
        >
          <button
            class="lk-button lk-button--primary"
            :disabled="savingMappings"
            @click="saveMappings"
          >
            {{ savingMappings ? 'Сохранение...' : 'Сохранить привязки' }}
          </button>
        </div>
      </div>
    </div>
  </BaseModal>
</template>

<script>
import { useUiStore } from '@/stores/ui';
import {
  getTemplate, uploadTemplate, updateMappings, deleteTemplate,
  getTemplateFields, getTemplateFile, saveBlobAs,
} from '@/api/attachment-templates';
import BaseModal from '@/components/ui/BaseModal.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import XlsxViewer from './XlsxViewer.vue';

const PATH_COLORS = [
  '#4F5BDF', '#e85d75', '#2e9e5a', '#e8a317', '#8e44ad',
  '#16a085', '#c0392b', '#2980b9', '#d35400', '#7f8c8d',
];

export default {
  name: 'AttachmentTemplateEditor',
  components: { BaseModal, ToggleSwitch, XlsxViewer },
  props: {
    show: { type: Boolean, required: true },
    uniqueAttachmentId: { type: Number, required: true },
  },
  emits: ['close'],
  data() {
    return {
      template: null,
      mappings: [],
      fieldGroups: [],
      enabled: false,
      showUpload: false,
      isDragging: false,
      form: { file: null, listStartRow: 1, listEndRow: 1, maxListRows: 0 },
      uploading: false,
      savingMappings: false,
      templateFileBuffer: null,
      pendingFieldPath: '',
      pendingFieldLabel: '',
      pendingCellRef: '',
      searchQuery: '',
      activeCategory: '',
      showPaths: false,
      showMappings: false,
      hoveredFieldPath: '',
      pathPopup: null,
      rebindingMode: false,
      rebindMapping: null,
      pathLines: [],
      svgWidth: 0,
      svgHeight: 0,
      rafId: null,
    };
  },
  computed: {
    enrichedMappings() {
      return this.mappings.map(m => ({
        ...m,
        fieldLabel: this.getFieldLabel(m.field_path),
      }));
    },
    filteredFieldGroups() {
      let groups = this.fieldGroups;
      if (this.activeCategory) {
        groups = groups.filter(g => g.group === this.activeCategory);
      }
      if (this.searchQuery) {
        const q = this.searchQuery.toLowerCase();
        groups = groups.map(g => ({
          ...g,
          fields: g.fields.filter(f =>
            f.label.toLowerCase().includes(q) ||
            f.path.toLowerCase().includes(q)
          ),
        })).filter(g => g.fields.length > 0);
      }
      return groups;
    },
    activeCellRef() {
      if (this.rebindingMode && this.rebindMapping) return this.rebindMapping.old_cell_ref;
      return this.pendingCellRef;
    },
    cellColorMap() {
      if (!this.showPaths) return new Map();
      const map = new Map();
      this.mappings.forEach((m, i) => {
        const color = PATH_COLORS[i % PATH_COLORS.length];
        map.set(m.cell_ref.toUpperCase(), color);
      });
      return map;
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.loadAll();
      } else {
        this.cleanupPathListeners();
        this.resetState();
      }
    },
    showPaths(val) {
      if (val) {
        this.$nextTick(() => this.setupPathListeners());
      } else {
        this.cleanupPathListeners();
        this.pathLines = [];
        this.pathPopup = null;
      }
    },
    mappings: {
      deep: true,
      handler() {
        if (this.showPaths) this.$nextTick(() => this.updatePaths());
      },
    },
  },
  beforeUnmount() {
    this.cleanupPathListeners();
  },
  methods: {
    resetState() {
      this.pendingFieldPath = '';
      this.pendingCellRef = '';
      this.rebindingMode = false;
      this.rebindMapping = null;
      this.pathPopup = null;
      this.hoveredFieldPath = '';
    },
    onClose() {
      if (this.rebindingMode) {
        this.cancelRebind();
        return;
      }
      this.$emit('close');
    },
    async loadAll() {
      await Promise.all([this.loadTemplate(), this.loadFields()]);
    },
    async loadTemplate() {
      try {
        const data = await getTemplate(this.uniqueAttachmentId);
        this.template = data;
        this.mappings = (data && data.mappings) || [];
        this.enabled = !!(data && data.file_path);
        this.form.listStartRow = data && data.list_start_row || 1;
        this.form.listEndRow = data && data.list_end_row || 1;
        this.form.maxListRows = data && data.max_list_rows || 0;
        if (this.enabled) this.loadTemplateFile();
      } catch {
        this.template = null;
        this.mappings = [];
        this.enabled = false;
        this.templateFileBuffer = null;
      }
    },
    async loadTemplateFile() {
      try {
        this.templateFileBuffer = await getTemplateFile(this.uniqueAttachmentId);
      } catch {
        this.templateFileBuffer = null;
      }
    },
    async loadFields() {
      try {
        const data = await getTemplateFields(this.uniqueAttachmentId);
        this.fieldGroups = Array.isArray(data) ? data : [];
      } catch {
        this.fieldGroups = [];
      }
    },
    onToggleEnabled(val) {
      if (!val && this.template && this.template.file_path) {
        if (!confirm('Отключить генерацию бланка? Текущий шаблон будет удален.')) return;
        this.onDeleteTemplate();
      }
      this.enabled = val;
    },
    onFileChange(e) {
      this.form.file = e.target.files[0] || null;
    },
    onDrop(e) {
      this.isDragging = false;
      const files = e.dataTransfer.files;
      if (files.length > 0 && files[0].name.endsWith('.xlsx')) {
        this.form.file = files[0];
      }
    },
    async onUpload() {
      if (!this.form.file) return;
      this.uploading = true;
      try {
        await uploadTemplate(this.uniqueAttachmentId, this.form.file, {
          listStartRow: this.form.listStartRow,
          listEndRow: this.form.listEndRow,
          maxListRows: this.form.maxListRows,
        });
        useUiStore().success('Шаблон загружен');
        this.showUpload = false;
        this.form.file = null;
        await this.loadTemplate();
      } catch (err) {
        useUiStore().error(err.message || 'Не удалось загрузить шаблон');
      } finally {
        this.uploading = false;
      }
    },
    async onDeleteTemplate() {
      try {
        await deleteTemplate(this.uniqueAttachmentId);
        useUiStore().success('Шаблон удален');
        this.templateFileBuffer = null;
        await this.loadTemplate();
      } catch {
        useUiStore().error('Не удалось удалить шаблон');
      }
    },
    async downloadCurrentTemplate() {
      try {
        const buf = this.templateFileBuffer || await getTemplateFile(this.uniqueAttachmentId);
        const blob = new Blob([buf], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        saveBlobAs(blob, this.template.original_file_name || 'template.xlsx');
      } catch {
        useUiStore().error('Не удалось скачать шаблон');
      }
    },

    selectField(field) {
      if (this.rebindingMode) return;
      if (this.pendingFieldPath === field.path) {
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
      } else {
        this.pendingFieldPath = field.path;
        this.pendingFieldLabel = field.label;
      }
    },
    onCellClick(cellRef) {
      this.pathPopup = null;

      if (this.rebindingMode) {
        this.finishRebind(cellRef);
        return;
      }

      if (this.pendingFieldPath) {
        const existing = this.mappings.find(m => m.cell_ref === cellRef);
        if (existing && existing.field_path !== this.pendingFieldPath) {
          useUiStore().error(`Ячейка ${cellRef} уже привязана к "${this.getFieldLabel(existing.field_path)}"`);
          return;
        }
        const duplicate = this.mappings.find(
          m => m.cell_ref === cellRef && m.field_path === this.pendingFieldPath
        );
        if (duplicate) {
          this.pendingFieldPath = '';
          return;
        }
        this.mappings.push({
          cell_ref: cellRef,
          field_path: this.pendingFieldPath,
          is_list_field: this.isListField(this.pendingFieldPath),
        });
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
        return;
      }

      const mapped = this.mappings.find(m => m.cell_ref === cellRef);
      if (mapped) {
        this.startRebind(mapped, cellRef);
        return;
      }

      this.pendingCellRef = cellRef;
    },

    startRebind(mapping, cellRef) {
      const idx = this.mappings.findIndex(
        m => m.cell_ref === cellRef && m.field_path === mapping.field_path
      );
      this.rebindingMode = true;
      this.rebindMapping = {
        index: idx,
        field_path: mapping.field_path,
        old_cell_ref: cellRef,
      };
    },
    finishRebind(newCellRef) {
      if (!this.rebindMapping) return;
      const existing = this.mappings.find(m => m.cell_ref === newCellRef);
      if (existing && existing.field_path !== this.rebindMapping.field_path) {
        useUiStore().error(`Ячейка ${newCellRef} уже привязана к "${this.getFieldLabel(existing.field_path)}"`);
        return;
      }
      if (this.rebindMapping.index >= 0) {
        this.mappings[this.rebindMapping.index].cell_ref = newCellRef;
      }
      this.rebindingMode = false;
      this.rebindMapping = null;
    },
    cancelRebind() {
      this.rebindingMode = false;
      this.rebindMapping = null;
    },

    removeMapping(idx) {
      this.mappings.splice(idx, 1);
    },
    removeMappingsByPath(fieldPath) {
      this.mappings = this.mappings.filter(m => m.field_path !== fieldPath);
    },
    isListField(fieldPath) {
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === fieldPath);
        if (f) return !!f.is_list;
      }
      return false;
    },
    fieldPathUsed(path) {
      return this.mappings.some(m => m.field_path === path);
    },
    fieldCellRefs(path) {
      return this.mappings
        .filter(m => m.field_path === path)
        .map(m => m.cell_ref)
        .join(', ');
    },
    getFieldLabel(fieldPath) {
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === fieldPath);
        if (f) return f.label;
      }
      return fieldPath;
    },
    getFieldColor(fieldPath) {
      const idx = this.mappings.findIndex(m => m.field_path === fieldPath);
      return idx >= 0 ? PATH_COLORS[idx % PATH_COLORS.length] : '';
    },
    chipStyle(fieldPath) {
      if (!this.showPaths || !this.fieldPathUsed(fieldPath)) return {};
      return { borderColor: this.getFieldColor(fieldPath), borderWidth: '2px' };
    },
    async saveMappings() {
      this.savingMappings = true;
      try {
        const payload = this.mappings.filter(m => m.cell_ref && m.field_path);
        await updateMappings(this.uniqueAttachmentId, payload);
        useUiStore().success('Привязки сохранены');
        await this.loadTemplate();
      } catch (err) {
        useUiStore().error(err.message || 'Не удалось сохранить');
      } finally {
        this.savingMappings = false;
      }
    },

    onChipHover(fieldPath) {
      this.hoveredFieldPath = fieldPath;
    },
    onPathHover(line) {
      this.hoveredFieldPath = line.fieldPath;
    },
    onPathLeave() {
      this.hoveredFieldPath = '';
    },
    onPathClick(line, e) {
      e.stopPropagation();
      this.pathPopup = {
        x: (line.x1 + line.x2) / 2,
        y: (line.y1 + line.y2) / 2,
        fieldPath: line.fieldPath,
        cellRef: line.cellRef,
      };
    },
    confirmPathDelete() {
      if (!this.pathPopup) return;
      const idx = this.mappings.findIndex(
        m => m.field_path === this.pathPopup.fieldPath && m.cell_ref === this.pathPopup.cellRef
      );
      if (idx >= 0) this.mappings.splice(idx, 1);
      this.pathPopup = null;
    },

    setupPathListeners() {
      this.cleanupPathListeners();
      const body = this.$refs.modalBody;
      if (!body) return;

      const scrollables = body.querySelectorAll(
        '.te-preview-panel, .xv-table-wrap, .te-field-picker-scroll, .te-settings-panel'
      );
      this._scrollHandler = () => {
        if (this.rafId) cancelAnimationFrame(this.rafId);
        this.rafId = requestAnimationFrame(() => this.updatePaths());
      };
      scrollables.forEach(el =>
        el.addEventListener('scroll', this._scrollHandler, { passive: true })
      );

      this._resizeObserver = new ResizeObserver(this._scrollHandler);
      this._resizeObserver.observe(body);

      this.updatePaths();
    },
    cleanupPathListeners() {
      if (this._scrollHandler) {
        const body = this.$refs.modalBody;
        if (body) {
          const scrollables = body.querySelectorAll(
            '.te-preview-panel, .xv-table-wrap, .te-field-picker-scroll, .te-settings-panel'
          );
          scrollables.forEach(el => el.removeEventListener('scroll', this._scrollHandler));
        }
      }
      if (this._resizeObserver) {
        this._resizeObserver.disconnect();
        this._resizeObserver = null;
      }
      if (this.rafId) {
        cancelAnimationFrame(this.rafId);
        this.rafId = null;
      }
    },
    updatePaths() {
      const body = this.$refs.modalBody;
      const previewPanel = this.$refs.previewPanel;
      const settingsPanel = this.$refs.settingsPanel;
      if (!body || !previewPanel || !settingsPanel) {
        this.pathLines = [];
        return;
      }

      const bodyRect = body.getBoundingClientRect();
      const previewRect = previewPanel.getBoundingClientRect();

      const pickerScroll = this.$refs.fieldPickerScroll;
      const pickerRect = pickerScroll ? pickerScroll.getBoundingClientRect() : null;

      this.svgWidth = bodyRect.width;
      this.svgHeight = bodyRect.height;

      const fadeZone = 30;
      const lines = [];

      this.enrichedMappings.forEach((m, i) => {
        const cellEl = previewPanel.querySelector(`[data-cell-ref="${m.cell_ref}"]`);
        const chipEl = settingsPanel.querySelector(`[data-field-path="${m.field_path}"]`);
        if (!cellEl || !chipEl) return;

        const cellRect = cellEl.getBoundingClientRect();
        const chipRect = chipEl.getBoundingClientRect();

        const cellDistTop = cellRect.top - previewRect.top;
        const cellDistBot = previewRect.bottom - cellRect.bottom;
        const chipDistTop = pickerRect ? chipRect.top - pickerRect.top : 100;
        const chipDistBot = pickerRect ? pickerRect.bottom - chipRect.bottom : 100;

        const minDist = Math.min(cellDistTop, cellDistBot, chipDistTop, chipDistBot);
        if (minDist < -5) return;

        let opacity = 0.6;
        if (minDist < fadeZone) opacity = Math.max(0.05, 0.6 * (minDist / fadeZone));

        const x1 = chipRect.left - bodyRect.left;
        const y1 = chipRect.top + chipRect.height / 2 - bodyRect.top;
        const x2 = cellRect.right - bodyRect.left;
        const y2 = cellRect.top + cellRect.height / 2 - bodyRect.top;

        const dx = Math.abs(x1 - x2) * 0.4;
        const d = `M${x1},${y1} C${x1 - dx},${y1} ${x2 + dx},${y2} ${x2},${y2}`;

        lines.push({
          id: m.field_path + '-' + m.cell_ref,
          fieldPath: m.field_path,
          cellRef: m.cell_ref,
          d,
          x1, y1, x2, y2,
          opacity,
          color: PATH_COLORS[i % PATH_COLORS.length],
        });
      });

      this.pathLines = lines;
    },
  },
};
</script>

<style scoped>
/* ---- Layout ---- */
.te-modal-body {
  display: flex;
  position: relative;
  height: calc(85vh - 120px);
  min-height: 300px;
}

.te-preview-panel {
  flex: 1;
  min-width: 0;
  overflow: auto;
  border-right: 1px solid var(--color-border);
}

.te-preview-panel :deep(.xlsx-viewer) {
  border: none;
  border-radius: 0;
  min-height: 100%;
}

.te-preview-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-muted);
  font-size: 14px;
}

.te-settings-panel {
  width: 360px;
  flex-shrink: 0;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* ---- Rebind banner ---- */
.te-rebind-banner {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 25;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  background: #fff3cd;
  border-bottom: 1px solid #ffc107;
  font-size: 13px;
  color: #856404;
}

/* ---- SVG paths ---- */
.te-path-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 15;
  overflow: visible;
}

.te-path-line {
  fill: none;
  stroke-width: 1.5;
  pointer-events: stroke;
  cursor: pointer;
  transition: opacity 0.25s ease, stroke-width 0.25s ease;
}

.te-path-line:hover {
  stroke-width: 3;
}

.te-path-line.te-path-highlighted {
  stroke-width: 2.5;
}

.te-path-line.te-path-dimmed {
  opacity: 0 !important;
}

.te-path-dot {
  transition: opacity 0.25s ease;
}

.te-path-dot.te-path-dimmed {
  opacity: 0 !important;
}

.te-path-delete-btn {
  display: block;
  width: 100%;
  padding: 4px 8px;
  background: var(--color-danger);
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  text-align: center;
}

.te-path-delete-btn:hover {
  background: #c82333;
}

/* ---- Sections ---- */
.te-section {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  background: #fff;
}

.te-section--compact {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* ---- File block ---- */
.te-file-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.te-file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.te-file-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.te-file-meta {
  font-size: 11px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 6px;
  border-radius: 4px;
}

.te-file-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

/* ---- Drag & Drop ---- */
.te-upload-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.te-dropzone {
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-sm);
  padding: 16px;
  text-align: center;
  transition: all 0.2s ease;
  background: #fff;
}

.te-dropzone--active {
  border-color: var(--color-primary);
  background: #f0f4ff;
}

.te-dropzone--has-file {
  border-style: solid;
  border-color: var(--color-success);
  background: #f0fdf4;
}

.te-dropzone__file {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.te-dropzone__filename {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.te-dropzone__clear {
  background: none;
  border: none;
  color: var(--color-danger);
  font-size: 18px;
  cursor: pointer;
  line-height: 1;
}

.te-dropzone__placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.te-dropzone__hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

.te-dropzone__or {
  font-size: 11px;
  color: #bbb;
}

.te-dropzone__browse {
  cursor: pointer;
}

.te-upload-fields {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 6px;
}

.te-form-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.te-form-field label {
  font-size: 11px;
  font-weight: 500;
  color: #888;
}

.te-compact-input {
  padding: 5px 8px !important;
  font-size: 12px !important;
}

.te-upload-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

/* ---- Filters ---- */
.te-category-filter {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.te-cat-btn {
  padding: 3px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  background: #fff;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
  white-space: nowrap;
}

.te-cat-btn:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.te-cat-btn.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

/* ---- Field picker ---- */
.te-field-picker {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: #fff;
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
}

.te-picker-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.te-picker-header h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.te-pick-hint {
  font-size: 11px;
  color: var(--color-primary);
  font-weight: 500;
  animation: te-pulse 1.5s ease-in-out infinite;
}

@keyframes te-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.te-field-picker-scroll {
  overflow-y: auto;
  padding: 8px 10px;
  flex: 1;
  min-height: 0;
}

.te-field-group {
  margin-bottom: 8px;
}

.te-field-group:last-child {
  margin-bottom: 0;
}

.te-field-group-label {
  display: block;
  font-size: 10px;
  color: var(--color-text-muted);
  margin-bottom: 3px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}

.te-field-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}

.te-field-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  font-size: 11px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
}

.te-field-chip:hover {
  border-color: var(--color-primary);
  background: #f0f4ff;
}

.te-field-chip.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.te-field-chip.used {
  background: #e8f8e8;
  border-color: var(--color-success);
  color: #1a6e2e;
}

.te-field-chip.used:hover {
  background: #d4f0d4;
}

.te-chip-label {
  white-space: nowrap;
}

.te-chip-ref {
  font-size: 9px;
  opacity: 0.7;
  font-weight: 600;
}

.te-chip-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  font-size: 13px;
  line-height: 1;
  color: var(--color-danger);
  background: rgba(220, 53, 69, 0.1);
  transition: all 0.15s;
}

.te-chip-remove:hover {
  background: var(--color-danger);
  color: #fff;
}

.te-no-results {
  font-size: 12px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 12px;
}

/* ---- Mappings (collapsible) ---- */
.te-mappings-section {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: #fff;
  flex-shrink: 0;
}

.te-section-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px 12px;
}

.te-section-toggle h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.te-mappings-count {
  font-size: 10px;
  background: var(--color-primary);
  color: #fff;
  padding: 1px 6px;
  border-radius: var(--radius-pill);
  font-weight: 600;
}

.te-chevron {
  margin-left: auto;
  transition: transform 0.2s;
  font-size: 11px;
  color: var(--color-text-muted);
}

.te-chevron.open {
  transform: rotate(90deg);
}

.te-mappings-body {
  padding: 0 10px 8px;
  max-height: 180px;
  overflow-y: auto;
}

.te-empty-state {
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 8px 0;
}

.te-mapping-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 11px;
  transition: background 0.1s;
}

.te-mapping-row:hover {
  background: var(--color-bg-secondary);
}

.te-mapping-row.highlight {
  background: #e8f4fd;
}

.te-mapping-cell {
  font-weight: 600;
  min-width: 30px;
  color: var(--color-primary);
}

.te-mapping-field {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
}

.te-list-badge {
  font-size: 9px;
  background: #e3f2fd;
  color: #1565c0;
  padding: 1px 5px;
  border-radius: var(--radius-pill);
  font-weight: 500;
}

.te-mapping-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: none;
  background: none;
  color: var(--color-text-muted);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.15s;
  flex-shrink: 0;
}

.te-mapping-remove:hover {
  background: rgba(220, 53, 69, 0.1);
  color: var(--color-danger);
}

/* ---- Save ---- */
.te-save-area {
  flex-shrink: 0;
}

/* ---- Button sizes ---- */
.te-btn-sm {
  padding: 5px 12px !important;
  font-size: 11px !important;
}

/* ---- Responsive ---- */
@media (max-width: 900px) {
  .te-modal-body {
    flex-direction: column;
    height: auto;
  }

  .te-preview-panel {
    max-height: 50vh;
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }

  .te-settings-panel {
    width: 100%;
  }
}
</style>
